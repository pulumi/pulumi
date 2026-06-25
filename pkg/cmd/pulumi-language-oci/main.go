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

// pulumi-language-oci is a prototype language host for containerized program
// execution. Instead of running a program as a child process in some ambient
// language toolchain, it runs the program as an OCI container — the container
// IS the program's shape declaration (see oci-execution-design.md).
//
// Run() has three operating modes so the plumbing can be validated in layers:
//
//   - subprocess mode (default): exec the program binary directly, passing the
//     monitor address through unchanged. Proves discovery + the RPC sequence +
//     Run + the backend with zero networking variables.
//   - pod mode, engine on the host (PULUMI_POD_MODE=true, no pod network):
//     `docker run` the program image on the default bridge and rewrite the
//     advertised monitor/engine addresses to host.docker.internal so the
//     container dials back to the host engine (design Option A).
//   - pod mode, engine in a container (PULUMI_POD_MODE=true + PULUMI_POD_NETWORK):
//     the engine itself runs in a container on a shared pod network; the program
//     joins that network and reaches the engine by its container DNS name (design
//     Option C). PULUMI_POD_ADVERTISE_HOST names that DNS host; absent it, we fall
//     back to this process's own hostname (the engine container's name).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// version is reported via GetPluginInfo. This is a prototype, hence 0.x.
const version = "0.1.0"

func main() {
	// The engine launches a language host as: pulumi-language-oci [-tracing=…] <engineAddress>
	// Parse leniently: we ignore flags and take the first positional as the engine address.
	fs := flag.NewFlagSet("pulumi-language-oci", flag.ContinueOnError)
	fs.String("tracing", "", "ignored")
	_ = fs.Parse(os.Args[1:])

	var engineAddress string
	if rest := fs.Args(); len(rest) > 0 {
		engineAddress = rest[0]
	}

	host := &ociHost{engineAddress: engineAddress}

	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterLanguageRuntimeServer(srv, host)
			return nil
		},
	})
	if err != nil {
		cmdutil.Exit(fmt.Errorf("could not start language host RPC server: %w", err))
	}

	// Print the port so the engine knows how to reach us.
	fmt.Printf("%d\n", handle.Port)

	if err := <-handle.Done; err != nil {
		cmdutil.Exit(fmt.Errorf("language host RPC stopped serving: %w", err))
	}
}

// ociHost implements the minimal subset of LanguageRuntime needed to run a
// program. Everything else is left to UnimplementedLanguageRuntimeServer, which
// returns codes.Unimplemented — the engine does not call the rest during `up`.
type ociHost struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	engineAddress string
}

func (h *ociHost) GetPluginInfo(context.Context, *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{Version: version}, nil
}

func (h *ociHost) Handshake(
	_ context.Context, req *pulumirpc.LanguageHandshakeRequest,
) (*pulumirpc.LanguageHandshakeResponse, error) {
	if req != nil && req.EngineAddress != "" {
		h.engineAddress = req.EngineAddress
	}
	return &pulumirpc.LanguageHandshakeResponse{}, nil
}

// GetRequiredPlugins/GetRequiredPackages are best-effort pre-pull hints; lazy
// discovery at RegisterResource time is authoritative. The prototype reports
// none and lets discovery drive provider startup.
func (h *ociHost) GetRequiredPlugins(
	context.Context, *pulumirpc.GetRequiredPluginsRequest,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	return &pulumirpc.GetRequiredPluginsResponse{}, nil
}

func (h *ociHost) GetRequiredPackages(
	context.Context, *pulumirpc.GetRequiredPackagesRequest,
) (*pulumirpc.GetRequiredPackagesResponse, error) {
	return &pulumirpc.GetRequiredPackagesResponse{}, nil
}

// RuntimeOptionsPrompts is consulted by the CLI to fill in missing runtime
// options. The OCI runtime needs no interactive options, so report none.
func (h *ociHost) RuntimeOptionsPrompts(
	context.Context, *pulumirpc.RuntimeOptionsRequest,
) (*pulumirpc.RuntimeOptionsResponse, error) {
	return &pulumirpc.RuntimeOptionsResponse{}, nil
}

func (h *ociHost) About(context.Context, *pulumirpc.AboutRequest) (*pulumirpc.AboutResponse, error) {
	return &pulumirpc.AboutResponse{Executable: "docker", Version: version}, nil
}

