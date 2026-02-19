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
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// GitRepoInfo contains information about a git repository for Neo task creation.
type GitRepoInfo struct {
	Repo       *git.Repository // The git repository object
	RepoRoot   string          // Root directory of the repository
	RemoteURL  string          // Remote URL (origin)
	Owner      string          // Repository owner/organization
	RepoName   string          // Repository name
	Forge      string          // Forge/provider (e.g., "github", "gitlab")
	BranchName string          // Current branch name
}

// DetectGitRepo detects if the given directory is within a git repository and returns
// information about it. Returns nil if not in a git repository.
func DetectGitRepo(projectRoot string) (*GitRepoInfo, error) {
	repo, err := gitutil.GetGitRepository(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("detecting git repository: %w", err)
	}
	if repo == nil {
		return nil, nil // Not in a git repository
	}

	// Get the worktree to find the repo root
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("getting worktree: %w", err)
	}
	repoRoot := worktree.Filesystem.Root()

	// Get the remote URL
	remoteURL, err := gitutil.GetGitRemoteURL(repo, "origin")
	if err != nil {
		logging.V(7).Infof("could not get origin remote URL: %v", err)
		// Continue without remote URL - we can still detect local repo
	}

	info := &GitRepoInfo{
		Repo:      repo,
		RepoRoot:  repoRoot,
		RemoteURL: remoteURL,
	}

	// Try to get VCS info (owner, repo name, forge) from remote URL
	if remoteURL != "" {
		vcsInfo, err := gitutil.TryGetVCSInfo(remoteURL)
		if err != nil {
			logging.V(7).Infof("could not parse VCS info from remote URL: %v", err)
		} else {
			info.Owner = vcsInfo.Owner
			info.RepoName = vcsInfo.Repo
			info.Forge = normalizeForge(vcsInfo.Kind)
		}
	}

	// Get current branch name
	head, err := repo.Head()
	if err == nil {
		if head.Name().IsBranch() {
			info.BranchName = head.Name().Short()
		}
	}

	return info, nil
}

// normalizeForge normalizes the VCS kind to a forge name for the Neo API.
func normalizeForge(vcsKind string) string {
	switch vcsKind {
	case gitutil.GitHubHostName:
		return "github"
	case gitutil.GitLabHostName:
		return "gitlab"
	case gitutil.AzureDevOpsHostName:
		return "azure"
	case gitutil.BitbucketHostName:
		return "bitbucket"
	default:
		// Return the hostname as-is for unknown forges
		return vcsKind
	}
}

// HasUncommittedChanges checks if the git repository has uncommitted changes.
func HasUncommittedChanges(repoRoot string) (bool, error) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		return false, fmt.Errorf("git not found: %w", err)
	}

	gitStatusCmd := exec.Command(gitBin, "status", "--porcelain", "-z")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	gitStatusCmd.Dir = repoRoot
	gitStatusCmd.Stdout = &stdout
	gitStatusCmd.Stderr = &stderr
	if err = gitStatusCmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			ee.Stderr = stderr.Bytes()
		}
		return false, fmt.Errorf("git status failed: %w", err)
	}

	return stdout.Len() > 0, nil
}

// savedFileState represents the original on-disk state of a file before git operations.
type savedFileState struct {
	content []byte // file contents (nil when the file did not exist)
	exists  bool   // whether the file existed on disk
}

// saveWorktreeState captures the on-disk contents of every dirty file in the
// worktree so they can be restored later.  The returned map is keyed by the
// path relative to the repository root.
func saveWorktreeState(worktree *git.Worktree) map[string]savedFileState {
	status, err := worktree.Status()
	if err != nil {
		logging.V(7).Infof("could not get worktree status for save: %v", err)
		return nil
	}

	root := worktree.Filesystem.Root()
	saved := make(map[string]savedFileState, len(status))

	for path, fileStatus := range status {
		if fileStatus.Worktree == git.Unmodified && fileStatus.Staging == git.Unmodified {
			continue
		}

		fullPath := filepath.Join(root, path)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				saved[path] = savedFileState{exists: false}
			}
			// TODO - abort instead so we don't lose user's changes
			// Skip files we cannot read.
			continue
		}
		saved[path] = savedFileState{content: content, exists: true}
	}

	return saved
}

// restoreWorktreeState writes the previously captured file contents back to
// disk so the user's working directory matches what they had before the
// debug-branch operation.
func restoreWorktreeState(worktree *git.Worktree, saved map[string]savedFileState) {
	if len(saved) == 0 {
		return
	}

	root := worktree.Filesystem.Root()
	for path, state := range saved {
		fullPath := filepath.Join(root, path)
		if !state.exists {
			_ = os.Remove(fullPath)
			continue
		}
		// TODO: error handling
		_ = os.MkdirAll(filepath.Dir(fullPath), 0o755)
		_ = os.WriteFile(fullPath, state.content, 0o644)
	}
}

