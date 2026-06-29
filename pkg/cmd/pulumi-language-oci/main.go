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
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/pkg/v3/oci"
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

// InstallDependencies builds the program's local component images. In the OCI
// model the "dependency" that needs installing is a local program-as-component:
// its image does not exist yet (unlike a published MLC, whose image is pulled by
// convention), so we build it here and tag it by the provider convention
// (pulumi-provider-<name>:v<version>) — the same ref the container host resolves
// when it starts the component at Construct time.
//
// This works across the process split that defeated stashing the *program* image
// ref (the up pre-install host, this, and the engine-update host are all different
// processes): the built image lands in the shared container runtime (the docker
// daemon), which every process sees. The daemon is the cross-process artifact
// store; no in-process handoff is needed.
//
// Each component is built by `oci.BuildPackage` — the SAME mechanism behind the
// `pulumi package build` command. So InstallDependencies is just a *caller* of the
// build phase, not a hand-roller of it: `pulumi install` triggers the build, the
// build itself is shared and lives once. The component *describes itself* in its own
// PulumiPlugin.yaml (runtime: oci + options.{name, version, build}); the program only
// names the local component *paths* in its `components` runtime option (relative to the
// program directory). This retires the earlier throwaway components schema that carried
// name/version/build in the *consumer's* Pulumi.yaml.
func (h *ociHost) InstallDependencies(
	req *pulumirpc.InstallDependenciesRequest,
	stream pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	dir := req.GetInfo().GetProgramDirectory()
	if dir == "" {
		dir = req.GetDirectory()
	}
	components := req.GetInfo().GetOptions().GetFields()["components"].GetListValue().GetValues()
	registry := os.Getenv("PULUMI_POD_PLUGIN_REGISTRY")

	out := &installStreamWriter{stream: stream}
	// Log the parsed count so a silent options round-trip failure (the `components`
	// list dropped between Pulumi.yaml and here) is distinguishable from a build
	// failure — the two look identical downstream (the container host can't find
	// the image either way).
	fmt.Fprintf(out, "oci: %d local component(s) to build\n", len(components))

	for _, v := range components {
		path := v.GetStringValue()
		if path == "" {
			return fmt.Errorf("oci: each component entry must be a path string (to a self-describing package directory)")
		}
		cdir := path
		if !filepath.IsAbs(cdir) {
			cdir = filepath.Join(dir, path)
		}
		// Build via the shared package-build mechanism: it reads the component's own
		// PulumiPlugin.yaml and tags the image by convention (so the container host
		// resolves it at Construct time). The returned ref is not needed here — the
		// image lands in the shared daemon under that tag.
		if _, err := oci.BuildPackage(stream.Context(), cdir, registry, out); err != nil {
			return fmt.Errorf("oci: building local component %s: %w", path, err)
		}
		fmt.Fprintf(out, "oci: built local component %s\n", path)
	}
	return nil
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

	if podMode {
		image, err := resolveProgramImage(ctx, opts, req.GetInfo().GetProgramDirectory())
		if err != nil {
			return nil, err
		}
		return runProgramContainer(ctx, image, env)
	}

	// Subprocess mode: exec the program binary directly. This is the fast
	// inner-loop dev path — no image build or container start — not the spine;
	// pod mode is the normal form.
	program := optString(opts, "program")
	if program == "" {
		return nil, errors.New("oci: runtime option 'program' is required for subprocess mode")
	}
	if !filepath.IsAbs(program) {
		program = filepath.Join(req.GetInfo().GetProgramDirectory(), program)
	}
	cmd := exec.CommandContext(ctx, program)
	cmd.Env = append(os.Environ(), envSlice(env)...)
	// The program's output goes to stderr; stdout is reserved for the language
	// host's port-line protocol with the engine.
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// The program ran and exited non-zero; its own output already explained
			// why. Bail so the engine halts without double-reporting.
			return &pulumirpc.RunResponse{Bail: true}, nil
		}
		return nil, fmt.Errorf("oci: starting program: %w", err)
	}
	return &pulumirpc.RunResponse{}, nil
}

