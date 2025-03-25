// Copyright 2016-2024, Pulumi Corporation.
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
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/stretchr/testify/assert"
)

const (
	//nolint:gosec
	pulumiCredentialsPathEnvVar = "PULUMI_CREDENTIALS_PATH"
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
	// Set to true to use the local Pulumi dev build from ~/.pulumi-dev/bin/pulumi which get from `make install`
	UseLocalPulumiBuild bool
	// Content to pass on stdin, if any
	Stdin io.Reader
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
	assert.NoError(t, err, "creating temp directory")
	assert.NoError(t, WriteYarnRCForTest(root), "writing .yarnrc file")

	// We always use a clean PULUMI_HOME for each environment to avoid any potential conflicts with plugins or config.
	//nolint:usetesting // We control the lifecycle of the environment.
	home, err := os.MkdirTemp("", "test-env-home")
	assert.NoError(t, err, "creating temp PULUMI_HOME directory")

	t.Logf("Created new test environment:  %v", root)
	return &Environment{
		T:        t,
		HomePath: home,
		RootPath: root,
		CWD:      root,
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
		e.T.Fatalf("error importing directory: %v", err)
	}
}

// DeleteEnvironment deletes the environment's HomePath and RootPath, and everything underneath them.
func (e *Environment) DeleteEnvironment() {
	e.Helper()
	for _, path := range []string{e.HomePath, e.RootPath} {
		if err := os.RemoveAll(path); err != nil {
			// In CI, Windows sometimes lags behind in marking a resource
			// as unused. This causes otherwise passing tests to fail.
			// So ignore errors during cleanup.
			e.Logf("error cleaning up test directory %q: %v", path, err)
		}
	}
}

// DeleteEnvironment deletes the environment's HomePath and RootPath, and everything
// underneath them. It tolerates failing to delete the environment.
func (e *Environment) DeleteEnvironmentFallible() error {
	e.Helper()
	err1 := os.RemoveAll(e.HomePath)
	err2 := os.RemoveAll(e.RootPath)
	return errors.Join(err1, err2)
}

// DeleteIfNotFailed deletes the environment's HomePath and RootPath if the test hasn't failed. Otherwise
// keeps the files around for aiding debugging.
func (e *Environment) DeleteIfNotFailed() {
	if !e.T.Failed() {
		e.DeleteEnvironment()
	}
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
	// We don't want to time out on yarn installs.
	if cmd == "yarn" {
		YarnInstallMutex.Lock()
		defer YarnInstallMutex.Unlock()
	}

	if e.UseLocalPulumiBuild {
		home, err := os.UserHomeDir()
		if err != nil {
			e.Logf("Run Error: %v", err)
			e.Fatalf("Ran command %v args %v and expected success. Instead got failure.", cmd, args)
		}
		if home != "" {
			cmd = filepath.Join(home, ".pulumi-dev", "bin", "pulumi")
		}
	}

	e.Helper()
	stdout, stderr, err := e.GetCommandResults(cmd, args...)
	if err != nil {
		e.Logf("Run Error: %v", err)
		e.Logf("STDOUT: %v", stdout)
		e.Logf("STDERR: %v", stderr)
		e.Fatalf("Ran command %v args %v and expected success. Instead got failure.", cmd, args)
	}
	return stdout, stderr
}

// RunCommandExpectError runs the command expecting a non-zero exit code, returning stdout and stderr.
func (e *Environment) RunCommandExpectError(cmd string, args ...string) (string, string) {
	e.Helper()
	stdout, stderr, err := e.GetCommandResults(cmd, args...)
	if err == nil {
		e.Errorf("Ran command %v args %v and expected failure. Instead got success.", cmd, args)
		e.Logf("STDOUT: %v", stdout)
		e.Logf("STDERR: %v", stderr)
	}
	return stdout, stderr
}

// LocalURL returns a URL that uses the "fire and forget", storing its data inside the test folder (so multiple tests)
// may reuse stack names.
func (e *Environment) LocalURL() string {
	return "file://" + filepath.ToSlash(e.RootPath)
}

// GetCommandResults runs the given command and args in the Environments CWD, returning
// STDOUT, STDERR, and the result of os/exec.Command{}.Run.
func (e *Environment) GetCommandResults(command string, args ...string) (string, string, error) {
	return e.GetCommandResultsIn(e.CWD, command, args...)
}

// GetCommandResultsIn runs the given command and args in the given directory, returning
// STDOUT, STDERR, and the result of os/exec.Command{}.Run.
func (e *Environment) GetCommandResultsIn(dir string, command string, args ...string) (string, string, error) {
	e.T.Helper()
	e.T.Logf("Running command %v %v", command, strings.Join(args, " "))

	cmd := e.SetupCommandIn(dir, command, args...)

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
func (e *Environment) SetupCommandIn(dir string, command string, args ...string) *exec.Cmd {
	e.T.Helper()

	passphrase := "correct horse battery staple"
	if e.Passphrase != "" {
		passphrase = e.Passphrase
	}

	//nolint:gas
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	if e.Stdin != nil {
		cmd.Stdin = e.Stdin
	}
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", pulumiCredentialsPathEnvVar, e.RootPath))
	cmd.Env = append(cmd.Env, "PULUMI_DEBUG_COMMANDS=true")
	cmd.Env = append(cmd.Env, "PULUMI_HOME="+e.HomePath)
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
		e.T.Fatalf("error making directories for test file (%v): %v", filename, err)
	}

	if err := os.WriteFile(filename, []byte(contents), 0o600); err != nil {
		e.T.Fatalf("writing test file (%v): %v", filename, err)
	}
}
