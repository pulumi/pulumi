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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFSStatCacheRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c, err := NewFSStatCache(filepath.Join(dir, "stat"))
	require.NoError(t, err)

	file := filepath.Join(dir, "a.txt")
	require.NoError(t, os.WriteFile(file, []byte("hello"), 0o600))
	info, err := os.Stat(file)
	require.NoError(t, err)

	// Nothing cached yet: a miss.
	_, ok := c.Get(file, info)
	assert.False(t, ok, "expected a miss before Put")

	// Record a digest, then read it back for the same file identity.
	require.NoError(t, c.Put(file, info, "deadbeef"))
	got, ok := c.Get(file, info)
	require.True(t, ok)
	assert.Equal(t, "deadbeef", got)
}

func TestFSStatCacheStaleOnChange(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c, err := NewFSStatCache(filepath.Join(dir, "stat"))
	require.NoError(t, err)

	file := filepath.Join(dir, "a.txt")
	require.NoError(t, os.WriteFile(file, []byte("hello"), 0o600))
	info, err := os.Stat(file)
	require.NoError(t, err)
	require.NoError(t, c.Put(file, info, "deadbeef"))

	// Changing the file changes its recorded size, so the entry no longer
	// matches and the cached digest must not be served.
	require.NoError(t, os.WriteFile(file, []byte("hello, world"), 0o600))
	changed, err := os.Stat(file)
	require.NoError(t, err)
	_, ok := c.Get(file, changed)
	assert.False(t, ok, "expected a stale miss after the file changed")
}

func TestFSStatCachePersistsAcrossInstances(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	statDir := filepath.Join(dir, "stat")
	c, err := NewFSStatCache(statDir)
	require.NoError(t, err)

	file := filepath.Join(dir, "a.txt")
	require.NoError(t, os.WriteFile(file, []byte("hello"), 0o600))
	info, err := os.Stat(file)
	require.NoError(t, err)
	require.NoError(t, c.Put(file, info, "deadbeef"))

	// A fresh cache over the same directory -- modeling a new process -- still
	// finds the entry.
	c2, err := NewFSStatCache(statDir)
	require.NoError(t, err)
	got, ok := c2.Get(file, info)
	require.True(t, ok)
	assert.Equal(t, "deadbeef", got)
}
