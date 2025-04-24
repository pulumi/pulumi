// Copyright 2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
)

// TestReadingGitRepo tests the functions which read data fom the local Git repo
// to add metadata to any updates.
//
//nolint:paralleltest // mutates environment variables
func TestReadingGitRepo(t *testing.T) {
	// Disable our CI/CD detection code, since if this unit test is ran under CI
	// it will change the expected behavior.
	t.Setenv("PULUMI_DISABLE_CI_DETECTION", "1")

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.RunCommand("git", "init", "-b", "master")
	e.RunCommand("git", "config", "user.email", "test@test.org")
	e.RunCommand("git", "config", "user.name", "test")
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
	os.Unsetenv("PULUMI_DISABLE_CI_DETECTION") // Restore our CI/CD detection logic.
	t.Setenv("TRAVIS", "1")
	t.Setenv("TRAVIS_BRANCH", "branch-from-ci")
	t.Setenv("GITHUB_REF", "branch-from-ci")

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
//
//nolint:paralleltest // mutates environment variables
func TestReadingGitLabMetadata(t *testing.T) {
	// Disable our CI/CD detection code, since if this unit test is ran under CI
	// it will change the expected behavior.
	t.Setenv("PULUMI_DISABLE_CI_DETECTION", "1")

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.RunCommand("git", "init", "-b", "master")
	e.RunCommand("git", "config", "user.email", "test@test.org")
	e.RunCommand("git", "config", "user.name", "test")
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

// TestPulumiCLIMetadata tests that the update metadata is correctly populated
// when running a Pulumi program.
func TestPulumiCLIMetadata(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{
		Use: "my-cli-command",
	}

	cmd.PersistentFlags().String(
		"name", "",
		"This is a mock string flag that is set on the command.")

	cmd.PersistentFlags().Int(
		"age", 0,
		"This is a mock int flag that is set on the command.")

	cmd.PersistentFlags().Bool(
		"human", false,
		"This is a mock boolean flag that is set on the command.")

	cmd.PersistentFlags().Bool(
		"unused", false,
		"This is an unused boolean flag that is NOT set on the command.")

	actualEnv := map[string]string{}

	getEnv := func() []string { // This is the function that returns the environment variables.
		return []string{
			// Checks that a normal flag is set in the environment.
			"PULUMI_SKIP_UPDATE_CHECK=1",
			// Checks that a flag is set in the environment to 'false'.
			"PULUMI_EXPERIMENTAL=0",
			// Checks that a non boolean flag set to true is 'set'.
			"PULUMI_STRING_FLAG=true",
			// Checks that an old and forgotten CLI flag is detected.
			"PULUMI_DEPRECATED_FLAG=1.2.3",
		}
	}

	subCmd := &cobra.Command{
		Use: "subcommand",
		Run: func(cmd *cobra.Command, args []string) {
			addPulumiCLIMetadataToEnvironment(actualEnv, cmd.Flags(), getEnv)
		},
	}

	cmd.AddCommand(subCmd)
	cmd.SetArgs([]string{"subcommand", "--name", "pulumipus", "--age", "100", "--human"})

	err := cmd.Execute()
	assert.NoError(t, err, "Expected command to execute successfully")

	// Check that normal flags are set in the environment.
	for _, flagName := range []string{
		"pulumi.version",
		"pulumi.arch",
		"pulumi.os",
	} {
		switch flagName {
		case "pulumi.version":
		default:
			value := actualEnv[flagName]
			assert.NotEmpty(t, value, "Expected %s to be set in update environment map", flagName)
		}
		// Set these to a known sentinel value to keep the later check from failing.
		actualEnv[flagName] = "SKIP"
	}

	// Check that sensitive flags are not leaked in the environment.
	assert.Equal(t, map[string]string{
		// These have unknown runtime values across machines and are set to a sentinel value in the test.
		"pulumi.version": "SKIP",
		"pulumi.arch":    "SKIP",
		"pulumi.os":      "SKIP",

		// CLI flags
		"pulumi.flag.name":  "set",
		"pulumi.flag.age":   "set",
		"pulumi.flag.human": "true",

		// Environment variables

		// Env Vars are considered truthy if they are "1" or "true" case-insensitive.
		"pulumi.env.PULUMI_SKIP_UPDATE_CHECK": "true",
		// The following is a boolean flag that is set to "0", which is falsey.
		"pulumi.env.PULUMI_EXPERIMENTAL": "false",
		// The following is truthy, but NOT a boolean flag.
		"pulumi.env.PULUMI_STRING_FLAG": "set",
		// The following is falsey, but NOT a boolean flag.
		"pulumi.env.PULUMI_DEPRECATED_FLAG": "set",
	}, actualEnv)
}

func TestAddEscMetadataToEnvironment(t *testing.T) {
	t.Parallel()

	env := map[string]string{}

	addEscMetadataToEnvironment(env, []string{"proj/env1", "proj/env2@stable"})

	expected := "[{\"id\":\"proj/env1\"},{\"id\":\"proj/env2@stable\"}]"
	assert.Equal(t, expected, env[backend.StackEnvironments])
}

// Tests that Git metadata can be read from the environment if there is no Git repository present.
//
//nolint:paralleltest // mutates environment variables
func TestGitMetadataIsReadFromEnvironmentWhenNoRepo(t *testing.T) {
	// Disable CI/CD detection code, since we don't care about those variables for this test and we don't want its
	// behaviour to change if it is being run in CI.
	t.Setenv("PULUMI_DISABLE_CI_DETECTION", "1")

	// Arrange.
	t.Setenv("PULUMI_VCS_REPO_OWNER", "owner-name")
	t.Setenv("PULUMI_VCS_REPO_NAME", "repo-name")
	t.Setenv("PULUMI_VCS_REPO_KIND", "repo-kind")
	t.Setenv("PULUMI_VCS_REPO_ROOT", "/repo/root")
	t.Setenv("PULUMI_GIT_HEAD_NAME", "refs/heads/some-branch")
	t.Setenv("PULUMI_GIT_HEAD", "1234567890abcdef")
	t.Setenv("PULUMI_GIT_COMMIT_MESSAGE", "message")
	t.Setenv("PULUMI_GIT_COMMITTER", "committer")
	t.Setenv("PULUMI_GIT_COMMITTER_EMAIL", "committer@example.com")
	t.Setenv("PULUMI_GIT_AUTHOR", "author")
	t.Setenv("PULUMI_GIT_AUTHOR_EMAIL", "author@example.com")

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	test := &backend.UpdateMetadata{
		Environment: make(map[string]string),
	}

	// Act.
	err := addGitMetadata(e.RootPath, test)

	// Assert.
	assert.NoError(t, err)
	assertEnvValue(t, test, backend.VCSRepoOwner, "owner-name")
	assertEnvValue(t, test, backend.VCSRepoName, "repo-name")
	assertEnvValue(t, test, backend.VCSRepoKind, "repo-kind")
	assertEnvValue(t, test, backend.VCSRepoRoot, "/repo/root")
	assertEnvValue(t, test, backend.GitHeadName, "refs/heads/some-branch")
	assertEnvValue(t, test, backend.GitHead, "1234567890abcdef")
	assert.Equal(t, test.Message, "message")
	assertEnvValue(t, test, backend.GitCommitter, "committer")
	assertEnvValue(t, test, backend.GitCommitterEmail, "committer@example.com")
	assertEnvValue(t, test, backend.GitAuthor, "author")
	assertEnvValue(t, test, backend.GitAuthorEmail, "author@example.com")
}

// Tests that Git metadata is not read from the environment in the event that a real Git repository is present.
//
//nolint:paralleltest // mutates environment variables
func TestGitMetadataIsNotReadFromEnvironmentWhenRepo(t *testing.T) {
	// Disable CI/CD detection code, since we don't care about those variables for this test and we don't want its
	// behaviour to change if it is being run in CI.
	t.Setenv("PULUMI_DISABLE_CI_DETECTION", "1")

	// Arrange.
	t.Setenv("PULUMI_VCS_REPO_OWNER", "env-owner-name")
	t.Setenv("PULUMI_VCS_REPO_NAME", "env-repo-name")
	t.Setenv("PULUMI_VCS_REPO_KIND", "env-repo-kind")
	t.Setenv("PULUMI_VCS_REPO_ROOT", "/env/repo/root")
	t.Setenv("PULUMI_GIT_HEAD_NAME", "refs/heads/env-branch")
	t.Setenv("PULUMI_GIT_HEAD", "1234567890abcdef")
	t.Setenv("PULUMI_GIT_COMMIT_MESSAGE", "env-message")
	t.Setenv("PULUMI_GIT_COMMITTER", "env-committer")
	t.Setenv("PULUMI_GIT_COMMITTER_EMAIL", "env-committer@example.com")
	t.Setenv("PULUMI_GIT_AUTHOR", "env-author")
	t.Setenv("PULUMI_GIT_AUTHOR_EMAIL", "env-author@example.com")

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.RunCommand("mkdir", "subdirectory")
	e.RunCommand("git", "init", "-b", "master")
	e.RunCommand("git", "config", "user.email", "repo-user@example.com")
	e.RunCommand("git", "config", "user.name", "repo-user")
	e.RunCommand("git", "remote", "add", "origin", "git@github.com:repo-owner-name/repo-repo-name")
	e.RunCommand("git", "checkout", "-b", "master")

	e.WriteTestFile("alpha.txt", "")
	e.RunCommand("git", "add", ".")
	e.RunCommand("git", "commit", "-m", "repo-message")

	gitHead, _ := e.RunCommand("git", "rev-parse", "HEAD")

	test := &backend.UpdateMetadata{
		Environment: make(map[string]string),
	}

	// Act.
	err := addGitMetadata(filepath.Join(e.RootPath, "subdirectory"), test)

	// Assert.
	assert.NoError(t, err)
	assertEnvValue(t, test, backend.VCSRepoOwner, "repo-owner-name")
	assertEnvValue(t, test, backend.VCSRepoName, "repo-repo-name")
	assertEnvValue(t, test, backend.VCSRepoKind, "github.com")
	assertEnvValue(t, test, backend.VCSRepoRoot, "subdirectory")
	assertEnvValue(t, test, backend.GitHeadName, "refs/heads/master")
	assertEnvValue(t, test, backend.GitHead, strings.Trim(gitHead, "\n"))
	assert.Equal(t, test.Message, "repo-message")
	assertEnvValue(t, test, backend.GitCommitter, "repo-user")
	assertEnvValue(t, test, backend.GitCommitterEmail, "repo-user@example.com")
	assertEnvValue(t, test, backend.GitAuthor, "repo-user")
	assertEnvValue(t, test, backend.GitAuthorEmail, "repo-user@example.com")
}

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
