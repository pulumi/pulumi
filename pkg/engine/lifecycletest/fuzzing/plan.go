package fuzzing

import fuzzing "github.com/pulumi/pulumi/sdk/v3/pkg/engine/lifecycletest/fuzzing"

// A PlanSpec specifies a lifecycle test operation that executes some program against an initial snapshot using a
// configured set of providers.
type PlanSpec = fuzzing.PlanSpec

// The type of operations that may be executed as part of a PlanSpec.
type OperationSpec = fuzzing.OperationSpec

// A set of options for configuring the generation of a PlanSpec.
type PlanSpecOptions = fuzzing.PlanSpecOptions

const PlanOperationUpdate = fuzzing.PlanOperationUpdate

const PlanOperationRefresh = fuzzing.PlanOperationRefresh

const PlanOperationRefreshV2 = fuzzing.PlanOperationRefreshV2

const PlanOperationDestroy = fuzzing.PlanOperationDestroy

const PlanOperationDestroyV2 = fuzzing.PlanOperationDestroyV2

// Given a SnapshotSpec and a set of options, returns a rapid.Generator that will produce PlanSpecs that can be executed
// against the specified snapshot.
func GeneratedPlanSpec(ss *SnapshotSpec, pso PlanSpecOptions) *interface{} {
	return fuzzing.GeneratedPlanSpec(ss, pso)
}

