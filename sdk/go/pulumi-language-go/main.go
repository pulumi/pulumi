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

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/blang/semver"
	"github.com/opentracing/opentracing-go"
	"golang.org/x/mod/modfile"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/go/pulumi-language-go/v3/buildutil"
	"github.com/pulumi/pulumi/sdk/go/pulumi-language-go/v3/goversion"
	"github.com/pulumi/pulumi/sdk/v3/go/common/constant"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tail"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/errutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi-internal/netutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	codegen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	hclsyntax "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// The preferred debug port.  Chosen arbitrarily.
const preferredDebugPort = 57134

// This function takes a file target to specify where to compile to.
// If `outfile` is "", the binary is compiled to a new temporary file.
// This function returns the path of the file that was produced.
func compileProgram(programDirectory string, outfile string, withDebugFlags bool) (string, error) {
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
	args := []string{"build", "-o", outfile}
	if withDebugFlags {
		args = append(args, "-gcflags", "all=-N -l")
	}
	buildCmd := exec.Command(gobin, args...)
	buildCmd.Dir = programDirectory
	buildCmd.Stdout, buildCmd.Stderr = os.Stdout, os.Stderr

	if err := buildCmd.Run(); err != nil {
		return "", fmt.Errorf("unable to run `go build`: %w", err)
	}

	isExecutable, err := executable.IsExecutable(outfile)
	if err != nil {
		return "", fmt.Errorf("unable to check if go program is executable: %w", err)
	}
	if !isExecutable {
		return "", errors.New("go program is not executable, does your program have a 'main' package?")
	}

	return outfile, nil
}

// runParams defines the command line arguments accepted by this program.
type runParams struct {
	tracing       string
	engineAddress string
}

// parseRunParams parses the given arguments into a runParams structure,
// using the provided FlagSet.
func parseRunParams(flag *flag.FlagSet, args []string) (*runParams, error) {
	var p runParams
	flag.StringVar(&p.tracing, "tracing", "", "Emit tracing to a Zipkin-compatible tracing endpoint")
	flag.String("binary", "", "[obsolete] Look on path for a binary executable with this name")
	flag.String("buildTarget", "", "[obsolete] Path to use to output the compiled Pulumi Go program")
	flag.String("root", "", "[obsolete] Project root path to use")

	if err := flag.Parse(args); err != nil {
		return nil, err
	}

	// Pluck out the engine so we can do logging, etc.
	args = flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Warning: launching without arguments, only for debugging")
	} else {
		p.engineAddress = args[0]
	}

	return &p, nil
}

// Launches the language host, which in turn fires up an RPC server implementing the LanguageRuntimeServer endpoint.
func main() {
	p, err := parseRunParams(flag.CommandLine, os.Args[1:])
	if err != nil {
		cmdutil.Exit(err)
	}

	logging.InitLogging(false, 0, false)
	cmdutil.InitTracing("pulumi-language-go", "pulumi-language-go", p.tracing)

	var cmd mainCmd
	if err := cmd.Run(p); err != nil {
		cmdutil.Exit(err)
	}
}

type mainCmd struct {
	Stdout io.Writer              // == os.Stdout
	Getwd  func() (string, error) // == os.Getwd
}

func (cmd *mainCmd) init() {
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}
	if cmd.Getwd == nil {
		cmd.Getwd = os.Getwd
	}
}

