// Copyright 2026, Pulumi Corporation.
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

package conformancetestrunner

import (
	"bytes"
	"context"
	b64 "encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
	"github.com/pulumi/pulumi/pkg/v3/backend"
	backendDisplay "github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	b64secrets "github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/tests"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi-internal/gsync"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	testingrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/testing"
	"github.com/segmentio/encoding/json"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pbempty "google.golang.org/protobuf/types/known/emptypb"
)

// LanguageTestServer is the interface for the language test gRPC server.
type LanguageTestServer interface {
	testingrpc.LanguageTestServer
	pulumirpc.EngineServer

	// Address returns the address at which the test RPC server may be reached.
	Address() string

	// Cancel signals that the test server should be terminated.
	Cancel()

	// Done awaits the test server's termination, and returns any errors that result.
	Done() error

	// SetDisableSnapshotWriting controls whether snapshot writing is disabled.
	// When true, snapshots are validated but not written to disk.
	SetDisableSnapshotWriting(bool)
}

// Start creates and starts a language test server parameterized by the given
// testdata filesystem and language-test map.
//
// testdata is an embed.FS (or any fs.FS) containing the PCL test-data tree,
// typically embedded via //go:embed testdata in the tests package.
//
// languageTests is the map of test-name → LanguageTest describing each
// conformance test to expose.
func Start(ctx context.Context, testdata fs.FS, languageTests map[string]tests.LanguageTest) (LanguageTestServer, error) {
	server := &languageTestServer{
		ctx:            ctx,
		cancel:         make(chan bool),
		providersLock:  gsync.Map[string, *sync.Mutex]{},
		providersCache: make(map[string]bool),
		sdkLocks:       gsync.Map[string, *sync.Mutex]{},
		artifactMap:    gsync.Map[string, string]{},
		cliVersion:     "3.200.0",
		testdata:       testdata,
		languageTests:  languageTests,
	}

	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: server.cancel,
		Init: func(srv *grpc.Server) error {
			testingrpc.RegisterLanguageTestServer(srv, server)
			pulumirpc.RegisterEngineServer(srv, server)
			return nil
		},
	})
	if err != nil {
		return nil, err
	}

	server.addr = fmt.Sprintf("127.0.0.1:%d", handle.Port)
	server.done = handle.Done

	return server, nil
}

