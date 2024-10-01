// Copyright 2022-2024, Pulumi Corporation.
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
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func createSnapshot() Snapshot {
	resourceUrns := []resource.URN{
		resource.NewURN("stack", "test", "typ", "aws:resource", "bar"),
		resource.NewURN("stack", "test", "typ", "aws:resource", "aname"),
		resource.NewURN("stack", "test", "typ", "azure:resource", "bar"),
	}
	resources := []*resource.State{}
	for _, u := range resourceUrns {
		resources = append(resources, &resource.State{URN: u})
	}
	return Snapshot{Resources: resources}
}

func createSnapshotPtr() *Snapshot {
	s := createSnapshot()
	return &s
}

func TestSnapshotNormalizeURNReferences(t *testing.T) {
	t.Parallel()
	s1 := createSnapshotPtr()
	s1n, err := s1.NormalizeURNReferences()
	assert.NoError(t, err)
	assert.Same(t, s1, s1n)

	s2 := createSnapshotPtr()
	r0 := s2.Resources[0]
	r0.Aliases = []resource.URN{r0.URN}
	s2.Resources[2].Parent = r0.URN
	r0.URN += "!"
	s2n, err := s2.NormalizeURNReferences()
	assert.NoError(t, err)
	assert.NotSame(t, s2, s2n)
	// before normalize in s2, Parent link uses outdated URL
	assert.Equal(t, s2.Resources[2].Parent+"!", s2.Resources[0].URN)
	// after normalize in s2n, Parent link uses the real URL rewritten via aliases
	assert.Equal(t, s2n.Resources[2].Parent, s2n.Resources[0].URN)
}

func TestSnapshotWithUpdatedResources(t *testing.T) {
	t.Parallel()
	s1 := createSnapshotPtr()

	s := s1.withUpdatedResources(func(r *resource.State) *resource.State {
		return r
	})
	assert.Same(t, s, s1)

	s = s1.withUpdatedResources(func(r *resource.State) *resource.State {
		out := r.Copy()
		out.URN += "!"
		return out
	})
	assert.NotSame(t, s, s1)
	assert.Equal(t, s1.Resources[0].URN+"!", s.Resources[0].URN)
}

func TestSnapshotToposort_PreservesValidSnapshots(t *testing.T) {
	t.Parallel()

	// Arrange.
	cases := []struct {
		name  string
		given []*resource.State
	}{
		{
			name:  "empty",
			given: []*resource.State{},
		},
		{
			name: "a single resource",
			given: []*resource.State{
				{URN: "urn:pulumi:stack::project::t::a"},
			},
		},
		{
			name: "two unrelated resources",
			given: []*resource.State{
				{URN: "urn:pulumi:stack::project::t::a"},
				{URN: "urn:pulumi:stack::project::t::b"},
			},
		},
		{
			name: "two resources with a valid provider dependency",
			given: []*resource.State{
				{
					Type: "pulumi:providers:p",
					URN:  "urn:pulumi:stack::project::pulumi:providers:p::a",
					ID:   "id",
				},
				{
					URN:      "urn:pulumi:stack::project::t::b",
					Provider: "urn:pulumi:stack::project::pulumi:providers:p::a::id",
				},
			},
		},
		{
			name: "two resources with a valid parent-child relationship",
			given: []*resource.State{
				{URN: "urn:pulumi:stack::project::t::a"},
				{URN: "urn:pulumi:stack::project::t$t::b", Parent: "urn:pulumi:stack::project::t::a"},
			},
		},
		{
			name: "two resources with a valid dependency",
			given: []*resource.State{
				{URN: "urn:pulumi:stack::project::t::a"},
				{URN: "urn:pulumi:stack::project::t::b", Dependencies: []resource.URN{"urn:pulumi:stack::project::t::a"}},
			},
		},
		{
			name: "two resources with a valid property dependency",
			given: []*resource.State{
				{URN: "urn:pulumi:stack::project::t::a"},
				{
					URN: "urn:pulumi:stack::project::t::b",
					PropertyDependencies: map[resource.PropertyKey][]resource.URN{
						"p": {"urn:pulumi:stack::project::t::a"},
					},
				},
			},
		},
		{
			name: "two resources with a valid deleted-with relationship",
			given: []*resource.State{
				{URN: "urn:pulumi:stack::project::t::a"},
				{URN: "urn:pulumi:stack::project::t::b", DeletedWith: "urn:pulumi:stack::project::t::a"},
			},
		},
		{
			name: "duplicate URNs due to deleted/non-deleted",
			given: []*resource.State{
				{URN: "urn:pulumi:stack::project::t::a"},
				{
					URN:          "urn:pulumi:stack::project::t::b",
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::a"},
				},
				{
					URN:    "urn:pulumi:stack::project::t::a",
					Delete: true,
				},
				{
					URN:          "urn:pulumi:stack::project::t::b",
					Delete:       true,
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::a"},
				},
			},
		},
		{
			name: "duplicate URNs due to deleted/non-deleted (false cycle)",
			given: []*resource.State{
				{URN: "urn:pulumi:stack::project::t::a"},
				{
					URN:          "urn:pulumi:stack::project::t::b",
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::a"},
				},
				{
					URN:          "urn:pulumi:stack::project::t::a",
					Delete:       true,
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::b"},
				},
			},
		},
		{
			name: "multiple duplicate URNs due to multiple deleted",
			given: []*resource.State{
				{URN: "urn:pulumi:stack::project::t::a"},
				{
					URN:          "urn:pulumi:stack::project::t::b",
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::a"},
				},
				{
					URN:          "urn:pulumi:stack::project::t::a",
					Delete:       true,
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::b"},
				},
				{
					URN:          "urn:pulumi:stack::project::t::c",
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::a"},
					Delete:       true,
				},
				{
					URN:          "urn:pulumi:stack::project::t::a",
					Delete:       true,
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::b"},
				},
			},
		},
		{
			name: "multiple sets of dependent resources",
			given: []*resource.State{
				{URN: "urn:pulumi:stack::project::t::a"},
				{
					URN:          "urn:pulumi:stack::project::t::b",
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::a"},
				},
				{
					URN:          "urn:pulumi:stack::project::t::c",
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::a"},
				},
				{
					URN: "urn:pulumi:stack::project::t::d",
					PropertyDependencies: map[resource.PropertyKey][]resource.URN{
						"pa": {"urn:pulumi:stack::project::t::a"},
						"pb": {"urn:pulumi:stack::project::t::b"},
					},
				},
				{
					URN: "urn:pulumi:stack::project::t::e",
					Dependencies: []resource.URN{
						"urn:pulumi:stack::project::t::c",
						"urn:pulumi:stack::project::t::d",
					},
				},
				{
					URN: "urn:pulumi:stack::project::t::f",
				},
				{
					URN:         "urn:pulumi:stack::project::t::g",
					DeletedWith: "urn:pulumi:stack::project::t::f",
				},
			},
		},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			snap := &Snapshot{Resources: c.given}
			assert.NoError(t, snap.VerifyIntegrity())

			// Act.
			err := snap.Toposort()

			// Assert.
			assert.NoError(t, err)
			assert.NoError(t, snap.VerifyIntegrity())
		})
	}
}

