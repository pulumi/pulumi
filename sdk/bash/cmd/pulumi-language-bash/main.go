// Copyright 2024-2025, Pulumi Corporation.
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

// pulumi-language-bash serves as the "language host" for Pulumi programs written in Bash.
// It is ultimately responsible for spawning the language runtime that executes the program.
//
// The program being executed is executed by a shim script called `run.sh`. This script
// is written in Bash and is responsible for initiating RPC links to the resource monitor
// and engine via the bridge subcommand.
//
// It's therefore the responsibility of this program to implement the LanguageHostServer
// endpoint by spawning instances of `bash run.sh` and forwarding the RPC request arguments
// to environment variables.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	codegen "github.com/pulumi/pulumi/pkg/v3/codegen/bash"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	hclsyntax "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
)

const (
	pulumiConfigVar           = "PULUMI_CONFIG"
	pulumiConfigSecretKeysVar = "PULUMI_CONFIG_SECRET_KEYS" //nolint:gosec

	bashProcessExitedAfterShowingUserActionableMessage = 32
)

func main() {
	var tracing string
	flag.StringVar(&tracing, "tracing", "", "Emit tracing to a Zipkin-compatible tracing endpoint")
	showVersion := flag.Bool("version", false, "Print the current plugin version and exit")

	flag.Parse()

	if *showVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	args := flag.Args()
	logging.InitLogging(false, 0, false)
	cmdutil.InitTracing("pulumi-language-bash", "pulumi-language-bash", tracing)

	// Check if the "bridge" subcommand is being invoked.
	if len(args) > 0 && args[0] == "bridge" {
		if err := runBridge(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Optionally pluck out the engine so we can do logging, etc.
	var engineAddress string
	if len(args) > 0 {
		engineAddress = args[0]
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	cancelChannel := make(chan bool)
	go func() {
		<-ctx.Done()
		cancel()
		close(cancelChannel)
	}()

	if engineAddress != "" {
		err := rpcutil.Healthcheck(ctx, engineAddress, 5*time.Minute, cancel)
		if err != nil {
			cmdutil.Exit(fmt.Errorf("could not start health check host RPC server: %w", err))
		}
	}

	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancelChannel,
		Init: func(srv *grpc.Server) error {
			host := newLanguageHost(engineAddress, tracing)
			pulumirpc.RegisterLanguageRuntimeServer(srv, host)
			return nil
		},
		Options: rpcutil.OpenTracingServerInterceptorOptions(nil),
	})
	if err != nil {
		cmdutil.Exit(fmt.Errorf("could not start language host RPC server: %w", err))
	}

	fmt.Printf("%d\n", handle.Port)

	if err := <-handle.Done; err != nil {
		cmdutil.Exit(fmt.Errorf("language host RPC stopped serving: %w", err))
	}
}

// bashLanguageHost implements the LanguageRuntimeServer interface.
type bashLanguageHost struct {
	pulumirpc.UnsafeLanguageRuntimeServer

	engineAddress string
	tracing       string
}

func newLanguageHost(engineAddress, tracing string) pulumirpc.LanguageRuntimeServer {
	return &bashLanguageHost{
		engineAddress: engineAddress,
		tracing:       tracing,
	}
}

// findExecutorScript locates the run.sh executor script.
func findExecutorScript() (string, error) {
	if sdkPath := os.Getenv("PULUMI_BASH_SDK_PATH"); sdkPath != "" {
		script := filepath.Join(sdkPath, "lib", "pulumi", "cmd", "run.sh")
		if _, err := os.Stat(script); err == nil {
			return script, nil
		}
	}

	thisPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not determine current executable: %w", err)
	}

	binDir := filepath.Dir(thisPath)

	// Try: <binDir>/../lib/pulumi/cmd/run.sh (installed layout)
	script := filepath.Join(binDir, "..", "lib", "pulumi", "cmd", "run.sh")
	if _, err := os.Stat(script); err == nil {
		abs, err := filepath.Abs(script)
		if err != nil {
			return script, nil
		}
		return abs, nil
	}

	// Try: relative to the repo structure
	script = filepath.Join(binDir, "..", "..", "lib", "pulumi", "cmd", "run.sh")
	if _, err := os.Stat(script); err == nil {
		abs, err := filepath.Abs(script)
		if err != nil {
			return script, nil
		}
		return abs, nil
	}

	return "", fmt.Errorf("could not find run.sh executor script; "+
		"set PULUMI_BASH_SDK_PATH or ensure the script is installed relative to the language host binary at %s", binDir)
}

