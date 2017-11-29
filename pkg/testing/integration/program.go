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
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// EditDir is an optional edit to apply to the example, as subsequent deployments.
type EditDir struct {
	Dir                    string
	ExtraRuntimeValidation func(t *testing.T, checkpoint stack.Checkpoint)
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
	ExtraRuntimeValidation func(t *testing.T, checkpoint stack.Checkpoint)
	// RelativeWorkDir is an optional path relative to `Dir` which should be used as working directory during tests.
	RelativeWorkDir string
	// Quick can be set to true to run a "quick" test that skips any non-essential steps (e.g., empty updates).
	Quick bool

	// ReportStats optionally specifies how to report results from the test for external collection.
	ReportStats TestStatsReporter

	// Stdout is the writer to use for all stdout messages.
	Stdout io.Writer
	// Stderr is the writer to use for all stderr messages.
	Stderr io.Writer
	// Verbose may be set to true to print messages as they occur, rather than buffering and showing upon failure.
	Verbose bool

	// DebugUpdates may be set to true to enable debug logging from `pulumi preview`, `pulumi update`, and
	// `pulumi destroy`.
	DebugUpdates bool

	// Bin is a location of a `pulumi` executable to be run.  Taken from the $PATH if missing.
	Bin string
	// YarnBin is a location of a `yarn` executable to be run.  Taken from the $PATH if missing.
	YarnBin string
}

