package integrationtesting

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

// LumiProgramTestOptions provides options for LumiProgramTest
type LumiProgramTestOptions struct {
	// Array of NPM packages which must be `yarn linked` (e.g. {"@lumi/lumi", "@lumi/aws"})
	Dependencies []string
	// Map of config keys and values to set on the Lumi environment (e.g. {"aws:config:region": "us-east-2"})
	Config map[string]string
}

// LumiProgramTest runs a lifecylce of Lumi commands in a Lumi program working directory.
// Uses the `lumijs`, `lumi`, and `yarn` binaries available on PATH. Executes the following
// workflow:
//   yarn link <each options.Depencies>
//   lumijs --verbose
//   lumi env init integrationtesting
//   lumi config <each options.Config>
//   lumi plan (currently being skipped due to pulumi/pulumi-fabric#276)
//   lumi deploy
//   lumi destroy --yes
//   lumi env rm --yes integrationtesting
// All commands must return success return codes for the test to succeed.
func LumiProgramTest(t *testing.T, programDir string, options LumiProgramTestOptions) {
	t.Parallel()
	lumijs, err := exec.LookPath("lumijs")
	if !assert.NoError(t, err, "expected to find lumijs binary: %v", err) {
		return
	}
	lumi, err := exec.LookPath("lumi")
	if !assert.NoError(t, err, "expected to find lumi binary: %v", err) {
		return
	}
	yarn, err := exec.LookPath("yarn")
	if !assert.NoError(t, err, "expected to find yarn binary: %v", err) {
		return
	}

	prefix := fmt.Sprintf("[ %30.30s ] ", programDir[len(programDir)-30:])
	stdout := newPrefixer(os.Stdout, prefix)
	stderr := newPrefixer(os.Stderr, prefix)

	_, err = fmt.Fprintf(stdout, "sample: %v\n", programDir)
	contract.Assert(err == nil)
	_, err = fmt.Fprintf(stdout, "lumijs: %v\n", lumijs)
	contract.Assert(err == nil)
	_, err = fmt.Fprintf(stdout, "lumi: %v\n", lumi)
	contract.Assert(err == nil)
	_, err = fmt.Fprintf(stdout, "yarn: %v\n", yarn)
	contract.Assert(err == nil)

	for _, dependency := range options.Dependencies {
		runCmd(t, []string{yarn, "link", dependency}, programDir, stdout, stderr)
	}
	runCmd(t, []string{lumijs, "--verbose"}, programDir, stdout, stderr)
	runCmd(t, []string{lumi, "env", "init", "integrationtesting"}, programDir, stdout, stderr)
	for key, value := range options.Config {
		runCmd(t, []string{lumi, "config", key, value}, programDir, stdout, stderr)
	}
	// TODO[pulumi/pulumi-fabric#276] Disabling `lumi plan` for now as it has issues with output properties
	// runCmd(t, []string{lumi, "plan"}, programDir, stdout, stderr)
	runCmd(t, []string{lumi, "deploy"}, programDir, stdout, stderr)
	runCmd(t, []string{lumi, "destroy", "--yes"}, programDir, stdout, stderr)
	runCmd(t, []string{lumi, "env", "rm", "--yes", "integrationtesting"}, programDir, stdout, stderr)
}

func runCmd(t *testing.T, args []string, wd string, stdout, stderr io.Writer) {
	path := args[0]
	command := strings.Join(args, " ")
	_, err := fmt.Fprintf(stdout, "\n**** Invoke '%v' in %v\n", command, wd)
	contract.Assert(err == nil)
	cmd := exec.Cmd{
		Path:   path,
		Dir:    wd,
		Args:   args,
		Stdout: stdout,
		Stderr: stderr,
	}
	err = cmd.Run()
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
