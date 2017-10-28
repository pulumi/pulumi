// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
	"github.com/stretchr/testify/assert"
)

// testLogWriter implements io.Writer writing data to a testcase log.
type testLogWriter struct {
	Suffix string
	t      *testing.T
}

func (lw testLogWriter) Write(b []byte) (int, error) {
	lw.t.Logf("%s: %s", lw.Suffix, string(b))
	return len(b), nil
}

// BlackboxTest creates a new temp directory, and provides routines for executing commands and
// writing their output to the test logs. This is a less opinionated test harness than
// integration.ProgramTest.
type BlackboxTest struct {
	*testing.T

	// RootPath is a new temp directory where you should place test files.
	RootPath string

	// Current working directory of the test. Commands are invoked from here.
	CWD string

	// Program output is written to STDOUT or STDERR. Diagnostic messages should be written to
	// Harness. But all are written to the underlying testing.T's log.
	Harness io.Writer
	Stdout  io.Writer
	Stderr  io.Writer
}

// NewBlackboxTest returns a new BlackboxTest in a temp directory.
func NewBlackboxTest(t *testing.T) *BlackboxTest {
	testRoot, err := ioutil.TempDir("", "pulumi-test")
	assert.NoError(t, err, "creating temp directory")
	cwd := testRoot

	return &BlackboxTest{
		T:        t,
		RootPath: testRoot,
		CWD:      cwd,
		Harness:  testLogWriter{"", t},
		Stdout:   testLogWriter{"    STDOUT", t},
		Stderr:   testLogWriter{"    STDERR", t},
	}
}

func (bt *BlackboxTest) DeleteTestDirectory() {
	bt.Helper()
	err := os.RemoveAll(bt.RootPath)
	assert.NoError(bt, err, "error while cleaning up the test directory '%v'", bt.RootPath)
}

// RunCommand invokes the command-line argument in the working directory of the test. Writes its
// output to the test's buffers. Reports failure on non-zero exit code. Returns STDOUT, STDERR.
func (bt *BlackboxTest) RunCommand(cmd string, args ...string) ([]string, []string) {
	bt.Helper()
	return bt.runCommandImpl(cmd, true, args...)
}

// RunCommandExpectError runs the command expecting a non-zero exit code.
func (bt *BlackboxTest) RunCommandExpectError(cmd string, args ...string) ([]string, []string) {
	bt.Helper()
	return bt.runCommandImpl(cmd, false, args...)
}

func (bt *BlackboxTest) runCommandImpl(command string, expectSuccess bool, args ...string) ([]string, []string) {
	bt.Helper()
	_, err := fmt.Fprintf(bt.Harness, "Running '%v' in %v\n", command, bt.CWD)
	contract.IgnoreError(err)

	// Spawn a goroutine to print out "still running..." messages.
	finished := false
	go func() {
		for !finished {
			time.Sleep(10 * time.Second)
			if !finished {
				_, stillErr := fmt.Fprintf(bt.Harness, "Still running command '%s'...\n", command)
				contract.IgnoreError(stillErr)
			}
		}
	}()

	// Buffer STDOUT and STDERR so we can later return them.
	var outBuffer bytes.Buffer
	var errBuffer bytes.Buffer

	cmd := exec.Command(command, args...)
	cmd.Dir = bt.CWD
	cmd.Stdout = io.MultiWriter(bt.Stdout, &outBuffer)
	cmd.Stderr = io.MultiWriter(bt.Stderr, &errBuffer)

	runErr := cmd.Run()
	finished = true
	if runErr != nil && expectSuccess {
		_, err = fmt.Fprintf(bt.Harness, "Command '%v' failed: %s", command, runErr)
		contract.IgnoreError(err)

		commandWithArgs := command
		if args != nil {
			commandWithArgs += " "
		}
		assert.NoError(bt, runErr, "Expected to successfully run '%v' in %v: %v", commandWithArgs, bt.CWD, runErr)
	}

	stdoutLines := strings.Split(outBuffer.String(), "\n")
	stderrLines := strings.Split(errBuffer.String(), "\n")
	// Returning nil makes handling the data easier on the consumer side.
	if len(stdoutLines) == 1 && stdoutLines[0] == "" {
		stdoutLines = nil
	}
	if len(stderrLines) == 1 && stderrLines[0] == "" {
		stderrLines = nil
	}
	return stdoutLines, stderrLines
}