// findBridgeCommand returns the path to the bridge command.
// If PULUMI_BASH_BRIDGE_CMD is set, it is used directly.
// Otherwise, the current executable is used (it handles the "bridge" subcommand).
func findBridgeCommand() (string, error) {
	if cmd := os.Getenv("PULUMI_BASH_BRIDGE_CMD"); cmd != "" {
		return cmd, nil
	}
	return os.Executable()
}

func (host *bashLanguageHost) Handshake(ctx context.Context,
	req *pulumirpc.LanguageHandshakeRequest,
) (*pulumirpc.LanguageHandshakeResponse, error) {
	host.engineAddress = req.EngineAddress

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	cancelChannel := make(chan bool)
	go func() {
		<-ctx.Done()
		cancel()
		close(cancelChannel)
	}()
	err := rpcutil.Healthcheck(ctx, host.engineAddress, 5*time.Minute, cancel)
	if err != nil {
		cmdutil.Exit(fmt.Errorf("could not start health check host RPC server: %w", err))
	}

	return &pulumirpc.LanguageHandshakeResponse{}, nil
}

func (host *bashLanguageHost) GetRequiredPlugins(ctx context.Context,
	req *pulumirpc.GetRequiredPluginsRequest,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetRequiredPlugins not implemented")
}

func (host *bashLanguageHost) GetRequiredPackages(ctx context.Context,
	req *pulumirpc.GetRequiredPackagesRequest,
) (*pulumirpc.GetRequiredPackagesResponse, error) {
	programDir := req.Info.ProgramDirectory
	if programDir == "" {
		programDir = req.Info.RootDirectory
	}

	vendorDir := filepath.Join(programDir, "vendor")
	entries, err := os.ReadDir(vendorDir)
	if err != nil {
		if os.IsNotExist(err) {
			return &pulumirpc.GetRequiredPackagesResponse{Packages: []*pulumirpc.PackageDependency{}}, nil
		}
		return nil, fmt.Errorf("read vendor directory: %w", err)
	}

	var packages []*pulumirpc.PackageDependency
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if !strings.HasPrefix(entry.Name(), "pulumi_") {
			continue
		}

		pluginJSONPath := filepath.Join(vendorDir, entry.Name(), "pulumi-plugin.json")
		pluginJSON, err := plugin.LoadPulumiPluginJSON(pluginJSONPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			logging.V(5).Infof("GetRequiredPackages: error reading %s: %v", pluginJSONPath, err)
			continue
		}

		if !pluginJSON.Resource {
			continue
		}

		name := pluginJSON.Name
		if name == "" {
			name = strings.ReplaceAll(strings.TrimPrefix(entry.Name(), "pulumi_"), "_", "-")
		}

		if name == "" || name == "pulumi" {
			continue
		}

		dep := &pulumirpc.PackageDependency{
			Name:    name,
			Kind:    "resource",
			Version: pluginJSON.Version,
		}

		if pluginJSON.Server != "" {
			dep.Server = pluginJSON.Server
		}

		if pluginJSON.Parameterization != nil {
			dep.Parameterization = &pulumirpc.PackageParameterization{
				Name:    pluginJSON.Parameterization.Name,
				Version: pluginJSON.Parameterization.Version,
				Value:   pluginJSON.Parameterization.Value,
			}
		}

		packages = append(packages, dep)
	}

	return &pulumirpc.GetRequiredPackagesResponse{Packages: packages}, nil
}

