// Copyright 2016-2021, Pulumi Corporation.
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

// Model-checks dependency_graph functionality against simple models
// using property-based testing.
//
// Currently this assumes a simplified model of `resource.State`
// relevant to dependency calculations; `dependency_graph` only
// accesses these fields:
//
//     type State struct {
//         Dependencies []resource.URN
//         URN          resource.URN
//         Parent       resource.URN
//         Provider     string
//         Custom       bool
//     }
//
// At the moment only `Custom=true` (Custom, not Component) resources
// are tested.
package graph

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Models ------------------------------------------------------------------------------------------

var isParent R = func(child, parent *resource.State) bool {
	return child.Parent == parent.URN
}

var hasProvider R = func(res, provider *resource.State) bool {
	return resource.URN(res.Provider) == provider.URN
}

var hasDependency R = func(res, dependency *resource.State) bool {
	for _, dep := range res.Dependencies {
		if dependency.URN == dep {
			return true
		}
	}
	return false
}

var expectedDependenciesOf R = union(isParent, hasProvider, hasDependency)

// Verify `DependneciesOf` against `expectedDependenciesOf`.
func TestRapidDependenciesOf(t *testing.T) {
	t.Parallel()

	graphCheck(t, func(t *rapid.T, universe []*resource.State) {
		dg := NewDependencyGraph(universe)
		for _, a := range universe {
			aD := dg.DependenciesOf(a)
			for _, b := range universe {
				if isParent(a, b) {
					assert.Truef(t, aD[b],
						"DependenciesOf(%v) is missing a parent %v",
						a.URN, b.URN)
				}
				if hasProvider(a, b) {
					assert.Truef(t, aD[b],
						"DependenciesOf(%v) is missing a provider %v",
						a.URN, b.URN)
				}
				if hasDependency(a, b) {
					assert.Truef(t, aD[b],
						"DependenciesOf(%v) is missing a dependecy %v",
						a.URN, b.URN)
				}
				if aD[b] {
					assert.True(t, expectedDependenciesOf(a, b),
						"DependenciesOf(%v) includes an unexpected %v",
						a.URN, b.URN)
				}
			}
		}
	})
}

// Additionally verify no immediate loops in `DependenciesOf`, no `B
// in DependenciesOf(A) && A in DependenciesOf(B)`.
func TestRapidDependenciesOfAntisymmetric(t *testing.T) {
	t.Parallel()

	graphCheck(t, func(t *rapid.T, universe []*resource.State) {
		dg := NewDependencyGraph(universe)
		for _, a := range universe {
			aD := dg.DependenciesOf(a)
			for _, b := range universe {
				bD := dg.DependenciesOf(b)
				assert.Falsef(t, aD[b] && bD[a],
					"DependenciesOf symmetric over (%v, %v)", a.URN, b.URN)
			}
		}
	})
}

// Model `DependingOn`.
func expectedDependingOn(universe []*resource.State, includeChildren bool) R {
	if !includeChildren {
		// TODO currently DependingOn is not the inverse transitive
		// closure of `dependenciesOf`. Should this be
		// `expectedDependenciesOf`?
		restrictedDependenciesOf := union(hasProvider, hasDependency)
		return inverse(transitively(universe)(restrictedDependenciesOf))
	}

	// TODO this extends base `expectedDependingOn` with
	// immediate children. Should it be a transitive closure?
	dependingOn := expectedDependingOn(universe, false)
	return func(a, b *resource.State) bool {
		if dependingOn(a, b) || isParent(b, a) {
			return true
		}
		for _, x := range universe {
			if dependingOn(x, b) && isParent(x, a) {
				return true
			}
		}
		return false
	}
}

// Verify `DependingOn` against `expectedDependingOn`. Note that
// `DependingOn` is specialised with an empty ignore map, the ignore
// map is not tested yet.
func TestRapidDependingOn(t *testing.T) {
	t.Parallel()

	test := func(t *rapid.T, universe []*resource.State, includingChildren bool) {
		expected := expectedDependingOn(universe, includingChildren)
		dg := NewDependencyGraph(universe)
		dependingOn := func(a, b *resource.State) bool {
			for _, x := range dg.DependingOn(a, nil, includingChildren) {
				if b.URN == x.URN {
					return true
				}
			}
			return false
		}
		for _, a := range universe {
			for _, b := range universe {
				actual := dependingOn(a, b)
				assert.Equalf(t, expected(a, b), actual,
					"Unexpected %v in dg.DependingOn(%v) = %v",
					b.URN, a.URN, actual)
			}
		}
	}

	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, includingChildren := range []bool{false, true} {
		includingChildren := includingChildren
		t.Run(fmt.Sprintf("includingChildren=%v", includingChildren), func(t *testing.T) {
			t.Parallel()

			graphCheck(t, func(t *rapid.T, universe []*resource.State) {
				test(t, universe, includingChildren)
			})
		})
	}
}

// Verify `DependingOn` results are ordered, if `D1` in
// `DependingOn(D2)` then `D1` appears before `D2`.
func TestRapidDependingOnOrdered(t *testing.T) {
	t.Parallel()

	test := func(t *rapid.T, universe []*resource.State, includingChildren bool) {
		expectedDependingOn := expectedDependingOn(universe, includingChildren)
		dg := NewDependencyGraph(universe)
		for _, a := range universe {
			depOnA := dg.DependingOn(a, nil, includingChildren)
			for d1i, d1 := range depOnA {
				for d2i, d2 := range depOnA {
					if expectedDependingOn(d2, d1) {
						require.Truef(t, d2i < d1i,
							"%v should appear before %v",
							d2.URN, d1.URN)
					}
				}
			}
		}
	}

	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, includingChildren := range []bool{false, true} {
		includingChildren := includingChildren
		t.Run(fmt.Sprintf("includingChildren=%v", includingChildren), func(t *testing.T) {
			t.Parallel()

			graphCheck(t, func(t *rapid.T, universe []*resource.State) {
				test(t, universe, includingChildren)
			})
		})
	}
}

