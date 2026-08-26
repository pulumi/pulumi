// Copyright 2016, Pulumi Corporation.
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

package testing

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/stretchr/testify/require"
)

const (
	//nolint:gosec
	pulumiCredentialsPathEnvVar = "PULUMI_CREDENTIALS_PATH"
	pulumiBinaryPathEnvVar      = "PULUMI_INTEGRATION_BINARY_PATH"
)

// Environment is an extension of the testing.T type that provides support for a test environment
// on the local disk. The Environment has a root directory (e.g. a newly created temp folder) and
// a current working directory (to virtually change directories).
type Environment struct {
	*testing.T

	// HomePath is the PULUMI_HOME directory for the environment
	HomePath string
	// RootPath is a new temp directory where the environment starts.
	RootPath string
	// Current working directory, defaults to the root path.
	CWD string
	// Backend to use for commands
	Backend string
	// Environment variables to add to the environment for commands (`key=value`).
	Env []string
	// Passphrase for config secrets, if any
	Passphrase string
	// Set to true to turn off setting PULUMI_CONFIG_PASSPHRASE.
	NoPassphrase bool
	// Content to pass on stdin, if any
	Stdin io.Reader

	// Stacks created through StackInit, to be removed when the environment is torn down.
	stacks []trackedStack
}

// trackedStack pairs a stack with the directory it was created in, which is what lets the CLI
// resolve an unqualified stack name back to its project.
type trackedStack struct {
	name    string
	dir     string
	backend string
}

// ForceStackRemovalEnvVar removes a stack whose destroy failed. Only set it where the resources it
// still manages are not real; against a real provider it orphans cloud infrastructure.
const ForceStackRemovalEnvVar = "PULUMI_TEST_FORCE_STACK_REMOVAL"

// ForceStackRemoval reports whether a stack should be removed even if its destroy failed.
func ForceStackRemoval() bool {
	return cmdutil.IsTruthy(os.Getenv(ForceStackRemovalEnvVar))
}

// WriteYarnRCForTest writes a .yarnrc file which sets global configuration for every yarn inovcation. We use this
// to work around some test issues we see in Travis.
func WriteYarnRCForTest(root string) error {
	// Write a .yarnrc file to pass --mutex network to all yarn invocations, since tests
	// may run concurrently and yarn may fail if invoked concurrently
	// https://github.com/yarnpkg/yarn/issues/683
	// Also add --network-concurrency 1 since we've been seeing
	// https://github.com/yarnpkg/yarn/issues/4563 as well
	return os.WriteFile(
		filepath.Join(root, ".yarnrc"),
		[]byte("--mutex network\n--network-concurrency 1\n"), 0o600)
}

// NewEnvironment returns a new Environment object, located in a temp directory.
func NewEnvironment(t *testing.T) *Environment {
	//nolint:usetesting // We control the lifecycle of the environment.
	root, err := os.MkdirTemp("", "test-env")
	require.NoError(t, err, "creating temp directory")
	require.NoError(t, WriteYarnRCForTest(root), "writing .yarnrc file")

	// We always use a clean PULUMI_HOME for each environment to avoid any potential conflicts with plugins or config.
	//nolint:usetesting // We control the lifecycle of the environment.
	home, err := os.MkdirTemp("", "test-env-home")
	require.NoError(t, err, "creating temp PULUMI_HOME directory")

	t.Logf("Created new test environment:  %v", root)
	e := &Environment{
		T:        t,
		HomePath: home,
		RootPath: root,
		CWD:      root,
	}
	// Backstop for tests that call neither DeleteEnvironment nor DeleteIfNotFailed; both of those
	// remove the tracked stacks themselves.
	t.Cleanup(e.RemoveStacks)
	return e
}

