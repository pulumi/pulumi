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

package selfupdate

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bundle lays out a directory the way the install scripts do, with the language plugins next to the CLI.
func bundle(t *testing.T, contents string) string {
	t.Helper()

	dir := t.TempDir()
	for _, name := range []string{"pulumi", "pulumi-language-nodejs", "pulumi-language-python"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(contents+" "+name), 0o600))
	}

	// The temp directory may itself sit behind a symlink, and the code under test resolves those.
	resolved, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	return resolved
}

func TestManagedPathMarker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		dir  string
		want string
	}{
		{"/opt/homebrew/Cellar/pulumi/3.100.0/bin", "/cellar/"},
		{"/nix/store/abc123-pulumi-3.100.0/bin", "/nix/store/"},
		{"/home/me/project/node_modules/.bin", "node_modules/"},
		{"/home/me/.asdf/installs/pulumi/3.100.0/bin", "/.asdf/"},
		{"/home/me/.local/share/mise/installs/pulumi/3.100.0", "/mise/"},
		{"/home/me/.pulumi/bin", ""},
		{"/usr/local/pulumi/bin", ""},
		// Windows installs use backslashes, and the npm shim is the package manager that reaches them.
		{`C:\Users\me\project\node_modules\.bin`, "node_modules/"},
		{`C:\Users\me\.pulumi\bin`, ""},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, managedPathMarker(tt.dir), "dir %s", tt.dir)
	}
}

func TestInstallDirForRejectsPackageManagerInstalls(t *testing.T) {
	t.Parallel()

	dir := bundle(t, "old")
	// A Cellar segment anywhere in the resolved path is enough to disqualify the install.
	cellar := filepath.Join(dir, "Cellar", "pulumi", "bin")
	require.NoError(t, os.MkdirAll(cellar, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(cellar, "pulumi"), []byte("old"), 0o600))

	_, err := installDirFor(filepath.Join(cellar, "pulumi"))

	var notUpdatable *errNotUpdatable
	require.ErrorAs(t, err, &notUpdatable)
	assert.Contains(t, err.Error(), "brew update && brew upgrade pulumi")
}

func TestInstallDirForRejectsDirectoryWithoutLanguagePlugins(t *testing.T) {
	t.Parallel()

	// A lone binary is a development build or a shim, not an install we should replace.
	dir := t.TempDir()
	exe := filepath.Join(dir, "pulumi")
	require.NoError(t, os.WriteFile(exe, []byte("built from source"), 0o600))

	_, err := installDirFor(exe)

	var notUpdatable *errNotUpdatable
	require.ErrorAs(t, err, &notUpdatable)
	assert.Contains(t, err.Error(), "language plugins")
}

func TestInstallDirForAcceptsBundledInstall(t *testing.T) {
	t.Parallel()

	dir := bundle(t, "old")

	got, err := installDirFor(filepath.Join(dir, "pulumi"))

	require.NoError(t, err)
	assert.Equal(t, dir, got)
}

func TestInstallDirForResolvesSymlinks(t *testing.T) {
	t.Parallel()

	dir := bundle(t, "old")
	link := filepath.Join(t.TempDir(), "pulumi")
	require.NoError(t, os.Symlink(filepath.Join(dir, "pulumi"), link))

	// A link on $PATH should be judged by the directory it points into, not the one holding the link.
	got, err := installDirFor(link)

	require.NoError(t, err)
	assert.Equal(t, dir, got)
}

func TestSwapInReplacesEveryBinary(t *testing.T) {
	t.Parallel()

	install := bundle(t, "old")
	staged := bundle(t, "new")

	require.NoError(t, swapIn(staged, install))

	for _, name := range []string{"pulumi", "pulumi-language-nodejs", "pulumi-language-python"} {
		contents, err := os.ReadFile(filepath.Join(install, name))
		require.NoError(t, err)
		assert.Equal(t, "new "+name, string(contents))
	}
}

