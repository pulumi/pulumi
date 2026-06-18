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

package cas

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func digestOf(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func TestFSStoreRoundTrip(t *testing.T) {
	t.Parallel()
	s, err := NewFSStore(t.TempDir())
	require.NoError(t, err)
	ctx := context.Background()

	content := []byte("hello, world")
	d := digestOf(content)

	has, err := s.Has(ctx, d)
	require.NoError(t, err)
	assert.False(t, has, "expected the store to start empty")

	require.NoError(t, s.Put(ctx, d, bytes.NewReader(content)))

	has, err = s.Has(ctx, d)
	require.NoError(t, err)
	assert.True(t, has)

	rc, err := s.Get(ctx, d)
	require.NoError(t, err)
	defer rc.Close()
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestFSStorePutIsIdempotent(t *testing.T) {
	t.Parallel()
	s, err := NewFSStore(t.TempDir())
	require.NoError(t, err)
	ctx := context.Background()

	content := []byte("dedup me")
	d := digestOf(content)
	require.NoError(t, s.Put(ctx, d, bytes.NewReader(content)))
	require.NoError(t, s.Put(ctx, d, bytes.NewReader(content)), "a second Put of the same digest is a no-op")

	has, err := s.Has(ctx, d)
	require.NoError(t, err)
	assert.True(t, has)
}

func TestFSStorePutVerifiesDigest(t *testing.T) {
	t.Parallel()
	s, err := NewFSStore(t.TempDir())
	require.NoError(t, err)
	ctx := context.Background()

	// Declare the digest of "a" but supply "b": Put must reject it and store nothing.
	wrong := digestOf([]byte("a"))
	require.Error(t, s.Put(ctx, wrong, bytes.NewReader([]byte("b"))))

	has, err := s.Has(ctx, wrong)
	require.NoError(t, err)
	assert.False(t, has, "a mismatched blob must not be committed")
}

func TestFSStoreInvalidDigest(t *testing.T) {
	t.Parallel()
	s, err := NewFSStore(t.TempDir())
	require.NoError(t, err)
	ctx := context.Background()

	for _, bad := range []string{"", "xyz", "ZZ", strings.Repeat("g", 64)} {
		_, err := s.Has(ctx, bad)
		assert.Error(t, err, "digest %q should be rejected", bad)
	}
}

func TestFSStoreGetMissing(t *testing.T) {
	t.Parallel()
	s, err := NewFSStore(t.TempDir())
	require.NoError(t, err)
	ctx := context.Background()

	_, err = s.Get(ctx, digestOf([]byte("nope")))
	assert.Error(t, err)
}
