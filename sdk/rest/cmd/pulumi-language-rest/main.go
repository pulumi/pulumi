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

// pulumi-language-rest is a Pulumi language host that generates and executes
// declarative JSON programs via the REST gateway API.
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
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	hashihclsyntax "github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	hclsyntax "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/rest/v3/restgateway"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumiencoding "github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

func main() {
	showVersion := flag.Bool("version", false, "Print the current plugin version and exit")
	var tracing string
	flag.StringVar(&tracing, "tracing", "", "Emit tracing to a Zipkin-compatible tracing endpoint")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	var engineAddress string
	args := flag.Args()
	if len(args) > 0 {
		engineAddress = args[0]
	}

	logging.InitLogging(false, 0, false)
	cmdutil.InitTracing("pulumi-language-rest", "pulumi-language-rest", tracing)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	cancelChannel := make(chan bool)
	go func() {
		<-ctx.Done()
		cancel()
		close(cancelChannel)
	}()

	if engineAddress != "" {
		err := rpcutil.Healthcheck(ctx, engineAddress, 5*time.Minute, cancel)
		if err != nil {
			cmdutil.Exit(fmt.Errorf("could not start health check: %w", err))
		}
	}

	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancelChannel,
		Init: func(srv *grpc.Server) error {
			host := &restLanguageHost{
				engineAddress: engineAddress,
			}
			pulumirpc.RegisterLanguageRuntimeServer(srv, host)
			return nil
		},
		Options: rpcutil.OpenTracingServerInterceptorOptions(nil),
	})
	if err != nil {
		cmdutil.Exit(fmt.Errorf("could not start language host RPC server: %w", err))
	}

	fmt.Fprintf(os.Stdout, "%d\n", handle.Port)

	if err := <-handle.Done; err != nil {
		cmdutil.Exit(fmt.Errorf("language host RPC stopped serving: %w", err))
	}
}

// restLanguageHost implements the LanguageRuntimeServer interface.
type restLanguageHost struct {
	pulumirpc.UnsafeLanguageRuntimeServer
	engineAddress string
}

func (host *restLanguageHost) Handshake(
	ctx context.Context,
	req *pulumirpc.LanguageHandshakeRequest,
) (*pulumirpc.LanguageHandshakeResponse, error) {
	if req == nil || req.EngineAddress == "" {
		return nil, errors.New("must contain address in request")
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
		return nil, fmt.Errorf("could not start health check: %w", err)
	}

	return &pulumirpc.LanguageHandshakeResponse{}, nil
}

func (host *restLanguageHost) GetPluginInfo(
	ctx context.Context, req *emptypb.Empty,
) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{Version: version.Version}, nil
}

func (host *restLanguageHost) GetRequiredPlugins(
	ctx context.Context, req *pulumirpc.GetRequiredPluginsRequest,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	return &pulumirpc.GetRequiredPluginsResponse{}, nil
}

func (host *restLanguageHost) GetRequiredPackages(
	ctx context.Context, req *pulumirpc.GetRequiredPackagesRequest,
) (*pulumirpc.GetRequiredPackagesResponse, error) {
	// Read SDK marker files (*.sdk.json) from the project directory for version info.
	sdkInfo := readSDKMarkerFiles(req.Info.ProgramDirectory)

	// Check if we have a generated program.json (post-GenerateProject).
	prog, err := readDeclarativeProgram(req.Info.ProgramDirectory)
	if err == nil && (len(prog.Resources) > 0 || len(prog.Invokes) > 0) {
		seen := map[string]bool{}
		var packages []*pulumirpc.PackageDependency
		for _, res := range prog.Resources {
			pkg := packageFromToken(res.Type)
			if pkg != "" && !seen[pkg] {
				seen[pkg] = true
				dep := &pulumirpc.PackageDependency{
					Name: pkg,
					Kind: "resource",
				}
				if info, ok := sdkInfo[pkg]; ok {
					dep.Version = info.Version
				}
				packages = append(packages, dep)
			}
		}
		for _, inv := range prog.Invokes {
			pkg := packageFromToken(inv.Token)
			if pkg != "" && !seen[pkg] {
				seen[pkg] = true
				dep := &pulumirpc.PackageDependency{
					Name: pkg,
					Kind: "resource",
				}
				if info, ok := sdkInfo[pkg]; ok {
					dep.Version = info.Version
				}
				packages = append(packages, dep)
			}
		}
		return &pulumirpc.GetRequiredPackagesResponse{Packages: packages}, nil
	}

	// Fall back to reading PCL source files for dependencies (pre-GenerateProject).
	packages, err := readPCLDependencies(req.Info.ProgramDirectory)
	if err != nil {
		return &pulumirpc.GetRequiredPackagesResponse{}, nil
	}
	return &pulumirpc.GetRequiredPackagesResponse{Packages: packages}, nil
}