// StackInit runs `pulumi stack init` and removes the stack when the environment is torn down.
// Prefer it to a bare `stack init`, which leaks whenever an assertion aborts the test before the
// trailing `stack rm`.
func (e *Environment) StackInit(stackName string, args ...string) (string, string) {
	e.Helper()
	// Only once creation succeeds: a name that was already taken belongs to someone else.
	stdout, stderr := e.RunCommand("pulumi", append([]string{"stack", "init", stackName}, args...)...)
	e.RemoveStackOnExit(stackName)
	return stdout, stderr
}

// RemoveStackOnExit arranges for the named stack to be removed when the environment is torn down.
// Use it for stacks created by something other than StackInit, such as `pulumi new`.
func (e *Environment) RemoveStackOnExit(stackName string) {
	e.stacks = append(e.stacks, trackedStack{name: stackName, dir: e.CWD, backend: e.Backend})
}

// RemoveStacks destroys, then removes, the stacks tracked by StackInit and RemoveStackOnExit. It is
// idempotent, and logs rather than fails, since it runs during teardown.
func (e *Environment) RemoveStacks() {
	e.Helper()
	stacks, backend := e.stacks, e.Backend
	e.stacks = nil
	defer func() { e.Backend = backend }()

	for _, stack := range stacks {
		// The environment's backend may have moved on since the stack was created.
		e.Backend = stack.backend
		e.removeStack(stack)
	}
}

func (e *Environment) removeStack(stack trackedStack) {
	e.Helper()
	// Not e.Context(): teardown can run as a t.Cleanup, by which point it is already canceled.
	ctx := context.Background()

	run := func(args ...string) (string, error) {
		args = append(args, "--stack", stack.name)
		_, stderr, err := e.getCommandResultsIn(ctx, stack.dir, "pulumi", args...)
		return stderr, err
	}

	destroyed := true
	if stderr, err := run("destroy", "--yes", "--skip-preview", "--non-interactive"); err != nil {
		if strings.Contains(stderr, "no stack named") {
			return
		}
		e.Logf("error destroying stack %q: %v: %s", stack.name, err, stderr)
		destroyed = false
	}

	rm := []string{"stack", "rm", "--yes"}
	if destroyed || ForceStackRemoval() {
		rm = append(rm, "--force")
	}
	if stderr, err := run(rm...); err != nil {
		e.Logf("error removing stack %q, it has leaked: %v: %s", stack.name, err, stderr)
	}
}

// SetBackend sets the backend to use for commands in this environment.
func (e *Environment) SetBackend(backend string) {
	e.Backend = backend
}

// SetEnvVars appends to the list of environment variables.
// According to https://pkg.go.dev/os/exec#Cmd.Env:
//
//	If Env contains duplicate environment keys, only the last
//	value in the slice for each duplicate key is used.
//
// So later values take precedence.
func (e *Environment) SetEnvVars(env ...string) {
	e.Env = append(e.Env, env...)
}

// ImportDirectory copies a folder into the test environment.
func (e *Environment) ImportDirectory(path string) {
	err := fsutil.CopyFile(e.CWD, path, nil)
	if err != nil {
		e.Fatalf("error importing directory: %v", err)
	}
}

// DeleteEnvironment deletes the environment's HomePath and RootPath, and everything underneath them.
func (e *Environment) DeleteEnvironment() {
	e.Helper()
	// Stacks first: removing them needs the credentials and plugins under these directories.
	e.RemoveStacks()
	for _, path := range []string{e.HomePath, e.RootPath} {
		if err := os.RemoveAll(path); err != nil {
			// In CI, Windows sometimes lags behind in marking a resource
			// as unused. This causes otherwise passing tests to fail.
			// So ignore errors during cleanup.
			e.Logf("error cleaning up test directory %q: %v", path, err)
		}
	}
}

// DeleteIfNotFailed deletes the environment's HomePath and RootPath if the test hasn't failed. Otherwise
// keeps the files around for aiding debugging.
func (e *Environment) DeleteIfNotFailed() {
	if !e.Failed() {
		e.DeleteEnvironment()
		return
	}
	// The files stay for debugging, but the stacks must not: they are never reclaimed.
	e.RemoveStacks()
}

