// Copyright 2022-2024, Pulumi Corporation.
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

package stack

import (
	"context"

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
	ctx := context.TODO()
	adds, err := SerializeProperties(ctx, diff.Adds, enc, showSecrets)
	if err != nil {
		return apitype.PlanDiffV1{}, err
	}

	updates, err := SerializeProperties(ctx, diff.Updates, enc, showSecrets)
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
) (deploy.PlanDiff, error) {
	adds, err := DeserializeProperties(diff.Adds, dec)
	if err != nil {
		return deploy.PlanDiff{}, err
	}

	updates, err := DeserializeProperties(diff.Updates, dec)
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
		ctx := context.TODO()
		outs, err := SerializeProperties(ctx, plan.Outputs, enc, showSecrets)
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
) (*deploy.ResourcePlan, error) {
	var goal *deploy.GoalPlan
	if plan.Goal != nil {
		inputDiff, err := DeserializePlanDiff(plan.Goal.InputDiff, dec)
		if err != nil {
			return nil, err
		}

		outputDiff, err := DeserializePlanDiff(plan.Goal.OutputDiff, dec)
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
		outs, err := DeserializeProperties(plan.Outputs, dec)
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

func DeserializePlan(plan apitype.DeploymentPlanV1, dec config.Decrypter) (*deploy.Plan, error) {
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
		deserializedResourcePlan, err := DeserializeResourcePlan(resourcePlan, dec)
		if err != nil {
			return nil, err
		}
		deserializedPlan.ResourcePlans[urn] = deserializedResourcePlan
	}
	return deserializedPlan, nil
}
