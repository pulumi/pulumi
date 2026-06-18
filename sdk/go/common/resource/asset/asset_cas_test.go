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

package asset

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/cas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAssetContentStore verifies that, when a content-addressed store is
// configured, identical asset contents are stored exactly once and a hash-only
// "content reference" asset materializes its bytes from the store.
func TestAssetContentStore(t *testing.T) {
	// Not parallel: SetContentStore is process-global.
	ctx := context.Background()
	dir := t.TempDir()
	store, err := cas.NewFSStore(filepath.Join(dir, "cas"))
	require.NoError(t, err)
	SetContentStore(store)
	t.Cleanup(func() { SetContentStore(nil) })

	// Two separate files with identical contents.
	content := []byte("int main() { return 0; }\n")
	pathA := filepath.Join(dir, "a.cpp")
	pathB := filepath.Join(dir, "b.cpp")
	require.NoError(t, os.WriteFile(pathA, content, 0o600))
	require.NoError(t, os.WriteFile(pathB, content, 0o600))

	a, err := FromPath(pathA)
	require.NoError(t, err)
	b, err := FromPath(pathB)
	require.NoError(t, err)

	// Identical contents -> identical hash, and the bytes are now in the CAS...
	assert.Equal(t, a.Hash, b.Hash)
	has, err := store.Has(ctx, a.Hash)
	require.NoError(t, err)
	assert.True(t, has)

	// ...stored exactly once, no matter how many assets reference it.
	assert.Equal(t, 1, countBlobs(t, filepath.Join(dir, "cas")))

	// A hash-only content-reference asset materializes its bytes from the CAS.
	ref := &Asset{Sig: AssetSig, Hash: a.Hash}
	got, err := ref.Bytes()
	require.NoError(t, err)
	assert.Equal(t, content, got)
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
