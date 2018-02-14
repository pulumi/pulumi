// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package integration

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/fsutil"
	"github.com/pulumi/pulumi/pkg/util/retry"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// RuntimeValidationStackInfo contains details related to the stack that runtime validation logic may want to use.
type RuntimeValidationStackInfo struct {
	Checkpoint   stack.Checkpoint
	Snapshot     deploy.Snapshot
	RootResource resource.State
	Outputs      map[string]interface{}
}

// EditDir is an optional edit to apply to the example, as subsequent deployments.
type EditDir struct {
	Dir                    string
	ExtraRuntimeValidation func(t *testing.T, stack RuntimeValidationStackInfo)

	// Additive is true if Dir should be copied *on top* of the test directory.
	// Otherwise Dir *replaces* the test directory, except we keep .pulumi/ and Pulumi.yaml.
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
	// Map of secure config keys and values to set on the Lumi stack (e.g. {"aws:config:region": "us-east-2"})
	Secrets map[string]string
	// EditDirs is an optional list of edits to apply to the example, as subsequent deployments.
	EditDirs []EditDir
	// ExtraRuntimeValidation is an optional callback for additional validation, called before applying edits.
	ExtraRuntimeValidation func(t *testing.T, stack RuntimeValidationStackInfo)
	// RelativeWorkDir is an optional path relative to `Dir` which should be used as working directory during tests.
	RelativeWorkDir string
	// ExpectFailure is true if we expect this test to fail.  This is very coarse grained, and will essentially
	// tolerate *any* failure in the program (IDEA: in the future, offer a way to narrow this down more).
	ExpectFailure bool
	// Quick can be set to true to run a "quick" test that skips any non-essential steps (e.g., empty updates).
	Quick bool
	// UpdateCommandlineFlags specifies flags to add to the `pulumi update` command line (e.g. "--color=raw")
	UpdateCommandlineFlags []string

	// CloudURL is an optional URL to a Pulumi Service API. If set, the program test will attempt to login
	// to that CloudURL (assuming PULUMI_ACCESS_TOKEN is set) and create the stack using that hosted service.
	// If nil, will test Pulumi using the fire-and-forget mode.
	CloudURL string
	// Owner and Repo are optional values to specify during calls to `pulumi init`. Otherwise the --owner and
	// --repo flags will not be set.
	Owner string
	Repo  string
	// PPCName is the name of the PPC to use when running a test against the hosted service. If
	// not set, the --ppc flag will not be set on `pulumi stack init`.
	PPCName string

	// StackName allows the stack name to be explicitly provided instead of computed from the
	// environment during tests.
	StackName string

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
}

