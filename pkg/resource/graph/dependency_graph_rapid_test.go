// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

package graph

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

// Assume a simplified model of resource.State since dependency_graph
// only accesses these fields:
//
//   type State struct {
// 	Dependencies []resource.URN
// 	URN          resource.URN
// 	Parent       resource.URN
// 	Provider     string
//   }

// Let us model the relations of interest as predicates. Start with the obvious relation:
func isParent(child, parent *resource.State) bool {
	return child.Parent == parent.URN
}

// Closure over `isParent`. Needs a closed universe.
func isDescendant(universe []*resource.State) R {
	return transitively(universe)(isParent)
}

func hasProvider(res, provider *resource.State) bool {
	return resource.URN(res.Provider) == provider.URN
}

func directlyDependsOn(a, b *resource.State) bool {
	for _, dep := range a.Dependencies {
		if b.URN == dep {
			return true
		}
	}
	return false
}

// If `directlyDependsOn(A, B)`, then also `dependsOnDescendant(A, D)`
// for any `isDescendant(D, B)`, `A != D`.
func dependsOnDescendant(universe []*resource.State) func(a, d *resource.State) bool {
	isDesc := isDescendant(universe)
	return func(a, d *resource.State) bool {
		if a.URN == d.URN {
			return false
		}
		for _, b := range universe {
			if directlyDependsOn(a, b) && isDesc(d, b) {
				return true
			}
		}
		return false
	}
}

// Verify a `DependneciesOf` model in terms of the relations above.
func TestRapidDependenciesOf(t *testing.T) {
	graphCheck(t, func(t *rapid.T, universe []*resource.State) {
		dg := NewDependencyGraph(universe)
		reachable1 := union(isParent, hasProvider, directlyDependsOn, dependsOnDescendant(universe))
		reachable := transitively(universe)(reachable1)
		dependenciesOf := func(a, b *resource.State) bool { return dg.DependenciesOf(a)[b] }
		transitiveDependenciesOf := transitively(universe)(dependenciesOf)
		depOnDescendant := dependsOnDescendant(universe)
		for _, a := range universe {
			aD := dg.DependenciesOf(a)
			for _, b := range universe {
				if isParent(a, b) {
					assert.Truef(t, aD[b], "DependenciesOf(%v) is missing a parent %v", a.URN, b.URN)
				}
				if hasProvider(a, b) {
					assert.Truef(t, aD[b], "DependenciesOf(%v) is missing a provider %v", a.URN, b.URN)
				}
				if directlyDependsOn(a, b) {
					assert.Truef(t, aD[b], "DependenciesOf(%v) is missing a direct dependecy %v", a.URN, b.URN)
				}
				if depOnDescendant(a, b) && !aD[b] {
					// Allow ignoring results in
					// `DependenciesOf` that would
					// have caused a loop.
					loop := reachable(b, a) && transitiveDependenciesOf(b, a)
					if !loop {
						assert.Truef(t, aD[b],
							"DependenciesOf(%v) is missing a descendant dependency on %v",
							a.URN, b.URN)
					}
				}
				if aD[b] {
					assert.True(t, reachable1(a, b), "DependenciesOf(%v) includes an unreachable %v",
						a.URN, b.URN)
				}
			}
		}
	})
}

// Additionally verify no immediate loops in `DependenciesOf`, no `B
// in DependenciesOf(A) && A in DependenciesOf(B)`.
func TestRapidDependenciesOfAntisymmetric(t *testing.T) {
	graphCheck(t, func(t *rapid.T, universe []*resource.State) {
		dg := NewDependencyGraph(universe)
		for _, a := range universe {
			aD := dg.DependenciesOf(a)
			for _, b := range universe {
				bD := dg.DependenciesOf(b)
				assert.Falsef(t, aD[b] && bD[a], "DependenciesOf symmetric over (%v, %v)", a.URN, b.URN)
			}
		}
	})
}

func TestRapidDependenciesOfAntisymmetricExample(t *testing.T) {
	a1 := res("a1", "")
	a2 := res("a2", "a1", "a1")
	a3 := res("a3", "a1")
	b1 := res("b1", "a2")
	dg := NewDependencyGraph([]*resource.State{a1, a2, a3, b1})
	// b1 should depend on a2 its parent
	assert.Contains(t, dg.DependenciesOf(b1), a2)
	// but a2 should not depend on b1
	assert.NotContains(t, dg.DependenciesOf(a2), b1)
}

