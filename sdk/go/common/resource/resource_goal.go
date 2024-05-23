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

// Goal is a desired state for a resource object.  Normally it represents a subset of the resource's state expressed by
// a program, however if Output is true, it represents a more complete, post-deployment view of the state.
type Goal struct {
	// the type of resource.
	Type tokens.Type
	// the name for the resource's URN.
	Name string
	// true if this resource is custom, managed by a plugin.
	Custom bool
	// the resource's property state.
	Properties PropertyMap
	// an optional parent URN for this resource.
	Parent URN
	// true to protect this resource from deletion.
	Protect bool
	// dependencies of this resource object.
	Dependencies []URN
	// the provider to use for this resource.
	Provider string
	// errors encountered as we attempted to initialize the resource.
	InitErrors []string
	// the set of dependencies that affect each property.
	PropertyDependencies map[PropertyKey][]URN
	// true if this resource should be deleted prior to replacement.
	DeleteBeforeReplace *bool
	// a list of property paths to ignore when diffing.
	IgnoreChanges []string
	// outputs that should always be treated as secrets.
	AdditionalSecretOutputs []PropertyKey
	// additional structured Aliases that should be assigned.
	Aliases []Alias
	// the expected ID of the resource, if any.
	ID ID
	// an optional config object for resource options
	CustomTimeouts CustomTimeouts
	// a list of property paths that if changed should force a replacement.
	ReplaceOnChanges []string
	// if set to True, the providers Delete method will not be called for this resource.
	RetainOnDelete bool
	// if set, the provider's Delete method will not be called for this resource
	// if specified resource is being deleted as well.
	DeletedWith URN
	// if set, this resource should be created if and only if the specified ID
	// does not exist in the provider.
	CreateIfNotExists ID
	// If set, the source location of the resource registration
	SourcePosition string
}

// NewGoal allocates a new resource goal state.
func NewGoal(t tokens.Type, name string, custom bool, props PropertyMap,
	parent URN, protect bool, dependencies []URN, provider string, initErrors []string,
	propertyDependencies map[PropertyKey][]URN, deleteBeforeReplace *bool, ignoreChanges []string,
	additionalSecretOutputs []PropertyKey, aliases []Alias, id ID, customTimeouts *CustomTimeouts,
	replaceOnChanges []string, retainOnDelete bool, deletedWith URN, createIfNotExists ID, sourcePosition string,
) *Goal {
	g := &Goal{
		Type:                    t,
		Name:                    name,
		Custom:                  custom,
		Properties:              props,
		Parent:                  parent,
		Protect:                 protect,
		Dependencies:            dependencies,
		Provider:                provider,
		InitErrors:              initErrors,
		PropertyDependencies:    propertyDependencies,
		DeleteBeforeReplace:     deleteBeforeReplace,
		IgnoreChanges:           ignoreChanges,
		AdditionalSecretOutputs: additionalSecretOutputs,
		Aliases:                 aliases,
		ID:                      id,
		ReplaceOnChanges:        replaceOnChanges,
		RetainOnDelete:          retainOnDelete,
		DeletedWith:             deletedWith,
		CreateIfNotExists:       createIfNotExists,
		SourcePosition:          sourcePosition,
	}

	if customTimeouts != nil {
		g.CustomTimeouts = *customTimeouts
	}

	return g
}
