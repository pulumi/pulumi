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
	"os"
	"strconv"
	"strings"
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
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
	envVar, has := os.LookupEnv(EnvPolicyPackAttach)
	if !has {
		return nil, nil
	}
	for _, entry := range strings.Split(envVar, ",") {
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 || parts[0] != string(name) {
			continue
		}
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("expected a numeric port for %q in %s, got %q: %w",
				parts[0], EnvPolicyPackAttach, parts[1], err)
		}
		return &port, nil
	}
	return nil, nil
}

const analyzerReadyTimeout = 2 * time.Minute

// localOCIImageRef resolves the image to run for a local `--policy-pack ./dir`
// OCI pack: <repository>:<version>, falling back to :latest when the manifest
// has no version. The image must have been built locally (the CLI never
// builds or implicitly pulls for local packs).
func localOCIImageRef(proj *workspace.PolicyPackProject, path string) (string, error) {
	repo, _ := proj.Runtime.Options()["repository"].(string)
	if repo == "" {
		return "", fmt.Errorf("policy pack at %q has runtime \"oci\" but no \"repository\" runtime option; "+
			"set runtime.options.repository in PulumiPolicy.yaml to the pack's registry repository", path)
	}
	tag := proj.Version
	if tag == "" {
		tag = "latest"
	}
	return repo + ":" + tag, nil
}

// newOCIPolicyAnalyzer launches the pack image in a container and connects to
// its analyzer.
func newOCIPolicyAnalyzer(
	host Host, ctx *Context, name tokens.QName, image string, opts *PolicyAnalyzerOptions,
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
		Image:    image,
		PackName: string(name),
		Env:      packEnv,
		Mode:     mode,
	})
	if err != nil {
		return nil, fmt.Errorf("policy pack %q: %w "+
			"(for a local pack, build and tag the image as %q first -- the CLI never builds it)",
			name, err, image)
	}

	conn, err := dialAnalyzerWithRetry(ctx.Request(), container.Addr, analyzerReadyTimeout,
		func() (bool, string) {
			return container.Running(ctx.Request()), container.Logs(ctx.Request())
		})
	if err != nil {
		contract.IgnoreClose(container)
		return nil, fmt.Errorf("policy pack %q: %w", name, err)
	}

	client := pulumirpc.NewAnalyzerClient(conn)

	// Handshake with an engine address the container can reach.
	engineAddr := oci.EngineAddressFor(mode, host.ServerAddr())
	if err := ociHandshake(ctx.Request(), client, name, engineAddr); err != nil {
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
		name:   name,
		client: client,
		closeFn: func() error {
			contract.IgnoreClose(conn)
			return container.Close()
		},
	}, nil
}

// attachPolicyAnalyzer connects to a policy pack that is already running at a
// known loopback port (PULUMI_POLICY_PACK_ATTACH).
func attachPolicyAnalyzer(
	host Host, ctx *Context, name tokens.QName, port int, opts *PolicyAnalyzerOptions,
) (Analyzer, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := dialAnalyzerWithRetry(ctx.Request(), addr, analyzerReadyTimeout, nil)
	if err != nil {
		return nil, fmt.Errorf("attaching to policy pack %q at %s (from %s): %w",
			name, addr, EnvPolicyPackAttach, err)
	}
	client := pulumirpc.NewAnalyzerClient(conn)

	if err := ociHandshake(ctx.Request(), client, name, host.ServerAddr()); err != nil {
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

// ociHandshake performs the analyzer handshake over an established connection.
// Containerized/attached packs get no Root/ProgramDirectory: host paths are
// meaningless inside the pack's filesystem. Unimplemented is tolerated, as in
// the process-launch path.
func ociHandshake(
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
// is listening — so we require the channel itself to become READY. If
// containerCheck is non-nil and reports the container has exited, we fail fast
// with its logs instead of waiting out the timeout.
func dialAnalyzerWithRetry(
	ctx context.Context, addr string, timeout time.Duration,
	containerCheck func() (running bool, logs string),
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
		if containerCheck != nil {
			if running, logs := containerCheck(); !running {
				contract.IgnoreClose(conn)
				return nil, fmt.Errorf("policy pack container exited before serving its analyzer; container logs:\n%s", logs)
			}
		}
		waitCtx, cancel := context.WithDeadline(ctx, deadline)
		changed := conn.WaitForStateChange(waitCtx, state)
		cancel()
		if !changed {
			var logs string
			if containerCheck != nil {
				_, logs = containerCheck()
				logs = "; container logs:\n" + logs
			}
			contract.IgnoreClose(conn)
			return nil, fmt.Errorf("timed out after %v waiting for policy pack analyzer at %s%s",
				timeout, addr, logs)
		}
	}
}