func (cmd *mainCmd) Run(p *runParams) error {
	cmd.init()

	cwd, err := cmd.Getwd()
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	// map the context Done channel to the rpcutil boolean cancel channel
	cancelChannel := make(chan bool)
	go func() {
		<-ctx.Done()
		cancel() // deregister handler so we don't catch another interrupt
		close(cancelChannel)
	}()
	if p.engineAddress != "" {
		// We must be debugging, wait to be informed of the engine address.
		err = rpcutil.Healthcheck(ctx, p.engineAddress, 5*time.Minute, cancel)
		if err != nil {
			return fmt.Errorf("could not start health check host RPC server: %w", err)
		}
	}

	// Fire up a gRPC server, letting the kernel choose a free port.
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancelChannel,
		Init: func(srv *grpc.Server) error {
			host := newLanguageHost(p.engineAddress, cwd, p.tracing)
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

	cwd           string
	engineAddress string
	tracing       string
}

type goOptions struct {
	// Look on path for a binary executable with this name.
	binary string
	// Path to use to output the compiled Pulumi Go program.
	buildTarget string
}

func parseOptions(root string, options map[string]interface{}) (goOptions, error) {
	var goOptions goOptions
	if binary, ok := options["binary"]; ok {
		if binary, ok := binary.(string); ok {
			goOptions.binary = binary
		} else {
			return goOptions, errors.New("binary option must be a string")
		}
	}

	if buildTarget, ok := options["buildTarget"]; ok {
		if args, ok := buildTarget.(string); ok {
			goOptions.buildTarget = args
		} else {
			return goOptions, errors.New("buildTarget option must be a string")
		}
	}

	if goOptions.binary != "" && goOptions.buildTarget != "" {
		return goOptions, errors.New("binary and buildTarget cannot both be specified")
	}

	return goOptions, nil
}

func newLanguageHost(engineAddress, cwd, tracing string) pulumirpc.LanguageRuntimeServer {
	return &goLanguageHost{
		engineAddress: engineAddress,
		cwd:           cwd,
		tracing:       tracing,
	}
}

func (host *goLanguageHost) connectToEngine() (pulumirpc.EngineClient, io.Closer, error) {
	conn, err := grpc.NewClient(
		host.engineAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("language host could not make connection to engine: %w", err)
	}

	// Make a client around that connection.
	engineClient := pulumirpc.NewEngineClient(conn)
	return engineClient, conn, nil
}

// modInfo is the useful portion of the output from
// 'go list -m -json' and 'go mod download -json'
// with respect to plugin acquisition.
// The listed fields are present in both command outputs.
//
// If we add fields that are only present in one or the other,
// we'll need to add a new struct type instead of re-using this.
type modInfo struct {
	// Path is the module import path.
	Path string

	// Version of the module.
	Version string

	// Dir is the directory holding the source code of the module, if any.
	Dir string
}

// findModuleSources finds the source code roots for the given modules.
//
// gobin is the path to the go binary to use.
// rootModuleDir is the path to the root directory of the program that may import the modules.
// It must contain the go.mod file for the program.
// modulePaths is a list of import paths for the modules to find.
//
// If $rootModuleDir/vendor exists, findModuleSources operates in vendor mode.
// In vendor mode, returned paths are inside the vendor directory exclusively.
func findModuleSources(ctx context.Context, gobin, rootModuleDir string, modulePaths []string) ([]modInfo, error) {
	contract.Requiref(gobin != "", "gobin", "must not be empty")
	contract.Requiref(rootModuleDir != "", "rootModuleDir", "must not be empty")
	if len(modulePaths) == 0 {
		return nil, nil
	}

	// To find the source code for a module, we would typically use
	// 'go list -m -json'.
	// Its output includes, among other things:
	//
	//    type Module struct {
	//       ...
	//       Dir string // directory holding local copy of files, if any
	//       ...
	//    }
	//
	// However, whether Dir is set or not depends on a few different factors.
	//
	//  - If the module is not in the local module cache,
	//    then Dir is always empty.
	//  - If the module is imported by the current module, then Dir is set.
	//  - If the module is not imported,
	//    then Dir is set if we run with -mod=mod.
	//  - If the module is not imported and we run without -mod=mod,
	//    then:
	//    - If there's a vendor/ directory, then Dir is not set
	//      because we're running in vendor mode.
	//    - If there's no vendor/ directory, then Dir is set
	//      if we add the module to the module cache
	//      with `go mod download $path`.
	//
	// These are all corner cases that aren't fully specified,
	// and may change between versions of Go.
	//
	// Therefore, the flow we use is:
	//
	//  - Run 'go list -m -json $path1 $path2 ...'
	//    to make a first pass at getting module information.
	//  - If there's a vendor/ directory,
	//    use the module information from the vendor directory,
	//    skipping anything that's missing.
	//    We can't make requests to download modules in vendor mode.
	//  - Otherwise, for modules with missing Dir fields,
	//    run `go mod download -json $path` to download them to the module cache
	//    and get their locations.

	modules, err := goListModules(ctx, gobin, rootModuleDir, modulePaths)
	if err != nil {
		return nil, fmt.Errorf("go list: %w", err)
	}

	// If there's a vendor directory, then we're in vendor mode.
	// In vendor mode, Dir won't be set for any modules.
	// Find these modules in the vendor directory.
	vendorDir := filepath.Join(rootModuleDir, "vendor")
	if _, err := os.Stat(vendorDir); err == nil {
		newModules := modules[:0] // in-place filter
		for _, module := range modules {
			if module.Dir == "" {
				vendoredModule := filepath.Join(vendorDir, module.Path)
				if _, err := os.Stat(vendoredModule); err == nil {
					module.Dir = vendoredModule
				}
			}

			// We can't download modules in vendor mode,
			// so we'll skip any modules that aren't already in the vendor directory.
			if module.Dir != "" {
				newModules = append(newModules, module)
			}
		}
		return newModules, nil
	}

	// We're not in vendor mode, so we can download modules and fill in missing directories.
	var (
		// Import paths of modules with no Dir field.
		missingDirs []string

		// Map from module path to index in modules.
		moduleIndex = make(map[string]int, len(modules))
	)
	for i, module := range modules {
		moduleIndex[module.Path] = i
		if module.Dir == "" {
			missingDirs = append(missingDirs, module.Path)
		}
	}

	// Fill in missing module directories with `go mod download`.
	if len(missingDirs) > 0 {
		missingMods, err := goModDownload(ctx, gobin, rootModuleDir, missingDirs)
		if err != nil {
			return nil, fmt.Errorf("go mod download: %w", err)
		}

		for _, m := range missingMods {
			if m.Dir == "" {
				continue
			}

			// If this was a module we were missing,
			// then we can fill in the directory now.
			if idx, ok := moduleIndex[m.Path]; ok && modules[idx].Dir == "" {
				modules[idx].Dir = m.Dir
			}
		}
	}

	// Any other modules with no Dir field can be discarded;
	// we tried our best to find their source.
	newModules := modules[:0] // in-place filter
	for _, module := range modules {
		if module.Dir != "" {
			newModules = append(newModules, module)
		}
	}
	return newModules, nil
}

// Runs 'go list -m' on the given list of modules
// and reports information about them.
func goListModules(ctx context.Context, gobin, dir string, modulePaths []string) ([]modInfo, error) {
	args := slice.Prealloc[string](len(modulePaths) + 3)
	args = append(args, "list", "-m", "-json")
	args = append(args, modulePaths...)

	span, ctx := opentracing.StartSpanFromContext(ctx,
		gobin+" list -m json",
		opentracing.Tag{Key: "component", Value: "exec.Command"},
		opentracing.Tag{Key: "command", Value: gobin},
		opentracing.Tag{Key: "args", Value: args})
	defer span.Finish()

	cmd := exec.CommandContext(ctx, gobin, args...)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start command: %w", err)
	}

	var modules []modInfo
	dec := json.NewDecoder(stdout)
	for dec.More() {
		var info modInfo
		if err := dec.Decode(&info); err != nil {
			return nil, fmt.Errorf("decode module info: %w", err)
		}
		modules = append(modules, info)
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("wait for command: %w", err)
	}

	return modules, nil
}

