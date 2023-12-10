// Copyright 2016-2022, Pulumi Corporation.
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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/blang/semver"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/google/shlex"
	"github.com/hashicorp/go-multierror"
	opentracing "github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/nodejs/npm"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	hclsyntax "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	codegen "github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

const (
	// The path to the "run" program which will spawn the rest of the language host. This may be overridden with
	// PULUMI_LANGUAGE_NODEJS_RUN_PATH, which we do in some testing cases.
	defaultRunPath = "@pulumi/pulumi/cmd/run"

	// The runtime expects the config object to be saved to this environment variable.
	pulumiConfigVar = "PULUMI_CONFIG"

	// The runtime expects the array of secret config keys to be saved to this environment variable.
	//nolint:gosec
	pulumiConfigSecretKeysVar = "PULUMI_CONFIG_SECRET_KEYS"

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
	flag.StringVar(&tracing, "tracing", "",
		"Emit tracing to a Zipkin-compatible tracing endpoint")
	flag.Bool("typescript", true,
		"[obsolete] Use ts-node at runtime to support typescript source natively")
	flag.String("root", "", "[obsolete] Project root path to use")
	flag.String("tsconfig", "",
		"[obsolete] Path to tsconfig.json to use")
	flag.String("nodeargs", "", "[obsolete] Arguments for the Node process")
	flag.Parse()

	args := flag.Args()
	logging.InitLogging(false, 0, false)
	cmdutil.InitTracing("pulumi-language-nodejs", "pulumi-language-nodejs", tracing)

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
		cancel() // deregister the interrupt handler
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
			host := newLanguageHost(engineAddress, tracing)
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

// locateModule resolves a node module name to a file path that can be loaded
func locateModule(ctx context.Context, mod, programDir, nodeBin string) (string, error) {
	args := []string{"-e", fmt.Sprintf("console.log(require.resolve('%s'));", mod)}

	tracingSpan, _ := opentracing.StartSpanFromContext(ctx,
		"locateModule",
		opentracing.Tag{Key: "module", Value: mod},
		opentracing.Tag{Key: "component", Value: "exec.Command"},
		opentracing.Tag{Key: "command", Value: nodeBin},
		opentracing.Tag{Key: "args", Value: args})

	defer tracingSpan.Finish()

	cmd := exec.Command(nodeBin, args...)
	cmd.Dir = programDir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// nodeLanguageHost implements the LanguageRuntimeServer interface
// for use as an API endpoint.
type nodeLanguageHost struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	engineAddress string
	tracing       string
}

type nodeOptions struct {
	// Use ts-node at runtime to support typescript source natively
	typescript bool
	// Path to tsconfig.json to use
	tsconfigpath string
	// Arguments for the Node process
	nodeargs string
}

func parseOptions(root string, options map[string]interface{}) (nodeOptions, error) {
	// typescript defaults to true
	nodeOptions := nodeOptions{
		typescript: true,
	}

	if typescript, ok := options["typescript"]; ok {
		if ts, ok := typescript.(bool); ok {
			nodeOptions.typescript = ts
		} else {
			return nodeOptions, errors.New("typescript option must be a boolean")
		}
	}

	if tsconfigpath, ok := options["tsconfig"]; ok {
		if tsconfig, ok := tsconfigpath.(string); ok {
			nodeOptions.tsconfigpath = tsconfig
		} else {
			return nodeOptions, errors.New("tsconfigpath option must be a string")
		}
	}

	if nodeargs, ok := options["nodeargs"]; ok {
		if args, ok := nodeargs.(string); ok {
			nodeOptions.nodeargs = args
		} else {
			return nodeOptions, errors.New("nodeargs option must be a string")
		}
	}

	return nodeOptions, nil
}

func newLanguageHost(
	engineAddress, tracing string,
) pulumirpc.LanguageRuntimeServer {
	return &nodeLanguageHost{
		engineAddress: engineAddress,
		tracing:       tracing,
	}
}

func compatibleVersions(a, b semver.Version) (bool, string) {
	switch {
	case a.Major == 0 && b.Major == 0:
		// If both major versions are pre-1.0, we require that the major and minor versions match.
		if a.Minor != b.Minor {
			return false, "Differing major or minor versions are not supported."
		}

	case a.Major >= 1 && a.Major <= 2 && b.Major >= 1 && b.Major <= 2:
		// If both versions are 1.0<=v<=2.0, they are compatible.

	case a.Major > 2 || b.Major > 2:
		// If either version is post-2.0, we require that the major versions match.
		if a.Major != b.Major {
			return false, "Differing major versions are not supported."
		}

	case a.Major == 1 && b.Major == 0 && b.Minor == 17 || b.Major == 1 && a.Major == 0 && a.Minor == 17:
		// If one version is pre-1.0 and the other is post-1.0, we unify 1.x.y and 0.17.z. This combination is legal.

	default:
		// All other combinations of versions are illegal.
		return false, "Differing major or minor versions are not supported."
	}

	return true, ""
}

// GetRequiredPlugins computes the complete set of anticipated plugins required by a program.
func (host *nodeLanguageHost) GetRequiredPlugins(ctx context.Context,
	req *pulumirpc.GetRequiredPluginsRequest,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	// To get the plugins required by a program, find all node_modules/ packages that contain {
	// "pulumi": true } inside of their package.json files.  We begin this search in the same
	// directory that contains the project. It's possible that a developer would do a
	// `require("../../elsewhere")` and that we'd miss this as a dependency, however the solution
	// for that is simple: install the package in the project root.

	// Keep track of the versions of @pulumi/pulumi that are pulled in.  If they differ on
	// minor version, we will issue a warning to the user.
	pulumiPackagePathToVersionMap := make(map[string]semver.Version)
	plugins, err := getPluginsFromDir(
		req.Info.ProgramDirectory,
		pulumiPackagePathToVersionMap,
		false, /*inNodeModules*/
		make(map[string]struct{}))

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
	inNodeModules bool, visitedPaths map[string]struct{},
) ([]*pulumirpc.PluginDependency, error) {
	// try to absolute the input path so visitedPaths can track it correctly
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("getting full path for plugin dir %s: %w", dir, err)
	}

	if _, has := visitedPaths[dir]; has {
		return nil, nil
	}
	visitedPaths[dir] = struct{}{}

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading plugin dir %s: %w", dir, err)
	}

	var plugins []*pulumirpc.PluginDependency
	var allErrors *multierror.Error
	for _, file := range files {
		name := file.Name()
		curr := filepath.Join(dir, name)
		isDir := file.IsDir()

		// if this is a symlink resolve it so our visitedPaths can track recursion
		if (file.Type() & fs.ModeSymlink) != 0 {
			symlink, err := filepath.EvalSymlinks(curr)
			if err != nil {
				allErrors = multierror.Append(allErrors, fmt.Errorf("resolving link in plugin dir %s: %w", curr, err))
				continue
			}
			curr = symlink

			// And re-stat the directory to get the resolved mode bits
			fi, err := os.Stat(curr)
			if err != nil {
				allErrors = multierror.Append(allErrors, err)
				continue
			}
			isDir = fi.IsDir()
		}

		if isDir {
			// if a directory, recurse.
			more, err := getPluginsFromDir(
				curr,
				pulumiPackagePathToVersionMap,
				inNodeModules || filepath.Base(dir) == "node_modules",
				visitedPaths)
			if err != nil {
				allErrors = multierror.Append(allErrors, err)
			}
			// Even if there was an error, still append any plugins found in the dir.
			plugins = append(plugins, more...)
		} else if inNodeModules && name == "package.json" {
			// if a package.json file within a node_modules package, parse it, and see if it's a source of plugins.
			b, err := os.ReadFile(curr)
			if err != nil {
				allErrors = multierror.Append(allErrors, fmt.Errorf("reading package.json %s: %w", curr, err))
				continue
			}

			var info packageJSON
			if err := json.Unmarshal(b, &info); err != nil {
				allErrors = multierror.Append(allErrors, fmt.Errorf("unmarshaling package.json %s: %w", curr, err))
				continue
			}

			if info.Name == "@pulumi/pulumi" {
				version, err := semver.Parse(info.Version)
				if err != nil {
					allErrors = multierror.Append(
						allErrors, fmt.Errorf("Could not understand version %s in '%s': %w", info.Version, curr, err))
					continue
				}

				pulumiPackagePathToVersionMap[curr] = version
			}

			ok, name, version, server, err := getPackageInfo(info)
			if err != nil {
				allErrors = multierror.Append(allErrors, fmt.Errorf("unmarshaling package.json %s: %w", curr, err))
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
	Name            string                  `json:"name"`
	Version         string                  `json:"version"`
	Pulumi          plugin.PulumiPluginJSON `json:"pulumi"`
	Main            string                  `json:"main"`
	Dependencies    map[string]string       `json:"dependencies"`
	DevDependencies map[string]string       `json:"devDependencies"`
}

// getPackageInfo returns a bool indicating whether the given package.json package has an associated Pulumi
// resource provider plugin.  If it does, three strings are returned, the plugin name, and its semantic version and
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
	// If it's specified in the "pulumi" section, return it as-is.
	if info.Pulumi.Name != "" {
		return info.Pulumi.Name, nil
	}

	// Otherwise, derive it from the top-level package name,
	// only if it has @pulumi scope, otherwise fail.
	name := info.Name
	if name == "" {
		return "", errors.New("missing expected \"name\" property")
	}

	// If the name has a @pulumi scope, we will just use its simple name.  Otherwise, we use the fully scoped name.
	// We do trim the leading @, however, since Pulumi resource providers do not use the same NPM convention.
	if strings.Index(name, "@pulumi/") == 0 {
		return name[strings.IndexRune(name, '/')+1:], nil
	}

	// if the package name does not start with @pulumi it means that it is a third-party package
	// third-party packages _MUST_ have the plugin name in package.json in the pulumi section
	// {
	//    "name": "@third-party-package"
	//    "pulumi": {
	//        "name": "<plugin name>"
	//    }
	// }

	return "", fmt.Errorf("Missing property \"name\" for the third-party plugin '%v' "+
		"inside package.json under the \"pulumi\" section.", name)
}

// getPluginVersion takes a parsed package.json file and returns the semantic version of the Pulumi plugin.
func getPluginVersion(info packageJSON) (string, error) {
	// See if it's specified in the "pulumi" section.
	version := info.Pulumi.Version
	if version == "" {
		// If not, use the top-level package version.
		version = info.Version
		if version == "" {
			return "", errors.New("Missing expected \"version\" property")
		}
	}
	if strings.IndexRune(version, 'v') != 0 {
		return fmt.Sprintf("v%s", version), nil
	}
	return version, nil
}

// When talking to the nodejs runtime we have three parties involved:
//
//  Engine Monitor <==> Language Host (this code) <==> NodeJS sdk runtime.
//
// Instead of having the NodeJS sdk runtime communicating directly with the Engine Monitor, we
// instead have it communicate with us and we send all those messages to the real engine monitor
// itself.  We do that by having ourselves launch our own grpc monitor server and passing the
// address of it to the NodeJS runtime.  As far as the NodeJS sdk runtime is concerned, it is
// communicating directly with the engine.
//
// When NodeJS then communicates back with us over our server, we then forward the messages
// along untouched to the Engine Monitor.  However, we also open an additional *non-grpc*
// channel to allow the sdk runtime to send us messages on.  Specifically, this non-grpc channel
// is used entirely to allow the sdk runtime to make 'invoke' calls in a synchronous fashion.
// This is accomplished by avoiding grpc entirely (which has no facility for synchronous rpc
// calls), and instead operating over a pair of files coordinated between us and the sdk
// runtime. One file is used by it to send us messages, and one file is used by us to send
// messages back.  Because these are just files, nodejs natively supports allowing both sides to
// read and write from each synchronously.
//
// When we receive the sync-invoke messages from the nodejs sdk we deserialize things off of the
// file and then make a synchronous call to the real engine `invoke` monitor endpoint.  Unlike
// nodejs, we have no problem calling this synchronously, and can block until we get the
// response which we can then synchronously send to nodejs.

// Run is the RPC endpoint for LanguageRuntimeServer::Run
func (host *nodeLanguageHost) Run(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	tracingSpan := opentracing.SpanFromContext(ctx)

	// Make a connection to the real monitor that we will forward messages to.
	conn, err := grpc.Dial(
		req.GetMonitorAddress(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, err
	}

	// Make a client around that connection.  We can then make our own server that will act as a
	// monitor for the sdk and forward to the real monitor.
	target := pulumirpc.NewResourceMonitorClient(conn)

	// Channel to control the server lifetime.  Once `Run` finishes, we'll shutdown the server.
	serverCancel := make(chan bool)
	defer func() {
		serverCancel <- true
		close(serverCancel)
	}()

	// Launch the rpc server giving it the real monitor to forward messages to.
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: serverCancel,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceMonitorServer(srv, &monitorProxy{target: target})
			return nil
		},
		Options: rpcutil.OpenTracingServerInterceptorOptions(tracingSpan),
	})
	if err != nil {
		return nil, err
	}

	// Create the pipes we'll use to communicate synchronously with the nodejs process. Once we're
	// done using the pipes clean them up so we don't leave anything around in the user file system.
	pipes, pipesDone, err := createAndServePipes(ctx, target)
	if err != nil {
		return nil, err
	}
	defer pipes.shutdown()

	nodeBin, err := exec.LookPath("node")
	if err != nil {
		cmdutil.Exit(fmt.Errorf("could not find node on the $PATH: %w", err))
	}

	runPath := os.Getenv("PULUMI_LANGUAGE_NODEJS_RUN_PATH")
	if runPath == "" {
		runPath = defaultRunPath
	}

	runPath, err = locateModule(ctx, runPath, req.Info.ProgramDirectory, nodeBin)
	if err != nil {
		cmdutil.ExitError(
			"It looks like the Pulumi SDK has not been installed. Have you run npm install or yarn install?")
	}

	// Channel producing the final response we want to issue to our caller. Will get the result of
	// the actual nodejs process we launch, or any results caused by errors in our server/pipes.
	responseChannel := make(chan *pulumirpc.RunResponse)
	// now, launch the nodejs process and actually run the user code in it.
	go func() {
		defer close(responseChannel)
		responseChannel <- host.execNodejs(
			ctx, req, nodeBin, runPath,
			fmt.Sprintf("127.0.0.1:%d", handle.Port), pipes.directory())
	}()

	// Wait for one of our launched goroutines to signal that we're done.  This might be our proxy
	// (in the case of errors), or the launched nodejs completing (either successfully, or with
	// errors).
	for {
		select {
		case err := <-handle.Done:
			if err != nil {
				return &pulumirpc.RunResponse{Error: err.Error()}, nil
			}
		case err := <-pipesDone:
			if err != nil {
				return &pulumirpc.RunResponse{Error: err.Error()}, nil
			}
		case response := <-responseChannel:
			return response, nil
		}
	}
}

