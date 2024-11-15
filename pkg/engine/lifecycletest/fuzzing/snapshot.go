// Copyright 2024, Pulumi Corporation.
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

package fuzzing

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"golang.org/x/exp/maps"
	"pgregory.net/rapid"
)

// A SnapshotSpec specifies a snapshot containing a set of resources managed by a set of providers.
type SnapshotSpec struct {
	// A mapping from package names to provider resources. The provider resources will also be included in the Resources
	// slice -- this field serves as a means to quickly look up provider resources by package name.
	Providers map[tokens.Package]*ResourceSpec

	// The set of resources in the snapshot.
	Resources []*ResourceSpec
}

// Creates a SnapshotSpec from the given deploy.Snapshot.
func FromSnapshot(s *deploy.Snapshot) *SnapshotSpec {
	ss := &SnapshotSpec{
		Providers: map[tokens.Package]*ResourceSpec{},
	}

	for _, r := range s.Resources {
		rs := FromResource(r)
		if providers.IsProviderType(rs.Type) {
			ss.AddProvider(rs)
		} else {
			ss.AddResource(rs)
		}
	}

	return ss
}

// Creates a SnapshotSpec from the ResourceV3s in the given DeploymentV3.
func FromDeploymentV3(d *apitype.DeploymentV3) *SnapshotSpec {
	ss := &SnapshotSpec{
		Providers: map[tokens.Package]*ResourceSpec{},
	}

	for _, r := range d.Resources {
		rs := FromResourceV3(r)
		if providers.IsProviderType(rs.Type) {
			ss.AddProvider(rs)
		} else {
			ss.AddResource(rs)
		}
	}

	return ss
}

// Adds the given provider to the snapshot's lookup table and list of resources.
func (s *SnapshotSpec) AddProvider(r *ResourceSpec) {
	if s.Providers == nil {
		s.Providers = map[tokens.Package]*ResourceSpec{}
	}

	s.Providers[tokens.Package(r.URN().Type().Name())] = r
	s.AddResource(r)
}

// Adds the given resource to the snapshot.
func (s *SnapshotSpec) AddResource(r *ResourceSpec) {
	s.Resources = append(s.Resources, r)
}

// Returns a deploy.Snapshot representation of this SnapshotSpec, suitable for use in setting up a lifecycle test.
func (s *SnapshotSpec) AsSnapshot() *deploy.Snapshot {
	resources := make([]*resource.State, len(s.Resources))
	for i, r := range s.Resources {
		resources[i] = r.AsResource()
	}

	return &deploy.Snapshot{
		Resources: resources,
	}
}

// Implements PrettySpec.Pretty. Returns a human-readable string representation of this SnapshotSpec, suitable for use
// in debugging output and error messages.
func (s *SnapshotSpec) Pretty(indent string) string {
	rendered := fmt.Sprintf("%sSnapshot %p", indent, s)
	if len(s.Resources) == 0 {
		rendered += fmt.Sprintf("\n%s  No resources", indent)
	} else {
		rendered += fmt.Sprintf("\n%s  Resources (%d):", indent, len(s.Resources))
		for _, r := range s.Resources {
			rendered += fmt.Sprintf("\n%s    %s", indent, r.Pretty(indent+"    "))
		}
	}

	return rendered
}

// A ResourceDependenciesSpec specifies the dependencies of a resource in a snapshot.
type ResourceDependenciesSpec struct {
	Parent               resource.URN
	Dependencies         []resource.URN
	PropertyDependencies map[resource.PropertyKey][]resource.URN
	DeletedWith          resource.URN
}

// ApplyTo applies the dependencies specified in this ResourceDependenciesSpec to the given ResourceSpec.
func (rds *ResourceDependenciesSpec) ApplyTo(r *ResourceSpec) {
	r.Parent = rds.Parent
	r.Dependencies = rds.Dependencies
	r.PropertyDependencies = rds.PropertyDependencies
	r.DeletedWith = rds.DeletedWith
}

