package apitype

import (
	"encoding/json"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// PlanDiffV1 is the serializable version of a plan diff.
type PlanDiffV1 struct {
	// the resource properties that will be added.
	Adds map[string]interface{} `json:"adds,omitempty"`
	// the resource properties that will be deleted.
	Deletes []string `json:"deletes,omitempty"`
	// the resource properties that will be updated.
	Updates map[string]interface{} `json:"updates,omitempty"`
}

// GoalV1 is the serializable version of a resource goal state.
type GoalV1 struct {
	// the type of resource.
	Type tokens.Type `json:"type"`
	// the name for the resource's URN.
	Name tokens.QName `json:"name"`
	// true if this resource is custom, managed by a plugin.
	Custom bool `json:"custom"`
	// the resource properties that will be changed.
	InputDiff PlanDiffV1 `json:"inputDiff,omitempty"`
	// the resource outputs that will be changed.
	OutputDiff PlanDiffV1 `json:"outputDiff,omitempty"`
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
	AliasURNs []resource.URN `json:"aliases,omitempty"`
	// structured alias objects to be assigned to this resource.
	Aliases []resource.Alias `json:"structuredAliases,omitempty"`
	// the expected ID of the resource, if any.
	ID resource.ID `json:"id,omitempty"`
	// an optional config object for resource options
	CustomTimeouts resource.CustomTimeouts `json:"customTimeouts,omitempty"`
}

// ResourcePlanV1 is the serializable version of a resource plan.
type ResourcePlanV1 struct {
	// The goal state for the resource.
	Goal *GoalV1 `json:"goal,omitempty"`
	// The steps to be performed on the resource.
	Steps []OpType `json:"steps,omitempty"`
	// The proposed outputs for the resource, if any. Purely advisory.
	Outputs map[string]interface{} `json:"state"`
	// The random byte seed used for resource goal.
	Seed []byte `json:"seed,omitempty"`
}

// VersionedDeploymentPlan is a version number plus a JSON document. The version number describes what
// version of the DeploymentPlan structure the DeploymentPlan member's JSON document can decode into.
type VersionedDeploymentPlan struct {
	Version int             `json:"version"`
	Plan    json.RawMessage `json:"plan"`
}

// DeploymentPlanV1 is the serializable version of a deployment plan.
type DeploymentPlanV1 struct {
	// TODO(pdg-plan): should there be a message here?

	// Manifest contains metadata about this plan.
	Manifest ManifestV1 `json:"manifest" yaml:"manifest"`
	// The configuration in use during the plan.
	Config config.Map `json:"config,omitempty"`

	// The set of resource plans.
	ResourcePlans map[resource.URN]ResourcePlanV1 `json:"resourcePlans,omitempty"`
}