// StackName returns a stack name to use for this test.
func (opts ProgramTestOptions) StackName() tokens.QName {
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

	return tokens.QName(strings.ToLower("p-it-" + host + "-" + test))
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
// All commands must return success return codes for the test to succeed.
func ProgramTest(t *testing.T, opts ProgramTestOptions) {
	t.Parallel()

	stackName := opts.StackName()
	dir, err := CopyTestToTemporaryDirectory(t, &opts, stackName)
	if !assert.NoError(t, err) {
		return
	}

	// If RelativeWorkDir is specified, apply that relative to the temp folder for use as working directory during tests.
	if opts.RelativeWorkDir != "" {
		dir = path.Join(dir, opts.RelativeWorkDir)
	}

	// Ensure all links are present, the stack is created, and all configs are applied.
	_, err = fmt.Fprintf(opts.Stdout, "Initializing project (dir %s; stack %s)\n", dir, stackName)
	contract.IgnoreError(err)
	if err = RunCommand(t, "pulumi-init",
		[]string{opts.Bin, "init"}, dir, opts); err != nil {
		return
	}
	if err = RunCommand(t, "pulumi-stack-init",
		[]string{opts.Bin, "stack", "init", string(stackName)}, dir, opts); err != nil {
		return
	}
	for key, value := range opts.Config {
		if err = RunCommand(t, "pulumi-config",
			[]string{opts.Bin, "config", "set", key, value}, dir, opts); err != nil {
			return
		}
	}

	for key, value := range opts.Secrets {
		if err = RunCommand(t, "pulumi-config",
			[]string{opts.Bin, "config", "set", "--secret", key, value}, dir, opts); err != nil {
			return
		}
	}

	preview := []string{opts.Bin, "preview"}
	update := []string{opts.Bin, "update"}
	destroy := []string{opts.Bin, "destroy", "--yes"}
	if opts.DebugUpdates {
		preview = append(preview, "-d")
		update = append(update, "-d")
		destroy = append(destroy, "-d")
	}

	// Now preview and update the real changes.
	_, err = fmt.Fprintf(opts.Stdout, "Performing primary preview and update\n")
	contract.IgnoreError(err)
	previewAndUpdate := func(d string, name string) error {
		if !opts.Quick {
			if preerr := RunCommand(t, "pulumi-preview-"+name,
				preview, d, opts); preerr != nil {
				return preerr
			}
		}
		if upderr := RunCommand(t, "pulumi-update-"+name,
			update, d, opts); upderr != nil {
			return upderr
		}
		return nil
	}

	// Perform the initial stack creation.
	initErr := previewAndUpdate(dir, "initial")

	// Ensure that before we exit, we attempt to destroy and remove the stack.
	defer func() {
		// Finally, tear down the stack, and clean up the stack.  Ignore errors to try to get as clean as possible.
		_, derr := fmt.Fprintf(opts.Stdout, "Destroying stack\n")
		contract.IgnoreError(derr)
		derr = RunCommand(t, "pulumi-destroy",
			destroy, dir, opts)
		contract.IgnoreError(derr)
		derr = RunCommand(t, "pulumi-stack-rm",
			[]string{opts.Bin, "stack", "rm", "--yes", string(stackName)}, dir, opts)
		contract.IgnoreError(derr)
	}()

	// If the initial preview/update failed, just exit without trying the rest (but make sure to destroy).
	if initErr != nil {
		return
	}

	// Perform an empty preview and update; nothing is expected to happen here.
	if !opts.Quick {
		_, err = fmt.Fprintf(opts.Stdout, "Performing empty preview and update (no changes expected)\n")
		contract.IgnoreError(err)
		if err = previewAndUpdate(dir, "empty"); err != nil {
			return
		}
	}

	// Run additional validation provided by the test options, passing in the checkpoint info.
	if opts.ExtraRuntimeValidation != nil {
		if err = performExtraRuntimeValidation(t, opts.ExtraRuntimeValidation, dir, stackName); err != nil {
			return
		}
	}

	// If there are any edits, apply them and run a preview and update for each one.
	for i, edit := range opts.EditDirs {
		_, err = fmt.Fprintf(opts.Stdout, "Applying edit '%v' and rerunning preview and update\n", edit)
		contract.IgnoreError(err)
		dir, err = prepareProject(t, stackName, edit.Dir, dir, opts)
		if !assert.NoError(t, err, "Expected to apply edit %v atop %v, but got an error %v", edit, dir, err) {
			return
		}
		if err = previewAndUpdate(dir, fmt.Sprintf("edit%d", i)); err != nil {
			return
		}
		if edit.ExtraRuntimeValidation != nil {
			if err = performExtraRuntimeValidation(t, edit.ExtraRuntimeValidation, dir, stackName); err != nil {
				return
			}
		}
	}
}

func performExtraRuntimeValidation(
	t *testing.T, extraRuntimeValidation func(t *testing.T, checkpoint stack.Checkpoint),
	dir string, stackName tokens.QName) error {
	// Load up the checkpoint file from .pulumi/stacks/<project-name>/<stack-name>.json.
	ws, err := workspace.NewProjectWorkspace(dir)
	if !assert.NoError(t, err, "expected to load project workspace at %v: %v", dir, err) {
		return err
	}
	chk, err := stack.GetCheckpoint(ws, stackName)
	if !assert.NoError(t, err, "expected to load checkpoint file for target %v: %v", stackName, err) {
		return err
	} else if !assert.NotNil(t, chk, "expected checkpoint file to be populated from %v: %v", stackName, err) {
		return errors.New("missing checkpoint")
	}
	extraRuntimeValidation(t, *chk)
	return nil
}

// CopyTestToTemporaryDirectory creates a temporary directory to run the test in and copies the test to it.
func CopyTestToTemporaryDirectory(t *testing.T, opts *ProgramTestOptions,
	stackName tokens.QName) (dir string, err error) {
	// Ensure the required programs are present.
	if opts.Bin == "" {
		var pulumi string
		pulumi, err = exec.LookPath("pulumi")
		if !assert.NoError(t, err, "Expected to find `pulumi` binary on $PATH: %v", err) {
			return dir, err
		}
		opts.Bin = pulumi
	}
	if opts.YarnBin == "" {
		var yarn string
		yarn, err = exec.LookPath("yarn")
		if !assert.NoError(t, err, "Expected to find `yarn` binary on $PATH: %v", err) {
			return dir, err
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

	_, err = fmt.Fprintf(opts.Stdout, "sample: %v\n", dir)
	contract.IgnoreError(err)
	_, err = fmt.Fprintf(opts.Stdout, "pulumi: %v\n", opts.Bin)
	contract.IgnoreError(err)
	_, err = fmt.Fprintf(opts.Stdout, "yarn: %v\n", opts.YarnBin)
	contract.IgnoreError(err)

	// Now copy the source project, excluding the .pulumi directory.
	dir, err = prepareProject(t, stackName, dir, "", *opts)
	if !assert.NoError(t, err, "Failed to copy source project %v to a new temp dir: %v", dir, err) {
		return dir, err
	}
	_, err = fmt.Fprintf(stdout, "projdir: %v\n", dir)
	contract.IgnoreError(err)
	return dir, err
}

// RunCommand executes the specified command and additional arguments, wrapping any output in the
// specialized test output streams that list the location the test is running in.
func RunCommand(t *testing.T, name string, args []string, wd string, opts ProgramTestOptions) error {
	path := args[0]
	command := strings.Join(args, " ")

	_, err := fmt.Fprintf(opts.Stdout, "**** Invoke '%v' in '%v'\n", command, wd)
	contract.IgnoreError(err)

	// Spawn a goroutine to print out "still running..." messages.
	finished := false
	go func() {
		for !finished {
			time.Sleep(30 * time.Second)
			if !finished {
				_, stillerr := fmt.Fprintf(opts.Stderr, "Still running command '%s' (%s)...\n", command, wd)
				contract.IgnoreError(stillerr)
			}
		}
	}()

	env := make([]string, 0, len(os.Environ())+2)

	for _, envEntry := range os.Environ() {
		// TODO(pulumi/pulumi#471) Force local execution now, but we'll have to do something better later
		if strings.HasPrefix(envEntry, "PULUMI_API=") {
			continue
		}

		env = append(env, envEntry)
	}

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
			StackName:      string(opts.StackName()),
			TestID:         wd,
			TestName:       filepath.Base(opts.Dir),
			IsError:        runerr != nil,
		})
	}

	finished = true
	if runerr != nil {
		_, err = fmt.Fprintf(opts.Stderr, "Invoke '%v' failed: %s\n", command, cmdutil.DetailedError(runerr))
		contract.IgnoreError(err)
		if !opts.Verbose {
			_, err = fmt.Fprintf(opts.Stderr, "%s\n", string(runout))
			contract.IgnoreError(err)
		}
	}
	assert.NoError(t, runerr, "Expected to successfully invoke '%v' in %v: %v", command, wd, runerr)
	return runerr
}

