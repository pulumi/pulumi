package integration

import (
	"fmt"
	"go/build"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_WebServer(t *testing.T) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		t.Skipf("Skipping test due to missing AWS_REGION environment variable")
	}
	cwd, err := os.Getwd()
	assert.NoError(t, err, "expected a valid working directory: %v", err)
	examplewd := path.Join(cwd, "..", "..", "examples", "scenarios", "aws", "webserver")
	lumijs := path.Join(cwd, "..", "..", "cmd", "lumijs", "lumijs")
	lumisrc := path.Join(cwd, "..", "..", "cmd", "lumi")
	lumipkg, err := build.ImportDir(lumisrc, build.FindOnly)
	assert.NoError(t, err, "expected to find lumi package info: %v", err)
	lumi := path.Join(lumipkg.BinDir, "lumi")
	_, err = os.Stat(lumi)
	assert.NoError(t, err, "expected to find lumi binary: %v", err)

	fmt.Printf("sample: %v\n", examplewd)
	fmt.Printf("lumijs: %v\n", lumijs)
	fmt.Printf("lumi: %v\n", lumi)

	fmt.Printf("\n**** Invoke 'lumijs --verbose'\n")
	cmd := exec.Cmd{
		Path:   lumijs,
		Dir:    examplewd,
		Args:   []string{"", "--verbose"},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	err = cmd.Run()
	assert.NoError(t, err, "expected to successfully compile lumijs to Lumipack: %v", err)

	fmt.Printf("\n**** Invoke 'lumi env init integrationtesting'\n")
	cmd = exec.Cmd{
		Path:   lumi,
		Dir:    examplewd,
		Args:   []string{"", "env", "init", "integrationtesting"},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	err = cmd.Run()
	assert.NoError(t, err, "expected to successfully initialize Lumi environment: %v", err)

	fmt.Printf("\n**** Invoke 'lumi config aws:config/index:region %v'\n", region)
	cmd = exec.Cmd{
		Path:   lumi,
		Dir:    examplewd,
		Args:   []string{"", "config", "aws:config/index:region", region},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	err = cmd.Run()
	assert.NoError(t, err, "expected to successfully run `lumi config aws:config/index:region %v`: %v", region, err)

	fmt.Printf("\n**** Invoke 'lumi plan'\n")
	cmd = exec.Cmd{
		Path:   lumi,
		Dir:    examplewd,
		Args:   []string{"", "plan"},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	err = cmd.Run()
	assert.NoError(t, err, "expected to successfully run `lumi plan`: %v", err)

	fmt.Printf("\n**** Invoke 'lumi deploy'\n")
	cmd = exec.Cmd{
		Path:   lumi,
		Dir:    examplewd,
		Args:   []string{"", "deploy"},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	err = cmd.Run()
	assert.NoError(t, err, "expected to successfully run `lumi deploy`: %v", err)

	fmt.Printf("\n**** Invoke 'lumi destroy'\n")
	cmd = exec.Cmd{
		Path:   lumi,
		Dir:    examplewd,
		Args:   []string{"", "destroy", "--yes"},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	err = cmd.Run()
	assert.NoError(t, err, "expected to successfully run `lumi destroy`: %v", err)

	fmt.Printf("\n**** Invoke 'lumi env rm integrationtesting'\n")
	cmd = exec.Cmd{
		Path:   lumi,
		Dir:    examplewd,
		Args:   []string{"", "env", "rm", "--yes", "integrationtesting"},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	err = cmd.Run()
	assert.NoError(t, err, "expected to successfully remove Lumi environment: %v", err)

}
