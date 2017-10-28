// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStackWithoutInit(t *testing.T) {
	t.Run("SanityTest", func(test *testing.T) {
		t := NewPulumiTest(test)
		defer t.DeleteTestDirectory()

		stdout, stderr := t.RunCommandExpectError("pulumi", "stack", "ls")
		assert.Nil(t, stdout, "expected nothing to be written to stdout")
		assert.Contains(t, stderr, "error: no repository")
	})
}

func TestStackCommands(t *testing.T) {
	// stack init, stack ls, stack rm, stack ls
	t.Run("SanityTest", func(test *testing.T) {
		t := NewPulumiTest(test)
		defer t.DeleteTestDirectory()

		createBasicPulumiRepo(t)
		t.RunCommand("pulumi", "stack", "init", "foo")

		stacks, current := t.GetStacks()
		assert.Equal(t, 1, len(stacks))
		assert.NotNil(t, current)
		assert.Equal(t, "foo", *current)
		assert.Contains(t, stacks, "foo")

		t.RunCommand("pulumi", "stack", "rm", "foo", "--yes")

		stacks, _ = t.GetStacks()
		assert.Equal(t, 0, len(stacks))
	})

	t.Run("Errors", func(test *testing.T) {
		t := NewPulumiTest(test)
		defer t.DeleteTestDirectory()

		// Have not ran `pulumi init` yet.
		out, err := t.RunCommandExpectError("pulumi", "stack", "ls")
		assert.Nil(t, out)
		assertConstainsSubstring(t.T, err, "error: no repository")

		// Create git repo so the .pulumi folder doesn't wind up on a parent directory.
		// `pulumi stack ls` now fails because it cannot locate a Pulumi.yaml project file.
		t.RunCommand("git", "init")
		t.RunCommand("pulumi", "init")
		out, err = t.RunCommandExpectError("pulumi", "stack", "ls")
		assert.Nil(t, out)
		assertConstainsSubstring(t.T, err, "error: no Pulumi program found (or in any of the parent directories)")
	})

	t.Run("StackSelect", func(test *testing.T) {
		t := NewPulumiTest(test)
		defer t.DeleteTestDirectory()

		createBasicPulumiRepo(t)
		t.RunCommand("pulumi", "stack", "init", "blighttown")
		t.RunCommand("pulumi", "stack", "init", "majula")
		t.RunCommand("pulumi", "stack", "init", "lothric")

		// Last one created is always selected.
		_, current := t.GetStacks()
		assert.Equal(t, "lothric", *current)

		// Select works
		t.RunCommand("pulumi", "stack", "select", "blighttown")
		_, current = t.GetStacks()
		assert.Equal(t, "blighttown", *current)

		// Error
		out, err := t.RunCommandExpectError("pulumi", "stack", "select", "anor-londo")
		assert.Nil(t, out)
		assertConstainsSubstring(t.T, err, ".pulumi/stacks/pulumi-test/anor-londo.json: no such file or directory")
	})

	t.Run("StackRm", func(test *testing.T) {
		t := NewPulumiTest(test)
		defer t.DeleteTestDirectory()

		createBasicPulumiRepo(t)

		t.RunCommand("pulumi", "stack", "init", "blighttown")
		t.RunCommand("pulumi", "stack", "init", "majula")
		t.RunCommand("pulumi", "stack", "init", "lothric")
		stacks, _ := t.GetStacks()
		assert.Equal(t, 3, len(stacks))

		t.RunCommand("pulumi", "stack", "rm", "majula", "--yes")
		stacks, _ = t.GetStacks()
		assert.Equal(t, 2, len(stacks))
		assertConstainsSubstring(t.T, stacks, "blighttown")
		assertConstainsSubstring(t.T, stacks, "lothric")

		t.RunCommand("pulumi", "stack", "rm", "lothric", "--yes")
		stacks, _ = t.GetStacks()
		assert.Equal(t, 1, len(stacks))
		assertConstainsSubstring(t.T, stacks, "blighttown")

		t.RunCommand("pulumi", "stack", "rm", "blighttown", "--yes")
		stacks, _ = t.GetStacks()
		assert.Equal(t, 0, len(stacks))

		// Error
		out, err := t.RunCommandExpectError("pulumi", "stack", "rm", "anor-londo", "--yes")
		assert.Nil(t, out)
		assertConstainsSubstring(t.T, err, ".pulumi/stacks/pulumi-test/anor-londo.json: no such file or directory")
	})
}
