// Copyright 2025, Pulumi Corporation.
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

package fuzzing

// ExclusionRule represents a rule that determines if a snapshot should be excluded from fuzzing.
// If the rule returns true, the snapshot will be rejected and a new one will be generated.
type ExclusionRule func(*SnapshotSpec, *ProgramSpec, *ProviderSpec, *PlanSpec) bool

// ExclusionRules is a collection of exclusion rules that can be applied to snapshots.
type ExclusionRules []ExclusionRule

// DefaultExclusionRules returns the default set of exclusion rules that prevent known
// problematic patterns from being generated.
func DefaultExclusionRules() ExclusionRules {
	return []ExclusionRule{}
}

// ShouldExclude checks if a snapshot should be excluded based on the configured exclusion rules.
// Returns true if any rule indicates the snapshot should be excluded.
func (er ExclusionRules) ShouldExclude(
	snap *SnapshotSpec,
	program *ProgramSpec,
	provider *ProviderSpec,
	plan *PlanSpec,
) bool {
	for _, rule := range er {
		if rule(snap, program, provider, plan) {
			return true
		}
	}
	return false
}
