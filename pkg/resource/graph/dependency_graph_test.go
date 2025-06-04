// Copyright 2016-2025, Pulumi Corporation.
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
	"fmt"
	"os"
	"strings"
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
	d6 := &resource.State{URN: "d6", Custom: true}
	d7 := &resource.State{URN: "d7", Parent: d6.URN}
	d8 := &resource.State{URN: "d8", DeletedWith: d6.URN}
	d9 := &resource.State{URN: "d9", Parent: d8.URN}

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
		// The "a", "b", and "c" resources are here to test basic dependencies -- providers, dependencies, etc. including
		// transitive overlaps (e.g. where X depends on Z both directly and through an intermediate Y).
		providerA, a1, a2,
		providerB, b1,
		c1,

		// The "d" resources are here to test parent/child relationship and how they factor into dependencies. This includes
		// the fact that, when a resource depends on a component (Custom: false), the children of the component that appear
		// before that resource should also be considered its dependencies (see commentary in dependency_graph.go for more
		// information).
		providerD, d1, d2, d3, d4, d5, d6, d7, d8, d9,

		// The "e" resources exercise the "other" kinds of dependencies -- property dependencies, deleted with, etc.
		e1, e2, e3, e4, e5,

		// The "f" resources test dependency tracking when there are multiple resources with the same URN due to one being
		// scheduled for deletion (Delete: true, typically as part of a replace).
		f1, f2, f1D,
	})

	// These tests are written explicitly (that is, not using a "table test") in order to make it easier to debug using
	// e.g. VSCode by clicking "Debug test", rather than looping through a large number of cases with conditional
	// breakpoints etc.

	t.Run("DependingOn", func(t *testing.T) {
		t.Parallel()

		t.Run("providerA", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{
				a1,        // a1's provider is providerA
				a2,        // a2's provider is providerA; a2 also depends on a1
				providerB, // providerB depends on a1 and a2
				b1,        // b1 depends on a1 and providerB
				c1,        // c1 depends on a2
			}

			// Act.
			actual := dg.DependingOn(providerA, nil /*ignore*/, false /*includeChildren*/)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("a1", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{
				a2,        // a2 depends on a1
				providerB, // providerB depends on a1 and a2
				b1,        // b1 depends on a1 and providerB
				c1,        // c1 depends on a2
			}

			// Act.
			actual := dg.DependingOn(a1, nil /*ignore*/, false /*includeChildren*/)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("a2", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{
				providerB, // providerB depends on a2
				b1,        // b1's provider is providerB
				c1,        // c1 depends on a2
			}

			// Act.
			actual := dg.DependingOn(a2, nil /*ignore*/, false /*includeChildren*/)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("providerB", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{
				b1, // b1's provider is providerB
			}

			// Act.
			actual := dg.DependingOn(providerB, nil /*ignore*/, false /*includeChildren*/)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("b1", func(t *testing.T) {
			t.Parallel()

			// Act.
			actual := dg.DependingOn(b1, nil /*ignore*/, false /*includeChildren*/)

			// Assert.
			assertSameStates(t, nil, actual)
		})

		t.Run("c1", func(t *testing.T) {
			t.Parallel()

			// Act.
			actual := dg.DependingOn(c1, nil /*ignore*/, false /*includeChildren*/)

			// Assert.
			assertSameStates(t, nil, actual)
		})

		t.Run("providerA ignorning a1", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{
				a2,        // a2's provider is providerA
				providerB, // providerB depends on a2
				b1,        // b1's provider is providerB
				c1,        // c1 depends on a2
			}

			// Act.
			actual := dg.DependingOn(providerA, map[resource.URN]bool{a1.URN: true}, false /*includeChildren*/)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("providerA ignorning a2", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{
				a1,        // a1's provider is providerA
				providerB, // providerB depends on a1
				b1,        // b1's provider is providerB
			}

			// Act.
			actual := dg.DependingOn(providerA, map[resource.URN]bool{a2.URN: true}, false /*includeChildren*/)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("providerA ignoring a1 and a2", func(t *testing.T) {
			t.Parallel()

			// Act.
			actual := dg.DependingOn(providerA, map[resource.URN]bool{a1.URN: true, a2.URN: true}, false /*includeChildren*/)

			// Assert.
			assertSameStates(t, nil, actual)
		})

		t.Run("a1 ignoring a2", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{
				providerB, // providerB depends on a1
				b1,        // b1's provider is providerB
			}

			// Act.
			actual := dg.DependingOn(a1, map[resource.URN]bool{a2.URN: true}, false /*includeChildren*/)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("a1 ignoring a2 and providerB", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{
				b1, // b1 depends on a1
			}

			// Act.
			actual := dg.DependingOn(a1, map[resource.URN]bool{a2.URN: true, providerB.URN: true}, false /*includeChildren*/)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("a2 ignoring providerB", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{
				c1, // c1 depends on a2
			}

			// Act.
			actual := dg.DependingOn(a2, map[resource.URN]bool{providerB.URN: true}, false /*includeChildren*/)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("b1 ignoring providerB", func(t *testing.T) {
			t.Parallel()

			// Act.
			actual := dg.DependingOn(b1, map[resource.URN]bool{providerB.URN: true}, false /*includeChildren*/)

			// Assert.
			assertSameStates(t, nil, actual)
		})

		t.Run("providerD", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{
				d1, // d1's provider is providerD
				d3, // d3's provider is providerD; d3 is deleted with d1
			}

			// Act.
			actual := dg.DependingOn(providerD, nil /*ignore*/, false /*includeChildren*/)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("providerD including parent/child relationships", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{
				d1, // d1's provider is providerD
				d2, // d2 is a child of d1
				d3, // d3's provider is providerD; d3 is deleted with d1
				d4, // d4 is a child of d2
				d5, // d5 is a child of d3
			}

			// Act.
			actual := dg.DependingOn(providerD, nil /*ignore*/, true /*includeChildren*/)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("providerD ignoring d1, including parent/child relationships", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{
				d3, // d3's provider is providerD
			}

			// Act.
			actual := dg.DependingOn(providerD, map[resource.URN]bool{d1.URN: true}, false /*includeChildren*/)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("d1", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{
				d3, // d3 is deleted with d1
			}

			// Act.
			actual := dg.DependingOn(d1, nil /*ignore*/, false /*includeChildren*/)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("d1 including parent/child relationships", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{
				d2, // d2 is a child of d1
				d3, // d3 is deleted with d1
				d4, // d4 is a child of d2
				d5, // d5 is a child of d3
			}

			// Act.
			actual := dg.DependingOn(d1, nil /*ignore*/, true /*includeChildren*/)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("d1 ignoring d3", func(t *testing.T) {
			t.Parallel()

			// Act.
			actual := dg.DependingOn(d1, map[resource.URN]bool{d3.URN: true}, false /*includeChildren*/)

			// Assert.
			assertSameStates(t, nil, actual)
		})

		t.Run("d2", func(t *testing.T) {
			t.Parallel()

			// Act.
			actual := dg.DependingOn(d2, nil /*ignore*/, false /*includeChildren*/)

			// Assert.
			assertSameStates(t, nil, actual)
		})

		t.Run("e1", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{
				e2, // e2 depends on e3
				e3, // e3 has a property dependency on e1
				e4, // e4 depends on e3
				e5, // e5 is deleted with e3
			}

			// Act.
			actual := dg.DependingOn(e1, nil /*ignore*/, false /*includeChildren*/)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("e1 ignoring e3", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{
				e2, // e2 depends on e3
			}

			// Act.
			actual := dg.DependingOn(e1, map[resource.URN]bool{e3.URN: true}, false /*includeChildren*/)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("e3", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{
				e4, // e4 depends on e3
				e5, // e5 is deleted with e3
			}

			// Act.
			actual := dg.DependingOn(e3, nil /*ignore*/, false /*includeChildren*/)

			// Assert.
			assertSameStates(t, expected, actual)
		})
	})

	t.Run("OnlyDependsOn", func(t *testing.T) {
		t.Parallel()

		t.Run("providerA", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{a1, a2, providerB, b1, c1}

			// Act.
			actual := dg.OnlyDependsOn(providerA)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("a1", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{a2, providerB, b1, c1}

			// Act.
			actual := dg.OnlyDependsOn(a1)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("a2", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{providerB, b1, c1}

			// Act.
			actual := dg.OnlyDependsOn(a2)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("e1", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{e2, e3, e4, e5}

			// Act.
			actual := dg.OnlyDependsOn(e1)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		// The point of OnlyDependsOn is to avoid finding resources which might depend on multiple resources with the same
		// URN (but different IDs, for instance). This happens particularly in the case where a provider might be being
		// upgraded, for example. The following test cases bulk out the suite for OnlyDependsOn to really exercise this
		// behaviour.

		providerF1, providerF1Ref := makeProvider("pkg", "providerF", "1")
		providerF2, providerF2Ref := makeProvider("pkg", "providerF", "2")
		providerF3, providerF3Ref := makeProvider("pkg", "providerF", "3")

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

		fu := &resource.State{URN: "fu", Provider: providerF3Ref}
		fv := &resource.State{
			URN:         "fv",
			Provider:    providerF3Ref,
			DeletedWith: fu.URN,
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
			fu,
			fv,
		})

		t.Run("providerF1", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{fx1}

			// Act.
			actual := dgOnly.OnlyDependsOn(providerF1)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("providerF2", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{fx2}

			// Act.
			actual := dgOnly.OnlyDependsOn(providerF2)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("providerF3", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{
				fy,
				fz,

				// BUG: fw actually depends on fx1, which depends on providerF1, so should not be returned.
				fw,
				fu,
				fv,
			}

			// Act.
			actual := dgOnly.OnlyDependsOn(providerF3)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("fx1", func(t *testing.T) {
			t.Parallel()

			// Act.
			actual := dgOnly.OnlyDependsOn(fx1)

			// Assert.
			assertSameStates(t, nil, actual)
		})

		t.Run("fx2", func(t *testing.T) {
			t.Parallel()

			// Act.
			actual := dgOnly.OnlyDependsOn(fx2)

			// Assert.
			assertSameStates(t, nil, actual)
		})

		t.Run("fy1", func(t *testing.T) {
			t.Parallel()

			// Act.
			actual := dgOnly.OnlyDependsOn(fy)

			// Assert.
			assertSameStates(t, nil, actual)
		})
	})

	t.Run("DependenciesOf", func(t *testing.T) {
		t.Parallel()

		t.Run("providerA", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet[*resource.State]()

			// Act.
			actual := dg.DependenciesOf(providerA)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("a1", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(providerA)

			// Act.
			actual := dg.DependenciesOf(
				a1, // a1's provider is providerA
			)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("a2", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				providerA, // a2's provider is providerA
				a1,        // a2 depends on a1
			)

			// Act.
			actual := dg.DependenciesOf(a2)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("providerB", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				a1, // providerB depends on a1
				a2, // providerB depends on a2
			)

			// Act.
			actual := dg.DependenciesOf(providerB)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("b1", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				providerB, // b1's provider is providerB
				a1,        // b1 depends on a1
			)

			// Act.
			actual := dg.DependenciesOf(b1)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("c1", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				a2, // c1 depends on a2
			)

			// Act.
			actual := dg.DependenciesOf(c1)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("providerD", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet[*resource.State]()

			// Act.
			actual := dg.DependenciesOf(providerD)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("d1", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				providerD, // d1's provider is providerD
			)

			// Act.
			actual := dg.DependenciesOf(d1)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("d2", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				d1, // d2 is a child of d1
			)

			// Act.
			actual := dg.DependenciesOf(d2)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("d3", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				providerD, // d3's provider is providerD
				d1,        // d3 is deleted with d1 (a component)
				d2,        // d2 is a child of d1 that appears before d3
			)

			// Act.
			actual := dg.DependenciesOf(d3)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("d6", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet[*resource.State]()

			// Act.
			actual := dg.DependenciesOf(d6)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("d7", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				d6, // d7 is a child of d6
			)

			// Act.
			actual := dg.DependenciesOf(d7)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("d8", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				d6, // d8 is deleted with d6
				// d7 does not appear as a child of d6 because d6 is not a component
			)

			// Act.
			actual := dg.DependenciesOf(d8)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("e1", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet[*resource.State]()

			// Act.
			actual := dg.DependenciesOf(e1)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("e2", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				e1, // e2 depends on e1
			)

			// Act.
			actual := dg.DependenciesOf(e2)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("e3", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				e1, // e3 has a property dependency on e1
			)

			// Act.
			actual := dg.DependenciesOf(e3)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("e4", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				e3, // e4 depends on e3
			)

			// Act.
			actual := dg.DependenciesOf(e4)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("e5", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				e3, // e5 is deleted with e3
			)

			// Act.
			actual := dg.DependenciesOf(e5)

			// Assert.
			assertSameSets(t, expected, actual)
		})
	})

	t.Run("TransitiveDependenciesOf", func(t *testing.T) {
		t.Parallel()

		t.Run("providerA", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet[*resource.State]()

			// Act.
			actual := dg.TransitiveDependenciesOf(providerA)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("a1", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				providerA, // a1's provider is providerA
			)

			// Act.
			actual := dg.TransitiveDependenciesOf(a1)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("a2", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				providerA, // a2's provider is providerA
				a1,        // a2 depends on a1
			)

			// Act.
			actual := dg.TransitiveDependenciesOf(a2)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("providerB", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				a1,        // providerB depends on a1
				a2,        // providerB depends on a2
				providerA, // (transitive) providerA is a1's and a2's provider
			)

			// Act.
			actual := dg.TransitiveDependenciesOf(providerB)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("b1", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				providerB, // b1's provider is providerB
				a1,        // b1 depends on a1
				a2,        // (transitive) providerB depends on a2
				providerA, // (transitive) providerA is a1's and a2's provider
			)

			// Act.
			actual := dg.TransitiveDependenciesOf(b1)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("c1", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				a2,        // c1 depends on a2
				providerA, // (transitive) providerA is a2's provider
				a1,        // (transitive) a2 depends on a1
			)

			// Act.
			actual := dg.TransitiveDependenciesOf(c1)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("providerD", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet[*resource.State]()

			// Act.
			actual := dg.TransitiveDependenciesOf(providerD)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("d1", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				providerD, // d1's provider is providerD
			)

			// Act.
			actual := dg.TransitiveDependenciesOf(d1)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("d2", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				d1,        // d2 is a child of d1
				providerD, // (transitive) d1's provider is providerD
			)

			// Act.
			actual := dg.TransitiveDependenciesOf(d2)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("d3", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				providerD, // d3's provider is providerD
				d1,        // d3 is deleted with d1 (a component)

				// BUG: Not currently true
				// d2,        // d2 is a child of d1
			)

			// Act.
			actual := dg.TransitiveDependenciesOf(d3)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("d8", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				d6, // d8 is deleted with d6
				// d7 does not appear as a child of d6 because d6 is not a component
			)

			// Act.
			actual := dg.TransitiveDependenciesOf(d8)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("d9", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				d8, // d9 is a child of d8
				d6, // (transitive) d8 is deleted with d6
				// d7 does not appear as a child of d6 because d6 is not a component
			)

			// Act.
			actual := dg.TransitiveDependenciesOf(d9)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("e1", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet[*resource.State]()

			// Act.
			actual := dg.TransitiveDependenciesOf(e1)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("e2", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				e1, // e2 depends on e1
			)

			// Act.
			actual := dg.TransitiveDependenciesOf(e2)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("e3", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				e1, // e3 has a property dependency on e1
			)

			// Act.
			actual := dg.TransitiveDependenciesOf(e3)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("e4", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				e3, // e4 depends on e3
				e1, // (transitive) e3 has a property dependency on e1
			)

			// Act.
			actual := dg.TransitiveDependenciesOf(e4)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("e5", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				e3, // e5 is deleted with e3
				e1, // (transitive) e3 has a property dependency on e1
			)

			// Act.
			actual := dg.TransitiveDependenciesOf(e5)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("f1", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet[*resource.State]()

			// Act.
			actual := dg.TransitiveDependenciesOf(f1)

			// Assert.
			assertSameSets(t, expected, actual)
		})

		t.Run("f2", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := mapset.NewSet(
				f1, // f2 is deleted with f1
			)

			// Act.
			actual := dg.TransitiveDependenciesOf(f2)

			// Assert.
			assertSameSets(t, expected, actual)
		})
	})

	t.Run("ParentsOf", func(t *testing.T) {
		t.Parallel()

		t.Run("e2", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{}

			// Act.
			actual := dg.ParentsOf(e2)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("d2", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{d1}

			// Act.
			actual := dg.ParentsOf(d2)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("d4", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{d2, d1}

			// Act.
			actual := dg.ParentsOf(d4)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("d5", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{d3}

			// Act.
			actual := dg.ParentsOf(d5)

			// Assert.
			assertSameStates(t, expected, actual)
		})
	})

	t.Run("ChildrenOf", func(t *testing.T) {
		t.Parallel()

		t.Run("d1", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{d2, d4}

			// Act.
			actual := dg.ChildrenOf(d1)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("d2", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{d4}

			// Act.
			actual := dg.ChildrenOf(d2)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("d3", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{d5}

			// Act.
			actual := dg.ChildrenOf(d3)

			// Assert.
			assertSameStates(t, expected, actual)
		})

		t.Run("e1", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			expected := []*resource.State{}

			// Act.
			actual := dg.ChildrenOf(e1)

			// Assert.
			assertSameStates(t, expected, actual)
		})
	})

	t.Run("Contains", func(t *testing.T) {
		t.Parallel()

		t.Run("in", func(t *testing.T) {
			t.Parallel()

			// Act.
			actual := dg.Contains(a1)

			// Assert.
			assert.True(t, actual)
		})

		t.Run("out", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			fx1 := &resource.State{URN: "fx1"}

			// Act.
			actual := dg.Contains(fx1)

			// Assert.
			assert.False(t, actual)
		})
	})
}

func makeProvider(pkg, name, id string, deps ...resource.URN) (*resource.State, string) {
	t := providers.MakeProviderType(tokens.Package(pkg))

	provider := &resource.State{
		Type:         t,
		Custom:       true,
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

func assertSameSets(t *testing.T, expected mapset.Set[*resource.State], actual mapset.Set[*resource.State]) {
	if !expected.Equal(actual) {
		assert.Failf(t, "expected and actual do not match",
			"expected:\n\n%s\nactual\n\n%s", prettyResourceSet(expected), prettyResourceSet(actual))
	}
}

func assertSameStates(t *testing.T, expecteds []*resource.State, actuals []*resource.State) {
	ne := len(expecteds)
	na := len(actuals)
	assert.Equal(t, ne, na, "different numbers of expected and actual states")

	for i := 0; i < max(ne, na); i++ {
		var expected, actual *resource.State
		if i < ne {
			expected = expecteds[i]
		}
		if i < na {
			actual = actuals[i]
		}
		assert.Samef(t, expected, actual, "expected and actual states do not match\n"+
			"expected:\n\n%s\nactual:\n\n%s", prettyResource(expected), prettyResource(actual))
	}
}

func prettyResource(r *resource.State) string {
	if r == nil {
		return "<nil>"
	}

	return fmt.Sprintf("%s\n  %v\n", r.URN, r)
}

func prettyResourceSet(s mapset.Set[*resource.State]) string {
	var sb strings.Builder
	for r := range s.Iter() {
		sb.WriteString(prettyResource(r))
	}
	return sb.String()
}

// TestPerf can be used for roughly testing the performance of the dependency graph implementation -- construction as
// well as its operations. We deliberately don't use a Bench*/*testing.B benchmark here because we don't want to loop
// all the various suboperations -- we just want a rough number that we can re-run to see if anything has gotten worse.
//
// Because this test can be slow, it will not run automatically. You'll want to set the PULUMI_DEPENDENCY_GRAPH_PERF
// environment variable to run it. For example:
//
//	(cd pkg/resource/graph; PULUMI_DEPENDENCY_GRAPH_PERF=1 go test ./... -v -run 'TestPerf')
//
// To run a specific configuration (i.e., a certain number of resources, with a certain number of iterations), you can
// pass a more specific test pattern:
//
//	  (cd pkg/resource/graph; PULUMI_DEPENDENCY_GRAPH_PERF=1 \
//			  go test ./... -v -run 'TestPerf/Linear/Size=10000/Iterations=5000')
//
// As with other tests in this file, these tests are written explicitly to aid in debugging with e.g. VSCode's "Debug
// Test" feature.
//
// Note: these tests *do not* exercise correctness! Any changes to the dependency graph implementation should be
// validated with the other tests in this file. These tests are purely for performance.
func TestPerf(t *testing.T) {
	shouldPerf := os.Getenv("PULUMI_DEPENDENCY_GRAPH_PERF")
	if shouldPerf == "" {
		t.Skip("PULUMI_DEPENDENCY_GRAPH_PERF not set")
	}

	// "Linear" is a linear dependency graph, where each resource depends on the one before it in the list. This makes for
	// long chains, but means that efficient algorithms which are careful to avoid accidentally quadratic behaviour will
	// still perform well.

	t.Run("Linear", func(t *testing.T) {
		t.Parallel()

		t.Run("Size=100", func(t *testing.T) {
			t.Parallel()

			t.Run("Iterations=1000", func(t *testing.T) {
				t.Parallel()

				testPerfOf(t, 100, linearResourceGraph, 1_000)
			})

			t.Run("Iterations=5000", func(t *testing.T) {
				t.Parallel()

				testPerfOf(t, 100, linearResourceGraph, 5_000)
			})
		})

		t.Run("Size=1000", func(t *testing.T) {
			t.Parallel()

			t.Run("Iterations=1000", func(t *testing.T) {
				t.Parallel()

				testPerfOf(t, 1_000, linearResourceGraph, 1_000)
			})

			t.Run("Iterations=5000", func(t *testing.T) {
				t.Parallel()

				testPerfOf(t, 1_000, linearResourceGraph, 5_000)
			})
		})

		t.Run("Size=10000", func(t *testing.T) {
			t.Parallel()

			t.Run("Iterations=1000", func(t *testing.T) {
				t.Parallel()

				testPerfOf(t, 10_000, linearResourceGraph, 1_000)
			})

			t.Run("Iterations=5000", func(t *testing.T) {
				t.Parallel()

				testPerfOf(t, 10_000, linearResourceGraph, 5_000)
			})
		})

		t.Run("Size=25000", func(t *testing.T) {
			t.Parallel()

			t.Run("Iterations=1000", func(t *testing.T) {
				t.Parallel()

				testPerfOf(t, 25_000, linearResourceGraph, 1_000)
			})

			t.Run("Iterations=5000", func(t *testing.T) {
				t.Parallel()

				testPerfOf(t, 25_000, linearResourceGraph, 5_000)
			})
		})
	})

	// "Triangle" sets up a triangular resource graph, where each resource depends on all of those that came before it. In
	// many ways, this is the worst case for most algorithms, since if each of N resources has a number of dependencies
	// approaching N, we are forced into an N^2 situation either at construction or later traversal. It's expected
	// therefore that some portion of these tests will really drag at larger numbers.

	t.Run("Triangle", func(t *testing.T) {
		t.Parallel()

		t.Run("Size=100", func(t *testing.T) {
			t.Parallel()

			t.Run("Iterations=1000", func(t *testing.T) {
				t.Parallel()

				testPerfOf(t, 100, triangleResourceGraph, 1_000)
			})

			t.Run("Iterations=5000", func(t *testing.T) {
				t.Parallel()

				testPerfOf(t, 100, triangleResourceGraph, 5_000)
			})
		})

		t.Run("Size=1000", func(t *testing.T) {
			t.Parallel()

			t.Run("Iterations=1000", func(t *testing.T) {
				t.Parallel()

				testPerfOf(t, 1_000, triangleResourceGraph, 1_000)
			})

			t.Run("Iterations=5000", func(t *testing.T) {
				t.Parallel()

				testPerfOf(t, 1_000, triangleResourceGraph, 5_000)
			})
		})

		t.Run("Size=10000", func(t *testing.T) {
			t.Parallel()

			t.Run("Iterations=1000", func(t *testing.T) {
				t.Parallel()

				testPerfOf(t, 10_000, triangleResourceGraph, 1_000)
			})

			t.Run("Iterations=5000", func(t *testing.T) {
				t.Parallel()

				testPerfOf(t, 10_000, triangleResourceGraph, 5_000)
			})
		})

		t.Run("Size=25000", func(t *testing.T) {
			t.Parallel()

			t.Run("Iterations=1000", func(t *testing.T) {
				t.Parallel()

				testPerfOf(t, 25_000, triangleResourceGraph, 1_000)
			})

			t.Run("Iterations=5000", func(t *testing.T) {
				t.Parallel()

				testPerfOf(t, 25_000, triangleResourceGraph, 5_000)
			})
		})
	})
}

func testPerfOf(
	t *testing.T,
	resourceCount int,
	resourcesF func(n int) []*resource.State,
	iterations int,
) {
	t.Helper()

	resources := resourcesF(resourceCount)

	var dg *DependencyGraph
	t.Run("NewDependencyGraph", func(t *testing.T) {
		dg = NewDependencyGraph(resources)
	})

	low := resources[0]
	mid := resources[resourceCount/2]
	high := resources[resourceCount-1]

	t.Run("DependenciesOf/low", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.DependenciesOf(low)
		}
	})
	t.Run("DependenciesOf/mid", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.DependenciesOf(mid)
		}
	})
	t.Run("DependenciesOf/high", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.DependenciesOf(high)
		}
	})

	t.Run("ParentsOf/low", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.ParentsOf(low)
		}
	})
	t.Run("ParentsOf/mid", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.ParentsOf(mid)
		}
	})
	t.Run("ParentsOf/high", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.ParentsOf(high)
		}
	})

	t.Run("ChildrenOf/low", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.ChildrenOf(low)
		}
	})
	t.Run("ChildrenOf/mid", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.ChildrenOf(mid)
		}
	})
	t.Run("ChildrenOf/high", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.ChildrenOf(high)
		}
	})

	t.Run("TransitiveDependenciesOf/low", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.TransitiveDependenciesOf(low)
		}
	})
	t.Run("TransitiveDependenciesOf/mid", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.TransitiveDependenciesOf(mid)
		}
	})
	t.Run("TransitiveDependenciesOf/high", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.TransitiveDependenciesOf(high)
		}
	})

	t.Run("Contains/low", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.Contains(low)
		}
	})
	t.Run("Contains/mid", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.Contains(mid)
		}
	})
	t.Run("Contains/high", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.Contains(high)
		}
	})

	t.Run("DependingOn/low", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.DependingOn(low, nil, false)
		}
	})
	t.Run("DependingOn/mid", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.DependingOn(mid, nil, false)
		}
	})
	t.Run("DependingOn/high", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.DependingOn(high, nil, false)
		}
	})

	t.Run("DependingOnWithChildren/low", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.DependingOn(low, nil, true)
		}
	})
	t.Run("DependingOnWithChildren/mid", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.DependingOn(mid, nil, true)
		}
	})
	t.Run("DependingOnWithChildren/high", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.DependingOn(high, nil, true)
		}
	})

	t.Run("OnlyDependsOn/low", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.OnlyDependsOn(low)
		}
	})
	t.Run("OnlyDependsOn/mid", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.OnlyDependsOn(mid)
		}
	})
	t.Run("OnlyDependsOn/high", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			dg.OnlyDependsOn(high)
		}
	})
}

// linearResourceGraph creates a set of resources where the ith resource depends on the i-1th resource (via a parent
// relationship).
func linearResourceGraph(n int) []*resource.State {
	resources := make([]*resource.State, n)
	for i := 0; i < n; i++ {
		resources[i] = &resource.State{
			URN: resource.URN(fmt.Sprintf("urn%d", i)),
		}

		if i > 0 {
			resources[i].Parent = resources[i-1].URN
		}
	}
	return resources
}

// triangleResourceGraph creates a set of resources where the ith resource depends on all previous resources (via a
// dependencies list).
func triangleResourceGraph(n int) []*resource.State {
	resources := make([]*resource.State, n)
	for i := 0; i < n; i++ {
		r := &resource.State{
			URN: resource.URN(fmt.Sprintf("urn%d", i)),
		}

		r.Dependencies = make([]resource.URN, i)
		for j := i - 1; j >= 0; j-- {
			r.Dependencies[i-j-1] = resources[j].URN
		}

		resources[i] = r
	}
	return resources
}