// Launch the nodejs process and wait for it to complete.  Report success or any errors using the
// `responseChannel` arg.
func (host *nodeLanguageHost) execNodejs(ctx context.Context, req *pulumirpc.RunRequest,
	nodeBin, runPath, address, pipesDirectory string,
) *pulumirpc.RunResponse {
	// Actually launch nodejs and process the result of it into an appropriate response object.
	args := host.constructArguments(req, runPath, address, pipesDirectory)
	config, err := host.constructConfig(req)
	if err != nil {
		err = fmt.Errorf("failed to serialize configuration: %w", err)
		return &pulumirpc.RunResponse{Error: err.Error()}
	}
	configSecretKeys, err := host.constructConfigSecretKeys(req)
	if err != nil {
		err = fmt.Errorf("failed to serialize configuration secret keys: %w", err)
		return &pulumirpc.RunResponse{Error: err.Error()}
	}

	env := os.Environ()
	env = append(env, pulumiConfigVar+"="+config)
	env = append(env, pulumiConfigSecretKeysVar+"="+configSecretKeys)

	opts, err := parseOptions(req.Info.RootDirectory, req.Info.Options.AsMap())
	if err != nil {
		return &pulumirpc.RunResponse{Error: err.Error()}
	}

	if opts.typescript {
		env = append(env, "PULUMI_NODEJS_TYPESCRIPT=true")
	}
	if opts.tsconfigpath != "" {
		env = append(env, "PULUMI_NODEJS_TSCONFIG_PATH="+opts.tsconfigpath)
	}

	nodeargs, err := shlex.Split(opts.nodeargs)
	if err != nil {
		return &pulumirpc.RunResponse{Error: err.Error()}
	}
	nodeargs = append(nodeargs, args...)

	if logging.V(5) {
		commandStr := strings.Join(nodeargs, " ")
		logging.V(5).Infoln("Language host launching process: ", nodeBin, commandStr)
	}

	// Now simply spawn a process to execute the requested program, wiring up stdout/stderr directly.
	var errResult string
	// #nosec G204
	cmd := exec.Command(nodeBin, nodeargs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env

	tracingSpan, _ := opentracing.StartSpanFromContext(ctx,
		"execNodejs",
		opentracing.Tag{Key: "component", Value: "exec.Command"},
		opentracing.Tag{Key: "command", Value: nodeBin},
		opentracing.Tag{Key: "args", Value: nodeargs})
	defer tracingSpan.Finish()

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
			switch code := exiterr.ExitCode(); code {
			case 0:
				// This really shouldn't happen, but if it does, we don't want to render "non-zero exit code"
				err = fmt.Errorf("Program exited unexpectedly: %w", exiterr)
			case nodeJSProcessExitedAfterShowingUserActionableMessage:
				// Check if we got special exit code that means "we already gave the user an
				// actionable message". In that case, we can simply bail out and terminate `pulumi`
				// without showing any more messages.
				return &pulumirpc.RunResponse{Error: "", Bail: true}
			default:
				err = fmt.Errorf("Program exited with non-zero exit code: %d", code)
			}
		} else {
			// Otherwise, we didn't even get to run the program.  This ought to never happen unless there's
			// a bug or system condition that prevented us from running the language exec.  Issue a scarier error.
			err = fmt.Errorf("Problem executing program (could not run language executor): %w", err)
		}

		errResult = err.Error()
	}

	// notify our caller of the response we got from the nodejs process.  Note: this is done
	// unilaterally. this is how we signal to nodeLanguageHost.Run that we are done and it can
	// return to its caller.
	return &pulumirpc.RunResponse{Error: errResult}
}

