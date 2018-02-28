// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package apitype

import (
	"time"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Checkpoint is a serialized deployment target plus a record of the latest deployment.
type Checkpoint struct {
	// Stack is the stack to update.
	Stack tokens.QName `json:"stack" yaml:"stack"`
	// Config contains a bag of optional configuration keys/values.
	Config config.Map `json:"config,omitempty" yaml:"config,omitempty"`
	// Latest is the latest/current deployment (if an update has occurred).
	Latest *Deployment `json:"latest,omitempty" yaml:"latest,omitempty"`
}

// Deployment represents a deployment that has actually occurred.   It is similar to the engine's snapshot structure,
// except that it flattens and rearranges a few data structures for serializability.
type Deployment struct {
	// Manifest contains metadata about this deployment.
	Manifest Manifest `json:"manifest" yaml:"manifest"`
	// Resources contains all resources that are currently part of this stack after this deployment has finished.
	Resources []Resource `json:"resources,omitempty" yaml:"resources,omitempty"`
}

// Resource describes a Cloud resource constructed by Pulumi.
type Resource struct {
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
}

// Manifest captures meta-information about this checkpoint file, such as versions of binaries, etc.
type Manifest struct {
	// Time of the update.
	Time time.Time `json:"time" yaml:"time"`
	// Magic number, used to identify integrity of the checkpoint.
	Magic string `json:"magic" yaml:"magic"`
	// Version of the Pulumi engine used to render the checkpoint.
	Version string `json:"version" yaml:"version"`
	// Plugins contains the binary version info of plug-ins used.
	Plugins []PluginInfo `json:"plugins,omitempty" yaml:"plugins,omitempty"`
}

// PluginInfo captures the version and information about a plugin.
type PluginInfo struct {
	Name    string               `json:"name" yaml:"name"`
	Path    string               `json:"path" yaml:"path"`
	Type    workspace.PluginKind `json:"type" yaml:"type"`
	Version string               `json:"version" yaml:"version"`
}

// ConfigValue describes a single (possibly secret) configuration value.
type ConfigValue struct {
	// String is either the plaintext value (for non-secrets) or the base64-encoded ciphertext (for secrets).
	String string `json:"string"`
	// Secret is true if this value is a secret and false otherwise.
	Secret bool `json:"secret"`
}

// Stack describes a Stack running on a Pulumi Cloud.
type Stack struct {
	CloudName string `json:"cloudName"`
	OrgName   string `json:"orgName"`

	RepoName    string       `json:"repoName"`
	ProjectName string       `json:"projName"`
	StackName   tokens.QName `json:"stackName"`

	ActiveUpdate string     `json:"activeUpdate"`
	Resources    []Resource `json:"resources,omitempty"`

	Version int `json:"version"`
}
