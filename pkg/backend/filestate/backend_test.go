package filestate

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"

	copier "github.com/otiai10/copy"
	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/workspace"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/tokens"

	"github.com/pulumi/pulumi/pkg/util/cancel"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

const (
	localURL         = "file://."
	localBackendPath = ".pulumi"
	testDir          = "testdata"
	testProjectPath  = "testdata/Pulumi.yaml"
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

var be Backend

func TestMain(m *testing.M) {
	var err error
	be, err = New(cmdutil.Diag(), localURL)
	if err != nil {
		panic(fmt.Sprintf("error creating new local backend to test: %+v", err))
	}

	exCode := m.Run()
	os.Exit(exCode)
}
func TestParseStackReference(t *testing.T) {
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

func TestCreateStack(t *testing.T) {
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

func TestGetStack(t *testing.T) {
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

func TestListStacks(t *testing.T) {
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

func TestRemoveStack(t *testing.T) {
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

func TestUpdate(t *testing.T) {
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
		testDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("error getting testdata working directory: %+v", err)
		}
		installNPMPackages(testDir, t)

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
		up1 := applyUpdate(ctx, testDir, proj, stackRef, t)
		if !up1.HasChanges() {
			t.Fatalf("expected changes on first update")
		}
		up2 := applyUpdate(ctx, testDir, proj, stackRef, t)
		if up2.HasChanges() {
			t.Fatalf("expected no changes between first and second update")
		}

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
	tmp, err := ioutil.TempDir("", t.Name())
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

func tearDownPulumi() {
	_ = os.RemoveAll(localBackendPath)
}

func tearDownProject(stackName string) {
	_ = os.RemoveAll(fmt.Sprintf("Pulumi.%s.yaml", stackName))
	_ = os.RemoveAll("node_modules")
}

func applyUpdate(ctx context.Context, root string, project *workspace.Project,
	stackRef backend.StackReference, t *testing.T) engine.ResourceChanges {
	m := backend.UpdateMetadata{
		Message:     "",
		Environment: make(map[string]string),
	}

	engineOpts := engine.UpdateOptions{
		Analyzers: []string{},
		Parallel:  0,
		Debug:     false,
		Refresh:   false,
	}

	res, err := be.Update(ctx, stackRef, backend.UpdateOperation{
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
	})
	if err != nil {
		t.Fatalf("error updating resources: %+v", err)
	}
	return res
}

// installNPMPackages runs npm install in the desired directory.
// Expect this to take some time.
func installNPMPackages(dir string, t *testing.T) {
	if err := exec.Command("npm", "install", dir).Run(); err != nil {
		t.Fatalf("failed to execute `npm install` in %s: %+v", dir, err)
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
	expectStackPath := fmt.Sprintf("%s/stacks/%s.json", localBackendPath, stackRef.Name().String())
	if _, err := os.Stat(expectStackPath); os.IsNotExist(err) {
		t.Fatalf("expected file %s to have been created on disk", expectStackPath)
	}
}

func stackDoesNotExist(stackRef backend.StackReference, t *testing.T) {
	expectStackPath := fmt.Sprintf("%s/stacks/%s.json", localBackendPath, stackRef.Name().String())
	if _, err := os.Stat(expectStackPath); !os.IsNotExist(err) {
		t.Fatalf("expected file %s to have been removed from disk", expectStackPath)
	}
}

func removeStack(ctx context.Context, stackRef backend.StackReference, t *testing.T) {
	_, err := be.RemoveStack(ctx, stackRef, false)
	if err != nil {
		t.Fatalf("error removing stack %s: %+v", stackRef.Name().String(), err)
	}
}
