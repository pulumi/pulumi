// Copyright 2016-2024, Pulumi Corporation.
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
	"sync"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// State is a structure containing state associated with a resource. This resource may have been serialized and
// deserialized, or snapshotted from a live graph of resource objects. The value's state is not, however, associated
// with any runtime objects in memory that may be actively involved in ongoing computations.
//
// Only test code should create State values directly. All other code should use [NewState].
//
//nolint:lll
type State struct {
	// Currently the engine implements RegisterResourceOutputs by directly mutating the state to change the `Outputs`. This
	// triggers a race between the snapshot serialization code and the engine. Ideally we'd do a more principled fix, but
	// just locking in these two places is sufficient to stop the race detector from firing on integration tests.
	Lock sync.Mutex

	Type                    tokens.Type           // the resource's type.
	URN                     URN                   // the resource's object urn, a human-friendly, unique name for the resource.
	Custom                  bool                  // true if the resource is custom, managed by a plugin.
	Delete                  bool                  // true if this resource is pending deletion due to a replacement.
	ID                      ID                    // the resource's unique ID, assigned by the resource provider (or blank if none/uncreated).
	Inputs                  PropertyMap           // the resource's input properties (as specified by the program).
	Outputs                 PropertyMap           // the resource's complete output state (as returned by the resource provider).
	Parent                  URN                   // an optional parent URN that this resource belongs to.
	Protect                 bool                  // true to "protect" this resource (protected resources cannot be deleted).
	Taint                   bool                  // true to force replacement of this resource during the next update.
	External                bool                  // true if this resource is "external" to Pulumi and we don't control the lifecycle.
	Dependencies            []URN                 // the resource's dependencies.
	InitErrors              []string              // the set of errors encountered in the process of initializing resource.
	Provider                string                // the provider to use for this resource.
	PropertyDependencies    map[PropertyKey][]URN // the set of dependencies that affect each property.
	PendingReplacement      bool                  // true if this resource was deleted and is awaiting replacement.
	AdditionalSecretOutputs []PropertyKey         // an additional set of outputs that should be treated as secrets.
	Aliases                 []URN                 // an optional set of URNs for which this resource is an alias.
	CustomTimeouts          CustomTimeouts        // A config block that will be used to configure timeouts for CRUD operations.
	ImportID                ID                    // the resource's import id, if this was an imported resource.
	RetainOnDelete          bool                  // if set to True, the providers Delete method will not be called for this resource.
	DeletedWith             URN                   // If set, the providers Delete method will not be called for this resource if specified resource is being deleted as well.
	Created                 *time.Time            // If set, the time when the state was initially added to the state file. (i.e. Create, Import)
	Modified                *time.Time            // If set, the time when the state was last modified in the state file.
	SourcePosition          string                // If set, the source location of the resource registration
	StackTrace              []StackFrame          // If set, the stack trace at time of registration
	IgnoreChanges           []string              // If set, the list of properties to ignore changes for.
	ReplaceOnChanges        []string              // If set, the list of properties that if changed trigger a replace.
	HideDetailedDiff        []property.GlobPath   // If set, the list of properties that should hide their detailed diffs.
	RefreshBeforeUpdate     bool                  // true if this resource should always be refreshed prior to updates.
	ViewOf                  URN                   // If set, the URN of the resource this resource is a view of.
	ResourceHooks           map[HookType][]string // The resource hooks attached to the resource, by type.
}

