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

func isParent(child, parent *resource.State) bool {
	return child.Parent == parent.URN
}

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

func directlyDependsOn(a, b *resource.State) bool {
	for _, dep := range a.Dependencies {
		if b.URN == dep {
			return true
		}
	}
	return false
}

func transitivelyDependsOn(a, b *resource.State, universe []*resource.State) bool {
	if a.URN == b.URN {
		return false
	}
	if directlyDependsOn(a, b) {
		return true
	}
	if isDescendant(a, b, universe) {
		return true
	}
	for _, x := range universe {
		if directlyDependsOn(a, x) {
			if transitivelyDependsOn(x, b, universe) {
				return true
			}
			if isDescendant(b, x, universe) {
				return true
			}
		}
	}
	return false
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

func TestRapidDependingOn(t *testing.T) {
	rss := resourceStateSliceGenerator()
	rapid.Check(t, func(t *rapid.T) {
		ress := rss.Draw(t, "rss").([]*resource.State)

		t.Logf("Checking resource-set: %s", showStates(ress))

		dg := NewDependencyGraph(ress)

		for _, res := range ress {
			dependingOn := dg.DependingOn(res, nil)
			dependingOnSet := ResourceSet{}

			for _, d := range dependingOn {
				dependingOnSet[d] = true
			}

			for d := range dependingOnSet {
				if !transitivelyDependsOn(d, res, ress) {
					t.Errorf("dg.DependingOn(%s) includes %s, but !transitivelyDependsOn(%s, %s)",
						res.URN, d.URN, d.URN, res.URN)
				}
			}

			for _, d := range ress {
				if transitivelyDependsOn(d, res, ress) && !dependingOnSet[d] {
					t.Errorf("dg.DependingOn(%s) omits %s, but transitivelyDependsOn(%s, %s)",
						res.URN, d.URN, d.URN, res.URN)
				}
			}
		}
	})

}

func TestRapidDependenciesOf(t *testing.T) {
	rss := resourceStateSliceGenerator()

	rapid.Check(t, func(t *rapid.T) {
		ress := rss.Draw(t, "rss").([]*resource.State)

		t.Logf("Checking resource-set: %s", showStates(ress))

		dg := NewDependencyGraph(ress)

		for _, res := range ress {
			resDeps := dg.DependenciesOf(res)

			for resDep := range resDeps {
				if !transitivelyDependsOn(res, resDep, ress) {
					t.Errorf("dg.DependenciesOf(%s) includes %s, but !transitivelyDependsOn(%s, %s)",
						res.URN, resDep.URN, res.URN, resDep.URN)
				}
			}

			for _, resDep := range ress {
				if transitivelyDependsOn(res, resDep, ress) && !resDeps[resDep] {
					t.Errorf("dg.DependenciesOf(%s) omits %s, but transitivelyDependsOn(%s, %s)",
						res.URN, resDep.URN, res.URN, resDep.URN)
				}
			}
		}
	})
}

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
