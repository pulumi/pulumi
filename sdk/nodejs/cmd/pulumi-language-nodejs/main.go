// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

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

	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/util/rpcutil"
	"github.com/pulumi/pulumi/pkg/version"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
	"google.golang.org/grpc"
)

const (
	// The path to the "run" program which will spawn the rest of the language host. This may be overriden with
	// PULUMI_LANGUAGE_NODEJS_RUN_PATH, which we do in some testing cases.
	defaultRunPath = "./node_modules/@pulumi/pulumi/cmd/run"

	// The runtime expects the config object to be saved to this environment variable.
	pulumiConfigVar = "PULUMI_CONFIG"
)

// Launches the language host RPC endpoint, which in turn fires
// up an RPC server implementing the LanguageRuntimeServer RPC
// endpoint.
func main() {
	var tracing string
	flag.StringVar(&tracing, "tracing", "",
		"Emit tracing to a Zipkin-compatible tracing endpoint")

	flag.Parse()
	args := flag.Args()
	logging.InitLogging(false, 0, false)
	cmdutil.InitTracing("pulumi-language-nodejs", "pulumi-langauge-nodejs", tracing)

	nodePath, err := exec.LookPath("node")
	if err != nil {
		cmdutil.Exit(errors.Wrapf(err, "could not find node on the $PATH"))
	}

	runPath := os.Getenv("PULUMI_LANGUAGE_NODEJS_RUN_PATH")
	if runPath == "" {
		runPath = defaultRunPath
	}

	if _, err = os.Stat(runPath); err != nil {
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
			host := newLanguageHost(nodePath, runPath, engineAddress, tracing)
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

// nodeLanguageHost implements the LanguageRuntimeServer interface
// for use as an API endpoint.
type nodeLanguageHost struct {
	nodeBin       string
	runPath       string
	engineAddress string
	tracing       string
}

func newLanguageHost(nodePath, runPath, engineAddress, tracing string) pulumirpc.LanguageRuntimeServer {
	return &nodeLanguageHost{
		nodeBin:       nodePath,
		runPath:       runPath,
		engineAddress: engineAddress,
		tracing:       tracing,
	}
}

// GetRequiredPlugins computes the complete set of anticipated plugins required by a program.
func (host *nodeLanguageHost) GetRequiredPlugins(ctx context.Context,
	req *pulumirpc.GetRequiredPluginsRequest) (*pulumirpc.GetRequiredPluginsResponse, error) {
	// To get the plugins required by a program, find all node_modules/ packages that contain { "pulumi": true }
	// inside of their packacge.json files.  We begin this search in the same directory that contains the project.
	// It's possible that a developer would do a `require("../../elsewhere")` and that we'd miss this as a
	// dependency, however the solution for that is simple: install the package in the project root.
	plugins, err := getPluginsFromDir(req.GetProgram(), false)
	if err != nil {
		return nil, err
	}
	return &pulumirpc.GetRequiredPluginsResponse{
		Plugins: plugins,
	}, nil
}

// getPluginsFromDir enumerates all node_modules/ directories, deeply, and returns the fully concatenated results.
func getPluginsFromDir(dir string, inNodeModules bool) ([]*pulumirpc.PluginDependency, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "reading plugin dir %s", dir)
	}

	var plugins []*pulumirpc.PluginDependency
	for _, file := range files {
		name := file.Name()
		curr := filepath.Join(dir, name)

		// Re-stat the directory, in case it is a symlink.
		file, err = os.Stat(curr)
		if err != nil {
			return nil, errors.Wrapf(err, "re-statting file %s", curr)
		}
		if file.IsDir() {
			// if a directory, recurse.
			more, err := getPluginsFromDir(curr, inNodeModules || filepath.Base(dir) == "node_modules")
			if err != nil {
				return nil, err
			}
			plugins = append(plugins, more...)
		} else if inNodeModules && name == "package.json" {
			// if a package.json file within a node_modules package, parse it, and see if it's a source of plugins.
			b, err := ioutil.ReadFile(curr)
			if err != nil {
				return nil, errors.Wrapf(err, "reading package.json %s", curr)
			}
			ok, name, version, err := getPackageInfo(b)
			if err != nil {
				return nil, errors.Wrapf(err, "unmarshaling package.json %s", curr)
			} else if ok {
				plugins = append(plugins, &pulumirpc.PluginDependency{
					Name:    name,
					Kind:    "resource",
					Version: version,
				})
			}
		}
	}
	return plugins, nil
}

// packageJSON is the minimal amount of package.json information we care about.
type packageJSON struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Pulumi  struct {
		Resource bool `json:"resource"`
	} `json:"pulumi"`
}

// getPackageInfo returns a bool indicating whether the given package.json package has an associated Pulumi
// resource provider plugin.  If it does, two strings are returned, the plugin name, and its semantic version.
func getPackageInfo(b []byte) (bool, string, string, error) {
	var info packageJSON
	if err := json.Unmarshal(b, &info); err != nil {
		return false, "", "", err
	}

	if info.Pulumi.Resource {
		name, err := getPluginName(info)
		if err != nil {
			return false, "", "", err
		}
		version, err := getPluginVersion(info)
		if err != nil {
			return false, "", "", err
		}
		return true, name, version, nil
	}

	return false, "", "", nil
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

	ourCmd, err := os.Executable()
	if err != nil {
		err = errors.Wrap(err, "failed to find our working directory")
		return nil, err
	}

	// Older versions of the pulumi runtime used a custom node module (which only worked on node 6.10.X) to support
	// closure serialization. While we no longer use this, we continue to ship this module with the language host in
	// the SDK, so we can deploy programs using older versions of the Pulumi framework.  So, for now, let's add this
	// folder with our native modules to the NODE_PATH so Node can find it.
	//
	// TODO(ellismg)[pulumi/pulumi#1298]: Remove this block of code when we no longer need to support older
	// @pulumi/pulumi versions.
	env := os.Environ()
	existingNodePath := os.Getenv("NODE_PATH")
	if existingNodePath != "" {
		env = append(env, fmt.Sprintf("NODE_PATH=%s/v6.10.2:%s", filepath.Dir(ourCmd), existingNodePath))
	} else {
		env = append(env, "NODE_PATH="+filepath.Dir(ourCmd)+"/v6.10.2")
	}

	env = append(env, pulumiConfigVar+"="+string(config))

	if logging.V(5) {
		commandStr := strings.Join(args, " ")
		logging.V(5).Infoln("Language host launching process: ", host.nodeBin, commandStr)
	}

	// Now simply spawn a process to execute the requested program, wiring up stdout/stderr directly.
	var errResult string
	cmd := exec.Command(host.nodeBin, args...) // nolint: gas, intentionally running dynamic program name.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	if err := cmd.Run(); err != nil {
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
