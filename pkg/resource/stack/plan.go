package stack

import stack "github.com/pulumi/pulumi/sdk/v3/pkg/resource/stack"

func SerializePlanDiff(diff deploy.PlanDiff, enc config.Encrypter, showSecrets bool) (apitype.PlanDiffV1, error) {
	return stack.SerializePlanDiff(diff, enc, showSecrets)
}

func DeserializePlanDiff(diff apitype.PlanDiffV1, dec config.Decrypter) (deploy.PlanDiff, error) {
	return stack.DeserializePlanDiff(diff, dec)
}

func SerializeResourcePlan(plan *deploy.ResourcePlan, enc config.Encrypter, showSecrets bool) (apitype.ResourcePlanV1, error) {
	return stack.SerializeResourcePlan(plan, enc, showSecrets)
}

func SerializePlan(plan *deploy.Plan, enc config.Encrypter, showSecrets bool) (apitype.DeploymentPlanV1, error) {
	return stack.SerializePlan(plan, enc, showSecrets)
}

func DeserializeResourcePlan(plan apitype.ResourcePlanV1, dec config.Decrypter) (*deploy.ResourcePlan, error) {
	return stack.DeserializeResourcePlan(plan, dec)
}

func DeserializePlan(plan apitype.DeploymentPlanV1, dec config.Decrypter) (*deploy.Plan, error) {
	return stack.DeserializePlan(plan, dec)
}

