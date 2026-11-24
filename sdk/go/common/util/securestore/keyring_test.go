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

package securestore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

//nolint:paralleltest // keyring.MockInit swaps go-keyring's global provider
func TestGetCacheServesTheProbeFetchExactlyOnce(t *testing.T) {
	keyring.MockInit()
	cache := &getCache{}
	cache.mu.Lock()
	cache.valid, cache.value = true, "from-the-probe"
	cache.mu.Unlock()
	store := keyringStore{cache: cache}

	value, err := store.get()
	require.NoError(t, err)
	assert.Equal(t, "from-the-probe", value)

	cache.mu.Lock()
	valid := cache.valid
	cache.mu.Unlock()
	assert.False(t, valid, "the cached fetch must not be served twice")
}

//nolint:paralleltest // keyring.MockInit swaps go-keyring's global provider
func TestWritesInvalidateTheCachedFetch(t *testing.T) {
	keyring.MockInit()
	for _, write := range []struct {
		name string
		do   func(keyringStore) error
	}{
		{"set", func(s keyringStore) error { _ = s.set("x"); return nil }},
		{"delete", func(s keyringStore) error { _ = s.delete(); return nil }},
	} {
		cache := &getCache{}
		cache.mu.Lock()
		cache.valid, cache.value = true, "stale"
		cache.mu.Unlock()
		store := keyringStore{cache: cache}

		require.NoError(t, write.do(store))

		cache.mu.Lock()
		valid := cache.valid
		cache.mu.Unlock()
		assert.False(t, valid, "%s must invalidate the cached fetch", write.name)
	}
}
