// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package examples

import (
	"bytes"
	"fmt"
	"go/build"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Examples(t *testing.T) {
	var examples []string
	if testing.Short() {
		examples = []string{
			path.Join("scenarios", "aws", "serverless"),
		}
	} else {
		examples = []string{
			path.Join("scenarios", "aws", "serverless-raw"),
			path.Join("scenarios", "aws", "serverless"),
			path.Join("scenarios", "aws", "webserver"),
			path.Join("scenarios", "aws", "webserver-comp"),
			path.Join("scenarios", "aws", "beanstalk"),
			path.Join("scenarios", "aws", "minimal"),
		}
	}
	for _, ex := range examples {
		example := ex
		t.Run(example, func(t *testing.T) {
			testExample(t, example)
		})
	}
}

func testExample(t *testing.T, exampleDir string) {
	t.Parallel()
	region := os.Getenv("AWS_REGION")
	if region == "" {
		t.Skipf("Skipping test due to missing AWS_REGION environment variable")
	}
	cwd, err := os.Getwd()
	if !assert.NoError(t, err, "expected a valid working directory: %v", err) {
		return
	}
	examplewd := path.Join(cwd, exampleDir)
	lumijs := path.Join(cwd, "..", "cmd", "lumijs", "lumijs")
	lumisrc := path.Join(cwd, "..", "cmd", "lumi")
	lumipkg, err := build.ImportDir(lumisrc, build.FindOnly)
	if !assert.NoError(t, err, "expected to find lumi package info: %v", err) {
		return
	}
	lumi := path.Join(lumipkg.BinDir, "lumi")
	_, err = os.Stat(lumi)
	if !assert.NoError(t, err, "expected to find lumi binary: %v", err) {
		return
	}
	yarn, err := exec.LookPath("yarn")
	if !assert.NoError(t, err, "expected to find yarn binary: %v", err) {
		return
	}

	prefix := fmt.Sprintf("[%30.30s ] ", exampleDir)
	stdout := newPrefixer(os.Stdout, prefix)
	stderr := newPrefixer(os.Stderr, prefix)

	fmt.Fprintf(stdout, "sample: %v\n", examplewd)
	fmt.Fprintf(stdout, "lumijs: %v\n", lumijs)
	fmt.Fprintf(stdout, "lumi: %v\n", lumi)
	fmt.Fprintf(stdout, "yarn: %v\n", yarn)

	runCmd(t, []string{yarn, "link", "@lumi/lumirt"}, examplewd, stdout, stderr)
	runCmd(t, []string{yarn, "link", "@lumi/lumi"}, examplewd, stdout, stderr)
	runCmd(t, []string{yarn, "link", "@lumi/aws"}, examplewd, stdout, stderr)
	runCmd(t, []string{lumijs, "--verbose"}, examplewd, stdout, stderr)
	runCmd(t, []string{lumi, "env", "init", "integrationtesting"}, examplewd, stdout, stderr)
	runCmd(t, []string{lumi, "config", "aws:config/index:region", region}, examplewd, stdout, stderr)
	runCmd(t, []string{lumi, "plan"}, examplewd, stdout, stderr)
	runCmd(t, []string{lumi, "deploy"}, examplewd, stdout, stderr)
	runCmd(t, []string{lumi, "destroy", "--yes"}, examplewd, stdout, stderr)
	runCmd(t, []string{lumi, "env", "rm", "--yes", "integrationtesting"}, examplewd, stdout, stderr)
}

func runCmd(t *testing.T, args []string, wd string, stdout, stderr io.Writer) {
	path := args[0]
	command := strings.Join(args, " ")
	fmt.Fprintf(stdout, "\n**** Invoke '%v' in %v\n", command, wd)
	cmd := exec.Cmd{
		Path:   path,
		Dir:    wd,
		Args:   args,
		Stdout: stdout,
		Stderr: stderr,
	}
	err := cmd.Run()
	assert.NoError(t, err, "expected to successfully invoke '%v' in %v: %v", command, wd, err)
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

func Test_Prefixer(t *testing.T) {
	byts := make([]byte, 0, 1000)
	buf := bytes.NewBuffer(byts)
	prefixer := newPrefixer(buf, "OK: ")
	prefixer.Write([]byte("\nsadsada\n\nasdsadsa\nasdsadsa\n"))
	assert.Equal(t, []byte("OK: \nOK: sadsada\nOK: \nOK: asdsadsa\nOK: asdsadsa\n"), buf.Bytes())
}
