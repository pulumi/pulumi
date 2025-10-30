package framework

import framework "github.com/pulumi/pulumi/sdk/v3/pkg/engine/lifecycletest/framework"

// TB is a subset of testing.TB that admits other T-like things, such as *rapid.T from the Rapid property-testing
// library. It covers the set of functionality that we actually need for lifecycle testing, and satisfies the interfaces
// of testify.assert and testify.require.
type TB = framework.TB

// The NopPluginManager is used by the test framework to avoid any interactions with ambient plugins.
type NopPluginManager = framework.NopPluginManager

type TestOp = framework.TestOp

type ValidateFunc = framework.ValidateFunc

type TestStep = framework.TestStep

// TestUpdateOptions is UpdateOptions for a TestPlan.
type TestUpdateOptions = framework.TestUpdateOptions

type TestPlan = framework.TestPlan

type TestBuilder = framework.TestBuilder

type Result = framework.Result

func NewUpdateInfo(project workspace.Project, target deploy.Target) engine.UpdateInfo {
	return framework.NewUpdateInfo(project, target)
}

func ImportOp(imports []deploy.Import) TestOp {
	return framework.ImportOp(imports)
}

func AssertDisplay(t TB, events []engine.Event, path string) {
	framework.AssertDisplay(t, events, path)
}

// CloneSnapshot makes a deep copy of the given snapshot and returns a pointer to the clone.
func CloneSnapshot(t TB, snap *deploy.Snapshot) *deploy.Snapshot {
	return framework.CloneSnapshot(t, snap)
}

// resCount is the expected number of resources registered during this test.
func MakeBasicLifecycleSteps(t *testing.T, resCount int) []TestStep {
	return framework.MakeBasicLifecycleSteps(t, resCount)
}

func NewTestBuilder(t *testing.T, snap *deploy.Snapshot) *TestBuilder {
	return framework.NewTestBuilder(t, snap)
}

