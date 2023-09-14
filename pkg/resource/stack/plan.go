package stack

import (
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

func SerializePlanDiff(
	diff deploy.PlanDiff,
	enc config.Encrypter,
	showSecrets bool,
) (apitype.PlanDiffV1, error) {
	adds, err := SerializeProperties(diff.Adds, enc, showSecrets)
	if err != nil {
		return apitype.PlanDiffV1{}, err
	}

	updates, err := SerializeProperties(diff.Updates, enc, showSecrets)
	if err != nil {
		return apitype.PlanDiffV1{}, err
	}

	deletes := make([]string, len(diff.Deletes))
	for i := range deletes {
		deletes[i] = string(diff.Deletes[i])
	}

	return apitype.PlanDiffV1{
		Adds:    adds,
		Updates: updates,
		Deletes: deletes,
	}, nil
}

func DeserializePlanDiff(
	diff apitype.PlanDiffV1,
	dec config.Decrypter,
	enc config.Encrypter,
) (deploy.PlanDiff, error) {
	adds, err := DeserializeProperties(diff.Adds, dec, enc)
	if err != nil {
		return deploy.PlanDiff{}, err
	}

	updates, err := DeserializeProperties(diff.Updates, dec, enc)
	if err != nil {
		return deploy.PlanDiff{}, err
	}

	deletes := make([]resource.PropertyKey, len(diff.Deletes))
	for i := range deletes {
		deletes[i] = resource.PropertyKey(diff.Deletes[i])
	}

	return deploy.PlanDiff{Adds: adds, Updates: updates, Deletes: deletes}, nil
}

func SerializeResourcePlan(
	plan *deploy.ResourcePlan,
	enc config.Encrypter,
	showSecrets bool,
) (apitype.ResourcePlanV1, error) {
	var outputs map[string]interface{}
	if plan.Outputs != nil {
		outs, err := SerializeProperties(plan.Outputs, enc, showSecrets)
		if err != nil {
			return apitype.ResourcePlanV1{}, err
		}
		outputs = outs
	}

	steps := make([]apitype.OpType, len(plan.Ops))
	for i, op := range plan.Ops {
		steps[i] = apitype.OpType(op)
	}

	var goal *apitype.GoalV1
	if plan.Goal != nil {
		inputDiff, err := SerializePlanDiff(plan.Goal.InputDiff, enc, showSecrets)
		if err != nil {
			return apitype.ResourcePlanV1{}, err
		}

		outputDiff, err := SerializePlanDiff(plan.Goal.OutputDiff, enc, showSecrets)
		if err != nil {
			return apitype.ResourcePlanV1{}, err
		}

		goal = &apitype.GoalV1{
			Type:                    plan.Goal.Type,
			Name:                    plan.Goal.Name,
			Custom:                  plan.Goal.Custom,
			InputDiff:               inputDiff,
			OutputDiff:              outputDiff,
			Parent:                  plan.Goal.Parent,
			Protect:                 plan.Goal.Protect,
			Dependencies:            plan.Goal.Dependencies,
			Provider:                plan.Goal.Provider,
			PropertyDependencies:    plan.Goal.PropertyDependencies,
			DeleteBeforeReplace:     plan.Goal.DeleteBeforeReplace,
			IgnoreChanges:           plan.Goal.IgnoreChanges,
			AdditionalSecretOutputs: plan.Goal.AdditionalSecretOutputs,
			Aliases:                 plan.Goal.Aliases,
			ID:                      plan.Goal.ID,
			CustomTimeouts:          plan.Goal.CustomTimeouts,
		}
	}

	return apitype.ResourcePlanV1{
		Goal:    goal,
		Seed:    plan.Seed,
		Steps:   steps,
		Outputs: outputs,
	}, nil
}

func SerializePlan(plan *deploy.Plan, enc config.Encrypter, showSecrets bool) (apitype.DeploymentPlanV1, error) {
	resourcePlans := map[resource.URN]apitype.ResourcePlanV1{}
	for urn, plan := range plan.ResourcePlans {
		serializedPlan, err := SerializeResourcePlan(plan, enc, showSecrets)
		if err != nil {
			return apitype.DeploymentPlanV1{}, err
		}
		resourcePlans[urn] = serializedPlan
	}

	return apitype.DeploymentPlanV1{
		Manifest:      plan.Manifest.Serialize(),
		ResourcePlans: resourcePlans,
		Config:        plan.Config,
	}, nil
}

func DeserializeResourcePlan(
	plan apitype.ResourcePlanV1,
	dec config.Decrypter,
	enc config.Encrypter,
) (*deploy.ResourcePlan, error) {
	var goal *deploy.GoalPlan
	if plan.Goal != nil {
		inputDiff, err := DeserializePlanDiff(plan.Goal.InputDiff, dec, enc)
		if err != nil {
			return nil, err
		}

		outputDiff, err := DeserializePlanDiff(plan.Goal.OutputDiff, dec, enc)
		if err != nil {
			return nil, err
		}

		goal = &deploy.GoalPlan{
			Type:                    plan.Goal.Type,
			Name:                    plan.Goal.Name,
			Custom:                  plan.Goal.Custom,
			InputDiff:               inputDiff,
			OutputDiff:              outputDiff,
			Parent:                  plan.Goal.Parent,
			Protect:                 plan.Goal.Protect,
			Dependencies:            plan.Goal.Dependencies,
			Provider:                plan.Goal.Provider,
			PropertyDependencies:    plan.Goal.PropertyDependencies,
			DeleteBeforeReplace:     plan.Goal.DeleteBeforeReplace,
			IgnoreChanges:           plan.Goal.IgnoreChanges,
			AdditionalSecretOutputs: plan.Goal.AdditionalSecretOutputs,
			Aliases:                 plan.Goal.Aliases,
			ID:                      plan.Goal.ID,
			CustomTimeouts:          plan.Goal.CustomTimeouts,
		}
	}

	var outputs resource.PropertyMap
	if plan.Outputs != nil {
		outs, err := DeserializeProperties(plan.Outputs, dec, enc)
		if err != nil {
			return nil, err
		}
		outputs = outs
	}

	ops := make([]display.StepOp, len(plan.Steps))
	for i, op := range plan.Steps {
		ops[i] = display.StepOp(op)
	}

	return &deploy.ResourcePlan{
		Goal:    goal,
		Seed:    plan.Seed,
		Ops:     ops,
		Outputs: outputs,
	}, nil
}

func DeserializePlan(plan apitype.DeploymentPlanV1, dec config.Decrypter, enc config.Encrypter) (*deploy.Plan, error) {
	manifest, err := deploy.DeserializeManifest(plan.Manifest)
	if err != nil {
		return nil, err
	}

	deserializedPlan := &deploy.Plan{
		Config:        plan.Config,
		Manifest:      *manifest,
		ResourcePlans: make(map[resource.URN]*deploy.ResourcePlan),
	}
	for urn, resourcePlan := range plan.ResourcePlans {
		deserializedResourcePlan, err := DeserializeResourcePlan(resourcePlan, dec, enc)
		if err != nil {
			return nil, err
		}
		deserializedPlan.ResourcePlans[urn] = deserializedResourcePlan
	}
	return deserializedPlan, nil
}
