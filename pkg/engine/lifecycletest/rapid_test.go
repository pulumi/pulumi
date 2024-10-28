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

package lifecycletest

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/fuzzing"
	"pgregory.net/rapid"
)

// strategy
//
// generate a random set of resources
// each resource can randomly depend on those before it
//	- in a provider/parent capacity if appropriate resources exist
//
// write out those resources to a snapshot
// verify that it is valid

var successfulUpdatesOnly = fuzzing.FixtureOptions{
	ProviderSpecOptions: fuzzing.ProviderSpecOptions{
		CreateAction: rapid.SampledFrom([]fuzzing.ProviderCreateSpecAction{""}),
		DeleteAction: rapid.SampledFrom([]fuzzing.ProviderDeleteSpecAction{""}),
		DiffAction: rapid.SampledFrom([]fuzzing.ProviderDiffSpecAction{
			"",
		}),
		ReadAction:   rapid.SampledFrom([]fuzzing.ProviderReadSpecAction{""}),
		UpdateAction: rapid.SampledFrom([]fuzzing.ProviderUpdateSpecAction{""}),
	},
	PlanSpecOptions: fuzzing.PlanSpecOptions{
		Operation: rapid.SampledFrom([]fuzzing.OperationSpec{fuzzing.PlanOperationUpdate}),
	},
}

var successfulTargetedUpdatesOnly = successfulUpdatesOnly.With(fuzzing.FixtureOptions{
	SnapshotSpecOptions: fuzzing.SnapshotSpecOptions{
		ResourceCount: rapid.Just(3),
		Action:        rapid.SampledFrom([]fuzzing.SnapshotSpecAction{fuzzing.SnapshotSpecNew}),
		ResourceOpts: fuzzing.ResourceSpecOptions{
			Custom:             rapid.Just(true),
			PendingReplacement: rapid.Just(false),
			Protect:            rapid.Just(false),
			RetainOnDelete:     rapid.Just(false),
		},
	},
	ProgramSpecOptions: fuzzing.ProgramSpecOptions{
		PrependCount: rapid.Just(0),
		Action:       rapid.SampledFrom([]fuzzing.ProgramSpecAction{fuzzing.ProgramSpecDelete, fuzzing.ProgramSpecCopy, fuzzing.ProgramSpecCopy}),
		AppendCount:  rapid.Just(0),
	},
	PlanSpecOptions: fuzzing.PlanSpecOptions{
		TargetCount: rapid.Just(1),
	},
})

func TestFoo(t *testing.T) {
	//t.Skip()
	t.Parallel()

	rapid.Check(t, fuzzing.GeneratedFixture(successfulUpdatesOnly))
}
