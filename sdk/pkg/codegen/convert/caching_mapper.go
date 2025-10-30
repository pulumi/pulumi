// Copyright 2025, Pulumi Corporation.
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

package convert

import (
	"context"
	"sync"
)

// cachingMapper wraps another Mapper, caching the results of GetMapping calls by source provider name.
type cachingMapper struct {
	// The underlying Mapper to which calls will be delegated when there is no cache hit.
	mapper Mapper

	// A cache of provider mappings, keyed by source provider name.
	entries map[string][]byte

	// Mutex to protect concurrent access to the entries map
	mu sync.RWMutex
}

// NewCachingMapper creates a new caching mapper backed by the given Mapper.
func NewCachingMapper(mapper Mapper) Mapper {
	return &cachingMapper{
		mapper:  mapper,
		entries: map[string][]byte{},
	}
}

// Implements Mapper.GetMapping. Returned results are cached by source provider key.
func (m *cachingMapper) GetMapping(
	ctx context.Context,
	provider string,
	hint *MapperPackageHint,
) ([]byte, error) {
	m.mu.RLock()
	mapping, ok := m.entries[provider]
	m.mu.RUnlock()

	if ok {
		return mapping, nil
	}

	mapping, err := m.mapper.GetMapping(ctx, provider, hint)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.entries[provider] = mapping
	m.mu.Unlock()

	return mapping, nil
}