// CreateAndPushDebugBranch creates a new branch with all current changes (staged and unstaged),
// commits them, and pushes to the remote. Returns the branch name and URL on success.
func CreateAndPushDebugBranch(info *GitRepoInfo, stackName string) (branchName string, branchURL string, err error) {
	if info.Repo == nil {
		return "", "", fmt.Errorf("no git repository")
	}
	if info.RemoteURL == "" {
		return "", "", fmt.Errorf("no remote URL configured")
	}

	// Generate branch name: pulumi-debug/<stack>/<timestamp>
	timestamp := time.Now().Format("20060102-150405")
	branchName = fmt.Sprintf("pulumi-debug/%s/%s", sanitizeBranchName(stackName), timestamp)

	// Get worktree
	worktree, err := info.Repo.Worktree()
	if err != nil {
		return "", "", fmt.Errorf("getting worktree: %w", err)
	}

	// Get current HEAD
	head, err := info.Repo.Head()
	if err != nil {
		return "", "", fmt.Errorf("getting HEAD: %w", err)
	}

	// Find the system git binary up front — we use it for add, commit, and push
	// instead of go-git so that .gitignore is respected and the user's configured
	// credentials (SSH agent, credential helpers, etc.) are used automatically.
	gitBin, err := exec.LookPath("git")
	if err != nil {
		return "", "", fmt.Errorf("git not found: %w", err)
	}

	// Save the on-disk state of all dirty files before we stage/commit them.
	// This lets us restore the user's working directory after the operation.
	saved := saveWorktreeState(worktree)

	// Stage all changes first (including untracked files but respecting .gitignore).
	// We use the system git binary because go-git's AddGlob does NOT honour .gitignore
	// and would stage ignored files (e.g. node_modules, .env, build artifacts).
	// This must be done before checkout to avoid "worktree contains unstaged changes" error.
	if err = runGit(gitBin, info.RepoRoot, "add", "."); err != nil {
		return "", "", fmt.Errorf("staging changes: %w", err)
	}

	// Create and checkout new branch with Keep option to preserve staged changes
	branchRef := plumbing.NewBranchReferenceName(branchName)
	err = worktree.Checkout(&git.CheckoutOptions{
		Hash:   head.Hash(),
		Branch: branchRef,
		Create: true,
		Keep:   true, // Keep staged changes
	})
	if err != nil {
		return "", "", fmt.Errorf("creating branch: %w", err)
	}


	// Commit the changes
	commitMsg := fmt.Sprintf("Pulumi debug snapshot for stack %s\n\nThis branch was automatically created by Pulumi CLI to help Neo debug an error.", stackName)
	_, err = worktree.Commit(commitMsg, &git.CommitOptions{
		All: true,
		Author: &object.Signature{
			Name:  "Pulumi CLI",
			Email: "support@pulumi.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		restoreOriginalBranch(worktree, head, saved)
		return "", "", fmt.Errorf("committing changes: %w", err)
	}

	// Push the branch
	if err = runGit(gitBin, info.RepoRoot, "push", "origin", branchName); err != nil {
		restoreOriginalBranch(worktree, head, saved)
		return "", "", fmt.Errorf("pushing branch: %w", err)
	}

	// Restore original branch/state
	restoreOriginalBranch(worktree, head, saved)

	// Build branch URL
	branchURL = buildBranchURL(info, branchName)

	return branchName, branchURL, nil
}

// runGit executes a git command using the system binary in the given directory.
// It captures stderr and includes it in the error message on failure.
func runGit(gitBin, dir string, args ...string) error {
	cmd := exec.Command(gitBin, args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("%s: %w", msg, err)
		}
		return err
	}
	return nil
}

// restoreOriginalBranch attempts to restore the worktree to the original HEAD
// state and then writes back the saved dirty-file contents so the user's
// unstaged changes are not lost.
func restoreOriginalBranch(worktree *git.Worktree, originalHead *plumbing.Reference, saved map[string]savedFileState) {
	if originalHead == nil {
		return
	}
	_ = worktree.Checkout(&git.CheckoutOptions{
		Branch: originalHead.Name(),
		Force:  true,
	})
	restoreWorktreeState(worktree, saved)
}

// sanitizeBranchName removes or replaces characters that are invalid in git branch names.
func sanitizeBranchName(name string) string {
	// Replace common problematic characters
	replacer := strings.NewReplacer(
		" ", "-",
		":", "-",
		"~", "-",
		"^", "-",
		"?", "-",
		"*", "-",
		"[", "-",
		"]", "-",
		"\\", "-",
	)
	return replacer.Replace(name)
}

// buildBranchURL constructs a URL to view the branch on the forge's web interface.
func buildBranchURL(info *GitRepoInfo, branchName string) string {
	if info.Owner == "" || info.RepoName == "" {
		return ""
	}

	switch info.Forge {
	case "github":
		return fmt.Sprintf("https://github.com/%s/%s/tree/%s", info.Owner, info.RepoName, branchName)
	case "gitlab":
		return fmt.Sprintf("https://gitlab.com/%s/%s/-/tree/%s", info.Owner, info.RepoName, branchName)
	case "bitbucket":
		return fmt.Sprintf("https://bitbucket.org/%s/%s/src/%s", info.Owner, info.RepoName, branchName)
	case "azure":
		// Azure DevOps URL structure is more complex, return empty for now
		return ""
	default:
		return ""
	}
}

// ToNeoRepoInfo converts GitRepoInfo to the client.NeoTaskRepoInfo format.
func (info *GitRepoInfo) ToNeoRepoInfo(branchURL string) *client.NeoTaskRepoInfo {
	if info.Owner == "" || info.RepoName == "" || info.Forge == "" {
		return nil
	}
	return &client.NeoTaskRepoInfo{
		Name:      info.RepoName,
		Org:       info.Owner,
		Forge:     info.Forge,
		BranchURL: branchURL,
	}
}
