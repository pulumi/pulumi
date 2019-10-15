// Copyright 2016-2018, Pulumi Corporation.
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

// Package apitype contains the full set of "exchange types" that are serialized and sent across separately versionable
// boundaries, including service APIs, plugins, and file formats.  As a result, we must consider the versioning impacts
// for each change we make to types within this package.  In general, this means the following:
//
//     1) DO NOT take anything away
//     2) DO NOT change processing rules
//     3) DO NOT make optional things required
//     4) DO make anything new be optional
//
// In the event that this is not possible, a breaking change is implied.  The preferred approach is to never make
// breaking changes.  If that isn't possible, the next best approach is to support both the old and new formats
// side-by-side (for instance, by using a union type for the property in question).
//
// nolint: lll
package apitype

import (
	"encoding/json"
	"time"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/workspace"
)

const (
	// DeploymentSchemaVersionCurrent is the current version of the `Deployment` schema.
	// Any deployments newer than this version will be rejected.
	DeploymentSchemaVersionCurrent = 3
)

// VersionedCheckpoint is a version number plus a json document. The version number describes what
// version of the Checkpoint structure the Checkpoint member's json document can decode into.
type VersionedCheckpoint struct {
	Version    int             `json:"version"`
	Checkpoint json.RawMessage `json:"checkpoint"`
}

// CheckpointV1 is a serialized deployment target plus a record of the latest deployment.
type CheckpointV1 struct {
	// Stack is the stack to update.
	Stack tokens.QName `json:"stack" yaml:"stack"`
	// Config contains a bag of optional configuration keys/values.
	Config config.Map `json:"config,omitempty" yaml:"config,omitempty"`
	// Latest is the latest/current deployment (if an update has occurred).
	Latest *DeploymentV1 `json:"latest,omitempty" yaml:"latest,omitempty"`
}

// CheckpointV2 is the second version of the Checkpoint. It contains a newer version of
// the latest deployment.
type CheckpointV2 struct {
	// Stack is the stack to update.
	Stack tokens.QName `json:"stack" yaml:"stack"`
	// Config contains a bag of optional configuration keys/values.
	Config config.Map `json:"config,omitempty" yaml:"config,omitempty"`
	// Latest is the latest/current deployment (if an update has occurred).
	Latest *DeploymentV2 `json:"latest,omitempty" yaml:"latest,omitempty"`
}

// CheckpointV3 is the third version of the Checkpoint. It contains a newer version of
// the latest deployment.
type CheckpointV3 struct {
	// Stack is the stack to update.
	Stack tokens.QName `json:"stack" yaml:"stack"`
	// Config contains a bag of optional configuration keys/values.
	Config config.Map `json:"config,omitempty" yaml:"config,omitempty"`
	// Latest is the latest/current deployment (if an update has occurred).
	Latest *DeploymentV3 `json:"latest,omitempty" yaml:"latest,omitempty"`
}

// DeploymentV1 represents a deployment that has actually occurred. It is similar to the engine's snapshot structure,
// except that it flattens and rearranges a few data structures for serializability.
type DeploymentV1 struct {
	// Manifest contains metadata about this deployment.
	Manifest ManifestV1 `json:"manifest" yaml:"manifest"`
	// Resources contains all resources that are currently part of this stack after this deployment has finished.
	Resources []ResourceV1 `json:"resources,omitempty" yaml:"resources,omitempty"`
}

// DeploymentV2 is the second version of the Deployment. It contains newer versions of the
// Resource API type.
type DeploymentV2 struct {
	// Manifest contains metadata about this deployment.
	Manifest ManifestV1 `json:"manifest" yaml:"manifest"`
	// Resources contains all resources that are currently part of this stack after this deployment has finished.
	Resources []ResourceV2 `json:"resources,omitempty" yaml:"resources,omitempty"`
	// PendingOperations are all operations that were known by the engine to be currently executing.
	PendingOperations []OperationV1 `json:"pending_operations,omitempty" yaml:"pending_operations,omitempty"`
}

// DeploymentV3 is the third version of the Deployment. It contains newer versions of the
// Resource and Operation API types and a placeholder for a stack's secrets configuration.
type DeploymentV3 struct {
	// Manifest contains metadata about this deployment.
	Manifest ManifestV1 `json:"manifest" yaml:"manifest"`
	// SecretsProviders is a placeholder for secret provider configuration.
	SecretsProviders *SecretsProvidersV1 `json:"secrets_providers,omitempty" yaml:"secrets_providers,omitempty"`
	// Resources contains all resources that are currently part of this stack after this deployment has finished.
	Resources []ResourceV3 `json:"resources,omitempty" yaml:"resources,omitempty"`
	// PendingOperations are all operations that were known by the engine to be currently executing.
	PendingOperations []OperationV2 `json:"pending_operations,omitempty" yaml:"pending_operations,omitempty"`
}

