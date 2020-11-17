// Copyright 2016-2018, Pulumi Corporation.
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

package integration

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	user "github.com/tweekmonster/luser"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v2/backend/filestate"
	"github.com/pulumi/pulumi/pkg/v2/engine"
	"github.com/pulumi/pulumi/pkg/v2/operations"
	"github.com/pulumi/pulumi/pkg/v2/resource/stack"
	"github.com/pulumi/pulumi/sdk/v2/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/config"
	pulumi_testing "github.com/pulumi/pulumi/sdk/v2/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tools"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/ciutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/retry"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
)

const PythonRuntime = "python"
const NodeJSRuntime = "nodejs"
const GoRuntime = "go"
const DotNetRuntime = "dotnet"

const windowsOS = "windows"

// RuntimeValidationStackInfo contains details related to the stack that runtime validation logic may want to use.
type RuntimeValidationStackInfo struct {
	StackName    tokens.QName
	Deployment   *apitype.DeploymentV3
	RootResource apitype.ResourceV3
	Outputs      map[string]interface{}
	Events       []apitype.EngineEvent
}

// EditDir is an optional edit to apply to the example, as subsequent deployments.
type EditDir struct {
	Dir                    string
	ExtraRuntimeValidation func(t *testing.T, stack RuntimeValidationStackInfo)

	// Additive is true if Dir should be copied *on top* of the test directory.
	// Otherwise Dir *replaces* the test directory, except we keep .pulumi/ and Pulumi.yaml and Pulumi.<stack>.yaml.
	Additive bool

	// ExpectFailure is true if we expect this test to fail.  This is very coarse grained, and will essentially
	// tolerate *any* failure in the program (IDEA: in the future, offer a way to narrow this down more).
	ExpectFailure bool

	// ExpectNoChanges is true if the edit is expected to not propose any changes.
	ExpectNoChanges bool

	// Stdout is the writer to use for all stdout messages.
	Stdout io.Writer
	// Stderr is the writer to use for all stderr messages.
	Stderr io.Writer
	// Verbose may be set to true to print messages as they occur, rather than buffering and showing upon failure.
	Verbose bool

	// Run program directory in query mode.
	QueryMode bool
}

// TestCommandStats is a collection of data related to running a single command during a test.
type TestCommandStats struct {
	// StartTime is the time at which the command was started
	StartTime string `json:"startTime"`
	// EndTime is the time at which the command exited
	EndTime string `json:"endTime"`
	// ElapsedSeconds is the time at which the command exited
	ElapsedSeconds float64 `json:"elapsedSeconds"`
	// StackName is the name of the stack
	StackName string `json:"stackName"`
	// TestId is the unique ID of the test run
	TestID string `json:"testId"`
	// StepName is the command line which was invoked
	StepName string `json:"stepName"`
	// CommandLine is the command line which was invoked
	CommandLine string `json:"commandLine"`
	// TestName is the name of the directory in which the test was executed
	TestName string `json:"testName"`
	// IsError is true if the command failed
	IsError bool `json:"isError"`
	// The Cloud that the test was run against, or empty for local deployments
	CloudURL string `json:"cloudURL"`
}

// TestStatsReporter reports results and metadata from a test run.
type TestStatsReporter interface {
	ReportCommand(stats TestCommandStats)
}

// ConfigValue is used to provide config values to a test program.
type ConfigValue struct {
	// The config key to pass to `pulumi config`.
	Key string
	// The config value to pass to `pulumi config`.
	Value string
	// Secret indicates that the `--secret` flag should be specified when calling `pulumi config`.
	Secret bool
	// Path indicates that the `--path` flag should be specified when calling `pulumi config`.
	Path bool
}

// ProgramTestOptions provides options for ProgramTest
type ProgramTestOptions struct {
	// Dir is the program directory to test.
	Dir string
	// Array of NPM packages which must be `yarn linked` (e.g. {"pulumi", "@pulumi/aws"})
	Dependencies []string
	// Map of package names to versions. The test will use the specified versions of these packages instead of what
	// is declared in `package.json`.
	Overrides map[string]string
	// Map of config keys and values to set (e.g. {"aws:region": "us-east-2"}).
	Config map[string]string
	// Map of secure config keys and values to set (e.g. {"aws:region": "us-east-2"}).
	Secrets map[string]string
	// List of config keys and values to set in order, including Secret and Path options.
	OrderedConfig []ConfigValue
	// SecretsProvider is the optional custom secrets provider to use instead of the default.
	SecretsProvider string
	// EditDirs is an optional list of edits to apply to the example, as subsequent deployments.
	EditDirs []EditDir
	// ExtraRuntimeValidation is an optional callback for additional validation, called before applying edits.
	ExtraRuntimeValidation func(t *testing.T, stack RuntimeValidationStackInfo)
	// RelativeWorkDir is an optional path relative to `Dir` which should be used as working directory during tests.
	RelativeWorkDir string
	// AllowEmptyPreviewChanges is true if we expect that this test's no-op preview may propose changes (e.g.
	// because the test is sensitive to the exact contents of its working directory and those contents change
	// incidentally between the initial update and the empty update).
	AllowEmptyPreviewChanges bool
	// AllowEmptyUpdateChanges is true if we expect that this test's no-op update may perform changes (e.g.
	// because the test is sensitive to the exact contents of its working directory and those contents change
	// incidentally between the initial update and the empty update).
	AllowEmptyUpdateChanges bool
	// ExpectFailure is true if we expect this test to fail.  This is very coarse grained, and will essentially
	// tolerate *any* failure in the program (IDEA: in the future, offer a way to narrow this down more).
	ExpectFailure bool
	// ExpectRefreshChanges may be set to true if a test is expected to have changes yielded by an immediate refresh.
	// This could occur, for example, is a resource's state is constantly changing outside of Pulumi (e.g., timestamps).
	ExpectRefreshChanges bool
	// RetryFailedSteps indicates that failed updates, refreshes, and destroys should be retried after a brief
	// intermission. A maximum of 3 retries will be attempted.
	RetryFailedSteps bool
	// SkipRefresh indicates that the refresh step should be skipped entirely.
	SkipRefresh bool
	// SkipPreview indicates that the preview step should be skipped entirely.
	SkipPreview bool
	// SkipUpdate indicates that the update step should be skipped entirely.
	SkipUpdate bool
	// SkipExportImport skips testing that exporting and importing the stack works properly.
	SkipExportImport bool
	// SkipEmptyPreviewUpdate skips the no-change preview/update that is performed that validates
	// that no changes happen.
	SkipEmptyPreviewUpdate bool
	// SkipStackRemoval indicates that the stack should not be removed. (And so the test's results could be inspected
	// in the Pulumi Service after the test has completed.)
	SkipStackRemoval bool
	// Quick implies SkipPreview, SkipExportImport and SkipEmptyPreviewUpdate
	Quick bool
	// PreviewCommandlineFlags specifies flags to add to the `pulumi preview` command line (e.g. "--color=raw")
	PreviewCommandlineFlags []string
	// UpdateCommandlineFlags specifies flags to add to the `pulumi up` command line (e.g. "--color=raw")
	UpdateCommandlineFlags []string
	// QueryCommandlineFlags specifies flags to add to the `pulumi query` command line (e.g. "--color=raw")
	QueryCommandlineFlags []string
	// RunBuild indicates that the build step should be run (e.g. run `yarn build` for `nodejs` programs)
	RunBuild bool
	// RunUpdateTest will ensure that updates to the package version can test for spurious diffs
	RunUpdateTest bool

	// CloudURL is an optional URL to override the default Pulumi Service API (https://api.pulumi-staging.io). The
	// PULUMI_ACCESS_TOKEN environment variable must also be set to a valid access token for the target cloud.
	CloudURL string

	// StackName allows the stack name to be explicitly provided instead of computed from the
	// environment during tests.
	StackName string

	// Tracing specifies the Zipkin endpoint if any to use for tracing Pulumi invocations.
	Tracing string
	// NoParallel will opt the test out of being ran in parallel.
	NoParallel bool

	// PrePulumiCommand specifies a callback that will be executed before each `pulumi` invocation. This callback may
	// optionally return another callback to be invoked after the `pulumi` invocation completes.
	PrePulumiCommand func(verb string) (func(err error) error, error)

	// ReportStats optionally specifies how to report results from the test for external collection.
	ReportStats TestStatsReporter

	// Stdout is the writer to use for all stdout messages.
	Stdout io.Writer
	// Stderr is the writer to use for all stderr messages.
	Stderr io.Writer
	// Verbose may be set to true to print messages as they occur, rather than buffering and showing upon failure.
	Verbose bool

	// DebugLogging may be set to anything >0 to enable excessively verbose debug logging from `pulumi`.  This is
	// equivalent to `--logtostderr -v=N`, where N is the value of DebugLogLevel.  This may also be enabled by setting
	// the environment variable PULUMI_TEST_DEBUG_LOG_LEVEL.
	DebugLogLevel int
	// DebugUpdates may be set to true to enable debug logging from `pulumi preview`, `pulumi up`, and
	// `pulumi destroy`.  This may also be enabled by setting the environment variable PULUMI_TEST_DEBUG_UPDATES.
	DebugUpdates bool

	// Bin is a location of a `pulumi` executable to be run.  Taken from the $PATH if missing.
	Bin string
	// YarnBin is a location of a `yarn` executable to be run.  Taken from the $PATH if missing.
	YarnBin string
	// GoBin is a location of a `go` executable to be run.  Taken from the $PATH if missing.
	GoBin string
	// PythonBin is a location of a `python` executable to be run.  Taken from the $PATH if missing.
	PythonBin string
	// PipenvBin is a location of a `pipenv` executable to run.  Taken from the $PATH if missing.
	PipenvBin string
	// DotNetBin is a location of a `dotnet` executable to be run.  Taken from the $PATH if missing.
	DotNetBin string

	// Additional environment variables to pass for each command we run.
	Env []string

	// Automatically create and use a virtual environment, rather than using the Pipenv tool.
	UseAutomaticVirtualEnv bool
}