// prepareProject copies the source directory, src (excluding .pulumi), to a new temporary directory.  It then copies
// .pulumi/ and Pulumi.yaml from origin, if any, for edits.  The function returns the newly resulting directory.
func prepareProject(t *testing.T, stackName tokens.QName,
	src string, origin string, opts ProgramTestOptions) (string, error) {
	// Create a new temp directory.
	dir, err := ioutil.TempDir("", string(stackName)+"-")
	if err != nil {
		return "", err
	}

	// Now copy the source into it, ignoring .pulumi/ and Pulumi.yaml if there's an origin.
	wdir := workspace.BookkeepingDir
	proj := workspace.ProjectFile + ".yaml"
	excl := make(map[string]bool)
	if origin != "" {
		excl[wdir] = true
		excl[proj] = true
	}
	if copyerr := copyFile(dir, src, excl); copyerr != nil {
		return "", copyerr
	}

	// Now, copy back the original project's .pulumi/ and Pulumi.yaml atop the target.
	projfile := filepath.Join(dir, proj)
	if origin != "" {
		if copyerr := copyFile(projfile, filepath.Join(origin, proj), nil); copyerr != nil {
			return "", copyerr
		}
		if copyerr := copyFile(filepath.Join(dir, wdir), filepath.Join(origin, wdir), nil); copyerr != nil {
			return "", copyerr
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

// copyFile is a braindead simple function that copies a src file to a dst file.  Note that it is not general purpose:
// it doesn't handle symbolic links, it doesn't try to be efficient, it doesn't handle copies where src and dst overlap,
// and it makes no attempt to preserve file permissions.  It is what we need for this test package, no more, no less.
func copyFile(dst string, src string, excl map[string]bool) error {
	info, err := os.Lstat(src)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	} else if excl[info.Name()] {
		return nil
	}
	if info.IsDir() {
		// Recursively copy all files in a directory.
		files, err := ioutil.ReadDir(src)
		if err != nil {
			return err
		}
		for _, file := range files {
			name := file.Name()
			copyerr := copyFile(filepath.Join(dst, name), filepath.Join(src, name), excl)
			if copyerr != nil {
				return copyerr
			}
		}
	} else if info.Mode().IsRegular() {
		// Copy files by reading and rewriting their contents.  Skip symlinks and other special files.
		data, err := ioutil.ReadFile(src)
		if err != nil {
			return err
		}
		dstdir := filepath.Dir(dst)
		if err = os.MkdirAll(dstdir, 0700); err != nil {
			return err
		}
		if err = ioutil.WriteFile(dst, data, info.Mode()); err != nil {
			return err
		}
	}
	return nil
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