func (host *restLanguageHost) GetProgramDependencies(
	ctx context.Context, req *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	// Read the generated program.json and report provider dependencies.
	prog, err := readDeclarativeProgram(req.Info.ProgramDirectory)
	if err != nil {
		return &pulumirpc.GetProgramDependenciesResponse{}, nil
	}

	seen := map[string]bool{}
	var deps []*pulumirpc.DependencyInfo
	for _, res := range prog.Resources {
		pkg := packageFromToken(res.Type)
		if pkg != "" && !seen[pkg] {
			seen[pkg] = true
			deps = append(deps, &pulumirpc.DependencyInfo{
				Name: pkg,
			})
		}
	}

	return &pulumirpc.GetProgramDependenciesResponse{Dependencies: deps}, nil
}

func (host *restLanguageHost) Run(
	ctx context.Context, req *pulumirpc.RunRequest,
) (*pulumirpc.RunResponse, error) {
	if host.engineAddress == "" {
		return nil, errors.New("must call Handshake before Run")
	}
	if req.Info == nil {
		return nil, errors.New("missing program info")
	}

	// Start an in-process REST gateway wrapping the engine's monitor.
	sess, err := restgateway.NewSessionFromMonitor(
		ctx,
		req.GetMonitorAddress(),
		host.engineAddress,
		req.GetProject(),
		req.GetStack(),
	)
	if err != nil {
		return &pulumirpc.RunResponse{Error: fmt.Sprintf("creating session: %v", err)}, nil
	}

	gw := restgateway.NewGateway()
	gw.AddSession(sess)

	// Check if a program.json exists (batch mode for conformance tests).
	programPath := filepath.Join(req.Info.ProgramDirectory, "program.json")
	if _, statErr := os.Stat(programPath); statErr == nil {
		// Batch mode: read and execute the program.json.
		prog, err := readDeclarativeProgram(req.Info.ProgramDirectory)
		if err != nil {
			return &pulumirpc.RunResponse{Error: fmt.Sprintf("reading program: %v", err)}, nil
		}

		ts := httptest.NewServer(gw.Handler())
		defer ts.Close()

		if err := executeProgram(ctx, ts.URL, sess.ID, prog); err != nil {
			return &pulumirpc.RunResponse{Error: err.Error()}, nil
		}

		return &pulumirpc.RunResponse{}, nil
	}

	// Interactive mode: start a real HTTP server and wait for the user
	// to send requests. The session ends when DELETE /sessions/:id is called.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return &pulumirpc.RunResponse{Error: fmt.Sprintf("starting HTTP server: %v", err)}, nil
	}

	server := &http.Server{Handler: gw.Handler()}
	go server.Serve(listener) //nolint:errcheck // shutdown handled below

	addr := fmt.Sprintf("http://%s", listener.Addr().String())
	fmt.Fprintf(os.Stderr, "\n=== REST Gateway ready ===\n")
	fmt.Fprintf(os.Stderr, "Session ID: %s\n", sess.ID)
	fmt.Fprintf(os.Stderr, "Base URL:   %s\n", addr)
	fmt.Fprintf(os.Stderr, "\nRegister resources:\n")
	fmt.Fprintf(os.Stderr, "  curl -X POST %s/sessions/%s/resources -d '{...}'\n", addr, sess.ID)
	fmt.Fprintf(os.Stderr, "\nFinish and apply:\n")
	fmt.Fprintf(os.Stderr, "  curl -X DELETE %s/sessions/%s\n", addr, sess.ID)
	fmt.Fprintf(os.Stderr, "==========================\n\n")

	// Block until the session is closed via DELETE or context is cancelled.
	select {
	case <-sess.Done():
		// User called DELETE — session is closed.
	case <-ctx.Done():
		// Context cancelled (e.g. Ctrl+C).
		sess.Close(ctx, nil)
	}

	server.Close()

	return &pulumirpc.RunResponse{}, nil
}

