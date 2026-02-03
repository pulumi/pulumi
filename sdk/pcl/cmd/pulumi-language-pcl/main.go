// Copyright 2016-2026, Pulumi Corporation.
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
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/hashicorp/hcl/v2"
	hashihclsyntax "github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	hclsyntax "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	aferoutil "github.com/pulumi/pulumi/pkg/v3/util/afero"
	pclruntime "github.com/pulumi/pulumi/sdk/pcl/v3/runtime"
	pulumiencoding "github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/zclconf/go-cty/cty"

	"github.com/spf13/afero"
)

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

	if err := flag.Parse(args); err != nil {
		return nil, err
	}

	args = flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Warning: launching without arguments, only for debugging")
	} else {
		p.engineAddress = args[0]
	}

	return &p, nil
}

// Launches the language host RPC endpoint.
func main() {
	showVersion := flag.Bool("version", false, "Print the current plugin version and exit")
	p, err := parseRunParams(flag.CommandLine, os.Args[1:])
	if err != nil {
		cmdutil.Exit(err)
	}

	if *showVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	logging.InitLogging(false, 0, false)
	cmdutil.InitTracing("pulumi-language-pcl", "pulumi-language-pcl", p.tracing)

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
	cancelChannel := make(chan bool)
	go func() {
		<-ctx.Done()
		cancel()
		close(cancelChannel)
	}()

	if p.engineAddress != "" {
		err := rpcutil.Healthcheck(ctx, p.engineAddress, 5*time.Minute, cancel)
		if err != nil {
			return fmt.Errorf("could not start health check host RPC server: %w", err)
		}
	}

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

	fmt.Fprintf(cmd.Stdout, "%d\n", handle.Port)

	if err := <-handle.Done; err != nil {
		return fmt.Errorf("language host RPC stopped serving: %w", err)
	}

	return nil
}

// pclLanguageHost implements the LanguageRuntimeServer interface.
type pclLanguageHost struct {
	pulumirpc.UnsafeLanguageRuntimeServer

	cwd           string
	engineAddress string
	tracing       string
}

func newLanguageHost(engineAddress, cwd, tracing string) pulumirpc.LanguageRuntimeServer {
	return &pclLanguageHost{
		engineAddress: engineAddress,
		cwd:           cwd,
		tracing:       tracing,
	}
}

func (host *pclLanguageHost) connectToEngine() (pulumirpc.EngineClient, io.Closer, error) {
	conn, err := grpc.NewClient(
		host.engineAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("language host could not make connection to engine: %w", err)
	}

	engineClient := pulumirpc.NewEngineClient(conn)
	return engineClient, conn, nil
}

func (host *pclLanguageHost) Handshake(
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
		cancel()
		close(cancelChannel)
	}()
	err := rpcutil.Healthcheck(ctx, host.engineAddress, 5*time.Minute, cancel)
	if err != nil {
		return nil, fmt.Errorf("could not start health check host RPC server: %w", err)
	}

	return &pulumirpc.LanguageHandshakeResponse{}, nil
}

func (host *pclLanguageHost) GetPluginInfo(
	ctx context.Context, req *emptypb.Empty,
) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{Version: version.Version}, nil
}

