package deploy

import deploy "github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy"

// A Plan is a mapping from URNs to ResourcePlans. The plan defines an expected set of resources and the expected
// inputs and operations for each. The inputs and operations are treated as constraints, and may allow for inputs or
// operations that do not exactly match those recorded in the plan. In the case of inputs, unknown values in the plan
// accept any value (including no value) as valid. For operations, a same step is allowed in place of an update or
// a replace step, and an update is allowed in place of a replace step. All resource options are required to match
// exactly.
type Plan = deploy.Plan

// PlanDiff holds the results of diffing two object property maps.
type PlanDiff = deploy.PlanDiff

// Goal is a desired state for a resource object.  Normally it represents a subset of the resource's state expressed by
// a program, however if Output is true, it represents a more complete, post-deployment view of the state.
type GoalPlan = deploy.GoalPlan

// A ResourcePlan represents the planned goal state and resource operations for a single resource. The operations are
// ordered.
type ResourcePlan = deploy.ResourcePlan

func NewPlan(config config.Map) Plan {
	return deploy.NewPlan(config)
}

func NewPlanDiff(inputDiff *resource.ObjectDiff) PlanDiff {
	return deploy.NewPlanDiff(inputDiff)
}

func NewGoalPlan(inputDiff *resource.ObjectDiff, goal *resource.Goal) *GoalPlan {
	return deploy.NewGoalPlan(inputDiff, goal)
}

