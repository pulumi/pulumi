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

package testing

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

	"github.com/pulumi/pulumi/sdk/v3/go/common/tools"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/stretchr/testify/assert"
)

const (
	pulumiCredentialsPathEnvVar = "PULUMI_CREDENTIALS_PATH"
)

// Environment is an extension of the testing.T type that provides support for a test environment
// on the local disk. The Environment has a root directory (e.g. a newly created temp folder) and
// a current working directory (to virtually change directories).
type Environment struct {
	*testing.T

	// RootPath is a new temp directory where the environment starts.
	RootPath string
	// Current working directory.
	CWD string
	// Backend to use for commands
	Backend string
	// Environment variables to add to the environment for commands (`key=value`).
	Env []string
	// Passphrase for config secrets, if any
	Passphrase string

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
	return ioutil.WriteFile(
		filepath.Join(root, ".yarnrc"),
		[]byte("--mutex network\n--network-concurrency 1\n"), 0600)
}

// NewGoEnvironment returns a new Environment object, located in a GOPATH temp directory.
func NewGoEnvironment(t *testing.T) *Environment {
	testRoot, err := tools.CreateTemporaryGoFolder("test-env")
	if err != nil {
		t.Errorf("error creating test directory %s", err)
	}

	t.Logf("Created new go test environment")
	return &Environment{
		T:        t,
		RootPath: testRoot,
		CWD:      testRoot,
	}
}

// NewEnvironment returns a new Environment object, located in a temp directory.
func NewEnvironment(t *testing.T) *Environment {
	root, err := ioutil.TempDir("", "test-env")
	assert.NoError(t, err, "creating temp directory")
	assert.NoError(t, WriteYarnRCForTest(root), "writing .yarnrc file")

	t.Logf("Created new test environment:  %v", root)
	return &Environment{
		T:        t,
		RootPath: root,
		CWD:      root,
	}
}

// SetBackend sets the backend to use for commands in this environment.
func (e *Environment) SetBackend(backend string) {
	e.Backend = backend
}

// SetBackend sets the backend to use for commands in this environment.
func (e *Environment) SetEnvVars(env []string) {
	e.Env = env
}

// ImportDirectory copies a folder into the test environment.
func (e *Environment) ImportDirectory(path string) {
	err := fsutil.CopyFile(e.RootPath, path, nil)
	if err != nil {
		e.T.Fatalf("error importing directory: %v", err)
	}
}

// DeleteEnvironment deletes the environment's RootPath, and everything underneath it.
func (e *Environment) DeleteEnvironment() {
	e.Helper()
	err := os.RemoveAll(e.RootPath)
	assert.NoError(e, err, "cleaning up the test directory")
}

// DeleteIfNotFailed deletes the environment's RootPath if the test hasn't failed. Otherwise
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

// RunCommand runs the command expecting a zero exit code, returning stdout and stderr.
func (e *Environment) RunCommand(cmd string, args ...string) (string, string) {
	e.Helper()
	stdout, stderr, err := e.GetCommandResults(cmd, args...)
	if err != nil {
		e.Errorf("Ran command %v args %v and expected success. Instead got failure.", cmd, args)
		e.Logf("Run Error: %v", err)
		e.Logf("STDOUT: %v", stdout)
		e.Logf("STDERR: %v", stderr)
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
	return "file://" + e.RootPath
}

// GetCommandResults runs the given command and args in the Environments CWD, returning
// STDOUT, STDERR, and the result of os/exec.Command{}.Run.
func (e *Environment) GetCommandResults(command string, args ...string) (string, string, error) {
	e.T.Helper()
	e.T.Logf("Running command %v %v", command, strings.Join(args, " "))

	// Buffer STDOUT and STDERR so we can return them later.
	var outBuffer bytes.Buffer
	var errBuffer bytes.Buffer

	passphrase := "correct horse battery staple"
	if e.Passphrase != "" {
		passphrase = e.Passphrase
	}

	// nolint: gas
	cmd := exec.Command(command, args...)
	cmd.Dir = e.CWD
	if e.Stdin != nil {
		cmd.Stdin = e.Stdin
	}
	cmd.Stdout = &outBuffer
	cmd.Stderr = &errBuffer
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, e.Env...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", pulumiCredentialsPathEnvVar, e.RootPath))
	cmd.Env = append(cmd.Env, "PULUMI_DEBUG_COMMANDS=true")
	cmd.Env = append(cmd.Env, fmt.Sprintf("PULUMI_CONFIG_PASSPHRASE=%s", passphrase))
	if e.Backend != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("PULUMI_BACKEND_URL=%s", e.Backend))
	}

	runErr := cmd.Run()
	return outBuffer.String(), errBuffer.String(), runErr
}

// WriteTestFile writes a new test file relative to the Environment's CWD with the given contents.
// Aborts the underlying test on any errors.
func (e *Environment) WriteTestFile(filename string, contents string) {
	filename = filepath.Join(e.CWD, filename)

	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		e.T.Fatalf("error making directories for test file (%v): %v", filename, err)
	}

	if err := ioutil.WriteFile(filename, []byte(contents), os.ModePerm); err != nil {
		e.T.Fatalf("writing test file (%v): %v", filename, err)
	}
}
