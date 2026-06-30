// Copyright 2016, Pulumi Corporation.
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

package resource

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Goal is a desired state for a resource object. Normally it represents a subset of the resource's state expressed by
// a program, however if Output is true, it represents a more complete, post-deployment view of the state.
type Goal struct {
	// the type of resource.
	Type tokens.Type
	// the name for the resource's URN.
	Name string
	// true if this resource is custom, managed by a plugin.
	Custom bool
	// the resource's property state.
	Properties resource.PropertyMap
	// an optional parent URN for this resource.
	Parent resource.URN
	// true to protect this resource from deletion.
	Protect *bool
	// dependencies of this resource object.
	Dependencies []resource.URN
	// the provider to use for this resource.
	Provider string
	// errors encountered as we attempted to initialize the resource.
	InitErrors []string
	// the set of dependencies that affect each property.
	PropertyDependencies map[resource.PropertyKey][]resource.URN
	// true if this resource should be deleted prior to replacement.
	DeleteBeforeReplace *bool
	// a list of property paths to ignore when diffing.
	IgnoreChanges []string
	// a list of property paths to hide the diffs of.
	HideDiff []resource.PropertyPath
	// outputs that should always be treated as secrets.
	AdditionalSecretOutputs []resource.PropertyKey
	// additional structured Aliases that should be assigned.
	Aliases []resource.Alias
	// the expected ID of the resource, if any.
	ID resource.ID
	// an optional config object for resource options
	CustomTimeouts resource.CustomTimeouts
	// a list of property paths that if changed should force a replacement.
	ReplaceOnChanges []string
	// if set, the engine will diff this with the last recorded value, and trigger a replace if they are not equal.
	ReplacementTrigger resource.PropertyValue
	// if set to True, the providers Delete method will not be called for this resource.
	RetainOnDelete *bool
	// if set, the providers Delete method will not be called for this resource.
	// if specified resource is being deleted as well.
	DeletedWith resource.URN
	// If set, the URNs of the resources whose replaces will also trigger a replace of the current resource.
	ReplaceWith []resource.URN
	// If set, the source location of the resource registration.
	SourcePosition string
	// If set, the stack trace at time of registration.
	StackTrace []resource.StackFrame
	// The resource hooks attached to the resource, by type.
	ResourceHooks map[resource.HookType][]string
}

// NewGoal is used to construct Goal values. The dataflow for Goal is rather sensitive, so all fields are required.
// Call [NewGoal.Make] to create the *Goal value.
type NewGoal struct {
	// the type of resource.
	Type tokens.Type // required

	// the name for the resource's URN.
	Name string // required

	// true if this resource is custom, managed by a plugin.
	Custom bool // required

	// the resource's property state.
	Properties resource.PropertyMap // required

	// an optional parent URN for this resource.
	Parent resource.URN // required

	// true to protect this resource from deletion.
	Protect *bool // required

	// dependencies of this resource object.
	Dependencies []resource.URN // required

	// the provider to use for this resource.
	Provider string // required

	// errors encountered as we attempted to initialize the resource.
	InitErrors []string // required

	// the set of dependencies that affect each property.
	PropertyDependencies map[resource.PropertyKey][]resource.URN // required

	// true if this resource should be deleted prior to replacement.
	DeleteBeforeReplace *bool // required

	// a list of property paths to ignore when diffing.
	IgnoreChanges []string // required

	// outputs that should always be treated as secrets.
	AdditionalSecretOutputs []resource.PropertyKey // required

	// additional structured Aliases that should be assigned.
	Aliases []resource.Alias // required

	// the expected ID of the resource, if any.
	ID resource.ID // required

	// an optional config object for resource options
	CustomTimeouts *resource.CustomTimeouts // required

	// a list of property paths that if changed should force a replacement.
	ReplaceOnChanges []string // required

	// if set, the engine will diff this with the last recorded value, and trigger a replace if they are not equal.
	ReplacementTrigger resource.PropertyValue // required

	// if set to True, the providers Delete method will not be called for this resource.
	// required
	RetainOnDelete *bool // required

	// if set, the providers Delete method will not be called for this resource
	// if specified resource is being deleted as well.
	DeletedWith resource.URN // required

	// If set, the URNs of the resources whose replaces will also trigger a replace of the current resource.
	ReplaceWith []resource.URN // required

	// If set, the source location of the resource registration
	SourcePosition string // required

	// If set, the stack trace at time of registration
	StackTrace []resource.StackFrame // required

	// The resource hooks attached to the resource, by type.
	ResourceHooks map[resource.HookType][]string // required

	// If set, the list of property paths to hide the diff output of.
	HideDiff []resource.PropertyPath // required
}

// Make consumes the NewGoal to create a *Goal.
func (g NewGoal) Make() *Goal {
	var customTimeouts resource.CustomTimeouts
	if g.CustomTimeouts != nil {
		customTimeouts = *g.CustomTimeouts
	}
	return &Goal{
		Type:                    g.Type,
		Name:                    g.Name,
		Custom:                  g.Custom,
		Properties:              g.Properties,
		Parent:                  g.Parent,
		Protect:                 g.Protect,
		Dependencies:            g.Dependencies,
		Provider:                g.Provider,
		InitErrors:              g.InitErrors,
		PropertyDependencies:    g.PropertyDependencies,
		DeleteBeforeReplace:     g.DeleteBeforeReplace,
		IgnoreChanges:           g.IgnoreChanges,
		HideDiff:                g.HideDiff,
		AdditionalSecretOutputs: g.AdditionalSecretOutputs,
		Aliases:                 g.Aliases,
		ID:                      g.ID,
		CustomTimeouts:          customTimeouts,
		ReplaceOnChanges:        g.ReplaceOnChanges,
		ReplacementTrigger:      g.ReplacementTrigger,
		RetainOnDelete:          g.RetainOnDelete,
		DeletedWith:             g.DeletedWith,
		ReplaceWith:             g.ReplaceWith,
		SourcePosition:          g.SourcePosition,
		StackTrace:              g.StackTrace,
		ResourceHooks:           g.ResourceHooks,
	}
}