func (host *restLanguageHost) InstallDependencies(
	req *pulumirpc.InstallDependenciesRequest,
	server pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	return nil
}

func (host *restLanguageHost) RuntimeOptionsPrompts(
	ctx context.Context, req *pulumirpc.RuntimeOptionsRequest,
) (*pulumirpc.RuntimeOptionsResponse, error) {
	return &pulumirpc.RuntimeOptionsResponse{}, nil
}

func (host *restLanguageHost) About(
	ctx context.Context, req *pulumirpc.AboutRequest,
) (*pulumirpc.AboutResponse, error) {
	return &pulumirpc.AboutResponse{}, nil
}

func (host *restLanguageHost) GenerateProgram(
	ctx context.Context, req *pulumirpc.GenerateProgramRequest,
) (*pulumirpc.GenerateProgramResponse, error) {
	loader, err := schema.NewLoaderClient(req.LoaderTarget)
	if err != nil {
		return nil, err
	}
	defer loader.Close()

	parser := hclsyntax.NewParser()
	for path, contents := range req.Source {
		if err := parser.ParseFile(strings.NewReader(contents), path); err != nil {
			return nil, err
		}
	}

	options := []pcl.BindOption{
		pcl.Loader(schema.NewCachedLoader(loader)),
		pcl.PreferOutputVersionedInvokes,
	}
	if !req.Strict {
		options = append(options, pcl.NonStrictBindOptions()...)
	}

	program, diags, err := pcl.BindProgram(parser.Files, options...)
	if err != nil {
		return nil, err
	}

	rpcDiags := plugin.HclDiagnosticsToRPCDiagnostics(diags)
	if diags.HasErrors() {
		return &pulumirpc.GenerateProgramResponse{Diagnostics: rpcDiags}, nil
	}
	if program == nil {
		return nil, errors.New("internal error: program was nil")
	}

	source, genDiags, err := generateProgram(program)
	if err != nil {
		return nil, err
	}
	if genDiags != nil {
		rpcDiags = append(rpcDiags, plugin.HclDiagnosticsToRPCDiagnostics(genDiags)...)
	}

	return &pulumirpc.GenerateProgramResponse{
		Source:      source,
		Diagnostics: rpcDiags,
	}, nil
}