// languageTestServer is the server side of the language testing RPC machinery.
type languageTestServer struct {
	testingrpc.UnsafeLanguageTestServer
	pulumirpc.UnimplementedEngineServer

	ctx    context.Context
	cancel chan bool
	done   <-chan error
	addr   string

	sdkLocks gsync.Map[string, *sync.Mutex]

	providersLock  gsync.Map[string, *sync.Mutex]
	providersCache map[string]bool

	// A map storing the paths to the generated package artifacts
	artifactMap gsync.Map[string, string]

	// Used by _bad snapshot_ tests to disable snapshot writing.
	DisableSnapshotWriting bool

	logLock sync.Mutex
	// Used by the Log method to track the number of times a message has been repeated.
	logRepeat int
	// Used by the Log method to track the last message logged, this is so we can elide duplicate messages.
	previousMessage string

	cliVersion string // Used by RequirePulumiVersion to mock the CLI version

	// testdata is the filesystem containing PCL test data.
	testdata fs.FS
	// languageTests is the map of all tests exposed by this server.
	languageTests map[string]tests.LanguageTest
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

func (eng *languageTestServer) SetDisableSnapshotWriting(v bool) {
	eng.DisableSnapshotWriting = v
}

// NewLanguageTestServer creates a languageTestServer pre-populated with the
// default test data and language tests from the tests sub-package.  It does
// NOT start a gRPC listener; call Start for a fully-networked server.
//
// The returned server is suitable for use in unit tests that call
// PrepareLanguageTests / RunLanguageTest / GetLanguageTests directly, without
// going through gRPC.
func NewLanguageTestServer() LanguageTestServer {
	return &languageTestServer{
		providersLock:  gsync.Map[string, *sync.Mutex]{},
		providersCache: make(map[string]bool),
		sdkLocks:       gsync.Map[string, *sync.Mutex]{},
		artifactMap:    gsync.Map[string, string]{},
		cliVersion:     "3.200.0",
		testdata:       tests.LanguageTestdata,
		languageTests:  tests.LanguageTests,
	}
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

func (eng *languageTestServer) RequirePulumiVersion(ctx context.Context, req *pulumirpc.RequirePulumiVersionRequest,
) (*pulumirpc.RequirePulumiVersionResponse, error) {
	if err := plugin.ValidatePulumiVersionRange(req.PulumiVersionRange, eng.cliVersion); err != nil {
		return nil, err
	}
	return &pulumirpc.RequirePulumiVersionResponse{}, nil
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
	workspaceDescriptor := workspace.PluginDescriptor{
		Kind:              apitype.ResourcePlugin,
		Name:              descriptor.Name,
		Version:           descriptor.Version,
		PluginDownloadURL: descriptor.DownloadURL,
	}

	provider, err := l.host.Provider(workspaceDescriptor, env.Global())
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
	filtered := make([]string, 0, len(eng.languageTests))
	for testName := range eng.languageTests {
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

type CompiledReplacement struct {
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
	LanguagePluginName    string
	LanguagePluginTarget  string
	TemporaryDirectory    string
	SnapshotDirectory     string
	CoreArtifact          string
	CoreVersion           string
	SnapshotEdits         []replacement
	LanguageInfo          string
	ProgramOverrides      map[string]programOverride
	PolicyPackDirectory   string
	Local                 bool
	ProvidersDirectory    string
	ConverterPluginTarget string

	// testdata is the filesystem containing PCL test data.
	// It is NOT serialized into the base64 token; it is injected at runtime.
	testdata fs.FS `json:"-"`
}

func installDependencies(
	languageClient plugin.LanguageRuntime,
	programInfo plugin.ProgramInfo,
	isPlugin bool,
) *testingrpc.RunLanguageTestResponse {
	installStdout, installStderr, installDone, err := languageClient.InstallDependencies(
		plugin.InstallDependenciesRequest{Info: programInfo, IsPlugin: isPlugin},
	)
	programOrPlugin := "program"
	if isPlugin {
		programOrPlugin = "plugin"
	}
	if err != nil {
		return makeTestResponse(fmt.Sprintf("install %s dependencies: %v", programOrPlugin, err))
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
			Messages: []string{fmt.Sprintf("install %s dependencies: %v", programOrPlugin, err)},
			Stdout:   string(installStdoutBytes),
			Stderr:   string(installStderrBytes),
		}
	}

	return nil
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
	pctx, err := plugin.NewContextWithRoot(ctx, snk, snk, nil, "", "", nil, false, nil, nil, nil, nil,
		nil, schema.NewLoaderServerFromHost)
	if err != nil {
		return nil, fmt.Errorf("setup plugin context: %w", err)
	}
	defer contract.IgnoreClose(pctx)

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
		test, has := eng.languageTests[testName]
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

	var providersDirectory string
	// Same as the policy pack directory, absolute the providers directory if set.
	if req.ProvidersDirectory != "" {
		providersDirectory, err = filepath.Abs(req.ProvidersDirectory)
		if err != nil {
			return nil, fmt.Errorf("get absolute path for providers directory %s: %w", req.ProvidersDirectory, err)
		}
	}

	tokenBytes, err := json.Marshal(&testToken{
		LanguagePluginName:    req.LanguagePluginName,
		LanguagePluginTarget:  req.LanguagePluginTarget,
		TemporaryDirectory:    req.TemporaryDirectory,
		SnapshotDirectory:     req.SnapshotDirectory,
		CoreArtifact:          coreArtifact,
		CoreVersion:           req.CoreSdkVersion,
		SnapshotEdits:         edits,
		LanguageInfo:          req.LanguageInfo,
		ProgramOverrides:      programOverrides,
		PolicyPackDirectory:   policyPackDirectory,
		Local:                 req.Local,
		ProvidersDirectory:    providersDirectory,
		ConverterPluginTarget: req.ConverterPluginTarget,
	})
	contract.AssertNoErrorf(err, "could not marshal test token")

	b64token := b64.StdEncoding.EncodeToString(tokenBytes)

	return &testingrpc.PrepareLanguageTestsResponse{
		Token: b64token,
	}, nil
}

func GetProviderVersion(provider plugin.Provider) (semver.Version, error) {
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

func (eng *languageTestServer) RunLanguageTest(
	ctx context.Context, req *testingrpc.RunLanguageTestRequest,
) (*testingrpc.RunLanguageTestResponse, error) {
	test, has := eng.languageTests[req.Test]
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
	token.testdata = eng.testdata

	// If the language defines any snapshot edits compile those regexs to apply now
	snapshotEdits := []CompiledReplacement{}
	for _, replace := range token.SnapshotEdits {
		pathRegex, err := regexp.Compile(replace.Path)
		if err != nil {
			return nil, fmt.Errorf("invalid path regex %s: %w", replace.Path, err)
		}
		editRegex, err := regexp.Compile(replace.Pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid edit regex %s: %w", replace.Pattern, err)
		}
		snapshotEdits = append(snapshotEdits, CompiledReplacement{
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
		ctx, snk, snk, nil, token.TemporaryDirectory, token.TemporaryDirectory, nil, false, nil, nil, nil, nil,
		nil, schema.NewLoaderServerFromHost)
	if err != nil {
		return nil, fmt.Errorf("setup plugin context: %w", err)
	}
	defer contract.IgnoreClose(pctx)

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
	host := &testHost{
		engine:                      eng,
		ctx:                         pctx,
		host:                        pctx.Host,
		runtime:                     languageClient,
		runtimeName:                 token.LanguagePluginName,
		providers:                   make(map[string]func() (plugin.Provider, error)),
		connections:                 make(map[plugin.Provider]io.Closer),
		skipEnsurePluginsValidation: test.SkipEnsurePluginsValidation,
	}

	pctx.Host = host

	// Setup a schema loader for any package lookups, installs and code generation.
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

	host.loaderAddress = grpcServer.Addr()

	// And fill that host with our test providers
	for _, provider := range test.Providers {
		p := provider()
		version, err := GetProviderVersion(p)
		if err != nil {
			return nil, err
		}
		key := fmt.Sprintf("%s@%s", p.Pkg(), version)

		// If this is a provider that should be overridden using the languages plugin directory try and do that now.
		pkg := p.Pkg().String()
		if slices.Contains(test.LanguageProviders, pkg) {
			cacheKey := fmt.Sprintf("%s@%s", key, token.TemporaryDirectory)
			lock, _ := eng.providersLock.LoadOrStore(cacheKey, &sync.Mutex{})
			lock.Lock()
			defer lock.Unlock()
			targetDirectory := filepath.Join(token.TemporaryDirectory, "providers", pkg)
			if eng.providersCache[cacheKey] {
				host.providers[key] = func() (plugin.Provider, error) {
					pluginProvider, err := plugin.NewProviderFromPath(host, pctx, p.Pkg(), targetDirectory)
					if err != nil {
						return nil, fmt.Errorf("load provider %s from %s: %w", pkg, targetDirectory, err)
					}
					return pluginProvider, nil
				}
			} else {
				if token.ProvidersDirectory == "" {
					return nil, fmt.Errorf("provider %s should be loaded from language providers directory, "+
						"but no providers directory was specified", pkg)
				}

				sourceDirectory := filepath.Join(token.ProvidersDirectory, pkg)

				// Copy to a new targetDirectory so we don't mutate the original test data
				err = copyDirectory(os.DirFS(sourceDirectory), ".", targetDirectory, nil, nil)
				if err != nil {
					return nil, fmt.Errorf("copy provider test data: %w", err)
				}

				// Link the provider program to the core SDK and install its dependencies
				providerInfo := plugin.NewProgramInfo(targetDirectory, targetDirectory, ".", nil)

				linkDeps := []workspace.LinkablePackageDescriptor{{
					Path: token.CoreArtifact,
					Descriptor: workspace.PackageDescriptor{
						PluginDescriptor: workspace.PluginDescriptor{
							Name: "pulumi",
						},
					},
				}}
				_, err = languageClient.Link(providerInfo, linkDeps, grpcServer.Addr())
				if err != nil {
					return makeTestResponse(fmt.Sprintf("link program: %v", err)), nil
				}

				resp := installDependencies(languageClient, providerInfo, true /* isPlugin */)
				if resp != nil {
					return resp, nil
				}

				host.providers[key] = func() (plugin.Provider, error) {
					pluginProvider, err := plugin.NewProviderFromPath(host, pctx, p.Pkg(), targetDirectory)
					if err != nil {
						return nil, fmt.Errorf("load provider %s from %s: %w", pkg, targetDirectory, err)
					}
					return pluginProvider, nil
				}
				eng.providersCache[cacheKey] = true
			}
		} else {
			host.providers[key] = func() (plugin.Provider, error) {
				return provider(), nil
			}
		}
	}

	// Generate SDKs for all the packages we need
	artifactsDir := filepath.Join(token.TemporaryDirectory, "artifacts")

	// For each test run collect the packages reported by PCL
	packages := []*schema.Package{}
	for i, run := range test.Runs {
		if i > 0 && test.RunsShareSource {
			break
		}

		// Create a source directory for the test
		sourceDir := filepath.Join(token.TemporaryDirectory, "source", req.Test)
		if len(test.Runs) > 1 && !test.RunsShareSource {
			sourceDir = filepath.Join(sourceDir, strconv.Itoa(i))
		}
		err = os.MkdirAll(sourceDir, 0o700)
		if err != nil {
			return nil, fmt.Errorf("create source dir: %w", err)
		}

		// Find and copy the tests PCL code to the source dir
		pclDir := filepath.Join("testdata", req.Test)
		if len(test.Runs) > 1 && !test.RunsShareSource {
			pclDir = filepath.Join(pclDir, strconv.Itoa(i))
		}
		err = copyDirectory(eng.testdata, pclDir, sourceDir, nil, nil)
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
	sdks := map[string]string{}
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

	// We only generate sdks if running in non-local mode
	if !token.Local {
		for _, pkg := range packages {
			sdkName := fmt.Sprintf("%s-%s", pkg.Name, pkg.Version)
			sdkTempDir := filepath.Join(token.TemporaryDirectory, "sdks", sdkName)
			sdks[sdkName] = sdkTempDir
			response, err := func() (*testingrpc.RunLanguageTestResponse, error) {
				lock, _ := eng.sdkLocks.LoadOrStore(sdkTempDir, &sync.Mutex{})
				lock.Lock()
				defer lock.Unlock()

				sdkArtifact, ok := eng.artifactMap.Load(sdkTempDir)
				if ok {
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
				if diags.HasErrors() {
					return makeTestResponse(fmt.Sprintf("generate package %s: %v", pkg.Name, diags)), nil
				}

				snapshotDir := filepath.Join(token.SnapshotDirectory, "sdks", sdkName)
				sdkSnapshotDir, err := editSnapshot(sdkTempDir, snapshotEdits)
				if err != nil {
					return nil, fmt.Errorf("sdk snapshot creation for %s: %w", pkg.Name, err)
				}
				validations, err := doSnapshot(eng.DisableSnapshotWriting, sdkSnapshotDir, snapshotDir)
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

				sdkArtifact, err = languageClient.Pack(sdkTempDir, artifactsDir)
				if err != nil {
					return nil, fmt.Errorf("sdk packing for %s: %w", pkg.Name, err)
				}
				localDependencies[pkg.Name] = sdkArtifact
				eng.artifactMap.Store(sdkTempDir, sdkArtifact)

				sdkSnapshotDir, err = editSnapshot(sdkTempDir, snapshotEdits)
				if err != nil {
					return nil, fmt.Errorf("sdk snapshot creation for %s: %w", pkg.Name, err)
				}
				validations, err = compareDirectories(sdkSnapshotDir, snapshotDir, true /* allowNewFiles */)
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
	}

	// Just use base64 "secrets" for these tests
	sm := b64secrets.NewBase64SecretsManager()

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
	if err := createStackReferences(ctx, sm, testBackend, test.StackReferences); err != nil {
		return nil, err
	}

	languageTestResult, err := runLanguageTests(ctx, token, req.Test, test, loader, packages,
		sdks, localDependencies, languageClient, grpcServer,
		eng.DisableSnapshotWriting, snapshotEdits, testBackend, stdout, stderr, pctx, "projects")
	if err != nil || !languageTestResult.Success || token.ConverterPluginTarget == "" || req.SkipConvertTests {
		return languageTestResult, err
	}

	// Now that normal language tests have run successfully, we want to run the converter test

	converterConn, err := grpc.NewClient(token.ConverterPluginTarget,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial converter plugin: %w", err)
	}

	ejectToken := token
	ejectToken.TemporaryDirectory = filepath.Join(token.TemporaryDirectory, "eject")
	if err := os.MkdirAll(ejectToken.TemporaryDirectory, 0o755); err != nil {
		return nil, fmt.Errorf("create eject temporary dir: %w", err)
	}

	ejectTestingClient := roundTripClient{
		LanguageRuntime:        languageClient,
		converter:              pulumirpc.NewConverterClient(converterConn),
		disableSnapshotWriting: eng.DisableSnapshotWriting,
		snapshotEdits:          snapshotEdits,
		projectsBaseDir:        filepath.Join(ejectToken.TemporaryDirectory, "round-tripped-project"),
		ejectSnapshotBaseDir:   filepath.Join(token.SnapshotDirectory, "eject-pcl"),
	}

	ejectBackendDir := filepath.Join(token.TemporaryDirectory, "backends", req.Test+"-eject")
	if err := os.MkdirAll(ejectBackendDir, 0o755); err != nil {
		return nil, fmt.Errorf("create eject backend dir: %w", err)
	}
	ejectTestBackend, err := diy.New(ctx, snk, "file://"+ejectBackendDir, nil)
	if err != nil {
		return nil, fmt.Errorf("create eject diy backend: %w", err)
	}
	if err := createStackReferences(ctx, sm, ejectTestBackend, test.StackReferences); err != nil {
		return nil, err
	}

	return runLanguageTests(ctx, ejectToken, req.Test, test, loader, packages,
		sdks, localDependencies, ejectTestingClient, grpcServer,
		eng.DisableSnapshotWriting, snapshotEdits, ejectTestBackend, stdout, stderr, pctx, "round-tripped-project")
}

func createStackReferences(
	ctx context.Context, sm secrets.Manager, b diy.Backend, stackRefs map[string]resource.PropertyMap,
) error {
	for name, outputs := range stackRefs {
		ref, err := b.ParseStackReference(name)
		if err != nil {
			return fmt.Errorf("parse test stack reference: %w", err)
		}

		s, err := b.CreateStack(ctx, ref, "", nil, nil)
		if err != nil {
			return fmt.Errorf("create test stack reference: %w", err)
		}

		stackName := ref.Name()
		projectName, has := ref.Project()
		if !has {
			return fmt.Errorf("stack reference %s has no project", ref)
		}
		resourceName := fmt.Sprintf("%s-%s", projectName, stackName)

		snap := &deploy.Snapshot{
			SecretsManager: sm,
			Resources: []*resource.State{
				{
					Type: resource.RootStackType,
					URN: resource.CreateURN(resourceName, string(resource.RootStackType), "",
						string(projectName), stackName.String()),
					Outputs: outputs,
				},
			},
		}

		untypedDeployment, err := stack.SerializeUntypedDeployment(ctx, snap, nil /*opts*/)
		if err != nil {
			return fmt.Errorf("serialize deployment: %w", err)
		}

		if err := backend.ImportStackDeployment(ctx, s, untypedDeployment); err != nil {
			return fmt.Errorf("import deployment: %w", err)
		}
	}
	return nil
}

func runLanguageTests(
	ctx context.Context, token testToken, testName string, test tests.LanguageTest,
	loader schema.ReferenceLoader, packages []*schema.Package, sdks, localDependencies map[string]string,
	languageClient plugin.LanguageRuntime, grpcServer *plugin.GrpcServer,
	disableSnapshotWriting bool, snapshotEdits []CompiledReplacement,
	testBackend diy.Backend,
	stdout, stderr *bytes.Buffer,
	pctx *plugin.Context,
	projectDir string,
) (*testingrpc.RunLanguageTestResponse, error) {
	sm := b64secrets.NewBase64SecretsManager()
	dec := sm.Decrypter()

	var result tests.LResult
	for i, run := range test.Runs {
		sourceDir := filepath.Join(token.TemporaryDirectory, "source", testName)
		projectSubDir := projectDir
		projectDir := filepath.Join(token.TemporaryDirectory, projectDir, testName)
		if i == 0 || !test.RunsShareSource {
			// Create a source directory for the test
			if len(test.Runs) > 1 && !test.RunsShareSource {
				sourceDir = filepath.Join(sourceDir, strconv.Itoa(i))
			}

			if err := os.MkdirAll(sourceDir, 0o700); err != nil {
				return nil, fmt.Errorf("create source dir: %w", err)
			}

			// Find and copy the tests PCL code to the source dir
			pclDir := filepath.Join("testdata", testName)
			if len(test.Runs) > 1 && !test.RunsShareSource {
				pclDir = filepath.Join(pclDir, strconv.Itoa(i))
			}

			if err := copyDirectory(token.testdata, pclDir, sourceDir, nil, nil); err != nil {
				return nil, fmt.Errorf("copy source test data: %w", err)
			}

			// Create a directory for the project
			if len(test.Runs) > 1 && !test.RunsShareSource {
				projectDir = filepath.Join(projectDir, strconv.Itoa(i))
			}

			if err := os.MkdirAll(projectDir, 0o755); err != nil {
				return nil, fmt.Errorf("create project dir: %w", err)
			}
		}

		// Generate the project and read in the Pulumi.yaml
		rootDirectory := sourceDir
		projectJSON := func() string {
			if run.Main == "" {
				return fmt.Sprintf(`{"name": "%s"}`, testName)
			}
			sourceDir = filepath.Join(sourceDir, run.Main)
			return fmt.Sprintf(`{"name": "%s", "main": "%s"}`, testName, run.Main)
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

		if i == 0 || !test.RunsShareSource {
			var diagnostics hcl.Diagnostics

			// If we're running in local mode we need to generate the packages first before calling GenerateProject
			if token.Local {
				for _, pkg := range packages {
					sdkName := fmt.Sprintf("%s-%s", pkg.Name, pkg.Version)
					sdkTargetDir := filepath.Join(projectDir, "sdks", sdkName)
					sdks[sdkName] = sdkTargetDir

					schemaBytes, err := pkg.MarshalJSON()
					if err != nil {
						return nil, fmt.Errorf("marshal schema for provider %s: %w", pkg.Name, err)
					}

					diags, err := languageClient.GeneratePackage(
						sdkTargetDir, string(schemaBytes), nil, grpcServer.Addr(), localDependencies, false)
					if err != nil {
						return makeTestResponse(fmt.Sprintf("generate package %s: %v", pkg.Name, err)), nil
					}
					if diags.HasErrors() {
						return makeTestResponse(fmt.Sprintf("generate package %s: %v", pkg.Name, diags)), nil
					}
					localDependencies[pkg.Name] = sdkTargetDir
				}
			}

			if programOverride, ok := token.ProgramOverrides[testName]; ok {
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

				err = copyDirectory(os.DirFS(rootDirectory), ".", projectDir, nil, []string{".pp"})
				if err != nil {
					return nil, fmt.Errorf("copy testdata: %w", err)
				}

				snapshotDir := filepath.Join(token.SnapshotDirectory, projectSubDir, testName)
				if len(test.Runs) > 1 && !test.RunsShareSource {
					snapshotDir = filepath.Join(snapshotDir, strconv.Itoa(i))
				}
				projectDirSnapshot, err := editSnapshot(projectDir, snapshotEdits)
				if err != nil {
					return nil, fmt.Errorf("program snapshot creation: %w", err)
				}
				validations, err := doSnapshot(disableSnapshotWriting, projectDirSnapshot, snapshotDir)
				if err != nil {
					return nil, fmt.Errorf("program snapshot validation: %w", err)
				}
				if len(validations) > 0 {
					return makeTestResponse("program snapshot validation failed:\n" + strings.Join(validations, "\n")), nil
				}
				if projectDirSnapshot != projectDir {
					err = os.RemoveAll(projectDirSnapshot)
					if err != nil {
						return nil, fmt.Errorf("remove snapshot dir: %w", err)
					}
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
				continue
			}

			expectedDependencies = append(expectedDependencies, plugin.DependencyInfo{
				Name:    pkg.Name(),
				Version: pkg.Version().String(),
			})
		}
		for _, expectedDependency := range expectedDependencies {
			versionsMatch := func(expected, actual string) bool {
				if expected == actual {
					return true
				}
				if actual == "" {
					return true
				}
				expectedSV := semver.MustParse(expected)
				expectedSV.Pre = nil
				expectedSV.Build = nil
				expected = expectedSV.String()

				return strings.Contains(actual, expected)
			}

			var found *string
			for _, actual := range dependencies {
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

		packages, err := languageClient.GetRequiredPackages(programInfo)
		if err != nil {
			return makeTestResponse(fmt.Sprintf("get required packages: %v", err)), nil
		}
		expectedPackages := []workspace.PackageDescriptor{}
		for _, pkg := range programPackages {
			if pkg.Name() == "pulumi" {
				continue
			}

			pkgDef, err := pkg.Definition()
			if err != nil {
				return makeTestResponse(fmt.Sprintf("get package definition: %v", err)), nil
			}

			var desc workspace.PackageDescriptor
			if pkgDef.Parameterization == nil {
				desc = workspace.PackageDescriptor{
					PluginDescriptor: workspace.PluginDescriptor{
						Name:    pkgDef.Name,
						Version: pkgDef.Version,
					},
				}
			} else {
				desc = workspace.PackageDescriptor{
					PluginDescriptor: workspace.PluginDescriptor{
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
		tags := run.StackTags
		if tags == nil {
			tags = map[string]string{}
		}
		err = testBackend.UpdateStackTags(ctx, s, tags)
		if err != nil {
			return nil, fmt.Errorf("update stack tags: %w", err)
		}

		updateOptions := run.UpdateOptions
		updateOptions.Host = pctx.Host

		if len(run.PolicyPacks) > 0 && token.PolicyPackDirectory == "" {
			return nil, errors.New("policy packs specified but no policy pack directory given")
		}

		for policyPack, policyConfig := range run.PolicyPacks {
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

			policyPackDir := filepath.Join(token.TemporaryDirectory, "policy_packs", policyPack)
			err = os.MkdirAll(policyPackDir, 0o755)
			if err != nil {
				return nil, fmt.Errorf("create policy pack dir: %w", err)
			}
			err = copyDirectory(os.DirFS(token.PolicyPackDirectory), policyPack, policyPackDir, nil, nil)
			if err != nil {
				return nil, fmt.Errorf("copy policy pack: %w", err)
			}

			policyInfo := plugin.NewProgramInfo(policyPackDir, policyPackDir, ".", nil)

			linkDeps := []workspace.LinkablePackageDescriptor{{
				Path: token.CoreArtifact,
				Descriptor: workspace.PackageDescriptor{
					PluginDescriptor: workspace.PluginDescriptor{
						Name: "pulumi",
					},
				},
			}}
			_, err = languageClient.Link(policyInfo, linkDeps, grpcServer.Addr())
			if err != nil {
				return makeTestResponse(fmt.Sprintf("link program: %v", err)), nil
			}

			resp := installDependencies(languageClient, policyInfo, true /* isPlugin */)
			if resp != nil {
				return resp, nil
			}

			pack := engine.LocalPolicyPack{
				Path:   policyPackDir,
				Config: policyConfigFile,
			}

			updateOptions.LocalPolicyPacks = append(updateOptions.LocalPolicyPacks, pack)
		}

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
			assertPreview = func(
				l *tests.L, args tests.AssertPreviewArgs,
			) {
				require.NoErrorf(l, err, "expected no error in preview")
			}
		}

		eventsCts := &promise.CompletionSource[[]engine.Event]{}
		eventSink := make(chan engine.Event, 1)
		go func() {
			events := []engine.Event{}
			for event := range eventSink {
				events = append(events, event)
			}
			eventsCts.Fulfill(events)
		}()

		plan, previewChanges, res := backend.PreviewStack(ctx, s, updateOperation, eventSink)
		close(eventSink)
		events, err := eventsCts.Promise().Result(ctx)
		if err != nil {
			return nil, fmt.Errorf("preview events: %w", err)
		}

		previewResult := tests.WithL(func(l *tests.L) {
			assertPreview(l, tests.AssertPreviewArgs{
				ProjectDirectory: projectDir,
				Err:              res,
				Plan:             plan,
				Changes:          previewChanges,
				Events:           events,
				SDKs:             sdks,
			})
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
		changes, res := backend.UpdateStack(ctx, s, updateOperation, eventSink)
		close(eventSink)
		events, err = eventsCts.Promise().Result(ctx)
		if err != nil {
			return nil, fmt.Errorf("preview events: %w", err)
		}

		var snap *deploy.Snapshot
		if res == nil {
			s, err = testBackend.GetStack(ctx, stackReference)
			if err != nil {
				return nil, fmt.Errorf("get stack: %w", err)
			}

			snap, err = s.Snapshot(ctx, b64secrets.Base64SecretsProvider)
			if err != nil {
				return nil, fmt.Errorf("snapshot: %w", err)
			}
		} else {
			s, err = testBackend.GetStack(ctx, stackReference)
			if err == nil {
				snap, _ = s.Snapshot(ctx, b64secrets.Base64SecretsProvider)
			}
		}

		result = tests.WithL(func(l *tests.L) {
			run.Assert(l, tests.AssertArgs{
				ProjectDirectory: projectDir,
				Err:              res,
				Snap:             snap,
				Changes:          changes,
				Events:           events,
				SDKs:             sdks,
			})
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

type roundTripClient struct {
	plugin.LanguageRuntime
	converter              pulumirpc.ConverterClient
	disableSnapshotWriting bool
	snapshotEdits          []CompiledReplacement
	projectsBaseDir        string
	ejectSnapshotBaseDir   string
}

func (rtc roundTripClient) GenerateProject(
	sourceDirectory, targetDirectory, project string,
	strict bool, loaderTarget string, localDependencies map[string]string,
) (hcl.Diagnostics, error) {
	pclDir, diags, err := rtc.roundTrip(context.TODO(), sourceDirectory, project, loaderTarget, strict, localDependencies)
	if err != nil || diags.HasErrors() {
		return diags, err
	}
	defer func() { contract.IgnoreError(os.RemoveAll(pclDir)) }()

	if rtc.ejectSnapshotBaseDir != "" && rtc.projectsBaseDir != "" {
		rel, relErr := filepath.Rel(rtc.projectsBaseDir, targetDirectory)
		if relErr == nil {
			ejectSnapshotDir := filepath.Join(rtc.ejectSnapshotBaseDir, rel)
			pclDirSnapshot, snapErr := editSnapshot(pclDir, rtc.snapshotEdits)
			if snapErr != nil {
				return diags, fmt.Errorf("eject PCL snapshot creation: %w", snapErr)
			}
			validations, snapErr := doSnapshot(rtc.disableSnapshotWriting, pclDirSnapshot, ejectSnapshotDir)
			if pclDirSnapshot != pclDir {
				contract.IgnoreError(os.RemoveAll(pclDirSnapshot))
			}
			if snapErr != nil {
				return diags, fmt.Errorf("eject PCL snapshot validation: %w", snapErr)
			}
			if len(validations) > 0 {
				return diags, fmt.Errorf("eject PCL snapshot validation failed:\n%s",
					strings.Join(validations, "\n"))
			}
		}
	}

	diags2, err := rtc.LanguageRuntime.GenerateProject(pclDir, targetDirectory, project,
		strict, loaderTarget, localDependencies)
	return diags.Extend(diags2), err
}

func (rtc roundTripClient) GenerateProgram(
	program map[string]string, loaderTarget string, strict bool,
) (map[string][]byte, hcl.Diagnostics, error) {
	sourceDir, err := os.MkdirTemp("", "lang-to-pcl-source-*")
	if err != nil {
		return nil, nil, err
	}
	defer func() { contract.IgnoreError(os.RemoveAll(sourceDir)) }()

	for p, f := range program {
		p = filepath.Join(sourceDir, p)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			return nil, nil, err
		}
		if err := os.WriteFile(p, []byte(f), 0o600); err != nil {
			return nil, nil, err
		}
	}

	project := "{\"name\": \"roundtrip\"}"
	pclDir, diags, err := rtc.roundTrip(context.TODO(), sourceDir, project, loaderTarget, strict, nil)
	if err != nil || diags.HasErrors() {
		return nil, diags, err
	}

	pclFiles := map[string]string{}
	walk := func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		pclFiles[strings.TrimPrefix(path, pclDir)] = string(content)
		return nil
	}
	if err := filepath.WalkDir(pclDir, walk); err != nil {
		return nil, diags, err
	}
	lang, diags2, err := rtc.LanguageRuntime.GenerateProgram(pclFiles, loaderTarget, strict)
	return lang, diags.Extend(diags2), err
}

func (rtc roundTripClient) roundTrip(
	ctx context.Context,
	sourceDirectory, project, loaderTarget string, strict bool,
	localDependencies map[string]string,
) (string, hcl.Diagnostics, error) {
	tmpDir, err := os.MkdirTemp("", "pcl-to-lang-1-*")
	if err != nil {
		return "", nil, err
	}
	defer func() { contract.IgnoreError(os.RemoveAll(tmpDir)) }()

	diags, err := rtc.LanguageRuntime.GenerateProject(
		sourceDirectory, tmpDir, project, strict, loaderTarget, localDependencies)
	if err != nil || diags.HasErrors() {
		return "", diags, err
	}

	ejectDir, err := os.MkdirTemp("", "lang-to-pcl-1-*")
	if err != nil {
		return "", diags, err
	}

	resp, err := rtc.converter.ConvertProgram(ctx, &pulumirpc.ConvertProgramRequest{
		SourceDirectory: tmpDir,
		TargetDirectory: ejectDir,
		LoaderTarget:    loaderTarget,
	})
	for _, diag := range resp.GetDiagnostics() {
		diags = append(diags, plugin.RPCDiagnosticToHclDiagnostic(diag))
	}
	if err != nil || diags.HasErrors() {
		contract.IgnoreError(os.RemoveAll(tmpDir))
		return "", diags, err
	}

	proj, err := workspace.LoadProject(filepath.Join(ejectDir, "Pulumi.yaml"))
	if err != nil {
		return "", diags, fmt.Errorf("load ejected project: %w", err)
	}
	if proj.Main != "" {
		ejectDir = filepath.Join(ejectDir, proj.Main)
	}

	return ejectDir, diags, nil
}
