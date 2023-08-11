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

package pulumi

import (
	"bytes"
	"context"
	"embed"
	b64 "encoding/base64"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	backendDisplay "github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/filestate"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	b64secrets "github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	enginerpc "github.com/pulumi/pulumi/sdk/v3/proto/go/engine"
	"github.com/segmentio/encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type EngineServer interface {
	enginerpc.EngineServer

	// Address returns the address at which the engine's RPC server may be reached.
	Address() string

	// Cancel signals that the engine should be terminated, awaits its termination, and returns any errors that result.
	Cancel() error

	// Done awaits the engines termination, and returns any errors that result.
	Done() error
}

func Start(ctx context.Context) (EngineServer, error) {
	// New up an engine RPC server.
	engine := &engineServer{
		ctx:    ctx,
		cancel: make(chan bool),
	}

	// Fire up a gRPC server and start listening for incomings.
	port, done, err := rpcutil.Serve(0, engine.cancel, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			enginerpc.RegisterEngineServer(srv, engine)
			return nil
		},
	}, nil)
	if err != nil {
		return nil, err
	}

	engine.addr = fmt.Sprintf("127.0.0.1:%d", port)
	engine.done = done

	return engine, nil
}

// engineServer is the server side of the engine RPC machinery.
type engineServer struct {
	enginerpc.UnsafeEngineServer

	ctx    context.Context
	cancel chan bool
	done   chan error
	addr   string
}

func (eng *engineServer) Address() string {
	return eng.addr
}

func (eng *engineServer) Cancel() error {
	eng.cancel <- true
	return <-eng.done
}

func (eng *engineServer) Done() error {
	return <-eng.done
}

type languageTest struct {
	config config.Map
	// TODO: This should be a function so we don't have to load all providers in memory all the time.
	providers []plugin.Provider
	// TODO: This should just return "string", if == "" then ok, else fail
	assert func(*L, result.Result, *deploy.Snapshot, display.ResourceChanges)
}

// L holds the state for the current language test.
//
// It provides an interface similar to testing.T,
// allowing its use with testing libraries like Testify.
type L struct {
	mu     sync.RWMutex // guards the fields below
	logs   []string
	failed bool
}

// FailNow marks this test as having failed and halts execution.
func (l *L) FailNow() {
	l.Fail()
	runtime.Goexit()
}

// Fail marks this test as failed but keeps executing.
func (l *L) Fail() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.failed = true
}

// Failed returns whether this test has failed.
func (l *L) Failed() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.failed
}

// Errorf records the given error message and marks this test as failed.
func (l *L) Errorf(format string, args ...interface{}) {
	l.log(1, fmt.Sprintf(format, args...))
	l.Fail()
}

// Logf records the given message in the L's logs.
func (l *L) Logf(format string, args ...interface{}) {
	l.log(1, fmt.Sprintf(format, args...))
}

// log records the given message in the L's logs.
//
// Skip specifies the number of stack frames to skip
// when recording the caller's location.
// 0 refers to the immediate caller of log.
//
// Typically, when used from an exported method on L,
// most callers will want to pass skip=1 to skip themselves
// and record the location of their caller.
func (l *L) log(skip int, msg string) {
	// TODO: When Helper() is added, we'll want to skip those too.
	// We can use runtime.Callers to iterate over the stack
	// and exclude uintptrs for helper functions to do that.
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		file = "???"
		line = 1
	}

	msg = fmt.Sprintf("%s:%d: %s", filepath.Base(file), line, msg)
	l.mu.Lock()
	l.logs = append(l.logs, msg)
	l.mu.Unlock()
}

// WithL runs the given function with a new L,
// blocking until the function returns.
//
// It returns the information recorded by the L.
func WithL(f func(*L)) LResult {
	// To be able to implement FailNow in the L,
	// we need to run it in a separate goroutine
	// so that we can call runtime.Goexit.
	done := make(chan struct{})
	var l L
	go func() {
		defer func() {
			if r := recover(); r != nil {
				l.failed = true
				l.logs = append(l.logs,
					fmt.Sprintf("panic: %v\n\n%s", r, debug.Stack()))
			}

			close(done)
		}()

		f(&l)
	}()
	<-done

	return LResult{
		Failed:   l.failed,
		Messages: l.logs,
	}
}