// PathExists returns whether or not a file or directory exists relative to Environment's working directory.
func (e *Environment) PathExists(p string) bool {
	fullPath := path.Join(e.CWD, p)
	_, err := os.Stat(fullPath)
	return err == nil
}

var YarnInstallMutex sync.Mutex

// RunCommand runs the command expecting a zero exit code, returning stdout and stderr.
func (e *Environment) RunCommand(cmd string, args ...string) (string, string) {
	e.Helper()
	return e.runCommand(e.Context(), cmd, args...)
}

// RunTestCommand is like [*Environment.RunTestCommand], but it kills the command if the test fails.
func (e *Environment) RunTestCommand(cmd string, args ...string) (string, string) {
	e.Helper()
	ctx, cancelWithCause := context.WithCancelCause(e.Context())
	cancel := cancelOnFail(e, cancelWithCause)
	defer cancel()
	return e.runCommand(ctx, cmd, args...)
}

func (e *Environment) runCommand(ctx context.Context, cmd string, args ...string) (string, string) {
	e.Helper()
	// We don't want to time out on yarn installs.
	if cmd == "yarn" {
		YarnInstallMutex.Lock()
		defer YarnInstallMutex.Unlock()
	}

	e.Helper()
	stdout, stderr, err := e.getCommandResults(ctx, cmd, args...)
	if err != nil {
		e.Logf("Run Error: %v", err)
		e.Logf("STDOUT: %v", stdout)
		e.Logf("STDERR: %v", stderr)
		e.Fatalf("Ran command %v args %v and expected success. Instead got failure.", cmd, args)
	}
	return stdout, stderr
}

func (e *Environment) RunCommandWithRetry(cmd string, args ...string) (string, string) {
	// We don't want to time out on yarn installs.
	if cmd == "yarn" {
		YarnInstallMutex.Lock()
		defer YarnInstallMutex.Unlock()
	}

	e.Helper()
	var stdout, stderr string
	var err error
	for i := range 3 {
		stdout, stderr, err = e.GetCommandResults(cmd, args...)
		if err == nil {
			return stdout, stderr
		}
		e.Logf("Run Error: %v", err)
		e.Logf("STDOUT: %v", stdout)
		e.Logf("STDERR: %v", stderr)
		if i == 2 {
			e.Logf("Giving up after 3 retries.")
		} else {
			e.Logf("Retrying command %v args %v (%d/3)", cmd, args, i+1)
		}
	}
	if err != nil {
		e.Fatalf("Ran command %v args %v and expected success. Instead got failure after 3 retries.", cmd, args)
	}
	return stdout, stderr
}

// RunCommandExpectError runs the command expecting a non-zero exit code, returning stdout and stderr.
func (e *Environment) RunCommandExpectError(cmd string, args ...string) (string, string) {
	stdout, stderr, _ := e.RunCommandReturnExpectedError(cmd, args...)
	return stdout, stderr
}

// Same as RunCommandExpectError but returns the error.
func (e *Environment) RunCommandReturnExpectedError(cmd string, args ...string) (string, string, error) {
	e.Helper()
	stdout, stderr, err := e.GetCommandResults(cmd, args...)
	if err == nil {
		e.Errorf("Ran command %v args %v and expected failure. Instead got success.", cmd, args)
		e.Logf("STDOUT: %v", stdout)
		e.Logf("STDERR: %v", stderr)
	}
	return stdout, stderr, err
}

// LocalURL returns a URL that uses the "fire and forget", storing its data inside the test folder (so multiple tests)
// may reuse stack names.
func (e *Environment) LocalURL() string {
	return "file://" + filepath.ToSlash(e.RootPath)
}

