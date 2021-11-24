package stack

import (
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

func SerializeResourcePlan(
	plan *deploy.ResourcePlan,
	enc config.Encrypter,
	showSecrets bool) (apitype.ResourcePlanV1, error) {

	adds, err := SerializeProperties(plan.Goal.Adds, enc, showSecrets)
	if err != nil {
		return apitype.ResourcePlanV1{}, err
	}

	updates, err := SerializeProperties(plan.Goal.Adds, enc, showSecrets)
	if err != nil {
		return apitype.ResourcePlanV1{}, err
	}

	deletes := make([]string, len(plan.Goal.Deletes))
	for i := range deletes {
		deletes[i] = string(plan.Goal.Deletes[i])
	}

	var outputs map[string]interface{}
	if plan.Outputs != nil {
		outs, err := SerializeProperties(plan.Outputs, enc, showSecrets)
		if err != nil {
			return apitype.ResourcePlanV1{}, err
		}
		outputs = outs
	}

	goal := apitype.GoalV1{
		Type:                    plan.Goal.Type,
		Name:                    plan.Goal.Name,
		Custom:                  plan.Goal.Custom,
		Adds:                    adds,
		Deletes:                 deletes,
		Updates:                 updates,
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

	steps := make([]apitype.OpType, len(plan.Ops))
	for i, op := range plan.Ops {
		steps[i] = apitype.OpType(op)
	}

	return apitype.ResourcePlanV1{
		Goal:    goal,
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
	enc config.Encrypter) (*deploy.ResourcePlan, error) {

	adds, err := DeserializeProperties(plan.Goal.Adds, dec, enc)
	if err != nil {
		return nil, err
	}

	updates, err := DeserializeProperties(plan.Goal.Updates, dec, enc)
	if err != nil {
		return nil, err
	}

	var outputs resource.PropertyMap
	if plan.Outputs != nil {
		outs, err := DeserializeProperties(plan.Outputs, dec, enc)
		if err != nil {
			return nil, err
		}
		outputs = outs
	}

	goal := &deploy.GoalPlan{
		Type:                    plan.Goal.Type,
		Name:                    plan.Goal.Name,
		Custom:                  plan.Goal.Custom,
		Adds:                    adds,
		Deletes:                 nil,
		Updates:                 updates,
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

	ops := make([]deploy.StepOp, len(plan.Steps))
	for i, op := range plan.Steps {
		ops[i] = deploy.StepOp(op)
	}

	return &deploy.ResourcePlan{
		Goal:    goal,
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
		Config:   plan.Config,
		Manifest: *manifest,
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