func (host *bashLanguageHost) constructConfig(req *pulumirpc.RunRequest) (string, error) {
	configMap := req.GetConfig()
	if configMap == nil {
		return "", nil
	}

	configJSON, err := json.Marshal(configMap)
	if err != nil {
		return "", err
	}

	return string(configJSON), nil
}

func (host *bashLanguageHost) constructConfigSecretKeys(req *pulumirpc.RunRequest) (string, error) {
	configSecretKeys := req.GetConfigSecretKeys()
	if configSecretKeys == nil {
		return "[]", nil
	}

	configSecretKeysJSON, err := json.Marshal(configSecretKeys)
	if err != nil {
		return "", err
	}

	return string(configSecretKeysJSON), nil
}

func (host *bashLanguageHost) Run(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	runScript, err := findExecutorScript()
	if err != nil {
		return nil, err
	}

	bridgeCmd, err := findBridgeCommand()
	if err != nil {
		return nil, fmt.Errorf("could not determine bridge command: %w", err)
	}

	config, err := host.constructConfig(req)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize configuration: %w", err)
	}
	configSecretKeys, err := host.constructConfigSecretKeys(req)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize configuration secret keys: %w", err)
	}

	entryPoint := req.Info.EntryPoint
	if entryPoint == "" {
		entryPoint = "."
	}

	args := make([]string, 0, 1+len(req.GetArgs()))
	args = append(args, runScript)
	args = append(args, req.GetArgs()...)

	if logging.V(5) {
		commandStr := strings.Join(args, " ")
		logging.V(5).Infoln("Language host launching process: bash", commandStr)
	}

	cmd := exec.CommandContext(ctx, "bash", args...)
	cmd.Dir = req.Info.ProgramDirectory
	cmd.Stdout = struct{ io.Writer }{os.Stdout}
	cmd.Stderr = struct{ io.Writer }{os.Stderr}

	// Determine the SDK path.
	sdkPath := os.Getenv("PULUMI_BASH_SDK_PATH")
	if sdkPath == "" {
		// Derive from the executor script path.
		sdkPath = filepath.Dir(filepath.Dir(filepath.Dir(runScript)))
	}

	env := os.Environ()
	env = append(env, "PULUMI_MONITOR_ADDRESS="+req.GetMonitorAddress())
	env = append(env, "PULUMI_ENGINE_ADDRESS="+host.engineAddress)
	env = append(env, "PULUMI_PROJECT="+req.GetProject())
	env = append(env, "PULUMI_STACK="+req.GetStack())
	env = append(env, fmt.Sprintf("PULUMI_DRY_RUN=%v", req.GetDryRun()))
	env = append(env, fmt.Sprintf("PULUMI_PARALLEL=%d", req.GetParallel()))
	env = append(env, "PULUMI_ORGANIZATION="+req.GetOrganization())
	env = append(env, "PULUMI_BASH_BRIDGE_CMD="+bridgeCmd)
	env = append(env, "PULUMI_BASH_SDK_PATH="+sdkPath)
	env = append(env, "PULUMI_BASH_PROGRAM_DIRECTORY="+req.Info.ProgramDirectory)
	env = append(env, "PULUMI_BASH_ROOT_DIRECTORY="+req.Info.RootDirectory)
	env = append(env, "PULUMI_BASH_ENTRY_POINT="+entryPoint)
	if config != "" {
		env = append(env, pulumiConfigVar+"="+config)
	}
	if configSecretKeys != "" {
		env = append(env, pulumiConfigSecretKeysVar+"="+configSecretKeys)
	}
	if host.tracing != "" {
		env = append(env, "PULUMI_TRACING="+host.tracing)
	}
	cmd.Env = env

	var errResult string
	if err := cmd.Run(); err != nil {
		contract.IgnoreError(os.Stdout.Sync())
		contract.IgnoreError(os.Stderr.Sync())

		if exiterr, ok := err.(*exec.ExitError); ok {
			if ws, stok := exiterr.Sys().(syscall.WaitStatus); stok {
				switch ws.ExitStatus() {
				case 0:
					err = fmt.Errorf("program exited unexpectedly: %w", exiterr)
				case bashProcessExitedAfterShowingUserActionableMessage:
					return &pulumirpc.RunResponse{Error: "", Bail: true}, nil
				default:
					err = fmt.Errorf("program exited with non-zero exit code: %d", ws.ExitStatus())
				}
			} else {
				err = fmt.Errorf("program exited unexpectedly: %w", exiterr)
			}
		} else {
			err = fmt.Errorf("problem executing program (could not run language executor): %w", err)
		}

		errResult = err.Error()
	}

	return &pulumirpc.RunResponse{Error: errResult}, nil
}