func (opts *ProgramTestOptions) GetDebugLogLevel() int {
	if opts.DebugLogLevel > 0 {
		return opts.DebugLogLevel
	}
	if du := os.Getenv("PULUMI_TEST_DEBUG_LOG_LEVEL"); du != "" {
		if n, e := strconv.Atoi(du); e != nil {
			panic(e)
		} else if n > 0 {
			return n
		}
	}
	return 0
}

func (opts *ProgramTestOptions) GetDebugUpdates() bool {
	return opts.DebugUpdates || os.Getenv("PULUMI_TEST_DEBUG_UPDATES") != ""
}

// GetStackName returns a stack name to use for this test.
func (opts *ProgramTestOptions) GetStackName() tokens.QName {
	if opts.StackName == "" {
		// Fetch the host and test dir names, cleaned so to contain just [a-zA-Z0-9-_] chars.
		hostname, err := os.Hostname()
		contract.AssertNoErrorf(err, "failure to fetch hostname for stack prefix")
		var host string
		for _, c := range hostname {
			if len(host) >= 10 {
				break
			}
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
				(c >= '0' && c <= '9') || c == '-' || c == '_' {
				host += string(c)
			}
		}

		var test string
		for _, c := range filepath.Base(opts.Dir) {
			if len(test) >= 10 {
				break
			}
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
				(c >= '0' && c <= '9') || c == '-' || c == '_' {
				test += string(c)
			}
		}

		b := make([]byte, 4)
		_, err = cryptorand.Read(b)
		contract.AssertNoError(err)

		opts.StackName = strings.ToLower("p-it-" + host + "-" + test + "-" + hex.EncodeToString(b))
	}

	return tokens.QName(opts.StackName)
}

// GetStackNameWithOwner gets the name of the stack prepended with an owner, if PULUMI_TEST_OWNER is set.
// We use this in CI to create test stacks in an organization that all developers have access to, for debugging.
func (opts *ProgramTestOptions) GetStackNameWithOwner() tokens.QName {
	if owner := os.Getenv("PULUMI_TEST_OWNER"); owner != "" {
		return tokens.QName(fmt.Sprintf("%s/%s", owner, opts.GetStackName()))
	}

	return opts.GetStackName()
}

// With combines a source set of options with a set of overrides.
func (opts ProgramTestOptions) With(overrides ProgramTestOptions) ProgramTestOptions {
	if overrides.Dir != "" {
		opts.Dir = overrides.Dir
	}
	if overrides.Dependencies != nil {
		opts.Dependencies = overrides.Dependencies
	}
	if overrides.Overrides != nil {
		opts.Overrides = overrides.Overrides
	}
	for k, v := range overrides.Config {
		if opts.Config == nil {
			opts.Config = make(map[string]string)
		}
		opts.Config[k] = v
	}
	for k, v := range overrides.Secrets {
		if opts.Secrets == nil {
			opts.Secrets = make(map[string]string)
		}
		opts.Secrets[k] = v
	}
	if overrides.SecretsProvider != "" {
		opts.SecretsProvider = overrides.SecretsProvider
	}
	if overrides.EditDirs != nil {
		opts.EditDirs = overrides.EditDirs
	}
	if overrides.ExtraRuntimeValidation != nil {
		opts.ExtraRuntimeValidation = overrides.ExtraRuntimeValidation
	}
	if overrides.RelativeWorkDir != "" {
		opts.RelativeWorkDir = overrides.RelativeWorkDir
	}
	if overrides.AllowEmptyPreviewChanges {
		opts.AllowEmptyPreviewChanges = overrides.AllowEmptyPreviewChanges
	}
	if overrides.AllowEmptyUpdateChanges {
		opts.AllowEmptyUpdateChanges = overrides.AllowEmptyUpdateChanges
	}
	if overrides.ExpectFailure {
		opts.ExpectFailure = overrides.ExpectFailure
	}
	if overrides.ExpectRefreshChanges {
		opts.ExpectRefreshChanges = overrides.ExpectRefreshChanges
	}
	if overrides.RetryFailedSteps {
		opts.RetryFailedSteps = overrides.RetryFailedSteps
	}
	if overrides.SkipRefresh {
		opts.SkipRefresh = overrides.SkipRefresh
	}
	if overrides.SkipPreview {
		opts.SkipPreview = overrides.SkipPreview
	}
	if overrides.SkipUpdate {
		opts.SkipUpdate = overrides.SkipUpdate
	}
	if overrides.SkipExportImport {
		opts.SkipExportImport = overrides.SkipExportImport
	}
	if overrides.SkipEmptyPreviewUpdate {
		opts.SkipEmptyPreviewUpdate = overrides.SkipEmptyPreviewUpdate
	}
	if overrides.SkipStackRemoval {
		opts.SkipStackRemoval = overrides.SkipStackRemoval
	}
	if overrides.Quick {
		opts.Quick = overrides.Quick
	}
	if overrides.PreviewCommandlineFlags != nil {
		opts.PreviewCommandlineFlags = append(opts.PreviewCommandlineFlags, overrides.PreviewCommandlineFlags...)
	}
	if overrides.UpdateCommandlineFlags != nil {
		opts.UpdateCommandlineFlags = append(opts.UpdateCommandlineFlags, overrides.UpdateCommandlineFlags...)
	}
	if overrides.QueryCommandlineFlags != nil {
		opts.QueryCommandlineFlags = append(opts.QueryCommandlineFlags, overrides.QueryCommandlineFlags...)
	}
	if overrides.RunBuild {
		opts.RunBuild = overrides.RunBuild
	}
	if overrides.RunUpdateTest {
		opts.RunUpdateTest = overrides.RunUpdateTest
	}
	if overrides.CloudURL != "" {
		opts.CloudURL = overrides.CloudURL
	}
	if overrides.StackName != "" {
		opts.StackName = overrides.StackName
	}
	if overrides.Tracing != "" {
		opts.Tracing = overrides.Tracing
	}
	if overrides.NoParallel {
		opts.NoParallel = overrides.NoParallel
	}
	if overrides.PrePulumiCommand != nil {
		opts.PrePulumiCommand = overrides.PrePulumiCommand
	}
	if overrides.ReportStats != nil {
		opts.ReportStats = overrides.ReportStats
	}
	if overrides.Stdout != nil {
		opts.Stdout = overrides.Stdout
	}
	if overrides.Stderr != nil {
		opts.Stderr = overrides.Stderr
	}
	if overrides.Verbose {
		opts.Verbose = overrides.Verbose
	}
	if overrides.DebugLogLevel != 0 {
		opts.DebugLogLevel = overrides.DebugLogLevel
	}
	if overrides.DebugUpdates {
		opts.DebugUpdates = overrides.DebugUpdates
	}
	if overrides.Bin != "" {
		opts.Bin = overrides.Bin
	}
	if overrides.YarnBin != "" {
		opts.YarnBin = overrides.YarnBin
	}
	if overrides.GoBin != "" {
		opts.GoBin = overrides.GoBin
	}
	if overrides.PipenvBin != "" {
		opts.PipenvBin = overrides.PipenvBin
	}
	if overrides.Env != nil {
		opts.Env = append(opts.Env, overrides.Env...)
	}
	return opts
}

type regexFlag struct {
	re *regexp.Regexp
}

func (rf *regexFlag) String() string {
	if rf.re == nil {
		return ""
	}
	return rf.re.String()
}

func (rf *regexFlag) Set(v string) error {
	r, err := regexp.Compile(v)
	if err != nil {
		return err
	}
	rf.re = r
	return nil
}

