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

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
)

// ExclusionRule represents a rule that determines if a snapshot should be excluded from fuzzing.
// If the rule returns true, the snapshot will be rejected and a new one will be generated.
type ExclusionRule func(*SnapshotSpec, *ProgramSpec, *ProviderSpec, *PlanSpec) bool

// ExclusionRules is a collection of exclusion rules that can be applied to snapshots.
type ExclusionRules []ExclusionRule

// DefaultExclusionRules returns the default set of exclusion rules that prevent known
// problematic patterns from being generated.
func DefaultExclusionRules() ExclusionRules {
	return []ExclusionRule{
		// TODO[pulumi/pulumi#21277]
		ExcludeProtectedResourceWithDuplicateProviderDestroyV2,
		// TODO[pulumi/pulumi#21282]
		ExcludeTargetedAliasDestroyV2,
	}
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

// ExcludeTargetedAlias excludes programs where a resource is renamed with an old
// alias, and the new name of the resource is targeted for deletion.
func ExcludeTargetedAliasDestroyV2(
	_ *SnapshotSpec,
	program *ProgramSpec,
	_ *ProviderSpec,
	plan *PlanSpec,
) bool {
	if plan.Operation != PlanOperationDestroyV2 {
		return false
	}

	hasTargetedResources := len(plan.TargetURNs) > 0
	for _, res := range program.ResourceRegistrations {
		if hasTargetedResources && len(res.Aliases) > 0 {
			// If there are targeted resources, and a resource registrations with
			// aliases happens, we need to exclude this snapshot, as there are
			// different issues with the handling of this.
			return true
		}
	}

	return false
}

// ExcludeProtectedResourceWithDuplicateProvider excludes snapshots where a protected component
// resource references a provider that will be deleted during the destroy.
func ExcludeProtectedResourceWithDuplicateProviderDestroyV2(
	snap *SnapshotSpec,
	_ *ProgramSpec,
	_ *ProviderSpec,
	plan *PlanSpec,
) bool {
	if plan.Operation != PlanOperationDestroyV2 {
		return false
	}
	providersByURN := make(map[string][]*ResourceSpec)
	for _, res := range snap.Resources {
		if providers.IsProviderType(res.Type) {
			urn := string(res.URN())
			providersByURN[urn] = append(providersByURN[urn], res)
		}
	}

	for _, res := range snap.Resources {
		if !res.Protect {
			continue
		}

		if res.Provider == "" {
			continue
		}

		providerRef, err := providers.ParseReference(res.Provider)
		if err != nil {
			continue
		}

		providerURN := string(providerRef.URN())

		_, ok := providersByURN[providerURN]
		if ok {
			return true
		}
	}

	return false
}
