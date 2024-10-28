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

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// A ProgramSpec specifies a Pulumi program whose execution will be mocked in a lifecycle test in order to register
// resources.
type ProgramSpec struct {
	// The set of resource registrations made by the program.
	ResourceRegistrations []*ResourceSpec

	// The set of resources present in the snapshot the program will execute against that will *not* be registered (that
	// is, they will be "dropped").
	Drops []*ResourceSpec
}

// Returns a new LanguageRuntimeFactory that will register the resources specified in this ProgramSpec when executed.
func (ps *ProgramSpec) AsLanguageRuntimeF(t require.TestingT) deploytest.LanguageRuntimeFactory {
	return deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		for _, r := range ps.ResourceRegistrations {
			opts := deploytest.ResourceOptions{
				Protect:        r.Protect,
				Dependencies:   r.Dependencies,
				PropertyDeps:   r.PropertyDependencies,
				RetainOnDelete: r.RetainOnDelete,
				DeletedWith:    r.DeletedWith,
			}

			_, err := monitor.RegisterResource(r.Type, r.Name, r.Custom, opts)
			if err != nil {
				require.NoError(t, err)
			}
			require.NoError(t, err)
		}

		return nil
	})
}

// Implements PrettySpec.Pretty. Returns a human-readable representation of this ProgramSpec, suitable for use in
// debugging output and error messages.
func (ps *ProgramSpec) Pretty(indent string) string {
	rendered := fmt.Sprintf("%sProgram %p", indent, ps)

	if len(ps.ResourceRegistrations) == 0 {
		rendered += fmt.Sprintf("\n%s  No registrations", indent)
	} else {
		rendered += fmt.Sprintf("\n%s  Registrations (%d):", indent, len(ps.ResourceRegistrations))
		for _, r := range ps.ResourceRegistrations {
			rendered += fmt.Sprintf("\n%s%s", indent, r.Pretty(indent+"    "))
		}
	}

	if len(ps.Drops) == 0 {
		rendered += fmt.Sprintf("\n%s  No drops", indent)
	} else {
		rendered += fmt.Sprintf("\n%s  Drops (%d):", indent, len(ps.Drops))
		for _, r := range ps.Drops {
			rendered += fmt.Sprintf("\n%s%s", indent, r.Pretty(indent+"    "))
		}
	}

	return rendered
}

// The type of tags that may be added to resources in a ProgramSpec.
type ProgramResourceTag string

const (
	// Tags a resource as having been newly prepended to the program.
	NewlyPrependedProgramResource ProgramResourceTag = "program.newly-prepended"

	// Tags a resource as having been dropped from the program.
	DroppedProgramResource ProgramResourceTag = "program.dropped"

	// Tags a resource as having been newly inserted into the program (that is, between two resources that already exist
	// in the snapshot the program will execute against).
	NewlyInsertedProgramResource ProgramResourceTag = "program.newly-inserted"

	// Tags a resource as having been updated in the program (that is, provided a new set of inputs to those held in the
	// snapshot that the program will execute against).
	UpdatedProgramResource ProgramResourceTag = "program.updated"

	// Tags a resource as having been copied from the snapshot the program will execute against.
	CopiedProgramResource ProgramResourceTag = "program.copied"

	// Tags a resource as having been newly appended to the program.
	NewlyAppendedProgramResource ProgramResourceTag = "program.newly-appended"
)

// A set of options for configuring the generation of a ProgramSpec.
type ProgramSpecOptions struct {
	PrependCount         *rapid.Generator[int]
	PrependResourceOpts  ResourceSpecOptions
	Action               *rapid.Generator[ProgramSpecAction]
	InsertResourceOpts   ResourceSpecOptions
	UpdateProtect        *rapid.Generator[bool]
	UpdateRetainOnDelete *rapid.Generator[bool]
	AppendCount          *rapid.Generator[int]
	AppendResourceOpts   ResourceSpecOptions
}