// Given a SnapshotSpec and ResourceSpec, returns a rapid.Generator that yields random (valid) sets of dependencies for
// the given resource on resources in the given snapshot.
func GeneratedResourceDependencies(
	ss *SnapshotSpec,
	r *ResourceSpec,
	include func(*ResourceSpec) bool,
) *rapid.Generator[*ResourceDependenciesSpec] {
	return rapid.Custom(func(t *rapid.T) *ResourceDependenciesSpec {
		rds := &ResourceDependenciesSpec{
			Dependencies:         []resource.URN{},
			PropertyDependencies: map[resource.PropertyKey][]resource.URN{},
			DeletedWith:          "",
		}

		// As the number of resources in a snapshot grows, the probability of picking none of them will decrease rapidly if
		// we simply ask whether or not to include each resource. Since an empty set of dependencies is a relatively common
		// case, we'll thus introduce a separate branch for it up front.
		if rapid.Bool().Draw(t, "SnapshotDependencies.Empty") {
			return rds
		}

		seenDeps := map[resource.URN]bool{}
		seenPropDeps := map[resource.PropertyKey]map[resource.URN]bool{}

		sampledDep := false
		for _, sr := range ss.Resources {
			if sr == nil || sr.URN() == r.URN() || !include(sr) {
				continue
			}

			sampledDep = true

			depType := rapid.SampledFrom(stateDependencyTypes).Draw(t, "SnapshotDependencies.DependencyType")
			switch depType {
			case resource.ResourceParent:
				rds.Parent = sr.URN()
			case resource.ResourceDependency:
				if seenDeps[sr.URN()] {
					continue
				}

				seenDeps[sr.URN()] = true
				rds.Dependencies = append(rds.Dependencies, sr.URN())
			case resource.ResourcePropertyDependency:
				k := rapid.SampledFrom(append(
					maps.Keys(rds.PropertyDependencies),
					resource.PropertyKey(
						rapid.StringMatching("^prop-[a-z][A-Za-z0-9]{3}$").Draw(t, "SnapshotDependencies.NewPropertyKey"),
					),
				)).Draw(t, "SnapshotDependencies.PropertyKey")

				if seenPropDeps[k] == nil {
					seenPropDeps[k] = map[resource.URN]bool{}
				}
				if seenPropDeps[k][sr.URN()] {
					continue
				}

				rds.PropertyDependencies[k] = append(rds.PropertyDependencies[k], sr.URN())
			case resource.ResourceDeletedWith:
				rds.DeletedWith = sr.URN()
			default:
				continue
			}
		}

		if !sampledDep {
			// Rapid insists that all generators must draw at least one bit of entropy. Therefore, if we didn't pick any
			// dependencies, we'll use the Just generator to explicitly draw one bit by returning the empty set of
			// dependencies.
			return rapid.Just(rds).Draw(t, "SnapshotDependencies.Empty")
		}

		return rds
	})
}

var stateDependencyTypes = []resource.StateDependencyType{
	"",
	resource.ResourceParent,
	resource.ResourceDependency,
	resource.ResourcePropertyDependency,
	resource.ResourceDeletedWith,
}

// A set of options for configuring the generation of a SnapshotSpec.
type SnapshotSpecOptions struct {
	// A source DeploymentV3 from which resources should be taken literally,
	// skipping the generation process.
	SourceDeploymentV3 *apitype.DeploymentV3

	// A generator for the maximum number of resources to generate in the snapshot.
	ResourceCount *rapid.Generator[int]

	// A generator for actions that should be taken when generating a snapshot.
	Action *rapid.Generator[SnapshotSpecAction]

	// A set of options for configuring the generation of resources in the snapshot.
	ResourceOpts ResourceSpecOptions
}

// Returns a copy of the given SnapshotSpecOptions with the given overrides applied.
func (sso SnapshotSpecOptions) With(overrides SnapshotSpecOptions) SnapshotSpecOptions {
	if overrides.SourceDeploymentV3 != nil {
		sso.SourceDeploymentV3 = overrides.SourceDeploymentV3
	}
	if overrides.ResourceCount != nil {
		sso.ResourceCount = overrides.ResourceCount
	}
	if overrides.Action != nil {
		sso.Action = overrides.Action
	}
	sso.ResourceOpts = sso.ResourceOpts.With(overrides.ResourceOpts)

	return sso
}

// The type of action to take when generating a resource in a snapshot.
type SnapshotSpecAction string

const (
	// Generate a new resource.
	SnapshotSpecNew SnapshotSpecAction = "snapshot.new"
	// Generate an old (deleted) version of an existing resource in the snapshot.
	SnapshotSpecOld SnapshotSpecAction = "snapshot.old"
	// Generate a provider resource.
	SnapshotSpecProvider SnapshotSpecAction = "snapshot.provider"
)

// A default set of SnapshotSpecOptions. By default, we'll generate a snapshot with between 2 and 5 resources, with
// equal probability that each resource will be new, old, or a provider. Resources will be created using the default set
// of ResourceSpecOptions.
var defaultSnapshotSpecOptions = SnapshotSpecOptions{
	SourceDeploymentV3: nil,
	ResourceCount:      rapid.IntRange(2, 5),
	Action:             rapid.SampledFrom(snapshotSpecActions),
	ResourceOpts:       defaultResourceSpecOptions,
}

var snapshotSpecActions = []SnapshotSpecAction{
	SnapshotSpecNew,
	SnapshotSpecOld,
	SnapshotSpecProvider,
}

