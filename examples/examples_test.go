package examples

import (
	"fmt"
	"go/build"
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

	fmt.Printf("sample: %v\n", examplewd)
	fmt.Printf("lumijs: %v\n", lumijs)
	fmt.Printf("lumi: %v\n", lumi)
	fmt.Printf("yarn: %v\n", yarn)

	runCmd(t, []string{yarn, "link", "@lumi/lumirt"}, examplewd)
	runCmd(t, []string{yarn, "link", "@lumi/lumi"}, examplewd)
	runCmd(t, []string{yarn, "link", "@lumi/aws"}, examplewd)
	runCmd(t, []string{lumijs, "--verbose"}, examplewd)
	runCmd(t, []string{lumi, "env", "init", "integrationtesting"}, examplewd)
	runCmd(t, []string{lumi, "config", "aws:config/index:region", region}, examplewd)
	runCmd(t, []string{lumi, "plan"}, examplewd)
	runCmd(t, []string{lumi, "deploy"}, examplewd)
	runCmd(t, []string{lumi, "destroy", "--yes"}, examplewd)
	runCmd(t, []string{lumi, "env", "rm", "--yes", "integrationtesting"}, examplewd)
}

func runCmd(t *testing.T, args []string, wd string) {
	path := args[0]
	command := strings.Join(args, " ")
	fmt.Printf("\n**** Invoke '%v' in %v\n", command, wd)
	cmd := exec.Cmd{
		Path:   path,
		Dir:    wd,
		Args:   args,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	err := cmd.Run()
	assert.NoError(t, err, "expected to successfully invoke '%v' in %v: %v", command, wd, err)
}