// The set of actions that may be taken on resources in a ProgramSpec.
type ProgramSpecAction string

const (
	// Deletes a resource from the program.
	ProgramSpecDelete ProgramSpecAction = "delete"
	// Inserts a new resource into the program.
	ProgramSpecInsert ProgramSpecAction = "insert"
	// Updates a resource in the program.
	ProgramSpecUpdate ProgramSpecAction = "update"
	// Copies a resource from the snapshot the program will execute against.
	ProgramSpecCopy ProgramSpecAction = "copy"
)

// Returns a copy of the given ProgramSpecOptions with the given overrides applied.
func (pso ProgramSpecOptions) With(overrides ProgramSpecOptions) ProgramSpecOptions {
	if overrides.PrependCount != nil {
		pso.PrependCount = overrides.PrependCount
	}
	pso.PrependResourceOpts = pso.PrependResourceOpts.With(overrides.PrependResourceOpts)
	if overrides.Action != nil {
		pso.Action = overrides.Action
	}
	pso.InsertResourceOpts = pso.InsertResourceOpts.With(overrides.InsertResourceOpts)
	if overrides.UpdateProtect != nil {
		pso.UpdateProtect = overrides.UpdateProtect
	}
	if overrides.UpdateRetainOnDelete != nil {
		pso.UpdateRetainOnDelete = overrides.UpdateRetainOnDelete
	}
	if overrides.AppendCount != nil {
		pso.AppendCount = overrides.AppendCount
	}
	pso.AppendResourceOpts = pso.AppendResourceOpts.With(overrides.AppendResourceOpts)

	return pso
}

// A default set of ProgramSpecOptions. By default, we'll prepend and append between 0 and 2 resources, and take actions
// on existing resources with equal probability.
var defaultProgramSpecOptions = ProgramSpecOptions{
	PrependCount:         rapid.IntRange(0, 2),
	PrependResourceOpts:  defaultResourceSpecOptions,
	Action:               rapid.SampledFrom(programSpecActions),
	InsertResourceOpts:   defaultResourceSpecOptions,
	UpdateProtect:        rapid.Bool(),
	UpdateRetainOnDelete: rapid.Bool(),
	AppendCount:          rapid.IntRange(0, 2),
	AppendResourceOpts:   defaultResourceSpecOptions,
}

var programSpecActions = []ProgramSpecAction{
	ProgramSpecDelete,
	ProgramSpecInsert,
	ProgramSpecUpdate,
	ProgramSpecCopy,
}