type SecretsProvidersV1 struct {
	Type  string          `json:"type"`
	State json.RawMessage `json:"state,omitempty"`
}

// OperationType is the type of an operation initiated by the engine. Its value indicates the type of operation
// that the engine initiated.
type OperationType string

const (
	// OperationTypeCreating is the state of resources that are being created.
	OperationTypeCreating OperationType = "creating"
	// OperationTypeUpdating is the state of resources that are being updated.
	OperationTypeUpdating OperationType = "updating"
	// OperationTypeDeleting is the state of resources that are being deleted.
	OperationTypeDeleting OperationType = "deleting"
	// OperationTypeReading is the state of resources that are being read.
	OperationTypeReading OperationType = "reading"
)

// OperationV1 represents an operation that the engine is performing. It consists of a Resource, which is the state
// that the engine used to initiate the operation, and a Status, which is a string representation of the operation
// that the engine initiated.
type OperationV1 struct {
	// Resource is the state that the engine used to initiate this operation.
	Resource ResourceV2 `json:"resource" yaml:"resource"`
	// Status is a string representation of the operation that the engine is performing.
	Type OperationType `json:"type" yaml:"type"`
}

// OperationV2 represents an operation that the engine is performing. It consists of a Resource, which is the state
// that the engine used to initiate the operation, and a Status, which is a string representation of the operation
// that the engine initiated.
type OperationV2 struct {
	// Resource is the state that the engine used to initiate this operation.
	Resource ResourceV3 `json:"resource" yaml:"resource"`
	// Status is a string representation of the operation that the engine is performing.
	Type OperationType `json:"type" yaml:"type"`
}

// UntypedDeployment contains an inner, untyped deployment structure.
type UntypedDeployment struct {
	// Version indicates the schema of the encoded deployment.
	Version int `json:"version,omitempty"`
	// The opaque Pulumi deployment. This is conceptually of type `Deployment`, but we use `json.Message` to
	// permit round-tripping of stack contents when an older client is talking to a newer server.  If we unmarshaled
	// the contents, and then remarshaled them, we could end up losing important information.
	Deployment json.RawMessage `json:"deployment,omitempty"`
}