// constructArguments constructs a command-line for `pulumi-language-nodejs`
// by enumerating all of the optional and non-optional arguments present
// in a RunRequest.
func (host *nodeLanguageHost) constructArguments(
	req *pulumirpc.RunRequest, runPath, address, pipesDirectory string,
) []string {
	args := []string{runPath}
	maybeAppendArg := func(k, v string) {
		if v != "" {
			args = append(args, "--"+k, v)
		}
	}

	maybeAppendArg("monitor", address)
	maybeAppendArg("engine", host.engineAddress)
	maybeAppendArg("sync", pipesDirectory)
	maybeAppendArg("organization", req.GetOrganization())
	maybeAppendArg("project", req.GetProject())
	maybeAppendArg("stack", req.GetStack())
	maybeAppendArg("pwd", req.Info.ProgramDirectory)
	if req.GetDryRun() {
		args = append(args, "--dry-run")
	}

	maybeAppendArg("query-mode", strconv.FormatBool(req.GetQueryMode()))
	maybeAppendArg("parallel", strconv.Itoa(int(req.GetParallel())))
	maybeAppendArg("tracing", host.tracing)

	// The engine should always pass a name for entry point, even if its just "." for the program directory.
	args = append(args, req.Info.EntryPoint)

	args = append(args, req.GetArgs()...)
	return args
}

