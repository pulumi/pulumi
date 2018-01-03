// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package integration

import (
	"bytes"
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
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/fsutil"
	pioutil "github.com/pulumi/pulumi/pkg/util/ioutil"
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
	Additive               bool // set to true to keep the prior test dir, applying this edit atop.

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

func (opts *ProgramTestOptions) PulumiCmd(args []string) []string {
	cmd := []string{opts.Bin}
	if du := opts.GetDebugLogLevel(); du > 0 {
		cmd = append(cmd, "--logtostderr")
		cmd = append(cmd, "-v="+strconv.Itoa(du))
	}
	return append(cmd, args...)
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
//   pulumi stack init integrationtesting
//   pulumi config set <each opts.Config>
//   pulumi config set --secret <each opts.Secrets>
//   pulumi preview
//   pulumi update
//   pulumi preview (expected to be empty)
//   pulumi update (expected to be empty)
//   pulumi destroy --yes
//   pulumi stack rm --yes integrationtesting
//
// All commands must return success return codes for the test to succeed, unless ExpectFailure is true.
func ProgramTest(t *testing.T, opts *ProgramTestOptions) {
	err := TestLifeCycleInitAndDestroy(t, opts, testPreviewUpdateAndEdits)
	assert.NoError(t, err)
}

func TestLifeCycleInitAndDestroy(
	t *testing.T, opts *ProgramTestOptions,
	between func(*testing.T, *ProgramTestOptions, string) (string, error)) error {

	t.Parallel()

	dir, err := TestLifeCycleInitialize(t, opts)
	if err != nil {
		return err
	}

	// Ensure that before we exit, we attempt to destroy and remove the stack.
	defer func() {
		if dir != "" {
			destroyErr := TestLifeCycleDestroy(t, opts, dir)
			assert.NoError(t, destroyErr)
		}
	}()

	dir, err = between(t, opts, dir)
	return err
}

func TestLifeCycleInitialize(t *testing.T, opts *ProgramTestOptions) (string, error) {
	stackName := opts.GetStackName()

	// Perform the initial stack creation.

	dir, err := CopyTestToTemporaryDirectory(t, opts)
	if err != nil {
		return "", err
	}

	// If RelativeWorkDir is specified, apply that relative to the temp folder for use as working directory during tests.
	if opts.RelativeWorkDir != "" {
		dir = path.Join(dir, opts.RelativeWorkDir)
	}

	// Ensure all links are present, the stack is created, and all configs are applied.
	pioutil.MustFprintf(opts.Stdout, "Initializing project (dir %s; stack %s)\n", dir, stackName)
	if err = RunCommand(t, "pulumi-init",
		opts.PulumiCmd([]string{"init"}), dir, opts); err != nil {
		return "", err
	}

	if err = RunCommand(t, "pulumi-stack-init",
		opts.PulumiCmd([]string{"stack", "init", "--local", string(stackName)}), dir, opts); err != nil {
		return "", err
	}

	for key, value := range opts.Config {
		if err = RunCommand(t, "pulumi-config",
			opts.PulumiCmd([]string{"config", "set", key, value}), dir, opts); err != nil {
			return "", err
		}
	}

	for key, value := range opts.Secrets {
		if err = RunCommand(t, "pulumi-config",
			opts.PulumiCmd([]string{"config", "set", "--secret", key, value}), dir, opts); err != nil {
			return "", err
		}
	}

	return dir, nil
}

func TestLifeCycleDestroy(t *testing.T, opts *ProgramTestOptions, dir string) error {
	stackName := opts.GetStackName()

	destroy := opts.PulumiCmd([]string{"destroy", "--yes"})
	if opts.GetDebugUpdates() {
		destroy = append(destroy, "-d")
	}

	// Finally, tear down the stack, and clean up the stack.  Ignore errors to try to get as clean as possible.
	pioutil.MustFprintf(opts.Stdout, "Destroying stack\n")
	if err := RunCommand(t, "pulumi-destroy", destroy, dir, opts); err != nil {
		return err
	}
	return RunCommand(t, "pulumi-stack-rm",
		opts.PulumiCmd([]string{"stack", "rm", "--yes", string(stackName)}), dir, opts)
}

func testPreviewUpdateAndEdits(t *testing.T, opts *ProgramTestOptions, dir string) (string, error) {
	// Now preview and update the real changes.
	pioutil.MustFprintf(opts.Stdout, "Performing primary preview and update\n")
	initErr := previewAndUpdate(t, opts, dir, "initial", opts.ExpectFailure)

	// If the initial preview/update failed, just exit without trying the rest (but make sure to destroy).
	if initErr != nil {
		return dir, initErr
	}

	// Perform an empty preview and update; nothing is expected to happen here.
	if !opts.Quick {
		pioutil.MustFprintf(opts.Stdout, "Performing empty preview and update (no changes expected)\n")
		if err := previewAndUpdate(t, opts, dir, "empty", false); err != nil {
			return dir, err
		}
	}

	// Run additional validation provided by the test options, passing in the checkpoint info.
	if err := performExtraRuntimeValidation(t, opts, opts.ExtraRuntimeValidation, dir); err != nil {
		return dir, err
	}

	// If there are any edits, apply them and run a preview and update for each one.
	return testEdits(t, opts, dir)
}

func previewAndUpdate(t *testing.T, opts *ProgramTestOptions, dir string, name string, shouldFail bool) error {
	preview := opts.PulumiCmd([]string{"preview"})
	update := opts.PulumiCmd([]string{"update"})
	if opts.GetDebugUpdates() {
		preview = append(preview, "-d")
		update = append(update, "-d")
	}
	if opts.UpdateCommandlineFlags != nil {
		update = append(update, opts.UpdateCommandlineFlags...)
	}

	if !opts.Quick {
		if err := RunCommand(t, "pulumi-preview-"+name, preview, dir, opts); err != nil {
			if shouldFail {
				pioutil.MustFprintf(opts.Stdout, "Permitting failure (ExpectFailure=true for this preview)\n")
				return nil
			}
			return err
		}
	}

	if err := RunCommand(t, "pulumi-update-"+name, update, dir, opts); err != nil {
		if shouldFail {
			pioutil.MustFprintf(opts.Stdout, "Permitting failure (ExpectFailure=true for this update)\n")
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

func testEdits(t *testing.T, opts *ProgramTestOptions, dir string) (string, error) {
	for i, edit := range opts.EditDirs {
		var err error
		if dir, err = testEdit(t, opts, dir, i, edit); err != nil {
			return dir, err
		}
	}
	return dir, nil
}

func testEdit(t *testing.T, opts *ProgramTestOptions, dir string, i int, edit EditDir) (string, error) {
	pioutil.MustFprintf(opts.Stdout, "Applying edit '%v' and rerunning preview and update\n", edit.Dir)

	newDir, err := prepareProject(t, opts, edit.Dir, dir, edit.Additive)
	if err != nil {
		return dir, errors.Wrapf(err, "Expected to apply edit %v atop %v, but got an error", edit, dir)
	}

	dir = newDir
	oldStdOut := opts.Stdout
	oldStderr := opts.Stderr
	oldVerbose := opts.Verbose
	if edit.Stdout != nil {
		opts.Stdout = edit.Stdout
	}
	if edit.Stderr != nil {
		opts.Stderr = edit.Stderr
	}
	if edit.Verbose {
		opts.Verbose = true
	}

	defer func() {
		opts.Stdout = oldStdOut
		opts.Stderr = oldStderr
		opts.Verbose = oldVerbose
	}()

	if err = previewAndUpdate(t, opts, dir, fmt.Sprintf("edit-%d", i), edit.ExpectFailure); err != nil {
		return dir, err
	}
	if err = performExtraRuntimeValidation(t, opts, edit.ExtraRuntimeValidation, dir); err != nil {
		return dir, err
	}

	return dir, nil
}

func performExtraRuntimeValidation(
	t *testing.T, opts *ProgramTestOptions,
	extraRuntimeValidation func(t *testing.T, stack RuntimeValidationStackInfo), dir string) error {

	if extraRuntimeValidation == nil {
		return nil
	}

	stackName := opts.GetStackName()

	// Load up the checkpoint file from .pulumi/stacks/<project-name>/<stack-name>.json.
	ws, err := workspace.NewFrom(dir)
	if err != nil {
		return errors.Wrapf(err, "expected to load project workspace at %v", dir)
	}
	chk, err := stack.GetCheckpoint(ws, stackName)
	if err != nil {
		return errors.Wrapf(err, "expected to load checkpoint file for target %v: %v", stackName)
	} else if !assert.NotNil(t, chk, "expected checkpoint file to be populated from %v: %v", stackName, err) {
		return errors.New("missing checkpoint")
	}

	// Deserialize snapshot from checkpoint
	snapshot, err := stack.DeserializeCheckpoint(chk)
	if err != nil {
		return errors.Wrapf(err, "expected checkpoint deserialization to succeed")
	} else if !assert.NotNil(t, snapshot, "expected snapshot to be populated from checkpoint file %v", stackName) {
		return errors.New("missing snapshot")
	}

	// Get root resources from snapshot
	rootResource, outputs := stack.GetRootStackResource(snapshot)
	if !assert.NotNil(t, rootResource, "expected root resource to be populated from snapshot file %v", stackName) {
		return errors.New("missing root resource")
	}

	// Populate stack info object with all of this data to pass to the validation function
	stackInfo := RuntimeValidationStackInfo{
		Checkpoint:   *chk,
		Snapshot:     *snapshot,
		RootResource: *rootResource,
		Outputs:      outputs,
	}
	extraRuntimeValidation(t, stackInfo)
	return nil
}

// CopyTestToTemporaryDirectory creates a temporary directory to run the test in and copies the test to it.
func CopyTestToTemporaryDirectory(t *testing.T, opts *ProgramTestOptions) (dir string, err error) {
	// Ensure the required programs are present.
	if opts.Bin == "" {
		var pulumi string
		pulumi, err = exec.LookPath("pulumi")
		if err != nil {
			return "", errors.Wrapf(err, "Expected to find `pulumi` binary on $PATH")
		}
		opts.Bin = pulumi
	}
	if opts.YarnBin == "" {
		var yarn string
		yarn, err = exec.LookPath("yarn")
		if err != nil {
			return "", errors.Wrapf(err, "Expected to find `yarn` binary on $PATH")
		}
		opts.YarnBin = yarn
	}

	// Set up a prefix so that all output has the test directory name in it.  This is important for debugging
	// because we run tests in parallel, and so all output will be interleaved and difficult to follow otherwise.
	dir = opts.Dir
	var prefix string
	if len(dir) <= 30 {
		prefix = fmt.Sprintf("[ %30.30s ] ", dir)
	} else {
		prefix = fmt.Sprintf("[ %30.30s ] ", dir[len(dir)-30:])
	}
	stdout := opts.Stdout
	if stdout == nil {
		stdout = newPrefixer(os.Stdout, prefix)
		opts.Stdout = stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = newPrefixer(os.Stderr, prefix)
		opts.Stderr = stderr
	}

	pioutil.MustFprintf(opts.Stdout, "sample: %v\n", dir)
	pioutil.MustFprintf(opts.Stdout, "pulumi: %v\n", opts.Bin)
	pioutil.MustFprintf(opts.Stdout, "yarn: %v\n", opts.YarnBin)

	// Now copy the source project, excluding the .pulumi directory.
	dir, err = prepareProject(t, opts, dir, "", false)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to copy source project %v to a new temp dir", dir)
	}

	pioutil.MustFprintf(stdout, "projdir: %v\n", dir)
	return dir, nil
}

// RunCommand executes the specified command and additional arguments, wrapping any output in the
// specialized test output streams that list the location the test is running in.
func RunCommand(t *testing.T, name string, args []string, wd string, opts *ProgramTestOptions) error {
	path := args[0]
	command := strings.Join(args, " ")

	pioutil.MustFprintf(opts.Stdout, "**** Invoke '%v' in '%v'\n", command, wd)

	// Spawn a goroutine to print out "still running..." messages.
	finished := false
	go func() {
		for !finished {
			time.Sleep(30 * time.Second)
			if !finished {
				pioutil.MustFprintf(opts.Stderr, "Still running command '%s' (%s)...\n", command, wd)
			}
		}
	}()

	var env []string
	env = append(env, os.Environ()...)
	env = append(env, "PULUMI_RETAIN_CHECKPOINTS=true")
	env = append(env, "PULUMI_CONFIG_PASSPHRASE=correct horse battery staple")

	cmd := exec.Cmd{
		Path: path,
		Dir:  wd,
		Args: args,
		Env:  env,
	}

	startTime := time.Now()

	var runout []byte
	var runerr error
	if opts.Verbose || os.Getenv("PULUMI_VERBOSE_TEST") != "" {
		cmd.Stdout = opts.Stdout
		cmd.Stderr = opts.Stderr
		runerr = cmd.Run()
	} else {
		runout, runerr = cmd.CombinedOutput()
	}

	endTime := time.Now()

	if opts.ReportStats != nil {
		// Note: This data is archived and used by external analytics tools.  Take care if changing the schema or format
		// of this data.
		opts.ReportStats.ReportCommand(TestCommandStats{
			StartTime:      startTime.Format("2006/01/02 15:04:05"),
			EndTime:        endTime.Format("2006/01/02 15:04:05"),
			ElapsedSeconds: float64((endTime.Sub(startTime)).Nanoseconds()) / 1000000000,
			StepName:       name,
			CommandLine:    command,
			StackName:      string(opts.GetStackName()),
			TestID:         wd,
			TestName:       filepath.Base(opts.Dir),
			IsError:        runerr != nil,
		})
	}

	finished = true
	if runerr != nil {
		pioutil.MustFprintf(opts.Stderr, "Invoke '%v' failed: %s\n", command, cmdutil.DetailedError(runerr))
		if !opts.Verbose {
			pioutil.MustFprintf(opts.Stderr, "%s\n", string(runout))
		}
	}

	return runerr
}

// prepareProject copies the source directory, src (excluding .pulumi), to a new temporary directory.  It then copies
// .pulumi/ and Pulumi.yaml from origin, if any, for edits.  The function returns the newly resulting directory.
func prepareProject(t *testing.T, opts *ProgramTestOptions, src, origin string, additive bool) (string, error) {
	stackName := opts.GetStackName()

	var dir string

	// If additive, keep the existing directory.  Otherwise, create a new one.
	if additive {
		dir = origin
	} else {
		// Create a new temp directory.
		var err error
		dir, err = ioutil.TempDir("", string(stackName)+"-")
		if err != nil {
			return "", err
		}
	}

	// Now copy the source into it, ignoring .pulumi/ and Pulumi.yaml if there's an origin.
	wdir := workspace.BookkeepingDir
	proj := workspace.ProjectFile + ".yaml"
	excl := make(map[string]bool)
	if origin != "" {
		excl[wdir] = true
		excl[proj] = true
	}
	if copyerr := fsutil.CopyFile(dir, src, excl); copyerr != nil {
		return "", copyerr
	}

	projfile := filepath.Join(dir, proj)
	if !additive {
		// Now, copy back the original project's .pulumi/ and Pulumi.yaml atop the target.
		if origin != "" {
			if copyerr := fsutil.CopyFile(projfile, filepath.Join(origin, proj), nil); copyerr != nil {
				return "", copyerr
			}
			if copyerr := fsutil.CopyFile(filepath.Join(dir, wdir), filepath.Join(origin, wdir), nil); copyerr != nil {
				return "", copyerr
			}
		}
	}

	// Write a .yarnrc file to pass --mutex network to all yarn invocations, since tests
	// may run concurrently and yarn may fail if invoked concurrently
	// https://github.com/yarnpkg/yarn/issues/683
	yarnrcerr := ioutil.WriteFile(filepath.Join(dir, ".yarnrc"), []byte("--mutex network\n"), 0644)
	if yarnrcerr != nil {
		return "", yarnrcerr
	}

	// Load up the package so we can run Yarn in the correct location.
	pkginfo, err := engine.ReadPackage(projfile)
	if err != nil {
		return "", err
	}
	cwd, _, err := pkginfo.GetPwdMain()
	if err != nil {
		return "", err
	}

	if opts.RelativeWorkDir != "" {
		cwd = path.Join(cwd, opts.RelativeWorkDir)
	}

	// Now ensure dependencies are present.
	if insterr := RunCommand(t,
		"yarn-install",
		withOptionalYarnFlags([]string{opts.YarnBin, "install", "--verbose"}), cwd, opts); insterr != nil {
		return "", insterr
	}
	for _, dependency := range opts.Dependencies {
		if linkerr := RunCommand(t,
			"yarn-link",
			withOptionalYarnFlags([]string{opts.YarnBin, "link", dependency}), cwd, opts); linkerr != nil {
			return "", linkerr
		}
	}

	// And finally compile it using whatever build steps are in the package.json file.
	if builderr := RunCommand(t,
		"yarn-build",
		withOptionalYarnFlags([]string{opts.YarnBin, "run", "build"}), cwd, opts); builderr != nil {
		return "", builderr
	}

	return dir, nil
}

func withOptionalYarnFlags(args []string) []string {
	flags := os.Getenv("YARNFLAGS")

	if flags != "" {
		return append(args, flags)
	}

	return args
}

type prefixer struct {
	writer    io.Writer
	prefix    []byte
	anyOutput bool
}

// newPrefixer wraps an io.Writer, prepending a fixed prefix after each \n emitting on the wrapped writer
func newPrefixer(writer io.Writer, prefix string) *prefixer {
	return &prefixer{writer, []byte(prefix), false}
}

var _ io.Writer = (*prefixer)(nil)

func (prefixer *prefixer) Write(p []byte) (int, error) {
	n := 0
	lines := bytes.SplitAfter(p, []byte{'\n'})
	for _, line := range lines {
		if len(line) > 0 {
			_, err := prefixer.writer.Write(prefixer.prefix)
			if err != nil {
				return n, err
			}
		}
		m, err := prefixer.writer.Write(line)
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}
