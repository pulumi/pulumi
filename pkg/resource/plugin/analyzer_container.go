// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package plugin

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin/oci"
)

// EnvPolicyPackAttach lists policy packs the engine should attach to at a
// known port instead of launching, in the form "<pack-name>:<port>[,…]".
// This is the policy pack analogue of PULUMI_DEBUG_PROVIDERS: it is how packs
// running as pod sidecars are reached, and a debugging affordance for pack
// authors (run the pack under a debugger and point the CLI at it).
const EnvPolicyPackAttach = "PULUMI_POLICY_PACK_ATTACH"

// GetPolicyPackAttachPort returns the attach port for the named policy pack
// from EnvPolicyPackAttach, or nil if the pack is not listed.
func GetPolicyPackAttachPort(name tokens.QName) (*int, error) {
	return attachPortFromEnv(EnvPolicyPackAttach, string(name))
}

// containerPackReason reports why a pack must be booted by a container-capable
// host, or "" when the ordinary process-launch path applies. The manifest
// runtime is the caller's to check: attach and image-ref packs have no local
// manifest to consult.
func containerPackReason(name tokens.QName, opts *PolicyAnalyzerOptions) (string, error) {
	if port, err := GetPolicyPackAttachPort(name); err != nil {
		return "", err
	} else if port != nil {
		return "is listed in " + EnvPolicyPackAttach, nil
	}
	if opts != nil && opts.ImageRef != "" {
		return fmt.Sprintf("is a container image (%s)", opts.ImageRef), nil
	}
	return "", nil
}

// errContainerHostRequired is the error for a container or attach pack
// reaching a host that cannot boot one.
func errContainerHostRequired(name tokens.QName, reason string) error {
	return fmt.Errorf("policy pack %q %s, which this host cannot run; "+
		"the plugin host must be wrapped with host.NewContainerHost", name, reason)
}

const analyzerReadyTimeout = 2 * time.Minute

// ContainerPack identifies the container image a policy pack runs as. Version
// and Description come from the pack's PulumiPolicy.yaml manifest when one was
// loaded (local packs); both are empty for a digest-pinned image-ref
// publish/install, which has no manifest to read.
type ContainerPack struct {
	Image       string
	Version     string
	Description string
}

// NewContainerPolicyAnalyzer launches the pack image in a container and connects to its analyzer.
func NewContainerPolicyAnalyzer(
	host Host, ctx *Context, name tokens.QName, pack ContainerPack, opts *PolicyAnalyzerOptions,
) (Analyzer, error) {
	rt, err := oci.DetectRuntime(nil)
	if err != nil {
		return nil, fmt.Errorf("policy pack %q: %w", name, err)
	}
	mode := oci.DetectMode()

	var packEnv map[string]string
	if opts != nil && len(opts.AdditionalEnv) > 0 {
		packEnv = opts.AdditionalEnv
	}

	container, err := rt.Launch(ctx.Request(), oci.LaunchOptions{
		Image:    pack.Image,
		PackName: string(name),
		Env:      packEnv,
		Mode:     mode,
	})
	if err != nil {
		return nil, fmt.Errorf("policy pack %q: %w "+
			"(for a local pack, build and tag the image as %q first -- the CLI never builds it)",
			name, err, pack.Image)
	}

	conn, err := dialAnalyzerWithRetry(ctx.Request(), container.Addr, analyzerReadyTimeout,
		func() bool { return container.Running(ctx.Request()) },
		func() string { return container.Logs(ctx.Request()) })
	if err != nil {
		contract.IgnoreClose(container)
		return nil, fmt.Errorf("policy pack %q: %w", name, err)
	}

	client := pulumirpc.NewAnalyzerClient(conn)

	// Handshake with an engine address the container can reach.
	engineAddr := oci.EngineAddressFor(mode, host.ServerAddr())
	if err := containerHandshake(ctx.Request(), client, name, engineAddr); err != nil {
		contract.IgnoreClose(conn)
		contract.IgnoreClose(container)
		return nil, err
	}

	if err := configureStack(ctx, client, name, opts); err != nil {
		contract.IgnoreClose(conn)
		contract.IgnoreClose(container)
		return nil, err
	}

	return &analyzer{
		name:        name,
		client:      client,
		version:     pack.Version,
		description: pack.Description,
		closeFn: func() error {
			contract.IgnoreClose(conn)
			return container.Close()
		},
	}, nil
}