func (host *restLanguageHost) GenerateProject(
	ctx context.Context, req *pulumirpc.GenerateProjectRequest,
) (*pulumirpc.GenerateProjectResponse, error) {
	loader, err := schema.NewLoaderClient(req.LoaderTarget)
	if err != nil {
		return nil, err
	}
	defer loader.Close()

	bindOptions := []pcl.BindOption{
		pcl.PreferOutputVersionedInvokes,
	}
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

	rpcDiags := plugin.HclDiagnosticsToRPCDiagnostics(diags)
	if diags.HasErrors() {
		return &pulumirpc.GenerateProjectResponse{Diagnostics: rpcDiags}, nil
	}
	if program == nil {
		return nil, errors.New("internal error: program was nil")
	}

	var project workspace.Project
	if err := json.Unmarshal([]byte(req.Project), &project); err != nil {
		return nil, err
	}
	project.Runtime = workspace.NewProjectRuntimeInfo("rest", nil)

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

	// Generate the declarative JSON program.
	source, genDiags, err := generateProgram(program)
	if err != nil {
		return nil, err
	}
	if genDiags != nil {
		rpcDiags = append(rpcDiags, plugin.HclDiagnosticsToRPCDiagnostics(genDiags)...)
	}

	for filename, content := range source {
		dest := filepath.Join(req.TargetDirectory, filename)
		if err := os.WriteFile(dest, content, 0o600); err != nil {
			return nil, fmt.Errorf("write %s: %w", filename, err)
		}
	}

	// Copy local dependencies (packed SDK marker files) to the project directory.
	// These contain version information needed by GetRequiredPackages.
	for name, depPath := range req.LocalDependencies {
		data, err := os.ReadFile(depPath)
		if err != nil {
			continue
		}
		dst := filepath.Join(req.TargetDirectory, name+".sdk.json")
		os.WriteFile(dst, data, 0o600)
	}

	return &pulumirpc.GenerateProjectResponse{Diagnostics: rpcDiags}, nil
}

func (host *restLanguageHost) GeneratePackage(
	ctx context.Context, req *pulumirpc.GeneratePackageRequest,
) (*pulumirpc.GeneratePackageResponse, error) {
	// REST programs don't need SDKs — they reference providers by token directly.
	// We write a minimal marker file so the conformance test framework is happy.
	if err := os.MkdirAll(req.Directory, 0o700); err != nil {
		return nil, err
	}

	var spec schema.PackageSpec
	if err := json.Unmarshal([]byte(req.Schema), &spec); err != nil {
		return nil, err
	}

	marker := map[string]string{
		"name":    spec.Name,
		"version": spec.Version,
	}
	data, _ := json.MarshalIndent(marker, "", "  ")
	dest := filepath.Join(req.Directory, spec.Name+".json")
	if err := os.WriteFile(dest, data, 0o600); err != nil {
		return nil, err
	}

	return &pulumirpc.GeneratePackageResponse{}, nil
}

func (host *restLanguageHost) Pack(
	ctx context.Context, req *pulumirpc.PackRequest,
) (*pulumirpc.PackResponse, error) {
	// Copy marker files to destination.
	if err := os.MkdirAll(req.DestinationDirectory, 0o700); err != nil {
		return nil, err
	}

	files, err := os.ReadDir(req.PackageDirectory)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		src := filepath.Join(req.PackageDirectory, file.Name())
		dst := filepath.Join(req.DestinationDirectory, file.Name())
		data, err := os.ReadFile(src)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(dst, data, 0o600); err != nil {
			return nil, err
		}
	}

	// Return first file as artifact.
	if len(files) > 0 {
		return &pulumirpc.PackResponse{
			ArtifactPath: filepath.Join(req.DestinationDirectory, files[0].Name()),
		}, nil
	}
	return &pulumirpc.PackResponse{}, nil
}

func (host *restLanguageHost) Link(
	ctx context.Context, req *pulumirpc.LinkRequest,
) (*pulumirpc.LinkResponse, error) {
	return nil, errors.New("not implemented")
}

func (host *restLanguageHost) Cancel(
	ctx context.Context, req *emptypb.Empty,
) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (host *restLanguageHost) Template(
	ctx context.Context, req *pulumirpc.TemplateRequest,
) (*pulumirpc.TemplateResponse, error) {
	return &pulumirpc.TemplateResponse{}, nil
}

func (host *restLanguageHost) RunPlugin(
	req *pulumirpc.RunPluginRequest,
	server pulumirpc.LanguageRuntime_RunPluginServer,
) error {
	return errors.New("not implemented")
}

// --- Helpers ---

func readDeclarativeProgram(dir string) (*DeclarativeProgram, error) {
	data, err := os.ReadFile(filepath.Join(dir, "program.json"))
	if err != nil {
		// Empty program (l1-empty test case).
		if os.IsNotExist(err) {
			return &DeclarativeProgram{}, nil
		}
		return nil, err
	}

	var prog DeclarativeProgram
	if err := json.Unmarshal(data, &prog); err != nil {
		return nil, fmt.Errorf("parsing program.json: %w", err)
	}
	return &prog, nil
}

