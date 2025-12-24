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

package gitlfs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	// defaultBranch is the default Git branch to use
	defaultBranch = "main"

	// gitLFSCacheDir is the subdirectory under the Pulumi home for Git LFS caches
	gitLFSCacheDir = "gitlfs"
)

var (
	// ErrRepoNotFound indicates the repository doesn't exist
	ErrRepoNotFound = errors.New("repository not found")

	// ErrPushConflict indicates a push conflict
	ErrPushConflict = errors.New("push conflict: remote has newer changes")

	// ErrGitNotFound indicates git is not installed
	ErrGitNotFound = errors.New("git executable not found")

	// ErrLFSNotInstalled indicates git-lfs is not installed
	ErrLFSNotInstalled = errors.New("git-lfs not installed")
)

// Repository manages a local Git clone for storing state
type Repository struct {
	// path is the local clone path
	path string

	// remote is the remote URL (e.g., "https://github.com/org/repo.git")
	remote string

	// branch is the branch to use
	branch string

	// subdir is the subdirectory within repo for state files
	subdir string

	// mu protects concurrent git operations
	mu sync.Mutex
}

// NewRepository clones or opens a Git repository
func NewRepository(ctx context.Context, remote, branch, subdir string) (*Repository, error) {
	// Verify git is installed
	if _, err := exec.LookPath("git"); err != nil {
		return nil, ErrGitNotFound
	}

	if branch == "" {
		branch = defaultBranch
	}

	// Compute the cache path based on the remote URL
	cacheDir, err := getCacheDir(remote)
	if err != nil {
		return nil, fmt.Errorf("getting cache directory: %w", err)
	}

	repo := &Repository{
		path:   cacheDir,
		remote: remote,
		branch: branch,
		subdir: subdir,
	}

	// Check if the repo already exists
	if _, err := os.Stat(filepath.Join(cacheDir, ".git")); err == nil {
		// Repo exists, fetch latest
		if err := repo.fetch(ctx); err != nil {
			logging.V(5).Infof("git fetch failed, will re-clone: %v", err)
			// If fetch fails, try to re-clone
			if err := os.RemoveAll(cacheDir); err != nil {
				return nil, fmt.Errorf("removing corrupt cache: %w", err)
			}
		} else {
			// Reset to remote branch
			if err := repo.resetToRemote(ctx); err != nil {
				logging.V(5).Infof("git reset failed: %v", err)
			}
			return repo, nil
		}
	}

	// Clone the repository
	if err := repo.clone(ctx); err != nil {
		return nil, err
	}

	// Initialize LFS in the repository
	if err := repo.initLFS(ctx); err != nil {
		// LFS init failure is not fatal - log and continue
		logging.V(5).Infof("git lfs install failed (may not be needed): %v", err)
	}

	return repo, nil
}

// getCacheDir returns the cache directory for a remote URL
func getCacheDir(remote string) (string, error) {
	pulumiHome, err := workspace.GetPulumiHomeDir()
	if err != nil {
		return "", err
	}

	// Use SHA256 hash of remote URL for cache directory name
	hash := sha256.Sum256([]byte(remote))
	hashStr := hex.EncodeToString(hash[:8]) // Use first 8 bytes for shorter path

	return filepath.Join(pulumiHome, gitLFSCacheDir, hashStr), nil
}

// clone clones the repository
func (r *Repository) clone(ctx context.Context) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	// Clone with depth 1 for faster clones
	args := []string{"clone", "--depth", "1", "--branch", r.branch, r.remote, r.path}
	if err := r.git(ctx, args...); err != nil {
		// If branch doesn't exist, try cloning without branch specification
		// and create the branch
		args = []string{"clone", "--depth", "1", r.remote, r.path}
		if err := r.git(ctx, args...); err != nil {
			return fmt.Errorf("git clone: %w", err)
		}
		// Create the branch if it doesn't exist
		if err := r.gitInRepo(ctx, "checkout", "-b", r.branch); err != nil {
			// If the branch already exists (just not on remote), check it out
			if err := r.gitInRepo(ctx, "checkout", r.branch); err != nil {
				return fmt.Errorf("creating branch %s: %w", r.branch, err)
			}
		}
	}

	return nil
}

