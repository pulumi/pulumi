// Copyright 2016-2021, Pulumi Corporation.
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
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/blang/semver"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/sdk/v3/go/common/constant"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/buildutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/goversion"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// This function takes a file target to specify where to compile to.
// If `outfile` is "", the binary is compiled to a new temporary file.
// This function returns the path of the file that was produced.
func compileProgram(programDirectory string, outfile string) (string, error) {
	goFileSearchPattern := filepath.Join(programDirectory, "*.go")
	if matches, err := filepath.Glob(goFileSearchPattern); err != nil || len(matches) == 0 {
		return "", fmt.Errorf("Failed to find go files for 'go build' matching %s", goFileSearchPattern)
	}

	if outfile == "" {
		// If no outfile is supplied, write the Go binary to a temporary file.
		f, err := os.CreateTemp("", "pulumi-go.*")
		if err != nil {
			return "", fmt.Errorf("unable to create go program temp file: %w", err)
		}

		if err := f.Close(); err != nil {
			return "", fmt.Errorf("unable to close go program temp file: %w", err)
		}
		outfile = f.Name()
	}

	gobin, err := executable.FindExecutable("go")
	if err != nil {
		return "", fmt.Errorf("unable to find 'go' executable: %w", err)
	}
	logging.V(5).Infof("Attempting to build go program in %s with: %s build -o %s", programDirectory, gobin, outfile)
	buildCmd := exec.Command(gobin, "build", "-o", outfile)
	buildCmd.Dir = programDirectory
	buildCmd.Stdout, buildCmd.Stderr = os.Stdout, os.Stderr

	if err := buildCmd.Run(); err != nil {
		return "", fmt.Errorf("unable to run `go build`: %w", err)
	}

	return outfile, nil
}

// runParams defines the command line arguments accepted by this program.
type runParams struct {
	tracing       string
	binary        string
	buildTarget   string
	root          string
	engineAddress string
}

// parseRunParams parses the given arguments into a runParams structure,
// using the provided FlagSet.
func parseRunParams(flag *flag.FlagSet, args []string) (*runParams, error) {
	var p runParams
	flag.StringVar(&p.tracing, "tracing", "", "Emit tracing to a Zipkin-compatible tracing endpoint")
	flag.StringVar(&p.binary, "binary", "", "Look on path for a binary executable with this name")
	flag.StringVar(&p.buildTarget, "buildTarget", "", "Path to use to output the compiled Pulumi Go program")
	flag.StringVar(&p.root, "root", "", "Project root path to use")

	if err := flag.Parse(args); err != nil {
		return nil, err
	}

	if p.binary != "" && p.buildTarget != "" {
		return nil, errors.New("binary and buildTarget cannot both be specified")
	}

	// Pluck out the engine so we can do logging, etc.
	args = flag.Args()
	if len(args) == 0 {
		return nil, errors.New("missing required engine RPC address argument")
	}
	p.engineAddress = args[0]

	return &p, nil
}

// Launches the language host, which in turn fires up an RPC server implementing the LanguageRuntimeServer endpoint.
func main() {
	p, err := parseRunParams(flag.CommandLine, os.Args[1:])
	if err != nil {
		cmdutil.Exit(err)
	}

	cmd := mainCmd{
		Stdout: os.Stdout,
	}
	if err := cmd.Run(p); err != nil {
		cmdutil.Exit(err)
	}
}

type mainCmd struct {
	Stdout io.Writer // == os.Stdout
}

func (cmd *mainCmd) Run(p *runParams) error {
	ctx, cancel := context.WithCancel(context.Background())
	// map the context Done channel to the rpcutil boolean cancel channel
	cancelChannel := make(chan bool)
	go func() {
		<-ctx.Done()
		close(cancelChannel)
	}()
	err := rpcutil.Healthcheck(ctx, p.engineAddress, 5*time.Minute, cancel)
	if err != nil {
		return fmt.Errorf("could not start health check host RPC server: %w", err)
	}

	// Fire up a gRPC server, letting the kernel choose a free port.
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancelChannel,
		Init: func(srv *grpc.Server) error {
			host := newLanguageHost(p.engineAddress, p.tracing, p.binary, p.buildTarget)
			pulumirpc.RegisterLanguageRuntimeServer(srv, host)
			return nil
		},
		Options: rpcutil.OpenTracingServerInterceptorOptions(nil),
	})
	if err != nil {
		return fmt.Errorf("could not start language host RPC server: %w", err)
	}

	// Otherwise, print out the port so that the spawner knows how to reach us.
	fmt.Fprintf(cmd.Stdout, "%d\n", handle.Port)

	// And finally wait for the server to stop serving.
	if err := <-handle.Done; err != nil {
		return fmt.Errorf("language host RPC stopped serving: %w", err)
	}

	return nil
}

