package tests

import (
	"encoding/json"
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
		e.RunCommand("yarn", "install")
		e.RunCommand("yarn", "link", "@pulumi/pulumi")
		_, currentStack := integration.GetStacks(e)
		stdout, _ := e.RunCommand("pulumi", "about", "--json")
		var res interface{}
		assert.Nil(t, json.Unmarshal([]byte(stdout), &res), "Should be valid json")
		assert.Contains(t, stdout, runtime.Version())
		assert.Contains(t, stdout, runtime.Compiler)
		assert.Containsf(t, stdout, "Current Stack: %s", *currentStack)
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
		e.RunCommand("yarn", "install")
		e.RunCommand("yarn", "link", "@pulumi/pulumi")
		e.RunCommand("pulumi", "stack", "init", stackName)
		e.RunCommand("pulumi", "up", "--non-interactive", "--yes", "--skip-preview")
		_, currentStack := integration.GetStacks(e)
		stdout, stderr := e.RunCommand("pulumi", "about")
		assert.Empty(t, stderr, "There should be no errors")
		assert.Contains(t, stdout, runtime.Version())
		assert.Contains(t, stdout, runtime.Compiler)
		assert.Containsf(t, stdout, "Current Stack: %s", *currentStack)
	})
}