// Copy creates a shallow copy of the resource state, except without copying the lock.
func (s *State) Copy() *State {
	return &State{
		Type:                    s.Type,
		URN:                     s.URN,
		Custom:                  s.Custom,
		Delete:                  s.Delete,
		ID:                      s.ID,
		Inputs:                  s.Inputs,
		Outputs:                 s.Outputs,
		Parent:                  s.Parent,
		Protect:                 s.Protect,
		Taint:                   s.Taint,
		External:                s.External,
		Dependencies:            s.Dependencies,
		InitErrors:              s.InitErrors,
		Provider:                s.Provider,
		PropertyDependencies:    s.PropertyDependencies,
		PendingReplacement:      s.PendingReplacement,
		AdditionalSecretOutputs: s.AdditionalSecretOutputs,
		Aliases:                 s.Aliases,
		CustomTimeouts:          s.CustomTimeouts,
		ImportID:                s.ImportID,
		RetainOnDelete:          s.RetainOnDelete,
		DeletedWith:             s.DeletedWith,
		Created:                 s.Created,
		Modified:                s.Modified,
		SourcePosition:          s.SourcePosition,
		StackTrace:              s.StackTrace,
		IgnoreChanges:           s.IgnoreChanges,
		ReplaceOnChanges:        s.ReplaceOnChanges,
		HideDetailedDiff:        s.HideDetailedDiff,
		RefreshBeforeUpdate:     s.RefreshBeforeUpdate,
		ViewOf:                  s.ViewOf,
		ResourceHooks:           s.ResourceHooks,
	}
}

func (s *State) GetAliasURNs() []URN {
	return s.Aliases
}

func (s *State) GetAliases() []Alias {
	aliases := make([]Alias, len(s.Aliases))
	for i, alias := range s.Aliases {
		aliases[i] = Alias{URN: alias}
	}
	return aliases
}

// NewState is used to construct State values. The dataflow for State is rather sensitive, so all fields are required.
// Call [NewState.Make] to create the *State value.
//
//nolint:lll
type NewState struct {
	// the resource's type.
	Type tokens.Type // required

	// the resource's object urn, a human-friendly, unique name for the resource.
	URN URN // required

	// true if the resource is custom, managed by a plugin.
	Custom bool // required

	// true if this resource is pending deletion due to a replacement.
	Delete bool // required

	// the resource's unique ID, assigned by the resource provider (or blank if none/uncreated).
	ID ID // required

	// the resource's input properties (as specified by the program).
	Inputs PropertyMap // required

	// the resource's complete output state (as returned by the resource provider).
	Outputs PropertyMap // required

	// an optional parent URN that this resource belongs to.
	Parent URN // required

	// true to "protect" this resource (protected resources cannot be deleted).
	Protect bool // required

	// true to force replacement of this resource during the next update.
	Taint bool // required

	// true if this resource is "external" to Pulumi and we don't control the lifecycle.
	External bool // required

	// the resource's dependencies.
	Dependencies []URN // required

	// the set of errors encountered in the process of initializing resource.
	InitErrors []string // required

	// the provider to use for this resource.
	Provider string // required

	// the set of dependencies that affect each property.
	PropertyDependencies map[PropertyKey][]URN // required

	// true if this resource was deleted and is awaiting replacement.
	PendingReplacement bool // required

	// an additional set of outputs that should be treated as secrets.
	AdditionalSecretOutputs []PropertyKey // required

	// an optional set of URNs for which this resource is an alias.
	Aliases []URN // required

	// A config block that will be used to configure timeouts for CRUD operations.
	CustomTimeouts *CustomTimeouts // required

	// the resource's import id, if this was an imported resource.
	ImportID ID // required

	// if set to True, the providers Delete method will not be called for this resource.
	RetainOnDelete bool // required

	// If set, the providers Delete method will not be called for this resource if specified resource is being deleted as well.
	DeletedWith URN // required

	// If set, the time when the state was initially added to the state file. (i.e. Create, Import)
	Created *time.Time // required

	// If set, the time when the state was last modified in the state file.
	Modified *time.Time // required

	// If set, the source location of the resource registration
	SourcePosition string // required

	// If set, the stack trace at time of registration
	StackTrace []StackFrame // required

	// If set, the list of properties to ignore changes for.
	IgnoreChanges []string // required

	// If set, the list of properties that if changed trigger a replace.
	ReplaceOnChanges []string // required

	// If set, the list of properties that should hide their detailed diffs.
	HideDetailedDiff []property.GlobPath // required

	// true if this resource should always be refreshed prior to updates.
	RefreshBeforeUpdate bool // required

	// If set, the URN of the resource this resource is a view of.
	ViewOf URN // required

	// The resource hooks attached to the resource, by type.
	ResourceHooks map[HookType][]string // required
}

