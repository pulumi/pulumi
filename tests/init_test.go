// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package tests

import (
	"os"
	"path"
	"testing"

	ptesting "github.com/pulumi/pulumi/pkg/testing"
	"github.com/pulumi/pulumi/pkg/testing/integration"
	"github.com/stretchr/testify/assert"
)

func TestPulumiInit(t *testing.T) {
	t.Run("SanityTest", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer e.DeleteEnvironment()

		// With a .git folder in the test root, `pulumi init` sets up shop there.
		e.RunCommand("git", "init")
		assert.False(t, e.PathExists(".pulumi"), "expecting no .pulumi folder yet")
		e.RunCommand("pulumi", "init")
		assert.True(t, e.PathExists(".pulumi"), "expecting .pulumi folder to be created")
	})

	t.Run("WalkUpToGitFolder", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer e.DeleteEnvironment()

		// Create a git repo in the root.
		e.RunCommand("git", "init")
		assert.True(t, e.PathExists(".git"), "expecting .git folder")

		// Create a subdirectory and CD into it.,
		subdir := path.Join(e.RootPath, "/foo/bar/baz/")
		err := os.MkdirAll(subdir, os.ModePerm)
		assert.NoError(t, err, "error creating subdirectory")
		e.CWD = subdir

		// Confirm we are in the new location (no .git folder found.)
		assert.False(t, e.PathExists(".git"), "expecting no .git folder (in new dir)")

		// pulumi init won't create the folder here, but rather along side .git.
		assert.False(t, e.PathExists(".pulumi"), "expecting no .pulumi folder")
		e.RunCommand("pulumi", "init")
		assert.False(t, e.PathExists(".pulumi"), "expecting no .pulumi folder. still")

		e.CWD = e.RootPath
		assert.True(t, e.PathExists(".pulumi"), "expecting .pulumi folder to be created")
	})

	t.Run("DefaultRepositoryInfo", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer e.DeleteEnvironment()

		e.RunCommand("git", "init")
		e.RunCommand("pulumi", "init")

		// Defaults
		settings := integration.GetSettings(e)
		testRootName := path.Base(e.RootPath)
		assert.Equal(t, os.Getenv("USER"), settings.Owner)
		assert.Equal(t, testRootName, settings.Name)
		assert.Equal(t, "", settings.Root)
	})

	t.Run("ReadRemoteInfo", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer e.DeleteEnvironment()

		e.RunCommand("git", "init")
		e.RunCommand("git", "remote", "add", "not-origin", "git@github.com:moolumi/pasture.git")
		e.RunCommand("git", "remote", "add", "origin", "git@github.com:pulumi/pulumi-cloud.git")
		e.RunCommand("pulumi", "init")

		// We pick up the settings from "origin", not any other remote name.
		settings := integration.GetSettings(e)
		assert.Equal(t, "pulumi", settings.Owner)
		assert.Equal(t, "pulumi-cloud", settings.Name)
		assert.Equal(t, "", settings.Root)
	})
}