// Given a set of StackSpecOptions and SnapshotSpecOptions, returns a rapid.Generator that yields random SnapshotSpecs.
func GeneratedSnapshotSpec(sso StackSpecOptions, snso SnapshotSpecOptions) *rapid.Generator[*SnapshotSpec] {
	sso = defaultStackSpecOptions.With(sso)
	snso = defaultSnapshotSpecOptions.With(snso)

	return rapid.Custom(func(t *rapid.T) *SnapshotSpec {
		if snso.SourceDeploymentV3 != nil {
			// Rapid insists that all generators must draw at least one bit of entropy. Therefore, if we were given a source
			// to draw resources from literally, we'll use the Just generator to explicitly draw one bit by returning those
			// resources as-is.
			ss := rapid.Just(FromDeploymentV3(snso.SourceDeploymentV3)).Draw(t, "SnapshotSpec.SourceDeploymentV3")
			return ss
		}

		ss := &SnapshotSpec{}

		newResource := generatedNewResourceSpec(ss, sso, snso.ResourceOpts)
		oldResource := generatedOldResourceSpec(ss, sso, snso.ResourceOpts)
		providerResource := generatedProviderResourceSpec(ss, sso)

		// Seed the snapshot with an initial provider resource that can be used.
		initialProvider := providerResource.Draw(t, "SnapshotSpec.InitialProvider")
		AddTag(initialProvider, "snapshot.provider.initial")
		ss.AddProvider(initialProvider)

		resourceCount := snso.ResourceCount.Draw(t, "SnapshotSpec.ResourceCount")

		for i := 0; i < resourceCount; i++ {
			action := snso.Action.Draw(t, "SnapshotSpec.Action")

			switch action {
			case SnapshotSpecNew:
				r := newResource.Draw(t, fmt.Sprintf("SnapshotSpec.ResourceSpec[%d]", i))
				ss.AddResource(r)
			case SnapshotSpecOld:
				r := oldResource.Draw(t, fmt.Sprintf("SnapshotSpec.ResourceSpec[%d]", i))
				ss.AddResource(r)
			case SnapshotSpecProvider:
				r := providerResource.Draw(t, fmt.Sprintf("SnapshotSpec.ResourceSpec[%d]", i))
				ss.AddProvider(r)
			}
		}

		return ss
	})
}

func generatedNewResourceSpec(
	ss *SnapshotSpec,
	sso StackSpecOptions,
	rso ResourceSpecOptions,
) *rapid.Generator[*ResourceSpec] {
	return rapid.Custom(func(t *rapid.T) *ResourceSpec {
		r := GeneratedResourceSpec(sso, rso, ss.Providers).Draw(t, "NewResourceSpec.Base")
		rds := GeneratedResourceDependencies(ss, r, includeAll).Draw(t, "NewResourceSpec.Dependencies")

		rds.ApplyTo(r)

		return r
	})
}

func generatedOldResourceSpec(
	ss *SnapshotSpec,
	sso StackSpecOptions,
	rso ResourceSpecOptions,
) *rapid.Generator[*ResourceSpec] {
	return rapid.Custom(func(t *rapid.T) *ResourceSpec {
		var r *ResourceSpec
		if len(ss.Resources) == 0 {
			r = GeneratedResourceSpec(sso, rso, ss.Providers).Draw(t, "OldResourceSpec.New")
		} else {
			r = rapid.SampledFrom(ss.Resources).Draw(t, "OldResourceSpec.Base").Copy()
		}

		AddTag(r, SnapshotSpecOld)
		rds := GeneratedResourceDependencies(ss, r, isDeleted).Draw(t, "OldResourceSpec.Dependencies")

		r.Delete = true

		// We'll randomly change some attributes of the old resource to cover cases where e.g. a deleted resource used to be
		// retained on deletion but its replacement isn't.
		r.Protect = rapid.Bool().Draw(t, "OldResourceSpec.Protect")
		r.RetainOnDelete = rapid.Bool().Draw(t, "OldResourceSpec.RetainOnDelete")

		rds.ApplyTo(r)

		return r
	})
}

func generatedProviderResourceSpec(ss *SnapshotSpec, sso StackSpecOptions) *rapid.Generator[*ResourceSpec] {
	return rapid.Custom(func(t *rapid.T) *ResourceSpec {
		r := GeneratedProviderResourceSpec(sso).Draw(t, "ProviderResourceSpec.Base")
		rds := GeneratedResourceDependencies(ss, r, includeAll).Draw(t, "ProviderResourceSpec.Dependencies")

		rds.ApplyTo(r)

		return r
	})
}

func includeAll(r *ResourceSpec) bool {
	return !providers.IsProviderType(r.Type)
}

func isDeleted(r *ResourceSpec) bool {
	return !providers.IsProviderType(r.Type) && r.Delete
}