var directoryMatcher regexFlag
var listDirs bool
var pipMutex *fsutil.FileMutex

func init() {
	flag.Var(&directoryMatcher, "dirs", "optional list of regexes to use to select integration tests to run")
	flag.BoolVar(&listDirs, "list-dirs", false, "list available integration tests without running them")

	mutexPath := filepath.Join(os.TempDir(), "pip-mutex.lock")
	pipMutex = fsutil.NewFileMutex(mutexPath)
}

// GetLogs retrieves the logs for a given stack in a particular region making the query provided.
//
// [provider] should be one of "aws" or "azure"
func GetLogs(
	t *testing.T,
	provider, region string,
	stackInfo RuntimeValidationStackInfo,
	query operations.LogQuery) *[]operations.LogEntry {

	snap, err := stack.DeserializeDeploymentV3(*stackInfo.Deployment, stack.DefaultSecretsProvider)
	assert.NoError(t, err)

	tree := operations.NewResourceTree(snap.Resources)
	if !assert.NotNil(t, tree) {
		return nil
	}

	cfg := map[config.Key]string{
		config.MustMakeKey(provider, "region"): region,
	}
	ops := tree.OperationsProvider(cfg)

	// Validate logs from example
	logs, err := ops.GetLogs(query)
	if !assert.NoError(t, err) {
		return nil
	}

	return logs
}

