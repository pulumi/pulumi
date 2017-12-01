// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package testing

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/util/fsutil"
	"github.com/stretchr/testify/assert"
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
}

// NewEnvironment returns a new Environment object, located in a temp directory.
func NewEnvironment(t *testing.T) *Environment {
	root, err := ioutil.TempDir("", "test-env")
	assert.NoError(t, err, "creating temp directory")

	t.Logf("Created new test environment:  %v", root)
	return &Environment{
		T:        t,
		RootPath: root,
		CWD:      root,
	}
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

// PathExists returns whether or not a file or directory exists relative to Environment's working directory.
func (e *Environment) PathExists(p string) bool {
	fullPath := path.Join(e.CWD, p)
	_, err := os.Stat(fullPath)
	return err == nil
}

// RunCommand invokes the command-line argument in the environment, returning stdout and stderr.
// Fails on non-zero exit code.
func (e *Environment) RunCommand(cmd string, args ...string) (string, string) {
	e.Helper()
	return runCommand(e.T, true, cmd, e.CWD, args...)
}

// RunCommandExpectError runs the command expecting a non-zero exit code.
func (e *Environment) RunCommandExpectError(cmd string, args ...string) (string, string) {
	e.Helper()
	return runCommand(e.T, false, cmd, e.CWD, args...)
}

func runCommand(t *testing.T, expectSuccess bool, command, cwd string, args ...string) (string, string) {
	t.Helper()
	t.Logf("Running command %v %v", command, strings.Join(args, " "))

	// Buffer STDOUT and STDERR so we can return them later.
	var outBuffer bytes.Buffer
	var errBuffer bytes.Buffer

	// nolint: gas
	cmd := exec.Command(command, args...)
	cmd.Dir = cwd
	cmd.Stdout = &outBuffer
	cmd.Stderr = &errBuffer

	runErr := cmd.Run()
	if (runErr == nil) != expectSuccess {
		t.Errorf("Finished with unexpected result. Expected success: %v, got error: %v", expectSuccess, runErr)
		t.Logf("STDOUT: %v", outBuffer.String())
		t.Logf("STDERR: %v", errBuffer.String())
	}
	return outBuffer.String(), errBuffer.String()
}
