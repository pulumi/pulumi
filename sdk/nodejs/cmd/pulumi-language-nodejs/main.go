// Copyright 2016, Pulumi Corporation.
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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/blang/semver"
	"github.com/google/shlex"
	"github.com/hashicorp/go-multierror"
	opentracing "github.com/opentracing/opentracing-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/v3"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/errutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/tracing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi-internal/netutil"
	"github.com/pulumi/pulumi/sdk/v3/nodejs/npm"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"github.com/pulumi/pulumi/pkg/v3/codegen/cgstrings"
	hclsyntax "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
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
	// need for us to print any additional error messages since the user already got a good
	// one they can handle.
	nodeJSProcessExitedAfterShowingUserActionableMessage = 32

	// The preferred port for debugging the language host.  Defaults to 9229, which is the default
	// port when using the --inspect-brk flag without additional arguments.
	preferredPort = 9229
)

// Launches the language host RPC endpoint, which in turn fires
// up an RPC server implementing the LanguageRuntimeServer RPC
// endpoint.
func main() {
	var tracing string
	flag.StringVar(&tracing, "tracing", "",
		"Emit tracing to a Zipkin-compatible tracing endpoint")
	var runtimeName string
	flag.StringVar(&runtimeName, "runtime", "nodejs",
		"Specify the runtime to use for running TypeScript programs, defaults to `nodejs`")
	showVersion := flag.Bool("version", false, "Print the current plugin version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	args := flag.Args()
	logging.InitLogging(false, 0, false)

	// Use OTel when the CLI provides an OTLP endpoint; fall back to
	// OpenTracing otherwise.  Only one system should be active to avoid
	// duplicate spans.
	otelEndpoint := os.Getenv("PULUMI_OTEL_EXPORTER_OTLP_ENDPOINT")
	if otelEndpoint == "" {
		cmdutil.InitTracing("pulumi-language-nodejs", "pulumi-language-nodejs", tracing)
	} else {
		if err := cmdutil.InitOtelTracing("pulumi-language-nodejs", otelEndpoint); err != nil {
			logging.V(3).Infof("failed to initialize OTel tracing: %v", err)
		}
		defer cmdutil.CloseOtelTracing()
	}

	// Optionally pluck out the engine so we can do logging, etc.
	var engineAddress string
	if len(args) > 0 {
		engineAddress = args[0]
	}

	if runtimeName != "nodejs" && runtimeName != "bun" {
		cmdutil.Exit(fmt.Errorf("unsupported runtime: %s", runtimeName))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	// map the context Done channel to the rpcutil boolean cancel channel
	cancelChannel := make(chan bool)
	go func() {
		<-ctx.Done()
		cancel() // deregister the interrupt handler
		close(cancelChannel)
	}()

	if engineAddress != "" {
		err := rpcutil.Healthcheck(ctx, engineAddress, 5*time.Minute, cancel)
		if err != nil {
			cmdutil.Exit(fmt.Errorf("could not start health check host RPC server: %w", err))
		}
	}

	// Fire up a gRPC server, letting the kernel choose a free port.
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancelChannel,
		Init: func(srv *grpc.Server) error {
			host := newLanguageHost(engineAddress, runtimeName, tracing, otelEndpoint, false /* forceTsc */)
			pulumirpc.RegisterLanguageRuntimeServer(srv, host)
			return nil
		},
		Options: rpcutil.TracingServerInterceptorOptions(nil),
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
func locateModule(ctx context.Context, mod, programDir, nodeBin string, isPlugin bool) (string, error) {
	installCommand := "pulumi install"
	if isPlugin {
		installCommand = "npm install in " + programDir
	}
	script := fmt.Sprintf(`try {
		console.log(require.resolve('%s'));
	} catch (error) {
		if (error.code === 'MODULE_NOT_FOUND') {
			console.error("It looks like the Pulumi SDK has not been installed. Have you run %s?")
		} else {
			console.error(error.message);
		}
		process.exit(1);
	}`, mod, installCommand)
	// The Volta package manager installs shim executables that route to the user's chosen nodejs
	// version. On Windows this does not properly handle arguments with newlines, so we need to
	// ensure that the script is a single line.
	// https://github.com/pulumi/pulumi/issues/16393
	script = strings.ReplaceAll(script, "\n", "")
	args := []string{"-e", script}

	tracingSpan, _ := opentracing.StartSpanFromContext(ctx,
		"locateModule",
		opentracing.Tag{Key: "module", Value: mod},
		opentracing.Tag{Key: "component", Value: "exec.Command"},
		opentracing.Tag{Key: "command", Value: nodeBin},
		opentracing.Tag{Key: "args", Value: args})
	defer tracingSpan.Finish()

	tracer := otel.Tracer("pulumi-language-nodejs")
	_, otelSpan := tracer.Start(ctx, "locateModule",
		trace.WithAttributes(
			attribute.String("module", mod),
			attribute.String("component", "exec.Command"),
			attribute.String("command", nodeBin),
			attribute.StringSlice("args", args),
		))
	defer otelSpan.End()

	cmd := exec.Command(nodeBin, args...)
	cmd.Dir = programDir
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", errors.New(string(ee.Stderr))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// nodeLanguageHost implements the LanguageRuntimeServer interface
// for use as an API endpoint.
type nodeLanguageHost struct {
	pulumirpc.UnsafeLanguageRuntimeServer

	engineAddress string
	runtime       string
	tracing       string
	otelEndpoint  string
	cancelCtx     context.Context    // Cancelled when we receive the `Cancel` RPC
	cancelFunc    context.CancelFunc // Cancel the cancelCtx

	// used by language conformance tests to force TSC usage
	forceTsc bool
}

type nodeOptions struct {
	// Use ts-node at runtime to support typescript source natively
	typescript bool
	// Path to tsconfig.json to use
	tsconfigpath string
	// Arguments for the Node process
	nodeargs string
	// The packagemanger to use to install dependencies.
	// One of auto, npm, yarn, pnpm or bun, defaults to auto.
	packagemanager npm.PackageManagerType
}

func parseOptions(options map[string]any, runtime string) (nodeOptions, error) {
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

	if packagemanager, ok := options["packagemanager"]; ok {
		if pm, ok := packagemanager.(string); ok {
			switch pm {
			case "auto":
				nodeOptions.packagemanager = npm.AutoPackageManager
			case "npm":
				nodeOptions.packagemanager = npm.NpmPackageManager
			case "yarn":
				nodeOptions.packagemanager = npm.YarnPackageManager
			case "pnpm":
				nodeOptions.packagemanager = npm.PnpmPackageManager
			case "bun":
				nodeOptions.packagemanager = npm.BunPackageManager
			default:
				return nodeOptions, fmt.Errorf("packagemanager option must be one of auto, npm, yarn, pnpm or bun, got %q", pm)
			}
		} else {
			return nodeOptions, errors.New("packagemanager option must be a string")
		}
	} else {
		nodeOptions.packagemanager = npm.AutoPackageManager
	}

	if runtime == "bun" {
		// Bun handles TypeScript natively; ts-node is not used, so PULUMI_NODEJS_TYPESCRIPT must not be set.
		if nodeOptions.typescript {
			logging.V(6).Info("bun runtime: ignoring 'typescript' option (bun handles TypeScript natively)")
		}
		nodeOptions.typescript = false
		// Bun resolves tsconfig itself; PULUMI_NODEJS_TSCONFIG_PATH must not be set.
		if nodeOptions.tsconfigpath != "" {
			logging.V(6).Infof("bun runtime: ignoring 'tsconfig' option %q (bun resolves tsconfig itself)",
				nodeOptions.tsconfigpath)
		}
		nodeOptions.tsconfigpath = ""
		if nodeOptions.nodeargs != "" {
			logging.V(6).Infof("bun runtime: ignoring 'nodeargs' option %q (not supported for bun)",
				nodeOptions.nodeargs)
		}
		nodeOptions.nodeargs = ""
		if nodeOptions.packagemanager != npm.BunPackageManager {
			logging.V(6).Info("bun runtime: ignoring 'packagemanager' option (bun always uses bun as package manager)")
		}
		nodeOptions.packagemanager = npm.BunPackageManager
	}

	return nodeOptions, nil
}

func newLanguageHost(
	engineAddress, runtime, tracing, otelEndpoint string, forceTsc bool,
) pulumirpc.LanguageRuntimeServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &nodeLanguageHost{
		engineAddress: engineAddress,
		tracing:       tracing,
		otelEndpoint:  otelEndpoint,
		forceTsc:      forceTsc,
		runtime:       runtime,
		cancelCtx:     ctx,
		cancelFunc:    cancel,
	}
}

func (host *nodeLanguageHost) connectToEngine() (pulumirpc.EngineClient, io.Closer, error) {
	if host.engineAddress == "" {
		return nil, nil, errors.New("when debugging or running explicitly, must call Handshake before Run")
	}

	dialOpts := append(
		rpcutil.TracingInterceptorDialOptions(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	conn, err := grpc.NewClient(host.engineAddress, dialOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("language host could not make connection to engine: %w", err)
	}

	// Make a client around that connection.
	engineClient := pulumirpc.NewEngineClient(conn)
	return engineClient, conn, nil
}

// minBunPulumiSDKVersion is the minimum @pulumi/pulumi version required for the bun runtime.
var minBunPulumiSDKVersion = semver.MustParse("3.226.0")

// checkPulumiSDKVersion verifies that the installed @pulumi/pulumi package meets the minimum version required by the
// bun runtime.
func checkPulumiSDKVersion(ctx context.Context, programDirectory string) error {
	pm, err := npm.ResolvePackageManager(npm.BunPackageManager, programDirectory)
	if err != nil {
		return fmt.Errorf("could not resolve package manager: %w", err)
	}

	packages, err := pm.ListPackages(ctx, programDirectory, true /*transitive*/)
	if err != nil {
		logging.V(5).Infof("skipping @pulumi/pulumi version check: %v", err)
		return nil
	}

	for _, pkg := range packages {
		if pkg.Name == "@pulumi/pulumi" {
			v, err := semver.Parse(pkg.Version)
			if err != nil {
				logging.V(5).Infof("skipping @pulumi/pulumi version check: could not parse version %q: %v",
					pkg.Version, err)
				continue
			}
			if v.LT(minBunPulumiSDKVersion) {
				return fmt.Errorf("the bun runtime requires @pulumi/pulumi >= %s, but %s is installed; "+
					"ensure all copies of @pulumi/pulumi are updated to >= %s and re-run `pulumi install`",
					minBunPulumiSDKVersion, v, minBunPulumiSDKVersion)
			}
		}
	}

	logging.V(5).Info("skipping @pulumi/pulumi version check: package not found in lockfile")
	return nil
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

func (host *nodeLanguageHost) GetRequiredPackages(ctx context.Context,
	req *pulumirpc.GetRequiredPackagesRequest,
) (*pulumirpc.GetRequiredPackagesResponse, error) {
	// To get the plugins required by a program, find all node_modules/ packages that contain {
	// "pulumi": true } inside of their package.json files.  We begin this search in the same
	// directory that contains the project. It's possible that a developer would do a
	// `require("../../elsewhere")` and that we'd miss this as a dependency, however the solution
	// for that is simple: install the package in the project root.

	// Keep track of the versions of @pulumi/pulumi that are pulled in.  If they differ on
	// minor version, we will issue a warning to the user.
	pulumiPackagePathToVersionMap := make(map[string]semver.Version)
	packages, err := getPackagesFromDir(
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
	return &pulumirpc.GetRequiredPackagesResponse{
		Packages: packages,
	}, nil
}

// GetRequiredPlugins computes the complete set of anticipated plugins required by a program.
func (host *nodeLanguageHost) GetRequiredPlugins(ctx context.Context,
	req *pulumirpc.GetRequiredPluginsRequest,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetRequiredPlugins not implemented")
}

// getPackagesFromDir enumerates all node_modules/ directories, deeply, and returns the fully concatenated results.
func getPackagesFromDir(
	dir string, pulumiPackagePathToVersionMap map[string]semver.Version,
	inNodeModules bool, visitedPaths map[string]struct{},
) ([]*pulumirpc.PackageDependency, error) {
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

	var packages []*pulumirpc.PackageDependency
	var allErrors *multierror.Error
	for _, file := range files {
		name := file.Name()
		curr := filepath.Join(dir, name)
		typ := file.Type()
		isDir := file.IsDir()

		// If this is an irregular file on windows it's a junction point which os.ReadDir can follow but
		// "IsDir" will return false.
		if (typ&fs.ModeIrregular) != 0 && runtime.GOOS == "windows" {
			isDir = true
		}

		// if this is a symlink resolve it so our visitedPaths can track recursion
		if (typ & fs.ModeSymlink) != 0 {
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
			// If a directory, recurse into it. However we have to take care to avoid recursing
			// into nested policy packs. The plugins in a policy pack are not dependencies of the
			// program, so we should not include them in the list of plugins to install.
			policyPack, err := workspace.DetectPolicyPackPathAt(curr)
			if err != nil {
				allErrors = multierror.Append(allErrors, err)
				continue
			}
			if policyPack != "" {
				// curr is a policy pack, so we should not recurse into it.
				continue
			}

			more, err := getPackagesFromDir(
				curr,
				pulumiPackagePathToVersionMap,
				inNodeModules || filepath.Base(dir) == "node_modules",
				visitedPaths)
			if err != nil {
				allErrors = multierror.Append(allErrors, err)
			}
			// Even if there was an error, still append any plugins found in the dir.
			packages = append(packages, more...)
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

			ok, name, version, server, parameterization, err := getPackageInfo(info)
			if err != nil {
				allErrors = multierror.Append(allErrors, fmt.Errorf("unmarshaling package.json %s: %w", curr, err))
			} else if ok {
				packages = append(packages, &pulumirpc.PackageDependency{
					Name:             name,
					Kind:             "resource",
					Version:          version,
					Server:           server,
					Parameterization: parameterization,
				})
			}
		}
	}
	return packages, allErrors.ErrorOrNil()
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
func getPackageInfo(info packageJSON) (bool, string, string, string, *pulumirpc.PackageParameterization, error) {
	if info.Pulumi.Resource {
		name, err := getPluginName(info)
		if err != nil {
			return false, "", "", "", nil, err
		}
		version, err := getPluginVersion(info)
		if err != nil {
			return false, "", "", "", nil, err
		}
		var parameterization *pulumirpc.PackageParameterization
		if info.Pulumi.Parameterization != nil {
			parameterization = &pulumirpc.PackageParameterization{
				Name:    info.Pulumi.Parameterization.Name,
				Version: info.Pulumi.Parameterization.Version,
				Value:   info.Pulumi.Parameterization.Value,
			}
		}

		return true, name, version, info.Pulumi.Server, parameterization, nil
	}

	return false, "", "", "", nil, nil
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
		return "v" + version, nil
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

	engineClient, closer, err := host.connectToEngine()
	if err != nil {
		return nil, err
	}
	defer contract.IgnoreClose(closer)

	// Make a connection to the real monitor that we will forward messages to.
	monitorDialOpts := append(
		rpcutil.TracingInterceptorDialOptions(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	conn, err := grpc.NewClient(req.GetMonitorAddress(), monitorDialOpts...)
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
		Options: rpcutil.TracingServerInterceptorOptions(tracingSpan),
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

	runtimeExec := host.runtime
	if runtimeExec == "nodejs" {
		runtimeExec = "node"
	}
	runtimeBin, err := exec.LookPath(runtimeExec)
	if err != nil {
		return &pulumirpc.RunResponse{
			Error: fmt.Sprintf("could not find %s on the $PATH: %s", runtimeExec, err.Error()),
		}, nil
	}

	if host.runtime == "bun" {
		if err := checkPulumiSDKVersion(ctx, req.Info.ProgramDirectory); err != nil {
			return &pulumirpc.RunResponse{Error: err.Error()}, nil
		}
	}

	runPath := os.Getenv("PULUMI_LANGUAGE_NODEJS_RUN_PATH")
	if runPath == "" {
		runPath = defaultRunPath
	}

	// If we're forcing tsc the program directory for running is actually ./bin, we fixup EntryPoint here so execRuntime
	// passes it to nodejs to run.
	if host.forceTsc {
		req.Info.EntryPoint = "bin"
	}

	runPath, err = locateModule(ctx, runPath, req.Info.ProgramDirectory, runtimeBin, false)
	if err != nil {
		return &pulumirpc.RunResponse{Error: err.Error()}, nil
	}

	// Channel producing the final response we want to issue to our caller. Will get the result of
	// the actual nodejs process we launch, or any results caused by errors in our server/pipes.
	responseChannel := make(chan *pulumirpc.RunResponse)
	// now, launch the nodejs process and actually run the user code in it.
	go func() {
		defer close(responseChannel)
		responseChannel <- host.execRuntime(
			ctx, req, engineClient, runtimeBin, runPath,
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

// Launch the runtime process and wait for it to complete.  Report success or any errors using the
// `responseChannel` arg.
func (host *nodeLanguageHost) execRuntime(ctx context.Context, req *pulumirpc.RunRequest,
	engineClient pulumirpc.EngineClient, runtimeBin, runPath, address, pipesDirectory string,
) *pulumirpc.RunResponse {
	// Actually launch nodejs or bun and process the result of it into an appropriate response object.
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

	opts, err := parseOptions(req.Info.Options.AsMap(), host.runtime)
	if err != nil {
		return &pulumirpc.RunResponse{Error: err.Error()}
	}

	if opts.typescript {
		env = append(env, "PULUMI_NODEJS_TYPESCRIPT=true")
	}
	if opts.tsconfigpath != "" {
		env = append(env, "PULUMI_NODEJS_TSCONFIG_PATH="+opts.tsconfigpath)
	}

	var runtimeArgs []string
	runtimeArgs, err = shlex.Split(opts.nodeargs)
	if err != nil {
		return &pulumirpc.RunResponse{Error: err.Error()}
	}

	var port int
	var ready <-chan error // non-nil for bun debug; receives nil (or an error) when inspector is ready
	if req.GetAttachDebugger() {
		var debugArgs, debugEnv []string
		port, debugArgs, debugEnv, ready, err = getDebuggerSetup(host.runtime)
		if err != nil {
			return &pulumirpc.RunResponse{Error: err.Error()}
		}
		runtimeArgs = append(runtimeArgs, debugArgs...)
		env = append(env, debugEnv...)
	}

	if host.runtime == "bun" {
		runtimeArgs = append([]string{"run"}, runtimeArgs...)
	}

	runtimeArgs = append(runtimeArgs, args...)

	if logging.V(5) {
		commandStr := strings.Join(runtimeArgs, " ")
		logging.V(5).Infoln("Language host launching process: ", runtimeBin, commandStr)
	}

	tracingSpan, _ := opentracing.StartSpanFromContext(ctx,
		"execRuntime",
		opentracing.Tag{Key: "component", Value: "exec.Command"},
		opentracing.Tag{Key: "command", Value: runtimeBin},
		opentracing.Tag{Key: "args", Value: runtimeArgs})
	defer tracingSpan.Finish()

	tracer := otel.Tracer("pulumi-language-nodejs")
	ctx, otelSpan := tracer.Start(ctx, "execRuntime",
		trace.WithAttributes(
			attribute.String("component", "exec.Command"),
			attribute.String("command", runtimeBin),
			attribute.StringSlice("args", runtimeArgs),
		))
	defer otelSpan.End()

	carrier := make(propagation.MapCarrier)
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	if traceparent := carrier.Get("traceparent"); traceparent != "" {
		env = append(env, "TRACEPARENT="+traceparent)
	}

	if host.otelEndpoint != "" {
		env = append(env, "PULUMI_OTEL_EXPORTER_OTLP_ENDPOINT="+host.otelEndpoint)
	}

	// Now simply spawn a process to execute the requested program, wiring up stdout/stderr directly.
	var errResult string
	// Cancel if either the Cancel RPC fires (host.cancelCtx) or the gRPC request ends (ctx)
	cmdCtx, cmdCancel := context.WithCancel(host.cancelCtx)
	context.AfterFunc(ctx, cmdCancel)
	cmd := exec.CommandContext(cmdCtx, runtimeBin, runtimeArgs...)
	cmd.Cancel = func() error {
		if err := cmd.Process.Signal(os.Interrupt); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
	cmd.WaitDelay = 5 * time.Second

	logging.V(5).Infof("Constructed NodeJS command to run: %s", cmd)

	// Copy cmd.Stdout to os.Stdout. Nodejs sometimes changes the blocking mode of its stdout/stderr,
	// so it's unsafe to assign cmd.Stdout directly to os.Stdout. See the description of
	// `runWithOutput` for more details.
	cmd.Stdout = struct{ io.Writer }{os.Stdout}
	r, w := io.Pipe()
	cmd.Stderr = w
	// Get a duplicate reader of stderr so that we can both scan it and write it to os.Stderr.
	stderrDup := io.TeeReader(r, os.Stderr)
	sniffer := newOOMSniffer()
	sniffer.Scan(stderrDup)

	cmd.Env = env

	run := func() error {
		err := cmd.Start()
		if err != nil {
			return err
		}
		if req.GetAttachDebugger() {
			if ready != nil {
				select {
				case err := <-ready:
					if err != nil {
						return err
					}
				case <-time.After(10 * time.Second):
					return errors.New("timed out waiting for bun inspector to be ready")
				}
			}
			err := startDebugging(ctx, engineClient, cmd, port, fmt.Sprintf("Pulumi: Program (%s)", host.runtime),
				host.runtime)
			if err != nil {
				return err
			}
		}
		return cmd.Wait()
	}
	if err := run(); err != nil {
		// NodeJS stdout is complicated enough that we should explicitly flush stdout and stderr here. NodeJS does
		// process writes using console.out and console.err synchronously, but it does not process writes using
		// `process.stdout.write` or `process.stderr.write` synchronously, and it is possible that there exist unflushed
		// writes on those file descriptors at the time that the Node process exits.
		//
		// Because of this, we explicitly flush stdout and stderr so that we are absolutely sure that we capture any
		// error messages in the engine.
		contract.IgnoreError(os.Stdout.Sync())
		contract.IgnoreError(os.Stderr.Sync())
		// Close the write end of the pipe to signal to the sniffer that it should stop scanning.
		contract.IgnoreClose(w)
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
				sniffer.Wait()
				if sniffer.Detected() {
					err = fmt.Errorf("Program exited with non-zero exit code: %d. %s", code, sniffer.Message())
				}
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
	maybeAppendArg("root-directory", req.GetInfo().RootDirectory)
	maybeAppendArg("stack", req.GetStack())
	maybeAppendArg("pwd", req.Info.ProgramDirectory)
	if req.GetDryRun() {
		args = append(args, "--dry-run")
	}

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

func (host *nodeLanguageHost) GetPluginInfo(ctx context.Context, req *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
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

	tracingSpan, ctx := opentracing.StartSpanFromContext(server.Context(), "InstallDependencies")
	defer tracingSpan.Finish()

	tracer := otel.Tracer("pulumi-language-nodejs")
	ctx, otelSpan := tracer.Start(ctx, "InstallDependencies",
		trace.WithAttributes(append(tracing.ProgramInfoAttributes(req.Info),
			attribute.Bool("useLanguageVersionTools", req.UseLanguageVersionTools),
			attribute.Bool("isPlugin", req.IsPlugin),
		)...),
	)
	defer otelSpan.End()

	if req.UseLanguageVersionTools {
		// Look for a .nvmrc or .node-version file, install the version specified in it, and set it
		// as the default nodejs version.
		if err := installNodeVersion(req.Info.ProgramDirectory, stdout); err != nil {
			// If no version file is present, or fnm is not installed, we ignore the error and
			// continue.
			if !errors.Is(err, errVersionFileNotFound) && !errors.Is(err, errFnmNotFound) {
				return fmt.Errorf("failed to install node version: %w", err)
			}
		}
	}

	stdout.Write([]byte("Installing dependencies...\n\n"))

	workspaceRoot := req.Info.ProgramDirectory
	newWorkspaceRoot, err := npm.FindWorkspaceRoot(req.Info.ProgramDirectory)
	if err != nil {
		// If we are not in a npm/yarn workspace, we will keep the current directory as the root.
		if !errors.Is(err, npm.ErrNotInWorkspace) {
			return fmt.Errorf("failure while trying to find workspace root: %w", err)
		}
	} else {
		fmt.Fprintf(stdout, "Detected workspace root at %s\n", newWorkspaceRoot)
		workspaceRoot = newWorkspaceRoot
	}

	opts, err := parseOptions(req.Info.Options.AsMap(), host.runtime)
	if err != nil {
		return fmt.Errorf("failed to parse options: %w", err)
	}
	otelSpan.SetAttributes(append(setOptionsAttributes(opts), attribute.String("workspaceRoot", workspaceRoot))...)

	_, err = npm.Install(ctx, opts.packagemanager, workspaceRoot, false /*production*/, stdout, stderr)
	if err != nil {
		return err
	}

	stdout.Write([]byte("Finished installing dependencies\n\n"))

	if host.forceTsc && !req.IsPlugin {
		// If we're forcing tsc for conformance testing this is our chance to run it before actually running the
		// program. We probably want to see about making something like this an explicit "pulumi build" step, but for
		// now shim'ing this in here works well enough for conformance testing. Note that we skip this step when
		// installing dependencies for plugins, as they may not be written in typescript or have tsc configured.
		tscCmd := exec.Command("npx", "tsc")
		tscCmd.Dir = req.Info.ProgramDirectory
		if err := runWithOutput(tscCmd, stdout, stderr); err != nil {
			return fmt.Errorf("failed to run tsc: %w", err)
		}
	}

	return closer.Close()
}

var (
	errVersionFileNotFound = errors.New("version file not found")
	errFnmNotFound         = errors.New("fnm not found")
)

// useFnm checks if the current directory or any of its parents contains a `.nvmrc` or
// `.node-version` file, and if fnm is installed. If both conditions are met, it returns the
// the version string specified in the found version file.
// `.nvmrc` takes precedence over `.node-version`.
// Note that the version string is not necessarily a semver. The version string can have a `v`
// prefix, or be a partial version string like `20` or `20.6`.
func useFnm(cwd string) (string, error) {
	if _, err := exec.LookPath("fnm"); err != nil {
		if !errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf("error while looking for fnm: %w", err)
		}
		// fnm is not installed
		logging.V(9).Infof("Could not find fnm executable")
		return "", errFnmNotFound
	}
	versionFile, err := fsutil.Searchup(cwd, ".nvmrc")
	if err != nil {
		if !errors.Is(err, fsutil.ErrNotFound) {
			return "", fmt.Errorf("error while looking for .nvmrc: %w", err)
		}
		// No .nvmrc file found, look for .node-version
		versionFile, err = fsutil.Searchup(cwd, ".node-version")
		if err != nil {
			if !errors.Is(err, fsutil.ErrNotFound) {
				return "", fmt.Errorf("error while looking for .node-version: %w", err)
			}
			// No .nvmrc or .node-version file found
			return "", errVersionFileNotFound
		}
	}
	versionBytes, err := os.ReadFile(versionFile)
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(string(versionBytes))
	logging.V(9).Infof("Found node version %s in file %s", version, versionFile)
	return version, nil
}

// installNodeVersion installs the node version specified in the `.nvmrc` or `.node-version` file.
// This is meant to be used in container like environments, where we do not run within a shell.
// When running locally in a shell, fnm, nvm and other similar tools already set the node version
// via their shell integration.
func installNodeVersion(cwd string, stdout io.Writer) error {
	version, err := useFnm(cwd)
	if err != nil {
		return err
	}
	if stdout != nil {
		fmt.Fprintf(stdout, "Setting Nodejs version to %s\n", version)
	}
	// `fnm install` always installs the latest available (upstream) matching
	// version if the version is not fully specified. For example, `fnm install
	// 20` will install the latest 20.x.x version, even if an older version of
	// 20.x.x is locally installed. This leads to unnecessary installations.
	//
	// `fnm use --install-if-missing` does what we want, it activates a locally
	// installed version if possible, and only installs a new version if the
	// requested version can't be satisfied locally.
	installCmd := exec.Command("fnm", "use", version, "--install-if-missing")
	// `fnm use` requires a shell setup for fnm to work correctly. Run the
	// command in a temporary shell, if we're not setup for fnm. This is
	// typically the case in container like environments, where we do not run
	// within a shell. Version switching is managed by setting the default
	// version of nodejs below.
	if os.Getenv("FNM_MULTISHELL_PATH") == "" {
		installCmd = exec.Command("bash", "-c", fmt.Sprintf("eval \"$(fnm env --shell bash)\";"+
			"fnm use '%s' --install-if-missing", version)) // #nosec G204
	}
	out, err := installCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install Nodejs version: %v: %s", err, out)
	}
	// Set the requested version as default so it is available at $FNM_DIR/aliases/default/bin
	// This allows us to set the ambient node version for the whole container, without requiring
	// us to pass environment variables to every `node` invocation.
	setDefaultCmd := exec.Command("fnm", "alias", version, "default")
	out, err = setDefaultCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set default Nodejs version: %v: %s", err, out)
	}
	return nil
}

func (host *nodeLanguageHost) RuntimeOptionsPrompts(ctx context.Context,
	req *pulumirpc.RuntimeOptionsRequest,
) (*pulumirpc.RuntimeOptionsResponse, error) {
	var prompts []*pulumirpc.RuntimeOptionPrompt

	// When using bun the package manager is always bun; no prompt is needed.
	if host.runtime != "bun" {
		rawOpts := req.Info.Options.AsMap()
		if _, hasPackagemanager := rawOpts["packagemanager"]; !hasPackagemanager {
			prompts = append(prompts, &pulumirpc.RuntimeOptionPrompt{
				Key:         "packagemanager",
				Description: "The package manager to use for installing dependencies",
				PromptType:  pulumirpc.RuntimeOptionPrompt_STRING,
				Choices:     plugin.MakeExecutablePromptChoices("npm", "pnpm", "yarn", "bun"),
				Default: &pulumirpc.RuntimeOptionPrompt_RuntimeOptionValue{
					PromptType:  pulumirpc.RuntimeOptionPrompt_STRING,
					StringValue: "npm",
				},
			})
		}
	}

	return &pulumirpc.RuntimeOptionsResponse{
		Prompts: prompts,
	}, nil
}

func (host *nodeLanguageHost) Template(ctx context.Context,
	req *pulumirpc.TemplateRequest,
) (*pulumirpc.TemplateResponse, error) {
	return &pulumirpc.TemplateResponse{}, nil
}

func (host *nodeLanguageHost) About(ctx context.Context,
	req *pulumirpc.AboutRequest,
) (*pulumirpc.AboutResponse, error) {
	opts, err := parseOptions(req.Info.Options.AsMap(), host.runtime)
	if err != nil {
		return nil, fmt.Errorf("failed to parse options: %w", err)
	}
	pm, err := npm.ResolvePackageManager(opts.packagemanager, req.Info.ProgramDirectory)
	if err != nil {
		return nil, fmt.Errorf("could not resolve packagemanager: %w", err)
	}

	runtimeExec := host.runtime
	if runtimeExec == "nodejs" {
		runtimeExec = "node"
	}

	runtimeExecutable, err := executable.FindExecutable(runtimeExec)
	if err != nil {
		return nil, fmt.Errorf("could not find executable %q: %w", runtimeExec, err)
	}

	var runtimeVersion, pmVersion string

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		cmd := exec.CommandContext(ctx, runtimeExec, "--version")
		var out []byte
		if out, err = cmd.Output(); err != nil {
			// Don't fail if we could not determine the runtime version
			return nil
		}
		runtimeVersion = strings.TrimSpace(string(out))
		return nil
	})
	g.Go(func() error {
		var err error
		version, err := pm.Version()
		if err != nil {
			// Don't fail if we could not parse package manager version
			return nil
		}
		pmVersion = version.String()
		return nil
	})

	if err := g.Wait(); err != nil {
		return &pulumirpc.AboutResponse{
			Executable: runtimeExecutable,
			Metadata: map[string]string{
				"packagemanager": pm.Name(),
			},
		}, nil
	}

	return &pulumirpc.AboutResponse{
		Executable: runtimeExecutable,
		Version:    runtimeVersion,
		Metadata: map[string]string{
			"packagemanager":        pm.Name(),
			"packagemanagerVersion": pmVersion,
		},
	}, nil
}

func (host *nodeLanguageHost) Handshake(ctx context.Context,
	req *pulumirpc.LanguageHandshakeRequest,
) (*pulumirpc.LanguageHandshakeResponse, error) {
	host.engineAddress = req.EngineAddress

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	// map the context Done channel to the rpcutil boolean cancel channel
	cancelChannel := make(chan bool)
	go func() {
		<-ctx.Done()
		cancel() // deregister the interrupt handler
		close(cancelChannel)
	}()
	err := rpcutil.Healthcheck(ctx, host.engineAddress, 5*time.Minute, cancel)
	if err != nil {
		cmdutil.Exit(fmt.Errorf("could not start health check host RPC server: %w", err))
	}

	return &pulumirpc.LanguageHandshakeResponse{}, nil
}

func (host *nodeLanguageHost) GetProgramDependencies(
	ctx context.Context, req *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	opts, err := parseOptions(req.Info.Options.AsMap(), host.runtime)
	if err != nil {
		return nil, fmt.Errorf("failed to parse options: %w", err)
	}
	pkgManager, err := npm.ResolvePackageManager(opts.packagemanager, req.Info.ProgramDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to determine package manager: %w", err)
	}

	packages, err := pkgManager.ListPackages(ctx, req.Info.ProgramDirectory, req.TransitiveDependencies)
	if err != nil {
		return nil, fmt.Errorf("failed to get node dependencies: %w", err)
	}

	dependencies := make([]*pulumirpc.DependencyInfo, len(packages))
	for i, pkg := range packages {
		dependencies[i] = &pulumirpc.DependencyInfo{
			Name:    pkg.Name,
			Version: pkg.Version,
		}
	}
	return &pulumirpc.GetProgramDependenciesResponse{
		Dependencies: dependencies,
	}, nil
}

func getDebuggerSetup(runtime string) (
	port int, flags []string, env []string, ready <-chan error, err error,
) {
	port, err = netutil.FindNextAvailablePort(preferredPort)
	if err != nil {
		return 0, nil, nil, nil, fmt.Errorf("failed to find available port for debugger: %w", err)
	}
	if runtime == "bun" {
		// BUN_INSPECT sets the inspector WebSocket address and mode (wait=1 starts execution and waits for the debugger
		// to attach).
		// BUN_INSPECT_NOTIFY is a TCP address bun connects to once the inspector is ready.
		ln, lnErr := net.Listen("tcp", "127.0.0.1:0")
		if lnErr != nil {
			return 0, nil, nil, nil, fmt.Errorf("failed to create bun inspect notify listener: %w", lnErr)
		}
		notifyPort := ln.Addr().(*net.TCPAddr).Port
		ch := make(chan error, 1)
		go func() {
			conn, connErr := ln.Accept()
			if connErr != nil {
				ch <- fmt.Errorf("bun inspect notify listener failed: %w", connErr)
			} else {
				contract.IgnoreClose(conn)
				close(ch)
			}
			contract.IgnoreClose(ln)
		}()
		return port, nil, []string{
			fmt.Sprintf("BUN_INSPECT=ws://127.0.0.1:%d?wait=1", port),
			fmt.Sprintf("BUN_INSPECT_NOTIFY=tcp://127.0.0.1:%d", notifyPort),
		}, ch, nil
	}
	return port, []string{
		fmt.Sprintf("--inspect-brk=%d", port),
		// suppress the console output "Debugger listening on..."
		"--inspect-publish-uid=http",
	}, nil, nil, nil
}

func startDebugging(
	ctx context.Context,
	engineClient pulumirpc.EngineClient,
	cmd *exec.Cmd,
	port int,
	name string,
	runtime string,
) error {
	var cfg map[string]any
	if runtime == "bun" {
		cfg = map[string]any{
			"name":    name,
			"type":    "bun",
			"request": "attach",
			"url":     fmt.Sprintf("ws://127.0.0.1:%d", port),
		}
	} else {
		cfg = map[string]any{
			"name":             name,
			"type":             "node",
			"request":          "attach",
			"processId":        cmd.Process.Pid,
			"continueOnAttach": true,
			"skipFiles":        []any{"<node_internals>/**"},
		}
		if port != preferredPort {
			cfg["port"] = port
		}
	}
	debugConfig, err := structpb.NewStruct(cfg)
	if err != nil {
		return err
	}
	message := fmt.Sprintf("on process id %d", cmd.Process.Pid)
	if port != preferredPort {
		message += fmt.Sprintf(" and port %d", port)
	}
	_, err = engineClient.StartDebugging(ctx, &pulumirpc.StartDebuggingRequest{
		Config:  debugConfig,
		Message: message,
	})
	if err != nil {
		return fmt.Errorf("unable to start debugging: %w", err)
	}
	return nil
}

func (host *nodeLanguageHost) RunPlugin2(
	server grpc.BidiStreamingServer[pulumirpc.RunPlugin2Request, pulumirpc.RunPluginResponse],
) error {
	return status.Errorf(codes.Unimplemented, "RunPlugin2 is not yet implemented")
}

func (host *nodeLanguageHost) RunPlugin(
	req *pulumirpc.RunPluginRequest, server pulumirpc.LanguageRuntime_RunPluginServer,
) error {
	logging.V(5).Infof("Attempting to run nodejs plugin in %s", req.Info.ProgramDirectory)
	ctx := server.Context()

	engineClient, closer, err := host.connectToEngine()
	if err != nil {
		return err
	}
	defer closer.Close()

	closer, stdout, stderr, err := rpcutil.MakeRunPluginStreams(server, false)
	if err != nil {
		return err
	}
	// best effort close, but we try an explicit close and error check at the end as well
	defer closer.Close()

	if host.runtime == "bun" {
		if err := checkPulumiSDKVersion(ctx, req.Info.ProgramDirectory); err != nil {
			return err
		}
	}

	runtimeExec := host.runtime
	if runtimeExec == "nodejs" {
		runtimeExec = "node"
	}
	runtimeBin, err := exec.LookPath(runtimeExec)
	if err != nil {
		return err
	}

	opts, err := parseOptions(req.Info.Options.AsMap(), host.runtime)
	if err != nil {
		return err
	}

	env := req.Env
	if opts.typescript {
		env = append(env, "PULUMI_NODEJS_TYPESCRIPT=true")
	}
	if opts.tsconfigpath != "" {
		env = append(env, "PULUMI_NODEJS_TSCONFIG_PATH="+opts.tsconfigpath)
	}

	var runPath string
	if req.Kind == string(apitype.AnalyzerPlugin) {
		// Policy packs (i.e. kind=analyzer) need to be treated specially for back compatibility reasons. We
		// used to have a dedicated shim plugin "pulumi-analyzer-policy" that would start policy packs up, but
		// now we just let the nodejs RunPlugin code handle that logic.
		runPath = "@pulumi/pulumi/cmd/run-policy-pack"
	} else {
		// TODO(https://github.com/pulumi/pulumi/issues/20941): Pretty sure this isn't right for convert/tool plugins.
		runPath = os.Getenv("PULUMI_LANGUAGE_NODEJS_RUN_PATH")
		if runPath == "" {
			runPath = "@pulumi/pulumi/cmd/run-plugin"
		}
	}

	runPath, err = locateModule(ctx, runPath, req.Info.ProgramDirectory, runtimeBin, true)
	if err != nil {
		return err
	}

	var args []string
	var port int
	var ready <-chan error
	if req.GetAttachDebugger() {
		var debugArgs, debugEnv []string
		port, debugArgs, debugEnv, ready, err = getDebuggerSetup(host.runtime)
		if err != nil {
			return err
		}
		args = append(args, debugArgs...)
		env = append(env, debugEnv...)
	}

	if host.runtime == "bun" {
		args = append([]string{"run"}, args...)
	}

	args = append(args, runPath)

	nodeargs, err := shlex.Split(opts.nodeargs)
	if err != nil {
		return err
	}

	args = append(args, nodeargs...)
	// For policy analyzers we need to send the program directory as an argument _last_, for everything else it comes first
	if req.Kind == string(apitype.AnalyzerPlugin) {
		args = append(args, req.Args...)
		args = append(args, req.Info.ProgramDirectory)
	} else {
		args = append(args, req.Info.ProgramDirectory)
		args = append(args, req.Args...)
	}

	// For policy packs we also need to adjust the environment to include stack configuration as well. Unfortunately to
	// get that info we need to connect to the engine as a policy pack so that we can get the StackConfiguration call,
	// we then need to wait for the _real_ policy pack to start up and shim all it's other request/responses.
	var policyPackServer *sdk.PolicyProxy
	if req.Kind == string(apitype.AnalyzerPlugin) {
		policyPackServer, stdout, err = sdk.NewPolicyProxy(ctx, stdout)
		if err != nil {
			return fmt.Errorf("could not start policy pack proxy: %w", err)
		}

		config, err := policyPackServer.AwaitConfiguration(ctx)
		if err != nil {
			return fmt.Errorf("could not get stack configuration: %w", err)
		}

		if config != nil {
			maybeAppendEnv := func(k, v string) {
				if v != "" {
					env = append(env, k+"="+v)
				}
			}
			maybeAppendEnv("PULUMI_NODEJS_ORGANIZATION", config.Organization)
			maybeAppendEnv("PULUMI_NODEJS_PROJECT", config.Project)
			maybeAppendEnv("PULUMI_NODEJS_STACK", config.Stack)
			maybeAppendEnv("PULUMI_NODEJS_DRY_RUN", strconv.FormatBool(config.DryRun))
			if config.Config != nil {
				configJSON, err := json.Marshal(config.Config)
				if err != nil {
					return fmt.Errorf("could not marshal stack configuration: %w", err)
				}
				maybeAppendEnv("PULUMI_CONFIG", string(configJSON))
			}
		}
	}

	// Now simply spawn a process to execute the requested program, wiring up stdout/stderr directly.
	cmd := exec.CommandContext(ctx, runtimeBin, args...)
	// node policy packs used to always run with the working directory set to the policy pack directory, not
	// the main working directory. We need to continue that for backwards compatibility.
	if req.Kind == string(apitype.AnalyzerPlugin) {
		cmd.Dir = req.Info.ProgramDirectory
	} else {
		cmd.Dir = req.Pwd
	}
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = stdout, stderr

	logging.V(5).Infof("Constructed NodeJS command to run: %s", cmd)

	run := func() error {
		if err := cmd.Start(); err != nil {
			return err
		}
		if req.GetAttachDebugger() {
			if ready != nil {
				select {
				case err := <-ready:
					if err != nil {
						return err
					}
				case <-time.After(10 * time.Second):
					return errors.New("timed out waiting for bun inspector to be ready")
				}
			}
			err := startDebugging(ctx, engineClient, cmd, port, fmt.Sprintf("Pulumi: Plugin (%s)", req.Name), host.runtime)
			if err != nil {
				return err
			}
		}
		// If we've got a proxy policy then tell it to attach now
		if policyPackServer != nil {
			err = policyPackServer.Attach(ctx, cmd)
			if err != nil {
				return fmt.Errorf("could not attach policy pack proxy: %w", err)
			}
			return nil
		}
		return cmd.Wait()
	}

	if err := run(); err != nil {
		var exiterr *exec.ExitError
		if errors.As(err, &exiterr) {
			// The program ran, but exited with a non-zero error code.  This will happen often, since user
			// errors will trigger this.  So, the error message should look as nice as possible.
			if status, stok := exiterr.Sys().(syscall.WaitStatus); stok {
				return fmt.Errorf("Program exited with non-zero exit code: %d", status.ExitStatus())
			}
			return fmt.Errorf("Program exited unexpectedly: %w", exiterr)
		}
		// Otherwise, we didn't even get to run the program.  This ought to never happen unless there's
		// a bug or system condition that prevented us from running the language exec.  Issue a scarier error.
		return fmt.Errorf("Problem executing plugin program (could not run language executor): %w", err)
	}

	if err := closer.Close(); err != nil {
		return err
	}

	return nil
}

func (host *nodeLanguageHost) GenerateProject(
	ctx context.Context, req *pulumirpc.GenerateProjectRequest,
) (*pulumirpc.GenerateProjectResponse, error) {
	loader, err := schema.NewLoaderClient(req.LoaderTarget)
	if err != nil {
		return nil, err
	}
	defer loader.Close()

	var extraOptions []pcl.BindOption
	if !req.Strict {
		extraOptions = append(extraOptions, pcl.NonStrictBindOptions()...)
	}

	// for nodejs, prefer output-versioned invokes
	extraOptions = append(extraOptions, pcl.PreferOutputVersionedInvokes)

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

	err = codegen.GenerateProject(req.TargetDirectory, project, program, req.LocalDependencies, host.forceTsc,
		host.runtime)
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
	defer loader.Close()

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

	bindOptions := []pcl.BindOption{
		pcl.Loader(schema.NewCachedLoader(loader)),
		// for nodejs, prefer output-versioned invokes
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

	files, diags, err := codegen.GenerateProgramWithOptions(program, codegen.ProgramOptions{Runtime: host.runtime})
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

	files, err := codegen.GeneratePackage(
		"pulumi-language-nodejs", pkg, req.ExtraFiles, req.LocalDependencies, req.Local, schema.NewCachedLoader(loader))
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

func readPackageJSON(packageJSONPath string) (map[string]any, error) {
	packageJSONData, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return nil, fmt.Errorf("read package.json: %w", err)
	}
	var packageJSON map[string]any
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
		if err := runWithOutput(yarnInstallCmd, os.Stdout, os.Stderr); err != nil {
			return nil, fmt.Errorf("yarn install: %w", err)
		}

		err = writeString("$ yarn run tsc\n")
		if err != nil {
			return nil, fmt.Errorf("write to output: %w", err)
		}
		yarnTscCmd := exec.Command(yarn, "run", "tsc")
		yarnTscCmd.Dir = req.PackageDirectory
		if err := runWithOutput(yarnTscCmd, os.Stdout, os.Stderr); err != nil {
			return nil, fmt.Errorf("yarn run tsc: %w", err)
		}

		// "tsc" doesn't copy in the "proto" and "vendor" directories.
		err = fsutil.CopyFile(
			filepath.Join(req.PackageDirectory, "bin", "proto"),
			filepath.Join(req.PackageDirectory, "proto"),
			nil)
		if err != nil {
			return nil, fmt.Errorf("copy proto: %w", err)
		}
		err = fsutil.CopyFile(
			filepath.Join(req.PackageDirectory, "bin", "vendor"),
			filepath.Join(req.PackageDirectory, "vendor"),
			nil)
		if err != nil {
			return nil, fmt.Errorf("copy vendor: %w", err)
		}
	} else {
		// Before we can build the package we need to install it's dependencies.
		err = writeString("$ npm install\n")
		if err != nil {
			return nil, fmt.Errorf("write to output: %w", err)
		}
		npmInstallCmd := exec.Command(npm, "install")
		npmInstallCmd.Dir = req.PackageDirectory
		if err := runWithOutput(npmInstallCmd, os.Stdout, os.Stderr); err != nil {
			return nil, errutil.ErrorWithStderr(err, "npm install")
		}

		// Pulumi SDKs always define a build command that will run tsc writing to a bin directory.
		// So we can run that, then edit the package.json in that directory, and then pack it.
		err = writeString("$ npm run build\n")
		if err != nil {
			return nil, fmt.Errorf("write to output: %w", err)
		}
		npmBuildCmd := exec.Command(npm, "run", "build")
		npmBuildCmd.Dir = req.PackageDirectory
		if err := runWithOutput(npmBuildCmd, os.Stdout, os.Stderr); err != nil {
			return nil, errutil.ErrorWithStderr(err, "npm run build")
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
	npmPackCmd.Stderr = struct{ io.Writer }{os.Stderr}
	err = npmPackCmd.Run()
	if err != nil {
		return nil, fmt.Errorf("npm pack: %w", err)
	}

	artifactName := strings.TrimSpace(stdoutBuffer.String())

	return &pulumirpc.PackResponse{
		ArtifactPath: filepath.Join(req.DestinationDirectory, artifactName),
	}, nil
}

// Nodejs sometimes sets stdout/stderr to non-blocking mode. When a nodejs subprocess is directly
// handed the go process's stdout/stderr file descriptors, nodejs's non-blocking configuration goes
// unnoticed by go, and a write from go can result in an error `write /dev/stdout: resource
// temporarily unavailable`.
//
// The solution to this is to not provide nodejs with the go process's stdout/stderr file
// descriptors, and instead proxy the writes through something else.
// In https://github.com/pulumi/pulumi/pull/16504 we used Cmd.StdoutPipe/StderrPipe for this.
// However this introduced a potential bug, as it is not safe to use these specific pipes along
// with Cmd.Run. The issue is that these pipes will be closed as soon as the process exits, which
// can lead to missing data when stdout/stderr are slow, or worse an error due to an attempted read
// from a closed pipe. (Creating and closing our own os.Pipes for this would be fine.)
//
// A simpler workaround is to wrap stdout/stderr in an io.Writer. As with the pipes, this makes
// exec.Cmd use a copying goroutine to shuffle the data from the subprocess to stdout/stderr.
//
// Cmd.Run will wait for all the data to be copied before returning, ensuring we do not miss any data.
//
// Non-blocking issue: https://github.com/golang/go/issues/58408#issuecomment-1423621323
// StdoutPipe/StderrPipe issues: https://pkg.go.dev/os/exec#Cmd.StdoutPipe
// Waiting for data: https://cs.opensource.google/go/go/+/refs/tags/go1.22.5:src/os/exec/exec.go;l=201
func runWithOutput(cmd *exec.Cmd, stdout, stderr io.Writer) error {
	cmd.Stdout = struct{ io.Writer }{stdout}
	cmd.Stderr = struct{ io.Writer }{stderr}
	return cmd.Run()
}

// oomSniffer is a scanner that detects OOM errors in the output of a nodejs process.
type oomSniffer struct {
	detected bool
	timeout  time.Duration
	waitChan chan struct{}
}

func newOOMSniffer() *oomSniffer {
	return &oomSniffer{
		timeout:  15 * time.Second,
		waitChan: make(chan struct{}),
	}
}

func (o *oomSniffer) Detected() bool {
	return o.detected
}

// Wait waits for the OOM sniffer to either:
//   - detect an OOM error
//   - the timeout to expire
//   - the reader to be closed
//
// Call Wait to ensure we've read all the output from the scanned process after it exits.
func (o *oomSniffer) Wait() {
	select {
	case <-o.waitChan:
	case <-time.After(o.timeout):
	}
}

func (o *oomSniffer) Message() string {
	return "Detected a possible out of memory error. Consider increasing the memory available to the nodejs process " +
		"by setting the `nodeargs` runtime option in Pulumi.yaml to `nodeargs: --max-old-space-size=<size>` where " +
		"`<size>` is the maximum memory in megabytes that can be allocated to nodejs. " +
		"See https://www.pulumi.com/docs/concepts/projects/project-file/#runtime-options"
}

func (o *oomSniffer) Scan(r io.Reader) {
	scanner := bufio.NewScanner(r)
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			if !o.detected && (strings.Contains(line, "<--- Last few GCs --->") /* "Normal" OOM output */ ||
				// Because we hook into the debugger API, the OOM error message can be obscured by
				// a failed assertion in the debugger https://github.com/pulumi/pulumi/issues/16596.
				//nolint:lll
				// https://github.com/nodejs/node/blob/cef2047b1fdd797d5125c4cafe9f17220a0774f7/deps/v8/src/debug/debug-scopes.cc#L447
				strings.Contains(line, "Check failed: needs_context && current_scope_ == closure_scope_")) {
				o.detected = true
				close(o.waitChan)
			}
		}
		contract.IgnoreError(scanner.Err())
		if !o.detected {
			close(o.waitChan)
		}
	}()
}

func (host *nodeLanguageHost) Link(
	ctx context.Context, req *pulumirpc.LinkRequest,
) (*pulumirpc.LinkResponse, error) {
	logging.V(5).Infof("Linking %+v in %s", req.Packages, req.Info.RootDirectory)
	opts, err := parseOptions(req.Info.Options.AsMap(), host.runtime)
	if err != nil {
		return nil, err
	}

	loader, err := schema.NewLoaderClient(req.LoaderTarget)
	if err != nil {
		return nil, err
	}
	defer loader.Close()
	cachedLoader := schema.NewCachedLoader(loader)

	pm, err := npm.ResolvePackageManager(opts.packagemanager, req.Info.ProgramDirectory)
	if err != nil {
		return nil, err
	}

	// Detect whether this is a typescript project, either run through our builtin ts-node, meaning the typescript
	// runtime option is true, or if there is a tsconfig.json
	isTypeScript := opts.typescript || opts.tsconfigpath != ""
	if !isTypeScript {
		if _, err := fsutil.Searchup(req.Info.ProgramDirectory, "tsconfig.json"); err == nil {
			isTypeScript = true
		}
	}
	// We'll use module syntax (import, export) it is a typescript project, or package.json["type"] is set to "module".
	usesModuleSyntax := isTypeScript
	if !usesModuleSyntax {
		if p, err := npm.SearchupPackageManifest(req.Info.ProgramDirectory); err == nil {
			if data, _, err := npm.ReadPackageManifest(filepath.Dir(p)); err == nil {
				if typ, ok := data["type"]; ok {
					if typeS, ok := typ.(string); ok && typeS == "module" {
						usesModuleSyntax = true
					}
				}
			}
		}
	}

	instructions := "You can import the SDK in your JavaScript code with:\n\n"
	if isTypeScript {
		instructions = "You can import the SDK in your TypeScript code with:\n\n"
	}

	var imports strings.Builder
	for _, dep := range req.Packages {
		var version *semver.Version
		v, err := semver.New(dep.Package.Version)
		if err != nil {
			logging.V(5).Infof("Invalid version %s for package %s", dep.Package.Version, dep.Package.Name)
		} else {
			version = v
		}
		var param *schema.ParameterizationDescriptor
		if dep.Package.Parameterization != nil {
			param = &schema.ParameterizationDescriptor{
				Name:  dep.Package.Parameterization.Name,
				Value: dep.Package.Parameterization.Value,
			}
			if dep.Package.Parameterization.Version != "" {
				v, err := semver.New(dep.Package.Parameterization.Version)
				if err != nil {
					logging.V(5).Infof("Invalid version %s for package %s", dep.Package.Version, dep.Package.Name)
				} else {
					param.Version = *v
				}
			}
		}
		packageDesc := &schema.PackageDescriptor{
			Name:             dep.Package.Name,
			Version:          version,
			DownloadURL:      dep.Package.Server,
			Parameterization: param,
		}
		pkgRef, err := schema.LoadPackageReferenceV2(ctx, cachedLoader, packageDesc)
		if err != nil {
			return nil, err
		}
		pkg, err := pkgRef.Definition()
		if err != nil {
			return nil, err
		}
		if err := pkg.ImportLanguages(map[string]schema.Language{"nodejs": codegen.Importer}); err != nil {
			return nil, err
		}
		packageName, err := getNodeJSPkgName(pkg)
		if err != nil {
			return nil, err
		}
		if err := pm.Link(ctx, req.Info.ProgramDirectory, packageName, dep.Path); err != nil {
			return nil, err
		}

		importName := cgstrings.Camel(pkgRef.Name())
		if usesModuleSyntax {
			imports.WriteString(fmt.Sprintf("  import * as %s from \"%s\";\n", importName, packageName))
		} else {
			imports.WriteString(fmt.Sprintf("  const %s = require(\"%s\");\n", importName, packageName))
		}
	}
	instructions += imports.String()

	return &pulumirpc.LinkResponse{
		ImportInstructions: instructions,
	}, nil
}

func (host *nodeLanguageHost) Cancel(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	host.cancelFunc()
	return &emptypb.Empty{}, nil
}

func getNodeJSPkgName(pkg *schema.Package) (string, error) {
	if info, ok := pkg.Language["nodejs"]; ok {
		if info, ok := info.(nodejs.NodePackageInfo); ok && info.PackageName != "" {
			return info.PackageName, nil
		}
	}

	if pkg.Namespace != "" {
		return "@" + pkg.Namespace + "/" + pkg.Name, nil
	}

	return "@pulumi/" + pkg.Name, nil
}

func setOptionsAttributes(opts nodeOptions) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.Bool("nodeOptions.typeScript", opts.typescript),
		attribute.String("nodeOptions.packageManager", string(opts.packagemanager)),
		attribute.String("nodeOptions.tsConfigPath", opts.tsconfigpath),
		attribute.String("nodeOptions.nodeArgs", opts.nodeargs),
	}
}