// goLanguageHost implements the LanguageRuntimeServer interface for use as an API endpoint.
type goLanguageHost struct {
	pulumirpc.UnsafeLanguageRuntimeServer // opt out of forward compat

	engineAddress string
	tracing       string
	binary        string
	buildTarget   string
}

func newLanguageHost(engineAddress, tracing, binary, buildTarget string) pulumirpc.LanguageRuntimeServer {
	return &goLanguageHost{
		engineAddress: engineAddress,
		tracing:       tracing,
		binary:        binary,
		buildTarget:   buildTarget,
	}
}

// modInfo is the useful portion of the output from `go list -m -json all`
// with respect to plugin acquisition
type modInfo struct {
	Path    string
	Version string
	Dir     string
}

// Returns the pulumi-plugin.json if found. If not found, then returns nil, nil.
// The lookup path for pulumi-plugin.json is
// 1. m.Dir
// 2. m.Dir/go
// 3. m.Dir/go/*
func (m *modInfo) readPulumiPluginJSON() (*plugin.PulumiPluginJSON, error) {
	if m.Dir == "" {
		return nil, nil
	}

	path1 := filepath.Join(m.Dir, "pulumi-plugin.json")
	path2 := filepath.Join(m.Dir, "go", "pulumi-plugin.json")
	path3, err := filepath.Glob(filepath.Join(m.Dir, "go", "*", "pulumi-plugin.json"))
	if err != nil {
		path3 = []string{}
	}
	paths := append([]string{path1, path2}, path3...)

	for _, path := range paths {
		plugin, err := plugin.LoadPulumiPluginJSON(path)
		switch {
		case os.IsNotExist(err):
			continue
		case err != nil:
			return nil, err
		default:
			return plugin, nil
		}
	}
	return nil, nil
}

func normalizeVersion(version string) (string, error) {
	v, err := semver.ParseTolerant(version)
	if err != nil {
		return "", errors.New("module does not have semver compatible version")
	}

	// psuedoversions are commits that don't have a corresponding tag at the specified git hash
	// https://golang.org/cmd/go/#hdr-Pseudo_versions
	// pulumi-aws v1.29.1-0.20200403140640-efb5e2a48a86 (first commit after 1.29.0 release)
	if buildutil.IsPseudoVersion(version) {
		// no prior tag means there was never a release build
		if v.Major == 0 && v.Minor == 0 && v.Patch == 0 {
			return "", errors.New("invalid pseduoversion with no prior tag")
		}
		// patch is typically bumped from the previous tag when using pseudo version
		// downgrade the patch by 1 to make sure we match a release that exists
		patch := v.Patch
		if patch > 0 {
			patch--
		}
		version = fmt.Sprintf("v%v.%v.%v", v.Major, v.Minor, patch)
	}
	return version, nil
}

func (m *modInfo) getPlugin() (*pulumirpc.PluginDependency, error) {
	pulumiPlugin, err := m.readPulumiPluginJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to load pulumi-plugin.json: %w", err)
	}

	if (!strings.HasPrefix(m.Path, "github.com/pulumi/pulumi-") && pulumiPlugin == nil) ||
		(pulumiPlugin != nil && !pulumiPlugin.Resource) {
		return nil, errors.New("module is not a pulumi provider")
	}

	var name string
	if pulumiPlugin != nil && pulumiPlugin.Name != "" {
		name = pulumiPlugin.Name
	} else {
		// github.com/pulumi/pulumi-aws/sdk/... => aws
		pluginPart := strings.Split(m.Path, "/")[2]
		name = strings.SplitN(pluginPart, "-", 2)[1]
	}

	version := m.Version
	if pulumiPlugin != nil && pulumiPlugin.Version != "" {
		version = pulumiPlugin.Version
	}
	version, err = normalizeVersion(version)
	if err != nil {
		return nil, err
	}

	var server string

	if pulumiPlugin != nil {
		// There is no way to specify server without using `pulumi-plugin.json`.
		server = pulumiPlugin.Server
	}

	plugin := &pulumirpc.PluginDependency{
		Name:    name,
		Version: version,
		Kind:    "resource",
		Server:  server,
	}

	return plugin, nil
}