func (host *bashLanguageHost) GetPluginInfo(ctx context.Context, req *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: version.Version,
	}, nil
}

func (host *bashLanguageHost) InstallDependencies(
	req *pulumirpc.InstallDependenciesRequest, server pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	closer, stdout, stderr, err := rpcutil.MakeInstallDependenciesStreams(server, req.IsTerminal)
	if err != nil {
		return err
	}
	defer closer.Close()

	stdout.Write([]byte("Installing dependencies...\n\n"))

	requirementsPath := filepath.Join(req.Info.ProgramDirectory, "requirements.txt")
	if _, err := os.Stat(requirementsPath); err == nil {
		data, err := os.ReadFile(requirementsPath)
		if err != nil {
			return fmt.Errorf("read requirements.txt: %w", err)
		}

		vendorDir := filepath.Join(req.Info.ProgramDirectory, "vendor")

		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			if strings.HasSuffix(line, ".tar.gz") {
				// Extract tar.gz package to vendor directory.
				if err := os.MkdirAll(vendorDir, 0o755); err != nil {
					return fmt.Errorf("create vendor directory: %w", err)
				}

				cmd := exec.CommandContext(server.Context(), "tar", "xzf", line, "-C", vendorDir)
				cmd.Dir = req.Info.ProgramDirectory
				cmd.Stdout = stdout
				cmd.Stderr = stderr

				if err := cmd.Run(); err != nil {
					return fmt.Errorf("extract %s failed: %w", line, err)
				}

				fmt.Fprintf(stdout, "Extracted %s\n", filepath.Base(line))
			}
		}
	}

	stdout.Write([]byte("Finished installing dependencies\n\n"))

	return closer.Close()
}

func (host *bashLanguageHost) RuntimeOptionsPrompts(ctx context.Context,
	req *pulumirpc.RuntimeOptionsRequest,
) (*pulumirpc.RuntimeOptionsResponse, error) {
	return &pulumirpc.RuntimeOptionsResponse{
		Prompts: nil,
	}, nil
}

func (host *bashLanguageHost) Template(
	ctx context.Context, req *pulumirpc.TemplateRequest,
) (*pulumirpc.TemplateResponse, error) {
	return &pulumirpc.TemplateResponse{}, nil
}

func (host *bashLanguageHost) About(ctx context.Context,
	req *pulumirpc.AboutRequest,
) (*pulumirpc.AboutResponse, error) {
	bashExec, err := exec.LookPath("bash")
	if err != nil {
		return nil, fmt.Errorf("could not find bash executable: %w", err)
	}

	cmd := exec.CommandContext(ctx, bashExec, "--version")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to get bash version: %w", err)
	}

	bashVersion := strings.TrimSpace(strings.Split(out.String(), "\n")[0])

	return &pulumirpc.AboutResponse{
		Executable: bashExec,
		Version:    bashVersion,
	}, nil
}

