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
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingStatCache is an in-memory StatCache that records how often it is
// consulted and populated, so a test can assert hashing took the cached path.
type countingStatCache struct {
	mu    sync.Mutex
	store map[string]string
	hits  int
	puts  int
}

func (c *countingStatCache) Get(path string, _ os.FileInfo) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	d, ok := c.store[path]
	if ok {
		c.hits++
	}
	return d, ok
}

func (c *countingStatCache) Put(path string, _ os.FileInfo, digest string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.store == nil {
		c.store = map[string]string{}
	}
	c.store[path] = digest
	c.puts++
	return nil
}

// TestEnsureHashUsesStatCache verifies that, when a stat-cache is configured,
// the first hash of a file populates it and a subsequent hash of the same file
// is served from it -- the same digest, with no second read recorded.
func TestEnsureHashUsesStatCache(t *testing.T) {
	// Not parallel: SetStatCache is process-global.
	sc := &countingStatCache{}
	SetStatCache(sc)
	t.Cleanup(func() { SetStatCache(nil) })

	file := filepath.Join(t.TempDir(), "a.cpp")
	require.NoError(t, os.WriteFile(file, []byte("int main() { return 0; }\n"), 0o600))

	// First hash: a miss that records the digest exactly once.
	a, err := FromPath(file)
	require.NoError(t, err)
	require.NotEmpty(t, a.Hash)
	assert.Equal(t, 1, sc.puts, "first hash should populate the cache")

	// Second hash of the same file: a hit, the same digest, no new Put.
	b, err := FromPath(file)
	require.NoError(t, err)
	assert.Equal(t, a.Hash, b.Hash)
	assert.Equal(t, 1, sc.puts, "second hash should be served from the cache")
	assert.GreaterOrEqual(t, sc.hits, 1, "second hash should hit the cache")
}