// ResourceV1 describes a Cloud resource constructed by Pulumi.
type ResourceV1 struct {
	// URN uniquely identifying this resource.
	URN resource.URN `json:"urn" yaml:"urn"`
	// Custom is true when it is managed by a plugin.
	Custom bool `json:"custom" yaml:"custom"`
	// Delete is true when the resource should be deleted during the next update.
	Delete bool `json:"delete,omitempty" yaml:"delete,omitempty"`
	// ID is the provider-assigned resource, if any, for custom resources.
	ID resource.ID `json:"id,omitempty" yaml:"id,omitempty"`
	// Type is the resource's full type token.
	Type tokens.Type `json:"type" yaml:"type"`
	// Inputs are the input properties supplied to the provider.
	Inputs map[string]interface{} `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	// Defaults contains the default values supplied by the provider (DEPRECATED, see #637).
	Defaults map[string]interface{} `json:"defaults,omitempty" yaml:"defaults,omitempty"`
	// Outputs are the output properties returned by the provider after provisioning.
	Outputs map[string]interface{} `json:"outputs,omitempty" yaml:"outputs,omitempty"`
	// Parent is an optional parent URN if this resource is a child of it.
	Parent resource.URN `json:"parent,omitempty" yaml:"parent,omitempty"`
	// Protect is set to true when this resource is "protected" and may not be deleted.
	Protect bool `json:"protect,omitempty" yaml:"protect,omitempty"`
	// Dependencies contains the dependency edges to other resources that this depends on.
	Dependencies []resource.URN `json:"dependencies" yaml:"dependencies,omitempty"`
	// InitErrors is the set of errors encountered in the process of initializing resource (i.e.,
	// during create or update).
	InitErrors []string `json:"initErrors" yaml:"initErrors,omitempty"`
}

// ResourceV2 is the second version of the Resource API type. It absorbs a few breaking changes:
//   1. The deprecated `Defaults` field is removed because it is not used anywhere,
//   2. It adds an additional bool field, "External", which reflects whether or not this resource
//      exists because of a call to `ReadResource`. This is motivated by a need to store
//      resources that Pulumi does not own in the deployment.
//   3. It adds an additional string field, "Provider", that is a reference to a first-class provider
//      associated with this resource.
//
// Migrating from ResourceV1 to ResourceV2 involves:
//  1. Dropping the `Defaults` field (it should be empty anyway)
//  2. Setting the `External` field to "false", since a ResourceV1 existing for a resource
//     implies that it is owned by Pulumi. Note that since this is the default value for
//     booleans in Go, no explicit assignment needs to be made.
//  3. Setting the "Provider" field to the empty string, because V1 deployments don't have first-class providers.
type ResourceV2 struct {
	// URN uniquely identifying this resource.
	URN resource.URN `json:"urn" yaml:"urn"`
	// Custom is true when it is managed by a plugin.
	Custom bool `json:"custom" yaml:"custom"`
	// Delete is true when the resource should be deleted during the next update.
	Delete bool `json:"delete,omitempty" yaml:"delete,omitempty"`
	// ID is the provider-assigned resource, if any, for custom resources.
	ID resource.ID `json:"id,omitempty" yaml:"id,omitempty"`
	// Type is the resource's full type token.
	Type tokens.Type `json:"type" yaml:"type"`
	// Inputs are the input properties supplied to the provider.
	Inputs map[string]interface{} `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	// Outputs are the output properties returned by the provider after provisioning.
	Outputs map[string]interface{} `json:"outputs,omitempty" yaml:"outputs,omitempty"`
	// Parent is an optional parent URN if this resource is a child of it.
	Parent resource.URN `json:"parent,omitempty" yaml:"parent,omitempty"`
	// Protect is set to true when this resource is "protected" and may not be deleted.
	Protect bool `json:"protect,omitempty" yaml:"protect,omitempty"`
	// External is set to true when the lifecycle of this resource is not managed by Pulumi.
	External bool `json:"external,omitempty" yaml:"external,omitempty"`
	// Dependencies contains the dependency edges to other resources that this depends on.
	Dependencies []resource.URN `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	// InitErrors is the set of errors encountered in the process of initializing resource (i.e.,
	// during create or update).
	InitErrors []string `json:"initErrors,omitempty" yaml:"initErrors,omitempty"`
	// Provider is a reference to the provider that is associated with this resource.
	Provider string `json:"provider,omitempty" yaml:"provider,omitempty"`
}

// ResourceV3 is the third version of the Resource API type. It absorbs a few breaking changes:
//   1. It adds a map from input property names to the dependencies that affect that input property. This is used to
//      improve the precision of delete-before-create operations.
//   2. It adds a new boolean field, `PendingReplacement`, that marks resources that have been deleted as part of a
//      delete-before-create operation but have not yet been recreated.
//
// Migrating from ResourceV2 to ResourceV3 involves:
//   1. Populating the map from input property names to dependencies by assuming that every dependency listed in
//      `Dependencies` affects every input property.
type ResourceV3 struct {
	// URN uniquely identifying this resource.
	URN resource.URN `json:"urn" yaml:"urn"`
	// Custom is true when it is managed by a plugin.
	Custom bool `json:"custom" yaml:"custom"`
	// Delete is true when the resource should be deleted during the next update.
	Delete bool `json:"delete,omitempty" yaml:"delete,omitempty"`
	// ID is the provider-assigned resource, if any, for custom resources.
	ID resource.ID `json:"id,omitempty" yaml:"id,omitempty"`
	// Type is the resource's full type token.
	Type tokens.Type `json:"type" yaml:"type"`
	// Inputs are the input properties supplied to the provider.
	Inputs map[string]interface{} `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	// Outputs are the output properties returned by the provider after provisioning.
	Outputs map[string]interface{} `json:"outputs,omitempty" yaml:"outputs,omitempty"`
	// Parent is an optional parent URN if this resource is a child of it.
	Parent resource.URN `json:"parent,omitempty" yaml:"parent,omitempty"`
	// Protect is set to true when this resource is "protected" and may not be deleted.
	Protect bool `json:"protect,omitempty" yaml:"protect,omitempty"`
	// External is set to true when the lifecycle of this resource is not managed by Pulumi.
	External bool `json:"external,omitempty" yaml:"external,omitempty"`
	// Dependencies contains the dependency edges to other resources that this depends on.
	Dependencies []resource.URN `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	// InitErrors is the set of errors encountered in the process of initializing resource (i.e.,
	// during create or update).
	InitErrors []string `json:"initErrors,omitempty" yaml:"initErrors,omitempty"`
	// Provider is a reference to the provider that is associated with this resource.
	Provider string `json:"provider,omitempty" yaml:"provider,omitempty"`
	// PropertyDependencies maps from an input property name to the set of resources that property depends on.
	PropertyDependencies map[resource.PropertyKey][]resource.URN `json:"propertyDependencies,omitempty" yaml:"property_dependencies,omitempty"`
	// PendingReplacement is used to track delete-before-replace resources that have been deleted but not yet
	// recreated.
	PendingReplacement bool `json:"pendingReplacement,omitempty" yaml:"pendingReplacement,omitempty"`
	// AdditionalSecretOutputs is a list of outputs that were explicitly marked as secret when the resource was created.
	AdditionalSecretOutputs []resource.PropertyKey `json:"additionalSecretOutputs,omitempty" yaml:"additionalSecretOutputs,omitempty"`
	// Aliases is a list of previous URNs that this resource may have had in previous deployments
	Aliases []resource.URN `json:"aliases,omitempty" yaml:"aliases,omitempty"`
	// CustomTimeouts is a configuration block that can be used to control timeouts of CRUD operations
	CustomTimeouts *resource.CustomTimeouts `json:"customTimeouts,omitempty" yaml:"customTimeouts,omitempty"`
}

// ManifestV1 captures meta-information about this checkpoint file, such as versions of binaries, etc.
type ManifestV1 struct {
	// Time of the update.
	Time time.Time `json:"time" yaml:"time"`
	// Magic number, used to identify integrity of the checkpoint.
	Magic string `json:"magic" yaml:"magic"`
	// Version of the Pulumi engine used to render the checkpoint.
	Version string `json:"version" yaml:"version"`
	// Plugins contains the binary version info of plug-ins used.
	Plugins []PluginInfoV1 `json:"plugins,omitempty" yaml:"plugins,omitempty"`
}

// PluginInfoV1 captures the version and information about a plugin.
type PluginInfoV1 struct {
	Name    string               `json:"name" yaml:"name"`
	Path    string               `json:"path" yaml:"path"`
	Type    workspace.PluginKind `json:"type" yaml:"type"`
	Version string               `json:"version" yaml:"version"`
}

// SecretV1 captures the information that a particular value is secret and must be decrypted before use.
//
// NOTE: nothing produces these values yet. This type is merely a placeholder for future use.
type SecretV1 struct {
	Sig        string `json:"4dabf18193072939515e22adb298388d" yaml:"4dabf18193072939515e22adb298388d"`
	Ciphertext string `json:"ciphertext" yaml:"ciphertext"`
}

// ConfigValue describes a single (possibly secret) configuration value.
type ConfigValue struct {
	// String is either the plaintext value (for non-secrets) or the base64-encoded ciphertext (for secrets).
	String string `json:"string"`
	// Secret is true if this value is a secret and false otherwise.
	Secret bool `json:"secret"`
	// Object is true if this value is a JSON encoded object. If both `Object` and `Secret` is true,
	// then the JSON encoded object contains at least one secure value.
	Object bool `json:"object"`
}

// StackTagName is the key for the tags bag in stack. This is just a string, but we use a type alias to provide a richer
// description of how the string is used in our apitype definitions.
type StackTagName = string

const (
	// ProjectNameTag is a tag that represents the name of a project (coresponds to the `name` property of Pulumi.yaml).
	ProjectNameTag StackTagName = "pulumi:project"
	// ProjectRuntimeTag is a tag that represents the runtime of a project (the `runtime` property of Pulumi.yaml).
	ProjectRuntimeTag StackTagName = "pulumi:runtime"
	// ProjectDescriptionTag is a tag that represents the description of a project (Pulumi.yaml's `description`).
	ProjectDescriptionTag StackTagName = "pulumi:description"
	// GitHubOwnerNameTag is a tag that represents the name of the owner on GitHub that this stack
	// may be associated with (inferred by the CLI based on git remote info).
	// TODO [pulumi/pulumi-service#2306] Once the UI is updated, we would no longer need the GitHub specific keys.
	GitHubOwnerNameTag StackTagName = "gitHub:owner"
	// GitHubRepositoryNameTag is a tag that represents the name of a repository on GitHub that this stack
	// may be associated with (inferred by the CLI based on git remote info).
	GitHubRepositoryNameTag StackTagName = "gitHub:repo"
	// VCSOwnerNameTag is a tag that represents the name of the owner on the cloud VCS that this stack
	// may be associated with (inferred by the CLI based on git remote info).
	VCSOwnerNameTag StackTagName = "vcs:owner"
	// VCSRepositoryNameTag is a tag that represents the name of a repository on the cloud VCS that this stack
	// may be associated with (inferred by the CLI based on git remote info).
	VCSRepositoryNameTag StackTagName = "vcs:repo"
	// VCSRepositoryKindTag is a tag that represents the kind of the cloud VCS that this stack
	// may be associated with (inferred by the CLI based on the git remote info).
	VCSRepositoryKindTag StackTagName = "vcs:kind"
)

// Stack describes a Stack running on a Pulumi Cloud.
type Stack struct {
	OrgName     string       `json:"orgName"`
	ProjectName string       `json:"projectName"`
	StackName   tokens.QName `json:"stackName"`

	ActiveUpdate string                  `json:"activeUpdate"`
	Tags         map[StackTagName]string `json:"tags,omitempty"`

	Version int `json:"version"`
}
