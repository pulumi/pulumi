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
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend/filestate"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/ciutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/fsutil"
	"github.com/pulumi/pulumi/pkg/util/retry"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// RuntimeValidationStackInfo contains details related to the stack that runtime validation logic may want to use.
type RuntimeValidationStackInfo struct {
	StackName    tokens.QName
	Deployment   *apitype.DeploymentV2
	RootResource apitype.ResourceV2
	Outputs      map[string]interface{}
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

	// Stdout is the writer to use for all stdout messages.
	Stdout io.Writer
	// Stderr is the writer to use for all stderr messages.
	Stderr io.Writer
	// Verbose may be set to true to print messages as they occur, rather than buffering and showing upon failure.
	Verbose bool
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
	// StepName is the command line which was invoked1
	StepName string `json:"stepName"`
	// CommandLine is the command line which was invoked1
	CommandLine string `json:"commandLine"`
	// TestName is the name of the directory in which the test was executed
	TestName string `json:"testName"`
	// IsError is true if the command failed
	IsError bool `json:"isError"`
	// The Cloud that the test was run against, or empty for local deployments
	CloudURL string `json:"cloudURL"`
	// The PPC that the test was run against, or empty for local deployments or for the default PPC
	CloudPPC string `json:"cloudPPC"`
}

// TestStatsReporter reports results and metadata from a test run.
type TestStatsReporter interface {
	ReportCommand(stats TestCommandStats)
}