func TestSnapshotToposort_FixesOrderInvalidSnapshots(t *testing.T) {
	t.Parallel()

	// Arrange.
	cases := []struct {
		name  string
		given []*resource.State
	}{
		{
			name: "two resources with an out-of-order provider dependency",
			given: []*resource.State{
				{
					URN:      "urn:pulumi:stack::project::t::b",
					Provider: "urn:pulumi:stack::project::pulumi:providers:p::a::id",
				},
				{
					Type: "pulumi:providers:p",
					URN:  "urn:pulumi:stack::project::pulumi:providers:p::a",
					ID:   "id",
				},
			},
		},
		{
			name: "two resources with an out-of-order parent-child relationship",
			given: []*resource.State{
				{URN: "urn:pulumi:stack::project::t$t::b", Parent: "urn:pulumi:stack::project::t::a"},
				{URN: "urn:pulumi:stack::project::t::a"},
			},
		},
		{
			name: "two resources with an out-of-order dependency",
			given: []*resource.State{
				{URN: "urn:pulumi:stack::project::t::b", Dependencies: []resource.URN{"urn:pulumi:stack::project::t::a"}},
				{URN: "urn:pulumi:stack::project::t::a"},
			},
		},
		{
			name: "two resources with an out-of-order property dependency",
			given: []*resource.State{
				{
					URN: "urn:pulumi:stack::project::t::b",
					PropertyDependencies: map[resource.PropertyKey][]resource.URN{
						"p": {"urn:pulumi:stack::project::t::a"},
					},
				},
				{URN: "urn:pulumi:stack::project::t::a"},
			},
		},
		{
			name: "two resources with an out-of-order deleted-with relationship",
			given: []*resource.State{
				{URN: "urn:pulumi:stack::project::t::b", DeletedWith: "urn:pulumi:stack::project::t::a"},
				{URN: "urn:pulumi:stack::project::t::a"},
			},
		},
		{
			name: "duplicate URNs due to deleted/non-deleted",
			given: []*resource.State{
				{
					URN:          "urn:pulumi:stack::project::t::b",
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::a"},
				},
				{URN: "urn:pulumi:stack::project::t::a"},
				{
					URN:          "urn:pulumi:stack::project::t::b",
					Delete:       true,
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::a"},
				},
				{
					URN:    "urn:pulumi:stack::project::t::a",
					Delete: true,
				},
			},
		},
		{
			name: "duplicate URNs due to deleted/non-deleted (false cycle)",
			given: []*resource.State{
				{
					URN:          "urn:pulumi:stack::project::t::b",
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::a"},
				},
				{URN: "urn:pulumi:stack::project::t::a"},
				{
					URN:          "urn:pulumi:stack::project::t::a",
					Delete:       true,
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::b"},
				},
			},
		},
		{
			name: "multiple duplicate URNs due to multiple deleted",
			given: []*resource.State{
				{
					URN:          "urn:pulumi:stack::project::t::b",
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::a"},
				},
				{URN: "urn:pulumi:stack::project::t::a"},
				{
					URN:          "urn:pulumi:stack::project::t::c",
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::a"},
					Delete:       true,
				},
				{
					URN:          "urn:pulumi:stack::project::t::a",
					Delete:       true,
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::d"},
				},
				{
					URN:          "urn:pulumi:stack::project::t::a",
					Delete:       true,
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::b"},
				},
				{
					URN:    "urn:pulumi:stack::project::t::d",
					Delete: true,
				},
			},
		},
		{
			name: "multiple sets of out-of-order dependent resources",
			given: []*resource.State{
				{
					URN:          "urn:pulumi:stack::project::t::c",
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::a"},
				},
				{URN: "urn:pulumi:stack::project::t::a"},
				{
					URN: "urn:pulumi:stack::project::t::e",
					Dependencies: []resource.URN{
						"urn:pulumi:stack::project::t::c",
						"urn:pulumi:stack::project::t::d",
					},
				},
				{
					URN:          "urn:pulumi:stack::project::t::b",
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::a"},
				},
				{
					URN: "urn:pulumi:stack::project::t::d",
					PropertyDependencies: map[resource.PropertyKey][]resource.URN{
						"pa": {"urn:pulumi:stack::project::t::a"},
						"pb": {"urn:pulumi:stack::project::t::b"},
					},
				},
				{
					URN:         "urn:pulumi:stack::project::t::g",
					DeletedWith: "urn:pulumi:stack::project::t::f",
				},
				{
					URN: "urn:pulumi:stack::project::t::f",
				},
			},
		},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			snap := &Snapshot{Resources: c.given}
			assert.Error(t, snap.VerifyIntegrity())

			// Act.
			err := snap.Toposort()

			// Assert.
			assert.NoError(t, err)
			assert.NoError(t, snap.VerifyIntegrity())
		})
	}
}