func prepareProgram(t *testing.T, opts *ProgramTestOptions) {
	// If we're just listing tests, simply print this test's directory.
	if listDirs {
		fmt.Printf("%s\n", opts.Dir)
	}

	// If we have a matcher, ensure that this test matches its pattern.
	if directoryMatcher.re != nil && !directoryMatcher.re.Match([]byte(opts.Dir)) {
		t.Skip(fmt.Sprintf("Skipping: '%v' does not match '%v'", opts.Dir, directoryMatcher.re))
	}

	// Disable stack backups for tests to avoid filling up ~/.pulumi/backups with unnecessary
	// backups of test stacks.
	if err := os.Setenv(filestate.DisableCheckpointBackupsEnvVar, "1"); err != nil {
		t.Errorf("error setting env var '%s': %v", filestate.DisableCheckpointBackupsEnvVar, err)
	}

	// We want tests to default into being ran in parallel, hence the odd double negative.
	if !opts.NoParallel {
		t.Parallel()
	}

	if ciutil.IsCI() && os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skip("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}

	// If the test panics, recover and log instead of letting the panic escape the test. Even though *this* test will
	// have run deferred functions and cleaned up, if the panic reaches toplevel it will kill the process and prevent
	// other tests running in parallel from cleaning up.
	defer func() {
		if failure := recover(); failure != nil {
			t.Errorf("panic testing %v: %v", opts.Dir, failure)
		}
	}()

	// Set up some default values for sending test reports and tracing data. We use environment varaiables to
	// control these globally and set reasonable values for our own use in CI.
	if opts.ReportStats == nil {
		if v := os.Getenv("PULUMI_TEST_REPORT_CONFIG"); v != "" {
			splits := strings.Split(v, ":")
			if len(splits) != 3 {
				t.Errorf("report config should be set to a value of the form: <aws-region>:<bucket-name>:<keyPrefix>")
			}

			opts.ReportStats = NewS3Reporter(splits[0], splits[1], splits[2])
		}
	}

	if opts.Tracing == "" {
		opts.Tracing = os.Getenv("PULUMI_TEST_TRACE_ENDPOINT")
	}
}

// ProgramTest runs a lifecycle of Pulumi commands in a program working directory, using the `pulumi` and `yarn`
// binaries available on PATH.  It essentially executes the following workflow:
//
//   yarn install
//   yarn link <each opts.Depencies>
//   (+) yarn run build
//   pulumi init
//   (*) pulumi login
//   pulumi stack init integrationtesting
//   pulumi config set <each opts.Config>
//   pulumi config set --secret <each opts.Secrets>
//   pulumi preview
//   pulumi up
//   pulumi stack export --file stack.json
//   pulumi stack import --file stack.json
//   pulumi preview (expected to be empty)
//   pulumi up (expected to be empty)
//   pulumi destroy --yes
//   pulumi stack rm --yes integrationtesting
//
//   (*) Only if PULUMI_ACCESS_TOKEN is set.
//   (+) Only if `opts.RunBuild` is true.
//
// All commands must return success return codes for the test to succeed, unless ExpectFailure is true.
func ProgramTest(t *testing.T, opts *ProgramTestOptions) {
	prepareProgram(t, opts)
	pt := newProgramTester(t, opts)
	err := pt.TestLifeCycleInitAndDestroy()
	assert.NoError(t, err)
}

// ProgramTestManualLifeCycle returns a ProgramTester than must be manually controlled in terms of its lifecycle
func ProgramTestManualLifeCycle(t *testing.T, opts *ProgramTestOptions) *ProgramTester {
	prepareProgram(t, opts)
	pt := newProgramTester(t, opts)
	return pt
}

// ProgramTester contains state associated with running a single test pass.
type ProgramTester struct {
	t            *testing.T          // the Go tester for this run.
	opts         *ProgramTestOptions // options that control this test run.
	bin          string              // the `pulumi` binary we are using.
	yarnBin      string              // the `yarn` binary we are using.
	goBin        string              // the `go` binary we are using.
	pythonBin    string              // the `python` binary we are using.
	pipenvBin    string              // The `pipenv` binary we are using.
	dotNetBin    string              // the `dotnet` binary we are using.
	eventLog     string              // The path to the event log for this test.
	maxStepTries int                 // The maximum number of times to retry a failed pulumi step.
	tmpdir       string              // the temporary directory we use for our test environment
	projdir      string              // the project directory we use for this run
	TestFinished bool                // whether or not the test if finished
}

func newProgramTester(t *testing.T, opts *ProgramTestOptions) *ProgramTester {
	stackName := opts.GetStackName()
	maxStepTries := 1
	if opts.RetryFailedSteps {
		maxStepTries = 3
	}
	if opts.Quick {
		opts.SkipPreview = true
		opts.SkipExportImport = true
		opts.SkipEmptyPreviewUpdate = true
	}
	return &ProgramTester{
		t:            t,
		opts:         opts,
		eventLog:     filepath.Join(os.TempDir(), string(stackName)+"-events.json"),
		maxStepTries: maxStepTries,
	}
}

func (pt *ProgramTester) getBin() (string, error) {
	return getCmdBin(&pt.bin, "pulumi", pt.opts.Bin)
}

func (pt *ProgramTester) getYarnBin() (string, error) {
	return getCmdBin(&pt.yarnBin, "yarn", pt.opts.YarnBin)
}

func (pt *ProgramTester) getGoBin() (string, error) {
	return getCmdBin(&pt.goBin, "go", pt.opts.GoBin)
}

// getPythonBin returns a path to the currently-installed `python` binary, or an error if it could not be found.
func (pt *ProgramTester) getPythonBin() (string, error) {
	if pt.pythonBin == "" {
		pt.pythonBin = pt.opts.PythonBin
		if pt.opts.PythonBin == "" {
			var err error
			// Look for "python3" by default, but fallback to `python` if not found as some Python 3
			// distributions (in particular the default python.org Windows installation) do not include
			// a `python3` binary.
			pythonCmds := []string{"python3", "python"}
			for _, bin := range pythonCmds {
				pt.pythonBin, err = exec.LookPath(bin)
				// Break on the first cmd we find on the path (if any).
				if err == nil {
					break
				}
			}
			if err != nil {
				return "", errors.Wrapf(err, "Expected to find one of %q on $PATH", pythonCmds)
			}
		}
	}
	return pt.pythonBin, nil
}

// getPipenvBin returns a path to the currently-installed Pipenv tool, or an error if the tool could not be found.
func (pt *ProgramTester) getPipenvBin() (string, error) {
	return getCmdBin(&pt.pipenvBin, "pipenv", pt.opts.PipenvBin)
}

func (pt *ProgramTester) getDotNetBin() (string, error) {
	return getCmdBin(&pt.dotNetBin, "dotnet", pt.opts.DotNetBin)
}

func (pt *ProgramTester) pulumiCmd(args []string) ([]string, error) {
	bin, err := pt.getBin()
	if err != nil {
		return nil, err
	}
	cmd := []string{bin}
	if du := pt.opts.GetDebugLogLevel(); du > 0 {
		cmd = append(cmd, "--logtostderr", "-v="+strconv.Itoa(du))
	}
	cmd = append(cmd, args...)
	if tracing := pt.opts.Tracing; tracing != "" {
		cmd = append(cmd, "--tracing", tracing)
	}
	return cmd, nil
}

func (pt *ProgramTester) yarnCmd(args []string) ([]string, error) {
	bin, err := pt.getYarnBin()
	if err != nil {
		return nil, err
	}
	result := []string{bin}
	result = append(result, args...)
	return withOptionalYarnFlags(result), nil
}

func (pt *ProgramTester) pythonCmd(args []string) ([]string, error) {
	bin, err := pt.getPythonBin()
	if err != nil {
		return nil, err
	}

	cmd := []string{bin}
	return append(cmd, args...), nil
}

func (pt *ProgramTester) pipenvCmd(args []string) ([]string, error) {
	bin, err := pt.getPipenvBin()
	if err != nil {
		return nil, err
	}

	cmd := []string{bin}
	return append(cmd, args...), nil
}

func (pt *ProgramTester) runCommand(name string, args []string, wd string) error {
	return RunCommand(pt.t, name, args, wd, pt.opts)
}

func (pt *ProgramTester) runPulumiCommand(name string, args []string, wd string, expectFailure bool) error {
	cmd, err := pt.pulumiCmd(args)
	if err != nil {
		return err
	}

	var postFn func(error) error
	if pt.opts.PrePulumiCommand != nil {
		postFn, err = pt.opts.PrePulumiCommand(args[0])
		if err != nil {
			return err
		}
	}

	isUpdate := args[0] == "preview" || args[0] == "up" || args[0] == "destroy" || args[0] == "refresh"

	// If we're doing a preview or an update and this project is a Python project, we need to run
	// the command in the context of the virtual environment that Pipenv created in order to pick up
	// the correct version of Python.  We also need to do this for destroy and refresh so that
	// dynamic providers are run in the right virtual environment.
	// This is only necessary when not using automatic virtual environment support.
	if !pt.opts.UseAutomaticVirtualEnv && isUpdate {
		projinfo, err := pt.getProjinfo(wd)
		if err != nil {
			return nil
		}

		if projinfo.Proj.Runtime.Name() == "python" {
			pipenvBin, err := pt.getPipenvBin()
			if err != nil {
				return err
			}

			// "pipenv run" activates the current virtual environment and runs the remainder of the arguments as if it
			// were a command.
			cmd = append([]string{pipenvBin, "run"}, cmd...)
		}
	}

	_, _, err = retry.Until(context.Background(), retry.Acceptor{
		Accept: func(try int, nextRetryTime time.Duration) (bool, interface{}, error) {
			runerr := pt.runCommand(name, cmd, wd)
			if runerr == nil {
				return true, nil, nil
			} else if _, ok := runerr.(*exec.ExitError); ok && isUpdate && !expectFailure {
				// the update command failed, let's try again, assuming we haven't failed a few times.
				if try+1 >= pt.maxStepTries {
					return false, nil, errors.Errorf("%v did not succeed after %v tries", cmd, try+1)
				}

				pt.t.Logf("%v failed: %v; retrying...", cmd, runerr)
				return false, nil, nil
			}

			// someother error, fail
			return false, nil, runerr
		},
	})
	if postFn != nil {
		if postErr := postFn(err); postErr != nil {
			return multierror.Append(err, postErr)
		}
	}
	return err
}

func (pt *ProgramTester) runYarnCommand(name string, args []string, wd string) error {
	cmd, err := pt.yarnCmd(args)
	if err != nil {
		return err
	}

	_, _, err = retry.Until(context.Background(), retry.Acceptor{
		Accept: func(try int, nextRetryTime time.Duration) (bool, interface{}, error) {
			runerr := pt.runCommand(name, cmd, wd)
			if runerr == nil {
				return true, nil, nil
			} else if _, ok := runerr.(*exec.ExitError); ok {
				// yarn failed, let's try again, assuming we haven't failed a few times.
				if try+1 >= 3 {
					return false, nil, errors.Errorf("%v did not complete after %v tries", cmd, try+1)
				}

				return false, nil, nil
			}

			// someother error, fail
			return false, nil, runerr
		},
	})
	return err
}

func (pt *ProgramTester) runPythonCommand(name string, args []string, wd string) error {
	cmd, err := pt.pythonCmd(args)
	if err != nil {
		return err
	}

	return pt.runCommand(name, cmd, wd)
}

func (pt *ProgramTester) runVirtualEnvCommand(name string, args []string, wd string) error {
	// When installing with `pip install -e`, a PKG-INFO file is created. If two packages are being installed
	// this way simultaneously (which happens often, when running tests), both installations will be writing the
	// same file simultaneously. If one process catches "PKG-INFO" in a half-written state, the one process that
	// observed the torn write will fail to install the package.
	//
	// To avoid this problem, we use pipMutex to explicitly serialize installation operations. Doing so avoids
	// the problem of multiple processes stomping on the same files in the source tree. Note that pipMutex is a
	// file mutex, so this strategy works even if the go test runner chooses to split up text execution across
	// multiple processes. (Furthermore, each test gets an instance of ProgramTester and thus the mutex, so we'd
	// need to be sharing the mutex globally in each test process if we weren't using the file system to lock.)
	if name == "virtualenv-pip-install-package" {
		if err := pipMutex.Lock(); err != nil {
			panic(err)
		}

		if pt.opts.Verbose {
			pt.t.Log("acquired pip install lock")
			defer pt.t.Log("released pip install lock")
		}
		defer func() {
			if err := pipMutex.Unlock(); err != nil {
				panic(err)
			}
		}()
	}

	virtualenvBinPath, err := getVirtualenvBinPath(wd, args[0])
	if err != nil {
		return err
	}

	cmd := append([]string{virtualenvBinPath}, args[1:]...)
	return pt.runCommand(name, cmd, wd)
}

func (pt *ProgramTester) runPipenvCommand(name string, args []string, wd string) error {
	// Pipenv uses setuptools to install and uninstall packages. Setuptools has an installation mode called "develop"
	// that we use to install the package being tested, since it is 1) lightweight and 2) not doing so has its own set
	// of annoying problems.
	//
	// Setuptools develop does three things:
	//   1. It invokes the "egg_info" command in the target package,
	//   2. It creates a special `.egg-link` sentinel file in the current site-packages folder, pointing to the package
	//      being installed's path on disk
	//   3. It updates easy-install.pth in site-packages so that pip understand that this package has been installed.
	//
	// Steps 2 and 3 operate entirely within the context of a virtualenv. The state that they mutate is fully contained
	// within the current virtualenv. However, step 1 operates in the context of the package's source tree. Egg info
	// is responsible for producing a minimal "egg" for a particular package, and its largest responsibility is creating
	// a PKG-INFO file for a package. PKG-INFO contains, among other things, the version of the package being installed.
	//
	// If two packages are being installed in "develop" mode simultaneously (which happens often, when running tests),
	// both installations will run "egg_info" on the source tree and both processes will be writing the same files
	// simultaneously. If one process catches "PKG-INFO" in a half-written state, the one process that observed the
	// torn write will fail to install the package (setuptools crashes).
	//
	// To avoid this problem, we use pipMutex to explicitly serialize installation operations. Doing so avoids the
	// problem of multiple processes stomping on the same files in the source tree. Note that pipMutex is a file
	// mutex, so this strategy works even if the go test runner chooses to split up text execution across multiple
	// processes. (Furthermore, each test gets an instance of ProgramTester and thus the mutex, so we'd need to be
	// sharing the mutex globally in each test process if we weren't using the file system to lock.)
	if name == "pipenv-install-package" {
		if err := pipMutex.Lock(); err != nil {
			panic(err)
		}

		if pt.opts.Verbose {
			pt.t.Log("acquired pip install lock")
			defer pt.t.Log("released pip install lock")
		}
		defer func() {
			if err := pipMutex.Unlock(); err != nil {
				panic(err)
			}
		}()
	}

	cmd, err := pt.pipenvCmd(args)
	if err != nil {
		return err
	}

	return pt.runCommand(name, cmd, wd)
}

// TestLifeCyclePrepare prepares a test by creating a temporary directory
func (pt *ProgramTester) TestLifeCyclePrepare() error {
	tmpdir, projdir, err := pt.copyTestToTemporaryDirectory()
	pt.tmpdir = tmpdir
	pt.projdir = projdir
	return err
}

// TestCleanUp cleans up the temporary directory that a test used
func (pt *ProgramTester) TestCleanUp() {
	testFinished := pt.TestFinished
	if pt.tmpdir != "" {
		if !testFinished || pt.t.Failed() {
			// Test aborted or failed. Maybe copy to "failed tests" directory.
			failedTestsDir := os.Getenv("PULUMI_FAILED_TESTS_DIR")
			if failedTestsDir != "" {
				dest := filepath.Join(failedTestsDir, pt.t.Name()+uniqueSuffix())
				contract.IgnoreError(fsutil.CopyFile(dest, pt.tmpdir, nil))
			}
		} else {
			contract.IgnoreError(os.RemoveAll(pt.tmpdir))
		}
	} else {
		// When tmpdir is empty, we ran "in tree", which means we wrote output
		// to the "command-output" folder in the projdir, and we should clean
		// it up if the test passed
		if testFinished && !pt.t.Failed() {
			contract.IgnoreError(os.RemoveAll(filepath.Join(pt.projdir, commandOutputFolderName)))
		}
	}
}

// TestLifeCycleInitAndDestroy executes the test and cleans up
func (pt *ProgramTester) TestLifeCycleInitAndDestroy() error {
	err := pt.TestLifeCyclePrepare()
	if err != nil {
		return errors.Wrapf(err, "copying test to temp dir %s", pt.tmpdir)
	}

	pt.TestFinished = false
	defer pt.TestCleanUp()

	err = pt.TestLifeCycleInitialize()
	if err != nil {
		return errors.Wrap(err, "initializing test project")
	}

	// Ensure that before we exit, we attempt to destroy and remove the stack.
	defer func() {
		destroyErr := pt.TestLifeCycleDestroy()
		assert.NoError(pt.t, destroyErr)
	}()

	if err = pt.TestPreviewUpdateAndEdits(); err != nil {
		return errors.Wrap(err, "running test preview, update, and edits")
	}

	if pt.opts.RunUpdateTest {
		err = upgradeProjectDeps(pt.projdir, pt)
		if err != nil {
			return errors.Wrap(err, "upgrading project dependencies")
		}

		if err = pt.TestPreviewUpdateAndEdits(); err != nil {
			return errors.Wrap(err, "running test preview, update, and edits")
		}
	}

	pt.TestFinished = true
	return nil
}

func upgradeProjectDeps(projectDir string, pt *ProgramTester) error {
	projInfo, err := pt.getProjinfo(projectDir)
	if err != nil {
		return errors.Wrap(err, "getting project info")
	}

	switch rt := projInfo.Proj.Runtime.Name(); rt {
	case NodeJSRuntime:
		if err = pt.yarnLinkPackageDeps(projectDir); err != nil {
			return err
		}
	case PythonRuntime:
		if err = pt.installPipPackageDeps(projectDir); err != nil {
			return err
		}
	default:
		return errors.Errorf("unrecognized project runtime: %s", rt)
	}

	return nil
}

// TestLifeCycleInitialize initializes the project directory and stack along with any configuration
func (pt *ProgramTester) TestLifeCycleInitialize() error {
	dir := pt.projdir
	stackName := pt.opts.GetStackName()

	// If RelativeWorkDir is specified, apply that relative to the temp folder for use as working directory during tests.
	if pt.opts.RelativeWorkDir != "" {
		dir = filepath.Join(dir, pt.opts.RelativeWorkDir)
	}

	// Set the default target Pulumi API if not overridden in options.
	if pt.opts.CloudURL == "" {
		pulumiAPI := os.Getenv("PULUMI_API")
		if pulumiAPI != "" {
			pt.opts.CloudURL = pulumiAPI
		}
	}

	// Ensure all links are present, the stack is created, and all configs are applied.
	pt.t.Logf("Initializing project (dir %s; stack %s)", dir, stackName)

	// Login as needed.
	stackInitName := string(pt.opts.GetStackNameWithOwner())

	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" && pt.opts.CloudURL == "" {
		fmt.Printf("Using existing logged in user for tests.  Set PULUMI_ACCESS_TOKEN and/or PULUMI_API to override.\n")
	} else {
		// Set PulumiCredentialsPathEnvVar to our CWD, so we use credentials specific to just this
		// test.
		pt.opts.Env = append(pt.opts.Env, fmt.Sprintf("%s=%s", workspace.PulumiCredentialsPathEnvVar, dir))

		loginArgs := []string{"login"}
		loginArgs = addFlagIfNonNil(loginArgs, "--cloud-url", pt.opts.CloudURL)

		// If this is a local OR cloud login, then don't attach the owner to the stack-name.
		if pt.opts.CloudURL != "" {
			stackInitName = string(pt.opts.GetStackName())
		}

		if err := pt.runPulumiCommand("pulumi-login", loginArgs, dir, false); err != nil {
			return err
		}
	}

	// Stack init
	stackInitArgs := []string{"stack", "init", stackInitName}
	if pt.opts.SecretsProvider != "" {
		stackInitArgs = append(stackInitArgs, "--secrets-provider", pt.opts.SecretsProvider)
	}
	if err := pt.runPulumiCommand("pulumi-stack-init", stackInitArgs, dir, false); err != nil {
		return err
	}

	for key, value := range pt.opts.Config {
		if err := pt.runPulumiCommand("pulumi-config",
			[]string{"config", "set", key, value}, dir, false); err != nil {
			return err
		}
	}

	for key, value := range pt.opts.Secrets {
		if err := pt.runPulumiCommand("pulumi-config",
			[]string{"config", "set", "--secret", key, value}, dir, false); err != nil {
			return err
		}
	}

	for _, cv := range pt.opts.OrderedConfig {
		configArgs := []string{"config", "set", cv.Key, cv.Value}
		if cv.Secret {
			configArgs = append(configArgs, "--secret")
		}
		if cv.Path {
			configArgs = append(configArgs, "--path")
		}
		if err := pt.runPulumiCommand("pulumi-config", configArgs, dir, false); err != nil {
			return err
		}
	}

	return nil
}

