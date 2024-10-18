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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"golang.org/x/exp/maps"
	"pgregory.net/rapid"
)

type SnapshotSpec struct {
	Providers map[tokens.Package]*ResourceSpec
	Resources []*ResourceSpec
}

func (s *SnapshotSpec) AddProvider(r *ResourceSpec) {
	if s.Providers == nil {
		s.Providers = map[tokens.Package]*ResourceSpec{}
	}

	s.Providers[tokens.Package(r.URN().Type().Name())] = r
}

func (s *SnapshotSpec) AsSnapshot() *deploy.Snapshot {
	resources := make([]*resource.State, len(s.Resources))
	for i, r := range s.Resources {
		resources[i] = r.AsResource()
	}

	return &deploy.Snapshot{
		Resources: resources,
	}
}

func (s *SnapshotSpec) Pretty(indent string) string {
	rendered := fmt.Sprintf("%sSnapshot %p", indent, s)
	if len(s.Resources) == 0 {
		rendered += fmt.Sprintf("\n%s  No resources", indent)
	} else {
		rendered += fmt.Sprintf("\n%s  Resources (%d):", indent, len(s.Resources))
		for _, r := range s.Resources {
			rendered += fmt.Sprintf("\n%s%s", indent, r.Pretty(indent+"    "))
		}
	}

	return rendered
}

type ResourceDependenciesSpec struct {
	Dependencies         []resource.URN
	PropertyDependencies map[resource.PropertyKey][]resource.URN
	DeletedWith          resource.URN
}

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
				// For now we don't support parents
				continue
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
			// TODO EXPLAIN that we must draw at least one bit of entropy from Rapid
			return rapid.Just(rds).Draw(t, "SnapshotDependencies.Empty")
		}

		return rds
	})
}

var stateDependencyTypes = []resource.StateDependencyType{
	"",
	resource.ResourceDependency,
	resource.ResourcePropertyDependency,
	resource.ResourceDeletedWith,
}

type SnapshotSpecOptions struct {
	ResourceCount *rapid.Generator[int]
	Action        *rapid.Generator[SnapshotSpecAction]
	ResourceOpts  ResourceSpecOptions
}

func (sso SnapshotSpecOptions) With(overrides SnapshotSpecOptions) SnapshotSpecOptions {
	if overrides.ResourceCount != nil {
		sso.ResourceCount = overrides.ResourceCount
	}
	if overrides.Action != nil {
		sso.Action = overrides.Action
	}
	sso.ResourceOpts = sso.ResourceOpts.With(overrides.ResourceOpts)

	return sso
}

type SnapshotSpecAction string

const (
	SnapshotSpecNew      SnapshotSpecAction = "snapshot.new"
	SnapshotSpecOld      SnapshotSpecAction = "snapshot.old"
	SnapshotSpecProvider SnapshotSpecAction = "snapshot.provider"
)

var defaultSnapshotSpecOptions = SnapshotSpecOptions{
	ResourceCount: rapid.IntRange(2, 5),
	Action:        rapid.SampledFrom(snapshotSpecActions),
	ResourceOpts:  defaultResourceSpecOptions,
}

var snapshotSpecActions = []SnapshotSpecAction{
	SnapshotSpecNew,
	SnapshotSpecOld,
	SnapshotSpecProvider,
}

func GeneratedSnapshotSpec(sso StackSpecOptions, snso SnapshotSpecOptions) *rapid.Generator[*SnapshotSpec] {
	sso = defaultStackSpecOptions.With(sso)
	snso = defaultSnapshotSpecOptions.With(snso)

	return rapid.Custom(func(t *rapid.T) *SnapshotSpec {
		ss := &SnapshotSpec{}

		newResource := generatedNewResourceSpec(ss, sso, snso.ResourceOpts)
		oldResource := generatedOldResourceSpec(ss, sso, snso.ResourceOpts)
		providerResource := generatedProviderResourceSpec(ss, sso)

		initialProvider := providerResource.Draw(t, "SnapshotSpec.InitialProvider")
		AddTag(initialProvider, "snapshot.provider.initial")
		ss.AddProvider(initialProvider)
		ss.Resources = append(ss.Resources, initialProvider)

		resourceCount := snso.ResourceCount.Draw(t, "SnapshotSpec.ResourceCount")

		for i := 0; i < resourceCount; i++ {
			action := snso.Action.Draw(t, "SnapshotSpec.Action")
			var r *ResourceSpec

			switch action {
			case SnapshotSpecNew:
				r = newResource.Draw(t, fmt.Sprintf("SnapshotSpec.ResourceSpec[%d]", i))
			case SnapshotSpecOld:
				r = oldResource.Draw(t, fmt.Sprintf("SnapshotSpec.ResourceSpec[%d]", i))
			case SnapshotSpecProvider:
				r = providerResource.Draw(t, fmt.Sprintf("SnapshotSpec.ResourceSpec[%d]", i))
				ss.AddProvider(r)
			}

			ss.Resources = append(ss.Resources, r)
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

		r.Dependencies = rds.Dependencies
		r.PropertyDependencies = rds.PropertyDependencies
		r.DeletedWith = rds.DeletedWith

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
		r.Protect = rapid.Bool().Draw(t, "OldResourceSpec.Protect")
		r.RetainOnDelete = rapid.Bool().Draw(t, "OldResourceSpec.RetainOnDelete")

		r.Dependencies = rds.Dependencies
		r.PropertyDependencies = rds.PropertyDependencies
		r.DeletedWith = rds.DeletedWith

		return r
	})
}

func generatedProviderResourceSpec(ss *SnapshotSpec, sso StackSpecOptions) *rapid.Generator[*ResourceSpec] {
	return rapid.Custom(func(t *rapid.T) *ResourceSpec {
		r := GeneratedProviderResourceSpec(sso).Draw(t, "ProviderResourceSpec.Base")
		rds := GeneratedResourceDependencies(ss, r, includeAll).Draw(t, "ProviderResourceSpec.Dependencies")

		r.Dependencies = rds.Dependencies
		r.PropertyDependencies = rds.PropertyDependencies
		r.DeletedWith = rds.DeletedWith

		return r
	})
}

func includeAll(r *ResourceSpec) bool {
	return !providers.IsProviderType(r.Type)
}

func isDeleted(r *ResourceSpec) bool {
	return !providers.IsProviderType(r.Type) && r.Delete
}
