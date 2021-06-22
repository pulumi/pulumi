// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"os"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	pul_testing "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
	"github.com/stretchr/testify/assert"
)

// assertEnvValue assert the update metadata's Environment map contains the given value.
func assertEnvValue(t *testing.T, md *backend.UpdateMetadata, key, val string) {
	t.Helper()
	got, ok := md.Environment[key]
	if !ok {
		t.Errorf("Didn't find expected update metadata key %q (full env %+v)", key, md.Environment)
	} else {
		assert.EqualValues(t, val, got, "got different value for update metadata %v than expected", key)
	}
}

// TestReadingGitRepo tests the functions which read data fom the local Git repo
// to add metadata to any updates.
func TestReadingGitRepo(t *testing.T) {
	// Disable our CI/CD detection code, since if this unit test is ran under CI
	// it will change the expected behavior.
	os.Setenv("PULUMI_DISABLE_CI_DETECTION", "1")
	defer func() {
		os.Unsetenv("PULUMI_DISABLE_CI_DETECTION")
	}()

	e := pul_testing.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.RunCommand("git", "init")
	e.RunCommand("git", "remote", "add", "origin", "git@github.com:owner-name/repo-name")
	e.RunCommand("git", "checkout", "-b", "master")

	// Commit alpha
	e.WriteTestFile("alpha.txt", "")
	e.RunCommand("git", "add", ".")
	e.RunCommand("git", "commit", "-m", "message for commit alpha\n\nDescription for commit alpha")

	// Test the state of the world from an empty git repo
	{
		test := &backend.UpdateMetadata{
			Environment: make(map[string]string),
		}
		assert.NoError(t, addGitMetadata(e.RootPath, test))

		assert.EqualValues(t, test.Message, "message for commit alpha")
		_, ok := test.Environment[backend.GitHead]
		assert.True(t, ok, "Expected to find Git SHA in update environment map")

		assertEnvValue(t, test, backend.GitHeadName, "refs/heads/master")
		assertEnvValue(t, test, backend.GitDirty, "false")

		assertEnvValue(t, test, backend.VCSRepoOwner, "owner-name")
		assertEnvValue(t, test, backend.VCSRepoName, "repo-name")
	}

	// Change branch, Commit beta
	e.RunCommand("git", "checkout", "-b", "feature/branch1")
	e.WriteTestFile("beta.txt", "")
	e.RunCommand("git", "add", ".")
	e.RunCommand("git", "commit", "-m", "message for commit beta\nDescription for commit beta")
	e.WriteTestFile("beta-unsubmitted.txt", "")

	var featureBranch1SHA string
	{
		test := &backend.UpdateMetadata{
			Environment: make(map[string]string),
		}
		assert.NoError(t, addGitMetadata(e.RootPath, test))

		assert.EqualValues(t, test.Message, "message for commit beta")
		featureBranch1SHA = test.Environment[backend.GitHead]
		_, ok := test.Environment[backend.GitHead]
		assert.True(t, ok, "Expected to find Git SHA in update environment map")
		assertEnvValue(t, test, backend.GitHeadName, "refs/heads/feature/branch1")
		assertEnvValue(t, test, backend.GitDirty, "true") // Because beta-unsubmitted.txt, after commit

		assertEnvValue(t, test, backend.VCSRepoOwner, "owner-name")
		assertEnvValue(t, test, backend.VCSRepoName, "repo-name")
	}

	// Two branches sharing the same commit. But head ref will differ.
	e.RunCommand("git", "checkout", "-b", "feature/branch2") // Same commit as feature/branch1.

	{
		test := &backend.UpdateMetadata{
			Environment: make(map[string]string),
		}
		assert.NoError(t, addGitMetadata(e.RootPath, test))

		assert.EqualValues(t, test.Message, "message for commit beta")
		featureBranch2SHA := test.Environment[backend.GitHead]
		assert.EqualValues(t, featureBranch1SHA, featureBranch2SHA)
		assertEnvValue(t, test, backend.GitHeadName, "refs/heads/feature/branch2")
	}

	// Detached HEAD
	e.RunCommand("git", "checkout", "HEAD^1")

	{
		test := &backend.UpdateMetadata{
			Environment: make(map[string]string),
		}
		assert.NoError(t, addGitMetadata(e.RootPath, test))

		assert.EqualValues(t, test.Message, "message for commit alpha") // The prior commit
		_, ok := test.Environment[backend.GitHead]
		assert.True(t, ok, "Expected to find Git SHA in update environment map")
		_, ok = test.Environment[backend.GitHeadName]
		assert.False(t, ok, "Expected no 'git.headName' key, since in detached head state.")
	}

	// Tag the commit
	e.RunCommand("git", "checkout", "feature/branch2")
	e.RunCommand("git", "tag", "v0.0.0")

	{
		test := &backend.UpdateMetadata{
			Environment: make(map[string]string),
		}
		assert.NoError(t, addGitMetadata(e.RootPath, test))
		// Ref is still branch2, since `git tag` didn't change anything.
		assertEnvValue(t, test, backend.GitHeadName, "refs/heads/feature/branch2")
	}

	// Change refs by checking out a tagged commit.
	// But since we'll be in a detached HEAD state, the git.headName isn't provided.
	e.RunCommand("git", "checkout", "v0.0.0")

	{
		test := &backend.UpdateMetadata{
			Environment: make(map[string]string),
		}
		assert.NoError(t, addGitMetadata(e.RootPath, test))
		_, ok := test.Environment[backend.GitHeadName]
		assert.False(t, ok, "Expected no 'git.headName' key, since in detached head state.")
	}

	// Confirm that data can be inferred from the CI system if unavailable.
	// Fake running under Travis CI.
	varsToSave := []string{"TRAVIS", "TRAVIS_BRANCH"}
	origEnvVars := make(map[string]string)
	for _, varName := range varsToSave {
		origEnvVars[varName] = os.Getenv(varName)
	}
	defer func() {
		for _, varName := range varsToSave {
			orig := origEnvVars[varName]
			if orig == "" {
				os.Unsetenv(varName)
			} else {
				os.Setenv(varName, orig)
			}
		}
	}()

	os.Unsetenv("PULUMI_DISABLE_CI_DETECTION") // Restore our CI/CD detection logic.
	os.Setenv("TRAVIS", "1")
	os.Setenv("TRAVIS_BRANCH", "branch-from-ci")
	os.Setenv("GITHUB_REF", "branch-from-ci")

	{
		test := &backend.UpdateMetadata{
			Environment: make(map[string]string),
		}
		assert.NoError(t, addGitMetadata(e.RootPath, test))
		name, ok := test.Environment[backend.GitHeadName]
		t.Log(name)
		assert.True(t, ok, "Expected 'git.headName' key, from CI util.")
		// assert.Equal(t, "branch-from-ci", name) # see https://github.com/pulumi/pulumi/issues/5303
	}

}