// AttachPolicyAnalyzer connects to a policy pack that is already running at a
// known loopback port (PULUMI_POLICY_PACK_ATTACH).
func AttachPolicyAnalyzer(
	host Host, ctx *Context, name tokens.QName, port int, opts *PolicyAnalyzerOptions,
) (Analyzer, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := dialAnalyzerWithRetry(ctx.Request(), addr, analyzerReadyTimeout, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("attaching to policy pack %q at %s (from %s): %w",
			name, addr, EnvPolicyPackAttach, err)
	}
	client := pulumirpc.NewAnalyzerClient(conn)

	if err := containerHandshake(ctx.Request(), client, name, host.ServerAddr()); err != nil {
		contract.IgnoreClose(conn)
		return nil, err
	}
	if err := configureStack(ctx, client, name, opts); err != nil {
		contract.IgnoreClose(conn)
		return nil, err
	}
	return &analyzer{
		name:    name,
		client:  client,
		closeFn: conn.Close,
	}, nil
}

// containerHandshake performs the analyzer handshake over an established connection.
// Containerized/attached packs get no Root/ProgramDirectory: host paths are
// meaningless inside the pack's filesystem. Unimplemented is tolerated, as in
// the process-launch path.
func containerHandshake(
	reqCtx context.Context, client pulumirpc.AnalyzerClient, name tokens.QName, engineAddr string,
) error {
	_, err := client.Handshake(reqCtx, &pulumirpc.AnalyzerHandshakeRequest{
		EngineAddress: engineAddr,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.Unimplemented {
			logging.V(7).Infof("Handshake: not supported by policy pack %q", name)
			return nil
		}
		return fmt.Errorf("handshake with policy pack %q failed: %w", name, err)
	}
	return nil
}

// dialAnalyzerWithRetry dials addr and waits until the gRPC channel is READY,
// retrying transient failures until timeout. A raw TCP connect is not
// sufficient readiness — container runtimes bind the host port before the pack
// is listening — so we require the channel itself to become READY. If running
// is non-nil and reports the container has exited, we fail fast instead of
// waiting out the timeout. Both the exited-early and timeout errors include
// container logs (via logs, if non-nil) — a timeout with a still-running
// container (wrong port, slow start) is the main case that diagnostic exists
// for. running is polled every retry iteration so it must stay cheap; logs is
// only invoked when an error is being built.
func dialAnalyzerWithRetry(
	ctx context.Context, addr string, timeout time.Duration,
	running func() bool, logs func() string,
) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions())
	if err != nil {
		return nil, fmt.Errorf("could not create connection to policy pack at %s: %w", addr, err)
	}

	deadline := time.Now().Add(timeout)
	conn.Connect()
	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			return conn, nil
		}
		if running != nil && !running() {
			contract.IgnoreClose(conn)
			var containerLogs string
			if logs != nil {
				containerLogs = logs()
			}
			return nil, fmt.Errorf("policy pack container exited before serving its analyzer; container logs:\n%s",
				containerLogs)
		}
		waitCtx, cancel := context.WithDeadline(ctx, deadline)
		changed := conn.WaitForStateChange(waitCtx, state)
		cancel()
		if !changed {
			var logsSuffix string
			if logs != nil {
				logsSuffix = "; container logs:\n" + logs()
			}
			contract.IgnoreClose(conn)
			return nil, fmt.Errorf("timed out after %v waiting for policy pack analyzer at %s%s",
				timeout, addr, logsSuffix)
		}
	}
}
