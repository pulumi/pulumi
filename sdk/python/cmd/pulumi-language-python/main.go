// Copyright 2016-2023, Pulumi Corporation.
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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/pulumi/pulumi/sdk/v3/python"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	hclsyntax "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	codegen "github.com/pulumi/pulumi/pkg/v3/codegen/python"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

const (
	// By convention, the executor is the name of the current program (pulumi-language-python) plus this suffix.
	pythonDefaultExec = "pulumi-language-python-exec" // the exec shim for Pulumi to run Python programs.

	// The runtime expects the config object to be saved to this environment variable.
	pulumiConfigVar = "PULUMI_CONFIG"

	// The runtime expects the array of secret config keys to be saved to this environment variable.
	//nolint:gosec
	pulumiConfigSecretKeysVar = "PULUMI_CONFIG_SECRET_KEYS"

	// A exit-code we recognize when the python process exits.  If we see this error, there's no
	// need for us to print any additional error messages since the user already got a a good
	// one they can handle.
	pythonProcessExitedAfterShowingUserActionableMessage = 32
)

var (
	// The minimum python version that Pulumi supports
	minimumSupportedPythonVersion = semver.MustParse("3.7.0")
	// Any version less then `eolPythonVersion` is EOL.
	eolPythonVersion = semver.MustParse("3.7.0")
	// An url to the issue discussing EOL.
	eolPythonVersionIssue = "https://github.com/pulumi/pulumi/issues/8131"
)

// Launches the language host RPC endpoint, which in turn fires up an RPC server implementing the
// LanguageRuntimeServer RPC endpoint.
func main() {
	var tracing string
	flag.StringVar(&tracing, "tracing", "", "Emit tracing to a Zipkin-compatible tracing endpoint")
	flag.String("virtualenv", "", "[obsolete] Virtual environment path to use")
	flag.String("root", "", "[obsolete] Project root path to use")

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
			err = fmt.Errorf("could not determine current executable: %w", err)
			cmdutil.Exit(err)
		}

		pathExec := filepath.Join(filepath.Dir(thisPath), pythonDefaultExec)
		if _, err = os.Stat(pathExec); os.IsNotExist(err) {
			err = fmt.Errorf("missing executor %s: %w", pathExec, err)
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

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	// map the context Done channel to the rpcutil boolean cancel channel
	cancelChannel := make(chan bool)
	go func() {
		<-ctx.Done()
		cancel() // deregister signal handler
		close(cancelChannel)
	}()
	err := rpcutil.Healthcheck(ctx, engineAddress, 5*time.Minute, cancel)
	if err != nil {
		cmdutil.Exit(fmt.Errorf("could not start health check host RPC server: %w", err))
	}

	// Fire up a gRPC server, letting the kernel choose a free port.
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancelChannel,
		Init: func(srv *grpc.Server) error {
			host := newLanguageHost(pythonExec, engineAddress, tracing)
			pulumirpc.RegisterLanguageRuntimeServer(srv, host)
			return nil
		},
		Options: rpcutil.OpenTracingServerInterceptorOptions(nil),
	})
	if err != nil {
		cmdutil.Exit(fmt.Errorf("could not start language host RPC server: %w", err))
	}

	// Otherwise, print out the port so that the spawner knows how to reach us.
	fmt.Printf("%d\n", handle.Port)

	// And finally wait for the server to stop serving.
	if err := <-handle.Done; err != nil {
		cmdutil.Exit(fmt.Errorf("language host RPC stopped serving: %w", err))
	}
}

// pythonLanguageHost implements the LanguageRuntimeServer interface
// for use as an API endpoint.
type pythonLanguageHost struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	exec          string
	engineAddress string
	tracing       string
}

type pythonOptions struct {
	// Virtual environment path to use.
	virtualenv string
	// The resolved virtual environment path.
	virtualenvPath string
}

