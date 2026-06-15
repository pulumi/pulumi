// Copyright 2016, Pulumi Corporation.
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

package schema

import (
	"context"
	"sync"

	"github.com/blang/semver"
)

func NewCachedLoader(loader ReferenceLoader) ReferenceLoader {
	return &cachedLoader{
		loader:  loader,
		entries: make(map[string]PackageReference),
	}
}

// NewCachedLoaderWithEntries creates a new cached loader with the passed in entries pre-loaded.
func NewCachedLoaderWithEntries(loader ReferenceLoader, entries map[string]PackageReference) ReferenceLoader {
	return &cachedLoader{
		loader:  loader,
		entries: entries,
	}
}

type cachedLoader struct {
	loader ReferenceLoader

	m       sync.RWMutex
	entries map[string]PackageReference
}

func (l *cachedLoader) LoadPackage(pkg string, version *semver.Version) (*Package, error) {
	ref, err := l.LoadPackageReference(pkg, version)
	if err != nil {
		return nil, err
	}
	return ref.Definition()
}

func (l *cachedLoader) LoadPackageV2(ctx context.Context, descriptor *PackageDescriptor) (*Package, error) {
	ref, err := l.LoadPackageReferenceV2(ctx, descriptor)
	if err != nil {
		return nil, err
	}
	return ref.Definition()
}

func (l *cachedLoader) LoadPackageReference(pkg string, version *semver.Version) (PackageReference, error) {
	return l.LoadPackageReferenceV2(context.Background(), &PackageDescriptor{
		Name:    pkg,
		Version: version,
	})
}

func (l *cachedLoader) LoadPackageReferenceV2(
	ctx context.Context, descriptor *PackageDescriptor,
) (PackageReference, error) {
	l.m.Lock()
	defer l.m.Unlock()

	key := entryKey(descriptor)
	if p, ok := l.entries[key]; ok {
		return p, nil
	}

	p, err := l.loader.LoadPackageReferenceV2(ctx, descriptor)
	if err != nil {
		return nil, err
	}

	l.entries[key] = p
	return p, nil
}

// LoadRawSchemaBytes implements RawLoader when the underlying loader does. A cached entry takes precedence over
// whatever the underlying loader would serve — entries may be pre-seeded with packages the underlying loader
// can't load at all, such as file-based schemas during package linking — so its presence forces ok=false and a
// bind-based load. The cache is never populated here: raw bytes are unbound.
func (l *cachedLoader) LoadRawSchemaBytes(
	ctx context.Context, descriptor *PackageDescriptor,
) ([]byte, bool, error) {
	raw, ok := l.loader.(RawLoader)
	if !ok {
		return nil, false, nil
	}

	l.m.RLock()
	_, cached := l.entries[entryKey(descriptor)]
	l.m.RUnlock()
	if cached {
		return nil, false, nil
	}

	return raw.LoadRawSchemaBytes(ctx, descriptor)
}

// entryKey returns the entry cache key for a package descriptor.
func entryKey(descriptor *PackageDescriptor) string {
	if descriptor.Parameterization == nil {
		return packageIdentity(descriptor.Name, descriptor.Version)
	}
	return packageIdentity(descriptor.Parameterization.Name, &descriptor.Parameterization.Version)
}
