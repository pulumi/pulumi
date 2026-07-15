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

// Package hgutil provides a minimal wrapper around the `hg` (Mercurial) binary for
// detecting a local repository and extracting metadata that Pulumi attaches to
// stack updates.
//
// There is no pure-Go Mercurial library, so every operation shells out to the
// `hg` binary. If `hg` is not available on PATH, discovery returns (nil, nil)
// so that callers degrade silently to other VCS detection (for example Git)
// or to environment-variable fallbacks.
package hgutil

import (
	"bytes"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
)

// Repository is an opaque handle to a Mercurial checkout. Root is the absolute
// path of the directory that contains the `.hg` metadata directory.
type Repository struct {
	Root string
}

// CommitInfo describes a Mercurial commit.
type CommitInfo struct {
	Hash        string
	Branch      string
	Author      string
	AuthorEmail string
	Message     string
	Dirty       bool
}

// hgBin locates the `hg` binary. Returns "" (no error) when not installed, so
// callers can treat missing Mercurial as "no repo" rather than a failure.
func hgBin() string {
	path, err := exec.LookPath("hg")
	if err != nil {
		return ""
	}
	return path
}

// FindRepository walks up from `dir` looking for a `.hg` directory. Returns
// (nil, nil) if no repository is found, or if the `hg` binary is not on PATH.
// This mirrors the contract of gitutil.GetGitRepository.
func FindRepository(dir string) (*Repository, error) {
	if hgBin() == "" {
		return nil, nil
	}

	hgRoot, err := fsutil.WalkUp(dir, func(s string) bool {
		return filepath.Base(s) == ".hg"
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("searching for hg repository from %v: %w", dir, err)
	}
	if hgRoot == "" {
		return nil, nil
	}

	// WalkUp returned the path of the `.hg` directory itself; the repository
	// root is its parent.
	return &Repository{Root: filepath.Dir(hgRoot)}, nil
}

// runHg runs `hg <args...>` in repo.Root and returns trimmed stdout.
func (r *Repository) runHg(args ...string) (string, error) {
	bin := hgBin()
	if bin == "" {
		return "", errors.New("hg binary not found on PATH")
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = r.Root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("'hg %s' failed: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// GetRemoteURL returns the URL configured as the default push/pull path, or
// "" if no default path is configured.
func (r *Repository) GetRemoteURL() (string, error) {
	bin := hgBin()
	if bin == "" {
		return "", nil
	}
	cmd := exec.Command(bin, "paths", "default")
	cmd.Dir = r.Root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		// `hg paths default` exits non-zero when no default is configured;
		// treat that as "no remote" rather than an error.
		stderrStr := stderr.String()
		if strings.Contains(stderrStr, "not found") || strings.Contains(stderrStr, "no such path") {
			return "", nil
		}
		return "", fmt.Errorf("'hg paths default' failed: %w: %s", err, strings.TrimSpace(stderrStr))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Unit separator used to delimit fields in `hg log --template` output. Chosen
// so multi-line commit messages survive parsing.
const logFieldSep = "\x1f"

// GetHeadCommit returns information about the commit at `.` (working
// directory parent) plus the dirty state of the working copy.
func (r *Repository) GetHeadCommit() (*CommitInfo, error) {
	// Template emits: hash\x1fbranch\x1fauthor\x1fdesc
	template := strings.Join([]string{"{node}", "{branch}", "{author}", "{desc}"}, logFieldSep)
	out, err := r.runHg("log", "-r", ".", "--template", template)
	if err != nil {
		return nil, err
	}

	parts := strings.SplitN(out, logFieldSep, 4)
	if len(parts) != 4 {
		return nil, fmt.Errorf("unexpected 'hg log' output: %q", out)
	}

	author, authorEmail := splitAuthor(parts[2])
	info := &CommitInfo{
		Hash:        parts[0],
		Branch:      parts[1],
		Author:      author,
		AuthorEmail: authorEmail,
		Message:     parts[3],
	}

	dirty, err := r.isDirty()
	if err != nil {
		return nil, err
	}
	info.Dirty = dirty
	return info, nil
}

// splitAuthor parses an hg author string into name and email. Mercurial's
// {author} template is typically "Name <email>" but doesn't enforce it.
func splitAuthor(author string) (name, email string) {
	author = strings.TrimSpace(author)
	if author == "" {
		return "", ""
	}
	if addr, err := mail.ParseAddress(author); err == nil {
		return addr.Name, addr.Address
	}
	return author, ""
}

// isDirty reports whether the working copy has any uncommitted changes or
// untracked files.
func (r *Repository) isDirty() (bool, error) {
	out, err := r.runHg("status")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// TryGetVCSInfo extracts owner / repo / kind information from a Mercurial
// remote URL. HTTP(S) remotes and the common `ssh://user@host/owner/repo`
// shape are supported. For other shapes, it delegates to gitutil.TryGetVCSInfo.
func TryGetVCSInfo(remoteURL string) (*gitutil.VCSInfo, error) {
	remoteURL = strings.TrimSpace(remoteURL)
	if strings.HasPrefix(remoteURL, "ssh://") {
		return parseSSHRemote(remoteURL)
	}
	return gitutil.TryGetVCSInfo(remoteURL)
}

func parseSSHRemote(remoteURL string) (*gitutil.VCSInfo, error) {
	u, err := url.Parse(remoteURL)
	if err != nil {
		return nil, fmt.Errorf("parsing hg ssh remote %q: %w", remoteURL, err)
	}
	host := u.Hostname()
	path := strings.TrimPrefix(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	if host == "" || path == "" {
		return nil, fmt.Errorf("hg ssh remote %q missing host or path", remoteURL)
	}
	split := strings.SplitN(path, "/", 2)
	if len(split) != 2 {
		return nil, fmt.Errorf("hg ssh remote %q path must include '/'", remoteURL)
	}
	return &gitutil.VCSInfo{
		Owner: split[0],
		Repo:  split[1],
		Kind:  host,
	}, nil
}