func TestRapidTransitiveDependenciesOf(t *testing.T) {
	t.Parallel()

	graphCheck(t, func(t *rapid.T, universe []*resource.State) {
		expectedInTDepsOf := transitively(universe)(expectedDependenciesOf)
		dg := NewDependencyGraph(universe)
		for _, a := range universe {
			tda := dg.TransitiveDependenciesOf(a)
			for _, b := range universe {
				assert.Equalf(t,
					expectedInTDepsOf(a, b),
					tda[b],
					"Mismatch on a=%v, b=%b",
					a.URN,
					b.URN)
			}
		}
	})
}

// Generators --------------------------------------------------------------------------------------

// Generates ordered values of type `[]ResourceState` that:
//
// - Have unique URNs
// - May reference preceding resouces in the slice as r.Parent
// - May reference preceding resouces in the slice in r.Dependencies
//
// In other words these slices conform with `NewDependencyGraph`
// ordering assumptions. There is a tradedoff: generated values will
// not test any error-checking code in `NewDependencyGraph`, but will
// more efficiently explore more complicated properties on the valid
// subspace of inputs.
//
// What is not currently done but may need to be extended:
//
// - Support Component resources
// - Support non-nil r.Provider references
func resourceStateSliceGenerator() *rapid.Generator {
	urnGen := rapid.StringMatching(`urn:pulumi:a::b::c:d:e::[abcd][123]`)

	stateGen := rapid.Custom(func(t *rapid.T) *resource.State {
		urn := urnGen.Draw(t, "URN").(string)
		return &resource.State{
			Custom: true,
			URN:    resource.URN(urn),
		}
	})

	getUrn := func(st *resource.State) resource.URN { return st.URN }

	statesGen := rapid.SliceOfDistinct(stateGen, getUrn)

	return rapid.Custom(func(t *rapid.T) []*resource.State {
		states := statesGen.Draw(t, "states").([]*resource.State)

		randInt := rapid.IntRange(-len(states), len(states))

		for i, r := range states {
			// Any resource at index `i` may want to declare `j < i` as parent.
			// Sample negative `j` to means "no parent".
			j := randInt.Draw(t, fmt.Sprintf("j%d", i)).(int)
			if j >= 0 && j < i {
				r.Parent = states[j].URN
			}
			// Similarly we can depend on resources defined prior.
			deps := rapid.SliceOfDistinct(
				randInt,
				func(i int) int { return i },
			).Draw(t, fmt.Sprintf("deps%d", i)).([]int)
			for _, dep := range deps {
				if dep >= 0 && dep < i {
					r.Dependencies = append(r.Dependencies, states[dep].URN)
				}
			}
		}

		return states
	})
}

// Helper code: relations --------------------------------------------------------------------------

// Shorthand for relations over `*resource.State`
type R = func(a, b *resource.State) bool

// Union of one or more relations.
func union(rs ...R) R {
	return func(a, b *resource.State) bool {
		for _, r := range rs {
			if r(a, b) {
				return true
			}
		}
		return false
	}
}

// Flips the relation, `inverse(R)(a,b) = R(b,a)`.
func inverse(r R) R {
	return func(a, b *resource.State) bool {
		return r(b, a)
	}
}

// Memoized transitive closure of a relation.
func transitively(universe []*resource.State) func(R) R {
	return func(rel R) R {
		trel := make(map[*resource.State]map[*resource.State]bool)
		for _, a := range universe {
			trel[a] = make(map[*resource.State]bool)
			for _, b := range universe {
				if rel(a, b) {
					trel[a][b] = true
				}
			}
		}

		extend := func() bool {
			more := false
			for _, a := range universe {
				for _, b := range universe {
					if !trel[a][b] {
						for _, x := range universe {
							if trel[x][b] && rel(a, x) {
								trel[a][b] = true
								more = true
							}
						}
					}
				}
			}
			return more
		}

		for extend() {
		}

		return func(a, b *resource.State) bool {
			return trel[a][b]
		}
	}
}

// Helper code: misc -------------------------------------------------------------------------------

func printState(w io.Writer, st *resource.State) {
	fmt.Fprintf(w, "%s", st.URN)
	if st.Parent != "" {
		fmt.Fprintf(w, " parent=%s", st.Parent)
	}
	if len(st.Dependencies) > 0 {
		fmt.Fprintf(w, " deps=[")
		for _, d := range st.Dependencies {
			fmt.Fprintf(w, "%s, ", d)
		}
		fmt.Fprintf(w, "]")
	}
	fmt.Fprintf(w, "\n")
}

func showStates(sts []*resource.State) string {
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "[\n\n")
	for _, st := range sts {
		printState(buf, st)
		fmt.Fprintf(buf, "\n\n")
	}
	fmt.Fprintf(buf, "]")
	return buf.String()
}

func graphCheck(t *testing.T, check func(*rapid.T, []*resource.State)) {
	rss := resourceStateSliceGenerator()
	rapid.Check(t, func(t *rapid.T) {
		universe := rss.Draw(t, "universe").([]*resource.State)
		t.Logf("Checking universe: %s", showStates(universe))
		check(t, universe)
	})
}
