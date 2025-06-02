// Copyright 2016-2025, Pulumi Corporation.
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
	b64 "encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/cmd/pulumi-test-language/tests"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	backendDisplay "github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	b64secrets "github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/pkg/v3/util/gsync"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	testingrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/testing"
	"github.com/segmentio/encoding/json"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pbempty "google.golang.org/protobuf/types/known/emptypb"
)

type LanguageTestServer interface {
	testingrpc.LanguageTestServer
	pulumirpc.EngineServer

	// Address returns the address at which the test RPC server may be reached.
	Address() string

	// Cancel signals that the test server should be terminated.
	Cancel()

	// Done awaits the test servers termination, and returns any errors that result.
	Done() error
}

func newLanguageTestServer() *languageTestServer {
	return &languageTestServer{
		sdkLocks:    gsync.Map[string, *sync.Mutex]{},
		artifactMap: gsync.Map[string, string]{},
	}
}

func installDependencies(
	languageClient plugin.LanguageRuntime,
	programInfo plugin.ProgramInfo,
	isPlugin bool,
) *testingrpc.RunLanguageTestResponse {
	installStdout, installStderr, installDone, err := languageClient.InstallDependencies(
		plugin.InstallDependenciesRequest{Info: programInfo, IsPlugin: isPlugin},
	)
	if err != nil {
		return makeTestResponse(fmt.Sprintf("install dependencies: %v", err))
	}

	// We'll use a WaitGroup to wait for the stdout (1) and stderr (2) readers to be fully drained, as well as for the
	// done channel to close (3), before we carry on.
	var wg sync.WaitGroup
	wg.Add(3)

	var installStdoutBytes []byte
	var installStderrBytes []byte

	installErrorChan := make(chan error, 3)

	go func() {
		defer wg.Done()
		var err error
		if installStdoutBytes, err = io.ReadAll(installStdout); err != nil {
			installErrorChan <- err
		}
	}()

	go func() {
		defer wg.Done()
		var err error
		if installStderrBytes, err = io.ReadAll(installStderr); err != nil {
			installErrorChan <- err
		}
	}()

	go func() {
		defer wg.Done()
		if err := <-installDone; err != nil {
			installErrorChan <- err
		}
	}()

	var installErrs []error
	wg.Wait()
	close(installErrorChan)
	for err := range installErrorChan {
		if err != nil {
			installErrs = append(installErrs, err)
		}
	}

	err = errors.Join(installErrs...)
	if err != nil {
		return &testingrpc.RunLanguageTestResponse{
			Success:  false,
			Messages: []string{fmt.Sprintf("install dependencies: %v", err)},
			Stdout:   string(installStdoutBytes),
			Stderr:   string(installStderrBytes),
		}
	}

	return nil
}

func Start(ctx context.Context) (LanguageTestServer, error) {
	// New up an engine RPC server.
	server := &languageTestServer{
		ctx:         ctx,
		cancel:      make(chan bool),
		sdkLocks:    gsync.Map[string, *sync.Mutex]{},
		artifactMap: gsync.Map[string, string]{},
	}

	// Fire up a gRPC server and start listening for incomings.
	port, done, err := rpcutil.Serve(0, server.cancel, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			testingrpc.RegisterLanguageTestServer(srv, server)
			pulumirpc.RegisterEngineServer(srv, server)
			return nil
		},
	}, nil)
	if err != nil {
		return nil, err
	}

	server.addr = fmt.Sprintf("127.0.0.1:%d", port)
	server.done = done

	return server, nil
}

// languageTestServer is the server side of the language testing RPC machinery.
type languageTestServer struct {
	testingrpc.UnsafeLanguageTestServer
	pulumirpc.UnimplementedEngineServer

	ctx    context.Context
	cancel chan bool
	done   chan error
	addr   string

	sdkLocks gsync.Map[string, *sync.Mutex]

	// A map storing the paths to the generated package artifacts
	artifactMap gsync.Map[string, string]

	// Used by _bad snapshot_ tests to disable snapshot writing.
	DisableSnapshotWriting bool

	logLock sync.Mutex
	// Used by the Log method to track the number of times a message has been repeated.
	logRepeat int
	// Used by the Log method to track the last message logged, this is so we can elide duplicate messages.
	previousMessage string
}

func (eng *languageTestServer) Address() string {
	return eng.addr
}

func (eng *languageTestServer) Cancel() {
	eng.cancel <- true
}

func (eng *languageTestServer) Done() error {
	return <-eng.done
}

func (eng *languageTestServer) Log(_ context.Context, req *pulumirpc.LogRequest) (*pbempty.Empty, error) {
	eng.logLock.Lock()
	defer eng.logLock.Unlock()

	var sev diag.Severity
	switch req.Severity {
	case pulumirpc.LogSeverity_DEBUG:
		sev = diag.Debug
	case pulumirpc.LogSeverity_INFO:
		sev = diag.Info
	case pulumirpc.LogSeverity_WARNING:
		sev = diag.Warning
	case pulumirpc.LogSeverity_ERROR:
		sev = diag.Error
	default:
		return nil, fmt.Errorf("Unrecognized logging severity: %v", req.Severity)
	}

	message := req.Message
	if os.Getenv("PULUMI_LANGUAGE_TEST_SHOW_FULL_OUTPUT") != "true" {
		// Cut down logs so they don't overwhelm the test output
		if len(message) > 2048 {
			message = message[:2048] + "... (truncated, run with PULUMI_LANGUAGE_TEST_SHOW_FULL_OUTPUT=true to see full logs))"
		}
	}

	if eng.previousMessage == message {
		eng.logRepeat++
		return &pbempty.Empty{}, nil
	}

	if eng.logRepeat > 1 {
		_, err := fmt.Fprintf(os.Stderr, "Last message repeated %d times\n", eng.logRepeat)
		if err != nil {
			return nil, err
		}
	}
	eng.logRepeat = 1
	eng.previousMessage = message

	var err error
	if req.StreamId != 0 {
		_, err = fmt.Fprintf(os.Stderr, "(%d) %s[%s]: %s", req.StreamId, sev, req.Urn, message)
	} else {
		_, err = fmt.Fprintf(os.Stderr, "%s[%s]: %s", sev, req.Urn, message)
	}
	if err != nil {
		return nil, err
	}

	return &pbempty.Empty{}, nil
}