// readPCLDependencies scans PCL files in a directory for resource and invoke tokens
// and returns the required package dependencies.
func readPCLDependencies(dir string) ([]*pulumirpc.PackageDependency, error) {
	parser := hclsyntax.NewParser()
	parseDiags, err := pcl.ParseDirectory(parser, dir)
	if err != nil {
		return nil, err
	}
	if parseDiags.HasErrors() {
		return nil, parseDiags
	}

	descriptors, diags := pcl.ReadAllPackageDescriptors(parser.Files)
	if diags.HasErrors() {
		return nil, diags
	}

	// Also scan resource blocks and invoke calls for implicit dependencies,
	// like the PCL host does.
	for _, file := range parser.Files {
		for _, item := range model.SourceOrderBody(file.Body) {
			block, ok := item.(*hashihclsyntax.Block)
			if !ok || block.Type != "resource" || len(block.Labels) < 2 {
				continue
			}
			pkg := packageFromToken(block.Labels[1])
			if pkg != "" {
				if _, exists := descriptors[pkg]; !exists {
					descriptors[pkg] = &schema.PackageDescriptor{Name: pkg}
				}
			}
		}
	}

	var packages []*pulumirpc.PackageDependency
	for _, desc := range descriptors {
		pkg := &pulumirpc.PackageDependency{
			Name:   desc.Name,
			Kind:   "resource",
			Server: desc.DownloadURL,
		}
		if desc.Version != nil {
			pkg.Version = desc.Version.String()
		}
		if desc.Parameterization != nil {
			pkg.Parameterization = &pulumirpc.PackageParameterization{
				Name:    desc.Parameterization.Name,
				Version: desc.Parameterization.Version.String(),
				Value:   desc.Parameterization.Value,
			}
		}
		packages = append(packages, pkg)
	}

	return packages, nil
}

// sdkMarker represents the JSON marker file written by GeneratePackage.
type sdkMarker struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// readSDKMarkerFiles reads *.sdk.json marker files from a directory.
func readSDKMarkerFiles(dir string) map[string]sdkMarker {
	result := map[string]sdkMarker{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return result
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sdk.json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var marker sdkMarker
		if err := json.Unmarshal(data, &marker); err != nil {
			continue
		}
		if marker.Name != "" {
			result[marker.Name] = marker
		}
	}
	return result
}

func packageFromToken(token string) string {
	parts := strings.SplitN(token, ":", 3)
	if len(parts) < 2 {
		return ""
	}
	if parts[0] == "pulumi" {
		return ""
	}
	return parts[0]
}

