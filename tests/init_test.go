// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package tests

import (
	"os"
	"path"
	"testing"

	ptesting "github.com/pulumi/pulumi/pkg/testing"
	"github.com/pulumi/pulumi/pkg/testing/integration"
	"github.com/pulumi/pulumi/pkg/workspace"
	"github.com/stretchr/testify/assert"
)

func TestPulumiInit(t *testing.T) {
	t.Run("SanityTest", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()

		// With a .git folder in the test root, `pulumi init` sets up shop there.
		const dirName = workspace.BookkeepingDir
		e.RunCommand("git", "init")
		assert.False(t, e.PathExists(dirName), "expecting no %s folder yet", dirName)
		e.RunCommand("pulumi", "init")
		assert.True(t, e.PathExists(dirName), "expecting %s folder to be created", dirName)
	})

	t.Run("WalkUpToGitFolder", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()

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
		const dirName = workspace.BookkeepingDir
		assert.False(t, e.PathExists(dirName), "expecting no %s folder", dirName)
		e.RunCommand("pulumi", "init")
		assert.False(t, e.PathExists(dirName), "expecting no %s folder. still.", dirName)

		e.CWD = e.RootPath
		assert.True(t, e.PathExists(dirName), "expecting %s folder to exist (next to .git)", dirName)
	})

	t.Run("DefaultRepositoryInfo", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()

		e.RunCommand("git", "init")
		e.RunCommand("pulumi", "init")

		// Defaults
		repo := integration.GetRepository(e)
		testRootName := path.Base(e.RootPath)
		assert.Equal(t, os.Getenv("USER"), repo.Owner)
		assert.Equal(t, testRootName, repo.Name)
		assert.Equal(t, "", repo.Root)
	})

	t.Run("ReadRemoteInfo", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()

		e.RunCommand("git", "init")
		e.RunCommand("git", "remote", "add", "not-origin", "git@github.com:moolumi/pasture.git")
		e.RunCommand("git", "remote", "add", "origin", "git@github.com:pulumi/pulumi-cloud.git")
		e.RunCommand("pulumi", "init")

		// We pick up the settings from "origin", not any other remote name.
		repo := integration.GetRepository(e)
		assert.Equal(t, "pulumi", repo.Owner)
		assert.Equal(t, "pulumi-cloud", repo.Name)
		assert.Equal(t, "", repo.Root)
	})
}