// TestReadingGitLabMetadata tests the functions which read data fom the local Git repo
// to add metadata to any updates.
func TestReadingGitLabMetadata(t *testing.T) {
	// Disable our CI/CD detection code, since if this unit test is ran under CI
	// it will change the expected behavior.
	os.Setenv("PULUMI_DISABLE_CI_DETECTION", "1")
	defer func() {
		os.Unsetenv("PULUMI_DISABLE_CI_DETECTION")
	}()

	e := pul_testing.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.RunCommand("git", "init")
	e.RunCommand("git", "remote", "add", "origin", "git@gitlab.com:owner-name/repo-name")
	e.RunCommand("git", "checkout", "-b", "master")

	// Commit alpha
	e.WriteTestFile("alpha.txt", "")
	e.RunCommand("git", "add", ".")
	e.RunCommand("git", "commit", "-m", "message for commit alpha\n\nDescription for commit alpha")

	// Test the state of the world from an empty git repo
	{
		test := &backend.UpdateMetadata{
			Environment: make(map[string]string),
		}
		assert.NoError(t, addGitMetadata(e.RootPath, test))

		_, ok := test.Environment[backend.GitHead]
		assert.True(t, ok, "Expected to find Git SHA in update environment map")

		assertEnvValue(t, test, backend.VCSRepoOwner, "owner-name")
		assertEnvValue(t, test, backend.VCSRepoName, "repo-name")
		assertEnvValue(t, test, backend.VCSRepoKind, gitutil.GitLabHostName)
	}
}