// TestLifeCycleDestroy destroys a stack and removes it
func (pt *ProgramTester) TestLifeCycleDestroy() error {
	if pt.projdir != "" {
		// Destroy and remove the stack.
		pt.t.Log("Destroying stack")
		destroy := []string{"destroy", "--non-interactive", "--yes", "--skip-preview"}
		if pt.opts.GetDebugUpdates() {
			destroy = append(destroy, "-d")
		}
		if err := pt.runPulumiCommand("pulumi-destroy", destroy, pt.projdir, false); err != nil {
			return err
		}

		if pt.t.Failed() {
			pt.t.Logf("Test failed, retaining stack '%s'", pt.opts.GetStackNameWithOwner())
			return nil
		}

		if !pt.opts.SkipStackRemoval {
			return pt.runPulumiCommand("pulumi-stack-rm", []string{"stack", "rm", "--yes"}, pt.projdir, false)
		}
	}
	return nil
}

// TestPreviewUpdateAndEdits runs the preview, update, and any relevant edits
func (pt *ProgramTester) TestPreviewUpdateAndEdits() error {
	dir := pt.projdir
	// Now preview and update the real changes.
	pt.t.Log("Performing primary preview and update")
	initErr := pt.PreviewAndUpdate(dir, "initial", pt.opts.ExpectFailure, false, false)

	// If the initial preview/update failed, just exit without trying the rest (but make sure to destroy).
	if initErr != nil {
		return initErr
	}

	// Perform an empty preview and update; nothing is expected to happen here.
	if !pt.opts.SkipExportImport {
		pt.t.Log("Roundtripping checkpoint via stack export and stack import")

		if err := pt.exportImport(dir); err != nil {
			return err
		}
	}

	if !pt.opts.SkipEmptyPreviewUpdate {
		msg := ""
		if !pt.opts.AllowEmptyUpdateChanges {
			msg = "(no changes expected)"
		}
		pt.t.Logf("Performing empty preview and update%s", msg)
		if err := pt.PreviewAndUpdate(
			dir, "empty", false, !pt.opts.AllowEmptyPreviewChanges, !pt.opts.AllowEmptyUpdateChanges); err != nil {

			return err
		}
	}

	// Run additional validation provided by the test options, passing in the checkpoint info.
	if err := pt.performExtraRuntimeValidation(pt.opts.ExtraRuntimeValidation, dir); err != nil {
		return err
	}

	if !pt.opts.SkipRefresh {
		// Perform a refresh and ensure it doesn't yield changes.
		refresh := []string{"refresh", "--non-interactive", "--yes", "--skip-preview"}
		if pt.opts.GetDebugUpdates() {
			refresh = append(refresh, "-d")
		}
		if !pt.opts.ExpectRefreshChanges {
			refresh = append(refresh, "--expect-no-changes")
		}
		if err := pt.runPulumiCommand("pulumi-refresh", refresh, dir, false); err != nil {
			return err
		}
	}

	// If there are any edits, apply them and run a preview and update for each one.
	return pt.testEdits(dir)
}

func (pt *ProgramTester) exportImport(dir string) error {
	exportCmd := []string{"stack", "export", "--file", "stack.json"}
	importCmd := []string{"stack", "import", "--file", "stack.json"}

	defer func() {
		contract.IgnoreError(os.Remove(filepath.Join(dir, "stack.json")))
	}()

	if err := pt.runPulumiCommand("pulumi-stack-export", exportCmd, dir, false); err != nil {
		return err
	}

	return pt.runPulumiCommand("pulumi-stack-import", importCmd, dir, false)
}

