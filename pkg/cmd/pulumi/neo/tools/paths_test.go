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
