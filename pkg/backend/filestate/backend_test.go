package filestate

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	pipe "github.com/b4b4r07/go-pipe"
	copier "github.com/otiai10/copy"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cancel"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

var (
	localURL        = "file://."
	testDir         = "testdata"
	testProjectPath = "testdata/Pulumi.yaml"
)

type mockCancellationScope struct {
	context *cancel.Context
}

func (s *mockCancellationScope) Context() *cancel.Context {
	return s.context
}

func (s *mockCancellationScope) Close() {
}

type mockCancellationScopeSource int

var cancellationScopes = backend.CancellationScopeSource(mockCancellationScopeSource(0))

func (mockCancellationScopeSource) NewScope(events chan<- engine.Event, isPreview bool) backend.CancellationScope {
	cancelContext, _ := cancel.NewContext(context.Background())

	c := &mockCancellationScope{
		context: cancelContext,
	}

	return c
}

var be Backend // Shared, do not run tests in parallel
var stores map[string]string
var isShort bool

func TestMain(m *testing.M) {
	flag.Parse()
	isShort = testing.Short()

	stores = make(map[string]string)
	if azureURL := os.Getenv("AZURE_URL"); azureURL != "" {
		stores["azure"] = azureURL
	}
	stores["local"] = localURL

	// Disable stack backups for tests to avoid filling up ~/.pulumi/backups with unnecessary
	// backups of test stacks.
	if err := os.Setenv(DisableCheckpointBackupsEnvVar, "1"); err != nil {
		fmt.Printf("error setting env var '%s': %v\n", DisableCheckpointBackupsEnvVar, err)
		os.Exit(1)
	}

	exCode := m.Run()
	os.Exit(exCode)
}

type backendInfo struct {
	name string
	url  string
}

func TestStorageBackends(t *testing.T) {
	backends := []backendInfo{}
	for k, v := range stores {
		backends = append(backends, backendInfo{
			name: k,
			url:  v,
		})
	}

	ctx := context.Background()
	for _, backend := range backends {
		var err error
		be, err = Login(ctx, cmdutil.Diag(), backend.url)
		if err != nil {
			panic(fmt.Sprintf("error creating new %s backend to test: %+v", backend.name, err))
		}
		runTestPlan(backend.name, t) // TODO: Handle success/failure
	}
}

func runTestPlan(backendName string, t *testing.T) bool {
	testPlan := []func(t *testing.T){
		ParseStackReference,
		CreateStack,
		RemoveStack,
		GetStack,
		ListStacks,
		Update,
		UpdateLocking,
	}
	for _, test := range testPlan {
		if success := runTest(backendName, test, t); !success {
			return false
		}
	}
	return true
}

func runTest(backendName string, testFn func(t *testing.T), t *testing.T) bool {
	testName := strings.Replace(fmt.Sprintf("%s%s", strings.Title(backendName), getFunctionName(testFn)), ".", "", -1)
	return t.Run(testName, testFn)
}

func getFunctionName(i interface{}) string {
	parts := strings.Split(runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name(), ".")
	return parts[len(parts)-1]
}

func ParseStackReference(t *testing.T) {
	tt := []struct {
		stackName string
	}{
		{
			stackName: "test",
		},
	}
	for _, test := range tt {
		stackRef, err := be.ParseStackReference(test.stackName)
		if err != nil {
			t.Fatalf("error parsing stack %s: %+v", test.stackName, err)
		}
		name := stackRef.Name()
		if name != tokens.AsQName(test.stackName) {
			t.Fatalf("expected name to be %s but got %s", test.stackName, name)
		}
	}
}

func CreateStack(t *testing.T) {
	tt := []struct {
		stackName string
	}{
		{
			stackName: "test",
		},
	}

	tearDown := setUp(t)
	defer tearDown()

	for _, test := range tt {
		stackRef, err := be.ParseStackReference(test.stackName)
		if err != nil {
			t.Fatalf("error parsing stack %s: %+v", test.stackName, err)
		}
		ctx := context.Background()
		createStack(ctx, stackRef, t)
		stackExists(stackRef, t)

		tearDownPulumi() // Removes the dirty .pulumi between test cases
	}
}

