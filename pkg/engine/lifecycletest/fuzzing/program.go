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

type ProgramSpec struct {
	ResourceRegistrations []*ResourceSpec
	Drops                 []*ResourceSpec
}

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

type ProgramResourceTag string

const (
	NewlyPrependedProgramResource ProgramResourceTag = "program.newly-prepended"
	DroppedProgramResource        ProgramResourceTag = "program.dropped"
	NewlyInsertedProgramResource  ProgramResourceTag = "program.newly-inserted"
	UpdatedProgramResource        ProgramResourceTag = "program.updated"
	CopiedProgramResource         ProgramResourceTag = "program.copied"
	NewlyAppendedProgramResource  ProgramResourceTag = "program.newly-appended"
)

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

type ProgramSpecAction string

const (
	ProgramSpecDelete ProgramSpecAction = "delete"
	ProgramSpecInsert ProgramSpecAction = "insert"
	ProgramSpecUpdate ProgramSpecAction = "update"
	ProgramSpecCopy   ProgramSpecAction = "copy"
)

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
			initialProvider := generatedProviderResourceSpec(newSS, sso).Draw(t, "SnapshotSpec.InitialProvider")
			AddTag(initialProvider, "program.provider.initial")
			newSS.AddProvider(initialProvider)
			newSS.Resources = append(newSS.Resources, initialProvider)
		}

		for i := 0; i < prependCount; i++ {
			r := generatedNewResourceSpec(
				newSS,
				sso,
				pso.PrependResourceOpts,
			).Draw(t, fmt.Sprintf("ProgramSpec.PrependedResource[%d]", i))
			AddTag(r, NewlyPrependedProgramResource)

			newSS.Resources = append(newSS.Resources, r)
		}

		for i := 0; i < len(ss.Resources); {
			action := pso.Action.Draw(t, fmt.Sprintf("ProgramSpec.Action[%d]", i))
			if action == ProgramSpecDelete || ss.Resources[i].Delete {
				r := ss.Resources[i].Copy()
				if providers.IsProviderType(r.Type) {
					AddTag(r, CopiedProgramResource)
					newSS.Resources = append(newSS.Resources, r)
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

				newSS.Resources = append(newSS.Resources, r)
			} else if action == ProgramSpecUpdate {
				r := ss.Resources[i].Copy()
				AddTag(r, UpdatedProgramResource)

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

				newSS.Resources = append(newSS.Resources, r)
				i++
			} else {
				r := ss.Resources[i].Copy()
				AddTag(r, CopiedProgramResource)

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

				newSS.Resources = append(newSS.Resources, r)
				i++
			}
		}

		appendCount := pso.AppendCount.Draw(t, "ProgramSpec.AppendCount")
		if appendCount > 0 && len(newSS.Providers) == 0 {
			initialProvider := generatedProviderResourceSpec(newSS, sso).Draw(t, "SnapshotSpec.InitialProvider")
			AddTag(initialProvider, "program.provider.initial")
			newSS.AddProvider(initialProvider)
			newSS.Resources = append(newSS.Resources, initialProvider)
		}

		for i := 0; i < appendCount; i++ {
			r := generatedNewResourceSpec(
				newSS,
				sso,
				pso.AppendResourceOpts,
			).Draw(t, fmt.Sprintf("ProgramSpec.AppendedResource[%d]", i))
			AddTag(r, NewlyAppendedProgramResource)

			newSS.Resources = append(newSS.Resources, r)
		}

		ps := &ProgramSpec{
			ResourceRegistrations: newSS.Resources,
			Drops:                 drops,
		}

		return ps
	})
}