// GetRequiredPlugins computes the complete set of anticipated plugins required by a program.
// We're lenient here as this relies on the `go list` command and the use of modules.
// If the consumer insists on using some other form of dependency management tool like
// dep or glide, the list command fails with "go list -m: not using modules".
// However, we do enforce that go 1.14.0 or higher is installed.
func (host *goLanguageHost) GetRequiredPlugins(ctx context.Context,
	req *pulumirpc.GetRequiredPluginsRequest,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	logging.V(5).Infof("GetRequiredPlugins: Determining pulumi packages")

	gobin, err := executable.FindExecutable("go")
	if err != nil {
		return nil, fmt.Errorf("couldn't find go binary: %w", err)
	}

	if err = goversion.CheckMinimumGoVersion(gobin); err != nil {
		return nil, err
	}

	args := []string{"list", "-m", "-json", "-mod=mod", "all"}

	tracingSpan, _ := opentracing.StartSpanFromContext(ctx,
		fmt.Sprintf("%s %s", gobin, strings.Join(args, " ")),
		opentracing.Tag{Key: "component", Value: "exec.Command"},
		opentracing.Tag{Key: "command", Value: gobin},
		opentracing.Tag{Key: "args", Value: args})

	// don't wire up stderr so non-module users don't see error output from list
	cmd := exec.Command(gobin, args...)
	cmd.Env = os.Environ()
	stdout, err := cmd.Output()

	tracingSpan.Finish()

	if err != nil {
		logging.V(5).Infof("GetRequiredPlugins: Error discovering plugin requirements using go modules: %s", err.Error())
		return &pulumirpc.GetRequiredPluginsResponse{}, nil
	}

	plugins := []*pulumirpc.PluginDependency{}

	dec := json.NewDecoder(bytes.NewReader(stdout))
	for {
		var m modInfo
		if err := dec.Decode(&m); err != nil {
			if err == io.EOF {
				break
			}
			logging.V(5).Infof("GetRequiredPlugins: Error parsing list output: %s", err.Error())
			return &pulumirpc.GetRequiredPluginsResponse{}, nil
		}

		plugin, err := m.getPlugin()
		if err == nil {
			logging.V(5).Infof("GetRequiredPlugins: Found plugin name: %s, version: %s", plugin.Name, plugin.Version)
			plugins = append(plugins, plugin)
		} else {
			logging.V(5).Infof(
				"GetRequiredPlugins: Ignoring dependency: %s, version: %s, error: %s",
				m.Path,
				m.Version,
				err.Error(),
			)
		}
	}

	return &pulumirpc.GetRequiredPluginsResponse{
		Plugins: plugins,
	}, nil
}

func runCmdStatus(cmd *exec.Cmd, env []string) (int, error) {
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr

	err := cmd.Run()
	// The returned error is nil if the command runs, has no problems copying stdin, stdout, and stderr, and
	// exits with a zero exit status.
	if err == nil {
		return 0, nil
	}

	// error handling
	exiterr, ok := err.(*exec.ExitError)
	if !ok {
		return 0, fmt.Errorf("command errored unexpectedly: %w", err)
	}

	// retrieve the status code
	status, ok := exiterr.Sys().(syscall.WaitStatus)
	if !ok {
		return 0, fmt.Errorf("program exited unexpectedly: %w", err)
	}

	return status.ExitStatus(), nil
}

func runProgram(pwd, bin string, env []string) *pulumirpc.RunResponse {
	cmd := exec.Command(bin)
	cmd.Dir = pwd
	status, err := runCmdStatus(cmd, env)
	if err != nil {
		return &pulumirpc.RunResponse{
			Error: err.Error(),
		}
	}

	if status == 0 {
		return &pulumirpc.RunResponse{}
	}

	// If the program ran, but returned an error,
	// the error message should look as nice as possible.
	if status == constant.ExitStatusLoggedError {
		// program failed but sent an error to the engine
		return &pulumirpc.RunResponse{
			Bail: true,
		}
	}

	// If the program ran, but exited with a non-zero and non-reserved error code.
	// indicate to the user which exit code the program returned.
	return &pulumirpc.RunResponse{
		Error: fmt.Sprintf("program exited with non-zero exit code: %d", status),
	}
}

