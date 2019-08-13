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

// pulumi-language-nodejs serves as the "language host" for Pulumi
// programs written in NodeJS. It is ultimately responsible for spawning the
// language runtime that executes the program.
//
// The program being executed is executed by a shim script called
// `pulumi-language-nodejs-exec`. This script is written in the hosted
// language (in this case, node) and is responsible for initiating RPC
// links to the resource monitor and engine.
//
// It's therefore the responsibility of this program to implement
// the LanguageHostServer endpoint by spawning instances of
// `pulumi-language-nodejs-exec` and forwarding the RPC request arguments
// to the command-line.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/hashicorp/go-multierror"

	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/util/rpcutil"
	"github.com/pulumi/pulumi/pkg/version"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
	"google.golang.org/grpc"

	"github.com/blang/semver"
)

const (
	// The path to the "run" program which will spawn the rest of the language host. This may be overridden with
	// PULUMI_LANGUAGE_NODEJS_RUN_PATH, which we do in some testing cases.
	defaultRunPath = "@pulumi/pulumi/cmd/run"

	// The runtime expects the config object to be saved to this environment variable.
	pulumiConfigVar = "PULUMI_CONFIG"

	// A exit-code we recognize when the nodejs process exits.  If we see this error, there's no
	// need for us to print any additional error messages since the user already got a a good
	// one they can handle.
	nodeJSProcessExitedAfterShowingUserActionableMessage = 32
)

// Launches the language host RPC endpoint, which in turn fires
// up an RPC server implementing the LanguageRuntimeServer RPC
// endpoint.
func main() {
	var tracing string
	var typescript bool
	flag.StringVar(&tracing, "tracing", "",
		"Emit tracing to a Zipkin-compatible tracing endpoint")
	flag.BoolVar(&typescript, "typescript", true,
		"Use ts-node at runtime to support typescript source natively")
	flag.Parse()

	args := flag.Args()
	logging.InitLogging(false, 0, false)
	cmdutil.InitTracing("pulumi-language-nodejs", "pulumi-language-nodejs", tracing)

	nodePath, err := exec.LookPath("node")
	if err != nil {
		cmdutil.Exit(errors.Wrapf(err, "could not find node on the $PATH"))
	}

	runPath := os.Getenv("PULUMI_LANGUAGE_NODEJS_RUN_PATH")
	if runPath == "" {
		runPath = defaultRunPath
	}

	runPath, err = locateModule(runPath, nodePath)
	if err != nil {
		cmdutil.ExitError(
			"It looks like the Pulumi SDK has not been installed. Have you run npm install or yarn install?")
	}

	// Optionally pluck out the engine so we can do logging, etc.
	var engineAddress string
	if len(args) > 0 {
		engineAddress = args[0]
	}

	// Fire up a gRPC server, letting the kernel choose a free port.
	port, done, err := rpcutil.Serve(0, nil, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			host := newLanguageHost(nodePath, runPath, engineAddress, tracing, typescript)
			pulumirpc.RegisterLanguageRuntimeServer(srv, host)
			return nil
		},
	})
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

