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

// Simplifies non-mutating edits on resource.State
type stateBuilder struct {
	state    resource.State
	original *resource.State
	edited   bool
}

func newStateBuilder(state *resource.State) *stateBuilder {
	return &stateBuilder{*(state.Copy()), state, false}
}

func (sb *stateBuilder) withUpdatedURN(update func(resource.URN) resource.URN) *stateBuilder {
	sb.setURN(&sb.state.URN, update(sb.state.URN))
	return sb
}

func (sb *stateBuilder) withUpdatedParent(update func(resource.URN) resource.URN) *stateBuilder {
	if sb.state.Parent != "" {
		sb.setURN(&sb.state.Parent, update(sb.state.Parent))
	}
	return sb
}

func (sb *stateBuilder) withUpdatedProvider(update func(string) string) *stateBuilder {
	if sb.state.Provider != "" {
		sb.setString(&sb.state.Provider, update(sb.state.Provider))
	}
	return sb
}

func (sb *stateBuilder) withUpdatedDependencies(update func(resource.URN) resource.URN) *stateBuilder {
	var edited bool
	edited, sb.state.Dependencies = sb.updateURNSlice(sb.state.Dependencies, update)
	sb.edited = sb.edited || edited
	return sb
}

func (sb *stateBuilder) withUpdatedPropertyDependencies(update func(resource.URN) resource.URN) *stateBuilder {
	m := map[resource.PropertyKey][]resource.URN{}
	edited := false
	for k, urns := range sb.state.PropertyDependencies {
		var edit bool
		edit, m[k] = sb.updateURNSlice(urns, update)
		edited = edited || edit
	}
	if edited {
		sb.state.PropertyDependencies = m
	}
	sb.edited = sb.edited || edited
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

// internal
func (sb *stateBuilder) updateURNSlice(
	slice []resource.URN,
	update func(resource.URN) resource.URN,
) (bool, []resource.URN) {
	needsUpdate := false
	for _, urn := range slice {
		if update(urn) != urn {
			needsUpdate = true
			break
		}
	}
	if !needsUpdate {
		return false, slice
	}
	updated := make([]resource.URN, len(slice))
	for i, urn := range slice {
		updated[i] = update(urn)
	}
	return true, updated
}