// Run is RPC endpoint for LanguageRuntimeServer::Run
func (host *goLanguageHost) Run(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	// Create the environment we'll use to run the process.  This is how we pass the RunInfo to the actual
	// Go program runtime, to avoid needing any sort of program interface other than just a main entrypoint.
	env, err := host.constructEnv(req)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare environment: %w", err)
	}

	// the user can explicitly opt in to using a binary executable by specifying
	// runtime.options.binary in the Pulumi.yaml
	if host.binary != "" {
		bin, err := executable.FindExecutable(host.binary)
		if err != nil {
			return nil, fmt.Errorf("unable to find '%s' executable: %w", host.binary, err)
		}
		return runProgram(req.Pwd, bin, env), nil
	}

	// feature flag to enable deprecated old behavior and use `go run`
	if os.Getenv("PULUMI_GO_USE_RUN") != "" {
		gobin, err := executable.FindExecutable("go")
		if err != nil {
			return nil, fmt.Errorf("unable to find 'go' executable: %w", err)
		}

		cmd := exec.Command(gobin, "run", req.Program)

		status, err := runCmdStatus(cmd, env)
		if err != nil {
			return &pulumirpc.RunResponse{
				Error: err.Error(),
			}, nil
		}

		// `go run` does not return the actual exit status of a program
		// it only returns 2 non-zero exit statuses {1, 2}
		// and it emits the exit status to stderr
		if status != 0 {
			return &pulumirpc.RunResponse{
				Bail: true,
			}, nil
		}

		return &pulumirpc.RunResponse{}, nil
	}

	// user did not specify a binary and we will compile and run the binary on-demand
	logging.V(5).Infof("No prebuilt executable specified, attempting invocation via compilation")

	program, err := compileProgram(req.Program, host.buildTarget)
	if err != nil {
		return nil, fmt.Errorf("error in compiling Go: %w", err)
	}
	if host.buildTarget == "" {
		// If there is no specified buildTarget, delete the temporary program after running it.
		defer os.Remove(program)
	}

	return runProgram(req.Pwd, program, env), nil
}

