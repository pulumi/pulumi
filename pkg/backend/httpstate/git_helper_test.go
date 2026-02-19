// Copyright 2025, Pulumi Corporation.
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

package httpstate

import (
	"os"
	"path/filepath"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
)

func TestDetectGitRepo(t *testing.T) {
	t.Parallel()

	t.Run("NotInGitRepo", func(t *testing.T) {
		t.Parallel()

		// Create a temporary directory that is not a git repo
		tmpDir := t.TempDir()

		info, err := DetectGitRepo(tmpDir)
		require.NoError(t, err)
		assert.Nil(t, info)
	})

	t.Run("InGitRepo", func(t *testing.T) {
		t.Parallel()

		// Create a temporary git repository
		tmpDir := t.TempDir()
		repo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		// Add an initial commit
		worktree, err := repo.Worktree()
		require.NoError(t, err)

		// Create a file and commit it
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test content"), 0o644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("Initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test",
				Email: "test@test.com",
			},
		})
		require.NoError(t, err)

		info, err := DetectGitRepo(tmpDir)
		require.NoError(t, err)
		require.NotNil(t, info)
		assert.Equal(t, tmpDir, info.RepoRoot)
		assert.NotNil(t, info.Repo)
	})

	t.Run("InSubdirectoryOfGitRepo", func(t *testing.T) {
		t.Parallel()

		// Create a temporary git repository
		tmpDir := t.TempDir()
		_, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		// Create a subdirectory
		subDir := filepath.Join(tmpDir, "subdir")
		err = os.MkdirAll(subDir, 0o755)
		require.NoError(t, err)

		info, err := DetectGitRepo(subDir)
		require.NoError(t, err)
		require.NotNil(t, info)
		assert.Equal(t, tmpDir, info.RepoRoot)
	})
}

func TestNormalizeForge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{gitutil.GitHubHostName, "github"},
		{gitutil.GitLabHostName, "gitlab"},
		{gitutil.AzureDevOpsHostName, "azure"},
		{gitutil.BitbucketHostName, "bitbucket"},
		{"unknown.example.com", "unknown.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			result := normalizeForge(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasUncommittedChanges(t *testing.T) {
	t.Parallel()

	t.Run("CleanRepo", func(t *testing.T) {
		t.Parallel()

		// Create a temporary git repository with a clean state
		tmpDir := t.TempDir()
		repo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		// Add an initial commit
		worktree, err := repo.Worktree()
		require.NoError(t, err)

		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test content"), 0o644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("Initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test",
				Email: "test@test.com",
			},
		})
		require.NoError(t, err)

		hasChanges, err := HasUncommittedChanges(tmpDir)
		require.NoError(t, err)
		assert.False(t, hasChanges)
	})

	t.Run("DirtyRepo", func(t *testing.T) {
		t.Parallel()

		// Create a temporary git repository
		tmpDir := t.TempDir()
		repo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		// Add an initial commit
		worktree, err := repo.Worktree()
		require.NoError(t, err)

		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test content"), 0o644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("Initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test",
				Email: "test@test.com",
			},
		})
		require.NoError(t, err)

		// Now modify the file to create uncommitted changes
		err = os.WriteFile(testFile, []byte("modified content"), 0o644)
		require.NoError(t, err)

		hasChanges, err := HasUncommittedChanges(tmpDir)
		require.NoError(t, err)
		assert.True(t, hasChanges)
	})

	t.Run("UntrackedFiles", func(t *testing.T) {
		t.Parallel()

		// Create a temporary git repository
		tmpDir := t.TempDir()
		repo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		// Add an initial commit
		worktree, err := repo.Worktree()
		require.NoError(t, err)

		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test content"), 0o644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("Initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test",
				Email: "test@test.com",
			},
		})
		require.NoError(t, err)

		// Add an untracked file
		newFile := filepath.Join(tmpDir, "untracked.txt")
		err = os.WriteFile(newFile, []byte("untracked content"), 0o644)
		require.NoError(t, err)

		hasChanges, err := HasUncommittedChanges(tmpDir)
		require.NoError(t, err)
		assert.True(t, hasChanges)
	})
}