// ProgramTestOptions provides options for ProgramTest
type ProgramTestOptions struct {
	// Dir is the program directory to test.
	Dir string
	// Array of NPM packages which must be `yarn linked` (e.g. {"pulumi", "@pulumi/aws"})
	Dependencies []string
	// Map of config keys and values to set (e.g. {"aws:config:region": "us-east-2"})
	Config map[string]string
	// Map of secure config keys and values to set on the stack (e.g. {"aws:config:region": "us-east-2"})
	Secrets map[string]string
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
	// SkipRefresh indicates that the refresh step should be skipped entirely.
	SkipRefresh bool
	// Quick can be set to true to run a "quick" test that skips any non-essential steps (e.g., empty updates).
	Quick bool
	// UpdateCommandlineFlags specifies flags to add to the `pulumi update` command line (e.g. "--color=raw")
	UpdateCommandlineFlags []string
	// RunBuild indicates that the build step should be run (e.g. run `yarn build` for `nodejs` programs)
	RunBuild bool

	// CloudURL is an optional URL to override the default Pulumi Service API (https://api.pulumi-staging.io). The
	// PULUMI_ACCESS_TOKEN environment variable must also be set to a valid access token for the target cloud.
	CloudURL string
	// PPCName is the name of the PPC to use when running a test against the hosted service. If
	// not set, the --ppc flag will not be set on `pulumi stack init`.
	PPCName string

	// StackName allows the stack name to be explicitly provided instead of computed from the
	// environment during tests.
	StackName string

	// Tracing specifies the Zipkin endpoint if any to use for tracing Pulumi invocatoions.
	Tracing string

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
	// DebugUpdates may be set to true to enable debug logging from `pulumi preview`, `pulumi update`, and
	// `pulumi destroy`.  This may also be enabled by setting the environment variable PULUMI_TEST_DEBUG_UPDATES.
	DebugUpdates bool

	// Bin is a location of a `pulumi` executable to be run.  Taken from the $PATH if missing.
	Bin string
	// YarnBin is a location of a `yarn` executable to be run.  Taken from the $PATH if missing.
	YarnBin string
	// GoBin is a location of a `go` executable to be run.  Taken from the $PATH if missing.
	GoBin string

	// Additional environment variaibles to pass for each command we run.
	Env []string
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
	if overrides.CloudURL != "" {
		opts.CloudURL = overrides.CloudURL
	}
	if overrides.PPCName != "" {
		opts.PPCName = overrides.PPCName
	}
	if overrides.Tracing != "" {
		opts.Tracing = overrides.Tracing
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
	if overrides.ReportStats != nil {
		opts.ReportStats = overrides.ReportStats
	}
	if overrides.RunBuild {
		opts.RunBuild = overrides.RunBuild
	}
	if overrides.ExpectFailure {
		opts.ExpectFailure = overrides.ExpectFailure
	}
	if overrides.ExpectRefreshChanges {
		opts.ExpectRefreshChanges = overrides.ExpectRefreshChanges
	}
	if overrides.SkipRefresh {
		opts.SkipRefresh = overrides.SkipRefresh
	}
	if overrides.AllowEmptyPreviewChanges {
		opts.AllowEmptyPreviewChanges = overrides.AllowEmptyPreviewChanges
	}
	if overrides.AllowEmptyUpdateChanges {
		opts.AllowEmptyUpdateChanges = overrides.AllowEmptyUpdateChanges
	}
	if overrides.Bin != "" {
		opts.Bin = overrides.Bin
	}
	if overrides.DebugLogLevel != 0 {
		opts.DebugLogLevel = overrides.DebugLogLevel
	}
	if overrides.DebugUpdates {
		opts.DebugUpdates = overrides.DebugUpdates
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

func init() {
	flag.Var(&directoryMatcher, "dirs", "optional list of regexes to use to select integration tests to run")
	flag.BoolVar(&listDirs, "list-dirs", false, "list available integration tests without running them")
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
//   pulumi update
//   pulumi stack export --file stack.json
//   pulumi stack import --file stack.json
//   pulumi preview (expected to be empty)
//   pulumi update (expected to be empty)
//   pulumi destroy --yes
//   pulumi stack rm --yes integrationtesting
//
//   (*) Only if PULUMI_ACCESS_TOKEN is set.
//   (+) Only if `opts.RunBuild` is true.
//
// All commands must return success return codes for the test to succeed, unless ExpectFailure is true.
func ProgramTest(t *testing.T, opts *ProgramTestOptions) {
	// If we're just listing tests, simply print this test's directory.
	if listDirs {
		fmt.Printf("%s\n", opts.Dir)
		return
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

	t.Parallel()

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

	pt := newProgramTester(t, opts)
	err := pt.testLifeCycleInitAndDestroy()
	assert.NoError(t, err)
}

// fprintf works like fmt.FPrintf, except it explicitly drops the return values. This keeps the linters happy, since
// they don't like to see errors dropped on the floor. It is possible that our call to fmt.Fprintf will fail, even
// for "standard" streams like `stdout` and `stderr`, if they have been set to non-blocking by an external process.
// In that case, we just drop the error on the floor and continue. We see this behavior in Travis when we try to write
// a lot of messages quickly (as we do when logging test failures)
func fprintf(w io.Writer, format string, a ...interface{}) {
	_, err := fmt.Fprintf(w, format, a...)
	contract.IgnoreError(err)
}

// programTester contains state associated with running a single test pass.
type programTester struct {
	t       *testing.T          // the Go tester for this run.
	opts    *ProgramTestOptions // options that control this test run.
	bin     string              // the `pulumi` binary we are using.
	yarnBin string              // the `yarn` binary we are using.
	goBin   string              // the `go` binary we are using.
}

func newProgramTester(t *testing.T, opts *ProgramTestOptions) *programTester {
	return &programTester{t: t, opts: opts}
}

func (pt *programTester) getBin() (string, error) {
	return getCmdBin(&pt.bin, "pulumi", pt.opts.Bin)
}

func (pt *programTester) getYarnBin() (string, error) {
	return getCmdBin(&pt.yarnBin, "yarn", pt.opts.YarnBin)
}

func (pt *programTester) getGoBin() (string, error) {
	return getCmdBin(&pt.goBin, "go", pt.opts.GoBin)
}

func (pt *programTester) pulumiCmd(args []string) ([]string, error) {
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

func (pt *programTester) yarnCmd(args []string) ([]string, error) {
	bin, err := pt.getYarnBin()
	if err != nil {
		return nil, err
	}
	result := []string{bin}
	result = append(result, args...)
	return withOptionalYarnFlags(result), nil
}

func (pt *programTester) runCommand(name string, args []string, wd string) error {
	return RunCommand(pt.t, name, args, wd, pt.opts)
}

func (pt *programTester) runPulumiCommand(name string, args []string, wd string) error {
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

	runErr := pt.runCommand(name, cmd, wd)
	if postFn != nil {
		if postErr := postFn(runErr); postErr != nil {
			return multierror.Append(runErr, postErr)
		}
	}
	return runErr
}

func (pt *programTester) runYarnCommand(name string, args []string, wd string) error {
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
				if try > 3 {
					return false, nil, errors.Errorf("%v did not complete after %v tries", cmd, try)
				}

				return false, nil, nil
			}

			// someother error, fail
			return false, nil, runerr
		},
	})
	return err
}

func (pt *programTester) testLifeCycleInitAndDestroy() error {
	tmpdir, projdir, err := pt.copyTestToTemporaryDirectory()
	if err != nil {
		return errors.Wrap(err, "copying test to temp dir")
	}

	testFinished := false
	defer func() {
		if tmpdir != "" {
			if !testFinished || pt.t.Failed() {
				// Test aborted or failed. Maybe copy to "failed tests" directory.
				failedTestsDir := os.Getenv("PULUMI_FAILED_TESTS_DIR")
				if failedTestsDir != "" {
					dest := filepath.Join(failedTestsDir, pt.t.Name()+uniqueSuffix())
					contract.IgnoreError(fsutil.CopyFile(dest, tmpdir, nil))
				}
			} else {
				contract.IgnoreError(os.RemoveAll(tmpdir))
			}
		} else {
			// When tmpdir is empty, we ran "in tree", which means we wrote output
			// to the "command-output" folder in the projdir, and we should clean
			// it up if the test passed
			if testFinished && !pt.t.Failed() {
				contract.IgnoreError(os.RemoveAll(filepath.Join(projdir, commandOutputFolderName)))
			}
		}
	}()

	err = pt.testLifeCycleInitialize(projdir)
	if err != nil {
		return errors.Wrap(err, "initializing test project")
	}

	// Ensure that before we exit, we attempt to destroy and remove the stack.
	defer func() {
		if projdir != "" {
			destroyErr := pt.testLifeCycleDestroy(projdir)
			assert.NoError(pt.t, destroyErr)
		}
	}()

	if err = pt.testPreviewUpdateAndEdits(projdir); err != nil {
		return errors.Wrap(err, "running test preview, update, and edits")
	}

	testFinished = true
	return nil
}

func (pt *programTester) testLifeCycleInitialize(dir string) error {
	stackName := pt.opts.GetStackName()

	// If RelativeWorkDir is specified, apply that relative to the temp folder for use as working directory during tests.
	if pt.opts.RelativeWorkDir != "" {
		dir = path.Join(dir, pt.opts.RelativeWorkDir)
	}

	// Set the default target Pulumi API if not overridden in options.
	if pt.opts.CloudURL == "" {
		pulumiAPI := os.Getenv("PULUMI_API")
		if pulumiAPI != "" {
			pt.opts.CloudURL = pulumiAPI
		}
	}

	// Set the target PPC from an environment variable if not overridden in options.
	if pt.opts.PPCName == "" {
		ppcName := os.Getenv("PULUMI_API_PPC_NAME")
		if ppcName != "" {
			pt.opts.PPCName = ppcName
		}
	}

	// Ensure all links are present, the stack is created, and all configs are applied.
	fprintf(pt.opts.Stdout, "Initializing project (dir %s; stack %s)\n", dir, stackName)

	// Login as needed.
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" && pt.opts.CloudURL == "" {
		fmt.Printf("Using existing logged in user for tests.  Set PULUMI_ACCESS_TOKEN and/or PULUMI_API to override.\n")
	} else {
		// Set PulumiCredentialsPathEnvVar to our CWD, so we use credentials specific to just this
		// test.
		pt.opts.Env = append(pt.opts.Env, fmt.Sprintf("%s=%s", workspace.PulumiCredentialsPathEnvVar, dir))

		loginArgs := []string{"login"}
		loginArgs = addFlagIfNonNil(loginArgs, "--cloud-url", pt.opts.CloudURL)

		if err := pt.runPulumiCommand("pulumi-login", loginArgs, dir); err != nil {
			return err
		}
	}

	// Stack init
	stackInitArgs := []string{"stack", "init", string(pt.opts.GetStackNameWithOwner())}
	stackInitArgs = addFlagIfNonNil(stackInitArgs, "--ppc", pt.opts.PPCName)

	if err := pt.runPulumiCommand("pulumi-stack-init", stackInitArgs, dir); err != nil {
		return err
	}

	for key, value := range pt.opts.Config {
		if err := pt.runPulumiCommand("pulumi-config",
			[]string{"config", "set", key, value}, dir); err != nil {
			return err
		}
	}

	for key, value := range pt.opts.Secrets {
		if err := pt.runPulumiCommand("pulumi-config",
			[]string{"config", "set", "--secret", key, value}, dir); err != nil {
			return err
		}
	}

	return nil
}

func (pt *programTester) testLifeCycleDestroy(dir string) error {
	// Destroy and remove the stack.
	fprintf(pt.opts.Stdout, "Destroying stack\n")
	destroy := []string{"destroy", "--non-interactive", "--skip-preview"}
	if pt.opts.GetDebugUpdates() {
		destroy = append(destroy, "-d")
	}
	if err := pt.runPulumiCommand("pulumi-destroy", destroy, dir); err != nil {
		return err
	}

	if pt.t.Failed() {
		fprintf(pt.opts.Stdout, "Test failed, retaining stack '%s'\n", pt.opts.GetStackNameWithOwner())
		return nil
	}

	return pt.runPulumiCommand("pulumi-stack-rm", []string{"stack", "rm", "--yes"}, dir)
}

func (pt *programTester) testPreviewUpdateAndEdits(dir string) error {
	// Now preview and update the real changes.
	fprintf(pt.opts.Stdout, "Performing primary preview and update\n")
	initErr := pt.previewAndUpdate(dir, "initial", pt.opts.ExpectFailure, false, false)

	// If the initial preview/update failed, just exit without trying the rest (but make sure to destroy).
	if initErr != nil {
		return initErr
	}

	// Perform an empty preview and update; nothing is expected to happen here.
	if !pt.opts.Quick {

		fprintf(pt.opts.Stdout, "Roundtripping checkpoint via stack export and stack import\n")

		if err := pt.exportImport(dir); err != nil {
			return err
		}

		msg := ""
		if !pt.opts.AllowEmptyUpdateChanges {
			msg = "(no changes expected)"
		}
		fprintf(pt.opts.Stdout, "Performing empty preview and update%s\n", msg)
		if err := pt.previewAndUpdate(
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
		refresh := []string{"refresh", "--non-interactive", "--skip-preview"}
		if pt.opts.GetDebugUpdates() {
			refresh = append(refresh, "-d")
		}
		if !pt.opts.ExpectRefreshChanges {
			refresh = append(refresh, "--expect-no-changes")
		}
		if err := pt.runPulumiCommand("pulumi-refresh", refresh, dir); err != nil {
			return err
		}
	}

	// If there are any edits, apply them and run a preview and update for each one.
	return pt.testEdits(dir)
}

func (pt *programTester) exportImport(dir string) error {
	exportCmd := []string{"stack", "export", "--file", "stack.json"}
	importCmd := []string{"stack", "import", "--file", "stack.json"}

	defer func() {
		contract.IgnoreError(os.Remove(filepath.Join(dir, "stack.json")))
	}()

	if err := pt.runPulumiCommand("pulumi-stack-export", exportCmd, dir); err != nil {
		return err
	}

	return pt.runPulumiCommand("pulumi-stack-import", importCmd, dir)
}

func (pt *programTester) previewAndUpdate(dir string, name string, shouldFail, expectNopPreview,
	expectNopUpdate bool) error {

	preview := []string{"preview", "--non-interactive"}
	update := []string{"up", "--non-interactive", "--skip-preview"}
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
	if pt.opts.UpdateCommandlineFlags != nil {
		update = append(update, pt.opts.UpdateCommandlineFlags...)
	}

	// If not in quick mode, run an explicit preview.
	if !pt.opts.Quick {
		if err := pt.runPulumiCommand("pulumi-preview-"+name, preview, dir); err != nil {
			if shouldFail {
				fprintf(pt.opts.Stdout, "Permitting failure (ExpectFailure=true for this preview)\n")
				return nil
			}
			return err
		}
	}

	// Now run an update.
	if err := pt.runPulumiCommand("pulumi-update-"+name, update, dir); err != nil {
		if shouldFail {
			fprintf(pt.opts.Stdout, "Permitting failure (ExpectFailure=true for this update)\n")
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

func (pt *programTester) testEdits(dir string) error {
	for i, edit := range pt.opts.EditDirs {
		var err error
		if err = pt.testEdit(dir, i, edit); err != nil {
			return err
		}
	}
	return nil
}

func (pt *programTester) testEdit(dir string, i int, edit EditDir) error {
	fprintf(pt.opts.Stdout, "Applying edit '%v' and rerunning preview and update\n", edit.Dir)

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

	if err = pt.previewAndUpdate(dir, fmt.Sprintf("edit-%d", i), edit.ExpectFailure, false, false); err != nil {
		return err
	}
	return pt.performExtraRuntimeValidation(edit.ExtraRuntimeValidation, dir)
}

func (pt *programTester) performExtraRuntimeValidation(
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
	fileName := path.Join(tempDir, "stack.json")

	// Invoke `pulumi stack export`
	if err = pt.runPulumiCommand("pulumi-export", []string{"stack", "export", "--file", fileName}, dir); err != nil {
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
	var deployment apitype.DeploymentV2
	if err = json.Unmarshal(untypedDeployment.Deployment, &deployment); err != nil {
		return err
	}

	// Get the root resource and outputs from the deployment
	var rootResource apitype.ResourceV2
	var outputs map[string]interface{}
	for _, res := range deployment.Resources {
		if res.Type == resource.RootStackType {
			rootResource = res
			outputs = res.Outputs
		}
	}

	// Populate stack info object with all of this data to pass to the validation function
	stackInfo := RuntimeValidationStackInfo{
		StackName:    pt.opts.GetStackName(),
		Deployment:   &deployment,
		RootResource: rootResource,
		Outputs:      outputs,
	}
	extraRuntimeValidation(pt.t, stackInfo)
	return nil
}

// copyTestToTemporaryDirectory creates a temporary directory to run the test in and copies the test to it.
func (pt *programTester) copyTestToTemporaryDirectory() (string, string, error) {
	// Get the source dir and project info.
	sourceDir := pt.opts.Dir
	projinfo, err := pt.getProjinfo(sourceDir)
	if err != nil {
		return "", "", err
	}

	// Set up a prefix so that all output has the test directory name in it.  This is important for debugging
	// because we run tests in parallel, and so all output will be interleaved and difficult to follow otherwise.
	var prefix string
	if len(sourceDir) <= 30 {
		prefix = fmt.Sprintf("[ %30.30s ] ", sourceDir)
	} else {
		prefix = fmt.Sprintf("[ %30.30s ] ", sourceDir[len(sourceDir)-30:])
	}
	stdout := pt.opts.Stdout
	if stdout == nil {
		stdout = newPrefixer(os.Stdout, prefix)
		pt.opts.Stdout = stdout
	}
	stderr := pt.opts.Stderr
	if stderr == nil {
		stderr = newPrefixer(os.Stderr, prefix)
		pt.opts.Stderr = stderr
	}

	fprintf(pt.opts.Stdout, "sample: %v\n", sourceDir)
	bin, err := pt.getBin()
	if err != nil {
		return "", "", err
	}
	fprintf(pt.opts.Stdout, "pulumi: %v\n", bin)

	// For most projects, we will copy to a temporary directory.  For Go projects, however, we must not perturb
	// the source layout, due to GOPATH and vendoring.  So, skip it for Go.
	var tmpdir, projdir string
	if projinfo.Proj.RuntimeInfo.Name() == "go" {
		projdir = projinfo.Root
	} else {
		stackName := string(pt.opts.GetStackName())
		targetDir, tempErr := ioutil.TempDir("", stackName+"-")
		if tempErr != nil {
			return "", "", errors.Wrap(tempErr, "Couldn't create temporary directory")
		}

		// Copy the source project.
		if copyErr := fsutil.CopyFile(targetDir, sourceDir, nil); copyErr != nil {
			return "", "", copyErr
		}

		// Set tmpdir so that the caller will clean up afterwards.
		tmpdir = targetDir
		projdir = targetDir
	}
	projinfo.Root = projdir

	err = pt.prepareProject(projinfo)
	if err != nil {
		return "", "", errors.Wrapf(err, "Failed to prepare %v", projdir)
	}

	fprintf(stdout, "projdir: %v\n", projdir)
	return tmpdir, projdir, nil
}

func (pt *programTester) getProjinfo(projectDir string) (*engine.Projinfo, error) {
	// Load up the package so we know things like what language the project is.
	projfile := filepath.Join(projectDir, workspace.ProjectFile+".yaml")
	proj, err := workspace.LoadProject(projfile)
	if err != nil {
		return nil, err
	}
	return &engine.Projinfo{Proj: proj, Root: projectDir}, nil
}

// prepareProject runs setup necessary to get the project ready for `pulumi` commands.
func (pt *programTester) prepareProject(projinfo *engine.Projinfo) error {
	// Based on the language, invoke the right routine to prepare the target directory.
	switch rt := projinfo.Proj.RuntimeInfo.Name(); rt {
	case "nodejs":
		return pt.prepareNodeJSProject(projinfo)
	case "python":
		return pt.preparePythonProject(projinfo)
	case "go":
		return pt.prepareGoProject(projinfo)
	default:
		return errors.Errorf("unrecognized project runtime: %s", rt)
	}
}

// prepareProjectDir runs setup necessary to get the project ready for `pulumi` commands.
func (pt *programTester) prepareProjectDir(projectDir string) error {
	projinfo, err := pt.getProjinfo(projectDir)
	if err != nil {
		return err
	}
	return pt.prepareProject(projinfo)
}

// prepareNodeJSProject runs setup necessary to get a Node.js project ready for `pulumi` commands.
func (pt *programTester) prepareNodeJSProject(projinfo *engine.Projinfo) error {
	// Write a .yarnrc file to pass --mutex network to all yarn invocations, since tests
	// may run concurrently and yarn may fail if invoked concurrently
	// https://github.com/yarnpkg/yarn/issues/683
	// Also add --network-concurrency 1 since we've been seeing
	// https://github.com/yarnpkg/yarn/issues/4563 as well
	if err := ioutil.WriteFile(
		filepath.Join(projinfo.Root, ".yarnrc"),
		[]byte("--mutex network\n--network-concurrency 1\n"), 0644); err != nil {
		return err
	}

	// Get the correct pwd to run Yarn in.
	cwd, _, err := projinfo.GetPwdMain()
	if err != nil {
		return err
	}

	// Now ensure dependencies are present.
	if err = pt.runYarnCommand("yarn-install", []string{"install", "--verbose"}, cwd); err != nil {
		return err
	}
	for _, dependency := range pt.opts.Dependencies {
		if err = pt.runYarnCommand("yarn-link", []string{"link", dependency}, cwd); err != nil {
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

// preparePythonProject runs setup necessary to get a Python project ready for `pulumi` commands.
func (pt *programTester) preparePythonProject(projinfo *engine.Projinfo) error {
	return nil
}

// prepareGoProject runs setup necessary to get a Go project ready for `pulumi` commands.
func (pt *programTester) prepareGoProject(projinfo *engine.Projinfo) error {
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

	// To compile, simply run `go build -o $GOPATH/bin/<projname> .` from the project's working directory.
	cwd, _, err := projinfo.GetPwdMain()
	if err != nil {
		return err
	}
	outBin := filepath.Join(gopath, "bin", string(projinfo.Proj.Name))
	return pt.runCommand("go-build", []string{goBin, "build", "-o", outBin, "."}, cwd)
}