// constructConfig JSON-serializes the configuration data given as part of
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

// constructConfigSecretKeys JSON-serializes the list of keys that contain secret values given as part of
// a RunRequest.
func (host *nodeLanguageHost) constructConfigSecretKeys(req *pulumirpc.RunRequest) (string, error) {
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

func (host *nodeLanguageHost) GetPluginInfo(ctx context.Context, req *pbempty.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: version.Version,
	}, nil
}

func (host *nodeLanguageHost) InstallDependencies(
	req *pulumirpc.InstallDependenciesRequest, server pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	closer, stdout, stderr, err := rpcutil.MakeInstallDependenciesStreams(server, req.IsTerminal)
	if err != nil {
		return err
	}
	// best effort close, but we try an explicit close and error check at the end as well
	defer closer.Close()

	tracingSpan, ctx := opentracing.StartSpanFromContext(server.Context(), "npm-install")
	defer tracingSpan.Finish()

	stdout.Write([]byte("Installing dependencies...\n\n"))

	_, err = npm.Install(ctx, req.Info.ProgramDirectory, false /*production*/, stdout, stderr)
	if err != nil {
		return fmt.Errorf("npm install failed: %w", err)
	}

	stdout.Write([]byte("Finished installing dependencies\n\n"))

	return closer.Close()
}