// GetCommandResults runs the given command and args in the Environments CWD, returning
// STDOUT, STDERR, and the result of os/exec.Command{}.Run.
func (e *Environment) GetCommandResults(command string, args ...string) (string, string, error) {
	e.Helper()
	return e.getCommandResults(e.Context(), command, args...)
}

func (e *Environment) getCommandResults(ctx context.Context, command string, args ...string) (string, string, error) {
	e.Helper()
	return e.getCommandResultsIn(ctx, e.CWD, command, args...)
}

// GetCommandResultsIn runs the given command and args in the given directory, returning
// STDOUT, STDERR, and the result of os/exec.Command{}.Run.
func (e *Environment) GetCommandResultsIn(dir string, command string, args ...string) (string, string, error) {
	e.Helper()
	return e.getCommandResultsIn(e.Context(), dir, command, args...)
}

func (e *Environment) getCommandResultsIn(
	ctx context.Context, dir string, command string, args ...string,
) (string, string, error) {
	e.Helper()

	cmd := e.SetupCommandIn(ctx, dir, command, args...)
	e.Logf("Running command %v %v", cmd.Path, strings.Join(args, " "))

	// Buffer STDOUT and STDERR so we can return them later.
	var outBuffer bytes.Buffer
	var errBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	cmd.Stderr = &errBuffer

	runErr := cmd.Run()
	return outBuffer.String(), errBuffer.String(), runErr
}

// SetupCommandIn creates a new exec.Cmd that's ready to run in the given
// directory, with the given command and args.
func (e *Environment) SetupCommandIn(ctx context.Context, dir string, command string, args ...string) *exec.Cmd {
	e.Helper()

	passphrase := "correct horse battery staple"
	if e.Passphrase != "" {
		passphrase = e.Passphrase
	}

	if command == "pulumi" {
		command = e.resolvePulumiPath()
	}

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	if e.Stdin != nil {
		cmd.Stdin = e.Stdin
	}
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", pulumiCredentialsPathEnvVar, e.RootPath))
	cmd.Env = append(cmd.Env, "PULUMI_DEBUG_COMMANDS=true")
	cmd.Env = append(cmd.Env, "PULUMI_HOME="+e.HomePath)
	if coverdir := os.Getenv("PULUMI_GOCOVERDIR"); coverdir != "" {
		cmd.Env = append(cmd.Env, "GOCOVERDIR="+coverdir)
	}
	if !e.NoPassphrase {
		cmd.Env = append(cmd.Env, "PULUMI_CONFIG_PASSPHRASE="+passphrase)
	}
	if e.Backend != "" {
		cmd.Env = append(cmd.Env, "PULUMI_BACKEND_URL="+e.Backend)
	}
	// According to https://pkg.go.dev/os/exec#Cmd.Env:
	//     If Env contains duplicate environment keys, only the last
	//     value in the slice for each duplicate key is used.
	// By putting `append e.Env` last, we allow our users to override variables we include.
	cmd.Env = append(cmd.Env, e.Env...)

	return cmd
}

// WriteTestFile writes a new test file relative to the Environment's CWD with the given contents.
// Aborts the underlying test on any errors.
func (e *Environment) WriteTestFile(filename string, contents string) {
	filename = filepath.Join(e.CWD, filename)

	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		e.Fatalf("error making directories for test file (%v): %v", filename, err)
	}

	if err := os.WriteFile(filename, []byte(contents), 0o600); err != nil {
		e.Fatalf("writing test file (%v): %v", filename, err)
	}
}

func (e *Environment) resolvePulumiPath() string {
	e.Helper()
	if pulumiPath, isSet := os.LookupEnv(pulumiBinaryPathEnvVar); isSet {
		return pulumiPath
	}
	pulumiPath, err := exec.LookPath("pulumi")
	if err == nil {
		return pulumiPath
	}
	if !os.IsNotExist(err) {
		e.Logf("error locating pulumi binary from path: %v", err)
	}
	return "pulumi"
}