func GetStack(t *testing.T) {
	tt := []struct {
		stackName   string
		createStack bool
	}{
		{
			stackName:   "test",
			createStack: false,
		},
		{
			stackName:   "test",
			createStack: true,
		},
	}

	tearDown := setUp(t)
	defer tearDown()

	for _, test := range tt {
		stackRef, err := be.ParseStackReference(test.stackName)
		if err != nil {
			t.Fatalf("error parsing stack %s: %+v", test.stackName, err)
		}
		ctx := context.Background()
		if test.createStack {
			createStack(ctx, stackRef, t)
			stackExists(stackRef, t)
		}
		stack, err := be.GetStack(ctx, stackRef)
		if err != nil {
			t.Fatalf("error getting stack %s: %+v", test.stackName, err)
		}
		stackCreated := (stack != nil)
		if test.createStack != stackCreated {
			t.Fatalf("expected stack to be created and got only when createStack is set to true")
		}

		tearDownPulumi() // Removes the dirty .pulumi between test cases
	}
}

func ListStacks(t *testing.T) {
	tt := []struct {
		numStacks int
	}{
		{
			numStacks: 2,
		},
		{
			numStacks: 5,
		},
		{
			numStacks: 10,
		},
	}

	tearDown := setUp(t)
	defer tearDown()

	for _, test := range tt {
		ctx := context.Background()
		for i := 0; i < test.numStacks; i++ {
			stackName := fmt.Sprintf("stack%d", i)
			stackRef, err := be.ParseStackReference(stackName)
			if err != nil {
				t.Fatalf("error parsing stack %s: %+v", stackName, err)
			}
			createStack(ctx, stackRef, t)
			stackExists(stackRef, t)
		}
		stacks, err := be.ListStacks(ctx, nil)
		if err != nil {
			t.Fatalf("error listing stacks: %+v", err)
		}
		if len(stacks) != test.numStacks {
			t.Fatalf("expected %d stacks but got %d", test.numStacks, len(stacks))
		}

		tearDownPulumi() // Removes the dirty .pulumi between test cases
	}
}

func RemoveStack(t *testing.T) {
	tt := []struct {
		stackName string
	}{
		{
			stackName: "test",
		},
	}

	tearDown := setUp(t)
	defer tearDown()

	for _, test := range tt {
		ctx := context.Background()
		stackRef, err := be.ParseStackReference(test.stackName)
		if err != nil {
			t.Fatalf("error parsing stack %s: %+v", test.stackName, err)
		}
		createStack(ctx, stackRef, t)
		stackExists(stackRef, t)
		removeStack(ctx, stackRef, t)
		stackDoesNotExist(stackRef, t)

		tearDownPulumi() // Removes the dirty .pulumi between test cases
	}
}

func Update(t *testing.T) {
	if isShort {
		// Skip as these test cases require the pulumi
		// binaries to be in the $PATH
		t.Skip("Skipping update test due to short flag")
	}

	tt := []struct {
		stackName string
	}{
		{
			stackName: "test",
		},
	}

	tearDown := setUp(t)
	defer tearDown()

	for _, test := range tt {
		ctx := context.Background()
		stackRef, err := be.ParseStackReference(test.stackName)
		if err != nil {
			t.Fatalf("error parsing stack %s: %+v", test.stackName, err)
		}

		createStack(ctx, stackRef, t)
		stackExists(stackRef, t)

		// Load project and a new project stack
		proj, err := workspace.LoadProject("Pulumi.yaml")
		if err != nil {
			t.Fatalf("error loading project from file %s: %+v", testProjectPath, err)
		}
		projStack, err := workspace.DetectProjectStack(stackRef.Name())
		if err != nil {
			t.Fatalf("error detecting project stack for stack %s: %+v", stackRef.Name(), err)
		}

		// os.Chdir() does not update the shell working directory
		// so we need to pass the test dir path to npm install.
		testDir, err := os.Getwd() // nolint: vetshadow
		if err != nil {
			t.Fatalf("error getting testdata working directory: %+v", err)
		}
		installPackages(testDir, t)

		// Set the configuration values (w,x,y) for the test
		// pulumi project to use.
		keys := []string{"simple:w", "simple:x", "simple:y"}
		for _, key := range keys {
			k, err := config.ParseKey(key)
			if err != nil {
				t.Fatalf("error parsing key %s: %+v", key, err)
			}
			v := config.NewValue("10")
			projStack.Config[k] = v
		}
		if err := workspace.SaveProjectStack(stackRef.Name(), projStack); err != nil {
			t.Fatalf("error saving project stack: %+v", err)
		}

		// Perform two sequential updates and check that
		// the expected number of changes are detected
		up1, _ := applyUpdate(ctx, testDir, proj, stackRef, t)
		if !up1.HasChanges() {
			t.Fatalf("expected changes on first update")
		}
		up2, destroy := applyUpdate(ctx, testDir, proj, stackRef, t)
		if up2.HasChanges() {
			t.Fatalf("expected no changes between first and second update")
		}

		// Remove provisioned resources
		destroy()

		tearDownPulumi()                // Removes the dirty .pulumi between test cases
		tearDownProject(test.stackName) // Removes generated project files and node_modules between test cases
	}
}