func (opts *ProgramTestOptions) GetDebugLogLevel() int {
	if opts.DebugLogLevel > 0 {
		return opts.DebugLogLevel
	}
	if du := os.Getenv("PULUMI_TEST_DEBUG_LOG_LEVEL"); du != "" {
		if n, _ := strconv.Atoi(du); n > 0 { // nolint: gas
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

		opts.StackName = strings.ToLower("p-it-" + host + "-" + test)
	}

	return tokens.QName(opts.StackName)
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
	if overrides.Owner != "" {
		opts.Owner = overrides.Owner
	}
	if overrides.Repo != "" {
		opts.Repo = overrides.Repo
	}
	if overrides.PPCName != "" {
		opts.PPCName = overrides.PPCName
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
	return opts
}

// ProgramTest runs a lifecycle of Pulumi commands in a program working directory, using the `pulumi` and `yarn`
// binaries available on PATH.  It essentially executes the following workflow:
//
//   yarn install
//   yarn link <each opts.Depencies>
//   yarn run build
//   pulumi init
//   (*) pulumi login
//   pulumi stack init integrationtesting
//   pulumi config set <each opts.Config>
//   pulumi config set --secret <each opts.Secrets>
//   pulumi preview
//   pulumi update
//   pulumi preview (expected to be empty)
//   pulumi update (expected to be empty)
//   pulumi destroy --yes
//   pulumi stack rm --yes integrationtesting
//   (*) pulumi logout
//
//   (*) Only if ProgramTestOptions.CloudURL is not empty.
//
// All commands must return success return codes for the test to succeed, unless ExpectFailure is true.
func ProgramTest(t *testing.T, opts *ProgramTestOptions) {
	t.Parallel()

	// If the test panics, recover and log instead of letting the panic escape the test. Even though *this* test will
	// have run deferred functions and cleaned up, if the panic reaches toplevel it will kill the process and prevent
	// other tests running in parallel from cleaning up.
	defer func() {
		if failure := recover(); failure != nil {
			t.Errorf("panic testing %v: %v", opts.Dir, failure)
		}
	}()

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

func (pt *programTester) pulumiCmd(args []string) ([]string, error) {
	bin, err := pt.getBin()
	if err != nil {
		return nil, err
	}
	cmd := []string{bin}
	if du := pt.opts.GetDebugLogLevel(); du > 0 {
		cmd = append(cmd, "--logtostderr")
		cmd = append(cmd, "-v="+strconv.Itoa(du))
	}
	return append(cmd, args...), nil
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
	return pt.runCommand(name, cmd, wd)
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
	dir, err := pt.copyTestToTemporaryDirectory()
	if err != nil {
		return errors.Wrap(err, "copying test to temp dir")
	}

	// Keep the temporary test directory around for debugging unless
	// the test completes successfully.
	keepTestDir := true
	defer func() {
		if keepTestDir {
			// Maybe copy to "failed tests" directory.
			failedTestsDir := os.Getenv("PULUMI_FAILED_TESTS_DIR")
			if failedTestsDir != "" {
				dest := filepath.Join(failedTestsDir, pt.t.Name()+uniqueSuffix())
				contract.IgnoreError(fsutil.CopyFile(dest, dir, nil))
			}
		} else {
			contract.IgnoreError(os.RemoveAll(dir))
		}
	}()

	err = pt.testLifeCycleInitialize(dir)
	if err != nil {
		return errors.Wrap(err, "initializing test project")
	}

	// Ensure that before we exit, we attempt to destroy and remove the stack.
	defer func() {
		if dir != "" {
			destroyErr := pt.testLifeCycleDestroy(dir)
			assert.NoError(pt.t, destroyErr)
		}
	}()

	if err = pt.testPreviewUpdateAndEdits(dir); err != nil {
		return errors.Wrap(err, "running test preview, update, and edits")
	}

	// Ran to completion. Delete the test directory if the test passed.
	keepTestDir = pt.t.Failed()

	return nil
}

func (pt *programTester) testLifeCycleInitialize(dir string) error {
	stackName := pt.opts.GetStackName()

	// If RelativeWorkDir is specified, apply that relative to the temp folder for use as working directory during tests.
	if pt.opts.RelativeWorkDir != "" {
		dir = path.Join(dir, pt.opts.RelativeWorkDir)
	}

	// Ensure all links are present, the stack is created, and all configs are applied.
	fprintf(pt.opts.Stdout, "Initializing project (dir %s; stack %s)\n", dir, stackName)

	initArgs := []string{"init"}
	initArgs = addFlagIfNonNil(initArgs, "--owner", pt.opts.Owner)
	initArgs = addFlagIfNonNil(initArgs, "--name", pt.opts.Repo)
	if err := pt.runPulumiCommand("pulumi-init", initArgs, dir); err != nil {
		return err
	}

	// Login as needed.
	if pt.opts.CloudURL != "" {
		if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
			pt.t.Fatalf("Unable to run pulumi login. PULUMI_ACCESS_TOKEN environment variable not set.")
		}

		// Set the "use alt location" flag so this test doesn't interact with any credentials already on the machine.
		// e.g. replacing the current user's with that of a test account.
		if err := os.Setenv(workspace.UseAltCredentialsLocationEnvVar, "1"); err != nil {
			pt.t.Fatalf("error setting env var '%s': %v", workspace.UseAltCredentialsLocationEnvVar, err)
		}

		if err := pt.runPulumiCommand("pulumi-login",
			append([]string{"login", "--cloud-url", pt.opts.CloudURL}), dir); err != nil {
			return err
		}
	}

	// Stack init
	stackInitArgs := []string{"stack", "init", string(stackName)}
	if pt.opts.CloudURL == "" {
		stackInitArgs = append(stackInitArgs, "--local")
	} else {
		stackInitArgs = addFlagIfNonNil(stackInitArgs, "--cloud-url", pt.opts.CloudURL)
		stackInitArgs = addFlagIfNonNil(stackInitArgs, "--ppc", pt.opts.PPCName)
	}
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
	stackName := pt.opts.GetStackName()

	// Destroy and remove the stack.
	fprintf(pt.opts.Stdout, "Destroying stack\n")
	destroy := []string{"destroy", "--yes"}
	if pt.opts.GetDebugUpdates() {
		destroy = append(destroy, "-d")
	}
	if err := pt.runPulumiCommand("pulumi-destroy", destroy, dir); err != nil {
		return err
	}

	if err := pt.runPulumiCommand("pulumi-stack-rm",
		[]string{"stack", "rm", "--yes", string(stackName)}, dir); err != nil {
		return err
	}

	if pt.opts.CloudURL != "" {
		return pt.runPulumiCommand("pulumi-logout",
			[]string{"logout", "--cloud-url", pt.opts.CloudURL}, dir)
	}

	return nil
}

func (pt *programTester) testPreviewUpdateAndEdits(dir string) error {
	// Now preview and update the real changes.
	fprintf(pt.opts.Stdout, "Performing primary preview and update\n")
	initErr := pt.previewAndUpdate(dir, "initial", pt.opts.ExpectFailure)

	// If the initial preview/update failed, just exit without trying the rest (but make sure to destroy).
	if initErr != nil {
		return initErr
	}

	// Perform an empty preview and update; nothing is expected to happen here.
	if !pt.opts.Quick {
		fprintf(pt.opts.Stdout, "Performing empty preview and update (no changes expected)\n")
		if err := pt.previewAndUpdate(dir, "empty", false); err != nil {
			return err
		}
	}

	// Run additional validation provided by the test options, passing in the checkpoint info.
	if err := pt.performExtraRuntimeValidation(pt.opts.ExtraRuntimeValidation, dir); err != nil {
		return err
	}

	// If there are any edits, apply them and run a preview and update for each one.
	return pt.testEdits(dir)
}

func (pt *programTester) previewAndUpdate(dir string, name string, shouldFail bool) error {
	preview := []string{"preview"}
	update := []string{"update"}
	if pt.opts.GetDebugUpdates() {
		preview = append(preview, "-d")
		update = append(update, "-d")
	}
	if pt.opts.UpdateCommandlineFlags != nil {
		update = append(update, pt.opts.UpdateCommandlineFlags...)
	}

	if !pt.opts.Quick {
		if err := pt.runPulumiCommand("pulumi-preview-"+name, preview, dir); err != nil {
			if shouldFail {
				fprintf(pt.opts.Stdout, "Permitting failure (ExpectFailure=true for this preview)\n")
				return nil
			}
			return err
		}
	}

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

		// Copy everything except Pulumi.yaml and .pulumi from source into new directory
		exclusions := make(map[string]bool)
		projectYaml := workspace.ProjectFile + ".yaml"
		exclusions[workspace.BookkeepingDir] = true
		exclusions[projectYaml] = true

		if err := fsutil.CopyFile(newDir, edit.Dir, exclusions); err != nil {
			return errors.Wrapf(err, "Couldn't copy %v into %v", edit.Dir, newDir)
		}

		// Copy Pulumi.yaml and .pulumi from old directory to new directory
		oldProjectYaml := filepath.Join(dir, projectYaml)
		newProjectYaml := filepath.Join(newDir, projectYaml)

		oldProjectDir := filepath.Join(dir, workspace.BookkeepingDir)
		newProjectDir := filepath.Join(newDir, workspace.BookkeepingDir)

		if err := fsutil.CopyFile(newProjectYaml, oldProjectYaml, nil); err != nil {
			return errors.Wrapf(err, "Couldn't copy Pulumi.yaml")
		}
		if err := fsutil.CopyFile(newProjectDir, oldProjectDir, nil); err != nil {
			return errors.Wrapf(err, "Couldn't copy .pulumi")
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

	err := pt.prepareProject(dir)
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

	if err = pt.previewAndUpdate(dir, fmt.Sprintf("edit-%d", i), edit.ExpectFailure); err != nil {
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

	// Load up the checkpoint file from .pulumi/stacks/<project-name>/<stack-name>.json.
	ws, err := workspace.NewFrom(dir)
	if err != nil {
		return errors.Wrapf(err, "expected to load project workspace at %v", dir)
	}
	chk, err := stack.GetCheckpoint(ws, stackName)
	if err != nil {
		return errors.Wrapf(err, "expected to load checkpoint file for target %v: %v", stackName)
	} else if !assert.NotNil(pt.t, chk, "expected checkpoint file to be populated from %v: %v", stackName, err) {
		return errors.New("missing checkpoint")
	}

	// Deserialize snapshot from checkpoint
	snapshot, err := stack.DeserializeCheckpoint(chk)
	if err != nil {
		return errors.Wrapf(err, "expected checkpoint deserialization to succeed")
	} else if !assert.NotNil(pt.t, snapshot, "expected snapshot to be populated from checkpoint file %v", stackName) {
		return errors.New("missing snapshot")
	}

	// Get root resources from snapshot
	rootResource, outputs := stack.GetRootStackResource(snapshot)
	if !assert.NotNil(pt.t, rootResource, "expected root resource to be populated from snapshot file %v", stackName) {
		return errors.New("missing root resource")
	}

	// Populate stack info object with all of this data to pass to the validation function
	stackInfo := RuntimeValidationStackInfo{
		Checkpoint:   *chk,
		Snapshot:     *snapshot,
		RootResource: *rootResource,
		Outputs:      outputs,
	}
	extraRuntimeValidation(pt.t, stackInfo)
	return nil
}

// copyTestToTemporaryDirectory creates a temporary directory to run the test in and copies the test to it.
func (pt *programTester) copyTestToTemporaryDirectory() (dir string, err error) {
	// Set up a prefix so that all output has the test directory name in it.  This is important for debugging
	// because we run tests in parallel, and so all output will be interleaved and difficult to follow otherwise.
	sourceDir := pt.opts.Dir
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
		return "", err
	}
	fprintf(pt.opts.Stdout, "pulumi: %v\n", bin)

	stackName := string(pt.opts.GetStackName())
	targetDir, err := ioutil.TempDir("", stackName+"-")
	if err != nil {
		return "", errors.Wrap(err, "Couldn't create temporary directory")
	}

	// Clean up the temporary directory on failure
	deleteTargetDir := true
	defer func() {
		if deleteTargetDir {
			contract.IgnoreError(os.RemoveAll(targetDir))
		}
	}()

	// Copy the source project
	if err = fsutil.CopyFile(targetDir, sourceDir, nil); err != nil {
		return "", err
	}

	err = pt.prepareProject(targetDir)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to prepare %v", targetDir)
	}

	fprintf(stdout, "projdir: %v\n", targetDir)
	deleteTargetDir = false
	return targetDir, nil
}

// prepareProject runs setup necessary to get the project ready for `pulumi` commands.
func (pt *programTester) prepareProject(projectDir string) error {
	// Write a .yarnrc file to pass --mutex network to all yarn invocations, since tests
	// may run concurrently and yarn may fail if invoked concurrently
	// https://github.com/yarnpkg/yarn/issues/683
	// Also add --network-concurrency 1 since we've been seeing
	// https://github.com/yarnpkg/yarn/issues/4563 as well
	yarnrcerr := ioutil.WriteFile(filepath.Join(projectDir, ".yarnrc"),
		[]byte("--mutex network\n--network-concurrency 1\n"), 0644)
	if yarnrcerr != nil {
		return yarnrcerr
	}

	// Load up the package so we can run Yarn in the correct location.
	projfile := filepath.Join(projectDir, workspace.ProjectFile+".yaml")
	proj, err := workspace.LoadProject(projfile)
	if err != nil {
		return err
	}
	projinfo := &engine.Projinfo{Proj: proj, Root: projectDir}
	cwd, _, err := projinfo.GetPwdMain()
	if err != nil {
		return err
	}

	if rwd := pt.opts.RelativeWorkDir; rwd != "" {
		cwd = path.Join(cwd, rwd)
	}

	// Now ensure dependencies are present.
	if insterr := pt.runYarnCommand("yarn-install", []string{"install", "--verbose"}, cwd); insterr != nil {
		return insterr
	}
	for _, dependency := range pt.opts.Dependencies {
		if linkerr := pt.runYarnCommand("yarn-link", []string{"link", dependency}, cwd); linkerr != nil {
			return linkerr
		}
	}

	// And finally compile it using whatever build steps are in the package.json file.
	return pt.runYarnCommand("yarn-build", []string{"run", "build"}, cwd)
}