// A providerLoader is a schema loader that loads schemas from a given set of providers.
type providerLoader struct {
	language, languageInfo string

	host plugin.Host
}

func (l *providerLoader) LoadPackageReference(pkg string, version *semver.Version) (schema.PackageReference, error) {
	return l.LoadPackageReferenceV2(context.TODO(), &schema.PackageDescriptor{
		Name:    pkg,
		Version: version,
	})
}

func (l *providerLoader) LoadPackageReferenceV2(
	ctx context.Context, descriptor *schema.PackageDescriptor,
) (schema.PackageReference, error) {
	if descriptor.Name == "pulumi" {
		return schema.DefaultPulumiPackage.Reference(), nil
	}

	// Defer to the host to find the provider for the given package descriptor.
	workspaceDescriptor := workspace.PackageDescriptor{
		PluginSpec: workspace.PluginSpec{
			Kind:              apitype.ResourcePlugin,
			Name:              descriptor.Name,
			Version:           descriptor.Version,
			PluginDownloadURL: descriptor.DownloadURL,
		},
	}
	if descriptor.Parameterization != nil {
		workspaceDescriptor.Parameterization = &workspace.Parameterization{
			Name:    descriptor.Parameterization.Name,
			Version: descriptor.Parameterization.Version,
			Value:   descriptor.Parameterization.Value,
		}
	}

	provider, err := l.host.Provider(workspaceDescriptor)
	if err != nil {
		return nil, fmt.Errorf("could not load schema for %s: %w", descriptor.Name, err)
	}

	if provider == nil {
		return nil, fmt.Errorf("could not load schema for %s, provider not known", descriptor.Name)
	}

	getSchemaRequest := plugin.GetSchemaRequest{}
	if descriptor.Parameterization != nil {
		parameter := &plugin.ParameterizeValue{
			Name:    descriptor.Parameterization.Name,
			Version: descriptor.Parameterization.Version,
			Value:   descriptor.Parameterization.Value,
		}

		_, err := provider.Parameterize(ctx, plugin.ParameterizeRequest{
			Parameters: parameter,
		})
		if err != nil {
			return nil, fmt.Errorf("parameterize package '%s' failed: %w", descriptor.Name, err)
		}

		getSchemaRequest.SubpackageName = descriptor.Parameterization.Name
		getSchemaRequest.SubpackageVersion = &descriptor.Parameterization.Version
	}

	jsonSchema, err := provider.GetSchema(context.TODO(), getSchemaRequest)
	if err != nil {
		return nil, fmt.Errorf("get schema for %s: %w", descriptor.Name, err)
	}

	var spec schema.PartialPackageSpec
	if _, err := json.Parse(jsonSchema.Schema, &spec, json.ZeroCopy); err != nil {
		return nil, err
	}

	// Unconditionally set SupportPack
	if spec.Meta == nil {
		spec.Meta = &schema.MetadataSpec{}
	}
	spec.Meta.SupportPack = true

	// Set the LanguageInfo field if given
	if l.languageInfo != "" {
		// We don't expect the language field to be set in the core providers, they should be language agnostic
		spec.Language = map[string]schema.RawMessage{
			l.language: schema.RawMessage(l.languageInfo),
		}
	}

	p, err := schema.ImportPartialSpec(spec, nil, l)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (l *providerLoader) LoadPackage(pkg string, version *semver.Version) (*schema.Package, error) {
	ref, err := l.LoadPackageReference(pkg, version)
	if err != nil {
		return nil, err
	}
	return ref.Definition()
}

func (l *providerLoader) LoadPackageV2(
	ctx context.Context, descriptor *schema.PackageDescriptor,
) (*schema.Package, error) {
	ref, err := l.LoadPackageReferenceV2(ctx, descriptor)
	if err != nil {
		return nil, err
	}
	return ref.Definition()
}

func (eng *languageTestServer) GetLanguageTests(
	ctx context.Context,
	req *testingrpc.GetLanguageTestsRequest,
) (*testingrpc.GetLanguageTestsResponse, error) {
	filtered := make([]string, 0, len(tests.LanguageTests))
	for testName := range tests.LanguageTests {
		// Don't return internal tests
		if strings.HasPrefix(testName, "internal-") {
			continue
		}
		filtered = append(filtered, testName)
	}

	return &testingrpc.GetLanguageTestsResponse{
		Tests: filtered,
	}, nil
}

func makeTestResponse(msg string) *testingrpc.RunLanguageTestResponse {
	return &testingrpc.RunLanguageTestResponse{
		Success:  false,
		Messages: []string{msg},
	}
}

type replacement struct {
	Path        string
	Pattern     string
	Replacement string
}

type compiledReplacement struct {
	Path        *regexp.Regexp
	Pattern     *regexp.Regexp
	Replacement string
}

// programOverrides represent overrides whereby a test may specify a set of hardcoded or pre-generated programs to be
// used, in place of running GenerateProject on source PCL. This is useful for testing SDK functionality when the
// requisite program code generation is not yet complete enough to support generating programs which exercise that
// functionality.
type programOverride struct {
	// A list of paths to directories containing programs to use for the test. The length of this list should correspond
	// to the number of `Runs` in the test, with each entry being used for the corresponding run (e.g. entry 0 for run 0,
	// entry 1 for run 1, etc.).
	Paths []string
}

type testToken struct {
	LanguagePluginName   string
	LanguagePluginTarget string
	TemporaryDirectory   string
	SnapshotDirectory    string
	CoreArtifact         string
	CoreVersion          string
	SnapshotEdits        []replacement
	LanguageInfo         string
	ProgramOverrides     map[string]programOverride
	PolicyPackDirectory  string
}

func (eng *languageTestServer) PrepareLanguageTests(
	ctx context.Context,
	req *testingrpc.PrepareLanguageTestsRequest,
) (*testingrpc.PrepareLanguageTestsResponse, error) {
	if req.LanguagePluginName == "" {
		return nil, errors.New("language plugin name must be specified")
	}
	if req.LanguagePluginTarget == "" {
		return nil, errors.New("language plugin target must be specified")
	}
	if req.SnapshotDirectory == "" {
		return nil, errors.New("snapshot directory must be specified")
	}
	if req.TemporaryDirectory == "" {
		return nil, errors.New("temporary directory must be specified")
	}

	err := os.MkdirAll(req.SnapshotDirectory, 0o755)
	if err != nil {
		return nil, fmt.Errorf("create snapshot directory %s: %w", req.SnapshotDirectory, err)
	}

	err = os.RemoveAll(req.TemporaryDirectory)
	if err != nil {
		return nil, fmt.Errorf("remove temporary directory %s: %w", req.TemporaryDirectory, err)
	}

	err = os.MkdirAll(req.TemporaryDirectory, 0o755)
	if err != nil {
		return nil, fmt.Errorf("create temporary directory %s: %w", req.TemporaryDirectory, err)
	}

	// Create a diagnostics sink for setup
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	snk := diag.DefaultSink(stdout, stderr, diag.FormatOptions{
		Color: colors.Never,
	})

	// Start up a plugin context
	pctx, err := plugin.NewContextWithRoot(ctx, snk, snk, nil, "", "", nil, false, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("setup plugin context: %w", err)
	}
	defer func() {
		contract.IgnoreError(pctx.Close())
	}()

	// Connect to the language host
	conn, err := grpc.NewClient(req.LanguagePluginTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial language plugin: %w", err)
	}

	languageClient := plugin.NewLanguageRuntimeClient(
		pctx, req.LanguagePluginName, pulumirpc.NewLanguageRuntimeClient(conn))

	// Setup the artifacts directory
	err = os.MkdirAll(filepath.Join(req.TemporaryDirectory, "artifacts"), 0o755)
	if err != nil {
		return nil, fmt.Errorf("create artifacts directory: %w", err)
	}

	var coreArtifact string
	if req.CoreSdkDirectory != "" {
		// Build the core SDK, use a slightly odd version so we can test dependencies later.
		coreArtifact, err = languageClient.Pack(
			req.CoreSdkDirectory, filepath.Join(req.TemporaryDirectory, "artifacts"))
		if err != nil {
			return nil, fmt.Errorf("pack core SDK: %w", err)
		}
	}

	edits := []replacement{}
	for _, replace := range req.SnapshotEdits {
		edits = append(edits, replacement{
			Path:        replace.Path,
			Pattern:     replace.Pattern,
			Replacement: replace.Replacement,
		})
	}

	programOverrides := map[string]programOverride{}
	for testName, override := range req.ProgramOverrides {
		test, has := tests.LanguageTests[testName]
		if !has {
			return nil, fmt.Errorf("program override for non-existent test: %s", testName)
		}

		if test.Runs != nil && len(test.Runs) != len(override.Paths) {
			return nil, fmt.Errorf(
				"program override for test %s has %d paths but %d runs",
				testName, len(override.Paths), len(test.Runs),
			)
		}

		programOverrides[testName] = programOverride{
			Paths: override.Paths,
		}
	}

	var policyPackDirectory string
	// If the policy pack directory is set, we need to absolute it so that later tests can find the policies
	// regardless of their working directory.
	if req.PolicyPackDirectory != "" {
		policyPackDirectory, err = filepath.Abs(req.PolicyPackDirectory)
		if err != nil {
			return nil, fmt.Errorf("get absolute path for policy pack directory %s: %w", req.PolicyPackDirectory, err)
		}
	}

	tokenBytes, err := json.Marshal(&testToken{
		LanguagePluginName:   req.LanguagePluginName,
		LanguagePluginTarget: req.LanguagePluginTarget,
		TemporaryDirectory:   req.TemporaryDirectory,
		SnapshotDirectory:    req.SnapshotDirectory,
		CoreArtifact:         coreArtifact,
		CoreVersion:          req.CoreSdkVersion,
		SnapshotEdits:        edits,
		LanguageInfo:         req.LanguageInfo,
		ProgramOverrides:     programOverrides,
		PolicyPackDirectory:  policyPackDirectory,
	})
	contract.AssertNoErrorf(err, "could not marshal test token")

	b64token := b64.StdEncoding.EncodeToString(tokenBytes)

	return &testingrpc.PrepareLanguageTestsResponse{
		Token: b64token,
	}, nil
}

func getProviderVersion(provider plugin.Provider) (semver.Version, error) {
	pkg := provider.Pkg()
	info, err := provider.GetPluginInfo(context.TODO())
	if err != nil {
		return semver.Version{}, fmt.Errorf("get plugin info for %s: %w", pkg, err)
	}
	if info.Version == nil {
		return semver.Version{}, fmt.Errorf("provider %s has no version", pkg)
	}
	return *info.Version, nil
}

func hasDependency(pkg *schema.Package, dep string) bool {
	for _, d := range pkg.Dependencies {
		if d.Name == dep {
			return true
		}
	}
	return false
}

// TODO(https://github.com/pulumi/pulumi/issues/13944): We need a RunLanguageTest(t *testing.T) function that
// handles the machinery of plugging the language test logs into the testing.T.

func (eng *languageTestServer) RunLanguageTest(
	ctx context.Context, req *testingrpc.RunLanguageTestRequest,
) (*testingrpc.RunLanguageTestResponse, error) {
	test, has := tests.LanguageTests[req.Test]
	if !has {
		return nil, fmt.Errorf("unknown test %s", req.Test)
	}

	// Decode the test token
	tokenBytes, err := b64.StdEncoding.DecodeString(req.Token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	var token testToken
	err = json.Unmarshal(tokenBytes, &token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// If the language defines any snapshot edits compile those regexs to apply now
	snapshotEdits := []compiledReplacement{}
	for _, replace := range token.SnapshotEdits {
		pathRegex, err := regexp.Compile(replace.Path)
		if err != nil {
			return nil, fmt.Errorf("invalid path regex %s: %w", replace.Path, err)
		}
		editRegex, err := regexp.Compile(replace.Pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid edit regex %s: %w", replace.Pattern, err)
		}
		snapshotEdits = append(snapshotEdits, compiledReplacement{
			Path:        pathRegex,
			Pattern:     editRegex,
			Replacement: replace.Replacement,
		})
	}

	// Create a diagnostics sink for the test
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	snk := diag.DefaultSink(stdout, stderr, diag.FormatOptions{
		Color: colors.Never,
	})

	// Start up a plugin context
	pctx, err := plugin.NewContextWithRoot(
		ctx, snk, snk, nil, token.TemporaryDirectory, token.TemporaryDirectory, nil, false, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("setup plugin context: %w", err)
	}

	// NewContextWithRoot will make a default plugin host, but we want to make sure we never actually use that
	pctx.Host = nil

	// Connect to the language host
	conn, err := grpc.NewClient(token.LanguagePluginTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial language plugin: %w", err)
	}

	languageClient := plugin.NewLanguageRuntimeClient(
		pctx, token.LanguagePluginName, pulumirpc.NewLanguageRuntimeClient(conn))

	// And now replace the context host with our own test host
	providers := make(map[string]plugin.Provider)
	for _, provider := range test.Providers {
		version, err := getProviderVersion(provider)
		if err != nil {
			return nil, err
		}
		providers[fmt.Sprintf("%s@%s", provider.Pkg(), version)] = provider
	}

	host := &testHost{
		engine:      eng,
		ctx:         pctx,
		host:        pctx.Host,
		runtime:     languageClient,
		runtimeName: token.LanguagePluginName,
		providers:   providers,
		connections: make(map[plugin.Provider]io.Closer),
	}

	pctx.Host = host

	// Generate SDKs for all the packages we need
	loader := &providerLoader{
		language:     token.LanguagePluginName,
		languageInfo: token.LanguageInfo,
		host:         host,
	}
	loaderServer := schema.NewLoaderServer(loader)
	grpcServer, err := plugin.NewServer(pctx, schema.LoaderRegistration(loaderServer))
	if err != nil {
		return nil, err
	}
	defer contract.IgnoreClose(grpcServer)

	artifactsDir := filepath.Join(token.TemporaryDirectory, "artifacts")

	// For each test run collect the packages reported by PCL
	packages := []*schema.Package{}
	for i, run := range test.Runs {
		// Create a source directory for the test
		sourceDir := filepath.Join(token.TemporaryDirectory, "source", req.Test)
		if len(test.Runs) > 1 {
			sourceDir = filepath.Join(sourceDir, strconv.Itoa(i))
		}
		err = os.MkdirAll(sourceDir, 0o700)
		if err != nil {
			return nil, fmt.Errorf("create source dir: %w", err)
		}

		// Find and copy the tests PCL code to the source dir
		pclDir := filepath.Join("testdata", req.Test)
		if len(test.Runs) > 1 {
			pclDir = filepath.Join(pclDir, strconv.Itoa(i))
		}
		err = copyDirectory(tests.LanguageTestdata, pclDir, sourceDir, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("copy source test data: %w", err)
		}
		if run.Main != "" {
			sourceDir = filepath.Join(sourceDir, run.Main)
		}

		program, diagnostics, err := pcl.BindDirectory(sourceDir, loader)
		if err != nil {
			return nil, fmt.Errorf("bind PCL program: %v", err)
		}
		if diagnostics.HasErrors() {
			return nil, fmt.Errorf("bind PCL program: %v", diagnostics)
		}

		pkgs := program.PackageReferences()
		// We should be able to get a full def for each package
		for _, pkg := range pkgs {
			if pkg.Name() == "pulumi" {
				// No need to write the pulumi package, it's builtin to core SDKs
				continue
			}
			def, err := pkg.Definition()
			if err != nil {
				return nil, fmt.Errorf("get package definition: %w", err)
			}
			exists := false
			for _, existing := range packages {
				if existing.Name == def.Name {
					exists = true
				}
			}
			if !exists {
				packages = append(packages, def)
			}
		}
	}

	// We always override the core "pulumi" package to point to the local core SDK we built as part of test
	// setup.
	localDependencies := map[string]string{}
	if token.CoreArtifact != "" {
		localDependencies["pulumi"] = token.CoreArtifact
	}
	packageSet := make(map[string]struct{}, len(packages))
	for _, pkg := range packages {
		packageSet[pkg.Name] = struct{}{}
	}
	// Check that all dependencies are fulfilled
	for _, pkg := range packages {
		for _, dep := range pkg.Dependencies {
			if _, ok := packageSet[dep.Name]; !ok {
				return nil, fmt.Errorf("package %s depends on %s, but it is not present", pkg.Name, dep.Name)
			}
		}
	}
	// Sort the packages, so dependent packages are generated first
	sort.Slice(packages, func(i, j int) bool {
		return hasDependency(packages[j], packages[i].Name)
	})

	for _, pkg := range packages {
		sdkName := fmt.Sprintf("%s-%s", pkg.Name, pkg.Version)
		sdkTempDir := filepath.Join(token.TemporaryDirectory, "sdks", sdkName)
		// Multiple tests might try to generate the same SDK at the same time so we need to be atomic here. We do this
		// using a per-sdk lock for fine grained control. The generated SDK artifacts are then cached, and will be
		// reused.
		response, err := func() (*testingrpc.RunLanguageTestResponse, error) {
			lock, _ := eng.sdkLocks.LoadOrStore(sdkTempDir, &sync.Mutex{})
			lock.Lock()
			defer lock.Unlock()

			sdkArtifact, ok := eng.artifactMap.Load(sdkTempDir)
			if ok {
				// If the directory already exists then we know we already created the artifact.
				// Just use it
				localDependencies[pkg.Name] = sdkArtifact
				return nil, nil
			}

			err = os.MkdirAll(sdkTempDir, 0o755)
			if err != nil {
				return nil, fmt.Errorf("create temp sdks dir: %w", err)
			}

			schemaBytes, err := pkg.MarshalJSON()
			if err != nil {
				return nil, fmt.Errorf("marshal schema for provider %s: %w", pkg.Name, err)
			}

			diags, err := languageClient.GeneratePackage(
				sdkTempDir, string(schemaBytes), nil, grpcServer.Addr(), localDependencies, false)
			if err != nil {
				return makeTestResponse(fmt.Sprintf("generate package %s: %v", pkg.Name, err)), nil
			}
			// TODO: Might be good to test warning diagnostics here
			if diags.HasErrors() {
				return makeTestResponse(fmt.Sprintf("generate package %s: %v", pkg.Name, diags)), nil
			}

			snapshotDir := filepath.Join(token.SnapshotDirectory, "sdks", sdkName)
			sdkSnapshotDir, err := editSnapshot(sdkTempDir, snapshotEdits)
			if err != nil {
				return nil, fmt.Errorf("sdk snapshot creation for %s: %w", pkg.Name, err)
			}
			validations, err := doSnapshot(eng.DisableSnapshotWriting, sdkSnapshotDir, snapshotDir)
			// If we made a snapshot edit we can clean it up now
			if sdkSnapshotDir != sdkTempDir {
				err := os.RemoveAll(sdkSnapshotDir)
				if err != nil {
					return nil, fmt.Errorf("remove snapshot dir: %w", err)
				}
			}
			if err != nil {
				return nil, fmt.Errorf("sdk snapshot validation for %s: %w", pkg.Name, err)
			}
			if len(validations) > 0 {
				return makeTestResponse(
					fmt.Sprintf("sdk snapshot validation for %s failed:\n%s",
						pkg.Name, strings.Join(validations, "\n"))), nil
			}

			// Pack the SDK and add it to the artifact dependencies, we do this in the temporary directory so that
			// any intermediate build files don't end up getting captured in the snapshot folder.
			sdkArtifact, err = languageClient.Pack(sdkTempDir, artifactsDir)
			if err != nil {
				return nil, fmt.Errorf("sdk packing for %s: %w", pkg.Name, err)
			}
			localDependencies[pkg.Name] = sdkArtifact
			eng.artifactMap.Store(sdkTempDir, sdkArtifact)

			// Check that packing the SDK didn't mutate any files, but it may have added ignorable build files.
			// Again we need to make a snapshot edit for this.
			sdkSnapshotDir, err = editSnapshot(sdkTempDir, snapshotEdits)
			if err != nil {
				return nil, fmt.Errorf("sdk snapshot creation for %s: %w", pkg.Name, err)
			}
			validations, err = compareDirectories(sdkSnapshotDir, snapshotDir, true /* allowNewFiles */)
			// If we made a snapshot edit we can clean it up now
			if sdkSnapshotDir != sdkTempDir {
				err := os.RemoveAll(sdkSnapshotDir)
				if err != nil {
					return nil, fmt.Errorf("remove snapshot dir: %w", err)
				}
			}
			if err != nil {
				return nil, fmt.Errorf("sdk post pack change validation for %s: %w", pkg.Name, err)
			}
			if len(validations) > 0 {
				return makeTestResponse(
					fmt.Sprintf("sdk post pack change validation for %s failed:\n%s",
						pkg.Name, strings.Join(validations, "\n"))), nil
			}

			return nil, nil
		}()
		if response != nil || err != nil {
			return response, err
		}
	}

	// Just use base64 "secrets" for these tests
	sm := b64secrets.NewBase64SecretsManager()
	dec := sm.Decrypter()

	// Create a temp dir for the a diy backend to run in for the test
	backendDir := filepath.Join(token.TemporaryDirectory, "backends", req.Test)
	err = os.MkdirAll(backendDir, 0o755)
	if err != nil {
		return nil, fmt.Errorf("create temp backend dir: %w", err)
	}
	testBackend, err := diy.New(ctx, snk, "file://"+backendDir, nil)
	if err != nil {
		return nil, fmt.Errorf("create diy backend: %w", err)
	}

	// Create any stack references needed for the test
	for name, outputs := range test.StackReferences {
		ref, err := testBackend.ParseStackReference(name)
		if err != nil {
			return nil, fmt.Errorf("parse test stack reference: %w", err)
		}

		s, err := testBackend.CreateStack(ctx, ref, "", nil, nil)
		if err != nil {
			return nil, fmt.Errorf("create test stack reference: %w", err)
		}

		stackName := ref.Name()
		projectName, has := ref.Project()
		if !has {
			return nil, fmt.Errorf("stack reference %s has no project", ref)
		}
		name := fmt.Sprintf("%s-%s", projectName, stackName)

		// Import the deployment for the stack reference
		snap := &deploy.Snapshot{
			SecretsManager: sm,
			Resources: []*resource.State{
				{
					Type:    resource.RootStackType,
					URN:     resource.CreateURN(name, string(resource.RootStackType), "", string(projectName), stackName.String()),
					Outputs: outputs,
				},
			},
		}
		serializedDeployment, err := stack.SerializeDeployment(ctx, snap, false)
		if err != nil {
			return nil, fmt.Errorf("serialize deployment: %w", err)
		}
		jsonDeployment, err := json.Marshal(serializedDeployment)
		if err != nil {
			return nil, fmt.Errorf("serialize deployment: %w", err)
		}

		untypedDeployment := &apitype.UntypedDeployment{
			Version:    apitype.DeploymentSchemaVersionCurrent,
			Deployment: jsonDeployment,
		}
		err = s.ImportDeployment(ctx, untypedDeployment)
		if err != nil {
			return nil, fmt.Errorf("import deployment: %w", err)
		}
	}

	var result tests.LResult
	for i, run := range test.Runs {
		// Create a source directory for the test
		sourceDir := filepath.Join(token.TemporaryDirectory, "source", req.Test)
		if len(test.Runs) > 1 {
			sourceDir = filepath.Join(sourceDir, strconv.Itoa(i))
		}
		err = os.MkdirAll(sourceDir, 0o700)
		if err != nil {
			return nil, fmt.Errorf("create source dir: %w", err)
		}

		// Find and copy the tests PCL code to the source dir
		pclDir := filepath.Join("testdata", req.Test)
		if len(test.Runs) > 1 {
			pclDir = filepath.Join(pclDir, strconv.Itoa(i))
		}
		err = copyDirectory(tests.LanguageTestdata, pclDir, sourceDir, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("copy source test data: %w", err)
		}

		// Create a directory for the project
		projectDir := filepath.Join(token.TemporaryDirectory, "projects", req.Test)
		if len(test.Runs) > 1 {
			projectDir = filepath.Join(projectDir, strconv.Itoa(i))
		}
		err = os.MkdirAll(projectDir, 0o755)
		if err != nil {
			return nil, fmt.Errorf("create project dir: %w", err)
		}

		// Generate the project and read in the Pulumi.yaml
		rootDirectory := sourceDir
		projectJSON := func() string {
			if run.Main == "" {
				return fmt.Sprintf(`{"name": "%s"}`, req.Test)
			}
			sourceDir = filepath.Join(sourceDir, run.Main)
			return fmt.Sprintf(`{"name": "%s", "main": "%s"}`, req.Test, run.Main)
		}()

		// Check the PCL is valid and get the list of packages it reports
		program, diags, err := pcl.BindDirectory(sourceDir, loader)
		if err != nil {
			return nil, fmt.Errorf("bind PCL program: %v", err)
		}
		if diags.HasErrors() {
			return nil, fmt.Errorf("bind PCL program: %v", diags)
		}
		programPackages := program.PackageReferences()

		// TODO(https://github.com/pulumi/pulumi/issues/13940): We don't report back warning diagnostics here
		var diagnostics hcl.Diagnostics

		// If an override has been supplied for the given test, we'll just copy that over as-is, instead of calling
		// GenerateProject to generate a program for testing.
		if programOverride, ok := token.ProgramOverrides[req.Test]; ok {
			err = copyDirectory(os.DirFS(programOverride.Paths[i]), ".", projectDir, nil, nil)
			if err != nil {
				return nil, fmt.Errorf("copy override testdata: %w", err)
			}
		} else {
			diagnostics, err = languageClient.GenerateProject(
				sourceDir, projectDir, projectJSON, true, grpcServer.Addr(), localDependencies)
			if err != nil {
				return makeTestResponse(fmt.Sprintf("generate project: %v", err)), nil
			}
			if diagnostics.HasErrors() {
				return makeTestResponse(fmt.Sprintf("generate project: %v", diagnostics)), nil
			}

			// GenerateProject only handles the .pp source files it doesn't copy across other files like testdata so we copy
			// them across here.
			err = copyDirectory(os.DirFS(rootDirectory), ".", projectDir, nil, []string{".pp"})
			if err != nil {
				return nil, fmt.Errorf("copy testdata: %w", err)
			}

			snapshotDir := filepath.Join(token.SnapshotDirectory, "projects", req.Test)
			if len(test.Runs) > 1 {
				snapshotDir = filepath.Join(snapshotDir, strconv.Itoa(i))
			}
			projectDirSnapshot, err := editSnapshot(projectDir, snapshotEdits)
			if err != nil {
				return nil, fmt.Errorf("program snapshot creation: %w", err)
			}
			validations, err := doSnapshot(eng.DisableSnapshotWriting, projectDirSnapshot, snapshotDir)
			if err != nil {
				return nil, fmt.Errorf("program snapshot validation: %w", err)
			}
			if len(validations) > 0 {
				return makeTestResponse("program snapshot validation failed:\n" + strings.Join(validations, "\n")), nil
			}
			// If we made a snapshot edit we can clean it up now
			if projectDirSnapshot != projectDir {
				err = os.RemoveAll(projectDirSnapshot)
				if err != nil {
					return nil, fmt.Errorf("remove snapshot dir: %w", err)
				}
			}
		}

		project, err := workspace.LoadProject(filepath.Join(projectDir, "Pulumi.yaml"))
		if err != nil {
			return makeTestResponse(fmt.Sprintf("load project: %v", err)), nil
		}

		info := &engine.Projinfo{Proj: project, Root: projectDir}
		pwd, main, err := info.GetPwdMain()
		if err != nil {
			return makeTestResponse(fmt.Sprintf("get pwd main: %v", err)), nil
		}

		programInfo := plugin.NewProgramInfo(
			projectDir, /* rootDirectory */
			pwd,        /* programDirectory */
			main,
			project.Runtime.Options())

		resp := installDependencies(languageClient, programInfo, false /* isPlugin */)
		if resp != nil {
			return resp, nil
		}

		// TODO(https://github.com/pulumi/pulumi/issues/13942): This should only add new things, don't modify

		// Query the language plugin for what it thinks the project dependencies are, we expect to see pulumi and the SDKs.
		// We make a transitive query here because some languages (e.g. Python) treat dependencies as transitive if any of
		// their dependencies has a dependency on the package, even if the program also directly lists it as a dependency as
		// well.
		dependencies, err := languageClient.GetProgramDependencies(programInfo, true)
		if err != nil {
			return makeTestResponse(fmt.Sprintf("get program dependencies: %v", err)), nil
		}
		expectedDependencies := []plugin.DependencyInfo{}
		if token.CoreVersion != "" {
			expectedDependencies = append(expectedDependencies, plugin.DependencyInfo{
				Name: "pulumi", Version: token.CoreVersion,
			})
		}
		for _, pkg := range programPackages {
			if pkg.Name() == "pulumi" {
				// Skip the pulumi package, the version for that is handled above.
				continue
			}

			expectedDependencies = append(expectedDependencies, plugin.DependencyInfo{
				Name:    pkg.Name(),
				Version: pkg.Version().String(),
			})
		}
		for _, expectedDependency := range expectedDependencies {
			// We have to do some fuzzy matching by name here because the language plugin returns the name of the
			// library, which is generally _not_ just the plugin name. e.g. "@pulumi/aws" for the nodejs aws library.

			// When checking for version equality we _want_ to do a semver exact match but not all languages support
			// semver, so we will allow a fuzzy match of the version as well.
			versionsMatch := func(expected, actual string) bool {
				if expected == actual {
					return true
				}
				// Actual might be the empty string, some languages can't always return versions especially for local
				// dependencies. In this case we treat it as matching as this is just supposed to be a best effort check.
				if actual == "" {
					return true
				}

				// Expected _will_ be a semver (because we got it from the provider version), but actual could be
				// _anything_. We assume it will at least have the major.minor.patch part from the expected semver.
				expectedSV := semver.MustParse(expected)
				expectedSV.Pre = nil
				expectedSV.Build = nil
				expected = expectedSV.String()

				return strings.Contains(actual, expected)
			}

			// found is the version we've found for this dependency, if any. We fuzzy match by name and then check version
			// so this is just to give better error messages. For our main dependencies we should have a different version
			// for every package, so the fuzzy check by name then exact check by version should be unique.
			var found *string
			for _, actual := range dependencies {
				actual := actual

				sanatize := func(s string) string {
					return strings.ToLower(
						strings.ReplaceAll(
							strings.ReplaceAll(s, "_", ""),
							"-", ""))
				}

				if strings.Contains(sanatize(actual.Name), sanatize(expectedDependency.Name)) {
					found = &actual.Version
					if versionsMatch(expectedDependency.Version, actual.Version) {
						break
					}
				}
			}

			if found == nil {
				return makeTestResponse("missing expected dependency " + expectedDependency.Name), nil
			} else if !versionsMatch(expectedDependency.Version, *found) {
				return makeTestResponse(fmt.Sprintf("dependency %s has unexpected version %s, expected %s",
					expectedDependency.Name, *found, expectedDependency.Version)), nil
			}
		}

		// Query the language plugin for what it thinks the project packages are, we expect to see the SDKs.
		packages, err := languageClient.GetRequiredPackages(programInfo)
		if err != nil {
			return makeTestResponse(fmt.Sprintf("get required packages: %v", err)), nil
		}
		expectedPackages := []workspace.PackageDescriptor{}
		for _, pkg := range programPackages {
			if pkg.Name() == "pulumi" {
				// Skip the pulumi package, the version for that is handled above.
				continue
			}

			pkgDef, err := pkg.Definition()
			if err != nil {
				return makeTestResponse(fmt.Sprintf("get package definition: %v", err)), nil
			}

			var desc workspace.PackageDescriptor
			if pkgDef.Parameterization == nil {
				desc = workspace.PackageDescriptor{
					PluginSpec: workspace.PluginSpec{
						Name:    pkgDef.Name,
						Version: pkgDef.Version,
					},
				}
			} else {
				desc = workspace.PackageDescriptor{
					PluginSpec: workspace.PluginSpec{
						Name:    pkgDef.Parameterization.BaseProvider.Name,
						Version: &pkgDef.Parameterization.BaseProvider.Version,
					},
					Parameterization: &workspace.Parameterization{
						Name:    pkgDef.Name,
						Version: *pkgDef.Version,
						Value:   pkgDef.Parameterization.Parameter,
					},
				}
			}

			expectedPackages = append(expectedPackages, desc)
		}

		versionsMatch := func(expected, actual *semver.Version) bool {
			if expected == nil && actual == nil {
				return true
			}
			if expected == nil || actual == nil {
				return false
			}
			return expected.EQ(*actual)
		}
		parameterizationsMatch := func(expected, actual *workspace.Parameterization) bool {
			if expected == nil && actual == nil {
				return true
			}
			if expected == nil || actual == nil {
				return false
			}
			return expected.Name == actual.Name &&
				versionsMatch(&expected.Version, &actual.Version) &&
				slices.Equal(expected.Value, actual.Value)
		}
		for _, expectedPackage := range expectedPackages {
			var found bool
			for _, actual := range packages {
				if actual.Name == expectedPackage.Name &&
					versionsMatch(expectedPackage.Version, actual.Version) &&
					parameterizationsMatch(expectedPackage.Parameterization, actual.Parameterization) {
					found = true
					break
				}
			}

			if !found {
				return makeTestResponse(fmt.Sprintf("missing expected package %v", expectedPackage)), nil
			}
		}
		// For packages we need a symmetric check, we shouldn't have any packages that _aren't_ expected.
		for _, actual := range packages {
			var found bool
			for _, expectedPackage := range expectedPackages {
				if actual.Name == expectedPackage.Name &&
					versionsMatch(expectedPackage.Version, actual.Version) &&
					parameterizationsMatch(expectedPackage.Parameterization, actual.Parameterization) {
					found = true
					break
				}
			}

			if !found {
				return makeTestResponse(fmt.Sprintf("unexpected extra package %v", actual)), nil
			}
		}

		testBackend.SetCurrentProject(project)

		// Create a new stack for the test
		stackReference, err := testBackend.ParseStackReference("test")
		if err != nil {
			return nil, fmt.Errorf("parse test stack reference: %w", err)
		}
		var s backend.Stack
		if i == 0 {
			s, err = testBackend.CreateStack(ctx, stackReference, projectDir, nil, nil)
			if err != nil {
				return nil, fmt.Errorf("create test stack: %w", err)
			}
		} else {
			s, err = testBackend.GetStack(ctx, stackReference)
			if err != nil {
				return nil, fmt.Errorf("get test stack: %w", err)
			}
		}

		updateOptions := run.UpdateOptions
		updateOptions.Host = pctx.Host

		// Translate the policy pack option on the test to point to the paths given by the testdata
		if len(run.PolicyPacks) > 0 && token.PolicyPackDirectory == "" {
			return nil, errors.New("policy packs specified but no policy pack directory given")
		}

		for policyPack, policyConfig := range run.PolicyPacks {
			// Write the policy config to a JSON file
			var policyConfigFile string
			if len(policyConfig) != 0 {
				policyConfigFile = filepath.Join(projectDir, policyPack+".json")
				jsonBytes, err := json.Marshal(policyConfig)
				if err != nil {
					return nil, fmt.Errorf("marshal policy config: %w", err)
				}
				err = os.WriteFile(policyConfigFile, jsonBytes, 0o600)
				if err != nil {
					return nil, fmt.Errorf("write policy config: %w", err)
				}
			}

			// Install the dependencies for the policy pack
			policyPath := filepath.Join(token.PolicyPackDirectory, policyPack)
			policyInfo := plugin.NewProgramInfo(policyPath, policyPath, ".", nil)
			resp := installDependencies(languageClient, policyInfo, true /* isPlugin */)
			if resp != nil {
				return resp, nil
			}

			pack := engine.LocalPolicyPack{
				Path:   policyPath,
				Config: policyConfigFile,
			}

			updateOptions.LocalPolicyPacks = append(updateOptions.LocalPolicyPacks, pack)
		}

		// Set up the stack and engine configuration
		opts := backend.UpdateOptions{
			AutoApprove: true,
			SkipPreview: true,
			Display: backendDisplay.Options{
				Color:  colors.Never,
				Stdout: stdout,
				Stderr: stderr,
			},
			Engine: updateOptions,
		}

		cfg := backend.StackConfiguration{
			Config:    run.Config,
			Decrypter: dec,
		}

		updateOperation := backend.UpdateOperation{
			Proj:               project,
			Root:               projectDir,
			Opts:               opts,
			M:                  &backend.UpdateMetadata{},
			StackConfiguration: cfg,
			SecretsManager:     sm,
			SecretsProvider:    b64secrets.Base64SecretsProvider,
			Scopes:             backend.CancellationScopes,
		}

		assertPreview := run.AssertPreview
		if assertPreview == nil {
			// if no assertPreview is provided for the test run, we create a default implementation
			// where we simply assert that the preview changes did not error
			assertPreview = func(
				l *tests.L, proj string, err error, p *deploy.Plan,
				changes display.ResourceChanges, events []engine.Event,
			) {
				assert.NoErrorf(l, err, "expected no error in preview")
			}
		}

		// Perform a preview on the stack
		eventsCts := &promise.CompletionSource[[]engine.Event]{}
		eventSink := make(chan engine.Event, 1)
		go func() {
			events := []engine.Event{}
			for event := range eventSink {
				events = append(events, event)
			}
			eventsCts.Fulfill(events)
		}()

		plan, previewChanges, res := s.Preview(ctx, updateOperation, eventSink)
		close(eventSink)
		events, err := eventsCts.Promise().Result(ctx)
		if err != nil {
			return nil, fmt.Errorf("preview events: %w", err)
		}

		// assert preview results
		previewResult := tests.WithL(func(l *tests.L) {
			assertPreview(l, projectDir, res, plan, previewChanges, events)
		})

		if previewResult.Failed {
			return &testingrpc.RunLanguageTestResponse{
				Success:  !previewResult.Failed,
				Messages: previewResult.Messages,
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
			}, nil
		}

		eventsCts = &promise.CompletionSource[[]engine.Event]{}
		eventSink = make(chan engine.Event, 1)
		go func() {
			events := []engine.Event{}
			for event := range eventSink {
				events = append(events, event)
			}
			eventsCts.Fulfill(events)
		}()
		changes, res := s.Update(ctx, updateOperation, eventSink)
		close(eventSink)
		events, err = eventsCts.Promise().Result(ctx)
		if err != nil {
			return nil, fmt.Errorf("preview events: %w", err)
		}

		var snap *deploy.Snapshot
		if res == nil {
			// Refetch the stack so we can get the snapshot
			s, err = testBackend.GetStack(ctx, stackReference)
			if err != nil {
				return nil, fmt.Errorf("get stack: %w", err)
			}

			snap, err = s.Snapshot(ctx, b64secrets.Base64SecretsProvider)
			if err != nil {
				return nil, fmt.Errorf("snapshot: %w", err)
			}
		} else {
			// We still want to try to get a snapshot, but won't error out
			// if we can't.
			s, err = testBackend.GetStack(ctx, stackReference)
			if err == nil {
				snap, _ = s.Snapshot(ctx, b64secrets.Base64SecretsProvider)
			}
		}

		result = tests.WithL(func(l *tests.L) {
			run.Assert(l, projectDir, res, snap, changes, events)
		})
		if result.Failed {
			return &testingrpc.RunLanguageTestResponse{
				Success:  !result.Failed,
				Messages: result.Messages,
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
			}, nil
		}
	}

	return &testingrpc.RunLanguageTestResponse{
		Success:  !result.Failed,
		Messages: result.Messages,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}, nil
}
