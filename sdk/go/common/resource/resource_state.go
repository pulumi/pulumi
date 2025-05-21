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
)

// State is a structure containing state associated with a resource.  This resource may have been serialized and
// deserialized, or snapshotted from a live graph of resource objects.  The value's state is not, however, associated
// with any runtime objects in memory that may be actively involved in ongoing computations.
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
	IgnoreChanges           []string              // If set, the list of properties to ignore changes for.
	ReplaceOnChanges        []string              // If set, the list of properties that if changed trigger a replace.
	RefreshBeforeUpdate     bool                  // If set, provider requested to always refresh before updating.
}

// Copy creates a deep copy of the resource state, except without copying the lock.
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
		IgnoreChanges:           s.IgnoreChanges,
		ReplaceOnChanges:        s.ReplaceOnChanges,
		RefreshBeforeUpdate:     s.RefreshBeforeUpdate,
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

// NewState creates a new resource value from existing resource state information.
func NewState(t tokens.Type, urn URN, custom bool, del bool, id ID,
	inputs PropertyMap, outputs PropertyMap, parent URN, protect bool,
	external bool, dependencies []URN, initErrors []string, provider string,
	propertyDependencies map[PropertyKey][]URN, pendingReplacement bool,
	additionalSecretOutputs []PropertyKey, aliases []URN, timeouts *CustomTimeouts,
	importID ID, retainOnDelete bool, deletedWith URN, created *time.Time, modified *time.Time,
	sourcePosition string, ignoreChanges []string, replaceOnChanges []string, refreshBeforeUpdate bool,
) *State {
	contract.Assertf(t != "", "type was empty")
	contract.Assertf(custom || id == "", "is custom or had empty ID")

	s := &State{
		Type:                    t,
		URN:                     urn,
		Custom:                  custom,
		Delete:                  del,
		ID:                      id,
		Inputs:                  inputs,
		Outputs:                 outputs,
		Parent:                  parent,
		Protect:                 protect,
		External:                external,
		Dependencies:            dependencies,
		InitErrors:              initErrors,
		Provider:                provider,
		PropertyDependencies:    propertyDependencies,
		PendingReplacement:      pendingReplacement,
		AdditionalSecretOutputs: additionalSecretOutputs,
		Aliases:                 aliases,
		ImportID:                importID,
		RetainOnDelete:          retainOnDelete,
		DeletedWith:             deletedWith,
		Created:                 created,
		Modified:                modified,
		SourcePosition:          sourcePosition,
		IgnoreChanges:           ignoreChanges,
		ReplaceOnChanges:        replaceOnChanges,
		RefreshBeforeUpdate:     refreshBeforeUpdate,
	}

	if timeouts != nil {
		s.CustomTimeouts = *timeouts
	}

	return s
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