func TestSnapshotToposort_DetectsCycles(t *testing.T) {
	t.Parallel()

	// Arrange.
	cases := []struct {
		name  string
		given []*resource.State
	}{
		{
			name: "direct cycle",
			given: []*resource.State{
				{
					URN:      "urn:pulumi:stack::project::t::b",
					Provider: "urn:pulumi:stack::project::pulumi:providers:p::a::id",
				},
				{
					Type:         "pulumi:providers:p",
					URN:          "urn:pulumi:stack::project::pulumi:providers:p::a",
					ID:           "id",
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::b"},
				},
			},
		},
		{
			name: "long-chain cycle",
			given: []*resource.State{
				{URN: "urn:pulumi:stack::project::t$t::b", Parent: "urn:pulumi:stack::project::t::a"},
				{
					URN:          "urn:pulumi:stack::project::t::a",
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::c"},
				},
				{
					URN:         "urn:pulumi:stack::project::t::c",
					DeletedWith: "urn:pulumi:stack::project::t$t::b",
				},
			},
		},
		{
			name: "two cycles",
			given: []*resource.State{
				{
					URN:          "urn:pulumi:stack::project::t::b",
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::a"},
				},
				{
					URN:         "urn:pulumi:stack::project::t::a",
					DeletedWith: "urn:pulumi:stack::project::t::b",
				},
				{
					URN: "urn:pulumi:stack::project::t::d",
					PropertyDependencies: map[resource.PropertyKey][]resource.URN{
						"pc": {"urn:pulumi:stack::project::t::c"},
					},
				},
				{
					URN:          "urn:pulumi:stack::project::t::c",
					Dependencies: []resource.URN{"urn:pulumi:stack::project::t::d"},
				},
			},
		},
		{
			name: "self reference",
			given: []*resource.State{
				{
					URN: "urn:pulumi:stack::project::t::a",
					PropertyDependencies: map[resource.PropertyKey][]resource.URN{
						"p": {"urn:pulumi:stack::project::t::a"},
					},
				},
			},
		},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			snap := &Snapshot{Resources: c.given}

			// Act.
			err := snap.Toposort()

			// Assert.
			assert.ErrorContains(t, err, "snapshot has cyclic dependencies")
		})
	}
}
