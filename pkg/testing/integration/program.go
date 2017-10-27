// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package integration

import (
	"bytes"
	"encoding/json"
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

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

const (
	testStackName = "integrationtesting"
)

// EditDir is an optional edit to apply to the example, as subsequent deployments.
type EditDir struct {
	Dir                    string
	ExtraRuntimeValidation func(t *testing.T, checkpoint stack.Checkpoint)
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

	// Stdout is the writer to use for all stdout messages.
	Stdout io.Writer
	// Stderr is the writer to use for all stderr messages.
	Stderr io.Writer

	// Bin is a location of a `pulumi` executable to be run.  Taken from the $PATH if missing.
	Bin string
	// YarnBin is a location of a `yarn` executable to be run.  Taken from the $PATH if missing.
	YarnBin string
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
	if overrides.EditDirs != nil {
		opts.EditDirs = overrides.EditDirs
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
//   pulumi config text <each opts.Config>
//   pulumi config secret <each opts.Secrets>
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

	dir, err := CopyTestToTemporaryDirectory(t, &opts)
	if !assert.NoError(t, err) {
		return
	}

	// Ensure all links are present, the stack is created, and all configs are applied.
	_, err = fmt.Fprintf(opts.Stdout, "Initializing project\n")
	contract.IgnoreError(err)
	RunCommand(t, []string{opts.Bin, "init"}, dir, opts)
	RunCommand(t, []string{opts.Bin, "stack", "init", testStackName}, dir, opts)
	for key, value := range opts.Config {
		RunCommand(t, []string{opts.Bin, "config", "text", key, value}, dir, opts)
	}

	for key, value := range opts.Secrets {
		RunCommand(t, []string{opts.Bin, "config", "secret", key, value}, dir, opts)
	}

	// Now preview and update the real changes.
	_, err = fmt.Fprintf(opts.Stdout, "Performing primary preview and update\n")
	contract.IgnoreError(err)
	previewAndUpdate := func(d string) {
		RunCommand(t, []string{opts.Bin, "preview"}, d, opts)
		RunCommand(t, []string{opts.Bin, "update"}, d, opts)
	}
	previewAndUpdate(dir)

	// Perform an empty preview and update; nothing is expected to happen here.
	_, err = fmt.Fprintf(opts.Stdout, "Performing empty preview and update (no changes expected)\n")
	contract.IgnoreError(err)
	previewAndUpdate(dir)

	// Run additional validation provided by the test options, passing in the
	if opts.ExtraRuntimeValidation != nil {
		err = performExtraRuntimeValidation(t, opts.ExtraRuntimeValidation, dir)
		if err != nil {
			return
		}
	}

	// If there are any edits, apply them and run a preview and update for each one.
	for _, edit := range opts.EditDirs {
		_, err = fmt.Fprintf(opts.Stdout, "Applying edit '%v' and rerunning preview and update\n", edit)
		contract.IgnoreError(err)
		dir, err = prepareProject(t, edit.Dir, dir, opts)
		if !assert.NoError(t, err, "Expected to apply edit %v atop %v, but got an error %v", edit, dir, err) {
			return
		}
		previewAndUpdate(dir)

		if edit.ExtraRuntimeValidation != nil {
			err = performExtraRuntimeValidation(t, edit.ExtraRuntimeValidation, dir)
			if err != nil {
				return
			}
		}
	}

	// Finally, tear down the stack, and clean up the stack.
	_, err = fmt.Fprintf(opts.Stdout, "Destroying stack\n")
	contract.IgnoreError(err)
	RunCommand(t, []string{opts.Bin, "destroy", "--yes"}, dir, opts)
	RunCommand(t, []string{opts.Bin, "stack", "rm", "--yes", testStackName}, dir, opts)
}

func performExtraRuntimeValidation(
	t *testing.T,
	extraRuntimeValidation func(t *testing.T, checkpoint stack.Checkpoint),
	dir string) (err error) {

	checkpointFile := path.Join(dir, workspace.BookkeepingDir, "stacks", filepath.Base(dir), testStackName+".json")
	var byts []byte
	byts, err = ioutil.ReadFile(checkpointFile)
	if !assert.NoError(t, err, "Expected to be able to read checkpoint file at %v: %v", checkpointFile, err) {
		return err
	}
	var checkpoint stack.Checkpoint
	err = json.Unmarshal(byts, &checkpoint)
	if !assert.NoError(t, err, "Expected to be able to deserialize checkpoint file at %v: %v", checkpointFile, err) {
		return err
	}
	extraRuntimeValidation(t, checkpoint)
	return nil
}

// CopyTestToTemporaryDirectory creates a temporary directory to run the test in and copies the test to it.
func CopyTestToTemporaryDirectory(t *testing.T, opts *ProgramTestOptions) (dir string, err error) {
	// Ensure the required programs are present.
	if opts.Bin == "" {
		var lumi string
		lumi, err = exec.LookPath("pulumi")
		if !assert.NoError(t, err, "Expected to find `pulumi` binary on $PATH: %v", err) {
			return dir, err
		}
		opts.Bin = lumi
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
	prefix := fmt.Sprintf("[ %30.30s ] ", dir[len(dir)-30:])
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
	dir, err = prepareProject(t, dir, "", *opts)
	if !assert.NoError(t, err, "Failed to copy source project %v to a new temp dir: %v", dir, err) {
		return dir, err
	}
	_, err = fmt.Fprintf(stdout, "projdir: %v\n", dir)
	contract.IgnoreError(err)
	return dir, err
}

// RunCommand executes the specified command and additional arguments, wrapping any output in the
// specialized test output streams that list the location the test is running in.
func RunCommand(t *testing.T, args []string, wd string, opts ProgramTestOptions) {
	path := args[0]
	command := strings.Join(args, " ")

	_, err := fmt.Fprintf(opts.Stdout, "\n**** Invoke '%v' in %v\n", command, wd)
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

	env := append(os.Environ(), "PULUMI_RETAIN_CHECKPOINTS=true")
	env = append(env, "PULUMI_CONFIG_PASSPHRASE=correct horse battery staple")

	cmd := exec.Cmd{
		Path:   path,
		Dir:    wd,
		Args:   args,
		Env:    env,
		Stdout: opts.Stdout,
		Stderr: opts.Stderr,
	}
	runerr := cmd.Run()
	finished = true
	if runerr != nil {
		_, err = fmt.Fprintf(opts.Stdout, "Invoke '%v' failed: %s\n", command, cmdutil.DetailedError(runerr))
		contract.IgnoreError(err)
	}
	assert.NoError(t, runerr, "Expected to successfully invoke '%v' in %v: %v", command, wd, runerr)
}

// prepareProject copies the source directory, src (excluding .pulumi), to a new temporary directory.  It then copies
// .pulumi/ and Pulumi.yaml from origin, if any, for edits.  The function returns the newly resulting directory.
func prepareProject(t *testing.T, src string, origin string, opts ProgramTestOptions) (string, error) {
	// Create a new temp directory.
	dir, err := ioutil.TempDir("", "lumi-integration-test-")
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
	if origin != "" {
		if copyerr := copyFile(filepath.Join(dir, proj), filepath.Join(origin, proj), nil); copyerr != nil {
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

	// Now ensure dependencies are present.
	RunCommand(t, withOptionalYarnFlags([]string{opts.YarnBin, "install", "--verbose"}), dir, opts)
	for _, dependency := range opts.Dependencies {
		RunCommand(t, withOptionalYarnFlags([]string{opts.YarnBin, "link", dependency}), dir, opts)
	}

	// And finally compile it using whatever build steps are in the package.json file.
	RunCommand(t, withOptionalYarnFlags([]string{opts.YarnBin, "run", "build"}), dir, opts)

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
