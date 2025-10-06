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

package resource

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Goal is a desired state for a resource object. Normally it represents a subset of the resource's state expressed by
// a program, however if Output is true, it represents a more complete, post-deployment view of the state.
type Goal struct {
	Type                    tokens.Type           // the type of resource.
	Name                    string                // the name for the resource's URN.
	Custom                  bool                  // true if this resource is custom, managed by a plugin.
	Properties              PropertyMap           // the resource's property state.
	Parent                  URN                   // an optional parent URN for this resource.
	Protect                 *bool                 // true to protect this resource from deletion.
	Dependencies            []URN                 // dependencies of this resource object.
	Provider                string                // the provider to use for this resource.
	InitErrors              []string              // errors encountered as we attempted to initialize the resource.
	PropertyDependencies    map[PropertyKey][]URN // the set of dependencies that affect each property.
	DeleteBeforeReplace     *bool                 // true if this resource should be deleted prior to replacement.
	IgnoreChanges           []string              // a list of property paths to ignore when diffing.
	HideDiff                []PropertyPath        // a list of property paths to hide the diffs of.
	AdditionalSecretOutputs []PropertyKey         // outputs that should always be treated as secrets.
	Aliases                 []Alias               // additional structured Aliases that should be assigned.
	ID                      ID                    // the expected ID of the resource, if any.
	CustomTimeouts          CustomTimeouts        // an optional config object for resource options
	ReplaceOnChanges        []string              // a list of property paths that if changed should force a replacement.
	// if set to True, the providers Delete method will not be called for this resource.
	RetainOnDelete *bool
	// if set, the providers Delete method will not be called for this resource
	// if specified resource is being deleted as well.
	DeletedWith    URN
	SourcePosition string                // If set, the source location of the resource registration
	StackTrace     []StackFrame          // If set, the stack trace at time of registration
	ResourceHooks  map[HookType][]string // The resource hooks attached to the resource, by type.
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
	Properties PropertyMap // required

	// an optional parent URN for this resource.
	Parent URN // required

	// true to protect this resource from deletion.
	Protect *bool // required

	// dependencies of this resource object.
	Dependencies []URN // required

	// the provider to use for this resource.
	Provider string // required

	// errors encountered as we attempted to initialize the resource.
	InitErrors []string // required

	// the set of dependencies that affect each property.
	PropertyDependencies map[PropertyKey][]URN // required

	// true if this resource should be deleted prior to replacement.
	DeleteBeforeReplace *bool // required

	// a list of property paths to ignore when diffing.
	IgnoreChanges []string // required

	// outputs that should always be treated as secrets.
	AdditionalSecretOutputs []PropertyKey // required

	// additional structured Aliases that should be assigned.
	Aliases []Alias // required

	// the expected ID of the resource, if any.
	ID ID // required

	// an optional config object for resource options
	CustomTimeouts *CustomTimeouts // required

	// a list of property paths that if changed should force a replacement.
	ReplaceOnChanges []string // required

	// if set to True, the providers Delete method will not be called for this resource.
	// required
	RetainOnDelete *bool // required

	// if set, the providers Delete method will not be called for this resource
	// if specified resource is being deleted as well.
	DeletedWith URN // required

	// If set, the source location of the resource registration
	SourcePosition string // required

	// If set, the stack trace at time of registration
	StackTrace []StackFrame // required

	// The resource hooks attached to the resource, by type.
	ResourceHooks map[HookType][]string // required

	// If set, the list of property paths to hide the diff output of.
	HideDiff []PropertyPath // required
}

// Make consumes the NewGoal to create a *Goal.
func (g NewGoal) Make() *Goal {
	var customTimeouts CustomTimeouts
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
		RetainOnDelete:          g.RetainOnDelete,
		DeletedWith:             g.DeletedWith,
		SourcePosition:          g.SourcePosition,
		StackTrace:              g.StackTrace,
		ResourceHooks:           g.ResourceHooks,
	}
}
