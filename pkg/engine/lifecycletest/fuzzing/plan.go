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

type PlanSpec struct {
	Operation  OperationSpec
	TargetURNs []resource.URN
}

type OperationSpec string

const (
	PlanOperationUpdate  OperationSpec = "plan.update"
	PlanOperationRefresh OperationSpec = "plan.refresh"
	PlanOperationDestroy OperationSpec = "plan.destroy"
)

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
	}

	return opts, op
}

type PlanSpecOptions struct {
	Operation   *rapid.Generator[OperationSpec]
	TargetCount *rapid.Generator[int]
}

func (pso PlanSpecOptions) With(overrides PlanSpecOptions) PlanSpecOptions {
	if overrides.Operation != nil {
		pso.Operation = overrides.Operation
	}
	if overrides.TargetCount != nil {
		pso.TargetCount = overrides.TargetCount
	}

	return pso
}

var defaultPlanSpecOptions = PlanSpecOptions{
	Operation:   rapid.SampledFrom(operationSpecs),
	TargetCount: rapid.IntRange(0, 5),
}

var operationSpecs = []OperationSpec{
	PlanOperationUpdate,
	PlanOperationRefresh,
	PlanOperationDestroy,
}

func GeneratedPlanSpec(ss *SnapshotSpec, pso PlanSpecOptions) *rapid.Generator[*PlanSpec] {
	pso = defaultPlanSpecOptions.With(pso)

	return rapid.Custom(func(t *rapid.T) *PlanSpec {
		op := pso.Operation.Draw(t, "PlanSpec.Operation")

		seen := map[resource.URN]bool{}
		targetURNs := []resource.URN{}

		targetCount := pso.TargetCount.Draw(t, "PlanSpec.TargetCount")
		for i := 0; i < targetCount; i++ {
			candidate := rapid.SampledFrom(ss.Resources).Draw(t, fmt.Sprintf("PlanSpec.TargetResource[%d]", i))
			if seen[candidate.URN()] {
				continue
			}

			seen[candidate.URN()] = true
			targetURNs = append(targetURNs, candidate.URN())
		}

		ps := &PlanSpec{
			Operation:  op,
			TargetURNs: targetURNs,
		}

		return ps
	})
}