// locateModule resolves a node module name to a file path that can be loaded
func locateModule(mod string, nodePath string) (string, error) {
	program := fmt.Sprintf("console.log(require.resolve('%s'));", mod)
	cmd := exec.Command(nodePath, "-e", program)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// nodeLanguageHost implements the LanguageRuntimeServer interface
// for use as an API endpoint.
type nodeLanguageHost struct {
	nodeBin       string
	runPath       string
	engineAddress string
	tracing       string
	typescript    bool
}

func newLanguageHost(nodePath, runPath, engineAddress,
	tracing string, typescript bool) pulumirpc.LanguageRuntimeServer {
	return &nodeLanguageHost{
		nodeBin:       nodePath,
		runPath:       runPath,
		engineAddress: engineAddress,
		tracing:       tracing,
		typescript:    typescript,
	}
}

func compatibleVersions(a, b semver.Version) (bool, string) {
	switch {
	case a.Major == 0 && b.Major == 0:
		// If both major versions are pre-1.0, we require that the major and minor versions match.
		if a.Minor != b.Minor {
			return false, "Differing major or minor versions are not supported."
		}

	case a.Major >= 1 && b.Major >= 1:
		// If both major versions are post-1.0, we require that the major versions match.
		if a.Major != b.Minor {
			return false, "Differing major versions are not supported."
		}

	case a.Major == 1 && b.Minor == 17 || b.Major == 1 && a.Minor == 17:
		// If one version is pre-1.0 and the other is post-1.0, we unify 1.x.y and 0.17.z. This combination is legal.

	default:
		// All other combinations of versions are illegal.
		return false, "Differing major or minor versions are not supported."
	}

	return true, ""
}

// GetRequiredPlugins computes the complete set of anticipated plugins required by a program.
func (host *nodeLanguageHost) GetRequiredPlugins(ctx context.Context,
	req *pulumirpc.GetRequiredPluginsRequest) (*pulumirpc.GetRequiredPluginsResponse, error) {
	// To get the plugins required by a program, find all node_modules/ packages that contain {
	// "pulumi": true } inside of their package.json files.  We begin this search in the same
	// directory that contains the project. It's possible that a developer would do a
	// `require("../../elsewhere")` and that we'd miss this as a dependency, however the solution
	// for that is simple: install the package in the project root.

	// Keep track of the versions of @pulumi/pulumi that are pulled in.  If they differ on
	// minor version, we will issue a warning to the user.
	pulumiPackagePathToVersionMap := make(map[string]semver.Version)
	plugins, err := getPluginsFromDir(req.GetProgram(), pulumiPackagePathToVersionMap, false /*inNodeModules*/)

	if err == nil {
		first := true
		var firstPath string
		var firstVersion semver.Version
		for path, version := range pulumiPackagePathToVersionMap {
			if first {
				first = false
				firstPath = path
				firstVersion = version
				continue
			}

			if ok, message := compatibleVersions(version, firstVersion); !ok {
				fmt.Fprintf(os.Stderr,
					`Found incompatible versions of @pulumi/pulumi. %s
  Version %s referenced at %s
  Version %s referenced at %s
`, message, firstVersion, firstPath, version, path)
				break
			}
		}
	}

	if err != nil {
		logging.V(3).Infof("one or more errors while discovering plugins: %s", err)
	}
	return &pulumirpc.GetRequiredPluginsResponse{
		Plugins: plugins,
	}, nil
}

// getPluginsFromDir enumerates all node_modules/ directories, deeply, and returns the fully concatenated results.
func getPluginsFromDir(
	dir string, pulumiPackagePathToVersionMap map[string]semver.Version,
	inNodeModules bool) ([]*pulumirpc.PluginDependency, error) {

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "reading plugin dir %s", dir)
	}

	var plugins []*pulumirpc.PluginDependency
	var allErrors *multierror.Error
	for _, file := range files {
		name := file.Name()
		curr := filepath.Join(dir, name)

		// Re-stat the directory, in case it is a symlink.
		file, err = os.Stat(curr)
		if err != nil {
			allErrors = multierror.Append(allErrors, err)
			continue
		}
		if file.IsDir() {
			// if a directory, recurse.
			more, err := getPluginsFromDir(
				curr, pulumiPackagePathToVersionMap, inNodeModules || filepath.Base(dir) == "node_modules")
			if err != nil {
				allErrors = multierror.Append(allErrors, err)
				continue
			}
			plugins = append(plugins, more...)
		} else if inNodeModules && name == "package.json" {
			// if a package.json file within a node_modules package, parse it, and see if it's a source of plugins.
			b, err := ioutil.ReadFile(curr)
			if err != nil {
				allErrors = multierror.Append(allErrors, errors.Wrapf(err, "reading package.json %s", curr))
				continue
			}

			var info packageJSON
			if err := json.Unmarshal(b, &info); err != nil {
				allErrors = multierror.Append(allErrors, errors.Wrapf(err, "unmarshaling package.json %s", curr))
				continue
			}

			if info.Name == "@pulumi/pulumi" {
				version, err := semver.Parse(info.Version)
				if err != nil {
					allErrors = multierror.Append(
						allErrors, errors.Wrapf(err, "Could not understand version %s in '%s'", info.Version, curr))
					continue
				}

				pulumiPackagePathToVersionMap[curr] = version
			}

			ok, name, version, server, err := getPackageInfo(info)
			if err != nil {
				allErrors = multierror.Append(allErrors, errors.Wrapf(err, "unmarshaling package.json %s", curr))
			} else if ok {
				plugins = append(plugins, &pulumirpc.PluginDependency{
					Name:    name,
					Kind:    "resource",
					Version: version,
					Server:  server,
				})
			}
		}
	}
	return plugins, allErrors.ErrorOrNil()
}

// packageJSON is the minimal amount of package.json information we care about.
type packageJSON struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Pulumi  struct {
		Resource bool   `json:"resource"`
		Server   string `json:"server"`
	} `json:"pulumi"`
}

// getPackageInfo returns a bool indicating whether the given package.json package has an associated Pulumi
// resource provider plugin.  If it does, thre strings are returned, the plugin name, and its semantic version and
// an optional server that can be used to download the plugin (this may be empty, in which case the "default" location
// should be used).
func getPackageInfo(info packageJSON) (bool, string, string, string, error) {
	if info.Pulumi.Resource {
		name, err := getPluginName(info)
		if err != nil {
			return false, "", "", "", err
		}
		version, err := getPluginVersion(info)
		if err != nil {
			return false, "", "", "", err
		}
		return true, name, version, info.Pulumi.Server, nil
	}

	return false, "", "", "", nil
}