// PreviewAndUpdate runs pulumi preview followed by pulumi up
func (pt *ProgramTester) PreviewAndUpdate(dir string, name string, shouldFail, expectNopPreview,
	expectNopUpdate bool) error {

	preview := []string{"preview", "--non-interactive"}
	update := []string{"up", "--non-interactive", "--yes", "--skip-preview", "--event-log", pt.eventLog}
	if pt.opts.GetDebugUpdates() {
		preview = append(preview, "-d")
		update = append(update, "-d")
	}
	if expectNopPreview {
		preview = append(preview, "--expect-no-changes")
	}
	if expectNopUpdate {
		update = append(update, "--expect-no-changes")
	}
	if pt.opts.PreviewCommandlineFlags != nil {
		preview = append(preview, pt.opts.PreviewCommandlineFlags...)
	}
	if pt.opts.UpdateCommandlineFlags != nil {
		update = append(update, pt.opts.UpdateCommandlineFlags...)
	}

	// If not in quick mode, run an explicit preview.
	if !pt.opts.SkipPreview {
		if err := pt.runPulumiCommand("pulumi-preview-"+name, preview, dir, shouldFail); err != nil {
			if shouldFail {
				pt.t.Log("Permitting failure (ExpectFailure=true for this preview)")
				return nil
			}
			return err
		}
	}

	// Now run an update.
	if !pt.opts.SkipUpdate {
		if err := pt.runPulumiCommand("pulumi-update-"+name, update, dir, shouldFail); err != nil {
			if shouldFail {
				pt.t.Log("Permitting failure (ExpectFailure=true for this update)")
				return nil
			}
			return err
		}
	}

	// If we expected a failure, but none occurred, return an error.
	if shouldFail {
		return errors.New("expected this step to fail, but it succeeded")
	}

	return nil
}

func (pt *ProgramTester) query(dir string, name string, shouldFail bool) error {

	query := []string{"query", "--non-interactive"}
	if pt.opts.GetDebugUpdates() {
		query = append(query, "-d")
	}
	if pt.opts.QueryCommandlineFlags != nil {
		query = append(query, pt.opts.QueryCommandlineFlags...)
	}

	// Now run a query.
	if err := pt.runPulumiCommand("pulumi-query-"+name, query, dir, shouldFail); err != nil {
		if shouldFail {
			pt.t.Log("Permitting failure (ExpectFailure=true for this update)")
			return nil
		}
		return err
	}

	// If we expected a failure, but none occurred, return an error.
	if shouldFail {
		return errors.New("expected this step to fail, but it succeeded")
	}

	return nil
}

func (pt *ProgramTester) testEdits(dir string) error {
	for i, edit := range pt.opts.EditDirs {
		var err error
		if err = pt.testEdit(dir, i, edit); err != nil {
			return err
		}
	}
	return nil
}

func (pt *ProgramTester) testEdit(dir string, i int, edit EditDir) error {
	pt.t.Logf("Applying edit '%v' and rerunning preview and update", edit.Dir)

	if edit.Additive {
		// Just copy new files into dir
		if err := fsutil.CopyFile(dir, edit.Dir, nil); err != nil {
			return errors.Wrapf(err, "Couldn't copy %v into %v", edit.Dir, dir)
		}
	} else {
		// Create a new temporary directory
		newDir, err := ioutil.TempDir("", pt.opts.StackName+"-")
		if err != nil {
			return errors.Wrapf(err, "Couldn't create new temporary directory")
		}

		// Delete whichever copy of the test is unused when we return
		dirToDelete := newDir
		defer func() {
			contract.IgnoreError(os.RemoveAll(dirToDelete))
		}()

		// Copy everything except Pulumi.yaml, Pulumi.<stack-name>.yaml, and .pulumi from source into new directory
		exclusions := make(map[string]bool)
		projectYaml := workspace.ProjectFile + ".yaml"
		configYaml := workspace.ProjectFile + "." + pt.opts.StackName + ".yaml"
		exclusions[workspace.BookkeepingDir] = true
		exclusions[projectYaml] = true
		exclusions[configYaml] = true

		if err := fsutil.CopyFile(newDir, edit.Dir, exclusions); err != nil {
			return errors.Wrapf(err, "Couldn't copy %v into %v", edit.Dir, newDir)
		}

		// Copy Pulumi.yaml, Pulumi.<stack-name>.yaml, and .pulumi from old directory to new directory
		oldProjectYaml := filepath.Join(dir, projectYaml)
		newProjectYaml := filepath.Join(newDir, projectYaml)

		oldConfigYaml := filepath.Join(dir, configYaml)
		newConfigYaml := filepath.Join(newDir, configYaml)

		oldProjectDir := filepath.Join(dir, workspace.BookkeepingDir)
		newProjectDir := filepath.Join(newDir, workspace.BookkeepingDir)

		if err := fsutil.CopyFile(newProjectYaml, oldProjectYaml, nil); err != nil {
			return errors.Wrap(err, "Couldn't copy Pulumi.yaml")
		}
		if err := fsutil.CopyFile(newConfigYaml, oldConfigYaml, nil); err != nil {
			return errors.Wrapf(err, "Couldn't copy Pulumi.%s.yaml", pt.opts.StackName)
		}
		if err := fsutil.CopyFile(newProjectDir, oldProjectDir, nil); err != nil {
			return errors.Wrap(err, "Couldn't copy .pulumi")
		}

		// Finally, replace our current temp directory with the new one.
		dirOld := dir + ".old"
		if err := os.Rename(dir, dirOld); err != nil {
			return errors.Wrapf(err, "Couldn't rename %v to %v", dir, dirOld)
		}

		// There's a brief window here where the old temp dir name could be taken from us.

		if err := os.Rename(newDir, dir); err != nil {
			return errors.Wrapf(err, "Couldn't rename %v to %v", newDir, dir)
		}

		// Keep dir, delete oldDir
		dirToDelete = dirOld
	}

	err := pt.prepareProjectDir(dir)
	if err != nil {
		return errors.Wrapf(err, "Couldn't prepare project in %v", dir)
	}

	oldStdOut := pt.opts.Stdout
	oldStderr := pt.opts.Stderr
	oldVerbose := pt.opts.Verbose
	if edit.Stdout != nil {
		pt.opts.Stdout = edit.Stdout
	}
	if edit.Stderr != nil {
		pt.opts.Stderr = edit.Stderr
	}
	if edit.Verbose {
		pt.opts.Verbose = true
	}

	defer func() {
		pt.opts.Stdout = oldStdOut
		pt.opts.Stderr = oldStderr
		pt.opts.Verbose = oldVerbose
	}()

	if !edit.QueryMode {
		if err = pt.PreviewAndUpdate(dir, fmt.Sprintf("edit-%d", i),
			edit.ExpectFailure, edit.ExpectNoChanges, edit.ExpectNoChanges); err != nil {
			return err
		}
	} else {
		if err = pt.query(dir, fmt.Sprintf("query-%d", i), edit.ExpectFailure); err != nil {
			return err
		}
	}
	return pt.performExtraRuntimeValidation(edit.ExtraRuntimeValidation, dir)
}

func (pt *ProgramTester) performExtraRuntimeValidation(
	extraRuntimeValidation func(t *testing.T, stack RuntimeValidationStackInfo), dir string) error {

	if extraRuntimeValidation == nil {
		return nil
	}

	stackName := pt.opts.GetStackName()

	// Create a temporary file name for the stack export
	tempDir, err := ioutil.TempDir("", string(stackName))
	if err != nil {
		return err
	}
	fileName := filepath.Join(tempDir, "stack.json")

	// Invoke `pulumi stack export`
	if err = pt.runPulumiCommand("pulumi-export",
		[]string{"stack", "export", "--file", fileName}, dir, false); err != nil {
		return errors.Wrapf(err, "expected to export stack to file: %s", fileName)
	}

	// Open the exported JSON file
	f, err := os.Open(fileName)
	if err != nil {
		return errors.Wrapf(err, "expected to be able to open file with stack exports: %s", fileName)
	}
	defer func() {
		contract.IgnoreClose(f)
		contract.IgnoreError(os.RemoveAll(tempDir))
	}()

	// Unmarshal the Deployment
	var untypedDeployment apitype.UntypedDeployment
	if err = json.NewDecoder(f).Decode(&untypedDeployment); err != nil {
		return err
	}
	var deployment apitype.DeploymentV3
	if err = json.Unmarshal(untypedDeployment.Deployment, &deployment); err != nil {
		return err
	}

	// Get the root resource and outputs from the deployment
	var rootResource apitype.ResourceV3
	var outputs map[string]interface{}
	for _, res := range deployment.Resources {
		if res.Type == resource.RootStackType {
			rootResource = res
			outputs = res.Outputs
		}
	}

	// Read the event log.
	eventsFile, err := os.Open(pt.eventLog)
	if err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "expected to be able to open event log file %s", pt.eventLog)
	}
	defer contract.IgnoreClose(eventsFile)
	decoder, events := json.NewDecoder(eventsFile), []apitype.EngineEvent{}
	for {
		var event apitype.EngineEvent
		if err = decoder.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			return errors.Wrapf(err, "decoding engine event")
		}
		events = append(events, event)
	}

	// Populate stack info object with all of this data to pass to the validation function
	stackInfo := RuntimeValidationStackInfo{
		StackName:    pt.opts.GetStackName(),
		Deployment:   &deployment,
		RootResource: rootResource,
		Outputs:      outputs,
		Events:       events,
	}

	pt.t.Log("Performing extra runtime validation.")
	extraRuntimeValidation(pt.t, stackInfo)
	pt.t.Log("Extra runtime validation complete.")
	return nil
}

