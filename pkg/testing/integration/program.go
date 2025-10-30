package integration

import integration "github.com/pulumi/pulumi/sdk/v3/pkg/testing/integration"

// RuntimeValidationStackInfo contains details related to the stack that runtime validation logic may want to use.
type RuntimeValidationStackInfo = integration.RuntimeValidationStackInfo

// EditDir is an optional edit to apply to the example, as subsequent deployments.
type EditDir = integration.EditDir

// TestCommandStats is a collection of data related to running a single command during a test.
type TestCommandStats = integration.TestCommandStats

// TestStatsReporter reports results and metadata from a test run.
type TestStatsReporter = integration.TestStatsReporter

// Environment is used to create environments for use by test programs.
type Environment = integration.Environment

// ConfigValue is used to provide config values to a test program.
type ConfigValue = integration.ConfigValue

// ProgramTestOptions provides options for ProgramTest
type ProgramTestOptions = integration.ProgramTestOptions

type LocalDependency = integration.LocalDependency

// ProgramTester contains state associated with running a single test pass.
type ProgramTester = integration.ProgramTester

// AssertPerfBenchmark implements the integration.TestStatsReporter interface, and reports test
// failures when a scenario exceeds the provided threshold.
type AssertPerfBenchmark = integration.AssertPerfBenchmark

const PythonRuntime = integration.PythonRuntime

const NodeJSRuntime = integration.NodeJSRuntime

const GoRuntime = integration.GoRuntime

const DotNetRuntime = integration.DotNetRuntime

const YAMLRuntime = integration.YAMLRuntime

const JavaRuntime = integration.JavaRuntime

var ErrTestFailed = integration.ErrTestFailed

// GetLogs retrieves the logs for a given stack in a particular region making the query provided.
// 
// [provider] should be one of "aws" or "azure"
func GetLogs(t *testing.T, provider, region string, stackInfo RuntimeValidationStackInfo, query operations.LogQuery) *[]operations.LogEntry {
	return integration.GetLogs(t, provider, region, stackInfo, query)
}

// ProgramTest runs a lifecycle of Pulumi commands in a program working directory, using the `pulumi` and `yarn`
// binaries available on PATH.  It essentially executes the following workflow:
// 
// 	yarn install
// 	yarn link <each opts.Depencies>
// 	(+) yarn run build
// 	pulumi init
// 	(*) pulumi login
// 	pulumi stack init integrationtesting
// 	pulumi config set <each opts.Config>
// 	pulumi config set --secret <each opts.Secrets>
// 	pulumi preview
// 	pulumi up
// 	pulumi stack export --file stack.json
// 	pulumi stack import --file stack.json
// 	pulumi preview (expected to be empty)
// 	pulumi up (expected to be empty)
// 	pulumi destroy --yes
// 	pulumi stack rm --yes integrationtesting
// 
// 	(*) Only if PULUMI_ACCESS_TOKEN is set.
// 	(+) Only if `opts.RunBuild` is true.
// 
// All commands must return success return codes for the test to succeed, unless ExpectFailure is true.
func ProgramTest(t *testing.T, opts *ProgramTestOptions) {
	integration.ProgramTest(t, opts)
}

// ProgramTestManualLifeCycle returns a ProgramTester than must be manually controlled in terms of its lifecycle
func ProgramTestManualLifeCycle(t *testing.T, opts *ProgramTestOptions) *ProgramTester {
	return integration.ProgramTestManualLifeCycle(t, opts)
}

// MakeTempBackend creates a temporary backend directory which will clean up on test exit.
func MakeTempBackend(t *testing.T) string {
	return integration.MakeTempBackend(t)
}

// Fetchs the GOPATH
func GoPath() (string, error) {
	return integration.GoPath()
}