func (host *bashLanguageHost) GetProgramDependencies(
	ctx context.Context, req *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	requirementsPath := filepath.Join(req.Info.ProgramDirectory, "requirements.txt")
	data, err := os.ReadFile(requirementsPath)
	if err != nil {
		return &pulumirpc.GetProgramDependenciesResponse{
			Dependencies: nil,
		}, nil
	}

	var deps []*pulumirpc.DependencyInfo
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		name := line
		ver := ""

		if strings.HasSuffix(line, ".tar.gz") {
			base := filepath.Base(line)
			base = strings.TrimSuffix(base, ".tar.gz")
			if idx := strings.Index(base, "-"); idx > 0 {
				name = base[:idx]
				ver = base[idx+1:]
			} else {
				name = base
			}
		}

		deps = append(deps, &pulumirpc.DependencyInfo{
			Name:    name,
			Version: ver,
		})
	}

	return &pulumirpc.GetProgramDependenciesResponse{
		Dependencies: deps,
	}, nil
}

func (host *bashLanguageHost) RunPlugin(
	req *pulumirpc.RunPluginRequest, server pulumirpc.LanguageRuntime_RunPluginServer,
) error {
	logging.V(5).Infof("Attempting to run bash plugin in %s with args %v", req.Info.ProgramDirectory, req.Args)
	ctx := server.Context()

	programPath := req.Info.ProgramDirectory
	if info, statErr := os.Stat(programPath); statErr == nil && info.IsDir() {
		programPath = filepath.Join(programPath, "__main__.sh")
	}

	args := make([]string, 0, 1+len(req.Args))
	args = append(args, programPath)
	args = append(args, req.Args...)

	cmd := exec.CommandContext(ctx, "bash", args...)

	closer, stdout, stderr, err := rpcutil.MakeRunPluginStreams(server, false)
	if err != nil {
		return err
	}
	defer closer.Close()

	cmd.Dir = req.Pwd
	env := os.Environ()
	env = append(env, req.Env...)
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = stdout, stderr

	if err := cmd.Run(); err != nil {
		var exiterr *exec.ExitError
		if errors.As(err, &exiterr) {
			if ws, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				return server.Send(&pulumirpc.RunPluginResponse{
					//nolint:gosec
					Output: &pulumirpc.RunPluginResponse_Exitcode{Exitcode: int32(ws.ExitStatus())},
				})
			}
			if len(exiterr.Stderr) > 0 {
				return fmt.Errorf("program exited unexpectedly: %w: %s", exiterr, exiterr.Stderr)
			}
			return fmt.Errorf("program exited unexpectedly: %w", exiterr)
		}
		return fmt.Errorf("problem executing plugin program (could not run language executor): %w", err)
	}

	return closer.Close()
}

func (host *bashLanguageHost) GenerateProject(
	ctx context.Context, req *pulumirpc.GenerateProjectRequest,
) (*pulumirpc.GenerateProjectResponse, error) {
	loader, err := schema.NewLoaderClient(req.LoaderTarget)
	if err != nil {
		return nil, err
	}
	defer loader.Close()

	extraOptions := []pcl.BindOption{pcl.PreferOutputVersionedInvokes}
	if !req.Strict {
		extraOptions = append(extraOptions, pcl.NonStrictBindOptions()...)
	}

	program, diags, err := pcl.BindDirectory(req.SourceDirectory, schema.NewCachedLoader(loader), extraOptions...)
	if err != nil {
		return nil, err
	}
	if diags.HasErrors() {
		rpcDiagnostics := plugin.HclDiagnosticsToRPCDiagnostics(diags)
		return &pulumirpc.GenerateProjectResponse{
			Diagnostics: rpcDiagnostics,
		}, nil
	}

	var project workspace.Project
	if err := json.Unmarshal([]byte(req.Project), &project); err != nil {
		return nil, err
	}

	err = codegen.GenerateProject(
		req.TargetDirectory, project, program, req.LocalDependencies)
	if err != nil {
		logging.V(3).Infof("GenerateProject failed: %v", err)
		return nil, err
	}

	rpcDiagnostics := plugin.HclDiagnosticsToRPCDiagnostics(diags)
	return &pulumirpc.GenerateProjectResponse{
		Diagnostics: rpcDiagnostics,
	}, nil
}