func (host *nodeLanguageHost) About(ctx context.Context, req *pbempty.Empty) (*pulumirpc.AboutResponse, error) {
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

	node, version, err := getResponse("node", "--version")
	if err != nil {
		return nil, err
	}

	return &pulumirpc.AboutResponse{
		Executable: node,
		Version:    version,
	}, nil
}

// The shape of a `yarn list --json`'s output.
type yarnLock struct {
	Type string       `json:"type"`
	Data yarnLockData `json:"data"`
}

type yarnLockData struct {
	Type  string         `json:"type"`
	Trees []yarnLockTree `json:"trees"`
}

type yarnLockTree struct {
	Name     string         `json:"name"`
	Children []yarnLockTree `json:"children"`
}

func parseYarnLockFile(path string) ([]*pulumirpc.DependencyInfo, error) {
	ex, err := executable.FindExecutable("yarn")
	if err != nil {
		return nil, fmt.Errorf("found %s but no yarn executable: %w", path, err)
	}
	cmdArgs := []string{"list", "--json"}
	cmd := exec.Command(ex, cmdArgs...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run \"%s %s\": %w", ex, strings.Join(cmdArgs, " "), err)
	}

	var lock yarnLock
	if err = json.Unmarshal(out, &lock); err != nil {
		return nil, fmt.Errorf("failed to parse\"%s %s\": %w", ex, strings.Join(cmdArgs, " "), err)
	}
	leafs := lock.Data.Trees

	result := make([]*pulumirpc.DependencyInfo, len(leafs))

	// Has the form name@version
	splitName := func(index int, nameVersion string) (string, string, error) {
		if nameVersion == "" {
			return "", "", fmt.Errorf("expected \"name\" in dependency %d", index)
		}
		split := strings.LastIndex(nameVersion, "@")
		if split == -1 {
			return "", "", fmt.Errorf("failed to parse name and version from %s", nameVersion)
		}
		return nameVersion[:split], nameVersion[split+1:], nil
	}

	for i, v := range leafs {
		name, version, err := splitName(i, v.Name)
		if err != nil {
			return nil, err
		}

		result[i] = &pulumirpc.DependencyInfo{
			Name:    name,
			Version: version,
		}
	}
	return result, nil
}

