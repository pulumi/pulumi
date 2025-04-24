// Copyright 2024, Pulumi Corporation.
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

package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetAllDependencies(t *testing.T) {
	t.Parallel()

	// Arrange.
	cases := []struct {
		name                string
		given               *State
		wantProvider        string
		wantAllDependencies []StateDependency
	}{
		{
			name:                "no provider, no dependencies",
			given:               &State{},
			wantProvider:        "",
			wantAllDependencies: nil,
		},
		{
			name:                "provider, no dependencies",
			given:               &State{Provider: "provider"},
			wantProvider:        "provider",
			wantAllDependencies: nil,
		},
		{
			name:         "no provider, parent",
			given:        &State{Parent: "urn"},
			wantProvider: "",
			wantAllDependencies: []StateDependency{
				{Type: ResourceParent, URN: "urn"},
			},
		},
		{
			name:         "provider, parent (non-empty)",
			given:        &State{Provider: "provider", Parent: "urn"},
			wantProvider: "provider",
			wantAllDependencies: []StateDependency{
				{Type: ResourceParent, URN: "urn"},
			},
		},
		{
			name:                "provider, parent (empty)",
			given:               &State{Provider: "provider", Parent: ""},
			wantProvider:        "provider",
			wantAllDependencies: nil,
		},
		{
			name:         "no provider, dependencies",
			given:        &State{Dependencies: []URN{"urn1", "urn2"}},
			wantProvider: "",
			wantAllDependencies: []StateDependency{
				{Type: ResourceDependency, URN: "urn1"},
				{Type: ResourceDependency, URN: "urn2"},
			},
		},
		{
			name:         "provider, dependencies (no empty)",
			given:        &State{Provider: "provider", Dependencies: []URN{"urn1", "urn2"}},
			wantProvider: "provider",
			wantAllDependencies: []StateDependency{
				{Type: ResourceDependency, URN: "urn1"},
				{Type: ResourceDependency, URN: "urn2"},
			},
		},
		{
			name:         "provider, dependencies (some empty)",
			given:        &State{Provider: "provider", Dependencies: []URN{"urn1", ""}},
			wantProvider: "provider",
			wantAllDependencies: []StateDependency{
				{Type: ResourceDependency, URN: "urn1"},
			},
		},
		{
			name: "no provider, property dependencies",
			given: &State{PropertyDependencies: map[PropertyKey][]URN{
				"key1": {"urn1", "urn2"},
				"key2": {"urn3"},
			}},
			wantProvider: "",
			wantAllDependencies: []StateDependency{
				{Type: ResourcePropertyDependency, Key: "key1", URN: "urn1"},
				{Type: ResourcePropertyDependency, Key: "key1", URN: "urn2"},
				{Type: ResourcePropertyDependency, Key: "key2", URN: "urn3"},
			},
		},
		{
			name: "provider, property dependencies (no empty)",
			given: &State{
				Provider: "provider",
				PropertyDependencies: map[PropertyKey][]URN{
					"key1": {"urn1", "urn2"},
					"key2": {"urn3"},
				},
			},
			wantProvider: "provider",
			wantAllDependencies: []StateDependency{
				{Type: ResourcePropertyDependency, Key: "key1", URN: "urn1"},
				{Type: ResourcePropertyDependency, Key: "key1", URN: "urn2"},
				{Type: ResourcePropertyDependency, Key: "key2", URN: "urn3"},
			},
		},
		{
			name: "provider, property dependencies (some empty)",
			given: &State{
				Provider: "provider",
				PropertyDependencies: map[PropertyKey][]URN{
					"key1": {"urn1", ""},
					"key2": {"urn2"},
					"key3": {},
				},
			},
			wantProvider: "provider",
			wantAllDependencies: []StateDependency{
				{Type: ResourcePropertyDependency, Key: "key1", URN: "urn1"},
				{Type: ResourcePropertyDependency, Key: "key2", URN: "urn2"},
			},
		},
		{
			name:         "no provider, deleted with",
			given:        &State{DeletedWith: "urn"},
			wantProvider: "",
			wantAllDependencies: []StateDependency{
				{Type: ResourceDeletedWith, URN: "urn"},
			},
		},
		{
			name:         "provider, deleted with (non-empty)",
			given:        &State{Provider: "provider", DeletedWith: "urn"},
			wantProvider: "provider",
			wantAllDependencies: []StateDependency{
				{Type: ResourceDeletedWith, URN: "urn"},
			},
		},
		{
			name:                "provider, deleted with (empty)",
			given:               &State{Provider: "provider", DeletedWith: ""},
			wantProvider:        "provider",
			wantAllDependencies: nil,
		},
		{
			name: "all dependencies (no empty)",
			given: &State{
				Provider:     "provider",
				Parent:       "urn1",
				Dependencies: []URN{"urn2", "urn3"},
				PropertyDependencies: map[PropertyKey][]URN{
					"key1": {"urn4", "urn5"},
					"key2": {"urn6"},
				},
				DeletedWith: "urn7",
			},
			wantProvider: "provider",
			wantAllDependencies: []StateDependency{
				{Type: ResourceParent, URN: "urn1"},
				{Type: ResourceDependency, URN: "urn2"},
				{Type: ResourceDependency, URN: "urn3"},
				{Type: ResourcePropertyDependency, Key: "key1", URN: "urn4"},
				{Type: ResourcePropertyDependency, Key: "key1", URN: "urn5"},
				{Type: ResourcePropertyDependency, Key: "key2", URN: "urn6"},
				{Type: ResourceDeletedWith, URN: "urn7"},
			},
		},
		{
			name: "all dependencies (some empty)",
			given: &State{
				Provider:     "provider",
				Parent:       "urn1",
				Dependencies: []URN{"urn2", ""},
				PropertyDependencies: map[PropertyKey][]URN{
					"key1": {"", "urn3"},
					"key2": {"urn4"},
				},
				DeletedWith: "",
			},
			wantProvider: "provider",
			wantAllDependencies: []StateDependency{
				{Type: ResourceParent, URN: "urn1"},
				{Type: ResourceDependency, URN: "urn2"},
				{Type: ResourcePropertyDependency, Key: "key1", URN: "urn3"},
				{Type: ResourcePropertyDependency, Key: "key2", URN: "urn4"},
			},
		},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			// Act.
			actualProvider, actualAllDeps := c.given.GetAllDependencies()

			// Assert.
			assert.Equal(t, c.wantProvider, actualProvider)
			assert.ElementsMatch(t, c.wantAllDependencies, actualAllDeps)
		})
	}
}