func (host *bashLanguageHost) GenerateProgram(
	ctx context.Context, req *pulumirpc.GenerateProgramRequest,
) (*pulumirpc.GenerateProgramResponse, error) {
	loader, err := schema.NewLoaderClient(req.LoaderTarget)
	if err != nil {
		return nil, err
	}
	defer loader.Close()

	parser := hclsyntax.NewParser()
	for path, contents := range req.Source {
		err = parser.ParseFile(strings.NewReader(contents), path)
		if err != nil {
			return nil, err
		}
		diags := parser.Diagnostics
		if diags.HasErrors() {
			return nil, diags
		}
	}

	bindOptions := []pcl.BindOption{
		pcl.Loader(schema.NewCachedLoader(loader)),
		pcl.PreferOutputVersionedInvokes,
	}

	if !req.Strict {
		bindOptions = append(bindOptions, pcl.NonStrictBindOptions()...)
	}

	program, diags, err := pcl.BindProgram(parser.Files, bindOptions...)
	if err != nil {
		return nil, err
	}

	rpcDiagnostics := plugin.HclDiagnosticsToRPCDiagnostics(diags)
	if diags.HasErrors() {
		return &pulumirpc.GenerateProgramResponse{
			Diagnostics: rpcDiagnostics,
		}, nil
	}
	if program == nil {
		return nil, errors.New("internal error program was nil")
	}

	files, progDiags, err := codegen.GenerateProgram(program)
	if err != nil {
		logging.V(3).Infof("GenerateProgram failed: %v", err)
		return nil, err
	}
	rpcDiagnostics = append(rpcDiagnostics, plugin.HclDiagnosticsToRPCDiagnostics(progDiags)...)

	return &pulumirpc.GenerateProgramResponse{
		Source:      files,
		Diagnostics: rpcDiagnostics,
	}, nil
}

func (host *bashLanguageHost) GeneratePackage(
	ctx context.Context, req *pulumirpc.GeneratePackageRequest,
) (*pulumirpc.GeneratePackageResponse, error) {
	loader, err := schema.NewLoaderClient(req.LoaderTarget)
	if err != nil {
		return nil, err
	}
	defer loader.Close()

	var spec schema.PackageSpec
	err = json.Unmarshal([]byte(req.Schema), &spec)
	if err != nil {
		return nil, err
	}

	pkg, diags, err := schema.BindSpec(spec, schema.NewCachedLoader(loader), schema.ValidationOptions{
		AllowDanglingReferences: true,
	})
	if err != nil {
		return nil, err
	}
	rpcDiagnostics := plugin.HclDiagnosticsToRPCDiagnostics(diags)
	if diags.HasErrors() {
		return &pulumirpc.GeneratePackageResponse{
			Diagnostics: rpcDiagnostics,
		}, nil
	}

	files, err := codegen.GeneratePackage("pulumi-language-bash", pkg, req.ExtraFiles, schema.NewCachedLoader(loader))
	if err != nil {
		logging.V(3).Infof("GeneratePackage failed: %v", err)
		return nil, err
	}

	for filename, data := range files {
		outPath := filepath.Join(req.Directory, filename)

		err := os.MkdirAll(filepath.Dir(outPath), 0o700)
		if err != nil {
			return nil, fmt.Errorf("could not create output directory %s: %w", filepath.Dir(filename), err)
		}

		err = os.WriteFile(outPath, data, 0o600)
		if err != nil {
			return nil, fmt.Errorf("could not write output file %s: %w", filename, err)
		}
	}

	return &pulumirpc.GeneratePackageResponse{
		Diagnostics: rpcDiagnostics,
	}, nil
}