// goModDownload downloads the given modules to the module cache,
// reporting information about them in the returned modInfo.
func goModDownload(ctx context.Context, gobin, dir string, modulePaths []string) ([]modInfo, error) {
	args := slice.Prealloc[string](len(modulePaths) + 3)
	args = append(args, "mod", "download", "-json")
	args = append(args, modulePaths...)

	span, ctx := opentracing.StartSpanFromContext(ctx,
		gobin+" mod download -json",
		opentracing.Tag{Key: "component", Value: "exec.Command"},
		opentracing.Tag{Key: "command", Value: gobin},
		opentracing.Tag{Key: "args", Value: args})
	defer span.Finish()

	cmd := exec.CommandContext(ctx, gobin, args...)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start command: %w", err)
	}

	var modules []modInfo
	dec := json.NewDecoder(stdout)
	for dec.More() {
		var info modInfo
		if err := dec.Decode(&info); err != nil {
			return nil, fmt.Errorf("decode module info: %w", err)
		}
		modules = append(modules, info)
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("wait for command: %w", err)
	}

	return modules, nil
}

// Returns the pulumi-plugin.json if found.
// If not found, then returns nil, nil.
//
// The lookup path for pulumi-plugin.json is:
//
//   - m.Dir
//   - m.Dir/go
//   - m.Dir/go/*
//
// moduleRoot is the root directory of the module that imports this plugin.
// It is used to resolve the vendor directory.
// moduleRoot may be empty if unknown.
func (m *modInfo) readPulumiPluginJSON(moduleRoot string) (*plugin.PulumiPluginJSON, error) {
	dir := m.Dir
	contract.Assertf(dir != "", "module directory must be known")

	paths := []string{
		filepath.Join(dir, "pulumi-plugin.json"),
		filepath.Join(dir, "go", "pulumi-plugin.json"),
	}
	if path, err := filepath.Glob(filepath.Join(dir, "go", "*", "pulumi-plugin.json")); err == nil {
		paths = append(paths, path...)
	}
	if path, err := filepath.Glob(filepath.Join(dir, "*", "pulumi-plugin.json")); err == nil {
		paths = append(paths, path...)
	}

	for _, path := range paths {
		plugin, err := plugin.LoadPulumiPluginJSON(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		return plugin, nil
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

// getPackage loads information about this package.
//
// moduleRoot is the root directory of the Go module that imports this package.
// It must hold the go.mod file and the vendor directory (if any).
func (m *modInfo) getPackage(moduleRoot string) (*pulumirpc.PackageDependency, error) {
	pulumiPlugin, err := m.readPulumiPluginJSON(moduleRoot)
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
	var parameterization *pulumirpc.PackageParameterization
	if pulumiPlugin != nil {
		// There is no way to specify server or parameterization without using `pulumi-plugin.json`.
		server = pulumiPlugin.Server
		if pulumiPlugin.Parameterization != nil {
			parameterization = &pulumirpc.PackageParameterization{
				Name:    pulumiPlugin.Parameterization.Name,
				Version: pulumiPlugin.Parameterization.Version,
				Value:   pulumiPlugin.Parameterization.Value,
			}
		}
	}

	plugin := &pulumirpc.PackageDependency{
		Name:             name,
		Version:          version,
		Kind:             "resource",
		Server:           server,
		Parameterization: parameterization,
	}

	return plugin, nil
}

// Reads and parses the go.mod file for the program at the given path.
// Returns the parsed go.mod file and the path to the module directory.
// Relies on the 'go' command to find the go.mod file for this program.
func (host *goLanguageHost) loadGomod(gobin, programDir string) (modDir string, modFile *modfile.File, err error) {
	// Get the path to the go.mod file.
	// This may be different from the programDir if the Pulumi program
	// is in a subdirectory of the Go module.
	//
	// The '-f {{.GoMod}}' specifies that the command should print
	// just the path to the go.mod file.
	//
	//	type Module struct {
	//		Path     string // module path
	//		...
	//		GoMod    string // path to go.mod file
	//	}
	//
	// See 'go help list' for the full definition.
	cmd := exec.Command(gobin, "list", "-m", "-f", "{{.GoMod}}")
	// Disable GOWORK, we only want the one module found in this (or a parent) directory. Workspaces being
	// enabled will cause "list -m" to list every module in the workspace.
	cmd.Env = append(os.Environ(), "GOWORK=off")
	cmd.Dir = programDir
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", nil, fmt.Errorf("go list -m: %w", err)
	}
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		// The 'go list' command above will exit successfully
		// and return no output if the program is not in a Go module.
		return "", nil, fmt.Errorf("no go.mod file found: %v", programDir)
	}

	modPath := string(out)
	body, err := os.ReadFile(modPath)
	if err != nil {
		return "", nil, err
	}

	f, err := modfile.Parse(modPath, body, nil)
	if err != nil {
		return "", nil, fmt.Errorf("parse: %w", err)
	}

	return filepath.Dir(modPath), f, nil
}

// GetRequiredPackages computes the complete set of anticipated packages required by a program.
// We're lenient here as this relies on the `go list` command and the use of modules.
// If the consumer insists on using some other form of dependency management tool like
// dep or glide, the list command fails with "go list -m: not using modules".
// However, we do enforce that go 1.14.0 or higher is installed.
func (host *goLanguageHost) GetRequiredPackages(ctx context.Context,
	req *pulumirpc.GetRequiredPackagesRequest,
) (*pulumirpc.GetRequiredPackagesResponse, error) {
	logging.V(5).Infof("GetRequiredPackages: Determining pulumi packages")

	gobin, err := executable.FindExecutable("go")
	if err != nil {
		return nil, fmt.Errorf("couldn't find go binary: %w", err)
	}

	if err = goversion.CheckMinimumGoVersion(gobin); err != nil {
		return nil, err
	}

	moduleDir, gomod, err := host.loadGomod(gobin, req.Info.ProgramDirectory)
	if err != nil {
		// Don't fail if not using Go modules.
		logging.V(5).Infof("GetRequiredPackages: Error reading go.mod: %v", err)
		return &pulumirpc.GetRequiredPackagesResponse{}, nil
	}

	modulePaths := slice.Prealloc[string](len(gomod.Require))
	for _, req := range gomod.Require {
		modulePaths = append(modulePaths, req.Mod.Path)
	}

	modInfos, err := findModuleSources(ctx, gobin, moduleDir, modulePaths)
	if err != nil {
		logging.V(5).Infof("GetRequiredPackages: Error finding module sources: %v", err)
		return &pulumirpc.GetRequiredPackagesResponse{}, nil
	}

	packages := []*pulumirpc.PackageDependency{}
	for _, m := range modInfos {
		pkg, err := m.getPackage(moduleDir)
		if err != nil {
			logging.V(5).Infof(
				"GetRequiredPackages: Ignoring dependency: %s, version: %s, error: %s",
				m.Path,
				m.Version,
				err,
			)
			continue
		}

		logging.V(5).Infof("GetRequiredPackages: Found package name: %s, version: %s", pkg.Name, pkg.Version)
		packages = append(packages, pkg)
	}

	return &pulumirpc.GetRequiredPackagesResponse{
		Packages: packages,
	}, nil
}

func (host *goLanguageHost) GetRequiredPlugins(ctx context.Context,
	req *pulumirpc.GetRequiredPluginsRequest,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetRequiredPlugins not implemented")
}

func runCmdStatus(runF func() error) (int, error) {
	err := runF()
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

type debugger struct {
	Host    string
	Port    int64
	LogDest string
}

// WaitForReady waits for Delve to be ready to accept connections.
// Returns an error if the context is canceled or the log file is unable to be tailed.
func (c *debugger) WaitForReady(ctx context.Context) error {
	t, err := tail.File(c.LogDest, tail.Config{
		Follow: true,
		Logger: tail.DiscardingLogger,
	})
	if err != nil {
		return err
	}
	defer func() {
		contract.IgnoreError(t.Stop())
		t.Cleanup()
	}()
	for line := range t.Lines {
		if line.Err != nil {
			continue
		}
		if addr, ok := strings.CutPrefix(line.Text, "API server listening at: "); ok {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return err
			}
			c.Host = host
			p, err := strconv.ParseInt(port, 10, 32)
			if err != nil {
				return err
			}
			c.Port = p
			break
		}
	}
	return nil
}

func (c *debugger) Cleanup() {
	contract.IgnoreError(os.Remove(c.LogDest))
}

func debugCommand(ctx context.Context, bin string, binArgs ...string) (*exec.Cmd, *debugger, error) {
	godlv, err := executable.FindExecutable("dlv")
	if err != nil {
		return nil, nil, fmt.Errorf("unable to find 'dlv' executable: %w", err)
	}
	logFile, err := os.CreateTemp("", "pulumi-go-dlv-")
	if err != nil {
		return nil, nil, fmt.Errorf("unable to allocate tmp file: %w", err)
	}
	contract.IgnoreClose(logFile)
	args := []string{"--headless=true", "--api-version=2"}
	args = append(args, "--log", "--log-dest", logFile.Name())
	port, err := netutil.FindNextAvailablePort(preferredDebugPort)
	if err == nil {
		args = append(args, "--listen=127.0.0.1:"+strconv.Itoa(port))
	}
	args = append(args, "exec", bin)
	if len(binArgs) > 0 {
		args = append(args, "")
		args = append(args, binArgs...)
	}
	dlvCmd := exec.CommandContext(ctx, godlv, args...)
	return dlvCmd, &debugger{Host: "127.0.0.1", LogDest: logFile.Name()}, nil
}

func startDebugging(ctx context.Context, engineClient pulumirpc.EngineClient, dbg *debugger, name string) error {
	// wait for the debugger to be ready
	ctx, cancel := context.WithTimeoutCause(ctx, 1*time.Minute, errors.New("debugger startup timed out"))
	defer cancel()
	err := dbg.WaitForReady(ctx)
	if err != nil {
		return err
	}

	debugConfig, err := structpb.NewStruct(map[string]interface{}{
		"name":    name,
		"type":    "go",
		"request": "attach",
		"mode":    "remote",
		"host":    dbg.Host,
		"port":    dbg.Port,
	})
	if err != nil {
		return err
	}
	_, err = engineClient.StartDebugging(ctx, &pulumirpc.StartDebuggingRequest{
		Config:  debugConfig,
		Message: "on port " + strconv.FormatInt(dbg.Port, 10),
	})
	if err != nil {
		return fmt.Errorf("unable to start debugging: %w", err)
	}

	return nil
}

func runProgram(
	ctx context.Context,
	engineClient pulumirpc.EngineClient,
	req *pulumirpc.RunRequest,
	pwd, bin string,
	env []string,
) *pulumirpc.RunResponse {
	var err error
	var dbg *debugger
	var cmd *exec.Cmd
	if req.GetAttachDebugger() {
		cmd, dbg, err = debugCommand(ctx, bin)
		if err != nil {
			return &pulumirpc.RunResponse{
				Error: err.Error(),
			}
		}
		defer dbg.Cleanup()
		// create a sub-context to cancel the startDebugging operation when the process exits.
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		go func() {
			err := startDebugging(ctx, engineClient, dbg, "Pulumi: Program (Go)")
			if err != nil {
				// kill the program if we can't start debugging.
				logging.Errorf("Unable to start debugging: %v", err)
				contract.IgnoreError(cmd.Process.Kill())
			}
		}()
	} else {
		cmd = exec.CommandContext(ctx, bin)
	}
	cmd.Dir = pwd
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	status, err := runCmdStatus(cmd.Run)
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
	if host.engineAddress == "" {
		return nil, errors.New("when debugging or running explicitly, must call Handshake before Run")
	}

	engineClient, closer, err := host.connectToEngine()
	if err != nil {
		return nil, err
	}
	defer contract.IgnoreClose(closer)

	opts, err := parseOptions(req.Info.RootDirectory, req.Info.Options.AsMap())
	if err != nil {
		return nil, err
	}

	// Create the environment we'll use to run the process.  This is how we pass the RunInfo to the actual
	// Go program runtime, to avoid needing any sort of program interface other than just a main entrypoint.
	env, err := host.constructEnv(req)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare environment: %w", err)
	}

	// the user can explicitly opt in to using a binary executable by specifying
	// runtime.options.binary in the Pulumi.yaml
	if opts.binary != "" {
		bin, err := executable.FindExecutable(opts.binary)
		if err != nil {
			return nil, fmt.Errorf("unable to find '%s' executable: %w", opts.binary, err)
		}
		return runProgram(ctx, engineClient, req, req.Pwd, bin, env), nil
	}

	// feature flag to enable deprecated old behavior and use `go run`
	if os.Getenv("PULUMI_GO_USE_RUN") != "" {
		gobin, err := executable.FindExecutable("go")
		if err != nil {
			return nil, fmt.Errorf("unable to find 'go' executable: %w", err)
		}

		cmd := exec.Command(gobin, "run", req.Info.ProgramDirectory)
		cmd.Dir = host.cwd
		cmd.Env = env
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		status, err := runCmdStatus(cmd.Run)
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

	program, err := compileProgram(req.Info.ProgramDirectory, opts.buildTarget, req.GetAttachDebugger())
	if err != nil {
		return nil, errutil.ErrorWithStderr(err, "error in compiling Go")
	}
	if opts.buildTarget == "" {
		// If there is no specified buildTarget, delete the temporary program after running it.
		defer os.Remove(program)
	}

	return runProgram(ctx, engineClient, req, req.Pwd, program, env), nil
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
	maybeAppendEnv(pulumi.EnvPulumiRootDirectory, req.GetInfo().RootDirectory)
	maybeAppendEnv(pulumi.EnvStack, req.GetStack())
	maybeAppendEnv(pulumi.EnvConfig, config)
	maybeAppendEnv(pulumi.EnvConfigSecretKeys, configSecretKeys)
	maybeAppendEnv(pulumi.EnvDryRun, strconv.FormatBool(req.GetDryRun()))
	maybeAppendEnv(pulumi.EnvParallel, strconv.Itoa(int(req.GetParallel())))
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

func (host *goLanguageHost) GetPluginInfo(ctx context.Context, req *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
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

	cmd := exec.Command(gobin, "mod", "tidy", "-compat=1.18")
	cmd.Dir = req.Info.ProgramDirectory
	cmd.Env = os.Environ()
	cmd.Stdout, cmd.Stderr = stdout, stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("`go mod tidy` failed to install dependencies: %w", err)
	}

	stdout.Write([]byte("Finished installing dependencies\n\n"))

	return closer.Close()
}

func (host *goLanguageHost) RuntimeOptionsPrompts(ctx context.Context,
	req *pulumirpc.RuntimeOptionsRequest,
) (*pulumirpc.RuntimeOptionsResponse, error) {
	return &pulumirpc.RuntimeOptionsResponse{}, nil
}

func (host *goLanguageHost) About(ctx context.Context, req *pulumirpc.AboutRequest) (*pulumirpc.AboutResponse, error) {
	getResponse := func(execString string, args ...string) (string, string, error) {
		ex, err := executable.FindExecutable(execString)
		if err != nil {
			return "", "", fmt.Errorf("could not find executable '%s': %w", execString, err)
		}
		cmd := exec.Command(ex, args...)
		cmd.Dir = host.cwd
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

func (host *goLanguageHost) GetProgramDependencies(
	ctx context.Context, req *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	gobin, err := executable.FindExecutable("go")
	if err != nil {
		return nil, fmt.Errorf("couldn't find go binary: %w", err)
	}

	_, gomod, err := host.loadGomod(gobin, req.Info.ProgramDirectory)
	if err != nil {
		return nil, fmt.Errorf("load go.mod: %w", err)
	}

	// If a module is locally replaced it doesn't really have a version anymore.
	replaced := make(map[string]bool)
	for _, r := range gomod.Replace {
		replaced[strings.TrimRight(r.Old.Path, "/")] = true
	}

	result := slice.Prealloc[*pulumirpc.DependencyInfo](len(gomod.Require))
	for _, d := range gomod.Require {
		if !d.Indirect || req.TransitiveDependencies {
			version := d.Mod.Version
			if replaced[strings.TrimRight(d.Mod.Path, "/")] {
				version = ""
			}
			datum := pulumirpc.DependencyInfo{
				Name:    d.Mod.Path,
				Version: version,
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
	logging.V(5).Infof("Attempting to run go plugin in %s", req.Info.ProgramDirectory)

	if host.engineAddress == "" {
		return errors.New("when debugging or running explicitly, must call Handshake before RunPlugin")
	}

	engineClient, closer, err := host.connectToEngine()
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(closer)

	program, err := compileProgram(req.Info.ProgramDirectory, "", false)
	if err != nil {
		return errutil.ErrorWithStderr(err, "error in compiling Go")
	}
	defer os.Remove(program)

	closer, stdout, stderr, err := rpcutil.MakeRunPluginStreams(server, false)
	if err != nil {
		return err
	}
	// best effort close, but we try an explicit close and error check at the end as well
	defer closer.Close()

	var cmd *exec.Cmd
	if req.GetAttachDebugger() {
		var dbg *debugger
		cmd, dbg, err = debugCommand(server.Context(), program, req.Args...)
		if err != nil {
			return err
		}
		defer dbg.Cleanup()
		// create a sub-context to cancel the startDebugging operation when the process exits.
		ctx, cancel := context.WithCancel(server.Context())
		defer cancel()
		go func() {
			err := startDebugging(ctx, engineClient, dbg, fmt.Sprintf("Pulumi: Plugin (%s)", req.Name))
			if err != nil {
				// kill the plugin if we can't start debugging.
				logging.Errorf("Unable to start debugging: %v", err)
				contract.IgnoreError(cmd.Process.Kill())
			}
		}()
	} else {
		cmd = exec.CommandContext(server.Context(), program, req.Args...)
		cmd.Dir = req.Pwd
		cmd.Env = req.Env
		cmd.Stdout, cmd.Stderr = stdout, stderr
	}

	if err = cmd.Run(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				err = server.Send(&pulumirpc.RunPluginResponse{
					//nolint:gosec // WaitStatus always uses the lower 8 bits for the exit code.
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

func (host *goLanguageHost) GenerateProject(
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

	rpcDiagnostics := plugin.HclDiagnosticsToRPCDiagnostics(diags)
	if diags.HasErrors() {
		return &pulumirpc.GenerateProjectResponse{
			Diagnostics: rpcDiagnostics,
		}, nil
	}
	if program == nil {
		return nil, errors.New("internal error: program was nil")
	}

	var project workspace.Project
	if err := json.Unmarshal([]byte(req.Project), &project); err != nil {
		return nil, err
	}

	err = codegen.GenerateProject(req.TargetDirectory, project, program, req.LocalDependencies)
	if err != nil {
		return nil, err
	}

	return &pulumirpc.GenerateProjectResponse{
		Diagnostics: rpcDiagnostics,
	}, nil
}

func (host *goLanguageHost) GenerateProgram(
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

	program, diags, err := pcl.BindProgram(parser.Files, pcl.Loader(schema.NewCachedLoader(loader)))
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
		return nil, errors.New("internal error: program was nil")
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

func (host *goLanguageHost) GeneratePackage(
	ctx context.Context, req *pulumirpc.GeneratePackageRequest,
) (*pulumirpc.GeneratePackageResponse, error) {
	if len(req.ExtraFiles) > 0 {
		return nil, errors.New("overlays are not supported for Go")
	}

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
	files, err := codegen.GeneratePackage("pulumi-language-go", pkg, req.LocalDependencies)
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

func (host *goLanguageHost) Pack(ctx context.Context, req *pulumirpc.PackRequest) (*pulumirpc.PackResponse, error) {
	// Go is very simple, there's nothing it can do to pack so it just copies the source to the target.

	// First read the modfile in the package directory to decide the folder name.
	data, err := os.ReadFile(filepath.Join(req.PackageDirectory, "go.mod"))
	if err != nil {
		return nil, fmt.Errorf("read go.mod: %w", err)
	}
	mod, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return nil, fmt.Errorf("parse go.mod: %w", err)
	}

	// The folder name is the module name, made into a valid directory name.
	folderName := strings.ReplaceAll(mod.Module.Mod.Path, "/", "_")
	artifactPath := filepath.Join(req.DestinationDirectory, folderName)

	err = fsutil.CopyFile(artifactPath, req.PackageDirectory, nil)
	if err != nil {
		return nil, fmt.Errorf("copy package: %w", err)
	}

	return &pulumirpc.PackResponse{
		ArtifactPath: artifactPath,
	}, nil
}

func (host *goLanguageHost) Handshake(
	ctx context.Context,
	req *pulumirpc.LanguageHandshakeRequest,
) (*pulumirpc.LanguageHandshakeResponse, error) {
	if req == nil || req.EngineAddress == "" {
		return nil, errors.New("Must contain address in request")
	}
	host.engineAddress = req.EngineAddress

	ctx, cancel := context.WithCancel(ctx)
	cancelChannel := make(chan bool)
	go func() {
		<-ctx.Done()
		cancel() // deregister handler so we don't catch another interrupt
		close(cancelChannel)
	}()
	err := rpcutil.Healthcheck(ctx, host.engineAddress, 5*time.Minute, cancel)
	if err != nil {
		return nil, fmt.Errorf("could not start health check host RPC server: %w", err)
	}

	return &pulumirpc.LanguageHandshakeResponse{}, nil
}

func (host *goLanguageHost) Link(
	ctx context.Context, req *pulumirpc.LinkRequest,
) (*pulumirpc.LinkResponse, error) {
	// Find the go.mod in the program directory, and add a replace statement to point to the given local dependency.
	// Currently this _only_ supports "pulumi" for the core SDK.

	path := filepath.Join(req.Info.ProgramDirectory, "go.mod")
	stat, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat go.mod: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read go.mod: %w", err)
	}
	mod, err := modfile.Parse(path, data, nil)
	if err != nil {
		return nil, fmt.Errorf("parse go.mod: %w", err)
	}

	core, ok := req.LocalDependencies["pulumi"]
	if !ok {
		// TODO: support other local dependencies.
		return nil, status.Errorf(codes.InvalidArgument, "no local dependency for 'pulumi' found")
	}

	err = mod.AddReplace("github.com/pulumi/pulumi/sdk/v3", "", core, "")
	if err != nil {
		return nil, fmt.Errorf("add replace: %w", err)
	}

	// Write the modified go.mod back to disk.
	data, err = mod.Format()
	if err != nil {
		return nil, fmt.Errorf("format go.mod: %w", err)
	}
	err = os.WriteFile(path, data, stat.Mode().Perm())
	if err != nil {
		return nil, fmt.Errorf("write go.mod: %w", err)
	}

	return &pulumirpc.LinkResponse{}, nil
}

func (host *goLanguageHost) Cancel(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
