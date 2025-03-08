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

package fuzzing

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"pgregory.net/rapid"
)

// A PlanSpec specifies a lifecycle test operation that executes some program against an initial snapshot using a
// configured set of providers.
type PlanSpec struct {
	// The operation that will be executed (e.g. update, refresh, destroy).
	Operation OperationSpec
	// The set of target URNs that will be passed to the operation, if any.
	TargetURNs []resource.URN
}

// The type of operations that may be executed as part of a PlanSpec.
type OperationSpec string

const (
	// An update operation.
	PlanOperationUpdate OperationSpec = "plan.update"
	// A refresh operation.
	PlanOperationRefresh OperationSpec = "plan.refresh"
	// A destroy operation.
	PlanOperationDestroy OperationSpec = "plan.destroy"
	// A destroy operation with program execution.
	PlanOperationDestroyV2 OperationSpec = "plan.destroyV2"
)

// Returns a set of test options and a test operation that can be used to execute this PlanSpec as part of a lifecycle
// test.
func (ps *PlanSpec) Executors(t lt.TB, hostF deploytest.PluginHostFactory) (lt.TestUpdateOptions, lt.TestOp) {
	opts := lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
	}

	if len(ps.TargetURNs) > 0 {
		opts.UpdateOptions = engine.UpdateOptions{
			Targets: deploy.NewUrnTargetsFromUrns(ps.TargetURNs),
		}
	}

	var op lt.TestOp
	switch ps.Operation {
	case PlanOperationUpdate:
		op = lt.TestOp(engine.Update)
	case PlanOperationRefresh:
		op = lt.TestOp(engine.Refresh)
	case PlanOperationDestroy:
		op = lt.TestOp(engine.Destroy)
	case PlanOperationDestroyV2:
		op = lt.TestOp(engine.DestroyV2)
	}

	return opts, op
}

// Implements PrettySpec.Pretty. Returns a human-readable string representation of this PlanSpec, suitable for use in
// debugging output and error messages.
func (ps *PlanSpec) Pretty(indent string) string {
	rendered := fmt.Sprintf("%sPlan %p", indent, ps)
	rendered += fmt.Sprintf("\n%s  Operation: %s", indent, ps.Operation)
	if len(ps.TargetURNs) > 0 {
		rendered += fmt.Sprintf("\n%s  Targets:", indent)
		for _, urn := range ps.TargetURNs {
			rendered += fmt.Sprintf("\n%s    %s", indent, Colored(urn))
		}
	} else {
		rendered += fmt.Sprintf("\n%s  No targets", indent)
	}

	return rendered
}

// A set of options for configuring the generation of a PlanSpec.
type PlanSpecOptions struct {
	// A generator for operations that might be planned.
	Operation *rapid.Generator[OperationSpec]

	// A source set of targets that should be used literally, skipping the target generation process.
	SourceTargets []resource.URN

	// A generator for the maximum number of resources to target in a plan.
	TargetCount *rapid.Generator[int]
}

// Returns a copy of the given PlanSpecOptions with the given overrides applied.
func (pso PlanSpecOptions) With(overrides PlanSpecOptions) PlanSpecOptions {
	if overrides.Operation != nil {
		pso.Operation = overrides.Operation
	}
	if overrides.SourceTargets != nil {
		pso.SourceTargets = overrides.SourceTargets
	}
	if overrides.TargetCount != nil {
		pso.TargetCount = overrides.TargetCount
	}

	return pso
}

// A default set of PlanSpecOptions. By default, a PlanSpec will have a random operation and between 0 and 5 target
// URNs.
var defaultPlanSpecOptions = PlanSpecOptions{
	Operation:     rapid.SampledFrom(operationSpecs),
	SourceTargets: nil,
	TargetCount:   rapid.IntRange(0, 5),
}

var operationSpecs = []OperationSpec{
	PlanOperationUpdate,
	PlanOperationRefresh,
	PlanOperationDestroy,
	PlanOperationDestroyV2,
}

// Given a SnapshotSpec and a set of options, returns a rapid.Generator that will produce PlanSpecs that can be executed
// against the specified snapshot.
func GeneratedPlanSpec(ss *SnapshotSpec, pso PlanSpecOptions) *rapid.Generator[*PlanSpec] {
	pso = defaultPlanSpecOptions.With(pso)

	return rapid.Custom(func(t *rapid.T) *PlanSpec {
		op := pso.Operation.Draw(t, "PlanSpec.Operation")

		var targetURNs []resource.URN
		if len(pso.SourceTargets) > 0 {
			targetURNs = pso.SourceTargets
		} else {
			seen := map[resource.URN]bool{}

			targetCount := pso.TargetCount.Draw(t, "PlanSpec.TargetCount")
			for i := 0; i < targetCount; i++ {
				candidate := rapid.SampledFrom(ss.Resources).Draw(t, fmt.Sprintf("PlanSpec.TargetResource[%d]", i))
				if seen[candidate.URN()] {
					continue
				}

				seen[candidate.URN()] = true
				targetURNs = append(targetURNs, candidate.URN())
			}
		}

		ps := &PlanSpec{
			Operation:  op,
			TargetURNs: targetURNs,
		}

		return ps
	})
}