func (host *bashLanguageHost) Pack(ctx context.Context, req *pulumirpc.PackRequest) (*pulumirpc.PackResponse, error) {
	entries, err := os.ReadDir(req.PackageDirectory)
	if err != nil {
		return nil, fmt.Errorf("read package directory: %w", err)
	}

	// Determine the package name and version.
	// First, try to find a pulumi-plugin.json in a pulumi_* subdirectory (generated package).
	// If not found, try the root. If still not found, this is a core SDK package.
	var name, ver string

	var pkgDirName string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "pulumi_") {
			pkgDirName = entry.Name()
			break
		}
	}

	pluginJSONFound := false
	for _, candidate := range []string{
		filepath.Join(req.PackageDirectory, pkgDirName, "pulumi-plugin.json"),
		filepath.Join(req.PackageDirectory, "pulumi-plugin.json"),
	} {
		if candidate == filepath.Join(req.PackageDirectory, "pulumi-plugin.json") && pkgDirName == "" {
			// Only try the root if there's no pulumi_ subdir, otherwise we'd skip.
		}
		data, readErr := os.ReadFile(candidate)
		if readErr != nil {
			continue
		}
		var pluginJSON plugin.PulumiPluginJSON
		if jsonErr := json.Unmarshal(data, &pluginJSON); jsonErr != nil {
			continue
		}
		name = pluginJSON.Name
		ver = pluginJSON.Version
		pluginJSONFound = true
		break
	}

	if !pluginJSONFound {
		// This is a core SDK package (no pulumi-plugin.json).
		// Use "pulumi" as the name and the SDK version.
		name = "pulumi"
		ver = sdk.Version.String()
	}

	if name == "" && pkgDirName != "" {
		name = strings.TrimPrefix(pkgDirName, "pulumi_")
	}
	if name == "" {
		name = "unknown"
	}
	if ver == "" {
		ver = "0.0.0"
	}

	tarName := fmt.Sprintf("pulumi-%s.tar.gz", ver)
	if name != "pulumi" {
		tarName = fmt.Sprintf("pulumi_%s-%s.tar.gz", strings.ReplaceAll(name, "-", "_"), ver)
	}
	dst := filepath.Join(req.DestinationDirectory, tarName)

	if err := os.MkdirAll(req.DestinationDirectory, 0o700); err != nil {
		return nil, fmt.Errorf("create destination directory: %w", err)
	}

	// Create tar.gz of the package directory contents.
	var tarArgs []string
	tarArgs = append(tarArgs, "czf", dst)
	for _, entry := range entries {
		// Skip cmd directory for core SDK packing — it contains the Go binary source.
		if name == "pulumi" && entry.Name() == "cmd" {
			continue
		}
		tarArgs = append(tarArgs, entry.Name())
	}

	cmd := exec.CommandContext(ctx, "tar", tarArgs...)
	cmd.Dir = req.PackageDirectory

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("tar failed: %w\n%s", err, stderr.String())
	}

	return &pulumirpc.PackResponse{
		ArtifactPath: dst,
	}, nil
}

func (host *bashLanguageHost) Link(
	ctx context.Context, req *pulumirpc.LinkRequest,
) (*pulumirpc.LinkResponse, error) {
	vendorDir := filepath.Join(req.Info.ProgramDirectory, "vendor")
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		return nil, fmt.Errorf("create vendor directory: %w", err)
	}

	for _, dep := range req.Packages {
		if dep.Path == "" {
			continue
		}

		// Extract the .tar.gz artifact to the vendor directory.
		cmd := exec.CommandContext(ctx, "tar", "xzf", dep.Path, "-C", vendorDir)
		cmd.Dir = req.Info.ProgramDirectory

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			logging.V(5).Infof("Link tar extract stdout: %s", stdout.String())
			logging.V(5).Infof("Link tar extract stderr: %s", stderr.String())
			return nil, fmt.Errorf("extract %s failed: %w\n%s", dep.Path, err, stderr.String())
		}

		logging.V(5).Infof("Linked package from %s to %s", dep.Path, vendorDir)
	}

	return &pulumirpc.LinkResponse{
		ImportInstructions: "",
	}, nil
}

func (host *bashLanguageHost) Cancel(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
