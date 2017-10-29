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

// PulumiTest is a harness for making it easier to test the Pulumi CLI in a blackbox fashion.
// It creates a new temp directory, and provides routines for executing commands and writing
// their output to the test logs. (See pkg/testing/integration for a more opinionated approach.)
type PulumiTest struct {
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

// NewPulumiTest returns a new PulumiTest in a temp directory.
func NewPulumiTest(t *testing.T) *PulumiTest {
	testRoot, err := ioutil.TempDir("", "pulumi-test")
	assert.NoError(t, err, "creating temp directory")
	cwd := testRoot

	return &PulumiTest{
		T:        t,
		RootPath: testRoot,
		CWD:      cwd,
		Harness:  testLogWriter{"", t},
		Stdout:   testLogWriter{"    STDOUT", t},
		Stderr:   testLogWriter{"    STDERR", t},
	}
}

func (pt *PulumiTest) DeleteTestDirectory() {
	pt.Helper()
	err := os.RemoveAll(pt.RootPath)
	assert.NoError(pt, err, "error while cleaning up the test directory '%v'", pt.RootPath)
}

// RunCommand invokes the command-line argument in the working directory of the test. Writes its
// output to the test's buffers. Reports failure on non-zero exit code. Returns STDOUT, STDERR.
func (pt *PulumiTest) RunCommand(cmd string, args ...string) ([]string, []string) {
	pt.Helper()
	return pt.runCommandImpl(cmd, true, args...)
}

// RunCommandExpectError runs the command expecting a non-zero exit code.
func (pt *PulumiTest) RunCommandExpectError(cmd string, args ...string) ([]string, []string) {
	pt.Helper()
	return pt.runCommandImpl(cmd, false, args...)
}

func (pt *PulumiTest) runCommandImpl(command string, expectSuccess bool, args ...string) ([]string, []string) {
	pt.Helper()
	_, err := fmt.Fprintf(pt.Harness, "Running '%v' in %v\n", command, pt.CWD)
	contract.IgnoreError(err)

	// Spawn a goroutine to print out "still running..." messages.
	finished := false
	go func() {
		for !finished {
			time.Sleep(10 * time.Second)
			if !finished {
				_, stillErr := fmt.Fprintf(pt.Harness, "Still running command '%s'...\n", command)
				contract.IgnoreError(stillErr)
			}
		}
	}()

	// Buffer STDOUT and STDERR so we can later return them.
	var outBuffer bytes.Buffer
	var errBuffer bytes.Buffer

	cmd := exec.Command(command, args...)
	cmd.Dir = pt.CWD
	cmd.Stdout = io.MultiWriter(pt.Stdout, &outBuffer)
	cmd.Stderr = io.MultiWriter(pt.Stderr, &errBuffer)

	runErr := cmd.Run()
	finished = true
	if runErr != nil && expectSuccess {
		_, err = fmt.Fprintf(pt.Harness, "Command '%v' failed: %s", command, runErr)
		contract.IgnoreError(err)

		commandWithArgs := command
		if args != nil {
			commandWithArgs += " "
		}
		assert.NoError(pt, runErr, "Expected to successfully run '%v' in %v: %v", commandWithArgs, pt.CWD, runErr)
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
func (pt *PulumiTest) PathExists(relPath string) bool {
	path := path.Join(pt.CWD, relPath)
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

// GetSettings returns the contents of settings.json in the .pulumi folder. (Assumed to be)
// in the CWD.) In case of IO error, will fail the test.
func (pt *PulumiTest) GetSettings() workspace.Repository {
	if !pt.PathExists(".pulumi/settings.json") {
		pt.Fatalf("did not find .pulumi/settings.json")
	}

	path := path.Join(pt.CWD, ".pulumi/settings.json")
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		pt.Fatalf("error reading settings.json: %v", err)
	}

	var settings workspace.Repository
	err = json.Unmarshal(contents, &settings)
	if err != nil {
		pt.Fatalf("error unmarshalling JSON: %v", err)
	}

	return settings
}

// GetStacks returns the list of stacks and current stack by scraping `pulumi stack ls`.`
// Assumes .pulumi is in the current working directory.
func (pt *PulumiTest) GetStacks() ([]string, *string) {
	out, err := pt.RunCommand("pulumi", "stack", "ls")
	assert.Nil(pt, err, "expected nothing on stderr")

	var stackNames []string
	var currentStack *string
	if len(out) == 0 {
		pt.Fatalf("command didn't output as expected")
	}
	// Confirm header row matches.
	assert.Equal(pt, out[0], "NAME                 LAST UPDATE                                      RESOURCE COUNT")

	if len(out) >= 2 {
		stackSummaries := out[1:]
		for _, summary := range stackSummaries {
			if summary == "" {
				continue // Last line of stdout is "".
			}
			stackName := strings.TrimSpace(summary[:20])
			if strings.HasSuffix(stackName, "*") {
				currentStack = &stackName
				stackName = strings.TrimSuffix(stackName, "*")
			}
			stackNames = append(stackNames, stackName)
		}
	}

	return stackNames, currentStack
}

// assertConstainsSubstring asserts that msg is found somewhere in the provided strs array.
func assertConstainsSubstring(t *testing.T, strs []string, msg string) {
	t.Helper()
	for _, str := range strs {
		if strings.Contains(str, msg) {
			return
		}
	}
	t.Errorf("did not find '%v' in %v", msg, strs)
}

func createBasicPulumiRepo(pt *PulumiTest) {
	pt.RunCommand("git", "init")
	pt.RunCommand("pulumi", "init")

	contents := "name: pulumi-test\ndescription: a test\nruntime: nodejs\n"
	pulumiFile := path.Join(pt.CWD, "Pulumi.yaml")
	err := ioutil.WriteFile(pulumiFile, []byte(contents), os.ModePerm)
	assert.NoError(pt, err, "writing Pulumi.yaml file")
}