// Describes the shape of `npm ls --json --depth=0`'s output.
type npmFile struct {
	Name            string                `json:"name"`
	LockFileVersion int                   `json:"lockfileVersion"`
	Requires        bool                  `json:"requires"`
	Dependencies    map[string]npmPackage `json:"dependencies"`
}

// A package in npmFile.
type npmPackage struct {
	Version  string `json:"version"`
	Resolved string `json:"resolved"`
}

func parseNpmLockFile(path string) ([]*pulumirpc.DependencyInfo, error) {
	ex, err := executable.FindExecutable("npm")
	if err != nil {
		return nil, fmt.Errorf("found %s but not npm: %w", path, err)
	}
	cmdArgs := []string{"ls", "--json", "--depth=0"}
	cmd := exec.Command(ex, cmdArgs...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf(`failed to run "%s %s": %w`, ex, strings.Join(cmdArgs, " "), err)
	}
	file := npmFile{}
	if err = json.Unmarshal(out, &file); err != nil {
		return nil, fmt.Errorf(`failed to parse \"%s %s": %w`, ex, strings.Join(cmdArgs, " "), err)
	}
	result := make([]*pulumirpc.DependencyInfo, len(file.Dependencies))
	var i int
	for k, v := range file.Dependencies {
		result[i] = &pulumirpc.DependencyInfo{
			Name:    k,
			Version: v.Version,
		}
		i++
	}
	return result, nil
}

// Intersect a list of packages with the contents of `package.json`. Returns
// only packages that appear in both sets. `path` is used only for error handling.
func crossCheckPackageJSONFile(path string, file []byte,
	packages []*pulumirpc.DependencyInfo,
) ([]*pulumirpc.DependencyInfo, error) {
	var body packageJSON
	if err := json.Unmarshal(file, &body); err != nil {
		return nil, fmt.Errorf("could not parse %s: %w", path, err)
	}
	dependencies := make(map[string]string)
	for k, v := range body.Dependencies {
		dependencies[k] = v
	}
	for k, v := range body.DevDependencies {
		dependencies[k] = v
	}

	// There should be 1 (& only 1) instantiated dependency for each
	// dependency in package.json. We do this because we want to get the
	// actual version (not the range) that exists in lock files.
	result := make([]*pulumirpc.DependencyInfo, len(dependencies))
	i := 0
	for _, v := range packages {
		if _, exists := dependencies[v.Name]; exists {
			result[i] = v
			// Some direct dependencies are also transitive dependencies. We
			// only want to grab them once.
			delete(dependencies, v.Name)
			i++
		}
	}
	return result, nil
}