func (h *ociHost) Cancel(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// Run starts the program, either as a local subprocess or as a container, and
// blocks until it exits.
func (h *ociHost) Run(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	podMode := os.Getenv("PULUMI_POD_MODE") == "true"

	monitor, engine := req.MonitorAddress, h.engineAddress
	if podMode {
		// The engine binds 0.0.0.0 but advertises a loopback host it can't know is
		// reachable from elsewhere. Rewrite the host portion to one the program
		// container can dial: host.docker.internal when the engine is on the host,
		// or the engine container's DNS name when it runs on the pod network. The
		// shim sets PULUMI_POD_ADVERTISE_HOST; absent it, fall back to our own
		// hostname (equal to the engine container's name in the in-container case).
		advertiseHost := os.Getenv("PULUMI_POD_ADVERTISE_HOST")
		if advertiseHost == "" {
			advertiseHost, _ = os.Hostname()
		}
		monitor = rewriteHost(monitor, advertiseHost)
		engine = rewriteHost(engine, advertiseHost)
		fmt.Fprintf(os.Stderr, "oci: pod mode — advertising monitor=%s engine=%s\n", monitor, engine)
	}

	env := map[string]string{
		"PULUMI_MONITOR":      monitor,
		"PULUMI_ENGINE":       engine,
		"PULUMI_PROJECT":      req.Project,
		"PULUMI_STACK":        req.Stack,
		"PULUMI_ORGANIZATION": req.Organization,
		"PULUMI_DRY_RUN":      strconv.FormatBool(req.DryRun),
		"PULUMI_PARALLEL":     strconv.Itoa(int(req.Parallel)),
	}
	if cfg, err := json.Marshal(orEmptyMap(req.Config)); err == nil {
		env["PULUMI_CONFIG"] = string(cfg)
	}
	if keys, err := json.Marshal(orEmptySlice(req.ConfigSecretKeys)); err == nil {
		env["PULUMI_CONFIG_SECRET_KEYS"] = string(keys)
	}

	opts := req.GetInfo().GetOptions()

	var cmd *exec.Cmd
	if podMode {
		image := optString(opts, "image")
		if image == "" {
			return nil, errors.New("oci: runtime option 'image' is required in pod mode")
		}
		cmd = exec.CommandContext(ctx, "docker", dockerRunArgs(image, env)...)
	} else {
		program := optString(opts, "program")
		if program == "" {
			return nil, errors.New("oci: runtime option 'program' is required for subprocess mode")
		}
		if !filepath.IsAbs(program) {
			program = filepath.Join(req.GetInfo().GetProgramDirectory(), program)
		}
		cmd = exec.CommandContext(ctx, program)
		cmd.Env = append(os.Environ(), envSlice(env)...)
	}

	// The program's output goes to stderr; stdout is reserved for the language
	// host's port-line protocol with the engine.
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// The program (or docker) ran and exited non-zero; its own output
			// already explained why. Bail so the engine halts without double-reporting.
			return &pulumirpc.RunResponse{Bail: true}, nil
		}
		return nil, fmt.Errorf("oci: starting program: %w", err)
	}
	return &pulumirpc.RunResponse{}, nil
}

// dockerRunArgs builds the argv for `docker run` of a program image. Env keys are
// emitted in sorted order so the argv is deterministic.
func dockerRunArgs(image string, env map[string]string) []string {
	args := []string{"run", "--rm"}
	if network := os.Getenv("PULUMI_POD_NETWORK"); network != "" {
		// Engine-in-container: join the pod network and reach the engine by its
		// container DNS name (no host gateway needed).
		args = append(args, "--network", network)
	} else {
		// Engine on the host: the program runs on the default bridge and reaches
		// the host engine through Docker's host-gateway alias.
		args = append(args, "--add-host=host.docker.internal:host-gateway")
	}
	for _, k := range sortedKeys(env) {
		args = append(args, "-e", k+"="+env[k])
	}
	return append(args, image)
}

// rewriteHost replaces the host portion of a host:port address, preserving the
// port. Returns addr unchanged if it is not a valid host:port.
func rewriteHost(addr, newHost string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return net.JoinHostPort(newHost, port)
}

func optString(s *structpb.Struct, key string) string {
	if s == nil {
		return ""
	}
	return s.GetFields()[key].GetStringValue() // nil-safe: missing key -> ""
}

func envSlice(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for _, k := range sortedKeys(env) {
		out = append(out, k+"="+env[k])
	}
	return out
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func orEmptyMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

func orEmptySlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
