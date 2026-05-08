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

package deploy

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

func TestAnnotationStore_SeedAndGet(t *testing.T) {
	t.Parallel()
	store := NewAnnotationStore()

	urn := resource.URN("urn:pulumi:stack::project::type::name")
	entries := map[resource.URN]map[string]resource.PropertyMap{
		urn: {
			"user:api/tags":  {"env": resource.NewProperty("prod")},
			"agent:neo/cost": {"monthly": resource.NewProperty(42.0)},
		},
	}

	store.SeedAll(entries)

	got := store.Get(urn)
	require.Len(t, got, 2)
	assert.Contains(t, got, "user:api/tags")
	assert.Contains(t, got, "agent:neo/cost")
}

func TestAnnotationStore_GetUnknownURN(t *testing.T) {
	t.Parallel()
	store := NewAnnotationStore()

	got := store.Get(resource.URN("urn:pulumi:stack::project::type::unknown"))
	assert.Empty(t, got)
}

func TestAnnotationStore_ApplyPolicyWrites_Valid(t *testing.T) {
	t.Parallel()
	store := NewAnnotationStore()

	urn := resource.URN("urn:pulumi:stack::project::type::name")
	writes := []plugin.AnalyzeAnnotationChange{
		{URN: urn, Key: "user:api/compliance", Data: resource.PropertyMap{"passed": resource.NewProperty(true)}},
		{URN: urn, Key: "user:api/cost", Data: resource.PropertyMap{"monthly": resource.NewProperty(100.0)}},
	}

	store.ApplyPolicyWrites(writes)

	got := store.Get(urn)
	require.Len(t, got, 2)
	assert.Contains(t, got, "user:api/compliance")
	assert.Contains(t, got, "user:api/cost")

	pending := store.PendingWrites()
	require.Len(t, pending, 2)
	assert.Equal(t, urn, pending[0].URN)
}

func TestAnnotationStore_ApplyPolicyWrites_RejectsBadSource(t *testing.T) {
	t.Parallel()
	store := NewAnnotationStore()

	urn := resource.URN("urn:pulumi:stack::project::type::name")
	store.ApplyPolicyWrites([]plugin.AnalyzeAnnotationChange{
		{URN: urn, Key: "agent:neo/foo", Data: resource.PropertyMap{"x": resource.NewProperty("y")}},
	})

	assert.Empty(t, store.Get(urn))
	assert.Empty(t, store.PendingWrites())
}

func TestAnnotationStore_ApplyPolicyWrites_RejectsMalformedKey(t *testing.T) {
	t.Parallel()
	store := NewAnnotationStore()

	urn := resource.URN("urn:pulumi:stack::project::type::name")
	store.ApplyPolicyWrites([]plugin.AnalyzeAnnotationChange{
		{URN: urn, Key: "user:api/", Data: resource.PropertyMap{}},
		{URN: urn, Key: "user:api/UPPER", Data: resource.PropertyMap{}},
		{URN: urn, Key: "no-slash", Data: resource.PropertyMap{}},
	})

	assert.Empty(t, store.Get(urn))
	assert.Empty(t, store.PendingWrites())
}

func TestAnnotationStore_PendingWritesAccumulates(t *testing.T) {
	t.Parallel()
	store := NewAnnotationStore()

	urn := resource.URN("urn:pulumi:stack::project::type::name")
	store.ApplyPolicyWrites([]plugin.AnalyzeAnnotationChange{
		{URN: urn, Key: "user:api/a", Data: resource.PropertyMap{}},
	})
	store.ApplyPolicyWrites([]plugin.AnalyzeAnnotationChange{
		{URN: urn, Key: "user:api/b", Data: resource.PropertyMap{}},
	})

	pending := store.PendingWrites()
	require.Len(t, pending, 2)
}

func TestAnnotationStore_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	store := NewAnnotationStore()

	urn := resource.URN("urn:pulumi:stack::project::type::name")
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			store.ApplyPolicyWrites([]plugin.AnalyzeAnnotationChange{
				{URN: urn, Key: "user:api/k", Data: resource.PropertyMap{"x": resource.NewProperty("y")}},
			})
		}()
		go func() {
			defer wg.Done()
			_ = store.Get(urn)
		}()
	}

	wg.Wait()
}