// fetch fetches the latest changes from remote
func (r *Repository) fetch(ctx context.Context) error {
	return r.gitInRepo(ctx, "fetch", "origin", r.branch)
}

// resetToRemote resets the local branch to match the remote
func (r *Repository) resetToRemote(ctx context.Context) error {
	return r.gitInRepo(ctx, "reset", "--hard", "origin/"+r.branch)
}

// initLFS initializes Git LFS in the repository
func (r *Repository) initLFS(ctx context.Context) error {
	// Check if git-lfs is installed
	if _, err := exec.LookPath("git-lfs"); err != nil {
		return ErrLFSNotInstalled
	}

	// Initialize LFS
	return r.gitInRepo(ctx, "lfs", "install", "--local")
}

// Pull updates the local repository
func (r *Repository) Pull(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.fetch(ctx); err != nil {
		return fmt.Errorf("git fetch: %w", err)
	}

	if err := r.resetToRemote(ctx); err != nil {
		return fmt.Errorf("git reset: %w", err)
	}

	// Pull LFS objects
	if err := r.gitInRepo(ctx, "lfs", "pull"); err != nil {
		// LFS pull failure is not fatal for non-LFS repos
		logging.V(5).Infof("git lfs pull (non-fatal): %v", err)
	}

	return nil
}

// Commit commits changes with a message
func (r *Repository) Commit(ctx context.Context, message string) error {
	// Configure git user if not set
	if err := r.ensureGitConfig(ctx); err != nil {
		return err
	}

	// Add all changes
	if err := r.gitInRepo(ctx, "add", "-A"); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	// Check if there are changes to commit
	output, err := r.gitOutput(ctx, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}
	if strings.TrimSpace(output) == "" {
		// No changes to commit
		return nil
	}

	// Commit
	if err := r.gitInRepo(ctx, "commit", "-m", message); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	return nil
}

// Push pushes commits to remote
func (r *Repository) Push(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Try to push
	err := r.gitInRepo(ctx, "push", "origin", r.branch)
	if err != nil {
		// Check if it's a conflict
		if strings.Contains(err.Error(), "rejected") ||
			strings.Contains(err.Error(), "non-fast-forward") {
			return ErrPushConflict
		}
		return fmt.Errorf("git push: %w", err)
	}

	return nil
}

// PushWithRetry pushes with automatic pull-and-retry on conflict
func (r *Repository) PushWithRetry(ctx context.Context, maxRetries int) error {
	for i := 0; i < maxRetries; i++ {
		err := r.Push(ctx)
		if err == nil {
			return nil
		}

		if !errors.Is(err, ErrPushConflict) {
			return err
		}

		// Pull and retry
		if err := r.Pull(ctx); err != nil {
			return fmt.Errorf("pull for retry: %w", err)
		}
	}

	return ErrPushConflict
}

// ensureGitConfig ensures git user is configured
func (r *Repository) ensureGitConfig(ctx context.Context) error {
	// Check if user.name is set
	if _, err := r.gitOutput(ctx, "config", "user.name"); err != nil {
		if err := r.gitInRepo(ctx, "config", "user.name", "Pulumi"); err != nil {
			return fmt.Errorf("setting user.name: %w", err)
		}
	}

	// Check if user.email is set
	if _, err := r.gitOutput(ctx, "config", "user.email"); err != nil {
		if err := r.gitInRepo(ctx, "config", "user.email", "pulumi@pulumi.com"); err != nil {
			return fmt.Errorf("setting user.email: %w", err)
		}
	}

	return nil
}