// Slightly degenerate case illustrating a cycle (self-reference) in DependingOn.
func TestDependingOnExcludesSelf(t *testing.T) {
	a1 := res("a1", "")
	a2 := res("a2", "a1")
	a3 := res("a3", "a2", "a1")
	a4 := res("a4", "a3")
	dg := NewDependencyGraph([]*resource.State{a1, a2, a3, a4})
	assert.NotContains(t, dg.DependingOn(a4, nil), a4)
}

// Test that `DependingOn` is the inverse transitive closure of `DependsOn`.
func TestRapidDependingOn(t *testing.T) {

	graphCheck(t, func(t *rapid.T, universe []*resource.State) {
		dg := NewDependencyGraph(universe)
		dependenciesOf := func(a, b *resource.State) bool { return dg.DependenciesOf(a)[b] }
		transitiveDependenciesOf := transitively(universe)(dependenciesOf)
		dependingOn := func(a, b *resource.State) bool {
			for _, x := range dg.DependingOn(a, nil) {
				if b.URN == x.URN {
					return true
				}
			}
			return false
		}
		for _, a := range universe {
			for _, b := range universe {
				if dependingOn(a, b) {
					assert.Truef(t, transitiveDependenciesOf(b, a),
						"dependingOn(%v, %v) but not transitiveDependenciesOf(%v, %v)",
						a.URN, b.URN, b.URN, a.URN)
				}
				if transitiveDependenciesOf(b, a) {
					assert.True(t, dependingOn(a, b),
						"transitiveDependenciesOf(%v, %v) but not dependingOn(%v, %v)",
						b.URN, a.URN, a.URN, b.URN)
				}
			}
		}
	})
}

// Test that `DependingOn` results are ordered according to the dep graph.
func TestRapidDependingOnOrdered(t *testing.T) {
	graphCheck(t, func(t *rapid.T, universe []*resource.State) {
		dg := NewDependencyGraph(universe)
		dependenciesOf := func(a, b *resource.State) bool { return dg.DependenciesOf(a)[b] }
		transitiveDependenciesOf := transitively(universe)(dependenciesOf)
		for _, a := range universe {
			for _, b := range universe {
				if transitiveDependenciesOf(a, b) {
					t.Logf("transitiveDependenciesOf(%v, %v)", a.URN, b.URN)
				}
			}
		}
		for _, a := range universe {
			depOnA := dg.DependingOn(a, nil)
			t.Logf("Inspecting DependingOn(%v) = %v", a.URN, showStates(depOnA))
			for d1i, d1 := range depOnA {
				for d2i, d2 := range depOnA {
					if transitiveDependenciesOf(d1, d2) {
						assert.Truef(t, d2i < d1i, "%v should appear before %v",
							d2.URN, d1.URN)
						if !(d2i < d1i) {
							t.FailNow()
						}
					}
				}
			}
		}
	})
}

// Helper code below -------------------------------------------------------------------------------

// Helper for constructing a resource State for a test.
func res(urnSuffix string, parentUrnSuffix string, depsUrnSuffixes ...string) *resource.State {
	toUrn := func(x string) resource.URN {
		return resource.URN(fmt.Sprintf("urn:pulumi:a::b::c:d:e::%s", x))
	}

	urn := toUrn(urnSuffix)
	var parent resource.URN
	if parentUrnSuffix != "" {
		parent = toUrn(parentUrnSuffix)
	}

	var deps []resource.URN
	for _, d := range depsUrnSuffixes {
		if d != "" {
			deps = append(deps, toUrn(d))
		}
	}

	return &resource.State{URN: urn, Parent: parent, Dependencies: deps}
}

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

// Memoizes a relation as a table.
func memo(universe []*resource.State) func(R) R {
	return func(rel R) R {
		mrel := make(map[*resource.State]map[*resource.State]bool)
		for _, a := range universe {
			mrel[a] = make(map[*resource.State]bool)
			for _, b := range universe {
				if rel(a, b) {
					mrel[a][b] = true
				}
			}
		}
		return func(a, b *resource.State) bool {
			return mrel[a][b]
		}
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

// Generates values of type `[]ResourceState`.
func resourceStateSliceGenerator() *rapid.Generator {
	urnGen := rapid.StringMatching(`urn:pulumi:a::b::c:d:e::[abcd][123]`)
	stateGen := rapid.Custom(func(t *rapid.T) *resource.State {
		return &resource.State{URN: resource.URN(urnGen.Draw(t, "URN").(string))}
	})
	statesGen := rapid.SliceOfDistinct(stateGen, func(st *resource.State) resource.URN { return st.URN })

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
