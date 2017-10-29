// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package tests

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/pulumi/pulumi/pkg/testing/integration"
	"github.com/stretchr/testify/assert"

	ptesting "github.com/pulumi/pulumi/pkg/testing"
)

func createBasicPulumiRepo(e *ptesting.Environment) {
	e.RunCommand("git", "init")
	e.RunCommand("pulumi", "init")

	contents := "name: pulumi-test\ndescription: a test\nruntime: nodejs\n"
	pulumiFile := path.Join(e.CWD, "Pulumi.yaml")
	err := ioutil.WriteFile(pulumiFile, []byte(contents), os.ModePerm)
	assert.NoError(e, err, "writing Pulumi.yaml file")
}

func TestStackWithoutInit(t *testing.T) {
	t.Run("SanityTest", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer e.DeleteEnvironment()

		stdout, stderr := e.RunCommandExpectError("pulumi", "stack", "ls")
		assert.Empty(t, stdout, "expected nothing to be written to stdout")
		assert.Contains(t, stderr, "error: no repository")
	})
}

func TestStackCommands(t *testing.T) {
	// stack init, stack ls, stack rm, stack ls
	t.Run("SanityTest", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer e.DeleteEnvironment()

		createBasicPulumiRepo(e)
		e.RunCommand("pulumi", "stack", "init", "foo")

		stacks, current := integration.GetStacks(e)
		assert.Equal(t, 1, len(stacks))
		assert.NotNil(t, current)
		assert.Equal(t, "foo", *current)
		assert.Contains(t, stacks, "foo")

		e.RunCommand("pulumi", "stack", "rm", "foo", "--yes")

		stacks, _ = integration.GetStacks(e)
		assert.Equal(t, 0, len(stacks))
	})

	t.Run("Errors", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer e.DeleteEnvironment()

		// Have not ran `pulumi init` yet.
		out, err := e.RunCommandExpectError("pulumi", "stack", "ls")
		assert.Empty(t, out)
		assert.Contains(t, err, "error: no repository")

		// Create git repo so the .pulumi folder doesn't wind up on a parent directory.
		// `pulumi stack ls` now fails because it cannot locate a Pulumi.yaml project file.
		e.RunCommand("git", "init")
		e.RunCommand("pulumi", "init")
		out, err = e.RunCommandExpectError("pulumi", "stack", "ls")
		assert.Empty(t, out)
		assert.Contains(t, err, "no Pulumi project file found, are you missing a Pulumi.yaml file?")
	})

	t.Run("StackSelect", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer e.DeleteEnvironment()

		createBasicPulumiRepo(e)
		e.RunCommand("pulumi", "stack", "init", "blighttown")
		e.RunCommand("pulumi", "stack", "init", "majula")
		e.RunCommand("pulumi", "stack", "init", "lothric")

		// Last one created is always selected.
		_, current := integration.GetStacks(e)
		assert.Equal(t, "lothric", *current)

		// Select works
		e.RunCommand("pulumi", "stack", "select", "blighttown")
		_, current = integration.GetStacks(e)
		assert.Equal(t, "blighttown", *current)

		// Error
		out, err := e.RunCommandExpectError("pulumi", "stack", "select", "anor-londo")
		assert.Empty(t, out)
		assert.Contains(t, err, ".pulumi/stacks/pulumi-test/anor-londo.json: no such file or directory")
	})

	t.Run("StackRm", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer e.DeleteEnvironment()

		createBasicPulumiRepo(e)

		e.RunCommand("pulumi", "stack", "init", "blighttown")
		e.RunCommand("pulumi", "stack", "init", "majula")
		e.RunCommand("pulumi", "stack", "init", "lothric")
		stacks, _ := integration.GetStacks(e)
		assert.Equal(t, 3, len(stacks))

		e.RunCommand("pulumi", "stack", "rm", "majula", "--yes")
		stacks, _ = integration.GetStacks(e)
		assert.Equal(t, 2, len(stacks))
		assert.Contains(t, stacks, "blighttown")
		assert.Contains(t, stacks, "lothric")

		e.RunCommand("pulumi", "stack", "rm", "lothric", "--yes")
		stacks, _ = integration.GetStacks(e)
		assert.Equal(t, 1, len(stacks))
		assert.Contains(t, stacks, "blighttown")

		e.RunCommand("pulumi", "stack", "rm", "blighttown", "--yes")
		stacks, _ = integration.GetStacks(e)
		assert.Equal(t, 0, len(stacks))

		// Error
		out, err := e.RunCommandExpectError("pulumi", "stack", "rm", "anor-londo", "--yes")
		assert.Empty(t, out)
		assert.Contains(t, err, ".pulumi/stacks/pulumi-test/anor-londo.json: no such file or directory")
	})
}
