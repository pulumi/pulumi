// Copyright 2016-2018, Pulumi Corporation.
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

// pulumi-language-python serves as the "language host" for Pulumi programs written in Python.  It is ultimately
// responsible for spawning the language runtime that executes the program.
//
// The program being executed is executed by a shim script called `pulumi-language-python-exec`. This script is
// written in the hosted language (in this case, Python) and is responsible for initiating RPC links to the resource
// monitor and engine.
//
// It's therefore the responsibility of this program to implement the LanguageHostServer endpoint by spawning
// instances of `pulumi-language-python-exec` and forwarding the RPC request arguments to the command-line.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/pulumi/pulumi/sdk/v3/python"
	"google.golang.org/grpc"
)

const (
	// By convention, the executor is the name of the current program (pulumi-language-python) plus this suffix.
	pythonDefaultExec = "pulumi-language-python-exec" // the exec shim for Pulumi to run Python programs.

	// The runtime expects the config object to be saved to this environment variable.
	pulumiConfigVar = "PULUMI_CONFIG"

	// The runtime expects the array of secret config keys to be saved to this environment variable.
	//nolint: gosec
	pulumiConfigSecretKeysVar = "PULUMI_CONFIG_SECRET_KEYS"
)

// Launches the language host RPC endpoint, which in turn fires up an RPC server implementing the
// LanguageRuntimeServer RPC endpoint.
func main() {
	var tracing string
	var virtualenv string
	var root string
	flag.StringVar(&tracing, "tracing", "", "Emit tracing to a Zipkin-compatible tracing endpoint")
	flag.StringVar(&virtualenv, "virtualenv", "", "Virtual environment path to use")
	flag.StringVar(&root, "root", "", "Project root path to use")

	cwd, err := os.Getwd()
	if err != nil {
		cmdutil.Exit(errors.Wrapf(err, "getting the working directory"))
	}

	// You can use the below flag to request that the language host load a specific executor instead of probing the
	// PATH.  This can be used during testing to override the default location.
	var givenExecutor string
	flag.StringVar(&givenExecutor, "use-executor", "",
		"Use the given program as the executor instead of looking for one on PATH")

	flag.Parse()
	args := flag.Args()
	logging.InitLogging(false, 0, false)
	cmdutil.InitTracing("pulumi-language-python", "pulumi-language-python", tracing)

	var pythonExec string
	if givenExecutor == "" {
		// By default, the -exec script is installed next to the language host.
		thisPath, err := os.Executable()
		if err != nil {
			err = errors.Wrap(err, "could not determine current executable")
			cmdutil.Exit(err)
		}

		pathExec := filepath.Join(filepath.Dir(thisPath), pythonDefaultExec)
		if _, err = os.Stat(pathExec); os.IsNotExist(err) {
			err = errors.Errorf("missing executor %s", pathExec)
			cmdutil.Exit(err)
		}

		logging.V(3).Infof("language host identified executor from path: `%s`", pathExec)
		pythonExec = pathExec
	} else {
		logging.V(3).Infof("language host asked to use specific executor: `%s`", givenExecutor)
		pythonExec = givenExecutor
	}

	// Optionally pluck out the engine so we can do logging, etc.
	var engineAddress string
	if len(args) > 0 {
		engineAddress = args[0]
	}

	// Resolve virtualenv path relative to root.
	virtualenvPath := resolveVirtualEnvironmentPath(root, virtualenv)

	// Fire up a gRPC server, letting the kernel choose a free port.
	port, done, err := rpcutil.Serve(0, nil, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			host := newLanguageHost(pythonExec, engineAddress, tracing, cwd, virtualenv, virtualenvPath)
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

// pythonLanguageHost implements the LanguageRuntimeServer interface
// for use as an API endpoint.
type pythonLanguageHost struct {
	exec          string
	engineAddress string
	tracing       string

	// current working directory
	cwd string

	// virtualenv option as passed from Pulumi.yaml runtime.options.virtualenv.
	virtualenv string

	// if non-empty, points to the resolved directory path of the virtualenv
	virtualenvPath string
}

func newLanguageHost(exec, engineAddress, tracing, cwd, virtualenv,
	virtualenvPath string) pulumirpc.LanguageRuntimeServer {

	return &pythonLanguageHost{
		cwd:            cwd,
		exec:           exec,
		engineAddress:  engineAddress,
		tracing:        tracing,
		virtualenv:     virtualenv,
		virtualenvPath: virtualenvPath,
	}
}

// GetRequiredPlugins computes the complete set of anticipated plugins required by a program.
func (host *pythonLanguageHost) GetRequiredPlugins(ctx context.Context,
	req *pulumirpc.GetRequiredPluginsRequest) (*pulumirpc.GetRequiredPluginsResponse, error) {

	// Prepare the virtual environment (if needed).
	err := host.prepareVirtualEnvironment(ctx, host.cwd)
	if err != nil {
		return nil, err
	}

	// Now, determine which Pulumi packages are installed.
	pulumiPackages, err := determinePulumiPackages(host.virtualenvPath, host.cwd)
	if err != nil {
		return nil, err
	}

	plugins := []*pulumirpc.PluginDependency{}
	for _, pkg := range pulumiPackages {

		plugin, err := determinePluginDependency(host.virtualenvPath, host.cwd, pkg.Name, pkg.Version)
		if err != nil {
			return nil, err
		}

		if plugin != nil {
			plugins = append(plugins, plugin)
		}
	}

	return &pulumirpc.GetRequiredPluginsResponse{Plugins: plugins}, nil
}

func resolveVirtualEnvironmentPath(root, virtualenv string) string {
	if virtualenv == "" {
		return ""
	}
	if !filepath.IsAbs(virtualenv) {
		return filepath.Join(root, virtualenv)
	}
	return virtualenv
}

// prepareVirtualEnvironment will create and install dependencies in the virtual environment if host.virtualenv is set.
func (host *pythonLanguageHost) prepareVirtualEnvironment(ctx context.Context, cwd string) error {

	if host.virtualenv == "" {
		return nil
	}

	virtualenv := host.virtualenvPath

	// If the virtual environment directory doesn't exist, create it.
	var createVirtualEnv bool
	info, err := os.Stat(virtualenv)
	if err != nil {
		if os.IsNotExist(err) {
			createVirtualEnv = true
		} else {
			return err
		}
	} else if !info.IsDir() {
		return errors.Errorf("the 'virtualenv' option in Pulumi.yaml is set to %q but it is not a directory", virtualenv)
	}

	// If the virtual environment directory exists, but is empty, it needs to be created.
	if !createVirtualEnv {
		empty, err := fsutil.IsDirEmpty(virtualenv)
		if err != nil {
			return err
		}
		createVirtualEnv = empty
	}

	// Create the virtual environment and install dependencies into it, if needed.
	if createVirtualEnv {
		// Make a connection to the real engine that we will log messages to.
		conn, err := grpc.Dial(
			host.engineAddress,
			grpc.WithInsecure(),
			rpcutil.GrpcChannelOptions(),
		)
		if err != nil {
			return errors.Wrapf(err, "language host could not make connection to engine")
		}

		// Make a client around that connection.
		engineClient := pulumirpc.NewEngineClient(conn)

		// Create writers that log the output of the install operation as ephemeral messages.
		streamID := rand.Int31() //nolint:gosec

		infoWriter := &logWriter{
			ctx:          ctx,
			engineClient: engineClient,
			streamID:     streamID,
			severity:     pulumirpc.LogSeverity_INFO,
		}

		errorWriter := &logWriter{
			ctx:          ctx,
			engineClient: engineClient,
			streamID:     streamID,
			severity:     pulumirpc.LogSeverity_ERROR,
		}

		if err := python.InstallDependenciesWithWriters(
			cwd, virtualenv, true /*showOutput*/, infoWriter, errorWriter); err != nil {
			return err
		}
	}

	// Ensure the specified virtual directory is a valid virtual environment.
	if !python.IsVirtualEnv(virtualenv) {
		return python.NewVirtualEnvError(host.virtualenv, virtualenv)
	}

	return nil
}

type logWriter struct {
	ctx          context.Context
	engineClient pulumirpc.EngineClient
	streamID     int32
	severity     pulumirpc.LogSeverity
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	val := string(p)
	if _, err := w.engineClient.Log(w.ctx, &pulumirpc.LogRequest{
		Message:   strings.ToValidUTF8(val, "�"),
		Urn:       "",
		Ephemeral: true,
		StreamId:  w.streamID,
		Severity:  w.severity,
	}); err != nil {
		return 0, err
	}
	return len(val), nil
}

// These packages are known not to have any plugins.
// TODO[pulumi/pulumi#5863]: Remove this once the `pulumi-policy` package includes a `pulumiplugin.json`
// file that indicates the package does not have an associated plugin, and enough time has passed.
var packagesWithoutPlugins = map[string]struct{}{
	"pulumi-policy": {},
}

type pythonPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func determinePulumiPackages(virtualenv, cwd string) ([]pythonPackage, error) {
	logging.V(5).Infof("GetRequiredPlugins: Determining pulumi packages")

	// Run the `python -m pip list --format json` command.
	args := []string{"-m", "pip", "list", "--format", "json"}
	output, err := runPythonCommand(virtualenv, cwd, args...)
	if err != nil {
		return nil, err
	}

	// Parse the JSON output.
	var packages []pythonPackage
	if err := json.Unmarshal(output, &packages); err != nil {
		return nil, errors.Wrapf(err, "parsing `python %s` output", strings.Join(args, " "))
	}

	// Only return Pulumi packages.
	var pulumiPackages []pythonPackage
	for _, pkg := range packages {
		// We're only interested in packages that start with "pulumi-".
		if !strings.HasPrefix(pkg.Name, "pulumi-") {
			continue
		}

		// Skip packages that are known not have an associated plugin.
		if _, ok := packagesWithoutPlugins[pkg.Name]; ok {
			continue
		}

		pulumiPackages = append(pulumiPackages, pkg)
	}

	logging.V(5).Infof("GetRequiredPlugins: Pulumi packages: %#v", pulumiPackages)

	return pulumiPackages, nil
}

// determinePluginDependency attempts to determine a plugin associated with a package. It checks to see if the package
// contains a pulumiplugin.json file and uses the information in that file to determine the plugin. If `resource` in
// pulumiplugin.json is set to false, nil is returned. If the name or version aren't specified in the file, these values
// are derived from the package name and version. If the plugin version cannot be determined from the package version,
// nil is returned.
func determinePluginDependency(
	virtualenv, cwd, packageName, packageVersion string) (*pulumirpc.PluginDependency, error) {

	logging.V(5).Infof("GetRequiredPlugins: Determining plugin dependency: %v, %v", packageName, packageVersion)

	// Determine the location of the installed package.
	packageLocation, err := determinePackageLocation(virtualenv, cwd, packageName)
	if err != nil {
		return nil, err
	}

	// The name of the module inside the package can be different from the package name.
	// However, our convention is to always use the same name, e.g. a package name of
	// "pulumi-aws" will have a module named "pulumi_aws", so we can determine the module
	// by replacing hyphens with underscores.
	packageModuleName := strings.ReplaceAll(packageName, "-", "_")

	pulumiPluginFilePath := filepath.Join(packageLocation, packageModuleName, "pulumiplugin.json")
	logging.V(5).Infof("GetRequiredPlugins: pulumiplugin.json file path: %s", pulumiPluginFilePath)

	var name, version, server string
	plugin, err := plugin.LoadPulumiPluginJSON(pulumiPluginFilePath)
	if err == nil {
		// If `resource` is set to false, the Pulumi package has indicated that there is no associated plugin.
		// Ignore it.
		if !plugin.Resource {
			logging.V(5).Infof("GetRequiredPlugins: Ignoring package %s with resource set to false", packageName)
			return nil, nil
		}

		name, version, server = plugin.Name, plugin.Version, plugin.Server
	} else if !os.IsNotExist(err) {
		// If the file doesn't exist, the name and version of the plugin will attempt to be determined from the
		// packageName and packageVersion. If it's some other error, report it.
		logging.V(5).Infof("GetRequiredPlugins: err: %v", err)
		return nil, err
	}

	if name == "" {
		name = strings.TrimPrefix(packageName, "pulumi-")
	}

	if version == "" {
		// The packageVersion may include additional pre-release tags (e.g. "2.14.0a1605583329" for an alpha
		// release, "2.14.0b1605583329" for a beta release, "2.14.0rc1605583329" for an rc release, etc.).
		// Unfortunately, this is not enough information to determine the plugin version. A package version of
		// "3.31.0a1605189729" will have an associated plugin with a version of "3.31.0-alpha.1605189729+42435656".
		// The "+42435656" suffix cannot be determined so the plugin version cannot be determined. In such cases,
		// log the issue and skip the package.
		version, err = determinePluginVersion(packageVersion)
		if err != nil {
			logging.V(5).Infof(
				"GetRequiredPlugins: Could not determine plugin version for package %s with version %s",
				packageName, packageVersion)
			return nil, nil
		}
	}
	if !strings.HasPrefix(version, "v") {
		// Add "v" prefix if not already present.
		version = fmt.Sprintf("v%s", version)
	}

	result := pulumirpc.PluginDependency{
		Name:    name,
		Version: version,
		Kind:    "resource",
		Server:  server,
	}

	logging.V(5).Infof("GetRequiredPlugins: Determining plugin dependency: %#v", result)
	return &result, nil
}

// determinePackageLocation determines the location on disk of the package by running `python -m pip show <package>`
// and parsing the output.
func determinePackageLocation(virtualenv, cwd, packageName string) (string, error) {
	b, err := runPythonCommand(virtualenv, cwd, "-m", "pip", "show", packageName)
	if err != nil {
		return "", err
	}
	return parseLocation(packageName, string(b))
}

func parseLocation(packageName, pipShowOutput string) (string, error) {
	// We want the value of Location from the following output of `python -m pip show <packageName>`:
	// $ python -m pip show pulumi-aws
	// Name: pulumi-aws
	// Version: 3.12.2
	// Summary: A Pulumi package for creating and managing Amazon Web Services (AWS) cloud resources.
	// Home-page: https://pulumi.io
	// Author: None
	// Author-email: None
	// License: Apache-2.0
	// Location: /Users/user/proj/venv/lib/python3.8/site-packages
	// Requires: parver, pulumi, semver
	// Required-by:
	lines := strings.Split(pipShowOutput, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Location:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Location:")), nil
		}
	}

	return "", errors.Errorf("determining location of package %s", packageName)
}

// determinePluginVersion attempts to convert a PEP440 package version into a plugin version.
// The package version must have only major.minor.patch components and each must be integers only.
// If there are any other characters in the component (e.g. pre-release tags), an error is returned
// because there isn't enough information to determine the plugin version from a pre-release tag.
func determinePluginVersion(packageVersion string) (string, error) {
	components := strings.Split(packageVersion, ".")
	if len(components) < 2 || len(components) > 3 {
		return "", errors.Errorf("unexpected number of components in version %q", packageVersion)
	}

	// Ensure each component is an integer.
	for i := range components {
		if _, err := strconv.ParseInt(components[i], 10, 64); err != nil {
			names := []string{"major", "minor", "patch"}
			return "", errors.Errorf("parsing %s: %q", names[i], components[i])
		}
	}

	return packageVersion, nil
}

func runPythonCommand(virtualenv, cwd string, arg ...string) ([]byte, error) {
	var err error
	var cmd *exec.Cmd
	if virtualenv != "" {
		cmd = python.VirtualEnvCommand(virtualenv, "python", arg...)
	} else {
		cmd, err = python.Command(arg...)
		if err != nil {
			return nil, err
		}
	}

	if logging.V(5) {
		commandStr := strings.Join(arg, " ")
		logging.V(5).Infof("Language host launching process: %s %s", cmd.Path, commandStr)
	}

	cmd.Dir = cwd
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	if logging.V(9) {
		logging.V(9).Infof("Process output: %s", string(output))
	}

	return output, err
}

// RPC endpoint for LanguageRuntimeServer::Run
func (host *pythonLanguageHost) Run(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	args := []string{host.exec}
	args = append(args, host.constructArguments(req)...)

	config, err := host.constructConfig(req)
	if err != nil {
		err = errors.Wrap(err, "failed to serialize configuration")
		return nil, err
	}
	configSecretKeys, err := host.constructConfigSecretKeys(req)
	if err != nil {
		err = errors.Wrap(err, "failed to serialize configuration secret keys")
		return nil, err
	}

	if logging.V(5) {
		commandStr := strings.Join(args, " ")
		logging.V(5).Infoln("Language host launching process: ", host.exec, commandStr)
	}

	// Now simply spawn a process to execute the requested program, wiring up stdout/stderr directly.
	var errResult string
	var cmd *exec.Cmd
	var virtualenv string
	if host.virtualenv != "" {
		virtualenv = host.virtualenvPath
		if !python.IsVirtualEnv(virtualenv) {
			return nil, python.NewVirtualEnvError(host.virtualenv, virtualenv)
		}
		cmd = python.VirtualEnvCommand(virtualenv, "python", args...)
	} else {
		cmd, err = python.Command(args...)
		if err != nil {
			return nil, err
		}
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if virtualenv != "" || config != "" || configSecretKeys != "" {
		env := os.Environ()
		if virtualenv != "" {
			env = python.ActivateVirtualEnv(env, virtualenv)
		}
		if config != "" {
			env = append(env, pulumiConfigVar+"="+config)
		}
		if configSecretKeys != "" {
			env = append(env, pulumiConfigSecretKeysVar+"="+configSecretKeys)
		}
		cmd.Env = env
	}
	if err := cmd.Run(); err != nil {
		// Python does not explicitly flush standard out or standard error when exiting abnormally. For this reason, we
		// need to explicitly flush our output streams so that, when we exit, the engine picks up the child Python
		// process's stdout and stderr writes.
		//
		// This is especially crucial for Python because it is possible for the child Python process to crash very fast
		// if Pulumi is misconfigured, so we must be sure to present a high-quality error message to the user.
		contract.IgnoreError(os.Stdout.Sync())
		contract.IgnoreError(os.Stderr.Sync())
		if exiterr, ok := err.(*exec.ExitError); ok {
			// If the program ran, but exited with a non-zero error code.  This will happen often, since user
			// errors will trigger this.  So, the error message should look as nice as possible.
			if status, stok := exiterr.Sys().(syscall.WaitStatus); stok {
				err = errors.Errorf("Program exited with non-zero exit code: %d", status.ExitStatus())
			} else {
				err = errors.Wrapf(exiterr, "Program exited unexpectedly")
			}
		} else {
			// Otherwise, we didn't even get to run the program.  This ought to never happen unless there's
			// a bug or system condition that prevented us from running the language exec.  Issue a scarier error.
			err = errors.Wrapf(err, "Problem executing program (could not run language executor)")
		}

		errResult = err.Error()
	}

	return &pulumirpc.RunResponse{Error: errResult}, nil
}

// constructArguments constructs a command-line for `pulumi-language-python`
// by enumerating all of the optional and non-optional arguments present
// in a RunRequest.
func (host *pythonLanguageHost) constructArguments(req *pulumirpc.RunRequest) []string {
	var args []string
	maybeAppendArg := func(k, v string) {
		if v != "" {
			args = append(args, "--"+k, v)
		}
	}

	maybeAppendArg("monitor", req.GetMonitorAddress())
	maybeAppendArg("engine", host.engineAddress)
	maybeAppendArg("project", req.GetProject())
	maybeAppendArg("stack", req.GetStack())
	maybeAppendArg("pwd", req.GetPwd())
	maybeAppendArg("dry_run", fmt.Sprintf("%v", req.GetDryRun()))
	maybeAppendArg("parallel", fmt.Sprint(req.GetParallel()))
	maybeAppendArg("tracing", host.tracing)

	// If no program is specified, just default to the current directory (which will invoke "__main__.py").
	if req.GetProgram() == "" {
		args = append(args, ".")
	} else {
		args = append(args, req.GetProgram())
	}

	args = append(args, req.GetArgs()...)
	return args
}

// constructConfig json-serializes the configuration data given as part of a RunRequest.
func (host *pythonLanguageHost) constructConfig(req *pulumirpc.RunRequest) (string, error) {
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
func (host *pythonLanguageHost) constructConfigSecretKeys(req *pulumirpc.RunRequest) (string, error) {
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

func (host *pythonLanguageHost) GetPluginInfo(ctx context.Context, req *pbempty.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: version.Version,
	}, nil
}