func (host *pclLanguageHost) constructConfig(req *pulumirpc.RunRequest) (string, error) {
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

func (host *pclLanguageHost) bindProgramFromDirectory(
	directory string,
	loaderTarget string,
	strict bool,
) (*pcl.Program, hcl.Diagnostics, error) {
	client, err := schema.NewLoaderClient(loaderTarget)
	if err != nil {
		return nil, nil, err
	}
	loader := schema.NewCachedLoader(client)
	defer func() {
		contract.IgnoreError(client.Close())
	}()

	options := []pcl.BindOption{pcl.PreferOutputVersionedInvokes}
	if !strict {
		options = append(options, pcl.NonStrictBindOptions()...)
	}

	return pcl.BindDirectory(directory, loader, options...)
}

func (host *pclLanguageHost) Run(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	if host.engineAddress == "" {
		return nil, errors.New("when debugging or running explicitly, must call Handshake before Run")
	}
	if req.Info == nil {
		return nil, errors.New("missing program info")
	}
	if req.GetAttachDebugger() {
		return nil, errors.New("debugging is not supported by the PCL runtime")
	}

	program, diags, err := host.bindProgramFromDirectory(req.Info.ProgramDirectory, req.GetLoaderTarget(), true)
	if err != nil {
		return nil, err
	}
	if diags.HasErrors() {
		if program == nil {
			return &pulumirpc.RunResponse{Error: diags.Error()}, nil
		}
		var buf bytes.Buffer
		writer := program.NewDiagnosticWriter(&buf, 0, false)
		contract.IgnoreError(writer.WriteDiagnostics(diags))
		return &pulumirpc.RunResponse{Error: buf.String()}, nil
	}

	interpreter := pclruntime.NewInterpreter(program, pclruntime.RunInfo{
		Project:        req.GetProject(),
		Stack:          req.GetStack(),
		Organization:   req.GetOrganization(),
		RootDirectory:  req.GetInfo().RootDirectory,
		ProgramDir:     req.GetInfo().ProgramDirectory,
		WorkingDir:     req.GetPwd(),
		Config:         req.GetConfig(),
		ConfigSecrets:  req.GetConfigSecretKeys(),
		MonitorAddress: req.GetMonitorAddress(),
		EngineAddress:  host.engineAddress,
		LoaderAddress:  req.GetLoaderTarget(),
		DryRun:         req.GetDryRun(),
		Parallel:       req.GetParallel(),
	})

	if err := interpreter.Run(ctx); err != nil {
		if result.IsBail(err) {
			return &pulumirpc.RunResponse{Bail: true}, nil
		}
		return &pulumirpc.RunResponse{Error: err.Error()}, nil
	}

	return &pulumirpc.RunResponse{}, nil
}

func (host *pclLanguageHost) InstallDependencies(
	req *pulumirpc.InstallDependenciesRequest,
	server pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	return nil
}

func (host *pclLanguageHost) RuntimeOptionsPrompts(
	ctx context.Context, req *pulumirpc.RuntimeOptionsRequest,
) (*pulumirpc.RuntimeOptionsResponse, error) {
	return &pulumirpc.RuntimeOptionsResponse{}, nil
}

func (host *pclLanguageHost) Template(
	ctx context.Context, req *pulumirpc.TemplateRequest,
) (*pulumirpc.TemplateResponse, error) {
	return &pulumirpc.TemplateResponse{}, nil
}

func (host *pclLanguageHost) About(
	ctx context.Context, req *pulumirpc.AboutRequest,
) (*pulumirpc.AboutResponse, error) {
	return &pulumirpc.AboutResponse{}, nil
}

func (host *pclLanguageHost) GetProgramDependencies(
	ctx context.Context, req *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	deps, err := readPclDependencies(req.Info.ProgramDirectory)
	if err != nil {
		return nil, err
	}

	result := slice.Prealloc[*pulumirpc.DependencyInfo](len(deps) + 1)

	for _, dep := range deps {
		version := ""
		if dep.Version != nil {
			version = dep.Version.String()
		}
		name := dep.Name
		if dep.Parameterization != nil {
			name = dep.Parameterization.Name
			version = dep.Parameterization.Version.String()
		}

		result = append(result, &pulumirpc.DependencyInfo{Name: name, Version: version})
	}

	return &pulumirpc.GetProgramDependenciesResponse{Dependencies: result}, nil
}

func (host *pclLanguageHost) GetRequiredPackages(
	ctx context.Context, req *pulumirpc.GetRequiredPackagesRequest,
) (*pulumirpc.GetRequiredPackagesResponse, error) {
	deps, err := readPclDependencies(req.Info.ProgramDirectory)
	if err != nil {
		return nil, err
	}

	packages := slice.Prealloc[*pulumirpc.PackageDependency](len(deps))
	for _, descriptor := range deps {
		pkg := &pulumirpc.PackageDependency{
			Name:    descriptor.Name,
			Kind:    string(workspace.ResourcePlugin),
			Server:  descriptor.DownloadURL,
			Version: "",
		}
		if descriptor.Version != nil {
			pkg.Version = descriptor.Version.String()
		}
		if descriptor.Parameterization != nil {
			pkg.Parameterization = &pulumirpc.PackageParameterization{
				Name:    descriptor.Parameterization.Name,
				Version: descriptor.Parameterization.Version.String(),
				Value:   descriptor.Parameterization.Value,
			}
		}
		packages = append(packages, pkg)
	}

	return &pulumirpc.GetRequiredPackagesResponse{Packages: packages}, nil
}

func (host *pclLanguageHost) GetRequiredPlugins(
	ctx context.Context, req *pulumirpc.GetRequiredPluginsRequest,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetRequiredPlugins not implemented")
}

func getPclDependencies(parser *hclsyntax.Parser) ([]*schema.PackageDescriptor, error) {
	descriptorMap, diags := pcl.ReadAllPackageDescriptors(parser.Files)
	if diags.HasErrors() {
		return nil, diags
	}

	for _, file := range parser.Files {
		for _, item := range model.SourceOrderBody(file.Body) {
			block, ok := item.(*hashihclsyntax.Block)
			if !ok || block.Type != "resource" || len(block.Labels) < 2 {
				continue
			}
			pkg := packageNameFromToken(block.Labels[1])
			if pkg != "" {
				if _, exists := descriptorMap[pkg]; !exists {
					descriptorMap[pkg] = &schema.PackageDescriptor{Name: pkg}
				}
			}
		}

		diags := hashihclsyntax.VisitAll(file.Body, func(node hashihclsyntax.Node) hcl.Diagnostics {
			call, ok := node.(*hashihclsyntax.FunctionCallExpr)
			if !ok {
				return nil
			}
			token, ok := invokeToken(call)
			if !ok {
				return nil
			}
			pkg := packageNameFromToken(token)
			if pkg != "" {
				if _, exists := descriptorMap[pkg]; !exists {
					descriptorMap[pkg] = &schema.PackageDescriptor{Name: pkg}
				}
			}
			return nil
		})
		contract.Assertf(len(diags) == 0, "unexpected diagnostics while walking AST: %v", diags)
	}

	keys := slice.Prealloc[string](len(descriptorMap))
	for k := range descriptorMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := slice.Prealloc[*schema.PackageDescriptor](len(keys))
	for _, k := range keys {
		result = append(result, descriptorMap[k])
	}

	return result, nil
}

func readPclDependencies(programDir string) ([]*schema.PackageDescriptor, error) {
	parser := hclsyntax.NewParser()
	parseDiagnostics, err := pcl.ParseDirectory(parser, programDir)
	if err != nil {
		return nil, err
	}
	if parseDiagnostics.HasErrors() {
		return nil, parseDiagnostics
	}

	return getPclDependencies(parser)
}

func invokeToken(call *hashihclsyntax.FunctionCallExpr) (string, bool) {
	if call.Name != pcl.Invoke || len(call.Args) < 1 {
		return "", false
	}
	template, ok := call.Args[0].(*hashihclsyntax.TemplateExpr)
	if !ok || len(template.Parts) != 1 {
		return "", false
	}
	literal, ok := template.Parts[0].(*hashihclsyntax.LiteralValueExpr)
	if !ok || literal.Val.Type() != cty.String {
		return "", false
	}
	return literal.Val.AsString(), true
}

func packageNameFromToken(token string) string {
	pkg, mod, name, diags := pcl.DecomposeToken(token, hcl.Range{})
	if diags.HasErrors() {
		return ""
	}
	if pkg == "pulumi" {
		if mod == "providers" {
			return name
		}
		return ""
	}
	return pkg
}

func (host *pclLanguageHost) RunPlugin(
	req *pulumirpc.RunPluginRequest,
	server pulumirpc.LanguageRuntime_RunPluginServer,
) error {
	return errors.New("not implemented")
}

func (host *pclLanguageHost) GenerateProject(
	ctx context.Context, req *pulumirpc.GenerateProjectRequest,
) (*pulumirpc.GenerateProjectResponse, error) {
	loader, err := schema.NewLoaderClient(req.LoaderTarget)
	if err != nil {
		return nil, err
	}
	defer loader.Close()

	bindOptions := []pcl.BindOption{pcl.PreferOutputVersionedInvokes}
	if !req.Strict {
		bindOptions = append(bindOptions, pcl.NonStrictBindOptions()...)
	}
	program, diags, err := pcl.BindDirectory(
		req.SourceDirectory,
		schema.NewCachedLoader(loader),
		bindOptions...,
	)
	if err != nil {
		return nil, err
	}

	rpcDiagnostics := plugin.HclDiagnosticsToRPCDiagnostics(diags)
	if diags.HasErrors() {
		return &pulumirpc.GenerateProjectResponse{Diagnostics: rpcDiagnostics}, nil
	}
	if program == nil {
		return nil, errors.New("internal error: program was nil")
	}

	var project workspace.Project
	if err := json.Unmarshal([]byte(req.Project), &project); err != nil {
		return nil, err
	}

	project.Runtime = workspace.NewProjectRuntimeInfo("pcl", nil)

	projectBytes, err := pulumiencoding.YAML.Marshal(project)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(req.TargetDirectory, 0o755); err != nil {
		return nil, fmt.Errorf("create target directory: %w", err)
	}

	if err := os.WriteFile(filepath.Join(req.TargetDirectory, "Pulumi.yaml"), projectBytes, 0o600); err != nil {
		return nil, fmt.Errorf("write Pulumi.yaml: %w", err)
	}

	// If main is set the Main.yaml file should be in a subdirectory
	directory := req.TargetDirectory
	if project.Main != "" {
		directory = path.Join(directory, project.Main)
		err := os.MkdirAll(directory, 0o700)
		if err != nil {
			return nil, fmt.Errorf("create output directory: %w", err)
		}
	}

	filter := func(info os.FileInfo) bool {
		return info.Name() != "Pulumi.yaml"
	}
	if err := aferoutil.CopyDir(afero.NewOsFs(), req.SourceDirectory, directory, filter); err != nil {
		return nil, fmt.Errorf("copy source directory: %w", err)
	}

	for name, content := range req.LocalDependencies {
		outPath := path.Join(directory, name+".pp")
		err := fsutil.CopyFile(outPath, content, nil)
		if err != nil {
			return nil, fmt.Errorf("copy local dependency: %w", err)
		}
	}

	return &pulumirpc.GenerateProjectResponse{Diagnostics: rpcDiagnostics}, nil
}

func (host *pclLanguageHost) GenerateProgram(
	ctx context.Context, req *pulumirpc.GenerateProgramRequest,
) (*pulumirpc.GenerateProgramResponse, error) {
	loader, err := schema.NewLoaderClient(req.LoaderTarget)
	if err != nil {
		return nil, err
	}
	defer loader.Close()

	parser := hclsyntax.NewParser()
	for path, contents := range req.Source {
		if err = parser.ParseFile(strings.NewReader(contents), path); err != nil {
			return nil, err
		}
		if parser.Diagnostics.HasErrors() {
			return nil, parser.Diagnostics
		}
	}

	options := []pcl.BindOption{pcl.Loader(schema.NewCachedLoader(loader)), pcl.PreferOutputVersionedInvokes}
	if !req.Strict {
		options = append(options, pcl.NonStrictBindOptions()...)
	}

	program, diags, err := pcl.BindProgram(parser.Files, options...)
	if err != nil {
		return nil, err
	}

	rpcDiagnostics := plugin.HclDiagnosticsToRPCDiagnostics(diags)
	if diags.HasErrors() {
		return &pulumirpc.GenerateProgramResponse{Diagnostics: rpcDiagnostics}, nil
	}
	if program == nil {
		return nil, errors.New("internal error program was nil")
	}

	convertedSource := make(map[string][]byte, len(req.Source))
	for filename, contents := range req.Source {
		convertedSource[filename] = []byte(contents)
	}

	return &pulumirpc.GenerateProgramResponse{
		Source:      convertedSource,
		Diagnostics: rpcDiagnostics,
	}, nil
}

func (host *pclLanguageHost) GeneratePackage(
	ctx context.Context, req *pulumirpc.GeneratePackageRequest,
) (*pulumirpc.GeneratePackageResponse, error) {
	// PCL doesn't generally have "SDKs" per-se but we can write out a "lock file" for a given package name
	// and version, and if using a parameterized package this is necessary so that we have somewhere to save
	// the parameter value.

	if len(req.ExtraFiles) > 0 {
		return nil, errors.New("overlays are not supported for PCL")
	}

	loader, err := schema.NewLoaderClient(req.LoaderTarget)
	if err != nil {
		return nil, err
	}

	var spec schema.PackageSpec
	err = json.Unmarshal([]byte(req.Schema), &spec)
	if err != nil {
		return nil, err
	}

	pkg, diags, err := schema.BindSpec(spec, loader, schema.ValidationOptions{
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

	// Generate a package declaration file in PCL (HCL) format. This contains the base provider information
	// and any parameterization data needed to rehydrate the package descriptor.

	var parameterization *schema.ParameterizationDescriptor
	baseProviderName := ""
	baseProviderVersion := ""
	baseProviderDownloadURL := pkg.PluginDownloadURL

	if pkg.Parameterization == nil {
		baseProviderName = pkg.Name
		if pkg.Version != nil {
			baseProviderVersion = pkg.Version.String()
		}
	} else {
		baseProviderName = pkg.Parameterization.BaseProvider.Name
		baseProviderVersion = pkg.Parameterization.BaseProvider.Version.String()
		if pkg.Version == nil {
			return nil, errors.New("parameterized package must have a version")
		}
		parameterization = &schema.ParameterizationDescriptor{
			Name:    pkg.Name,
			Version: *pkg.Version,
			Value:   pkg.Parameterization.Parameter,
		}
	}

	var version string
	if pkg.Version != nil {
		version = fmt.Sprintf("-%s", pkg.Version.String())
	}
	dest := filepath.Join(req.Directory, fmt.Sprintf("%s%s.pp", pkg.Name, version))

	err = os.MkdirAll(req.Directory, 0o700)
	if err != nil {
		return nil, fmt.Errorf("could not create output directory %s: %w", req.Directory, err)
	}

	file := hclwrite.NewEmptyFile()
	root := file.Body()
	block := root.AppendNewBlock("package", []string{pkg.Name})
	body := block.Body()
	if baseProviderName != "" {
		body.SetAttributeValue("baseProviderName", cty.StringVal(baseProviderName))
	}
	if baseProviderVersion != "" {
		body.SetAttributeValue("baseProviderVersion", cty.StringVal(baseProviderVersion))
	}
	if baseProviderDownloadURL != "" {
		body.SetAttributeValue("baseProviderDownloadUrl", cty.StringVal(baseProviderDownloadURL))
	}
	if parameterization != nil {
		paramBlock := body.AppendNewBlock("parameterization", nil)
		paramBody := paramBlock.Body()
		paramBody.SetAttributeValue("name", cty.StringVal(parameterization.Name))
		paramBody.SetAttributeValue("version", cty.StringVal(parameterization.Version.String()))
		paramBody.SetAttributeValue("value", cty.StringVal(base64.StdEncoding.EncodeToString(parameterization.Value)))
	}

	err = os.WriteFile(dest, file.Bytes(), 0o600)
	if err != nil {
		return nil, fmt.Errorf("could not write output file %s: %w", dest, err)
	}

	return &pulumirpc.GeneratePackageResponse{
		Diagnostics: rpcDiagnostics,
	}, nil
}

func (host *pclLanguageHost) Pack(
	ctx context.Context, req *pulumirpc.PackRequest,
) (*pulumirpc.PackResponse, error) {
	// PCL "SDKs" are just files, we can just copy the file
	if err := os.MkdirAll(req.DestinationDirectory, 0o700); err != nil {
		return nil, err
	}

	files, err := os.ReadDir(req.PackageDirectory)
	if err != nil {
		return nil, fmt.Errorf("reading package directory: %w", err)
	}

	copyFile := func(src, dst string) error {
		srcFile, err := os.Open(src)
		if err != nil {
			return fmt.Errorf("opening %s: %w", src, err)
		}
		defer srcFile.Close()
		dstFile, err := os.Create(dst)
		if err != nil {
			return fmt.Errorf("creating %s: %w", dst, err)
		}
		defer dstFile.Close()
		if _, err := io.Copy(dstFile, srcFile); err != nil {
			return fmt.Errorf("copying %s to %s: %w", src, dst, err)
		}
		return nil
	}

	// We only expect one file in the package directory
	var single string
	for _, file := range files {
		if single != "" {
			return nil, fmt.Errorf("multiple files in package directory %s: %s and %s", req.PackageDirectory, single, file.Name())
		}
		single = file.Name()
	}

	src := filepath.Join(req.PackageDirectory, single)
	dst := filepath.Join(req.DestinationDirectory, single)
	if err := copyFile(src, dst); err != nil {
		return nil, fmt.Errorf("copying %s to %s: %w", src, dst, err)
	}

	return &pulumirpc.PackResponse{
		ArtifactPath: dst,
	}, nil
}

func (host *pclLanguageHost) Link(
	ctx context.Context, req *pulumirpc.LinkRequest,
) (*pulumirpc.LinkResponse, error) {
	return nil, errors.New("not implemented")
}

func (host *pclLanguageHost) Cancel(
	ctx context.Context, req *emptypb.Empty,
) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
