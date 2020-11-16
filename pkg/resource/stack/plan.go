package stack

import (
	"github.com/pulumi/pulumi/pkg/v2/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v2/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/config"
)

func SerializeResourcePlan(plan *deploy.ResourcePlan, enc config.Encrypter, showSecrets bool) (apitype.ResourcePlanV1, error) {

	steps := make([]apitype.OpType, len(plan.Ops))
	for i, op := range plan.Ops {
		steps[i] = apitype.OpType(op)
	}

	return apitype.ResourcePlanV1{
		Steps: steps,
	}, nil
}

func SerializePlan(plan map[resource.URN]*deploy.ResourcePlan, enc config.Encrypter, showSecrets bool) (apitype.DeploymentPlanV1, error) {
	resourcePlans := map[resource.URN]apitype.ResourcePlanV1{}
	for urn, plan := range plan {
		serializedPlan, err := SerializeResourcePlan(plan, enc, showSecrets)
		if err != nil {
			return apitype.DeploymentPlanV1{}, err
		}
		resourcePlans[urn] = serializedPlan
	}
	return apitype.DeploymentPlanV1{ResourcePlans: resourcePlans}, nil
}

func DeserializeResourcePlan(plan apitype.ResourcePlanV1, dec config.Decrypter, enc config.Encrypter) (*deploy.ResourcePlan, error) {
	ops := make([]deploy.StepOp, len(plan.Steps))
	for i, op := range plan.Steps {
		ops[i] = deploy.StepOp(op)
	}

	return &deploy.ResourcePlan{
		Ops: ops,
	}, nil
}

func DeserializePlan(plan apitype.DeploymentPlanV1, dec config.Decrypter, enc config.Encrypter) (map[resource.URN]*deploy.ResourcePlan, error) {
	resourcePlans := map[resource.URN]*deploy.ResourcePlan{}
	for urn, plan := range plan.ResourcePlans {
		deserializedPlan, err := DeserializeResourcePlan(plan, dec, enc)
		if err != nil {
			return nil, err
		}
		resourcePlans[urn] = deserializedPlan
	}
	return resourcePlans, nil
}