// Given a SnapshotSpec and a set of options, returns a rapid.Generator that will produce ProgramSpecs that operate upon
// the specified snapshot.
func GeneratedProgramSpec(
	ss *SnapshotSpec,
	sso StackSpecOptions,
	pso ProgramSpecOptions,
) *rapid.Generator[*ProgramSpec] {
	sso = defaultStackSpecOptions.With(sso)
	pso = defaultProgramSpecOptions.With(pso)

	return rapid.Custom(func(t *rapid.T) *ProgramSpec {
		drops := []*ResourceSpec{}
		dropped := map[resource.URN]bool{}

		newSS := &SnapshotSpec{}

		prependCount := pso.PrependCount.Draw(t, "ProgramSpec.PrependCount")
		if prependCount > 0 {
			// If we're going to prepend resources, we need to ensure that we have a provider to use for them.
			initialProvider := generatedProviderResourceSpec(newSS, sso).Draw(t, "SnapshotSpec.InitialProvider")
			AddTag(initialProvider, "program.provider.initial")
			newSS.AddProvider(initialProvider)
		}

		for i := 0; i < prependCount; i++ {
			r := generatedNewResourceSpec(
				newSS,
				sso,
				pso.PrependResourceOpts,
			).Draw(t, fmt.Sprintf("ProgramSpec.PrependedResource[%d]", i))
			AddTag(r, NewlyPrependedProgramResource)
			newSS.AddResource(r)
		}

		for i := 0; i < len(ss.Resources); {
			action := pso.Action.Draw(t, fmt.Sprintf("ProgramSpec.Action[%d]", i))
			if action == ProgramSpecDelete || ss.Resources[i].Delete {
				r := ss.Resources[i].Copy()
				if providers.IsProviderType(r.Type) {
					AddTag(r, CopiedProgramResource)
					newSS.AddResource(r)
				} else {
					AddTag(r, DroppedProgramResource)
					drops = append(drops, r)
					dropped[r.URN()] = true
				}

				i++
			} else if action == ProgramSpecInsert && len(newSS.Providers) > 0 {
				r := generatedNewResourceSpec(
					newSS,
					sso,
					pso.InsertResourceOpts,
				).Draw(t, fmt.Sprintf("ProgramSpec.InsertedResource[%d]", i))
				AddTag(r, NewlyInsertedProgramResource)
				newSS.AddResource(r)
			} else if action == ProgramSpecUpdate {
				r := ss.Resources[i].Copy()
				AddTag(r, UpdatedProgramResource)

				// We'll generate a new set of dependencies for the updated resource, which means we'll automatically take any
				// drops or deletions into account.
				rds := GeneratedResourceDependencies(newSS, r, includeAll).
					Draw(t, fmt.Sprintf("ProgramSpec.UpdatedResource[%d].Dependencies", i))

				if pso.UpdateProtect != nil {
					r.Protect = pso.UpdateProtect.Draw(t, fmt.Sprintf("ProgramSpec.UpdatedResource[%d].Protect", i))
				}
				if pso.UpdateRetainOnDelete != nil {
					r.RetainOnDelete = pso.UpdateRetainOnDelete.Draw(
						t,
						fmt.Sprintf("ProgramSpec.UpdatedResource[%d].RetainOnDelete", i),
					)
				}

				r.Dependencies = rds.Dependencies
				r.PropertyDependencies = rds.PropertyDependencies
				r.DeletedWith = rds.DeletedWith

				newSS.AddResource(r)
				i++
			} else {
				r := ss.Resources[i].Copy()
				AddTag(r, CopiedProgramResource)

				// If we copy a resource from the snapshot, we need to ensure that its dependencies have been updated to take
				// deletions/drops into account.
				deps := []resource.URN{}
				propDeps := map[resource.PropertyKey][]resource.URN{}

				for _, dep := range r.Dependencies {
					if dropped[dep] {
						continue
					}

					deps = append(deps, dep)
				}

				for k, deps := range r.PropertyDependencies {
					for _, dep := range deps {
						if dropped[dep] {
							continue
						}

						propDeps[k] = append(propDeps[k], dep)
					}
				}

				if dropped[r.DeletedWith] {
					r.DeletedWith = ""
				}

				r.Dependencies = deps
				r.PropertyDependencies = propDeps

				newSS.AddResource(r)
				i++
			}
		}

		appendCount := pso.AppendCount.Draw(t, "ProgramSpec.AppendCount")
		if appendCount > 0 && len(newSS.Providers) == 0 {
			// If we're going to append resources and the snapshot is still empty, we need to ensure that we have a provider
			// to use for them.
			initialProvider := generatedProviderResourceSpec(newSS, sso).Draw(t, "SnapshotSpec.InitialProvider")
			AddTag(initialProvider, "program.provider.initial")
			newSS.AddProvider(initialProvider)
		}

		for i := 0; i < appendCount; i++ {
			r := generatedNewResourceSpec(
				newSS,
				sso,
				pso.AppendResourceOpts,
			).Draw(t, fmt.Sprintf("ProgramSpec.AppendedResource[%d]", i))
			AddTag(r, NewlyAppendedProgramResource)
			newSS.AddResource(r)
		}

		ps := &ProgramSpec{
			ResourceRegistrations: newSS.Resources,
			Drops:                 drops,
		}

		return ps
	})
}
