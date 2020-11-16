package apitype

import (
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
)

// ResourcePlanV1 is the serializable version of a resource plan.
type ResourcePlanV1 struct {
	// The steps to be performed on the resource.
	Steps []OpType `json:"steps,omitempty"`
}

// DeploymentPlanV1 is the serializable version of a deployment plan.
type DeploymentPlanV1 struct {
	// TODO(pdg-plan): should there be a message here?

	// The set of resource plans.
	ResourcePlans map[resource.URN]ResourcePlanV1 `json:"resourcePlans,omitempty"`
}