// FilePath returns the absolute path for a relative key
func (r *Repository) FilePath(key string) string {
	if r.subdir != "" {
		return filepath.Join(r.path, r.subdir, key)
	}
	return filepath.Join(r.path, key)
}

// ReadFile reads a file from the repository
func (r *Repository) ReadFile(key string) ([]byte, error) {
	path := r.FilePath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fs.ErrNotExist
		}
		return nil, err
	}
	return data, nil
}

// WriteFile writes a file to the repository
func (r *Repository) WriteFile(key string, data []byte) error {
	path := r.FilePath(key)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	return os.WriteFile(path, data, 0o600)
}

// DeleteFile deletes a file from the repository
func (r *Repository) DeleteFile(key string) error {
	path := r.FilePath(key)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil // Already deleted
	}
	return err
}

// FileExists checks if a file exists
func (r *Repository) FileExists(key string) (bool, error) {
	path := r.FilePath(key)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ListFiles lists files matching a prefix
func (r *Repository) ListFiles(prefix string) ([]string, error) {
	basePath := r.path
	if r.subdir != "" {
		basePath = filepath.Join(r.path, r.subdir)
	}

	searchPath := filepath.Join(basePath, prefix)
	searchDir := filepath.Dir(searchPath)

	// If the directory doesn't exist, return empty list
	if _, err := os.Stat(searchDir); os.IsNotExist(err) {
		return nil, nil
	}

	var files []string
	err := filepath.WalkDir(basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip .git directory
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		// Get relative path
		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			return err
		}

		// Convert to forward slashes for consistency
		relPath = filepath.ToSlash(relPath)

		// Check prefix match
		if strings.HasPrefix(relPath, prefix) {
			files = append(files, relPath)
		}

		return nil
	})

	return files, err
}

// FileInfo returns information about a file
type FileInfo struct {
	Key     string
	Size    int64
	ModTime time.Time
	IsDir   bool
}

// ListFilesWithInfo lists files with their metadata
func (r *Repository) ListFilesWithInfo(prefix string) ([]FileInfo, error) {
	basePath := r.path
	if r.subdir != "" {
		basePath = filepath.Join(r.path, r.subdir)
	}

	// If the directory doesn't exist, return empty list
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return nil, nil
	}

	var files []FileInfo
	err := filepath.WalkDir(basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip .git directory
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		// Get relative path
		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			return err
		}

		// Convert to forward slashes for consistency
		relPath = filepath.ToSlash(relPath)

		// Skip root
		if relPath == "." {
			return nil
		}

		// Check prefix match
		if !strings.HasPrefix(relPath, prefix) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		files = append(files, FileInfo{
			Key:     relPath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   d.IsDir(),
		})

		return nil
	})

	return files, err
}

// CopyFile copies a file within the repository
func (r *Repository) CopyFile(dstKey, srcKey string) error {
	srcPath := r.FilePath(srcKey)
	dstPath := r.FilePath(dstKey)

	// Read source
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}

	// Write destination
	return os.WriteFile(dstPath, data, 0o600)
}

// TrackLFS configures LFS tracking for a pattern
func (r *Repository) TrackLFS(ctx context.Context, pattern string) error {
	return r.gitInRepo(ctx, "lfs", "track", pattern)
}

// git runs a git command
func (r *Repository) git(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String())
	}
	return nil
}

// gitInRepo runs a git command in the repository directory
func (r *Repository) gitInRepo(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.path
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String())
	}
	return nil
}

// gitOutput runs a git command and returns its output
func (r *Repository) gitOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.path
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

// Path returns the local repository path
func (r *Repository) Path() string {
	return r.path
}

// Remote returns the remote URL
func (r *Repository) Remote() string {
	return r.remote
}

// Branch returns the branch name
func (r *Repository) Branch() string {
	return r.branch
}

// Close cleans up the repository (currently a no-op, but good for interface)
func (r *Repository) Close() error {
	return nil
}
