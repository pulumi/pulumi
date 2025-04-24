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

package state

import (
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// Given a list of states before and after a reordering operation (such as a topological sort), computes the set of
// states which have been moved. This function attempts to only return resources which have been "directly" moved, and
// not simply shifted in response to another resource changing position. For example, given:
//
// Before: 4, 1, 2, 3, 5
// After:  1, 2, 3, 4, 5
//
// We should expect to identify 4 as having been moved, but not 1, 2, or 3, even though their positions have technically
// changed. We do this by effectively transforming the list of states into a doubly-linked list and only returning
// elements whose previous *and* next elements changed. This is imperfect in the case where there are exactly two
// elements in total but otherwise works well. In the example above, we have:
//
// Before:
//
// 4 { Previous: nil, Next: 1 }
// 1 { Previous: 4,   Next: 2 }
// 2 { Previous: 1,   Next: 3 }
// 3 { Previous: 2,   Next: 5 }
// 5 { Previous: 3,   Next: nil }
//
// After:
//
// 1 { Previous: nil, Next: 2 }
// 2 { Previous: 1,   Next: 3 }
// 3 { Previous: 2,   Next: 4 }
// 4 { Previous: 3,   Next: 5 }
// 5 { Previous: 4,   Next: nil }
//
// and so we see that 4 is indeed the sole reordered element.
func computeStateRepairReorderings(
	before []*resource.State,
	after []*resource.State,
) []*resource.State {
	prevs := map[resource.URN]*resource.State{}
	nexts := map[resource.URN]*resource.State{}

	// First run through the list before reordering took place, recording for each state the ones that preceded and
	// succeeded it.
	var prev *resource.State
	for _, state := range before {
		prevs[state.URN] = prev
		if prev != nil {
			nexts[prev.URN] = state
		}

		prev = state
	}

	// Now run through the list after reordering. For each element, if *both* its preceding and succeeding states changed,
	// add it to the list of reorderings.
	var reorderings []*resource.State
	prev = nil
	for i, state := range after {
		if prev != prevs[state.URN] {
			var next *resource.State
			if i < len(after)-1 {
				next = after[i+1]
			}

			if next != nexts[state.URN] {
				reorderings = append(reorderings, state)
			}
		}

		prev = state
	}

	return reorderings
}

// Renders a set of state repair operations as a human-readable string.
func renderStateRepairOperations(
	colorization colors.Colorization,
	reorderings []*resource.State,
	pruneResults []deploy.PruneResult,
) string {
	var b strings.Builder

	if len(reorderings) > 0 {
		b.WriteString(`The following resources will be reordered to appear before their dependents:

`)

		for _, state := range reorderings {
			b.WriteString("* ")
			b.WriteString(string(state.URN))
			if state.Delete {
				b.WriteString(" (deleted)")
			}
			b.WriteString("\n")
		}

		b.WriteString("\n")
	}

	if len(pruneResults) > 0 {
		b.WriteString(`The following resources will be modified to remove missing dependencies:

`)

		for _, result := range pruneResults {
			b.WriteString(colorization.Colorize(colors.SpecUpdate + "~" + colors.Reset + " "))
			b.WriteString(string(result.OldURN))
			if result.Delete {
				b.WriteString(" (deleted)")
			}
			b.WriteString("\n")

			if result.NewURN != result.OldURN {
				b.WriteString("  " + colorization.Colorize(colors.SpecUpdate+"~"+colors.Reset+" "))
				b.WriteString(string(result.NewURN))
				b.WriteString(" [urn]\n")
			}

			for _, d := range result.RemovedDependencies {
				b.WriteString("  " + colorization.Colorize(colors.SpecDelete+"-"+colors.Reset+" "))
				b.WriteString(string(d.URN))
				b.WriteRune(' ')
				switch d.Type {
				case resource.ResourceParent:
					b.WriteString("[parent]")
				case resource.ResourceDependency:
					b.WriteString("[dependency]")
				case resource.ResourcePropertyDependency:
					b.WriteString("[property dependency: " + string(d.Key) + "]")
				case resource.ResourceDeletedWith:
					b.WriteString("[deleted with]")
				}
				b.WriteString("\n")
			}
		}
	}

	return b.String()
}