// copyTestToTemporaryDirectory creates a temporary directory to run the test in and copies the test to it.
func (pt *ProgramTester) copyTestToTemporaryDirectory() (string, string, error) {
	// Get the source dir and project info.
	sourceDir := pt.opts.Dir
	projinfo, err := pt.getProjinfo(sourceDir)
	if err != nil {
		return "", "", err
	}

	if pt.opts.Stdout == nil {
		pt.opts.Stdout = os.Stdout
	}
	if pt.opts.Stderr == nil {
		pt.opts.Stderr = os.Stderr
	}

	pt.t.Logf("sample: %v", sourceDir)
	bin, err := pt.getBin()
	if err != nil {
		return "", "", err
	}
	pt.t.Logf("pulumi: %v\n", bin)

	stackName := string(pt.opts.GetStackName())

	// For most projects, we will copy to a temporary directory.  For Go projects, however, we must create
	// a folder structure that adheres to GOPATH requirements
	var tmpdir, projdir string
	if projinfo.Proj.Runtime.Name() == "go" {
		targetDir, err := tools.CreateTemporaryGoFolder("stackName")
		if err != nil {
			return "", "", errors.Wrap(err, "Couldn't create temporary directory")
		}
		tmpdir = targetDir
		projdir = targetDir
	} else {
		targetDir, tempErr := ioutil.TempDir("", stackName+"-")
		if tempErr != nil {
			return "", "", errors.Wrap(tempErr, "Couldn't create temporary directory")
		}
		tmpdir = targetDir
		projdir = targetDir
	}
	// Copy the source project.
	if copyErr := fsutil.CopyFile(tmpdir, sourceDir, nil); copyErr != nil {
		return "", "", copyErr
	}
	projinfo.Root = projdir

	err = pt.prepareProject(projinfo)
	if err != nil {
		return "", "", errors.Wrapf(err, "Failed to prepare %v", projdir)
	}

	// TODO[pulumi/pulumi#5455]: Dynamic providers fail to load when used from multi-lang components.
	// Until that's been fixed, this environment variable can be set by a test, which results in
	// a package.json being emitted in the project directory and `yarn install && yarn link @pulumi/pulumi`
	// being run.
	// When the underlying issue has been fixed, the use of this environment variable should be removed.
	var yarnLinkPulumi bool
	for _, env := range pt.opts.Env {
		if env == "PULUMI_TEST_YARN_LINK_PULUMI=true" {
			yarnLinkPulumi = true
			break
		}
	}
	if yarnLinkPulumi {
		const packageJSON = `{
			"name": "test",
			"peerDependencies": {
				"@pulumi/pulumi": "latest"
			}
		}`
		if err := ioutil.WriteFile(filepath.Join(projdir, "package.json"), []byte(packageJSON), 0600); err != nil {
			return "", "", err
		}
		if err = pt.runYarnCommand("yarn-install", []string{"install"}, projdir); err != nil {
			return "", "", err
		}
		if err := pt.runYarnCommand("yarn-link", []string{"link", "@pulumi/pulumi"}, projdir); err != nil {
			return "", "", err
		}
	}

	pt.t.Logf("projdir: %v", projdir)
	return tmpdir, projdir, nil
}

func (pt *ProgramTester) getProjinfo(projectDir string) (*engine.Projinfo, error) {
	// Load up the package so we know things like what language the project is.
	projfile := filepath.Join(projectDir, workspace.ProjectFile+".yaml")
	proj, err := workspace.LoadProject(projfile)
	if err != nil {
		return nil, err
	}
	return &engine.Projinfo{Proj: proj, Root: projectDir}, nil
}

// prepareProject runs setup necessary to get the project ready for `pulumi` commands.
func (pt *ProgramTester) prepareProject(projinfo *engine.Projinfo) error {
	// Based on the language, invoke the right routine to prepare the target directory.
	switch rt := projinfo.Proj.Runtime.Name(); rt {
	case NodeJSRuntime:
		return pt.prepareNodeJSProject(projinfo)
	case PythonRuntime:
		return pt.preparePythonProject(projinfo)
	case GoRuntime:
		return pt.prepareGoProject(projinfo)
	case DotNetRuntime:
		return pt.prepareDotNetProject(projinfo)
	default:
		return errors.Errorf("unrecognized project runtime: %s", rt)
	}
}

// prepareProjectDir runs setup necessary to get the project ready for `pulumi` commands.
func (pt *ProgramTester) prepareProjectDir(projectDir string) error {
	projinfo, err := pt.getProjinfo(projectDir)
	if err != nil {
		return err
	}
	return pt.prepareProject(projinfo)
}

// prepareNodeJSProject runs setup necessary to get a Node.js project ready for `pulumi` commands.
func (pt *ProgramTester) prepareNodeJSProject(projinfo *engine.Projinfo) error {
	if err := pulumi_testing.WriteYarnRCForTest(projinfo.Root); err != nil {
		return err
	}

	// Get the correct pwd to run Yarn in.
	cwd, _, err := projinfo.GetPwdMain()
	if err != nil {
		return err
	}

	// If the test requested some packages to be overridden, we do two things. First, if the package is listed as a
	// direct dependency of the project, we change the version constraint in the package.json. For transitive
	// dependeices, we use yarn's "resolutions" feature to force them to a specific version.
	if len(pt.opts.Overrides) > 0 {
		packageJSON, err := readPackageJSON(cwd)
		if err != nil {
			return err
		}

		resolutions := make(map[string]interface{})

		for packageName, packageVersion := range pt.opts.Overrides {
			for _, section := range []string{"dependencies", "devDependencies"} {
				if _, has := packageJSON[section]; has {
					entry := packageJSON[section].(map[string]interface{})

					if _, has := entry[packageName]; has {
						entry[packageName] = packageVersion
					}

				}
			}

			pt.t.Logf("adding resolution for %s to version %s", packageName, packageVersion)
			resolutions["**/"+packageName] = packageVersion
		}

		// Wack any existing resolutions section with our newly computed one.
		packageJSON["resolutions"] = resolutions

		if err := writePackageJSON(cwd, packageJSON); err != nil {
			return err
		}
	}

	// Now ensure dependencies are present.
	if err = pt.runYarnCommand("yarn-install", []string{"install"}, cwd); err != nil {
		return err
	}

	if !pt.opts.RunUpdateTest {
		if err = pt.yarnLinkPackageDeps(cwd); err != nil {
			return err
		}
	}

	if pt.opts.RunBuild {
		// And finally compile it using whatever build steps are in the package.json file.
		if err = pt.runYarnCommand("yarn-build", []string{"run", "build"}, cwd); err != nil {
			return err
		}
	}

	return nil

}

// readPackageJSON unmarshals the package.json file located in pathToPackage.
func readPackageJSON(pathToPackage string) (map[string]interface{}, error) {
	f, err := os.Open(filepath.Join(pathToPackage, "package.json"))
	if err != nil {
		return nil, errors.Wrap(err, "opening package.json")
	}
	defer contract.IgnoreClose(f)

	var ret map[string]interface{}
	if err := json.NewDecoder(f).Decode(&ret); err != nil {
		return nil, errors.Wrap(err, "decoding package.json")
	}

	return ret, nil
}

func writePackageJSON(pathToPackage string, metadata map[string]interface{}) error {
	// os.Create truncates the already existing file.
	f, err := os.Create(filepath.Join(pathToPackage, "package.json"))
	if err != nil {
		return errors.Wrap(err, "opening package.json")
	}
	defer contract.IgnoreClose(f)

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")

	return errors.Wrap(encoder.Encode(metadata), "writing package.json")
}