// getPluginName takes a parsed package.json file and returns the corresponding Pulumi plugin name.
func getPluginName(info packageJSON) (string, error) {
	name := info.Name
	if name == "" {
		return "", errors.New("missing expected \"name\" property")
	}

	// If the name has a @pulumi scope, we will just use its simple name.  Otherwise, we use the fullly scoped name.
	// We do trim the leading @, however, since Pulumi resource providers do not use the same NPM convention.
	if strings.Index(name, "@pulumi/") == 0 {
		return name[strings.IndexRune(name, '/')+1:], nil
	}
	if strings.IndexRune(name, '@') == 0 {
		return name[1:], nil
	}
	return name, nil
}

// getPluginVersion takes a parsed package.json file and returns the semantic version of the Pulumi plugin.
func getPluginVersion(info packageJSON) (string, error) {
	version := info.Version
	if version == "" {
		return "", errors.New("Missing expected \"version\" property")
	}
	if strings.IndexRune(version, 'v') != 0 {
		return fmt.Sprintf("v%s", version), nil
	}
	return version, nil
}

// RPC endpoint for LanguageRuntimeServer::Run
func (host *nodeLanguageHost) Run(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	args := host.constructArguments(req)
	config, err := host.constructConfig(req)
	if err != nil {
		err = errors.Wrap(err, "failed to serialize configuration")
		return nil, err
	}

	env := os.Environ()
	env = append(env, pulumiConfigVar+"="+config)

	if host.typescript {
		env = append(env, "PULUMI_NODEJS_TYPESCRIPT=true")
	}

	if logging.V(5) {
		commandStr := strings.Join(args, " ")
		logging.V(5).Infoln("Language host launching process: ", host.nodeBin, commandStr)
	}

	// Now simply spawn a process to execute the requested program, wiring up stdout/stderr directly.
	var errResult string
	cmd := exec.Command(host.nodeBin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		// NodeJS stdout is complicated enough that we should explicitly flush stdout and stderr here. NodeJS does
		// process writes using console.out and console.err synchronously, but it does not process writes using
		// `process.stdout.write` or `process.stderr.write` synchronously, and it is possible that there exist unflushed
		// writes on those file descriptors at the time that the Node process exits.
		//
		// Because of this, we explicitly flush stdout and stderr so that we are absolutely sure that we capture any
		// error messages in the engine.
		contract.IgnoreError(os.Stdout.Sync())
		contract.IgnoreError(os.Stderr.Sync())
		if exiterr, ok := err.(*exec.ExitError); ok {
			// If the program ran, but exited with a non-zero error code.  This will happen often,
			// since user errors will trigger this.  So, the error message should look as nice as
			// possible.
			if status, stok := exiterr.Sys().(syscall.WaitStatus); stok {
				// Check if we got special exit code that means "we already gave the user an
				// actionable message". In that case, we can simply bail out and terminate `pulumi`
				// without showing any more messages.
				if status.ExitStatus() == nodeJSProcessExitedAfterShowingUserActionableMessage {
					return &pulumirpc.RunResponse{Error: "", Bail: true}, nil
				}

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

// constructArguments constructs a command-line for `pulumi-language-nodejs`
// by enumerating all of the optional and non-optional arguments present
// in a RunRequest.
func (host *nodeLanguageHost) constructArguments(req *pulumirpc.RunRequest) []string {
	args := []string{host.runPath}
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
	if req.GetDryRun() {
		args = append(args, "--dry-run")
	}

	maybeAppendArg("query-mode", fmt.Sprint(req.GetQueryMode()))
	maybeAppendArg("parallel", fmt.Sprint(req.GetParallel()))
	maybeAppendArg("tracing", host.tracing)
	if req.GetProgram() == "" {
		// If the program path is empty, just use "."; this will cause Node to try to load the default module
		// file, by default ./index.js, but possibly overridden in the "main" element inside of package.json.
		args = append(args, ".")
	} else {
		args = append(args, req.GetProgram())
	}

	args = append(args, req.GetArgs()...)
	return args
}

// constructConfig json-serializes the configuration data given as part of
// a RunRequest.
func (host *nodeLanguageHost) constructConfig(req *pulumirpc.RunRequest) (string, error) {
	configMap := req.GetConfig()
	if configMap == nil {
		return "{}", nil
	}

	// While we transition from the old format for config keys (<package>:config:<name> to <package>:<name>), we want
	// to support the newest version of the langhost running older packages, so the config bag we present to them looks
	// like the old world. Newer versions of the @pulumi/pulumi package handle both formats and when we stop supporting
	// older versions, we can remove this code.
	transformedConfig := make(map[string]string, len(configMap))
	for k, v := range configMap {
		pk, err := config.ParseKey(k)
		if err != nil {
			return "", err
		}
		transformedConfig[pk.Namespace()+":config:"+pk.Name()] = v
	}

	configJSON, err := json.Marshal(transformedConfig)
	if err != nil {
		return "", err
	}

	return string(configJSON), nil
}

func (host *nodeLanguageHost) GetPluginInfo(ctx context.Context, req *pbempty.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: version.Version,
	}, nil
}