func (host *nodeLanguageHost) GetProgramDependencies(
	ctx context.Context, req *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	// We get the node dependencies. This requires either a yarn.lock file and the
	// yarn executable, a package-lock.json file and the npm executable. If
	// transitive is false, we also need the package.json file.
	//
	// If we find a yarn.lock file, we assume that yarn is used.
	// Only then do we look for a package-lock.json file.

	// Neither "yarn list" or "npm ls" can describe what packages are required
	//
	// (direct dependencies). Only what packages they have installed (transitive
	// dependencies). This means that to accurately report only direct
	// dependencies, we need to also parse "package.json" and intersect it with
	// reported dependencies.
	var err error
	yarnFile := filepath.Join(req.Info.ProgramDirectory, "yarn.lock")
	npmFile := filepath.Join(req.Info.ProgramDirectory, "package-lock.json")
	packageFile := filepath.Join(req.Info.ProgramDirectory, "package.json")
	var result []*pulumirpc.DependencyInfo

	if _, err = os.Stat(yarnFile); err == nil {
		result, err = parseYarnLockFile(yarnFile)
		if err != nil {
			return nil, err
		}
	} else if _, err = os.Stat(npmFile); err == nil {
		result, err = parseNpmLockFile(npmFile)
		if err != nil {
			return nil, err
		}
	} else if os.IsNotExist(err) {
		return nil, fmt.Errorf("could not find either %s or %s", yarnFile, npmFile)
	} else {
		return nil, fmt.Errorf("could not get node dependency data: %w", err)
	}
	if !req.TransitiveDependencies {
		file, err := os.ReadFile(packageFile)
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("could not find %s. "+
				"Please include this in your report and run "+
				`pulumi about --transitive" to get a list of used packages`,
				packageFile)
		} else if err != nil {
			return nil, fmt.Errorf("could not read %s: %w", packageFile, err)
		}
		result, err = crossCheckPackageJSONFile(packageFile, file, result)
		if err != nil {
			return nil, err
		}
	}
	return &pulumirpc.GetProgramDependenciesResponse{
		Dependencies: result,
	}, nil
}

func (host *nodeLanguageHost) RunPlugin(
	req *pulumirpc.RunPluginRequest, server pulumirpc.LanguageRuntime_RunPluginServer,
) error {
	return errors.New("not supported")
}

