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
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/blang/semver"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"google.golang.org/grpc"

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

func compileProgramCwd(buildID string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", errors.Wrap(err, "unable to get current working directory")
	}

	goFileSearchPattern := filepath.Join(cwd, "*.go")
	if matches, err := filepath.Glob(goFileSearchPattern); err != nil || len(matches) == 0 {
		return "", errors.Errorf("Failed to find go files for 'go build' matching %s", goFileSearchPattern)
	}

	if err != nil {
		return "", errors.Wrap(err, "failed to compile")
	}

	filePrefix := fmt.Sprintf("pulumi-go.%s", buildID)
	if runtime.GOOS == "windows" {
		filePrefix = fmt.Sprintf("pulumi-go.%s.exe", buildID)
	}

	f, err := os.CreateTemp("", filePrefix+".*")
	if err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	outfile := f.Name()

	gobin, err := executable.FindExecutable("go")
	if err != nil {
		return "", errors.Wrap(err, "problem executing program (could not run language executor)")
	}
	buildCmd := exec.Command(gobin, "build", "-o", outfile, cwd)
	buildCmd.Stdout, buildCmd.Stderr = os.Stdout, os.Stderr

	if err := buildCmd.Run(); err != nil {
		return "", err
	}

	return outfile, nil
}

// Launches the language host, which in turn fires up an RPC server implementing the LanguageRuntimeServer endpoint.
func main() {
	var tracing string
	var binary string
	var root string
	flag.StringVar(&tracing, "tracing", "", "Emit tracing to a Zipkin-compatible tracing endpoint")
	flag.StringVar(&binary, "binary", "", "Look on path for a binary executable with this name")
	flag.StringVar(&root, "root", "", "Project root path to use")

	flag.Parse()
	args := flag.Args()
	logging.InitLogging(false, 0, false)
	cmdutil.InitTracing("pulumi-language-go", "pulumi-language-go", tracing)

	// Pluck out the engine so we can do logging, etc.
	if len(args) == 0 {
		cmdutil.Exit(errors.New("missing required engine RPC address argument"))
	}
	engineAddress := args[0]

	ctx, cancel := context.WithCancel(context.Background())
	// map the context Done channel to the rpcutil boolean cancel channel
	cancelChannel := make(chan bool)
	go func() {
		<-ctx.Done()
		close(cancelChannel)
	}()
	err := rpcutil.Healthcheck(ctx, engineAddress, 5*time.Minute, cancel)
	if err != nil {
		cmdutil.Exit(errors.Wrapf(err, "could not start health check host RPC server"))
	}

	// Fire up a gRPC server, letting the kernel choose a free port.
	port, done, err := rpcutil.Serve(0, cancelChannel, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			host := newLanguageHost(engineAddress, tracing, binary)
			pulumirpc.RegisterLanguageRuntimeServer(srv, host)
			return nil
		},
	}, nil)
	if err != nil {
		cmdutil.Exit(errors.Wrapf(err, "could not start language host RPC server"))
	}

	// Otherwise, print out the port so that the spawner knows how to reach us.
	fmt.Printf("%d\n", port)

	// And finally wait for the server to stop serving.
	if err := <-done; err != nil {
		cmdutil.Exit(errors.Wrapf(err, "language host RPC stopped serving"))
	}
}

// goLanguageHost implements the LanguageRuntimeServer interface for use as an API endpoint.
type goLanguageHost struct {
	engineAddress string
	tracing       string
	binary        string
}

func newLanguageHost(engineAddress, tracing, binary string) pulumirpc.LanguageRuntimeServer {
	return &goLanguageHost{
		engineAddress: engineAddress,
		tracing:       tracing,
		binary:        binary,
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
		return nil, fmt.Errorf("Failed to load pulumi-plugin.json: %w", err)
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
	req *pulumirpc.GetRequiredPluginsRequest) (*pulumirpc.GetRequiredPluginsResponse, error) {

	logging.V(5).Infof("GetRequiredPlugins: Determining pulumi packages")

	gobin, err := executable.FindExecutable("go")
	if err != nil {
		return nil, errors.Wrap(err, "couldn't find go binary")
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

func execProgramCmd(cmd *exec.Cmd, env []string) error {
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr

	err := cmd.Run()
	if err == nil {
		// Go program completed successfully
		return nil
	}

	// error handling
	exiterr, ok := err.(*exec.ExitError)
	if !ok {
		return errors.Wrapf(exiterr, "command errored unexpectedly")
	}

	// retrieve the status code
	status, ok := exiterr.Sys().(syscall.WaitStatus)
	if !ok {
		return errors.Wrapf(exiterr, "program exited unexpectedly")
	}

	// If the program ran, but exited with a non-zero error code. This will happen often, since user
	// errors will trigger this.  So, the error message should look as nice as possible.
	return errors.Errorf("program exited with non-zero exit code: %d", status.ExitStatus())
}

// RPC endpoint for LanguageRuntimeServer::Run
func (host *goLanguageHost) Run(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	// Create the environment we'll use to run the process.  This is how we pass the RunInfo to the actual
	// Go program runtime, to avoid needing any sort of program interface other than just a main entrypoint.
	env, err := host.constructEnv(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare environment")
	}

	if os.Getenv("PULUMI_GO_USE_RUN") != "" {
		// feature flag to enable old behavior and use `go run`
		gobin, err := executable.FindExecutable("go")
		if err != nil {
			return nil, err
		}

		cwd, err := os.Getwd()
		if err != nil {
			return nil, errors.Wrap(err, "unable to get current working directory")
		}

		cmd := exec.Command(gobin, "run", cwd)
		if err := execProgramCmd(cmd, env); err != nil {
			return &pulumirpc.RunResponse{
				Error: err.Error(),
			}, nil
		}

		return &pulumirpc.RunResponse{}, nil
	}

	// the user can explicitly opt in to using a binary executable by specifying
	// runtime.options.binary in the Pulumi.yaml
	program := host.binary
	if program == "" {
		// user did not specify a binary and we will compile and run the binary on-demand
		logging.V(5).Infof("No prebuilt executable specified, attempting invocation via compilation")

		program, err = compileProgramCwd(req.GetProject())
		if err != nil {
			return nil, errors.Wrap(err, "error in compiling Go")
		}
		defer os.RemoveAll(program)
	}

	bin, err := executable.FindExecutable(program)
	if err != nil {
		bin = program
	}

	cmd := exec.Command(bin)
	if err := execProgramCmd(cmd, env); err != nil {
		return &pulumirpc.RunResponse{
			Error: err.Error(),
		}, nil
	}

	return &pulumirpc.RunResponse{}, nil
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
	req *pulumirpc.InstallDependenciesRequest, server pulumirpc.LanguageRuntime_InstallDependenciesServer) error {

	closer, stdout, stderr, err := rpcutil.MakeStreams(server, req.IsTerminal)
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

	if err := closer.Close(); err != nil {
		return err
	}

	return nil
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
	ctx context.Context, req *pulumirpc.GetProgramDependenciesRequest) (*pulumirpc.GetProgramDependenciesResponse, error) {
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
		return nil, fmt.Errorf("Failed to get modules: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(out))
	parsed := []goModule{}
	for {
		var m goModule
		if err := dec.Decode(&m); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("Failed to parse \"%s %s\" output: %w", ex, strings.Join(cmdArgs, " "), err)
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
