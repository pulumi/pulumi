// Copyright 2016-2024, Pulumi Corporation.
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

package graph

import (
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
)

func TestDependencyGraph(t *testing.T) {
	t.Parallel()

	// Arrange.
	providerA, providerARef := makeProvider("pkg", "providerA", "0")

	a1 := &resource.State{URN: "a1", Provider: providerARef}
	a2 := &resource.State{
		URN:          "a2",
		Provider:     providerARef,
		Dependencies: []resource.URN{a1.URN},
	}

	providerB, providerBRef := makeProvider("pkg", "providerB", "1", a1.URN, a2.URN)

	b1 := &resource.State{
		URN:          "b1",
		Provider:     providerBRef,
		Dependencies: []resource.URN{a1.URN},
	}

	c1 := &resource.State{
		URN:          "c1",
		Dependencies: []resource.URN{a2.URN},
	}

	providerD, providerDRef := makeProvider("pkg", "providerD", "2")

	d1 := &resource.State{URN: "d1", Provider: providerDRef}
	d2 := &resource.State{URN: "d2", Parent: d1.URN}
	d3 := &resource.State{URN: "d3", Provider: providerDRef, DeletedWith: d1.URN}
	d4 := &resource.State{URN: "d4", Parent: d2.URN}
	d5 := &resource.State{URN: "d5", Parent: d3.URN}

	e1 := &resource.State{URN: "e1"}
	e2 := &resource.State{URN: "e2", Dependencies: []resource.URN{e1.URN}}
	e3 := &resource.State{
		URN: "e3",
		PropertyDependencies: map[resource.PropertyKey][]resource.URN{
			"e3Prop1": {e1.URN},
		},
	}

	e4 := &resource.State{URN: "e4", Dependencies: []resource.URN{e3.URN}}
	e5 := &resource.State{URN: "e5", DeletedWith: e3.URN}

	f1 := &resource.State{URN: "f1"}
	f2 := &resource.State{URN: "f2", DeletedWith: f1.URN}
	f1D := &resource.State{URN: "f1", Delete: true}

	dg := NewDependencyGraph([]*resource.State{
		providerA,
		a1,
		a2,
		providerB,
		b1,
		c1,
		providerD,
		d1,
		d2,
		d3,
		d4,
		d5,
		e1,
		e2,
		e3,
		e4,
		e5,
		f1,
		f2,
		f1D,
	})

	t.Run("DependingOn", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		cases := []struct {
			name            string
			res             *resource.State
			ignore          map[resource.URN]bool
			includeChildren bool
			expected        []*resource.State
		}{
			{
				name:            "providerA",
				res:             providerA,
				ignore:          nil,
				includeChildren: false,
				expected: []*resource.State{
					a1,        // a1's provider is providerA
					a2,        // a2's provider is providerA; a2 also depends on a1
					providerB, // providerB depends on a1 and a2
					b1,        // b1 depends on a1 and providerB
					c1,        // c1 depends on a2
				},
			},
			{
				name:            "a1",
				res:             a1,
				ignore:          nil,
				includeChildren: false,
				expected: []*resource.State{
					a2,        // a2 depends on a1
					providerB, // providerB depends on a1 and a2
					b1,        // b1 depends on a1 and providerB
					c1,        // c1 depends on a2
				},
			},
			{
				name:            "a2",
				res:             a2,
				ignore:          nil,
				includeChildren: false,
				expected: []*resource.State{
					providerB, // providerB depends on a2
					b1,        // b1's provider is providerB
					c1,        // c1 depends on a2
				},
			},
			{
				name:            "providerB",
				res:             providerB,
				ignore:          nil,
				includeChildren: false,
				expected: []*resource.State{
					b1, // b1's provider is providerB
				},
			},
			{
				name:            "b1",
				res:             b1,
				ignore:          nil,
				includeChildren: false,
				expected:        nil,
			},
			{
				name:            "c1",
				res:             c1,
				ignore:          nil,
				includeChildren: false,
				expected:        nil,
			},
			{
				name:            "providerA ignorning a1",
				res:             providerA,
				ignore:          map[resource.URN]bool{a1.URN: true},
				includeChildren: false,
				expected: []*resource.State{
					a2,        // a2's provider is providerA
					providerB, // providerB depends on a2
					b1,        // b1's provider is providerB
					c1,        // c1 depends on a2
				},
			},
			{
				name:            "providerA ignorning a2",
				res:             providerA,
				ignore:          map[resource.URN]bool{a2.URN: true},
				includeChildren: false,
				expected: []*resource.State{
					a1,        // a1's provider is providerA
					providerB, // providerB depends on a1
					b1,        // b1's provider is providerB
				},
			},
			{
				name:            "providerA ignoring a1 and a2",
				res:             providerA,
				ignore:          map[resource.URN]bool{a1.URN: true, a2.URN: true},
				includeChildren: false,
				expected:        nil,
			},
			{
				name:            "a1 ignoring a2",
				res:             a1,
				ignore:          map[resource.URN]bool{a2.URN: true},
				includeChildren: false,
				expected: []*resource.State{
					providerB, // providerB depends on a1
					b1,        // b1's provider is providerB
				},
			},
			{
				name:            "a1 ignoring a2 and providerB",
				res:             a1,
				ignore:          map[resource.URN]bool{a2.URN: true, providerB.URN: true},
				includeChildren: false,
				expected: []*resource.State{
					b1, // b1 depends on a1
				},
			},
			{
				name:            "a2 ignoring providerB",
				res:             a2,
				ignore:          map[resource.URN]bool{providerB.URN: true},
				includeChildren: false,
				expected: []*resource.State{
					c1, // c1 depends on a2
				},
			},
			{
				name:            "b1 ignoring providerB",
				res:             b1,
				ignore:          map[resource.URN]bool{providerB.URN: true},
				includeChildren: false,
				expected:        nil,
			},
			{
				name:            "providerD",
				res:             providerD,
				ignore:          nil,
				includeChildren: false,
				expected: []*resource.State{
					d1, // d1's provider is providerD
					d3, // d3's provider is providerD; d3 is deleted with d1
				},
			},
			{
				name:            "providerD including parent/child relationships",
				res:             providerD,
				ignore:          nil,
				includeChildren: true,
				expected: []*resource.State{
					d1, // d1's provider is providerD
					d2, // d2 is a child of d1
					d3, // d3's provider is providerD; d3 is deleted with d1
					d4, // d4 is a child of d2
					d5, // d5 is a child of d3
				},
			},
			{
				name:            "providerD ignoring d1, including parent/child relationships",
				res:             providerD,
				ignore:          map[resource.URN]bool{d1.URN: true},
				includeChildren: false,
				expected: []*resource.State{
					d3, // d3's provider is providerD
				},
			},
			{
				name:            "d1",
				res:             d1,
				ignore:          nil,
				includeChildren: false,
				expected: []*resource.State{
					d3, // d3 is deleted with d1
				},
			},
			{
				name:            "d1 including parent/child relationships",
				res:             d1,
				ignore:          nil,
				includeChildren: true,
				expected: []*resource.State{
					d2, // d2 is a child of d1
					d3, // d3 is deleted with d1
					d4, // d4 is a child of d2
					d5, // d5 is a child of d3
				},
			},
			{
				name:            "d1 ignoring d3",
				res:             d1,
				ignore:          map[resource.URN]bool{d3.URN: true},
				includeChildren: false,
				expected:        nil,
			},
			{
				name:            "d2",
				res:             d2,
				ignore:          nil,
				includeChildren: false,
				expected:        nil,
			},
			{
				name:            "e1",
				res:             e1,
				ignore:          nil,
				includeChildren: false,
				expected: []*resource.State{
					e2, // e2 depends on e3
					e3, // e3 has a property dependency on e1
					e4, // e4 depends on e3
					e5, // e5 is deleted with e3
				},
			},
			{
				name:            "e1 ignoring e3",
				res:             e1,
				ignore:          map[resource.URN]bool{e3.URN: true},
				includeChildren: false,
				expected: []*resource.State{
					e2, // e2 depends on e3
				},
			},
			{
				name:            "e3",
				res:             e3,
				ignore:          nil,
				includeChildren: false,
				expected: []*resource.State{
					e4, // e4 depends on e3
					e5, // e5 is deleted with e3
				},
			},
		}

		for _, c := range cases {
			c := c
			t.Run(c.name, func(t *testing.T) {
				t.Parallel()

				// Act.
				actual := dg.DependingOn(c.res, c.ignore, c.includeChildren)

				// Assert.
				assert.Equal(t, c.expected, actual)
			})
		}
	})

	t.Run("OnlyDependsOn", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		cases := []struct {
			name     string
			res      *resource.State
			expected []*resource.State
		}{
			{
				name:     "providerA",
				res:      providerA,
				expected: []*resource.State{a1, a2, providerB, b1, c1},
			},
			{
				name:     "a1",
				res:      a1,
				expected: []*resource.State{a2, providerB, b1, c1},
			},
			{
				name:     "d1",
				res:      d1,
				expected: []*resource.State{d2, d3, d4, d5},
			},
			{
				name:     "e1",
				res:      e1,
				expected: []*resource.State{e2, e3, e4, e5},
			},
		}

		for _, c := range cases {
			c := c
			t.Run(c.name, func(t *testing.T) {
				t.Parallel()

				// Act.
				actual := dg.OnlyDependsOn(c.res)

				// Assert.
				assert.Equal(t, c.expected, actual)
			})
		}
	})

	t.Run("DependenciesOf", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		cases := []struct {
			name     string
			res      *resource.State
			expected mapset.Set[*resource.State]
		}{
			{
				name:     "providerA",
				res:      providerA,
				expected: mapset.NewSet[*resource.State](),
			},
			{
				name: "a1",
				res:  a1,
				expected: mapset.NewSet[*resource.State](
					providerA, // a1's provider is providerA
				),
			},
			{
				name: "a2",
				res:  a2,
				expected: mapset.NewSet[*resource.State](
					providerA, // a2's provider is providerA
					a1,        // a2 depends on a1
				),
			},
			{
				name: "providerB",
				res:  providerB,
				expected: mapset.NewSet[*resource.State](
					a1, // providerB depends on a1
					a2, // providerB depends on a2
				),
			},
			{
				name: "b1",
				res:  b1,
				expected: mapset.NewSet[*resource.State](
					providerB, // b1's provider is providerB
					a1,        // b1 depends on a1
				),
			},
			{
				name: "c1",
				res:  c1,
				expected: mapset.NewSet[*resource.State](
					a2, // c1 depends on a2
				),
			},
			{
				name:     "providerD",
				res:      providerD,
				expected: mapset.NewSet[*resource.State](),
			},
			{
				name: "d1",
				res:  d1,
				expected: mapset.NewSet[*resource.State](
					providerD, // d1's provider is providerD
				),
			},
			{
				name: "d2",
				res:  d2,
				expected: mapset.NewSet[*resource.State](
					d1, // d2 is a child of d1
				),
			},
			{
				name: "d3",
				res:  d3,
				expected: mapset.NewSet[*resource.State](
					providerD, // d3's provider is providerD
					d1,        // d3 is deleted with d1
					d2,        // d2 is a child of d1
				),
			},
			{
				name:     "e1",
				res:      e1,
				expected: mapset.NewSet[*resource.State](),
			},
			{
				name: "e2",
				res:  e2,
				expected: mapset.NewSet[*resource.State](
					e1, // e2 depends on e1
				),
			},
			{
				name: "e3",
				res:  e3,
				expected: mapset.NewSet[*resource.State](
					e1, // e3 has a property dependency on e1
				),
			},
			{
				name: "e4",
				res:  e4,
				expected: mapset.NewSet[*resource.State](
					e3, // e4 depends on e3
				),
			},
			{
				name: "e5",
				res:  e5,
				expected: mapset.NewSet[*resource.State](
					e3, // e5 is deleted with e3
				),
			},
		}

		for _, c := range cases {
			c := c
			t.Run(c.name, func(t *testing.T) {
				t.Parallel()

				// Act.
				actual := dg.DependenciesOf(c.res)

				// Assert.
				if !c.expected.Equal(actual) {
					assert.Failf(t, "expected and actual do not match", "expected: %v\nactual  : %v", c.expected, actual)
				}
			})
		}
	})

	t.Run("TransitiveDependenciesOf", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		cases := []struct {
			name     string
			res      *resource.State
			expected mapset.Set[*resource.State]
		}{
			{
				name:     "providerA",
				res:      providerA,
				expected: mapset.NewSet[*resource.State](),
			},
			{
				name: "a1",
				res:  a1,
				expected: mapset.NewSet[*resource.State](
					providerA, // a1's provider is providerA
				),
			},
			{
				name: "a2",
				res:  a2,
				expected: mapset.NewSet[*resource.State](
					providerA, // a2's provider is providerA
					a1,        // a2 depends on a1
				),
			},
			{
				name: "providerB",
				res:  providerB,
				expected: mapset.NewSet[*resource.State](
					a1,        // providerB depends on a1
					a2,        // providerB depends on a2
					providerA, // (transitive) providerA is a1's and a2's provider
				),
			},
			{
				name: "b1",
				res:  b1,
				expected: mapset.NewSet[*resource.State](
					providerB, // b1's provider is providerB
					a1,        // b1 depends on a1
					a2,        // (transitive) providerB depends on a2
					providerA, // (transitive) providerA is a1's and a2's provider
				),
			},
			{
				name: "c1",
				res:  c1,
				expected: mapset.NewSet[*resource.State](
					a2,        // c1 depends on a2
					providerA, // (transitive) providerA is a2's provider
					a1,        // (transitive) a2 depends on a1
				),
			},
			{
				name:     "providerD",
				res:      providerD,
				expected: mapset.NewSet[*resource.State](),
			},
			{
				name: "d1",
				res:  d1,
				expected: mapset.NewSet[*resource.State](
					providerD, // d1's provider is providerD
				),
			},
			{
				name: "d2",
				res:  d2,
				expected: mapset.NewSet[*resource.State](
					d1,        // d2 is a child of d1
					providerD, // (transitive) d1's provider is providerD
				),
			},
			{
				name: "d3",
				res:  d3,
				expected: mapset.NewSet[*resource.State](
					providerD, // d3's provider is providerD
					d1,        // d3 is deleted with d1
				),
			},
			{
				name:     "e1",
				res:      e1,
				expected: mapset.NewSet[*resource.State](),
			},
			{
				name: "e2",
				res:  e2,
				expected: mapset.NewSet[*resource.State](
					e1, // e2 depends on e1
				),
			},
			{
				name: "e3",
				res:  e3,
				expected: mapset.NewSet[*resource.State](
					e1, // e3 has a property dependency on e1
				),
			},
			{
				name: "e4",
				res:  e4,
				expected: mapset.NewSet[*resource.State](
					e3, // e4 depends on e3
					e1, // (transitive) e3 has a property dependency on e1
				),
			},
			{
				name: "e5",
				res:  e5,
				expected: mapset.NewSet[*resource.State](
					e3, // e5 is deleted with e3
					e1, // (transitive) e3 has a property dependency on e1
				),
			},
			{
				name:     "f1",
				res:      f1,
				expected: mapset.NewSet[*resource.State](),
			},
			{
				name: "f2",
				res:  f2,
				expected: mapset.NewSet[*resource.State](
					f1, // f2 is deleted with f1
				),
			},
		}

		for _, c := range cases {
			c := c
			t.Run(c.name, func(t *testing.T) {
				t.Parallel()

				// Act.
				actual := dg.TransitiveDependenciesOf(c.res)

				// Assert.
				if !c.expected.Equal(actual) {
					assert.Failf(t, "expected and actual do not match", "expected: %v\nactual  : %v", c.expected, actual)
				}
			})
		}
	})

	providerF1, providerF1Ref := makeProvider("pkg", "providerF", "3")
	providerF2, providerF2Ref := makeProvider("pkg", "providerF", "4")
	providerF3, providerF3Ref := makeProvider("pkg", "providerF", "5")

	fx1 := &resource.State{URN: "fx", Provider: providerF1Ref}
	fx2 := &resource.State{URN: "fx", Provider: providerF2Ref}

	fy := &resource.State{URN: "fy", Provider: providerF3Ref, Dependencies: []resource.URN{fx1.URN}}
	fz := &resource.State{
		URN:      "fz",
		Provider: providerF3Ref,
		PropertyDependencies: map[resource.PropertyKey][]resource.URN{
			"fzProp1": {fx1.URN},
		},
	}

	fw := &resource.State{
		URN:         "fw",
		Provider:    providerF3Ref,
		DeletedWith: fx1.URN,
	}

	dgOnly := NewDependencyGraph([]*resource.State{
		providerF1,
		providerF2,
		providerF3,
		fx1,
		fx2,
		fy,
		fz,
		fw,
	})

	t.Run("OnlyDependsOn", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		cases := []struct {
			name     string
			res      *resource.State
			expected []*resource.State
		}{
			{
				name: "providerF1",
				res:  providerF1,
				expected: []*resource.State{
					fx1,
				},
			},
			{
				name: "providerF2",
				res:  providerF2,
				expected: []*resource.State{
					fx2,
				},
			},
			{
				name: "providerF3",
				res:  providerF3,
				expected: []*resource.State{
					fy,
					fz,
					fw,
				},
			},
			{
				name:     "fx1",
				res:      fx1,
				expected: nil,
			},
			{
				name:     "fx2",
				res:      fx2,
				expected: nil,
			},
			{
				name:     "fy1",
				res:      fy,
				expected: nil,
			},
		}

		for _, c := range cases {
			c := c
			t.Run(c.name, func(t *testing.T) {
				t.Parallel()

				// Act.
				actual := dgOnly.OnlyDependsOn(c.res)

				// Assert.
				assert.Equal(t, c.expected, actual)
			})
		}
	})

	t.Run("ParentsOf", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		cases := []struct {
			name     string
			res      *resource.State
			expected []*resource.State
		}{
			{
				name:     "e2",
				res:      e2,
				expected: []*resource.State{},
			},
			{
				name:     "d2",
				res:      d2,
				expected: []*resource.State{d1},
			},
			{
				name:     "d4",
				res:      d4,
				expected: []*resource.State{d2, d1},
			},
			{
				name:     "d5",
				res:      d5,
				expected: []*resource.State{d3},
			},
		}

		for _, c := range cases {
			c := c
			t.Run(c.name, func(t *testing.T) {
				t.Parallel()

				// Act.
				actual := dg.ParentsOf(c.res)

				// Assert.
				assert.Equal(t, c.expected, actual)
			})
		}
	})

	t.Run("ChildrenOf", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		cases := []struct {
			name     string
			res      *resource.State
			expected []*resource.State
		}{
			{
				name:     "d1",
				res:      d1,
				expected: []*resource.State{d2, d4},
			},
			{
				name:     "d2",
				res:      d2,
				expected: []*resource.State{d4},
			},
			{
				name:     "d3",
				res:      d3,
				expected: []*resource.State{d5},
			},
			{
				name:     "e1",
				res:      e1,
				expected: []*resource.State{},
			},
		}

		for _, c := range cases {
			c := c
			t.Run(c.name, func(t *testing.T) {
				t.Parallel()

				// Act.
				actual := dg.ChildrenOf(c.res)

				// Assert.
				assert.Equal(t, c.expected, actual)
			})
		}
	})

	t.Run("Contains", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		cases := []struct {
			name     string
			res      *resource.State
			expected bool
		}{
			{
				name:     "a1",
				res:      a1,
				expected: true,
			},
			{
				name:     "fx1",
				res:      fx1,
				expected: false,
			},
		}

		for _, c := range cases {
			c := c
			t.Run(c.name, func(t *testing.T) {
				t.Parallel()

				// Act.
				actual := dg.Contains(c.res)

				// Assert.
				assert.Equal(t, c.expected, actual)
			})
		}
	})
}

func makeProvider(pkg, name, id string, deps ...resource.URN) (*resource.State, string) {
	t := providers.MakeProviderType(tokens.Package(pkg))

	provider := &resource.State{
		Type:         t,
		URN:          resource.NewURN("stack", "proj", "", t, name),
		ID:           resource.ID(id),
		Inputs:       resource.PropertyMap{},
		Outputs:      resource.PropertyMap{},
		Dependencies: deps,
	}

	providerRef, err := providers.NewReference(provider.URN, provider.ID)
	if err != nil {
		panic(err)
	}

	return provider, providerRef.String()
}
