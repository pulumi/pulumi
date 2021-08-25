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
		stdout, _ := e.RunCommand("pulumi", "about")
		assert.Contains(t, stdout, runtime.Version())
		assert.Contains(t, stdout, runtime.Compiler)
	})
}