func TestSwapInLeavesNoDisplacedBinariesBehind(t *testing.T) {
	t.Parallel()

	install := bundle(t, "old")
	staged := bundle(t, "new")

	require.NoError(t, swapIn(staged, install))

	stale, err := filepath.Glob(filepath.Join(install, "*"+staleSuffix+".*"))
	require.NoError(t, err)
	assert.Empty(t, stale)
}

func TestSwapInRemovesBinariesDisplacedByAnEarlierUpdate(t *testing.T) {
	t.Parallel()

	install := bundle(t, "old")
	staged := bundle(t, "new")

	// Windows cannot delete the executable it is running, so one may survive until the following update.
	leftover := filepath.Join(install, "pulumi"+staleSuffix+".999")
	require.NoError(t, os.WriteFile(leftover, []byte("older still"), 0o600))

	require.NoError(t, swapIn(staged, install))

	assert.NoFileExists(t, leftover)
}

func TestSwapInRestoresTheOldBinariesWhenOneCannotBeInstalled(t *testing.T) {
	t.Parallel()

	install := bundle(t, "old")
	staged := bundle(t, "new")

	// Binaries are swapped in name order, so blocking the last one fails the swap once the earlier two are done.
	// A non-empty directory cannot be renamed over, which is what makes moving the old binary aside fail.
	blocked := filepath.Join(install, "pulumi-language-python"+staleSuffix+"."+strconv.Itoa(os.Getpid()))
	require.NoError(t, os.MkdirAll(blocked, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(blocked, "blocker"), []byte("x"), 0o600))

	err := swapIn(staged, install)

	require.Error(t, err)
	// Every binary displaced before the failure must be back where it started.
	for _, name := range []string{"pulumi", "pulumi-language-nodejs"} {
		contents, readErr := os.ReadFile(filepath.Join(install, name))
		require.NoError(t, readErr, "%s should have been restored", name)
		assert.Equal(t, "old "+name, string(contents))
	}
}

func TestRemoveOrphanedStagingClearsLeftoverDirectories(t *testing.T) {
	t.Parallel()

	// Windows may hold a handle to a freshly written executable, so an update can fail to remove its own staging
	// directory and leave it for a later run to clear.
	parent := t.TempDir()
	orphan := filepath.Join(parent, stagingPrefix+"2600156136")
	require.NoError(t, os.MkdirAll(orphan, 0o700))
	keep := filepath.Join(parent, "bin")
	require.NoError(t, os.MkdirAll(keep, 0o700))

	removeOrphanedStaging(parent)

	assert.NoDirExists(t, orphan)
	assert.DirExists(t, keep)
}

func TestSwapInRemovesBinariesTheNewReleaseNoLongerShips(t *testing.T) {
	t.Parallel()

	install := bundle(t, "old")
	staged := bundle(t, "new")

	// The set of bundled binaries changes between releases, and one that is left behind stays on $PATH.
	dropped := filepath.Join(install, "pulumi-language-dropped")
	require.NoError(t, os.WriteFile(dropped, []byte("old pulumi-language-dropped"), 0o600))

	require.NoError(t, swapIn(staged, install))

	assert.NoFileExists(t, dropped)
	for _, name := range []string{"pulumi", "pulumi-language-nodejs", "pulumi-language-python"} {
		contents, err := os.ReadFile(filepath.Join(install, name))
		require.NoError(t, err)
		assert.Equal(t, "new "+name, string(contents))
	}
}

func TestSwapInIgnoresUnrelatedFiles(t *testing.T) {
	t.Parallel()

	install := bundle(t, "old")
	staged := bundle(t, "new")

	// The install directory may be on $PATH and hold binaries Pulumi does not own.
	unrelated := filepath.Join(install, "something-else")
	require.NoError(t, os.WriteFile(unrelated, []byte("not ours"), 0o600))

	require.NoError(t, swapIn(staged, install))

	contents, err := os.ReadFile(unrelated)
	require.NoError(t, err)
	assert.Equal(t, "not ours", string(contents))
}
