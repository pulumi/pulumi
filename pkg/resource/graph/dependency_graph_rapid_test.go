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
func isDescendant(descendant, ancestor *resource.State, universe []*resource.State) bool {
	if descendant.Parent == "" {
		return false
	}

	if isParent(descendant, ancestor) {
		return true
	}

	for _, x := range universe {
		if isParent(descendant, x) && isDescendant(x, ancestor, universe) {
			return true
		}
	}

	return false
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

// This relation models the notion that if A directly depends on
// B, then it also `dependsOnDescendant` for any descendant of B.
func dependsOnDescendant(a, descendant *resource.State, universe []*resource.State) bool {
	for _, b := range universe {
		if directlyDependsOn(a, b) && isDescendant(descendant, b, universe) {
			return true
		}
	}
	return false
}

// Our primary notion of dependency.
func dependsPrimary(a, b *resource.State, universe []*resource.State) bool {

	return directlyDependsOn(a, b) ||
		isParent(a, b) ||
		hasProvider(a, b) ||
		dependsOnDescendant(a, b, universe)
}

func dependsOn(a, b *resource.State, universe []*resource.State) bool {
	if a == b {
		return false
	}

	if directlyDependsOn(a, b) ||
		isParent(a, b) ||
		hasProvider(a, b) {
		return true
	}

	aDependsOnB := dependsPrimary(a, b, universe)
	bDependsOnA := dependsPrimary(a, b, universe)

	if aDependsOnB && bDependsOnA {
		return false
	}

	return aDependsOnB
}

// Transitive closure over `dependsOn`.
func transitivelyDependsOn(a, b *resource.State, universe []*resource.State) bool {
	if dependsOn(a, b, universe) {
		return true
	}
	for _, x := range universe {
		if dependsOn(a, x, universe) && transitivelyDependsOn(x, b, universe) {
			return true
		}
	}
	return false
}

// Test that `DependenciesOf` is computing the `dependsOn` relation.
func TestRapidDependenciesOf(t *testing.T) {
	rss := resourceStateSliceGenerator()
	rapid.Check(t, func(t *rapid.T) {
		universe := rss.Draw(t, "universe").([]*resource.State)
		t.Logf("Checking universe: %s", showStates(universe))
		dg := NewDependencyGraph(universe)

		for _, res := range universe {
			resDeps := dg.DependenciesOf(res)

			for d := range resDeps {
				if !dependsOn(res, d, universe) {
					t.Errorf("dg.DependenciesOf(%s) includes %s, but !dependsOn(%s, %s)",
						res.URN, d.URN, res.URN, d.URN)
				}
			}

			for _, d := range universe {
				if dependsOn(res, d, universe) && !resDeps[d] {
					t.Errorf("dg.DependenciesOf(%s) omits %s, but dependsOn(%s, %s)",
						res.URN, d.URN, res.URN, d.URN)
				}
			}
		}
	})
}

func TestRapidDependenciesOfAntisymmetric(t *testing.T) {
	rss := resourceStateSliceGenerator()
	rapid.Check(t, func(t *rapid.T) {
		universe := rss.Draw(t, "universe").([]*resource.State)
		t.Logf("Checking universe: %s", showStates(universe))
		dg := NewDependencyGraph(universe)

		for _, a := range universe {
			aD := dg.DependenciesOf(a)
			for _, b := range universe {
				bD := dg.DependenciesOf(b)
				if aD[b] && bD[a] {
					assert.FailNowf(t, "FAIL",
						"DependenciesOf symmetric over (%v, %v)", a.URN, b.URN)
				}
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

// // Test that `DependingOn` is
// func TestRapidDependingOn(t *testing.T) {
// 	rss := resourceStateSliceGenerator()
// 	rapid.Check(t, func(t *rapid.T) {
// 		ress := rss.Draw(t, "rss").([]*resource.State)

// 		t.Logf("Checking resource-set: %s", showStates(ress))

// 		dg := NewDependencyGraph(ress)

// 		for _, res := range ress {
// 			dependingOn := dg.DependingOn(res, nil)
// 			dependingOnSet := ResourceSet{}

// 			for _, d := range dependingOn {
// 				dependingOnSet[d] = true
// 			}

// 			for d := range dependingOnSet {
// 				if !transitivelyDependsOn(d, res, ress) {
// 					t.Errorf("dg.DependingOn(%s) includes %s, but !transitivelyDependsOn(%s, %s)",
// 						res.URN, d.URN, d.URN, res.URN)
// 				}
// 			}

// 			for _, d := range ress {
// 				if transitivelyDependsOn(d, res, ress) && !dependingOnSet[d] {
// 					t.Errorf("dg.DependingOn(%s) omits %s, but transitivelyDependsOn(%s, %s)",
// 						res.URN, d.URN, d.URN, res.URN)
// 				}
// 			}
// 		}
// 	})

// }

// helper code below

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
