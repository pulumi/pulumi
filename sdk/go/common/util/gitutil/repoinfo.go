// Copyright 2026, Pulumi Corporation.
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

package gitutil

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
)

// CommitInfo describes the commit at HEAD of a git repository.
type CommitInfo struct {
	// Hash is the full hex hash of the commit.
	Hash string
	// HeadName is the symbolic name of HEAD (e.g. "refs/heads/master"), or "HEAD"
	// when the repository is in a detached HEAD state.
	HeadName string
	// Message is the commit message, with leading and trailing whitespace trimmed.
	Message        string
	AuthorName     string
	AuthorEmail    string
	CommitterName  string
	CommitterEmail string
}

// RepoInfo describes the git repository that contains a directory.
type RepoInfo struct {
	// Root is the absolute path to the root of the repository's worktree, with
	// symlinks resolved.
	Root string
	// RemoteURL is the first URL of the "origin" remote, or empty when the remote
	// doesn't exist.
	RemoteURL string
	// Head describes the commit at HEAD, or nil when the repository has no commits.
	Head *CommitInfo
}

// ReadRepoInfo describes the git repository found by walking up from dir, returning
// (nil, nil) when dir isn't inside one. It prefers the git CLI, which can read any
// repository git itself writes, while go-git rejects some valid repositories (e.g.
// ones with extensions.worktreeConfig enabled). go-git is used as a fallback when no
// git binary is available.
func ReadRepoInfo(dir string) (*RepoInfo, error) {
	gitRoot, err := fsutil.WalkUp(dir, func(s string) bool { return filepath.Base(s) == ".git" }, nil)
	if err != nil {
		return nil, fmt.Errorf("searching for git repository from %v: %w", dir, err)
	}
	if gitRoot == "" {
		return nil, nil
	}
	worktreeDir := filepath.Dir(gitRoot)

	if _, err := exec.LookPath("git"); err != nil {
		return readRepoInfoGoGit(worktreeDir)
	}
	return readRepoInfoSystemGit(worktreeDir)
}

func readRepoInfoSystemGit(dir string) (*RepoInfo, error) {
	root, err := runGit(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("detecting repository root: %w", err)
	}
	info := &RepoInfo{Root: filepath.Clean(filepath.FromSlash(strings.TrimSpace(root)))}

	// Read the configured URL rather than using `git remote get-url`, which would
	// apply the user's url.insteadOf rewrite rules. `git config --get-all` exits
	// with code 1 when the key is not set; when the remote has multiple URLs, the
	// first one is the primary, matching go-git's behavior.
	switch remoteURLs, err := runGit(dir, "config", "--get-all", "remote.origin.url"); {
	case err == nil:
		urls := strings.Split(strings.TrimSpace(remoteURLs), "\n")
		info.RemoteURL = strings.TrimSpace(urls[0])
	case gitExitCode(err) != 1:
		return nil, fmt.Errorf("detecting origin remote URL: %w", err)
	}

	// `rev-parse -q --verify` exits with code 1 and no output when the repository
	// has no commits.
	if _, err := runGit(dir, "rev-parse", "-q", "--verify", "HEAD"); err != nil {
		if gitExitCode(err) == 1 {
			return info, nil
		}
		return nil, fmt.Errorf("detecting repository HEAD: %w", err)
	}

	// `symbolic-ref -q` exits with code 1 when HEAD is detached.
	headName := "HEAD"
	switch out, err := runGit(dir, "symbolic-ref", "-q", "HEAD"); {
	case err == nil:
		headName = strings.TrimSpace(out)
	case gitExitCode(err) != 1:
		return nil, fmt.Errorf("detecting HEAD branch: %w", err)
	}

	out, err := runGit(dir, "-c", "log.showSignature=false", "log", "-n", "1",
		"--format=%H%x00%an%x00%ae%x00%cn%x00%ce%x00%B")
	if err != nil {
		return nil, fmt.Errorf("reading HEAD commit: %w", err)
	}
	parts := strings.SplitN(out, "\x00", 6)
	if len(parts) != 6 {
		return nil, fmt.Errorf("unexpected git log output: %q", out)
	}
	info.Head = &CommitInfo{
		Hash:           parts[0],
		HeadName:       headName,
		Message:        strings.TrimSpace(parts[5]),
		AuthorName:     parts[1],
		AuthorEmail:    parts[2],
		CommitterName:  parts[3],
		CommitterEmail: parts[4],
	}
	return info, nil
}

// readRepoInfoGoGit mirrors readRepoInfoSystemGit using go-git, for hosts without a
// git binary.
func readRepoInfoGoGit(dir string) (*RepoInfo, error) {
	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{EnableDotGitCommonDir: true})
	if err == git.ErrRepositoryNotExists {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading git repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("detecting repository root: %w", err)
	}
	root, err := filepath.EvalSymlinks(worktree.Filesystem.Root())
	if err != nil {
		return nil, fmt.Errorf("detecting repository root: %w", err)
	}
	info := &RepoInfo{Root: root}

	if remoteURL, err := GetGitRemoteURL(repo, "origin"); err == nil {
		info.RemoteURL = remoteURL
	}

	head, err := repo.Head()
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return info, nil
	}
	if err != nil {
		return nil, fmt.Errorf("detecting repository HEAD: %w", err)
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("reading HEAD commit: %w", err)
	}
	info.Head = &CommitInfo{
		Hash:           head.Hash().String(),
		HeadName:       head.Name().String(),
		Message:        strings.TrimSpace(commit.Message),
		AuthorName:     commit.Author.Name,
		AuthorEmail:    commit.Author.Email,
		CommitterName:  commit.Committer.Name,
		CommitterEmail: commit.Committer.Email,
	}
	return info, nil
}

func runGit(dir string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("git %v: %w: %s", strings.Join(args, " "), err, msg)
		}
		return "", fmt.Errorf("git %v: %w", strings.Join(args, " "), err)
	}
	return stdout.String(), nil
}

func gitExitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}
