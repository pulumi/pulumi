package apitype

import (
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
)

// GoalV1 is the serializable version of a resource goal state.
type GoalV1 struct {
	// the type of resource.
	Type tokens.Type `json:"type"`
	// the name for the resource's URN.
	Name tokens.QName `json:"name"`
	// true if this resource is custom, managed by a plugin.
	Custom bool `json:"custom"`
	// the resource's input properties.
	Properties map[string]interface{} `json:"properties,omitempty"`
	// an optional parent URN for this resource.
	Parent resource.URN `json:"parent,omitempty"`
	// true to protect this resource from deletion.
	Protect bool `json:"protect"`
	// dependencies of this resource object.
	Dependencies []resource.URN `json:"dependencies,omitempty"`
	// the provider to use for this resource.
	Provider string `json:"provider,omitempty"`
	// the set of dependencies that affect each property.
	PropertyDependencies map[resource.PropertyKey][]resource.URN `json:"propertyDependencies,omitempty"`
	// true if this resource should be deleted prior to replacement.
	DeleteBeforeReplace *bool `json:"deleteBeforeReplace,omitempty"`
	// a list of property names to ignore during changes.
	IgnoreChanges []string `json:"ignoreChanges,omitempty"`
	// outputs that should always be treated as secrets.
	AdditionalSecretOutputs []resource.PropertyKey `json:"additionalSecretOutputs,omitempty"`
	// additional URNs that should be aliased to this resource.
	Aliases []resource.URN `json:"aliases,omitempty"`
	// the expected ID of the resource, if any.
	ID resource.ID `json:"id,omitempty"`
	// an optional config object for resource options
	CustomTimeouts resource.CustomTimeouts `json:"customTimeouts,omitempty"`
}

// ResourcePlanV1 is the serializable version of a resource plan.
type ResourcePlanV1 struct {
	// The goal state for the resource.
	Goal GoalV1 `json:"goal"`
	// The steps to be performed on the resource.
	Steps []OpType `json:"steps,omitempty"`
}

// DeploymentPlanV1 is the serializable version of a deployment plan.
type DeploymentPlanV1 struct {
	// TODO(pdg-plan): should there be a message here?

	// The set of resource plans.
	ResourcePlans map[resource.URN]ResourcePlanV1 `json:"resourcePlans,omitempty"`
}
