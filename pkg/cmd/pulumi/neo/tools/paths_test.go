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

package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveUnderRoot_InsideRoot(t *testing.T) {
	t.Parallel()

	root, err := canonicalRoot(t.TempDir())
	require.NoError(t, err)
	target := filepath.Join(root, "a.txt")
	require.NoError(t, os.WriteFile(target, nil, 0o600))

	got, err := resolveUnderRoot(root, target, false)
	require.NoError(t, err)
	assert.Equal(t, target, got)
}

func TestResolveUnderRoot_RejectsEscapeOutsideRoot(t *testing.T) {
	t.Parallel()

	root, err := canonicalRoot(t.TempDir())
	require.NoError(t, err)
	// A sibling tempdir exists on disk but is not under root, so EvalSymlinks
	// succeeds and the containment check fires.
	outside, err := canonicalRoot(t.TempDir())
	require.NoError(t, err)

	_, err = resolveUnderRoot(root, outside, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the working directory")
}

func TestResolveUnderRoot_RejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	root, err := canonicalRoot(t.TempDir())
	require.NoError(t, err)
	outside, err := canonicalRoot(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(outside, "passwd"), nil, 0o600))

	link := filepath.Join(root, "escape")
	require.NoError(t, os.Symlink(outside, link))

	_, err = resolveUnderRoot(root, filepath.Join(link, "passwd"), false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the working directory")
}

func TestResolveUnderRoot_MissingLeafAllowed(t *testing.T) {
	t.Parallel()

	root, err := canonicalRoot(t.TempDir())
	require.NoError(t, err)
	target := filepath.Join(root, "sub", "nested", "new.txt")

	got, err := resolveUnderRoot(root, target, true)
	require.NoError(t, err)
	assert.Equal(t, target, got)
}

func TestResolveUnderRoot_MissingLeafRejectedWhenNotAllowed(t *testing.T) {
	t.Parallel()

	root, err := canonicalRoot(t.TempDir())
	require.NoError(t, err)
	target := filepath.Join(root, "does-not-exist")

	_, err = resolveUnderRoot(root, target, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolving")
}

func TestResolveUnderRoot_MissingLeafEscapeStillRejected(t *testing.T) {
	t.Parallel()

	// Even with allowMissing=true, a path whose ancestor resolves outside the root
	// must be rejected. Constructed here via a symlinked ancestor that escapes.
	root, err := canonicalRoot(t.TempDir())
	require.NoError(t, err)
	outside := t.TempDir()
	link := filepath.Join(root, "escape")
	require.NoError(t, os.Symlink(outside, link))

	_, err = resolveUnderRoot(root, filepath.Join(link, "new-file"), true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the working directory")
}

func TestResolveUnderRoot_MissingChainOfAncestors(t *testing.T) {
	t.Parallel()

	// Several missing intermediate directories should all be reattached on top of
	// the closest existing ancestor — evalClosestAncestor must walk up until it
	// finds one.
	root, err := canonicalRoot(t.TempDir())
	require.NoError(t, err)
	target := filepath.Join(root, "a", "b", "c", "d", "leaf.txt")

	got, err := resolveUnderRoot(root, target, true)
	require.NoError(t, err)
	assert.Equal(t, target, got)
}

func TestCanonicalRoot_ReturnsErrorWhenRootMissing(t *testing.T) {
	t.Parallel()

	// canonicalRoot calls EvalSymlinks, which must fail for a non-existent path.
	// NewFilesystem relies on this error path to reject bad roots early.
	_, err := canonicalRoot(filepath.Join(t.TempDir(), "does-not-exist"))
	require.Error(t, err)
}

func TestEvalClosestAncestor_UsesClosestExistingDirectory(t *testing.T) {
	t.Parallel()

	// Verify the helper directly: only the first two segments exist on disk; the
	// remaining tail must be re-attached unchanged.
	tmp := t.TempDir()
	existing := filepath.Join(tmp, "real")
	require.NoError(t, os.Mkdir(existing, 0o755))

	missing := filepath.Join(existing, "x", "y", "z.txt")
	got, err := evalClosestAncestor(missing)
	require.NoError(t, err)

	// Normalize the expected path against the resolved ancestor — on macOS
	// t.TempDir() can sit under /var which symlinks to /private/var, so the
	// ancestor's symlink-evaluated form is what evalClosestAncestor returns.
	resolvedExisting, err := filepath.EvalSymlinks(existing)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(resolvedExisting, "x", "y", "z.txt"), got)
}