func UpdateLocking(t *testing.T) {
	if isShort {
		// Skip as these test cases require the pulumi
		// binaries to be in the $PATH
		t.Skip("Skipping update test due to short flag")
	}

	tt := []struct {
		stackName string
	}{
		{
			stackName: "test",
		},
	}

	tearDown := setUp(t)
	defer tearDown()

	for _, test := range tt {
		ctx := context.Background()
		stackRef, err := be.ParseStackReference(test.stackName)
		if err != nil {
			t.Fatalf("error parsing stack %s: %+v", test.stackName, err)
		}
		createStack(ctx, stackRef, t)
		stackExists(stackRef, t)

		// Load project and a new project stack
		proj, err := workspace.LoadProject("Pulumi.yaml")
		if err != nil {
			t.Fatalf("error loading project from file %s: %+v", testProjectPath, err)
		}
		projStack, err := workspace.DetectProjectStack(stackRef.Name())
		if err != nil {
			t.Fatalf("error detecting project stack for stack %s: %+v", stackRef.Name(), err)
		}

		// os.Chdir() does not update the shell working directory
		// so we need to pass the test dir path to npm install.
		testDir, err := os.Getwd() // nolint: vetshadow
		if err != nil {
			t.Fatalf("error getting testdata working directory: %+v", err)
		}
		installPackages(testDir, t)

		// Set the configuration values (w,x,y) for the test
		// pulumi project to use.
		keys := []string{"simple:w", "simple:x", "simple:y"}
		for _, key := range keys {
			k, err := config.ParseKey(key)
			if err != nil {
				t.Fatalf("error parsing key %s: %+v", key, err)
			}
			v := config.NewValue("10")
			projStack.Config[k] = v
		}
		if err := workspace.SaveProjectStack(stackRef.Name(), projStack); err != nil {
			t.Fatalf("error saving project stack: %+v", err)
		}

		// Perform two parallel updates and check that
		// the locking mechanism stops overlaps
		var wg sync.WaitGroup
		wg.Add(2)

		timeApplyUpdate := func(c context.Context, d string, p *workspace.Project,
			r backend.StackReference, t *testing.T) (time.Duration, func()) {
			start := time.Now()
			_, destroy := applyUpdate(c, d, p, r, t)

			elapsed := time.Since(start)
			return elapsed, destroy
		}

		times := make([]time.Duration, 0) // Used to ignore ordering of goroutine execution
		var time1 time.Duration
		var time2 time.Duration
		var destroy func()
		go func() {
			time1, destroy = timeApplyUpdate(ctx, testDir, proj, stackRef, t)
			wg.Done()
			times = append(times, time1)
		}()
		go func() {
			time2, destroy = timeApplyUpdate(ctx, testDir, proj, stackRef, t)
			wg.Done()
			times = append(times, time2)
		}()
		wg.Wait()

		// Take a benchmark from the first goroutine
		// to complete minus an offset to allow for
		// the initial goroutine startup cost.
		base := times[0].Seconds() * 0.6
		// Create a range of acceptable times based
		// on the base benchmark and make sure the
		// second goroutine falls in it.
		// The lower treshold gives us an indicator
		// that the goroutines ran in sequence.
		// The upper treshold gives us an indiciator
		// we're not deadlocking.
		upperTreshold := base * 1.5
		lowerTreshold := base * 0.5
		diff := times[1] - times[0]
		diffSec := diff.Seconds()
		if diffSec < lowerTreshold || diffSec > upperTreshold {
			t.Fatalf(`expected time diff between updates to be above %f seconds and below %f
			seconds but was %f seconds`,
				lowerTreshold, upperTreshold, diffSec)
		}

		destroy() // Remove provisioned resources

		tearDownPulumi()                // Removes the dirty .pulumi between test cases
		tearDownProject(test.stackName) // Removes generated project files and node_modules between test cases
	}
}