// preparePythonProject runs setup necessary to get a Python project ready for `pulumi` commands.
func (pt *ProgramTester) preparePythonProject(projinfo *engine.Projinfo) error {
	cwd, _, err := projinfo.GetPwdMain()
	if err != nil {
		return err
	}

	if pt.opts.UseAutomaticVirtualEnv {
		if err = pt.runPythonCommand("python-venv", []string{"-m", "venv", "venv"}, cwd); err != nil {
			return err
		}

		projinfo.Proj.Runtime.SetOption("virtualenv", "venv")
		projfile := filepath.Join(projinfo.Root, workspace.ProjectFile+".yaml")
		if err = projinfo.Proj.Save(projfile); err != nil {
			return errors.Wrap(err, "saving project")
		}

		if err := pt.runVirtualEnvCommand("virtualenv-pip-install",
			[]string{"pip", "install", "-r", "requirements.txt"}, cwd); err != nil {
			return err
		}
	} else {
		if err = pt.preparePythonProjectWithPipenv(cwd); err != nil {
			return err
		}
	}

	if !pt.opts.RunUpdateTest {
		if err = pt.installPipPackageDeps(cwd); err != nil {
			return err
		}
	}

	return nil
}

func (pt *ProgramTester) preparePythonProjectWithPipenv(cwd string) error {
	// Create a new Pipenv environment. This bootstraps a new virtual environment containing the version of Python that
	// we requested. Note that this version of Python is sourced from the machine, so you must first install the version
	// of Python that you are requesting on the host machine before building a virtualenv for it.
	pythonVersion := "3"
	if runtime.GOOS == windowsOS {
		// Due to https://bugs.python.org/issue34679, Python Dynamic Providers on Windows do not
		// work on Python 3.8.0 (but are fixed in 3.8.1).  For now we will force Windows to use 3.7
		// to avoid this bug, until 3.8.1 is available in all our CI systems.
		pythonVersion = "3.7"
	}
	if err := pt.runPipenvCommand("pipenv-new", []string{"--python", pythonVersion}, cwd); err != nil {
		return err
	}

	// Install the package's dependencies. We do this by running `pip` inside the virtualenv that `pipenv` has created.
	// We don't use `pipenv install` because we don't want a lock file and prefer the similar model of `pip install`
	// which matches what our customers do
	err := pt.runPipenvCommand("pipenv-install", []string{"run", "pip", "install", "-r", "requirements.txt"}, cwd)
	if err != nil {
		return err
	}
	return nil
}

// YarnLinkPackageDeps bring in package dependencies via yarn
func (pt *ProgramTester) yarnLinkPackageDeps(cwd string) error {
	for _, dependency := range pt.opts.Dependencies {
		if err := pt.runYarnCommand("yarn-link", []string{"link", dependency}, cwd); err != nil {
			return err
		}
	}

	return nil
}

// InstallPipPackageDeps brings in package dependencies via pip install
func (pt *ProgramTester) installPipPackageDeps(cwd string) error {
	var err error
	for _, dep := range pt.opts.Dependencies {
		// If the given filepath isn't absolute, make it absolute. We're about to pass it to pipenv and pipenv is
		// operating inside of a random folder in /tmp.
		if !filepath.IsAbs(dep) {
			dep, err = filepath.Abs(dep)
			if err != nil {
				return err
			}
		}

		if pt.opts.UseAutomaticVirtualEnv {
			if err := pt.runVirtualEnvCommand("virtualenv-pip-install-package",
				[]string{"pip", "install", "-e", dep}, cwd); err != nil {
				return err
			}
		} else {
			if err := pt.runPipenvCommand("pipenv-install-package",
				[]string{"run", "pip", "install", "-e", dep}, cwd); err != nil {
				return err
			}
		}
	}

	return nil
}

func getVirtualenvBinPath(cwd, bin string) (string, error) {
	virtualenvBinPath := filepath.Join(cwd, "venv", "bin", bin)
	if runtime.GOOS == windowsOS {
		virtualenvBinPath = filepath.Join(cwd, "venv", "Scripts", fmt.Sprintf("%s.exe", bin))
	}
	if info, err := os.Stat(virtualenvBinPath); err != nil || info.IsDir() {
		return "", errors.Errorf("Expected %s to exist in virtual environment at %q", bin, virtualenvBinPath)
	}
	return virtualenvBinPath, nil
}

// getSanitizedPkg strips the version string from a go dep
// Note: most of the pulumi modules don't use major version subdirectories for modules
func getSanitizedModulePath(pkg string) string {
	re := regexp.MustCompile(`v\d`)
	v := re.FindString(pkg)
	if v != "" {
		return strings.TrimSuffix(strings.Replace(pkg, v, "", -1), "/")
	}
	return pkg

}

func getRewritePath(pkg string, gopath string, depRoot string) string {

	var depParts []string
	sanitizedPkg := getSanitizedModulePath(pkg)

	splitPkg := strings.Split(sanitizedPkg, "/")

	if depRoot != "" {
		// Get the package name
		// This is the value after "github.com/foo/bar"
		repoName := splitPkg[2]
		basePath := splitPkg[len(splitPkg)-1]
		if basePath == repoName {
			depParts = append([]string{depRoot, repoName})
		} else {
			depParts = append([]string{depRoot, repoName, basePath})
		}
		return filepath.Join(depParts...)
	}
	depParts = append([]string{gopath, "src"}, splitPkg...)
	return filepath.Join(depParts...)

}

// prepareGoProject runs setup necessary to get a Go project ready for `pulumi` commands.
func (pt *ProgramTester) prepareGoProject(projinfo *engine.Projinfo) error {
	// Go programs are compiled, so we will compile the project first.
	goBin, err := pt.getGoBin()
	if err != nil {
		return errors.Wrap(err, "locating `go` binary")
	}

	// Ensure GOPATH is known.
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		usr, userErr := user.Current()
		if userErr != nil {
			return userErr
		}
		gopath = filepath.Join(usr.HomeDir, "go")
	}

	depRoot := os.Getenv("PULUMI_GO_DEP_ROOT")

	cwd, _, err := projinfo.GetPwdMain()
	if err != nil {
		return err
	}

	// initialize a go.mod for dependency resolution if one doesn't exist
	_, err = os.Stat(filepath.Join(cwd, "go.mod"))
	if err != nil {
		err = pt.runCommand("go-mod-init", []string{goBin, "mod", "init"}, cwd)
		if err != nil {
			return err
		}
	}

	// initial tidy to resolve dependencies
	err = pt.runCommand("go-mod-tidy", []string{goBin, "mod", "tidy"}, cwd)
	if err != nil {
		return err
	}

	// link local dependencies
	for _, pkg := range pt.opts.Dependencies {

		dep := getRewritePath(pkg, gopath, depRoot)

		editStr := fmt.Sprintf("%s=%s", pkg, dep)
		err = pt.runCommand("go-mod-edit", []string{goBin, "mod", "edit", "-replace", editStr}, cwd)
		if err != nil {
			return err
		}
	}
	// resolve dependencies
	err = pt.runCommand("go-mod-download", []string{goBin, "mod", "download"}, cwd)
	if err != nil {
		return err
	}

	if pt.opts.RunBuild {
		outBin := filepath.Join(gopath, "bin", string(projinfo.Proj.Name))
		if runtime.GOOS == windowsOS {
			outBin = fmt.Sprintf("%s.exe", outBin)
		}
		err = pt.runCommand("go-build", []string{goBin, "build", "-o", outBin, "."}, cwd)
		if err != nil {
			return err
		}

		_, err = os.Stat(outBin)
		if err != nil {
			return fmt.Errorf("error finding built application artifact: %w", err)
		}
	}

	return nil
}

// prepareDotNetProject runs setup necessary to get a .NET project ready for `pulumi` commands.
func (pt *ProgramTester) prepareDotNetProject(projinfo *engine.Projinfo) error {
	dotNetBin, err := pt.getDotNetBin()
	if err != nil {
		return errors.Wrap(err, "locating `dotnet` binary")
	}

	cwd, _, err := projinfo.GetPwdMain()
	if err != nil {
		return err
	}

	localNuget := os.Getenv("PULUMI_LOCAL_NUGET")
	if localNuget == "" {
		localNuget = "/opt/pulumi/nuget"
	}

	for _, dep := range pt.opts.Dependencies {

		// dotnet add package requires a specific version in case of a pre-release, so we have to look it up.
		matches, err := filepath.Glob(filepath.Join(localNuget, dep+".?.*.nupkg"))
		if err != nil {
			return errors.Wrap(err, "failed to find a local Pulumi NuGet package")
		}
		if len(matches) != 1 {
			return errors.New(fmt.Sprintf("attempting to find a local Pulumi NuGet package yielded %v results", matches))
		}
		file := filepath.Base(matches[0])
		r := strings.NewReplacer(dep+".", "", ".nupkg", "")
		version := r.Replace(file)

		err = pt.runCommand("dotnet-add-package",
			[]string{dotNetBin, "add", "package", dep, "-s", localNuget, "-v", version}, cwd)
		if err != nil {
			return errors.Wrapf(err, "failed to add dependency on %s", dep)
		}
	}

	return nil
}
