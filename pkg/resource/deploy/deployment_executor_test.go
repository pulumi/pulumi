// Copyright 2016-2022, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/stretchr/testify/assert"
)

// Note: the only valid way to add a resource to the node list is via the `add` method.
// This ensures that the `sorted` method works correctly.
type nodeList []*resource.State

func (nl *nodeList) Add(urn string, delete bool, children ...*resource.State) *resource.State {
	n := &resource.State{URN: resource.URN(urn), Delete: delete}
	for _, child := range children {
		child.Parent = n.URN
	}
	*nl = append(*nl, n)
	return n
}

func (nl *nodeList) AsRefreshSteps() map[*resource.State]Step {
	m := make(map[*resource.State]Step, len(*nl))
	for _, r := range *nl {
		m[r] = &RefreshStep{
			old: r,
			new: r,
		}
	}
	return m
}

func (nl *nodeList) sorted() []*resource.State {
	// Since we add elements before we add their parents, we are guaranteed a reverse
	// topological sort. We can retrieve a topological sort by reversing the list.
	l := slice.Prealloc[*resource.State](len(*nl))
	for i := len(*nl) - 1; i >= 0; i-- {
		l = append(l, (*nl)[i])
	}
	return l
}

func (nl *nodeList) Executor() *deploymentExecutor {
	return &deploymentExecutor{
		deployment: &Deployment{
			prev: &Snapshot{
				Resources: nl.sorted(),
			},
		},
	}
}

func TestRebuildBaseState(t *testing.T) {
	t.Parallel()

	t.Run("simple-deps", func(t *testing.T) {
		t.Parallel()
		nl := &nodeList{}
		nl.Add("A", true, nl.Add("B", false))

		ex := nl.Executor()

		ex.rebuildBaseState(nl.AsRefreshSteps())

		assert.EqualValues(t, map[resource.URN]*resource.State{
			"B": {URN: "B"},
		}, ex.deployment.olds)
	})

	t.Run("tree", func(t *testing.T) {
		t.Parallel()
		nl := &nodeList{}
		nl.Add("A", false,
			nl.Add("C", true,
				nl.Add("F", false)),
			nl.Add("D", false,
				nl.Add("G", false),
				nl.Add("H", true)))
		nl.Add("B", true,
			nl.Add("E", true,
				nl.Add("I", false)))

		ex := nl.Executor()

		ex.rebuildBaseState(nl.AsRefreshSteps())

		assert.EqualValues(t, map[resource.URN]*resource.State{
			"A": {URN: "A"},
			"I": {URN: "I"},
			"F": {URN: "F", Parent: "A"},
			"G": {URN: "G", Parent: "D"},
			"D": {URN: "D", Parent: "A"},
		}, ex.deployment.olds)
	})
}
