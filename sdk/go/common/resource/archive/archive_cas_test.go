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

package archive

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/cas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestArchiveContentStore verifies that, when a content store is configured, a
// directory archive's canonical tar is stored exactly once (identical trees dedup by
// hash) and a hash-only "content reference" archive materializes its members from the
// store.
func TestArchiveContentStore(t *testing.T) {
	// Not parallel: SetContentStore is process-global.
	ctx := context.Background()
	dir := t.TempDir()
	store, err := cas.NewFSStore(filepath.Join(dir, "cas"))
	require.NoError(t, err)
	SetContentStore(store)
	t.Cleanup(func() { SetContentStore(nil) })

	// Two separate directory trees with identical contents.
	write := func(root string) {
		require.NoError(t, os.MkdirAll(filepath.Join(root, "sub"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, "a.txt"), []byte("alpha\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(root, "sub", "b.txt"), []byte("beta\n"), 0o644))
	}
	treeA := filepath.Join(dir, "treeA")
	treeB := filepath.Join(dir, "treeB")
	write(treeA)
	write(treeB)

	a, err := FromPath(treeA)
	require.NoError(t, err)
	b, err := FromPath(treeB)
	require.NoError(t, err)

	// Identical contents -> identical canonical-tar hash, and the tar is in the CAS...
	assert.Equal(t, a.Hash, b.Hash)
	has, err := store.Has(ctx, a.Hash)
	require.NoError(t, err)
	assert.True(t, has)

	// ...stored exactly once, no matter how many archives reference it.
	assert.Equal(t, 1, countBlobs(t, filepath.Join(dir, "cas")))

	// A hash-only content-reference archive materializes its members from the CAS.
	ref := &Archive{Sig: ArchiveSig, Hash: a.Hash}
	assert.True(t, ref.HasContents())
	members := readMembers(t, ref)
	assert.Equal(t, map[string]string{
		"a.txt":     "alpha\n",
		"sub/b.txt": "beta\n",
	}, members)
}

// readMembers drains an archive into a name->contents map.
func readMembers(t *testing.T, a *Archive) map[string]string {
	t.Helper()
	r, err := a.Open()
	require.NoError(t, err)
	defer func() { require.NoError(t, r.Close()) }()
	out := map[string]string{}
	for {
		name, blob, err := r.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		data, err := io.ReadAll(blob)
		require.NoError(t, err)
		out[name] = string(data)
	}
	return out
}

// countBlobs counts the regular files stored under a CAS root (ignoring shard dirs).
func countBlobs(t *testing.T, root string) int {
	t.Helper()
	n := 0
	require.NoError(t, filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			n++
		}
		return nil
	}))
	return n
}