func TestSanitizeBranchName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with spaces", "with-spaces"},
		{"with:colons", "with-colons"},
		{"with~tilde", "with-tilde"},
		{"with^caret", "with-caret"},
		{"with?question", "with-question"},
		{"with*star", "with-star"},
		{"with[brackets]", "with-brackets-"},
		{"with\\backslash", "with-backslash"},
		{"multiple: special~ chars", "multiple--special--chars"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			result := sanitizeBranchName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildBranchURL(t *testing.T) {
	t.Parallel()

	t.Run("GitHub", func(t *testing.T) {
		t.Parallel()

		info := &GitRepoInfo{
			Owner:    "my-org",
			RepoName: "my-repo",
			Forge:    "github",
		}
		url := buildBranchURL(info, "feature/test")
		assert.Equal(t, "https://github.com/my-org/my-repo/tree/feature/test", url)
	})

	t.Run("GitLab", func(t *testing.T) {
		t.Parallel()

		info := &GitRepoInfo{
			Owner:    "my-group",
			RepoName: "my-project",
			Forge:    "gitlab",
		}
		url := buildBranchURL(info, "feature/test")
		assert.Equal(t, "https://gitlab.com/my-group/my-project/-/tree/feature/test", url)
	})

	t.Run("Bitbucket", func(t *testing.T) {
		t.Parallel()

		info := &GitRepoInfo{
			Owner:    "my-team",
			RepoName: "my-repo",
			Forge:    "bitbucket",
		}
		url := buildBranchURL(info, "feature/test")
		assert.Equal(t, "https://bitbucket.org/my-team/my-repo/src/feature/test", url)
	})

	t.Run("UnknownForge", func(t *testing.T) {
		t.Parallel()

		info := &GitRepoInfo{
			Owner:    "my-org",
			RepoName: "my-repo",
			Forge:    "unknown",
		}
		url := buildBranchURL(info, "feature/test")
		assert.Equal(t, "", url)
	})

	t.Run("MissingOwner", func(t *testing.T) {
		t.Parallel()

		info := &GitRepoInfo{
			RepoName: "my-repo",
			Forge:    "github",
		}
		url := buildBranchURL(info, "feature/test")
		assert.Equal(t, "", url)
	})
}

func TestGitRepoInfoToNeoRepoInfo(t *testing.T) {
	t.Parallel()

	t.Run("ValidInfo", func(t *testing.T) {
		t.Parallel()

		info := &GitRepoInfo{
			Owner:    "my-org",
			RepoName: "my-repo",
			Forge:    "github",
		}
		neoInfo := info.ToNeoRepoInfo("https://github.com/my-org/my-repo/tree/test-branch")

		require.NotNil(t, neoInfo)
		assert.Equal(t, "my-repo", neoInfo.Name)
		assert.Equal(t, "my-org", neoInfo.Org)
		assert.Equal(t, "github", neoInfo.Forge)
		assert.Equal(t, "https://github.com/my-org/my-repo/tree/test-branch", neoInfo.BranchURL)
	})

	t.Run("MissingOwner", func(t *testing.T) {
		t.Parallel()

		info := &GitRepoInfo{
			RepoName: "my-repo",
			Forge:    "github",
		}
		neoInfo := info.ToNeoRepoInfo("https://example.com/branch")
		assert.Nil(t, neoInfo)
	})

	t.Run("MissingRepoName", func(t *testing.T) {
		t.Parallel()

		info := &GitRepoInfo{
			Owner: "my-org",
			Forge: "github",
		}
		neoInfo := info.ToNeoRepoInfo("https://example.com/branch")
		assert.Nil(t, neoInfo)
	})

	t.Run("MissingForge", func(t *testing.T) {
		t.Parallel()

		info := &GitRepoInfo{
			Owner:    "my-org",
			RepoName: "my-repo",
		}
		neoInfo := info.ToNeoRepoInfo("https://example.com/branch")
		assert.Nil(t, neoInfo)
	})
}

func TestDetectGitRepoWithRemote(t *testing.T) {
	t.Parallel()

	// Create a temporary git repository with a remote
	tmpDir := t.TempDir()
	repo, err := git.PlainInit(tmpDir, false)
	require.NoError(t, err)

	// Add a remote
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"https://github.com/pulumi/pulumi.git"},
	})
	require.NoError(t, err)

	// Add an initial commit
	worktree, err := repo.Worktree()
	require.NoError(t, err)

	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0o644)
	require.NoError(t, err)

	_, err = worktree.Add("test.txt")
	require.NoError(t, err)

	_, err = worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})
	require.NoError(t, err)

	info, err := DetectGitRepo(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.Equal(t, tmpDir, info.RepoRoot)
	assert.Equal(t, "https://github.com/pulumi/pulumi.git", info.RemoteURL)
	assert.Equal(t, "pulumi", info.Owner)
	assert.Equal(t, "pulumi", info.RepoName)
	assert.Equal(t, "github", info.Forge)
	assert.Equal(t, "master", info.BranchName)
}
