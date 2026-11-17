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

package hgutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requireHg skips the test if the `hg` binary is not available on PATH.
func requireHg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("hg"); err != nil {
		t.Skip("hg binary not found on PATH; skipping")
	}
}

// initHgRepo creates a new hg repository rooted at a temporary directory and
// isolates the test from the user's real ~/.hgrc by setting HGRCPATH to a
// minimal config.
func initHgRepo(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()

	// Isolated hgrc with just enough config for commits to succeed. Keep this
	// outside the repo so it doesn't show up in `hg status`.
	hgrc := filepath.Join(parent, "hgrc")
	require.NoError(t, os.WriteFile(hgrc, []byte(
		"[ui]\nusername = Test User <test@test.org>\n",
	), 0o600))
	t.Setenv("HGRCPATH", hgrc)

	root := filepath.Join(parent, "repo")
	require.NoError(t, os.MkdirAll(root, 0o755))
	run(t, root, "init")
	return root
}

// run executes `hg <args...>` in dir and fails the test if it errors.
func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("hg", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "hg %v failed: %s", args, string(out))
	return string(out)
}

//nolint:paralleltest // t.Setenv("HGRCPATH") is incompatible with t.Parallel.
func TestFindRepository_NoRepo(t *testing.T) {
	requireHg(t)
	dir := t.TempDir()
	repo, err := FindRepository(dir)
	require.NoError(t, err)
	assert.Nil(t, repo)
}

//nolint:paralleltest // t.Setenv("HGRCPATH") is incompatible with t.Parallel.
func TestFindRepository_FindsWalkingUp(t *testing.T) {
	requireHg(t)
	root := initHgRepo(t)
	sub := filepath.Join(root, "a", "b", "c")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	repo, err := FindRepository(sub)
	require.NoError(t, err)
	require.NotNil(t, repo)

	// hg/symlink-aware equality: resolve both sides.
	wantRoot, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)
	gotRoot, err := filepath.EvalSymlinks(repo.Root)
	require.NoError(t, err)
	assert.Equal(t, wantRoot, gotRoot)
}

//nolint:paralleltest // t.Setenv("HGRCPATH") is incompatible with t.Parallel.
func TestGetHeadCommit(t *testing.T) {
	requireHg(t)
	root := initHgRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("hi"), 0o600))
	run(t, root, "add", "alpha.txt")
	run(t, root, "commit", "-m", "first commit\n\nbody line")

	repo, err := FindRepository(root)
	require.NoError(t, err)
	require.NotNil(t, repo)

	info, err := repo.GetHeadCommit()
	require.NoError(t, err)

	require.Len(t, info.Hash, 40, "expected full 40-char hg node hash")
	assert.Equal(t, "default", info.Branch)
	assert.Equal(t, "Test User", info.Author)
	assert.Equal(t, "test@test.org", info.AuthorEmail)
	assert.Equal(t, "first commit\n\nbody line", info.Message)
	assert.False(t, info.Dirty)

	// Untracked file makes the tree dirty.
	require.NoError(t, os.WriteFile(filepath.Join(root, "beta.txt"), []byte(""), 0o600))
	info2, err := repo.GetHeadCommit()
	require.NoError(t, err)
	assert.True(t, info2.Dirty)
}

//nolint:paralleltest // t.Setenv("HGRCPATH") is incompatible with t.Parallel.
func TestGetHeadCommit_NamedBranch(t *testing.T) {
	requireHg(t)
	root := initHgRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("hi"), 0o600))
	run(t, root, "add", "alpha.txt")
	run(t, root, "commit", "-m", "first")

	run(t, root, "branch", "feature/x")
	require.NoError(t, os.WriteFile(filepath.Join(root, "beta.txt"), []byte("hi"), 0o600))
	run(t, root, "add", "beta.txt")
	run(t, root, "commit", "-m", "second")

	repo, err := FindRepository(root)
	require.NoError(t, err)
	require.NotNil(t, repo)

	info, err := repo.GetHeadCommit()
	require.NoError(t, err)
	assert.Equal(t, "feature/x", info.Branch)
}

//nolint:paralleltest // t.Setenv("HGRCPATH") is incompatible with t.Parallel.
func TestGetRemoteURL(t *testing.T) {
	requireHg(t)
	root := initHgRepo(t)

	// No default path configured yet.
	repo, err := FindRepository(root)
	require.NoError(t, err)
	url, err := repo.GetRemoteURL()
	require.NoError(t, err)
	assert.Empty(t, url)

	// Configure a default path via the repo-local hgrc.
	hgrc := filepath.Join(root, ".hg", "hgrc")
	require.NoError(t, os.WriteFile(hgrc, []byte(
		"[paths]\ndefault = https://bitbucket.org/owner-name/repo-name\n",
	), 0o600))

	url, err = repo.GetRemoteURL()
	require.NoError(t, err)
	assert.Equal(t, "https://bitbucket.org/owner-name/repo-name", url)
}

func TestSplitAuthor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, name, email string
	}{
		{"Test User <test@test.org>", "Test User", "test@test.org"},
		{"test@test.org", "", "test@test.org"},
		{"plain name", "plain name", ""},
		{"", "", ""},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			t.Parallel()
			name, email := splitAuthor(c.in)
			assert.Equal(t, c.name, name)
			assert.Equal(t, c.email, email)
		})
	}
}

func TestTryGetVCSInfo_HTTPS(t *testing.T) {
	t.Parallel()
	info, err := TryGetVCSInfo("https://bitbucket.org/owner-name/repo-name")
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "owner-name", info.Owner)
	assert.Equal(t, "repo-name", info.Repo)
	assert.Equal(t, "bitbucket.org", info.Kind)
}

func TestTryGetVCSInfo_SSH(t *testing.T) {
	t.Parallel()
	info, err := TryGetVCSInfo("ssh://hg@hg.example.com/owner-name/repo-name")
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "owner-name", info.Owner)
	assert.Equal(t, "repo-name", info.Repo)
	assert.Equal(t, "hg.example.com", info.Kind)
}