// Make consumes the NewState to create a *State.
func (s NewState) Make() *State {
	contract.Assertf(s.Type != "", "type was empty")
	contract.Assertf(s.Custom || s.ID == "", "is custom or had empty ID")

	var customTimeouts CustomTimeouts
	if s.CustomTimeouts != nil {
		customTimeouts = *s.CustomTimeouts
	}

	return &State{
		Type:                    s.Type,
		URN:                     s.URN,
		Custom:                  s.Custom,
		Delete:                  s.Delete,
		ID:                      s.ID,
		Inputs:                  s.Inputs,
		Outputs:                 s.Outputs,
		Parent:                  s.Parent,
		Protect:                 s.Protect,
		Taint:                   s.Taint,
		External:                s.External,
		Dependencies:            s.Dependencies,
		InitErrors:              s.InitErrors,
		Provider:                s.Provider,
		PropertyDependencies:    s.PropertyDependencies,
		PendingReplacement:      s.PendingReplacement,
		AdditionalSecretOutputs: s.AdditionalSecretOutputs,
		Aliases:                 s.Aliases,
		CustomTimeouts:          customTimeouts,
		ImportID:                s.ImportID,
		RetainOnDelete:          s.RetainOnDelete,
		DeletedWith:             s.DeletedWith,
		Created:                 s.Created,
		Modified:                s.Modified,
		SourcePosition:          s.SourcePosition,
		StackTrace:              s.StackTrace,
		IgnoreChanges:           s.IgnoreChanges,
		ReplaceOnChanges:        s.ReplaceOnChanges,
		HideDetailedDiff:        s.HideDetailedDiff,
		RefreshBeforeUpdate:     s.RefreshBeforeUpdate,
		ViewOf:                  s.ViewOf,
		ResourceHooks:           s.ResourceHooks,
	}
}

// StateDependency objects are used when enumerating all the dependencies of a
// resource. They encapsulate the various types of dependency relationships that
// Pulumi resources may have with one another.
type StateDependency struct {
	// The type of dependency.
	Type StateDependencyType
	// If the dependency is a property dependency, the property key that owns the
	// dependency.
	Key PropertyKey
	// The URN of the resource that is being depended on.
	URN URN
}

// The type of dependencies that a resource may have.
type StateDependencyType string

const (
	// ResourceParent is the type of parent-child dependency relationships. The
	// resource being depended on is the parent of the dependent resource.
	ResourceParent StateDependencyType = "parent"
	// ResourceDependency is the type of dependency relationships where there is
	// no specific property owning the dependency.
	ResourceDependency StateDependencyType = "dependency"
	// ResourcePropertyDependency is the type of dependency relationships where a
	// specific property makes reference to another resource.
	ResourcePropertyDependency StateDependencyType = "property-dependency"
	// ResourceDeletedWith is the type of dependency relationships where a
	// resource will be "deleted with" another. The resource being depended on is
	// one whose deletion subsumes the deletion of the dependent resource.
	ResourceDeletedWith StateDependencyType = "deleted-with"
)

// GetAllDependencies returns a resource's provider and all of its dependencies.
// For use cases that rely on processing all possible links between sets of
// resources, this method (coupled with e.g. an exhaustive switch over the types
// of dependencies returned) should be preferred over direct access to e.g.
// Dependencies, PropertyDependencies, and so on.
func (s *State) GetAllDependencies() (string, []StateDependency) {
	var allDeps []StateDependency
	if s.Parent != "" {
		allDeps = append(allDeps, StateDependency{Type: ResourceParent, URN: s.Parent})
	}
	for _, dep := range s.Dependencies {
		if dep != "" {
			allDeps = append(allDeps, StateDependency{Type: ResourceDependency, URN: dep})
		}
	}
	for key, deps := range s.PropertyDependencies {
		for _, dep := range deps {
			if dep != "" {
				allDeps = append(allDeps, StateDependency{Type: ResourcePropertyDependency, Key: key, URN: dep})
			}
		}
	}
	if s.DeletedWith != "" {
		allDeps = append(allDeps, StateDependency{Type: ResourceDeletedWith, URN: s.DeletedWith})
	}
	return s.Provider, allDeps
}

// StackFrames are used to record the stack at the time a resource is registered.
type StackFrame struct {
	// The source position associated with the stack frame.
	SourcePosition string
}