func parseOptions(root string, options map[string]interface{}) (pythonOptions, error) {
	var pythonOptions pythonOptions
	if virtualenv, ok := options["virtualenv"]; ok {
		if virtualenv, ok := virtualenv.(string); ok {
			pythonOptions.virtualenv = virtualenv
		} else {
			return pythonOptions, errors.New("virtualenv option must be a string")
		}
	}

	// Resolve virtualenv path relative to root.
	pythonOptions.virtualenvPath = resolveVirtualEnvironmentPath(root, pythonOptions.virtualenv)

	return pythonOptions, nil
}

func newLanguageHost(exec, engineAddress, tracing string,
) pulumirpc.LanguageRuntimeServer {
	return &pythonLanguageHost{
		exec:          exec,
		engineAddress: engineAddress,
		tracing:       tracing,
	}
}

// GetRequiredPlugins computes the complete set of anticipated plugins required by a program.
func (host *pythonLanguageHost) GetRequiredPlugins(ctx context.Context,
	req *pulumirpc.GetRequiredPluginsRequest,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	opts, err := parseOptions(req.Info.RootDirectory, req.Info.Options.AsMap())
	if err != nil {
		return nil, err
	}

	// Prepare the virtual environment (if needed).
	err = host.prepareVirtualEnvironment(ctx, req.Info.ProgramDirectory, opts.virtualenvPath)
	if err != nil {
		return nil, err
	}

	validateVersion(ctx, opts.virtualenvPath)

	// Now, determine which Pulumi packages are installed.
	pulumiPackages, err := determinePulumiPackages(ctx, opts.virtualenvPath, req.Info.ProgramDirectory)
	if err != nil {
		return nil, err
	}

	plugins := []*pulumirpc.PluginDependency{}
	for _, pkg := range pulumiPackages {
		plugin, err := determinePluginDependency(opts.virtualenvPath, req.Info.ProgramDirectory, pkg)
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
func (host *pythonLanguageHost) prepareVirtualEnvironment(ctx context.Context, pwd, virtualenv string) error {
	if virtualenv == "" {
		return nil
	}

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
		return fmt.Errorf("the 'virtualenv' option in Pulumi.yaml is set to %q but it is not a directory", virtualenv)
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
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			rpcutil.GrpcChannelOptions(),
		)
		if err != nil {
			return fmt.Errorf("language host could not make connection to engine: %w", err)
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

		if err := python.InstallDependenciesWithWriters(ctx,
			pwd, virtualenv, true /*showOutput*/, infoWriter, errorWriter); err != nil {
			return err
		}
	}

	// Ensure the specified virtual directory is a valid virtual environment.
	if !python.IsVirtualEnv(virtualenv) {
		return python.NewVirtualEnvError(virtualenv, virtualenv)
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
// TODO[pulumi/pulumi#5863]: Remove this once the `pulumi-policy` package includes a `pulumi-plugin.json`
// file that indicates the package does not have an associated plugin, and enough time has passed.
var packagesWithoutPlugins = map[string]struct{}{
	"pulumi-policy": {},
}

type pythonPackage struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Location string `json:"location"`
	plugin   *plugin.PulumiPluginJSON
}

// Returns if pkg is a pulumi package.
//
// We check:
// 1. If there is a pulumi-plugin.json file.
// 2. If the first segment is "pulumi". This implies a first party package.
func (pkg *pythonPackage) isPulumiPackage() bool {
	plugin, err := pkg.readPulumiPluginJSON()
	if err == nil && plugin != nil {
		return true
	}

	return strings.HasPrefix(pkg.Name, "pulumi-")
}

func (pkg *pythonPackage) readPulumiPluginJSON() (*plugin.PulumiPluginJSON, error) {
	if pkg.plugin != nil {
		return pkg.plugin, nil
	}

	// The name of the module inside the package can be different from the package name.
	// However, our convention is to always use the same name, e.g. a package name of
	// "pulumi-aws" will have a module named "pulumi_aws", so we can determine the module
	// by replacing hyphens with underscores.
	packageModuleName := strings.ReplaceAll(pkg.Name, "-", "_")
	pulumiPluginFilePath := filepath.Join(pkg.Location, packageModuleName, "pulumi-plugin.json")
	logging.V(5).Infof("readPulumiPluginJSON: pulumi-plugin.json file path: %s", pulumiPluginFilePath)

	plugin, err := plugin.LoadPulumiPluginJSON(pulumiPluginFilePath)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	pkg.plugin = plugin
	return plugin, nil
}

func determinePulumiPackages(ctx context.Context, virtualenv, cwd string) ([]pythonPackage, error) {
	logging.V(5).Infof("GetRequiredPlugins: Determining pulumi packages")

	// Run the `python -m pip list -v --format json` command.
	args := []string{"-m", "pip", "list", "-v", "--format", "json"}
	output, err := runPythonCommand(ctx, virtualenv, cwd, args...)
	if err != nil {
		return nil, fmt.Errorf("calling `python %s`: %w", strings.Join(args, " "), err)
	}

	// Parse the JSON output; on some systems pip -v verbose mode
	// follows JSON with non-JSON trailer, so we need to be
	// careful when parsing and ignore the trailer.
	var packages []pythonPackage
	jsonDecoder := json.NewDecoder(bytes.NewBuffer(output))
	if err := jsonDecoder.Decode(&packages); err != nil {
		return nil, fmt.Errorf("parsing `python %s` output: %w", strings.Join(args, " "), err)
	}

	// Only return Pulumi packages.
	pulumiPackages := slice.Prealloc[pythonPackage](len(packages))
	for _, pkg := range packages {
		if !pkg.isPulumiPackage() {
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
// contains a pulumi-plugin.json file and uses the information in that file to determine the plugin. If `resource` in
// pulumi-plugin.json is set to false, nil is returned. If the name or version aren't specified in the file, these
// values are derived from the package name and version. If the plugin version cannot be determined from the package
// version, nil is returned.
func determinePluginDependency(
	virtualenv, cwd string, pkg pythonPackage,
) (*pulumirpc.PluginDependency, error) {
	var name, version, server string
	plugin, err := pkg.readPulumiPluginJSON()
	if plugin != nil && err == nil {
		// If `resource` is set to false, the Pulumi package has indicated that there is no associated plugin.
		// Ignore it.
		if !plugin.Resource {
			logging.V(5).Infof("GetRequiredPlugins: Ignoring package %s with resource set to false", pkg.Name)
			return nil, nil
		}

		name, version, server = plugin.Name, plugin.Version, plugin.Server
	} else if err != nil {
		logging.V(5).Infof("GetRequiredPlugins: err: %v", err)
		return nil, err
	}

	if name == "" {
		name = strings.TrimPrefix(pkg.Name, "pulumi-")
	}

	if version == "" {
		// The packageVersion may include additional pre-release tags (e.g. "2.14.0a1605583329" for an alpha
		// release, "2.14.0b1605583329" for a beta release, "2.14.0rc1605583329" for an rc release, etc.).
		// Unfortunately, this is not enough information to determine the plugin version. A package version of
		// "3.31.0a1605189729" will have an associated plugin with a version of "3.31.0-alpha.1605189729+42435656".
		// The "+42435656" suffix cannot be determined so the plugin version cannot be determined. In such cases,
		// log the issue and skip the package.
		version, err = determinePluginVersion(pkg.Version)
		if err != nil {
			logging.V(5).Infof(
				"GetRequiredPlugins: Could not determine plugin version for package %s with version %s: %s",
				pkg.Name, pkg.Version, err)
			return nil, nil
		}
	}
	if !strings.HasPrefix(version, "v") {
		// Add "v" prefix if not already present.
		version = fmt.Sprintf("v%s", version)
	}

	result := &pulumirpc.PluginDependency{
		Name:    name,
		Version: version,
		Kind:    "resource",
		Server:  server,
	}

	logging.V(5).Infof("GetRequiredPlugins: Determining plugin dependency: %#v", result)
	return result, nil
}

// determinePluginVersion attempts to convert a PEP440 package version into a plugin version.
//
// Supported versions:
//
//	PEP440 defines a version as `[N!]N(.N)*[{a|b|rc}N][.postN][.devN]`, but
//	determinePluginVersion only supports a subset of that. Translations are provided for
//	`N(.N)*[{a|b|rc}N][.postN][.devN]`.
//
// Translations:
//
//	We ensure that there are at least 3 version segments. Missing segments are `0`
//	padded.
//	Example: 1.0 => 1.0.0
//
//	We translate a,b,rc to alpha,beta,rc respectively with a hyphen separator.
//	Example: 1.2.3a4 => 1.2.3-alpha.4, 1.2.3rc4 => 1.2.3-rc.4
//
//	We translate `.post` and `.dev` by replacing the `.` with a `+`. If both `.post`
//	and `.dev` are present, only one separator is used.
//	Example: 1.2.3.post4 => 1.2.3+post4, 1.2.3.post4.dev5 => 1.2.3+post4dev5
//
// Reference on PEP440: https://www.python.org/dev/peps/pep-0440/
func determinePluginVersion(packageVersion string) (string, error) {
	if len(packageVersion) == 0 {
		return "", fmt.Errorf("cannot parse empty string")
	}
	// Verify ASCII
	for i := 0; i < len(packageVersion); i++ {
		c := packageVersion[i]
		if c > unicode.MaxASCII {
			return "", fmt.Errorf("byte %d is not ascii", i)
		}
	}

	parseNumber := func(s string) (string, string) {
		i := 0
		for _, c := range s {
			if c > '9' || c < '0' {
				break
			}
			i++
		}
		return s[:i], s[i:]
	}

	// Explicitly err on epochs
	if num, maybeEpoch := parseNumber(packageVersion); num != "" && strings.HasPrefix(maybeEpoch, "!") {
		return "", fmt.Errorf("epochs are not supported")
	}

	segments := []string{}
	num, rest := "", packageVersion
	foundDot := false
	for {
		if num, rest = parseNumber(rest); num != "" {
			foundDot = false
			segments = append(segments, num)
			if strings.HasPrefix(rest, ".") {
				rest = rest[1:]
				foundDot = true
			} else {
				break
			}
		} else {
			break
		}
	}
	if foundDot {
		rest = "." + rest
	}

	for len(segments) < 3 {
		segments = append(segments, "0")
	}

	if rest == "" {
		r := strings.Join(segments, ".")
		return r, nil
	}

	var preRelease string

	switch {
	case rest[0] == 'a':
		preRelease, rest = parseNumber(rest[1:])
		preRelease = "-alpha." + preRelease
	case rest[0] == 'b':
		preRelease, rest = parseNumber(rest[1:])
		preRelease = "-beta." + preRelease
	case strings.HasPrefix(rest, "rc"):
		preRelease, rest = parseNumber(rest[2:])
		preRelease = "-rc." + preRelease
	}

	var postRelease string
	if strings.HasPrefix(rest, ".post") {
		postRelease, rest = parseNumber(rest[5:])
		postRelease = "+post" + postRelease
	}

	var developmentRelease string
	if strings.HasPrefix(rest, ".dev") {
		developmentRelease, rest = parseNumber(rest[4:])
		join := ""
		if postRelease == "" {
			join = "+"
		}
		developmentRelease = join + "dev" + developmentRelease
	}

	if rest != "" {
		return "", fmt.Errorf("'%s' still unparsed", rest)
	}

	result := strings.Join(segments, ".") + preRelease + postRelease + developmentRelease

	return result, nil
}

func runPythonCommand(ctx context.Context, virtualenv, cwd string, arg ...string) ([]byte, error) {
	var err error
	var cmd *exec.Cmd
	if virtualenv != "" {
		// Default to the "python" executable in the virtual environment, but allow the user to override it
		// with PULUMI_PYTHON_CMD.
		pythonCmd := os.Getenv("PULUMI_PYTHON_CMD")
		if pythonCmd == "" {
			pythonCmd = "python"
		}
		cmd = python.VirtualEnvCommand(virtualenv, pythonCmd, arg...)
	} else {
		cmd, err = python.Command(ctx, arg...)
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

// Run is RPC endpoint for LanguageRuntimeServer::Run
func (host *pythonLanguageHost) Run(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	opts, err := parseOptions(req.Info.RootDirectory, req.Info.Options.AsMap())
	if err != nil {
		return nil, err
	}

	args := []string{host.exec}
	args = append(args, host.constructArguments(req)...)

	config, err := host.constructConfig(req)
	if err != nil {
		err = fmt.Errorf("failed to serialize configuration: %w", err)
		return nil, err
	}
	configSecretKeys, err := host.constructConfigSecretKeys(req)
	if err != nil {
		err = fmt.Errorf("failed to serialize configuration secret keys: %w", err)
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
	if opts.virtualenv != "" {
		virtualenv = opts.virtualenvPath
		if !python.IsVirtualEnv(virtualenv) {
			return nil, python.NewVirtualEnvError(opts.virtualenv, virtualenv)
		}
		cmd = python.VirtualEnvCommand(virtualenv, "python", args...)
	} else {
		cmd, err = python.Command(ctx, args...)
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
				switch status.ExitStatus() {
				case 0:
					// This really shouldn't happen, but if it does, we don't want to render "non-zero exit code"
					err = fmt.Errorf("program exited unexpectedly: %w", exiterr)
				case pythonProcessExitedAfterShowingUserActionableMessage:
					return &pulumirpc.RunResponse{Error: "", Bail: true}, nil
				default:
					err = fmt.Errorf("program exited with non-zero exit code: %d", status.ExitStatus())
				}
			} else {
				err = fmt.Errorf("program exited unexpectedly: %w", exiterr)
			}
		} else {
			// Otherwise, we didn't even get to run the program.  This ought to never happen unless there's
			// a bug or system condition that prevented us from running the language exec.  Issue a scarier error.
			err = fmt.Errorf("problem executing program (could not run language executor): %w", err)
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
	maybeAppendArg("dry_run", strconv.FormatBool(req.GetDryRun()))
	maybeAppendArg("parallel", strconv.Itoa(int(req.GetParallel())))
	maybeAppendArg("tracing", host.tracing)
	maybeAppendArg("organization", req.GetOrganization())

	// The engine should always pass a name for entry point, even if its just "." for the program directory.
	args = append(args, req.Info.EntryPoint)

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

func (host *pythonLanguageHost) GetPluginInfo(ctx context.Context, req *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: version.Version,
	}, nil
}

// validateVersion checks that python is running a valid version. If a version
// is invalid, it prints to os.Stderr. This is interpreted as diagnostic message
// by the Pulumi CLI program.
func validateVersion(ctx context.Context, virtualEnvPath string) {
	var versionCmd *exec.Cmd
	var err error
	versionArgs := []string{"--version"}
	if virtualEnvPath != "" {
		versionCmd = python.VirtualEnvCommand(virtualEnvPath, "python", versionArgs...)
	} else if versionCmd, err = python.Command(ctx, versionArgs...); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to find python executable\n")
		return
	}
	var out []byte
	if out, err = versionCmd.Output(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to resolve python version command: %s\n", err)
		return
	}
	version := strings.TrimSpace(strings.TrimPrefix(string(out), "Python "))
	parsed, err := semver.Parse(version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse python version: '%s'\n", version)
		return
	}
	if parsed.LT(minimumSupportedPythonVersion) {
		fmt.Fprintf(os.Stderr, "Pulumi does not support Python %s."+
			" Please upgrade to at least %s\n", parsed, minimumSupportedPythonVersion)
	} else if parsed.LT(eolPythonVersion) {
		fmt.Fprintf(os.Stderr, "Python %d.%d is approaching EOL and will not be supported in Pulumi soon."+
			" Check %s for more details\n", parsed.Major,
			parsed.Minor, eolPythonVersionIssue)
	}
}

func (host *pythonLanguageHost) InstallDependencies(
	req *pulumirpc.InstallDependenciesRequest, server pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	opts, err := parseOptions(req.Info.RootDirectory, req.Info.Options.AsMap())
	if err != nil {
		return err
	}

	closer, stdout, stderr, err := rpcutil.MakeInstallDependenciesStreams(server, req.IsTerminal)
	if err != nil {
		return err
	}
	// best effort close, but we try an explicit close and error check at the end as well
	defer closer.Close()

	stdout.Write([]byte("Installing dependencies...\n\n"))

	if err := python.InstallDependenciesWithWriters(server.Context(),
		req.Info.ProgramDirectory, opts.virtualenvPath, true /*showOutput*/, stdout, stderr); err != nil {
		return err
	}

	stdout.Write([]byte("Finished installing dependencies\n\n"))

	return closer.Close()
}

func (host *pythonLanguageHost) About(ctx context.Context, req *emptypb.Empty) (*pulumirpc.AboutResponse, error) {
	errCouldNotGet := func(err error) (*pulumirpc.AboutResponse, error) {
		return nil, fmt.Errorf("failed to get version: %w", err)
	}

	var cmd *exec.Cmd
	// if CommandPath has an error, then so will Command. The error can
	// therefore be ignored as redundant.
	pyexe, _, _ := python.CommandPath()
	cmd, err := python.Command(ctx, "--version")
	if err != nil {
		return nil, err
	}
	var out []byte
	if out, err = cmd.Output(); err != nil {
		return errCouldNotGet(err)
	}
	version := strings.TrimSpace(strings.TrimPrefix(string(out), "Python "))

	return &pulumirpc.AboutResponse{
		Executable: pyexe,
		Version:    version,
	}, nil
}

// Calls a python command as pulumi would. This means we need to accommodate for
// a virtual environment if it exists.
func (host *pythonLanguageHost) callPythonCommand(
	ctx context.Context, virtualenvPath string, args ...string,
) (string, error) {
	if virtualenvPath == "" {
		return callPythonCommandNoEnvironment(ctx, args...)
	}
	// We now know that a virtual environment exists.
	cmd := python.VirtualEnvCommand(virtualenvPath, "python", args...)
	result, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(result), nil
}

// Call a python command in a runtime agnostic way. Call python from the path.
// Do not use a virtual environment.
func callPythonCommandNoEnvironment(ctx context.Context, args ...string) (string, error) {
	cmd, err := python.Command(ctx, args...)
	if err != nil {
		return "", err
	}

	var result []byte
	if result, err = cmd.Output(); err != nil {
		return "", err
	}
	return string(result), nil
}

type pipDependency struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (host *pythonLanguageHost) GetProgramDependencies(
	ctx context.Context, req *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	opts, err := parseOptions(req.Info.RootDirectory, req.Info.Options.AsMap())
	if err != nil {
		return nil, err
	}

	cmdArgs := []string{"-m", "pip", "list", "--format=json"}
	if !req.TransitiveDependencies {
		cmdArgs = append(cmdArgs, "--not-required")
	}
	out, err := host.callPythonCommand(ctx, opts.virtualenvPath, cmdArgs...)
	if err != nil {
		return nil, err
	}
	var result []pipDependency
	err = json.Unmarshal([]byte(out), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse \"python %s\" result: %w", strings.Join(cmdArgs, " "), err)
	}

	dependencies := make([]*pulumirpc.DependencyInfo, len(result))
	for i, dep := range result {
		dependencies[i] = &pulumirpc.DependencyInfo{
			Name:    dep.Name,
			Version: dep.Version,
		}
	}

	return &pulumirpc.GetProgramDependenciesResponse{
		Dependencies: dependencies,
	}, nil
}

func (host *pythonLanguageHost) RunPlugin(
	req *pulumirpc.RunPluginRequest, server pulumirpc.LanguageRuntime_RunPluginServer,
) error {
	logging.V(5).Infof("Attempting to run python plugin in %s", req.Info.ProgramDirectory)

	opts, err := parseOptions(req.Info.RootDirectory, req.Info.Options.AsMap())
	if err != nil {
		return err
	}

	args := []string{req.Info.ProgramDirectory}
	args = append(args, req.Args...)

	var cmd *exec.Cmd
	var virtualenv string
	if opts.virtualenv != "" {
		virtualenv = opts.virtualenvPath
		if !python.IsVirtualEnv(virtualenv) {
			return python.NewVirtualEnvError(opts.virtualenv, virtualenv)
		}
		cmd = python.VirtualEnvCommand(virtualenv, "python", args...)
	} else {
		var err error
		cmd, err = python.Command(server.Context(), args...)
		if err != nil {
			return err
		}
	}

	closer, stdout, stderr, err := rpcutil.MakeRunPluginStreams(server, false)
	if err != nil {
		return err
	}
	// best effort close, but we try an explicit close and error check at the end as well
	defer closer.Close()

	cmd.Dir = req.Pwd
	cmd.Env = req.Env
	cmd.Stdout, cmd.Stderr = stdout, stderr

	if virtualenv != "" {
		cmd.Env = python.ActivateVirtualEnv(cmd.Env, virtualenv)
	}
	if err = cmd.Run(); err != nil {
		var exiterr *exec.ExitError
		if errors.As(err, &exiterr) {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				return server.Send(&pulumirpc.RunPluginResponse{
					Output: &pulumirpc.RunPluginResponse_Exitcode{Exitcode: int32(status.ExitStatus())},
				})
			}
			return fmt.Errorf("program exited unexpectedly: %w", exiterr)
		}
		return fmt.Errorf("problem executing plugin program (could not run language executor): %w", err)
	}

	return closer.Close()
}

func (host *pythonLanguageHost) GenerateProject(
	ctx context.Context, req *pulumirpc.GenerateProjectRequest,
) (*pulumirpc.GenerateProjectResponse, error) {
	loader, err := schema.NewLoaderClient(req.LoaderTarget)
	if err != nil {
		return nil, err
	}

	var extraOptions []pcl.BindOption
	if !req.Strict {
		extraOptions = append(extraOptions, pcl.NonStrictBindOptions()...)
	}

	// for python, prefer output-versioned invokes
	extraOptions = append(extraOptions, pcl.PreferOutputVersionedInvokes)

	program, diags, err := pcl.BindDirectory(req.SourceDirectory, loader, extraOptions...)
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

	err = codegen.GenerateProject(req.TargetDirectory, project, program, req.LocalDependencies)
	if err != nil {
		return nil, err
	}

	rpcDiagnostics := plugin.HclDiagnosticsToRPCDiagnostics(diags)

	return &pulumirpc.GenerateProjectResponse{
		Diagnostics: rpcDiagnostics,
	}, nil
}

func (host *pythonLanguageHost) GenerateProgram(
	ctx context.Context, req *pulumirpc.GenerateProgramRequest,
) (*pulumirpc.GenerateProgramResponse, error) {
	loader, err := schema.NewLoaderClient(req.LoaderTarget)
	if err != nil {
		return nil, err
	}

	parser := hclsyntax.NewParser()
	// Load all .pp files in the directory
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

	program, diags, err := pcl.BindProgram(parser.Files,
		pcl.Loader(loader),
		pcl.PreferOutputVersionedInvokes)
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
		return nil, fmt.Errorf("internal error program was nil")
	}

	files, diags, err := codegen.GenerateProgram(program)
	if err != nil {
		return nil, err
	}
	rpcDiagnostics = append(rpcDiagnostics, plugin.HclDiagnosticsToRPCDiagnostics(diags)...)

	return &pulumirpc.GenerateProgramResponse{
		Source:      files,
		Diagnostics: rpcDiagnostics,
	}, nil
}

func (host *pythonLanguageHost) GeneratePackage(
	ctx context.Context, req *pulumirpc.GeneratePackageRequest,
) (*pulumirpc.GeneratePackageResponse, error) {
	loader, err := schema.NewLoaderClient(req.LoaderTarget)
	if err != nil {
		return nil, err
	}

	var spec schema.PackageSpec
	err = json.Unmarshal([]byte(req.Schema), &spec)
	if err != nil {
		return nil, err
	}

	pkg, diags, err := schema.BindSpec(spec, loader)
	if err != nil {
		return nil, err
	}
	rpcDiagnostics := plugin.HclDiagnosticsToRPCDiagnostics(diags)
	if diags.HasErrors() {
		return &pulumirpc.GeneratePackageResponse{
			Diagnostics: rpcDiagnostics,
		}, nil
	}
	files, err := codegen.GeneratePackage("pulumi-language-python", pkg, req.ExtraFiles)
	if err != nil {
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
