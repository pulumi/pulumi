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
		_, currentStack := integration.GetStacks(e)
		stdout, stderr := e.RunCommand("pulumi", "about")
		assert.Empty(t, stderr, "We shouldn't print anything to stderr")
		assert.Contains(t, stdout, runtime.Version())
		assert.Contains(t, stdout, runtime.Compiler)
		assert.Containsf(t, stdout, "Current Stack: %s", *currentStack)
	})
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
		_, currentStack := integration.GetStacks(e)
		stdout, stderr := e.RunCommand("pulumi", "about", "--json")
		assert.Empty(t, stderr, "We shouldn't print anything to stderr")
		var res interface{}
		assert.Nil(t, json.Unmarshal([]byte(stdout), &res), "Should be valid json")
		assert.Contains(t, stdout, runtime.Version())
		assert.Contains(t, stdout, runtime.Compiler)
		assert.Containsf(t, stdout, "Current Stack: %s", *currentStack)
	})
}
