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
		// We want to emulate an actual lifecycle test as much as possible here. Consequently, rather than "hardcoding" URNs
		// from the ResourceSpecs, we'll map those URNs to the actual RegisterResourceResponses we get from the resource
		// monitor and use those to refer to resources in subsequent registrations. This is particularly important in the
		// presence of targeted operations. Consider the following ResourceSpec for a resource named res2:
		//
		//  ResourceSpec{
		//    Type: "pkgA:modA:typeA",
		//    Name: "res2",
		//    Parent: "urn:pulumi:stack::project::pkgA:modA:typeA::res1",
		//    AliasURNs: []resource.URN{"urn:pulumi:stack::project::pkgA:modA:typeA::res2"},
		//  }
		//
		// res2 has been updated to add a parent, res1, which previously it did not have. Its new URN,
		// urn:pulumi:stack::project::pkgA:modA:typeA$pkgA:modA:typeA::res2, will include res1's type. Its old URN,
		// urn:pulumi:stack::project::pkgA:modA:typeA::res2, with just res2's type, has been added as an alias. When we
		// register res2, we will receive a different URN back based on whether we target it or not:
		//
		// * If we target it, we'll get the new URN, since the alias will find the old state, which will then be updated to
		//   add the parent.
		//
		// * If we don't target it, we'll get the old URN, since the alias will find the old state and we'll emit a SameStep
		//   with that state.
		//
		// Hardcoding the URN from the ResourceSpec would cause us to generate bad programs in the second case.
		actuals := map[resource.URN]*deploytest.RegisterResourceResponse{}
		rewriteProviderRef := func(oldRef string) string {
			if oldRef == "" {
				return ""
			}

			parsed, err := providers.ParseReference(oldRef)
			require.NoError(t, err)

			res, hasRes := actuals[parsed.URN()]
			if !hasRes {
				return oldRef
			}

			newRef, err := providers.NewReference(res.URN, res.ID)
			require.NoError(t, err)

			return newRef.String()
		}

		rewriteURN := func(oldURN resource.URN) resource.URN {
			if res, hasRes := actuals[oldURN]; hasRes {
				return res.URN
			}

			return oldURN
		}

		for _, r := range ps.ResourceRegistrations {
			opts := deploytest.ResourceOptions{
				// TODO: We should sometimes leave this null
				Protect:        &r.Protect,
				RetainOnDelete: r.RetainOnDelete,
				Parent:         rewriteURN(r.Parent),
				Provider:       rewriteProviderRef(r.Provider),
				DeletedWith:    rewriteURN(r.DeletedWith),

				// We explicitly *don't* want to rewrite aliases since they are not dependencies and refer to (we expect)
				// resources in the state, not the program we are running.
				AliasURNs: r.Aliases,
			}

			deps := make([]resource.URN, len(r.Dependencies))
			for i, dep := range r.Dependencies {
				deps[i] = rewriteURN(dep)
			}

			propDeps := map[resource.PropertyKey][]resource.URN{}
			for k, deps := range r.PropertyDependencies {
				propDeps[k] = make([]resource.URN, len(deps))
				for i, dep := range deps {
					propDeps[k][i] = rewriteURN(dep)
				}
			}

			opts.Dependencies = deps
			opts.PropertyDeps = propDeps

			res, err := monitor.RegisterResource(r.Type, r.Name, r.Custom, opts)
			require.NoError(t, err)

			actuals[r.URN()] = res
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
			rendered += fmt.Sprintf("\n%s    %s", indent, r.Pretty(indent+"    "))
		}
	}

	if len(ps.Drops) == 0 {
		rendered += fmt.Sprintf("\n%s  No drops", indent)
	} else {
		rendered += fmt.Sprintf("\n%s  Drops (%d):", indent, len(ps.Drops))
		for _, r := range ps.Drops {
			rendered += fmt.Sprintf("\n%s    %s", indent, r.Pretty(indent+"    "))
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
	AddAliases           *rapid.Generator[bool]
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
	if overrides.AddAliases != nil {
		pso.AddAliases = overrides.AddAliases
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
	AddAliases:           rapid.Bool(),
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

		// Whenever we copy a resource from the snapshot, we need to ensure that its dependencies have been updated to take
		// deletions/drops into account. The dropped and rewritten maps and the updateDependencies function take care of
		// this.
		//
		// While in many cases updating is just a case of removing references to dropped resources, we may also need to
		// *rewrite* references whenever parent/child relationships change. This is because a resource's URN changes
		// whenever its parent changes.
		//
		// When a URN changes due to parent/child changes, we'll consult the AddAliases generator to decide whether to drop
		// the old URN, or add an alias to the new resource pointing to it. Based on this, subsequent resources that refer
		// to the old URN will either have their references pruned or updated to the new URN.

		dropped := map[resource.URN]bool{}
		rewritten := map[resource.URN]resource.URN{}

		updateDependencies := func(r *ResourceSpec) {
			deps := []resource.URN{}
			propDeps := map[resource.PropertyKey][]resource.URN{}

			// We'll start with parents first. If our parent was dropped, we'll need to remove them from our parent reference.
			// If they were rewritten, we'll update the reference to point to the new URN.
			oldURN := r.URN()
			if dropped[r.Parent] {
				r.Parent = ""
			} else if newParent, hasNewParent := rewritten[r.Parent]; hasNewParent {
				r.Parent = newParent
			}

			// If our URN changed (e.g. because our parent changed), we'll consult the AddAliases generator to decide whether
			// to drop the old URN or add an alias to the new resource pointing to it.
			if oldURN != r.URN() {
				shouldAlias := pso.AddAliases.Draw(t, "ProgramSpec.PrunedResource.AddAliases")
				if shouldAlias {
					r.Aliases = []resource.URN{oldURN}
					rewritten[oldURN] = r.URN()
				} else {
					dropped[oldURN] = true
				}
			}

			// Updating dependencies, property dependencies and deleted-with relationships is simpler, since these don't
			// affect our URN.
			for _, dep := range r.Dependencies {
				if dropped[dep] {
					continue
				} else if newDep, hasNewDep := rewritten[dep]; hasNewDep {
					deps = append(deps, newDep)
				} else {
					deps = append(deps, dep)
				}
			}

			for k, deps := range r.PropertyDependencies {
				for _, dep := range deps {
					if dropped[dep] {
						continue
					} else if newDep, hasNewDep := rewritten[dep]; hasNewDep {
						propDeps[k] = append(propDeps[k], newDep)
					} else {
						propDeps[k] = append(propDeps[k], dep)
					}
				}
			}

			if dropped[r.DeletedWith] {
				r.DeletedWith = ""
			} else if newDep, hasNewDep := rewritten[r.DeletedWith]; hasNewDep {
				r.DeletedWith = newDep
			}

			r.Dependencies = deps
			r.PropertyDependencies = propDeps
		}

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
			if ss.Resources[i].Delete {
				i++
				continue
			}

			action := pso.Action.Draw(t, fmt.Sprintf("ProgramSpec.Action[%d]", i))
			if action == ProgramSpecDelete {
				r := ss.Resources[i].Copy()
				if providers.IsProviderType(r.Type) {
					AddTag(r, CopiedProgramResource)
					updateDependencies(r)

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

				oldURN := r.URN()
				rds.ApplyTo(r)
				if oldURN != r.URN() {
					shouldAlias := pso.AddAliases.Draw(t, fmt.Sprintf("ProgramSpec.UpdatedResource[%d].AddAliases", i))
					if shouldAlias {
						r.Aliases = []resource.URN{oldURN}
						rewritten[oldURN] = r.URN()
					} else {
						dropped[oldURN] = true
					}
				}

				newSS.AddResource(r)
				i++
			} else {
				r := ss.Resources[i].Copy()
				AddTag(r, CopiedProgramResource)
				updateDependencies(r)

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