// setUp creates a new temp directory and copies the contents
// of testdata into it and sets it as the current directory.
// The test will then run in the temp directory.
// The temp directory will then be torn down and the current
// directory set back to the original directory.
func setUp(t *testing.T) func() {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("error getting current working directory: %+v", err)
	}
	tmp, err := ioutil.TempDir("", filepath.Base(t.Name()))
	if err != nil {
		t.Fatalf("error creating temp dir: %+v", err)
	}
	err = copier.Copy(testDir, tmp)
	if err != nil {
		t.Fatalf("error copying %s to %s: %+v", testDir, tmp, err)
	}
	err = os.Chdir(tmp)
	if err != nil {
		t.Fatalf("error changing to testdata directory: %+v", err)
	}
	// This function should be deferred by the caller
	// to clean up the temp directory after the test
	// has finished.
	return func() {
		_ = os.RemoveAll(tmp)
		_ = os.Chdir(cwd) // Must run last
	}
}

// tearDownPulumi cleans the pulumi directory of any
// stacks created during the test
func tearDownPulumi() {
	ctx := context.Background()
	stacks, err := be.ListStacks(ctx, nil)
	if err != nil {
		panic("unable to list stacks during teardown")
	}
	for _, stack := range stacks {
		_, err = be.RemoveStack(ctx, stack.Name(), false)
		if err != nil {
			panic(fmt.Errorf("unable to destroy resources for stack %s during teardown", stack.Name().String()))
		}
	}
}

// tearDownProject remove any project files created
// during a test run
func tearDownProject(stackName string) {
	_ = os.RemoveAll(fmt.Sprintf("Pulumi.%s.yaml", stackName))
	_ = os.RemoveAll("node_modules")
}

func applyUpdate(ctx context.Context, root string, project *workspace.Project,
	stackRef backend.StackReference, t *testing.T) (engine.ResourceChanges, func()) {
	m := &backend.UpdateMetadata{
		Message:     "",
		Environment: make(map[string]string),
	}

	engineOpts := engine.UpdateOptions{
		Analyzers: []string{},
		Parallel:  0,
		Debug:     false,
		Refresh:   false,
	}

	op := backend.UpdateOperation{
		Proj: project,
		Root: root,
		M:    m,
		Opts: backend.UpdateOptions{
			Engine: engineOpts,
			Display: display.Options{
				Color: cmdutil.GetGlobalColorization(),
			},
			AutoApprove: true,
			SkipPreview: true,
		},
		Scopes: cancellationScopes,
	}

	res, err := be.Update(ctx, stackRef, op)
	if err != nil {
		t.Fatalf("error updating resources: %+v", err)
	}
	return res, func() {
		be.Destroy(ctx, stackRef, op)
	}
}

// installPackages runs `yarn || npm install` in the desired directory.
// Expect this to take some time.
func installPackages(dir string, t *testing.T) {
	var b bytes.Buffer
	if err := pipe.Command(&b,
		exec.Command("yarn"),
		exec.Command("npm", "install"),
	); err != nil {
		t.Fatalf("failed to execute `yarn || npm install` in %s: %+v", dir, err)
	}
	if _, err := io.Copy(os.Stdout, &b); err != nil {
		t.Fatalf("failed to copy command output buffer to stdout: %+v", err)
	}
}

func createStack(ctx context.Context, stackRef backend.StackReference, t *testing.T) {
	stack, err := be.CreateStack(ctx, stackRef, nil)
	if err != nil {
		t.Fatalf("error creating stack %s: %+v", stackRef.Name().String(), err)
	}
	if stack == nil {
		t.Fatalf("expected stack to be non nil")
	}
}

func stackExists(stackRef backend.StackReference, t *testing.T) {
	stack, err := be.GetStack(context.Background(), stackRef)
	if err != nil {
		t.Fatalf("error getting stack %s: %+v", stackRef.Name().String(), err)
	}
	if stack == nil {
		t.Fatalf("expected stack %s to have been created", stackRef.Name().String())
	}
}

func stackDoesNotExist(stackRef backend.StackReference, t *testing.T) {
	stack, err := be.GetStack(context.Background(), stackRef)
	if err != nil {
		t.Fatalf("error getting stack %s: %+v", stackRef.Name().String(), err)
	}
	if stack != nil {
		t.Fatalf("expected stack %s to have been removed", stackRef.Name().String())
	}
}

func removeStack(ctx context.Context, stackRef backend.StackReference, t *testing.T) {
	_, err := be.RemoveStack(ctx, stackRef, false)
	if err != nil {
		t.Fatalf("error removing stack %s: %+v", stackRef.Name().String(), err)
	}
}
