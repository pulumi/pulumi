package tests

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/stretchr/testify/assert"
)

func TestAboutCommands(t *testing.T) {
	stackName := "pulumi-about"

	// pulumi about --json

	t.Run("jsonAbout", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()
		integration.CreateBasicPulumiRepo(e)
		e.SetBackend(e.LocalURL())
		stdout, _ := e.RunCommand("pulumi", "about", "--json")
		var res interface{}
		assert.Nil(t, json.Unmarshal([]byte(stdout), &res), "Should be valid json")
		assert.Contains(t, stdout, runtime.Version())
		assert.Contains(t, stdout, runtime.Compiler)
		assert.Contains(t, stdout, "Failed to get information about the current stack:")
	})

	// pulumi about
	t.Run("plainAbout", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()
		integration.CreateBasicPulumiRepo(e)
		e.SetBackend(e.LocalURL())
		createIndexTs(e)
		e.RunCommand("pulumi", "stack", "init", stackName)
		e.RunCommand("yarn", "install")
		e.RunCommand("pulumi", "up", "--non-interactive", "--yes", "--skip-preview")
		_, currentStack := integration.GetStacks(e)
		stdout, stderr := e.RunCommand("pulumi", "about")
		assert.Empty(t, stderr, "There should be no errors")
		assert.Contains(t, stdout, runtime.Version())
		assert.Contains(t, stdout, runtime.Compiler)
		assert.Contains(t, stdout, fmt.Sprintf("Current Stack: %s", *currentStack))
	})
}

func createIndexTs(e *ptesting.Environment) {
	contents := ""
	filePath := "index.ts"
	filePath = path.Join(e.CWD, filePath)
	err := ioutil.WriteFile(filePath, []byte(contents), os.ModePerm)
	assert.NoError(e, err, "writing %s file", filePath)

}