// PathExists returns whether or not a file or directory exists relative to test's working directory.
func (bt *BlackboxTest) PathExists(relPath string) bool {
	path := path.Join(bt.CWD, relPath)
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

// readPulumiSettings returns the contents of settings.json in the .pulumi folder. (Assumed to be)
// in the CWD.) In case of IO error, will fail the test.
func readPulumiSettings(bt *BlackboxTest) workspace.Repository {
	if !bt.PathExists(".pulumi/settings.json") {
		bt.Fatalf("did not find .pulumi/settings.json")
	}

	path := path.Join(bt.CWD, ".pulumi/settings.json")
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		bt.Fatalf("error reading settings.json: %v", err)
	}

	var settings workspace.Repository
	err = json.Unmarshal(contents, &settings)
	if err != nil {
		bt.Fatalf("error unmarshalling JSON: %v", err)
	}

	return settings
}

func TestPulumiInit(t *testing.T) {
	t.Run("SanityTest", func(test *testing.T) {
		t := NewBlackboxTest(test)
		defer t.DeleteTestDirectory()

		// With a .git folder in the test root, `pulumi init` sets up shop there.
		t.RunCommand("git", "init")
		assert.False(t, t.PathExists(".pulumi"), "expecting no .pulumi folder yet")
		t.RunCommand("pulumi", "init")
		assert.True(t, t.PathExists(".pulumi"), "expecting .pulumi folder to be created")
	})

	t.Run("WalkUpToGitFolder", func(test *testing.T) {
		t := NewBlackboxTest(test)
		defer t.DeleteTestDirectory()

		// Create a git repo in the root.
		t.RunCommand("git", "init")
		assert.True(t, t.PathExists(".git"), "expecting .git folder")

		// Create a subdirectory and CD into it.,
		subdir := path.Join(t.RootPath, "/foo/bar/baz/")
		err := os.MkdirAll(subdir, os.ModePerm)
		assert.NoError(t, err, "error creating subdirectory")
		t.CWD = subdir

		// Confirm we are in the new location (no .git folder found.)
		assert.False(t, t.PathExists(".git"), "expecting no .git folder (in new dir)")

		// pulumi init won't create the folder here, but rather along side .git.
		assert.False(t, t.PathExists(".pulumi"), "expecting no .pulumi folder")
		t.RunCommand("pulumi", "init")
		assert.False(t, t.PathExists(".pulumi"), "expecting no .pulumi folder. still")

		t.CWD = t.RootPath
		assert.True(t, t.PathExists(".pulumi"), "expecting .pulumi folder to be created")
	})

	t.Run("DefaultRepositoryInfo", func(test *testing.T) {
		t := NewBlackboxTest(test)
		defer t.DeleteTestDirectory()

		t.RunCommand("git", "init")
		t.RunCommand("pulumi", "init")

		// Defaults
		settings := readPulumiSettings(t)
		testRootName := path.Base(t.RootPath)
		assert.Equal(t, os.Getenv("USER"), settings.Owner)
		assert.Equal(t, testRootName, settings.Name)
		assert.Equal(t, "", settings.Root)
	})

	t.Run("ReadRemoteInfo", func(test *testing.T) {
		t := NewBlackboxTest(test)
		defer t.DeleteTestDirectory()

		t.RunCommand("git", "init")
		t.RunCommand("git", "remote", "add", "not-origin", "git@github.com:moolumi/pasture.git")
		t.RunCommand("git", "remote", "add", "origin", "git@github.com:pulumi/pulumi-cloud.git")
		t.RunCommand("pulumi", "init")

		// We pick up the settings from "origin", not any other remote name.
		settings := readPulumiSettings(t)
		assert.Equal(t, "pulumi", settings.Owner)
		assert.Equal(t, "pulumi-cloud", settings.Name)
		assert.Equal(t, "", settings.Root)
	})
}