func (host *nodeLanguageHost) GenerateProject(
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

	// for nodejs, prefer output-versioned invokes
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

func (host *nodeLanguageHost) GenerateProgram(
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

func (host *nodeLanguageHost) GeneratePackage(
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
	files, err := codegen.GeneratePackage("pulumi-language-nodejs", pkg, req.ExtraFiles)
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

func readPackageJSON(packageJSONPath string) (map[string]interface{}, error) {
	packageJSONData, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return nil, fmt.Errorf("read package.json: %w", err)
	}
	var packageJSON map[string]interface{}
	err = json.Unmarshal(packageJSONData, &packageJSON)
	if err != nil {
		return nil, fmt.Errorf("unmarshal package.json: %w", err)
	}
	return packageJSON, nil
}

func (host *nodeLanguageHost) Pack(ctx context.Context, req *pulumirpc.PackRequest) (*pulumirpc.PackResponse, error) {
	// Verify npm exists and is set up: npm, user login
	npm, err := executable.FindExecutable("npm")
	if err != nil {
		return nil, fmt.Errorf("find npm: %w", err)
	}

	// Annoyingly the engine will call Pack for the core SDK which is not setup in at all the same way as the
	// generated sdks, so we have to detect that and do a big branch to pack it totally differently.
	packageJSON, err := readPackageJSON(filepath.Join(req.PackageDirectory, "package.json"))
	if err != nil {
		return nil, err
	}

	// TODO: We're just going to write to stderr for now, but we should probably have a way to return this
	// directly to the engine hosting us. That's a bit awkward because we need the subprocesses to write to
	// this as well, and the host side of this pipe might be a tty and if it is we want the subprocesses to
	// think they're connected to a tty as well. We've solved this problem before in two slightly different
	// ways for InstallDependencies and RunPlugin, it would be good to come up with a clean way to do this for
	// all these cases.
	writeString := func(s string) error {
		_, err = os.Stderr.Write([]byte(s))
		return err
	}

	if packageJSON["name"] == "@pulumi/pulumi" {
		// This is pretty much a copy of the makefiles build_package. Short term we should see about changing
		// the makefile to just build the nodejs plugin first and then simply invoke "pulumi package
		// pack-sdk". Long term we should try and unify the style of the code sdk with that of generated sdks
		// so we don't need this special case.

		yarn, err := executable.FindExecutable("yarn")
		if err != nil {
			return nil, fmt.Errorf("find yarn: %w", err)
		}

		err = writeString("$ yarn install --frozen-lockfile\n")
		if err != nil {
			return nil, fmt.Errorf("write to output: %w", err)
		}
		yarnInstallCmd := exec.Command(yarn, "install", "--frozen-lockfile")
		yarnInstallCmd.Dir = req.PackageDirectory
		yarnInstallCmd.Stdout = os.Stdout
		yarnInstallCmd.Stderr = os.Stderr
		err = yarnInstallCmd.Run()
		if err != nil {
			return nil, fmt.Errorf("yarn install: %w", err)
		}

		err = writeString("$ yarn run tsc\n")
		if err != nil {
			return nil, fmt.Errorf("write to output: %w", err)
		}
		yarnTscCmd := exec.Command(yarn, "run", "tsc")
		yarnTscCmd.Dir = req.PackageDirectory
		yarnTscCmd.Stdout = os.Stdout
		yarnTscCmd.Stderr = os.Stderr
		err = yarnTscCmd.Run()
		if err != nil {
			return nil, fmt.Errorf("yarn run tsc: %w", err)
		}

		// "tsc" doesn't copy in the "proto" directory of .js files.
		err = fsutil.CopyFile(
			filepath.Join(req.PackageDirectory, "bin", "proto"),
			filepath.Join(req.PackageDirectory, "proto"),
			nil)
		if err != nil {
			return nil, fmt.Errorf("copy proto: %w", err)
		}

	} else {
		// Before we can build the package we need to install it's dependencies.
		err = writeString("$ npm install\n")
		if err != nil {
			return nil, fmt.Errorf("write to output: %w", err)
		}
		npmInstallCmd := exec.Command(npm, "install")
		npmInstallCmd.Dir = req.PackageDirectory
		npmInstallCmd.Stdout = os.Stdout
		npmInstallCmd.Stderr = os.Stderr
		err = npmInstallCmd.Run()
		if err != nil {
			return nil, fmt.Errorf("npm install: %w", err)
		}

		// Pulumi SDKs always define a build command that will run tsc writing to a bin directory.
		// So we can run that, then edit the package.json in that directory, and then pack it.
		err = writeString("$ npm run build\n")
		if err != nil {
			return nil, fmt.Errorf("write to output: %w", err)
		}
		npmBuildCmd := exec.Command(npm, "run", "build")
		npmBuildCmd.Dir = req.PackageDirectory
		npmBuildCmd.Stdout = os.Stdout
		npmBuildCmd.Stderr = os.Stderr
		err = npmBuildCmd.Run()
		if err != nil {
			return nil, fmt.Errorf("npm run build: %w", err)
		}

		// "build" in SDKs isn't setup to copy the package.json to ./bin/
		err = fsutil.CopyFile(
			filepath.Join(req.PackageDirectory, "bin", "package.json"),
			filepath.Join(req.PackageDirectory, "package.json"),
			nil)
		if err != nil {
			return nil, fmt.Errorf("copy package.json: %w", err)
		}
	}

	// Mutate the package.json to replace version with the version we've been given.
	packageJSONPath := filepath.Join(req.PackageDirectory, "bin", "package.json")
	packageJSON, err = readPackageJSON(packageJSONPath)
	if err != nil {
		return nil, err
	}
	packageJSON["version"] = req.Version
	packageJSONData, err := json.Marshal(packageJSON)
	if err != nil {
		return nil, fmt.Errorf("marshal package.json: %w", err)
	}
	err = os.WriteFile(packageJSONPath, packageJSONData, 0o600)
	if err != nil {
		return nil, fmt.Errorf("write package.json: %w", err)
	}

	err = writeString("$ npm pack\n")
	if err != nil {
		return nil, fmt.Errorf("write to output: %w", err)
	}
	var stdoutBuffer bytes.Buffer
	npmPackCmd := exec.Command(npm,
		"pack",
		filepath.Join(req.PackageDirectory, "bin"),
		"--pack-destination", req.DestinationDirectory)
	npmPackCmd.Stdout = &stdoutBuffer
	npmPackCmd.Stderr = os.Stderr
	err = npmPackCmd.Run()
	if err != nil {
		return nil, fmt.Errorf("npm pack: %w", err)
	}

	artifactName := strings.TrimSpace(stdoutBuffer.String())

	return &pulumirpc.PackResponse{
		ArtifactPath: filepath.Join(req.DestinationDirectory, artifactName),
	}, nil
}
