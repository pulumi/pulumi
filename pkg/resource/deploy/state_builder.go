// Copyright 2016-2022, Pulumi Corporation.
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

package deploy

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// stateBuilder offers a fluent API for making edits to a resource.State object in a way that avoids mutation and
// allocation where possible.
type stateBuilder struct {
	state    resource.State
	original *resource.State
	edited   bool
}

// newStateBuilder creates a new builder atop the given state.
func newStateBuilder(state *resource.State) *stateBuilder {
	return &stateBuilder{
		state:    *(state.Copy()),
		original: state,
		edited:   false,
	}
}

// withUpdatedURN updates the URN of the state being modified using the given function.
func (sb *stateBuilder) withUpdatedURN(update func(resource.URN) resource.URN) *stateBuilder {
	sb.setURN(&sb.state.URN, update(sb.state.URN))
	return sb
}

// withAllUpdatedDependencies updates all dependencies in the state being modified using the given functions to modify
// the provider reference and any URNs encountered respectively. A third function may be supplied in order to determine
// which dependency types should be targeted.
func (sb *stateBuilder) withAllUpdatedDependencies(
	updateProviderRef func(string) string,
	updateURN func(resource.URN) resource.URN,
	include func(dep resource.StateDependency) bool,
) *stateBuilder {
	provider, allDeps := sb.state.GetAllDependencies()
	if provider != "" {
		sb.setString(&sb.state.Provider, updateProviderRef(provider))
	}

	editedDeps := false
	newDeps := []resource.URN{}

	editedPropDeps := false
	newPropDeps := map[resource.PropertyKey][]resource.URN{}

	for _, dep := range allDeps {
		if include != nil && !include(dep) {
			continue
		}

		switch dep.Type {
		case resource.ResourceParent:
			sb.setURN(&sb.state.Parent, updateURN(sb.state.Parent))
		case resource.ResourceDependency:
			newURN := updateURN(dep.URN)
			newDeps = append(newDeps, newURN)

			if newURN != dep.URN {
				editedDeps = true
			}
		case resource.ResourcePropertyDependency:
			newURN := updateURN(dep.URN)
			newPropDeps[dep.Key] = append(newPropDeps[dep.Key], newURN)

			if newURN != dep.URN {
				editedPropDeps = true
			}
		case resource.ResourceDeletedWith:
			sb.setURN(&sb.state.DeletedWith, updateURN(sb.state.DeletedWith))
		}
	}

	// Only update dependencies and property dependencies if we've actually made changes.
	if editedDeps {
		sb.state.Dependencies = newDeps
		sb.edited = true
	}
	if editedPropDeps {
		sb.state.PropertyDependencies = newPropDeps
		sb.edited = true
	}

	return sb
}

// Removes all "Aliases" from the state. Once URN normalisation is done we don't want to write aliases out.
func (sb *stateBuilder) withUpdatedAliases() *stateBuilder {
	if len(sb.state.Aliases) > 0 {
		sb.state.Aliases = nil
		sb.edited = true
	}
	return sb
}

// build returns the resulting state object. If no visible changes have been made, build will return the original object
// unmodified.
func (sb *stateBuilder) build() *resource.State {
	if !sb.edited {
		return sb.original
	}
	return &sb.state
}

// internal
func (sb *stateBuilder) setString(loc *string, value string) {
	if *loc == value {
		return
	}
	sb.edited = true
	*loc = value
}

// internal
func (sb *stateBuilder) setURN(loc *resource.URN, value resource.URN) {
	if *loc == value {
		return
	}
	sb.edited = true
	*loc = value
}