// runProgramContainer runs the program image as a pod container through the
// PodManager — the same runtime abstraction the container host uses to start
// providers — rather than shelling out to `docker` directly. It streams the
// container's output to stderr (stdout is reserved for the port-line protocol),
// blocks until the program exits, and maps a non-zero exit to a Bail.
func runProgramContainer(ctx context.Context, image string, env map[string]string) (*pulumirpc.RunResponse, error) {
	podID := os.Getenv("PULUMI_POD_ID")
	if podID == "" {
		// Mirror the container host's fallback: without an explicit pod id, derive
		// one from this (engine) container's hostname so the container is still
		// labelled for cleanup.
		podID, _ = os.Hostname()
	}
	pod := oci.NewDockerPodManager(podID)

	// Shared workspace volume: the program writes its runtime workspace here (build
	// outputs an asset-consuming provider must read, e.g. cloudflare WorkerVersion's
	// asset files), and providers mount the same volume at the same path. Creating it
	// (vs. mounting a bare name) labels it for pod cleanup; Docker seeds the empty volume
	// from the program image's workspace on first mount, so the program runs with its
	// baked source/deps and builds into the shared volume providers then read.
	wsVol, err := pod.CreateVolume(ctx, oci.WorkspaceVolumeLogical)
	if err != nil {
		return nil, fmt.Errorf("oci: creating shared workspace volume: %w", err)
	}

	network := os.Getenv("PULUMI_POD_NETWORK")
	cfg := oci.ContainerConfig{
		Image:   image,
		Name:    "program",
		Network: network,
		Env:     env,
		// Engine-on-host mode (Option A) has no pod network; the program reaches
		// the host engine through the host-gateway alias. On the pod network
		// (Option C) it reaches the engine by container DNS and needs no gateway.
		HostGateway: network == "",
		Volumes:     []oci.VolumeMount{{Source: wsVol.Name, Target: oci.WorkspaceMountPath}},
	}

	c, err := pod.RunContainer(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("oci: starting program container: %w", err)
	}
	// The program container is detached (no --rm); remove it when Run returns. The
	// pod label is a crash backstop. WithoutCancel so a cancelled ctx still cleans up.
	defer func() { _ = pod.StopContainer(context.WithoutCancel(ctx), c) }()

	// Follow the container's combined output onto our stderr. `docker logs -f`
	// replays from the start, so output emitted before this attaches is not lost.
	logs, err := pod.ContainerLogs(ctx, c, true)
	if err != nil {
		return nil, fmt.Errorf("oci: streaming program logs: %w", err)
	}
	copied := make(chan struct{})
	go func() {
		defer close(copied)
		_, _ = io.Copy(os.Stderr, logs)
	}()

	code, waitErr := pod.WaitContainer(ctx, c)
	_ = logs.Close()
	<-copied
	if waitErr != nil {
		return nil, fmt.Errorf("oci: waiting for program container: %w", waitErr)
	}
	if code != 0 {
		// The program ran and exited non-zero; its own output already explained
		// why. Bail so the engine halts without double-reporting.
		return &pulumirpc.RunResponse{Bail: true}, nil
	}
	return &pulumirpc.RunResponse{}, nil
}

// resolveProgramImage determines the program image to run in pod mode. The
// `build` runtime option takes the shape:
//
//   - a struct {image, command, …}: run the command in a dedicated *builder
//     container* whose image supplies the build toolchain (the build/run seam —
//     the build no longer borrows the engine container's rootfs, so a build needing
//     nix/bazel/buildpacks works as long as the builder image carries it).
//
// Otherwise a prebuilt `image` option is used.
func resolveProgramImage(ctx context.Context, opts *structpb.Struct, dir string) (string, error) {
	if spec := opts.GetFields()["build"].GetStructValue(); spec != nil {
		return buildProgramImageInContainer(ctx, spec, dir)
	}
	if image := optString(opts, "image"); image != "" {
		return image, nil
	}
	return "", errors.New(
		"oci: no program image — set runtime option 'build' ({image, command}) " +
			"or 'image' (a prebuilt one)")
}

// buildProgramImageInContainer builds the program image in a builder container and
// returns the ref the build prints to stdout.
func buildProgramImageInContainer(ctx context.Context, spec *structpb.Struct, dir string) (string, error) {
	image := optString(spec, "image")
	command := optString(spec, "command")
	if image == "" || command == "" {
		return "", fmt.Errorf("oci: build needs 'image' and 'command' (got image=%q command=%q)", image, command)
	}
	fmt.Fprintf(os.Stderr, "oci: building program image in builder %s: %s\n", image, command)
	stdout, err := oci.BuildInContainer(ctx, image, command, dir, optStringList(spec, "caches"), nil, os.Stderr)
	if err != nil {
		return "", fmt.Errorf("oci: builder %q failed: %w", image, err)
	}
	ref := lastLine(stdout)
	if ref == "" {
		return "", fmt.Errorf("oci: builder %q produced no image ref on stdout", image)
	}
	fmt.Fprintf(os.Stderr, "oci: built program image %s\n", ref)
	return ref, nil
}

// lastLine returns the last non-empty, trimmed line of s — the image ref, even if
// the build command emitted other chatter on stdout before it.
func lastLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if t := strings.TrimSpace(lines[i]); t != "" {
			return t
		}
	}
	return ""
}

// installStreamWriter forwards bytes to the InstallDependencies response stream as
// stderr chunks, so build progress reaches the engine as it happens. Using one
// instance for a command's Stdout and Stderr is safe: os/exec guarantees at most
// one goroutine writes to a shared (==) writer at a time.
type installStreamWriter struct {
	stream pulumirpc.LanguageRuntime_InstallDependenciesServer
}

func (w *installStreamWriter) Write(p []byte) (int, error) {
	// Copy p: the gRPC layer may retain the message past this call.
	if err := w.stream.Send(&pulumirpc.InstallDependenciesResponse{
		Stderr: append([]byte(nil), p...),
	}); err != nil {
		return 0, err
	}
	return len(p), nil
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

// optStringList reads a list-of-strings option, skipping non-string/empty entries.
// nil-safe: a missing key yields nil.
func optStringList(s *structpb.Struct, key string) []string {
	if s == nil {
		return nil
	}
	vals := s.GetFields()[key].GetListValue().GetValues()
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if str := v.GetStringValue(); str != "" {
			out = append(out, str)
		}
	}
	return out
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