// executeProgram runs the declarative JSON program by making REST API calls.
func executeProgram(ctx context.Context, baseURL, sessionID string, prog *DeclarativeProgram) error {
	// Track registered resource outputs for reference resolution.
	resourceOutputs := map[string]restgateway.RegisterResourceResponse{}

	// Register resources in order (they're already linearized).
	for _, res := range prog.Resources {
		// Resolve ${...} references in properties.
		props := resolveProperties(res.Properties, resourceOutputs)

		// Resolve dependency URNs.
		var depURNs []string
		for _, dep := range res.Dependencies {
			if out, ok := resourceOutputs[dep]; ok {
				depURNs = append(depURNs, out.URN)
			}
		}

		reqBody := restgateway.RegisterResourceRequest{
			Type:         res.Type,
			Name:         res.Name,
			Custom:       res.Custom,
			Properties:   props,
			Dependencies: depURNs,
		}
		if res.Parent != "" {
			reqBody.Parent = fmt.Sprintf("%v", resolveString(res.Parent, resourceOutputs))
		}
		if res.Options != nil {
			reqBody.Protect = res.Options.Protect
			reqBody.RetainOnDelete = res.Options.RetainOnDelete
			reqBody.DeleteBeforeReplace = res.Options.DeleteBeforeReplace
			reqBody.IgnoreChanges = res.Options.IgnoreChanges
			reqBody.ReplaceOnChanges = res.Options.ReplaceOnChanges
			reqBody.AdditionalSecretOutputs = res.Options.AdditionalSecretOutputs
			reqBody.HideDiffs = res.Options.HideDiffs
			reqBody.Version = res.Options.Version
			reqBody.PluginDownloadURL = res.Options.PluginDownloadURL
			reqBody.ImportID = res.Options.ImportID
			// Resolve replaceWith resource references to URNs.
			for _, ref := range res.Options.ReplaceWith {
				resolved := fmt.Sprintf("%v", resolveValue(ref, resourceOutputs))
				reqBody.ReplaceWith = append(reqBody.ReplaceWith, resolved)
			}
		}

		var resp restgateway.RegisterResourceResponse
		if err := doHTTP(ctx, baseURL, "POST", "/sessions/"+sessionID+"/resources", reqBody, &resp); err != nil {
			return fmt.Errorf("register resource %s: %w", res.Name, err)
		}
		resourceOutputs[res.Name] = resp
	}

	// Resolve outputs and close session.
	exports := map[string]interface{}{}
	for name, ref := range prog.Outputs {
		exports[name] = resolveValue(ref, resourceOutputs)
	}

	if err := doHTTP(ctx, baseURL, "DELETE", "/sessions/"+sessionID, restgateway.DeleteSessionRequest{
		Exports: exports,
	}, nil); err != nil {
		return fmt.Errorf("close session: %w", err)
	}

	return nil
}

// resolveProperties resolves ${...} references in a properties map.
func resolveProperties(props map[string]interface{}, outputs map[string]restgateway.RegisterResourceResponse) map[string]interface{} {
	if props == nil {
		return nil
	}
	result := make(map[string]interface{}, len(props))
	for k, v := range props {
		result[k] = resolveValue(v, outputs)
	}
	return result
}

// resolveValue resolves ${...} references in a value.
func resolveValue(v interface{}, outputs map[string]restgateway.RegisterResourceResponse) interface{} {
	switch val := v.(type) {
	case string:
		return resolveString(val, outputs)
	case map[string]interface{}:
		return resolveProperties(val, outputs)
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, item := range val {
			result[i] = resolveValue(item, outputs)
		}
		return result
	default:
		return v
	}
}

// resolveString resolves "${name.field}" references in a string.
func resolveString(s string, outputs map[string]restgateway.RegisterResourceResponse) interface{} {
	// Check if the entire string is a single reference.
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") && strings.Count(s, "${") == 1 {
		ref := s[2 : len(s)-1]
		return lookupRef(ref, outputs)
	}

	// Otherwise, interpolate references within the string.
	result := s
	for {
		start := strings.Index(result, "${")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		end += start
		ref := result[start+2 : end]
		val := lookupRef(ref, outputs)
		result = result[:start] + fmt.Sprintf("%v", val) + result[end+1:]
	}
	return result
}

// lookupRef looks up a dotted reference like "myBucket.bucket" in resource outputs.
func lookupRef(ref string, outputs map[string]restgateway.RegisterResourceResponse) interface{} {
	parts := strings.SplitN(ref, ".", 2)
	resName := parts[0]
	out, ok := outputs[resName]
	if !ok {
		return "${" + ref + "}"
	}

	if len(parts) == 1 {
		return out.URN
	}

	field := parts[1]
	switch field {
	case "urn":
		return out.URN
	case "id":
		return out.ID
	default:
		if out.Properties != nil {
			if val, ok := out.Properties[field]; ok {
				return val
			}
		}
		return "${" + ref + "}"
	}
}

// doHTTP sends an HTTP request to the REST gateway.
func doHTTP(ctx context.Context, baseURL, method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errResp restgateway.ErrorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, errResp.Error)
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}

	return nil
}