// LResult is the result of running a language test.
type LResult struct {
	// Failed is true if the test failed.
	Failed bool

	// Messages contains the messages logged by the test.
	//
	// This doesn't necessarily mean that the test failed.
	// For example, a test may log debugging information
	// that is only useful when the test fails.
	Messages []string
}

// TestingT is a subset of the testing.T interface.
// [L] implements this interface.
type TestingT interface {
	// TODO: Helper()

	FailNow()
	Fail()
	Failed() bool
	Errorf(string, ...interface{})
	Logf(string, ...interface{})
}

var (
	_ TestingT         = (*L)(nil)
	_ require.TestingT = (TestingT)(nil) // ensure testify compatibility
)

func assertStackResource(t TestingT, res result.Result, changes display.ResourceChanges) (ok bool) {
	// TODO: t.Helper()

	ok = true
	ok = ok && assert.Nil(t, res, "expected no error, got %v", res)
	ok = ok && assert.NotEmpty(t, changes, "expected at least 1 StepOp")
	ok = ok && assert.NotZero(t, changes[deploy.OpCreate], "expected at least 1 Create")
	return ok
}

func requireStackResource(t TestingT, res result.Result, changes display.ResourceChanges) {
	// TODO: t.Helper()

	if !assertStackResource(t, res, changes) {
		t.FailNow()
	}
}

// assertPropertyMapMember asserts that the given property map has a member with the given key and value.
func assertPropertyMapMember(
	t TestingT,
	props resource.PropertyMap,
	key string,
	want resource.PropertyValue,
) (ok bool) {
	// TODO: t.Helper()

	got, ok := props[resource.PropertyKey(key)]
	if !assert.True(t, ok, "expected property %q", key) {
		return false
	}

	return assert.Equal(t, want, got, "expected property %q to be %v", key, want)
}

//go:embed testdata
var languageTestdata embed.FS

var languageTests = map[string]languageTest{
	"l1-empty": {
		assert: func(l *L, res result.Result, snap *deploy.Snapshot, changes display.ResourceChanges) {
			assertStackResource(l, res, changes)
		},
	},
	"l1-output-bool": {
		assert: func(l *L, res result.Result, snap *deploy.Snapshot, changes display.ResourceChanges) {
			requireStackResource(l, res, changes)

			// Check we have two outputs in the stack for true and false
			require.NotEmpty(l, snap.Resources, "expected at least 1 resource")
			stack := snap.Resources[0]
			require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")

			outputs := stack.Outputs

			assertPropertyMapMember(l, outputs, "output_true", resource.NewBoolProperty(true))
			assertPropertyMapMember(l, outputs, "output_false", resource.NewBoolProperty(false))
		},
	},
	"l2-resource-simple": {
		providers: []plugin.Provider{&simpleProvider{}},
		assert: func(l *L, res result.Result, snap *deploy.Snapshot, changes display.ResourceChanges) {
			requireStackResource(l, res, changes)

			// Check we have the one simple resource in the snapshot, it's provider and the stack.
			require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

			provider := snap.Resources[1]
			assert.Equal(l, "pulumi:providers:simple", provider.Type.String(), "expected simple provider")

			simple := snap.Resources[2]
			assert.Equal(l, "simple:index:Resource", simple.Type.String(), "expected simple resource")

			want := resource.NewPropertyMapFromMap(map[string]any{"value": true})
			assert.Equal(l, want, simple.Inputs, "expected inputs to be {value: true}")
			assert.Equal(l, simple.Inputs, simple.Outputs, "expected inptus and outputs to match")
		},
	},
}

// A providerLoader is a schema loader that loads schemas from a given set of providers.
type providerLoader struct {
	providers []plugin.Provider
}

