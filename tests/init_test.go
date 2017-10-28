// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package tests

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPulumiInit(t *testing.T) {
	t.Run("SanityTest", func(test *testing.T) {
		t := NewPulumiTest(test)
		defer t.DeleteTestDirectory()

		// With a .git folder in the test root, `pulumi init` sets up shop there.
		t.RunCommand("git", "init")
		assert.False(t, t.PathExists(".pulumi"), "expecting no .pulumi folder yet")
		t.RunCommand("pulumi", "init")
		assert.True(t, t.PathExists(".pulumi"), "expecting .pulumi folder to be created")
	})

	t.Run("WalkUpToGitFolder", func(test *testing.T) {
		t := NewPulumiTest(test)
		defer t.DeleteTestDirectory()

		// Create a git repo in the root.
		t.RunCommand("git", "init")
		assert.True(t, t.PathExists(".git"), "expecting .git folder")

		// Create a subdirectory and CD into it.,
		subdir := path.Join(t.RootPath, "/foo/bar/baz/")
		err := os.MkdirAll(subdir, os.ModePerm)
		assert.NoError(t, err, "error creating subdirectory")
		t.CWD = subdir

		// Confirm we are in the new location (no .git folder found.)
		assert.False(t, t.PathExists(".git"), "expecting no .git folder (in new dir)")

		// pulumi init won't create the folder here, but rather along side .git.
		assert.False(t, t.PathExists(".pulumi"), "expecting no .pulumi folder")
		t.RunCommand("pulumi", "init")
		assert.False(t, t.PathExists(".pulumi"), "expecting no .pulumi folder. still")

		t.CWD = t.RootPath
		assert.True(t, t.PathExists(".pulumi"), "expecting .pulumi folder to be created")
	})

	t.Run("DefaultRepositoryInfo", func(test *testing.T) {
		t := NewPulumiTest(test)
		defer t.DeleteTestDirectory()

		t.RunCommand("git", "init")
		t.RunCommand("pulumi", "init")

		// Defaults
		settings := t.GetSettings()
		testRootName := path.Base(t.RootPath)
		assert.Equal(t, os.Getenv("USER"), settings.Owner)
		assert.Equal(t, testRootName, settings.Name)
		assert.Equal(t, "", settings.Root)
	})

	t.Run("ReadRemoteInfo", func(test *testing.T) {
		t := NewPulumiTest(test)
		defer t.DeleteTestDirectory()

		t.RunCommand("git", "init")
		t.RunCommand("git", "remote", "add", "not-origin", "git@github.com:moolumi/pasture.git")
		t.RunCommand("git", "remote", "add", "origin", "git@github.com:pulumi/pulumi-cloud.git")
		t.RunCommand("pulumi", "init")

		// We pick up the settings from "origin", not any other remote name.
		settings := t.GetSettings()
		assert.Equal(t, "pulumi", settings.Owner)
		assert.Equal(t, "pulumi-cloud", settings.Name)
		assert.Equal(t, "", settings.Root)
	})
}