// constructEnv constructs an environment for a Go progam by enumerating all of the optional and non-optional
// arguments present in a RunRequest.
func (host *goLanguageHost) constructEnv(req *pulumirpc.RunRequest) ([]string, error) {
	config, err := host.constructConfig(req)
	if err != nil {
		return nil, err
	}
	configSecretKeys, err := host.constructConfigSecretKeys(req)
	if err != nil {
		return nil, err
	}

	env := os.Environ()
	maybeAppendEnv := func(k, v string) {
		if v != "" {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	maybeAppendEnv(pulumi.EnvOrganization, req.GetOrganization())
	maybeAppendEnv(pulumi.EnvProject, req.GetProject())
	maybeAppendEnv(pulumi.EnvStack, req.GetStack())
	maybeAppendEnv(pulumi.EnvConfig, config)
	maybeAppendEnv(pulumi.EnvConfigSecretKeys, configSecretKeys)
	maybeAppendEnv(pulumi.EnvDryRun, fmt.Sprintf("%v", req.GetDryRun()))
	maybeAppendEnv(pulumi.EnvParallel, fmt.Sprint(req.GetParallel()))
	maybeAppendEnv(pulumi.EnvMonitor, req.GetMonitorAddress())
	maybeAppendEnv(pulumi.EnvEngine, host.engineAddress)

	return env, nil
}

// constructConfig JSON-serializes the configuration data given as part of a RunRequest.
func (host *goLanguageHost) constructConfig(req *pulumirpc.RunRequest) (string, error) {
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

// constructConfigSecretKeys JSON-serializes the list of keys that contain secret values given as part of
// a RunRequest.
func (host *goLanguageHost) constructConfigSecretKeys(req *pulumirpc.RunRequest) (string, error) {
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

func (host *goLanguageHost) GetPluginInfo(ctx context.Context, req *pbempty.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: version.Version,
	}, nil
}

func (host *goLanguageHost) InstallDependencies(
	req *pulumirpc.InstallDependenciesRequest, server pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	closer, stdout, stderr, err := rpcutil.MakeInstallDependenciesStreams(server, req.IsTerminal)
	if err != nil {
		return err
	}
	// best effort close, but we try an explicit close and error check at the end as well
	defer closer.Close()

	stdout.Write([]byte("Installing dependencies...\n\n"))

	gobin, err := executable.FindExecutable("go")
	if err != nil {
		return err
	}

	if err = goversion.CheckMinimumGoVersion(gobin); err != nil {
		return err
	}

	cmd := exec.Command(gobin, "mod", "tidy")
	cmd.Dir = req.Directory
	cmd.Env = os.Environ()
	cmd.Stdout, cmd.Stderr = stdout, stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("`go mod tidy` failed to install dependencies: %w", err)
	}

	stdout.Write([]byte("Finished installing dependencies\n\n"))

	return closer.Close()
}

func (host *goLanguageHost) About(ctx context.Context, req *pbempty.Empty) (*pulumirpc.AboutResponse, error) {
	getResponse := func(execString string, args ...string) (string, string, error) {
		ex, err := executable.FindExecutable(execString)
		if err != nil {
			return "", "", fmt.Errorf("could not find executable '%s': %w", execString, err)
		}
		cmd := exec.Command(ex, args...)
		var out []byte
		if out, err = cmd.Output(); err != nil {
			cmd := ex
			if len(args) != 0 {
				cmd += " " + strings.Join(args, " ")
			}
			return "", "", fmt.Errorf("failed to execute '%s'", cmd)
		}
		return ex, strings.TrimSpace(string(out)), nil
	}

	goexe, version, err := getResponse("go", "version")
	if err != nil {
		return nil, err
	}

	return &pulumirpc.AboutResponse{
		Executable: goexe,
		Version:    version,
	}, nil
}

type goModule struct {
	Path     string
	Version  string
	Time     string
	Indirect bool
	Dir      string
	GoMod    string
	Main     bool
}

func (host *goLanguageHost) GetProgramDependencies(
	ctx context.Context, req *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	// go list -m ...
	//
	//Go has a --json flag, but it doesn't emit a single json object (which
	//makes it invalid json).
	ex, err := executable.FindExecutable("go")
	if err != nil {
		return nil, err
	}
	if err := goversion.CheckMinimumGoVersion(ex); err != nil {
		return nil, err
	}
	cmdArgs := []string{"list", "--json", "-m", "..."}
	cmd := exec.Command(ex, cmdArgs...)
	var out []byte
	if out, err = cmd.Output(); err != nil {
		return nil, fmt.Errorf("failed to get modules: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(out))
	parsed := []goModule{}
	for {
		var m goModule
		if err := dec.Decode(&m); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to parse \"%s %s\" output: %w", ex, strings.Join(cmdArgs, " "), err)
		}
		parsed = append(parsed, m)

	}

	result := []*pulumirpc.DependencyInfo{}
	for _, d := range parsed {
		if (!d.Indirect || req.TransitiveDependencies) && !d.Main {
			datum := pulumirpc.DependencyInfo{
				Name:    d.Path,
				Version: d.Version,
			}
			result = append(result, &datum)
		}
	}
	return &pulumirpc.GetProgramDependenciesResponse{
		Dependencies: result,
	}, nil
}

func (host *goLanguageHost) RunPlugin(
	req *pulumirpc.RunPluginRequest, server pulumirpc.LanguageRuntime_RunPluginServer,
) error {
	logging.V(5).Infof("Attempting to run go plugin in %s", req.Program)

	program, err := compileProgram(req.Program, "")
	if err != nil {
		return fmt.Errorf("error in compiling Go: %w", err)
	}
	defer os.Remove(program)

	closer, stdout, stderr, err := rpcutil.MakeRunPluginStreams(server, false)
	if err != nil {
		return err
	}
	// best effort close, but we try an explicit close and error check at the end as well
	defer closer.Close()

	cmd := exec.Command(program, req.Args...)
	cmd.Dir = req.Pwd
	cmd.Env = req.Env
	cmd.Stdout, cmd.Stderr = stdout, stderr

	if err = cmd.Run(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				err = server.Send(&pulumirpc.RunPluginResponse{
					Output: &pulumirpc.RunPluginResponse_Exitcode{Exitcode: int32(status.ExitStatus())},
				})
			} else {
				err = fmt.Errorf("program exited unexpectedly: %w", exiterr)
			}
		} else {
			return fmt.Errorf("problem executing plugin program (could not run language executor): %w", err)
		}
	}

	if err != nil {
		return err
	}

	return closer.Close()
}