func (l *providerLoader) LoadPackageReference(pkg string, version *semver.Version) (schema.PackageReference, error) {
	// Find the provider with the given package name
	var provider plugin.Provider
	for _, p := range l.providers {
		if string(p.Pkg()) == pkg {
			info, err := p.GetPluginInfo()
			if err != nil {
				return nil, fmt.Errorf("get plugin info for %s: %w", pkg, err)
			}

			if version == nil || (info.Version != nil && version.EQ(*info.Version)) {
				provider = p
				break
			}
		}
	}

	if provider == nil {
		return nil, fmt.Errorf("could not load schema for %s, provider not known", pkg)
	}

	jsonSchema, err := provider.GetSchema(0)
	if err != nil {
		return nil, fmt.Errorf("get schema for %s: %w", pkg, err)
	}

	var spec schema.PartialPackageSpec
	if _, err := json.Parse(jsonSchema, &spec, json.ZeroCopy); err != nil {
		return nil, err
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

func (eng *engineServer) GetLanguageTests(
	ctx context.Context,
	req *enginerpc.GetLanguageTestsRequest,
) (*enginerpc.GetLanguageTestsResponse, error) {
	tests := make([]string, 0, len(languageTests))
	for testName := range languageTests {
		tests = append(tests, testName)
	}

	return &enginerpc.GetLanguageTestsResponse{
		Tests: tests,
	}, nil
}

func makeTestResponse(msg string) *enginerpc.RunLanguageTestResponse {
	return &enginerpc.RunLanguageTestResponse{
		Success: false,
		Message: msg,
	}
}

func copyDirectory(fs iofs.FS, src string, dst string) error {
	return iofs.WalkDir(fs, src, func(path string, d iofs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(src, path)
		contract.AssertNoErrorf(err, "path %s should be relative to %s", path, src)

		srcPath := filepath.Join(src, relativePath)
		dstPath := filepath.Join(dst, relativePath)
		contract.Assertf(srcPath == path, "srcPath %s should be equal to path %s", srcPath, path)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o700)
		}

		srcFile, err := fs.Open(srcPath)
		if err != nil {
			return fmt.Errorf("open file %s: %w", srcPath, err)
		}
		defer srcFile.Close()

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return fmt.Errorf("create file %s: %w", dstPath, err)
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
			return fmt.Errorf("copy file %s->%s: %w", srcPath, dstPath, err)
		}

		return nil
	})
}

// compareDirectories compares two directories, returning a list of validation failures where files had
// different contents, or we only present in on of the directories. If allowNewFiles is true then it's ok to
// have extra files in the actual directory, we use this for checking building SDKs doesn't mutate any files,
// but doing so might add new build files (which would then normally be .gitignored).
func compareDirectories(actualDir, expectedDir string, allowNewFiles bool) ([]string, error) {
	var validations []string
	// Check that every file in expected is also in actual with the same content
	err := filepath.WalkDir(expectedDir, func(path string, d iofs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// No need to check directories, just recurse into them
		if d.IsDir() {
			return nil
		}

		relativePath, err := filepath.Rel(expectedDir, path)
		contract.AssertNoErrorf(err, "path %s should be relative to %s", path, expectedDir)

		// Check that the file is present in the expected directory and has the same contents
		expectedContents, err := os.ReadFile(filepath.Join(expectedDir, relativePath))
		if err != nil {
			return fmt.Errorf("read expected file: %w", err)
		}

		actualPath := filepath.Join(actualDir, relativePath)
		actualContents, err := os.ReadFile(actualPath)
		// An error here is a test failure rather than an error, add this to the validation list
		if err != nil {
			validations = append(validations, fmt.Sprintf("expected file %s could not be read", relativePath))
			// Move on to the next file
			return nil
		}

		if !bytes.Equal(actualContents, expectedContents) {
			// TODO: Find a way to show better diffs here
			validations = append(validations, fmt.Sprintf("expected file %s does not match actual file", relativePath))
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk expected dir: %w", err)
	}

	// Now walk the actual directory and check every file found is present in the expected directory, i.e.
	// there aren't any new files that aren't expected. We've already done contents checking so we just need
	// existence checks.
	if !allowNewFiles {
		err = filepath.WalkDir(actualDir, func(path string, d iofs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// No need to check directories
			if d.IsDir() {
				return nil
			}

			relativePath, err := filepath.Rel(actualDir, path)
			contract.AssertNoErrorf(err, "path %s should be relative to %s", path, actualDir)

			// Just need to see if this file exists in expected, if it doesn't return add a validation failure.
			_, err = os.Stat(filepath.Join(expectedDir, relativePath))
			if err == nil {
				// File exists in expected, we've already done a contents check so just move on to
				// the next file.
				return nil
			}

			// Check if this was a NotFound error in which case add a validation failure, else return the error
			if os.IsNotExist(err) {
				validations = append(validations, fmt.Sprintf("file %s is not expected", relativePath))
				return nil
			}

			return err
		})
		if err != nil {
			return nil, fmt.Errorf("walk actual dir: %w", err)
		}
	}

	return validations, nil
}

// Do a snapshot check of the generated source code against the snapshot code. If PULUMI_ACCEPT is true just
// write the new files instead.
func doSnapshot(sourceDirectory, snapshotDirectory string) ([]string, error) {
	if cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT")) {
		// Write files
		err := os.RemoveAll(snapshotDirectory)
		if err != nil {
			return nil, fmt.Errorf("remove snapshot dir: %w", err)
		}
		err = os.MkdirAll(snapshotDirectory, 0o755)
		if err != nil {
			return nil, fmt.Errorf("create snapshot dir: %w", err)
		}
		err = copyDirectory(os.DirFS(sourceDirectory), ".", snapshotDirectory)
		if err != nil {
			return nil, fmt.Errorf("copy snapshot dir: %w", err)
		}
		return nil, nil
	}
	// Validate files, we need to walk twice to get this correct because we need to check all expected
	// files are present, but also that no unexpected files are present.
	validations, err := compareDirectories(sourceDirectory, snapshotDirectory, false /* allowNewFiles */)
	if err != nil {
		return nil, err
	}

	return validations, nil
}

type testHost struct {
	host        plugin.Host
	runtime     plugin.LanguageRuntime
	runtimeName string
	providers   map[string]plugin.Provider
}

var _ plugin.Host = (*testHost)(nil)

func (h *testHost) ServerAddr() string {
	panic("not implemented")
}

func (h *testHost) Log(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	panic("not implemented")
}

func (h *testHost) LogStatus(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	panic("not implemented")
}

func (h *testHost) Analyzer(nm tokens.QName) (plugin.Analyzer, error) {
	panic("not implemented")
}

func (h *testHost) PolicyAnalyzer(
	name tokens.QName, path string, opts *plugin.PolicyAnalyzerOptions,
) (plugin.Analyzer, error) {
	panic("not implemented")
}

func (h *testHost) ListAnalyzers() []plugin.Analyzer {
	// We're not using analyzers for matrix tests, yet.
	return nil
}

func (h *testHost) Provider(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
	// Look in the providers map for this provider
	if version == nil {
		return nil, fmt.Errorf("unexpected provider request with no version")
	}

	key := fmt.Sprintf("%s@%s", pkg, version)
	provider, has := h.providers[key]
	if !has {
		return nil, fmt.Errorf("unknown provider %s", key)
	}
	return provider, nil
}

func (h *testHost) CloseProvider(provider plugin.Provider) error {
	// We don't actually need to close off these providers.
	return nil
}

func (h *testHost) LanguageRuntime(
	root, pwd, runtime string, options map[string]interface{},
) (plugin.LanguageRuntime, error) {
	return h.runtime, nil
}

// difference returns the elements in `a` that aren't in `b`.
func difference(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

func (h *testHost) EnsurePlugins(plugins []workspace.PluginSpec, kinds plugin.Flags) error {
	// EnsurePlugins will be called with the result of GetRequiredPlugins, so we can use this to check
	// that that returned the expected plugins (with expected versions).
	expected := []string{
		fmt.Sprintf("language-%s@<nil>", h.runtimeName),
	}
	for _, provider := range h.providers {
		pkg := provider.Pkg()
		version, err := getProviderVersion(provider)
		if err != nil {
			return fmt.Errorf("get provider version %s: %w", pkg, err)
		}
		expected = append(expected, fmt.Sprintf("resource-%s@%s", pkg, version))
	}

	actual := make([]string, len(plugins))
	for i, plugin := range plugins {
		actual[i] = fmt.Sprintf("%s-%s@%s", plugin.Kind, plugin.Name, plugin.Version)
	}

	// Symmetric difference, we want to know if there are any unexpected plugins, or any missing plugins.
	diff := append(difference(expected, actual), difference(actual, expected)...)
	if len(diff) > 0 {
		return fmt.Errorf("unexpected required plugins: actual %v, expected %v", actual, expected)
	}

	return nil
}

func (h *testHost) ResolvePlugin(
	kind workspace.PluginKind, name string, version *semver.Version,
) (*workspace.PluginInfo, error) {
	panic("not implemented")
}

func (h *testHost) GetProjectPlugins() []workspace.ProjectPlugin {
	// We're not using project plugins, in fact this method shouldn't even really exists on Host given it's
	// just reading off Pulumi.yaml.
	return nil
}

func (h *testHost) SignalCancellation() error {
	panic("not implemented")
}

func (h *testHost) Close() error {
	return nil
}

type testToken struct {
	LanguagePluginName   string
	LanguagePluginTarget string
	TemporaryDirectory   string
	SnapshotDirectory    string
	CoreArtifact         string
}

func (eng *engineServer) PrepareLanguageTests(
	ctx context.Context,
	req *enginerpc.PrepareLanguageTestsRequest,
) (*enginerpc.PrepareLanguageTestsResponse, error) {
	if req.LanguagePluginName == "" {
		return nil, fmt.Errorf("language plugin name must be specified")
	}
	if req.LanguagePluginTarget == "" {
		return nil, fmt.Errorf("language plugin target must be specified")
	}
	if req.SnapshotDirectory == "" {
		return nil, fmt.Errorf("snapshot directory must be specified")
	}
	if req.TemporaryDirectory == "" {
		return nil, fmt.Errorf("temporary directory must be specified")
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
	pctx, err := plugin.NewContextWithContext(ctx, snk, snk, nil, "", "", nil, false, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("setup plugin context: %w", err)
	}
	defer func() {
		contract.IgnoreError(pctx.Close())
	}()

	// Connect to the language host
	conn, err := grpc.Dial(req.LanguagePluginTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial language plugin: %w", err)
	}

	languageClient := plugin.NewLanguageRuntimeClient(pctx, "uut", pulumirpc.NewLanguageRuntimeClient(conn))

	// Setup the artifacts directory
	err = os.MkdirAll(filepath.Join(req.TemporaryDirectory, "artifacts"), 0o755)
	if err != nil {
		return nil, fmt.Errorf("create artifacts directory: %w", err)
	}

	// Build the core SDK
	version := semver.MustParse("1.0.0")
	coreArtifact, err := languageClient.Pack(
		req.CoreSdkDirectory, version, filepath.Join(req.TemporaryDirectory, "artifacts"))
	if err != nil {
		return nil, fmt.Errorf("pack core SDK: %w", err)
	}

	tokenBytes, err := json.Marshal(&testToken{
		LanguagePluginName:   req.LanguagePluginName,
		LanguagePluginTarget: req.LanguagePluginTarget,
		TemporaryDirectory:   req.TemporaryDirectory,
		SnapshotDirectory:    req.SnapshotDirectory,
		CoreArtifact:         coreArtifact,
	})
	contract.AssertNoErrorf(err, "could not marshal test token")

	b64token := b64.StdEncoding.EncodeToString(tokenBytes)

	return &enginerpc.PrepareLanguageTestsResponse{
		Token: b64token,
	}, nil
}

func getProviderVersion(provider plugin.Provider) (semver.Version, error) {
	pkg := provider.Pkg()
	info, err := provider.GetPluginInfo()
	if err != nil {
		return semver.Version{}, fmt.Errorf("get plugin info for %s: %w", pkg, err)
	}
	if info.Version == nil {
		return semver.Version{}, fmt.Errorf("provider %s has no version", pkg)
	}
	return *info.Version, nil
}

// TODO: We need a RunLanguageTest(t *testing.T) function
// that handles the machinery of plugging the language test logs
// into the testing.T.

func (eng *engineServer) RunLanguageTest(
	ctx context.Context, req *enginerpc.RunLanguageTestRequest,
) (*enginerpc.RunLanguageTestResponse, error) {
	test, has := languageTests[req.Test]
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

	// Create a diagnostics sink for the test
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	snk := diag.DefaultSink(stdout, stderr, diag.FormatOptions{
		Color: colors.Never,
	})

	// Start up a plugin context
	pctx, err := plugin.NewContextWithContext(
		ctx, snk, snk, nil, token.TemporaryDirectory, token.TemporaryDirectory, nil, false, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("setup plugin context: %w", err)
	}

	// Connect to the language host
	conn, err := grpc.Dial(token.LanguagePluginTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial language plugin: %w", err)
	}

	languageClient := plugin.NewLanguageRuntimeClient(pctx, "uut", pulumirpc.NewLanguageRuntimeClient(conn))

	// Create a source directory for the test
	sourceDir := filepath.Join(token.TemporaryDirectory, "source", req.Test)
	err = os.MkdirAll(sourceDir, 0o700)
	if err != nil {
		return nil, fmt.Errorf("create source dir: %w", err)
	}

	// Find and copy the tests PCL code to the source dir
	err = copyDirectory(languageTestdata, filepath.Join("testdata", req.Test), sourceDir)
	if err != nil {
		return nil, fmt.Errorf("copy source test data: %w", err)
	}

	// Generate SDKs for all the providers we need
	loader := &providerLoader{providers: test.providers}
	loaderServer := schema.NewLoaderServer(loader)
	grpcServer, err := plugin.NewServer(pctx, schema.LoaderRegistration(loaderServer))
	if err != nil {
		return nil, err
	}
	defer contract.IgnoreClose(grpcServer)

	artifactsDir := filepath.Join(token.TemporaryDirectory, "artifacts")

	// We always override the core "pulumi" package to point to the local core SDK we built as part of test
	// setup.
	localDependencies := map[string]string{
		"pulumi": token.CoreArtifact,
	}
	for _, provider := range test.providers {
		pkg := string(provider.Pkg())
		version, err := getProviderVersion(provider)
		if err != nil {
			return nil, err
		}

		sdkName := fmt.Sprintf("%s-%s", pkg, version)
		sdkTempDir := filepath.Join(token.TemporaryDirectory, "sdks", sdkName)
		err = os.MkdirAll(sdkTempDir, 0o755)
		if err != nil {
			return nil, fmt.Errorf("create temp sdks dir: %w", err)
		}

		schema, err := provider.GetSchema(0)
		if err != nil {
			return nil, fmt.Errorf("get schema for provider %s: %w", pkg, err)
		}

		err = languageClient.GeneratePackage(sdkTempDir, string(schema), nil, grpcServer.Addr())
		if err != nil {
			return makeTestResponse(fmt.Sprintf("generate package %s: %v", pkg, err)), nil
		}

		snapshotDir := filepath.Join(token.SnapshotDirectory, "sdks", sdkName)
		validations, err := doSnapshot(sdkTempDir, snapshotDir)
		if err != nil {
			return nil, fmt.Errorf("sdk snapshot validation for %s: %w", pkg, err)
		}
		if len(validations) > 0 {
			return makeTestResponse(
				fmt.Sprintf("sdk snapshot validation for %s failed:\n%s",
					pkg, strings.Join(validations, "\n"))), nil
		}

		// Pack the SDK and add it to the artifact dependencies, we do this in the temporary directory so that
		// any intermediate build files don't end up getting captured in the snapshot folder.
		sdkArtifact, err := languageClient.Pack(sdkTempDir, version, artifactsDir)
		if err != nil {
			return nil, fmt.Errorf("sdk packing for %s: %w", pkg, err)
		}
		localDependencies[pkg] = sdkArtifact

		// Check that packing the SDK didn't mutate any files, but it may have added ignorable build files.
		validations, err = compareDirectories(sdkTempDir, snapshotDir, true /* allowNewFiles */)
		if err != nil {
			return nil, fmt.Errorf("sdk post pack change validation for %s: %w", pkg, err)
		}
		if len(validations) > 0 {
			return makeTestResponse(
				fmt.Sprintf("sdk post pack change validation for %s failed:\n%s",
					pkg, strings.Join(validations, "\n"))), nil
		}
	}

	// Create a directory for the project
	projectDir := filepath.Join(token.TemporaryDirectory, "projects", req.Test)
	err = os.MkdirAll(projectDir, 0o755)
	if err != nil {
		return nil, fmt.Errorf("create project dir: %w", err)
	}

	// Generate the project and read in the Pulumi.yaml
	projectJSON := fmt.Sprintf(`{"name": "%s"}`, req.Test)

	// TODO: We don't report back warning diagnostics here
	diagnostics, err := languageClient.GenerateProject(
		sourceDir, projectDir, projectJSON, true, grpcServer.Addr(), localDependencies)
	if err != nil {
		return makeTestResponse(fmt.Sprintf("generate project: %v", err)), nil
	}
	if diagnostics.HasErrors() {
		return makeTestResponse(fmt.Sprintf("generate project: %v", diagnostics)), nil
	}

	snapshotDir := filepath.Join(token.SnapshotDirectory, "projects", req.Test)
	validations, err := doSnapshot(projectDir, snapshotDir)
	if err != nil {
		return nil, fmt.Errorf("program snapshot validation: %w", err)
	}
	if len(validations) > 0 {
		return makeTestResponse(fmt.Sprintf("program snapshot validation failed:\n%s", strings.Join(validations, "\n"))), nil
	}

	// TODO: We don't capture stdout/stderr from the language plugin, so we can't show it back to the test.
	err = languageClient.InstallDependencies(projectDir)
	if err != nil {
		return makeTestResponse(fmt.Sprintf("install dependencies: %v", err)), nil
	}
	// TODO: This should only add new things, don't modify

	project, err := workspace.LoadProject(filepath.Join(projectDir, "Pulumi.yaml"))
	if err != nil {
		return makeTestResponse(fmt.Sprintf("load project: %v", err)), nil
	}

	// Create a temp dir for the a filestate backend to run in for the test
	backendDir := filepath.Join(token.TemporaryDirectory, "backends", req.Test)
	err = os.MkdirAll(backendDir, 0o755)
	if err != nil {
		return nil, fmt.Errorf("create temp backend dir: %w", err)
	}
	testBackend, err := filestate.New(ctx, snk, "file://"+backendDir, project)
	if err != nil {
		return nil, fmt.Errorf("create filestate backend: %w", err)
	}

	// Create a new stack for the test
	stackReference, err := testBackend.ParseStackReference("test")
	if err != nil {
		return nil, fmt.Errorf("parse test stack reference: %w", err)
	}
	s, err := testBackend.CreateStack(ctx, stackReference, projectDir, nil)
	if err != nil {
		return nil, fmt.Errorf("create test stack: %w", err)
	}

	providers := make(map[string]plugin.Provider)
	for _, provider := range test.providers {
		version, err := getProviderVersion(provider)
		if err != nil {
			return nil, err
		}
		providers[fmt.Sprintf("%s@%s", provider.Pkg(), version)] = provider
	}

	host := &testHost{
		host:        pctx.Host,
		runtime:     languageClient,
		runtimeName: token.LanguagePluginName,
		providers:   providers,
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
		Engine: engine.UpdateOptions{
			Host: host,
		},
	}
	sm := b64secrets.NewBase64SecretsManager()
	dec, err := sm.Decrypter()
	contract.AssertNoErrorf(err, "base64 must be able to create a Decrypter")

	cfg := backend.StackConfiguration{
		Config:    test.config,
		Decrypter: dec,
	}

	changes, res := s.Update(ctx, backend.UpdateOperation{
		Proj:               project,
		Root:               projectDir,
		Opts:               opts,
		M:                  &backend.UpdateMetadata{},
		StackConfiguration: cfg,
		SecretsManager:     sm,
		SecretsProvider:    stack.DefaultSecretsProvider,
		Scopes:             backend.CancellationScopes,
	})

	var snap *deploy.Snapshot
	if res == nil {
		// Refetch the stack so we can get the snapshot
		s, err = testBackend.GetStack(ctx, stackReference)
		if err != nil {
			return nil, fmt.Errorf("get stack: %w", err)
		}

		snap, err = s.Snapshot(ctx, stack.DefaultSecretsProvider)
		if err != nil {
			return nil, fmt.Errorf("snapshot: %w", err)
		}
	}

	// TODO:
	// Consider makign res, snap, and changes available to the test
	// as methods on some object with internal state.
	result := WithL(func(l *L) {
		test.assert(l, res, snap, changes)
	})

	return &enginerpc.RunLanguageTestResponse{
		Success: !result.Failed,
		// TODO: Send back as a list instead of a string.
		// TODO: Consider streaming messages back instead.
		Message: strings.Join(result.Messages, "\n"),
		Stdout:  stdout.String(),
		Stderr:  stderr.String(),
	}, nil
}
