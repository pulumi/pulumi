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
	"context"
	"fmt"
	"slices"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"golang.org/x/exp/maps"
	"pgregory.net/rapid"
)

// A ProviderSpec specifies the behavior of a set of providers that will be mocked in a lifecycle test.
type ProviderSpec struct {
	Packages map[tokens.Package]bool
	Create   ProviderCreateSpec
	Delete   ProviderDeleteSpec
	Diff     ProviderDiffSpec
	Read     ProviderReadSpec
	Update   ProviderUpdateSpec
}

// Implements PrettySpec.Pretty. Returns a human-readable representation of this ProviderSpec, suitable for use in
// debugging output and error messages.
func (ps *ProviderSpec) Pretty(indent string) string {
	rendered := fmt.Sprintf("%sProvider %p", indent, ps)
	if len(ps.Packages) == 0 {
		rendered += fmt.Sprintf("\n%s  No packages", indent)
	} else {
		rendered += fmt.Sprintf("\n%s  Packages (%d):", indent, len(ps.Packages))

		pkgs := maps.Keys(ps.Packages)
		slices.Sort(pkgs)

		for _, p := range pkgs {
			rendered += fmt.Sprintf("\n%s    %s", indent, p)
		}
	}

	renderedCreate := ps.Create.Pretty(indent + "  ")
	renderedDelete := ps.Delete.Pretty(indent + "  ")
	renderedDiff := ps.Diff.Pretty(indent + "  ")
	renderedRead := ps.Read.Pretty(indent + "  ")
	renderedUpdate := ps.Update.Pretty(indent + "  ")

	hasAny := len(renderedCreate) > 0 ||
		len(renderedDelete) > 0 ||
		len(renderedDiff) > 0 ||
		len(renderedRead) > 0 ||
		len(renderedUpdate) > 0

	if !hasAny {
		rendered += fmt.Sprintf("\n%s  No modified operations", indent)
	} else {
		rendered += renderedCreate
		rendered += renderedDelete
		rendered += renderedDiff
		rendered += renderedRead
		rendered += renderedUpdate
	}

	return rendered
}

// Adds the given package to the set of packages that this ProviderSpec will mock.
func (ps *ProviderSpec) AddPackage(pkg tokens.Package) {
	if pkg == "" {
		return
	}

	if ps.Packages == nil {
		ps.Packages = map[tokens.Package]bool{}
	}

	ps.Packages[pkg] = true
}

// Returns a deploytest.ProviderLoader representation of this ProviderSpec, suitable for use in a lifecycle test.
func (ps *ProviderSpec) AsProviderLoaders() []*deploytest.ProviderLoader {
	version := semver.MustParse("1.0.0")
	load := func() (plugin.Provider, error) {
		return &deploytest.Provider{
			CreateF: ps.Create.AsCreateF(),
			DeleteF: ps.Delete.AsDeleteF(),
			DiffF:   ps.Diff.AsDiffF(),
			ReadF:   ps.Read.AsReadF(),
			UpdateF: ps.Update.AsUpdateF(),
		}, nil
	}

	loaders := make([]*deploytest.ProviderLoader, len(ps.Packages))

	pkgs := maps.Keys(ps.Packages)
	slices.Sort(pkgs)
	for i, pkg := range pkgs {
		loaders[i] = deploytest.NewProviderLoader(pkg, version, load)
	}

	return loaders
}

// A ProviderCreateSpec specifies the behavior of a provider's create function. It maps resource URNs to the action that
// should be taken if Create is called on that URN. The absence of a URN in the map indicates that the default behavior
// (a successful create) should be taken.
type ProviderCreateSpec map[resource.URN]ProviderCreateSpecAction

// ProviderCreateSpecAction captures the set of actions that can be taken by a Create implementation for a given
// resource.
type ProviderCreateSpecAction string

const (
  // Fail the Create operation.
	ProviderCreateFailure ProviderCreateSpecAction = "provider.create-failure"
)

// Implements PrettySpec.Pretty. Returns a human-readable string representation of this ProviderCreateSpec, suitable for
// use in debugging output or error messages.
func (pcs ProviderCreateSpec) Pretty(indent string) string {
	if len(pcs) == 0 {
		return ""
	}

	rendered := fmt.Sprintf("\n%sCreate:", indent)
	for r, action := range pcs {
		switch action {
		case ProviderCreateFailure:
			rendered += fmt.Sprintf("\n%s  !  %s", indent, Colored(r))
		}
	}

	return rendered
}

// Returns a CreateF-compatible callback that implements this ProviderCreateSpec.
func (pcs ProviderCreateSpec) AsCreateF() func(context.Context, plugin.CreateRequest) (plugin.CreateResponse, error) {
	return func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
		if action, ok := pcs[req.URN]; ok {
			switch action {
			case ProviderCreateFailure:
				return plugin.CreateResponse{
					Status: resource.StatusUnknown,
				}, fmt.Errorf("create failure for %s", req.URN)
			}
		}

    // To avoid having to randomly generate IDs here, we allow resources to specify an __id input that we'll use as the
    // ID we return. ResourceSpec.AsResource makes use of this, for instance.
		id := req.Properties["__id"].String()
		return plugin.CreateResponse{
			ID:         resource.ID(id),
			Properties: req.Properties,
			Status:     resource.StatusOK,
		}, nil
	}
}

type ProviderDeleteSpec map[resource.URN]ProviderDeleteSpecAction

type ProviderDeleteSpecAction string

const (
	ProviderDeleteFailure ProviderDeleteSpecAction = "provider.delete-failure"
)

func (pds ProviderDeleteSpec) Pretty(indent string) string {
	if len(pds) == 0 {
		return ""
	}

	rendered := fmt.Sprintf("\n%sDelete:", indent)
	for r, action := range pds {
		switch action {
		case ProviderDeleteFailure:
			rendered += fmt.Sprintf("\n%s  !  %s\n", indent, Colored(r))
		}
	}

	return rendered
}

func (pds ProviderDeleteSpec) AsDeleteF() func(context.Context, plugin.DeleteRequest) (plugin.DeleteResponse, error) {
	return func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
		if action, ok := pds[req.URN]; ok {
			switch action {
			case ProviderDeleteFailure:
				return plugin.DeleteResponse{
					Status: resource.StatusUnknown,
				}, fmt.Errorf("delete failure for %s", req.URN)
			}
		}

		return plugin.DeleteResponse{
			Status: resource.StatusOK,
		}, nil
	}
}

type ProviderDiffSpec map[resource.URN]ProviderDiffSpecAction

type ProviderDiffSpecAction string

const (
	ProviderDiffDeleteBeforeReplace ProviderDiffSpecAction = "provider.diff-delete-before-replace"
	ProviderDiffDeleteAfterReplace  ProviderDiffSpecAction = "provider.diff-delete-after-replace"
	ProviderDiffChange              ProviderDiffSpecAction = "provider.diff-change"
	ProviderDiffFailure             ProviderDiffSpecAction = "provider.diff-failure"
)

func (pds ProviderDiffSpec) Pretty(indent string) string {
	if len(pds) == 0 {
		return ""
	}

	rendered := fmt.Sprintf("\n%sDiff:", indent)
	for r, action := range pds {
		switch action {
		case ProviderDiffDeleteBeforeReplace:
			rendered += fmt.Sprintf("\n%s  -+ %s", indent, Colored(r))
		case ProviderDiffDeleteAfterReplace:
			rendered += fmt.Sprintf("\n%s  +- %s", indent, Colored(r))
		case ProviderDiffChange:
			rendered += fmt.Sprintf("\n%s  ~  %s", indent, Colored(r))
		case ProviderDiffFailure:
			rendered += fmt.Sprintf("\n%s  !  %s", indent, Colored(r))
		}
	}

	return rendered
}

func (pds ProviderDiffSpec) AsDiffF() func(context.Context, plugin.DiffRequest) (plugin.DiffResponse, error) {
	return func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
		if action, ok := pds[req.URN]; ok {
			switch action {
			case ProviderDiffDeleteBeforeReplace:
				return plugin.DiffResponse{
					Changes:             plugin.DiffSome,
					ReplaceKeys:         []resource.PropertyKey{"__replace"},
					DeleteBeforeReplace: true,
				}, nil
			case ProviderDiffDeleteAfterReplace:
				return plugin.DiffResponse{
					Changes:             plugin.DiffSome,
					ReplaceKeys:         []resource.PropertyKey{"__replace"},
					DeleteBeforeReplace: false,
				}, nil
			case ProviderDiffChange:
				return plugin.DiffResponse{Changes: plugin.DiffSome}, nil
			case ProviderDiffFailure:
				return plugin.DiffResponse{}, fmt.Errorf("diff failure for %s", req.URN)
			}
		}

		return plugin.DiffResponse{}, nil
	}
}

type ProviderReadSpec map[resource.URN]ProviderReadSpecAction

type ProviderReadSpecAction string

const (
	ProviderReadDeleted ProviderReadSpecAction = "provider.read-deleted"
	ProviderReadFailure ProviderReadSpecAction = "provider.read-failure"
)

func (prs ProviderReadSpec) Pretty(indent string) string {
	if len(prs) == 0 {
		return ""
	}

	rendered := fmt.Sprintf("\n%sRead:", indent)
	for r, action := range prs {
		switch action {
		case ProviderReadDeleted:
			rendered += fmt.Sprintf("\n%s  -  %s", indent, Colored(r))
		case ProviderReadFailure:
			rendered += fmt.Sprintf("\n%s  !  %s", indent, Colored(r))
		}
	}

	return rendered
}

func (prs ProviderReadSpec) AsReadF() func(context.Context, plugin.ReadRequest) (plugin.ReadResponse, error) {
	return func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
		if action, ok := prs[req.URN]; ok {
			switch action {
			case ProviderReadDeleted:
				return plugin.ReadResponse{}, nil
			case ProviderReadFailure:
				return plugin.ReadResponse{Status: resource.StatusPartialFailure}, fmt.Errorf("read failure for %s", req.URN)
			}
		}

		return plugin.ReadResponse{
			ReadResult: plugin.ReadResult{
				Outputs: resource.PropertyMap{},
			},
			Status: resource.StatusOK,
		}, nil
	}
}

type ProviderUpdateSpec map[resource.URN]ProviderUpdateSpecAction

type ProviderUpdateSpecAction string

const (
	ProviderUpdateFailure ProviderUpdateSpecAction = "provider.update-failure"
)

func (pus ProviderUpdateSpec) Pretty(indent string) string {
	if len(pus) == 0 {
		return ""
	}

	rendered := fmt.Sprintf("\n%sUpdate:", indent)
	for r, action := range pus {
		switch action {
		case ProviderUpdateFailure:
			rendered += fmt.Sprintf("\n%s  !  %s", indent, Colored(r))
		}
	}

	return rendered
}

func (pus ProviderUpdateSpec) AsUpdateF() func(context.Context, plugin.UpdateRequest) (plugin.UpdateResponse, error) {
	return func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
		if action, ok := pus[req.URN]; ok {
			switch action {
			case ProviderUpdateFailure:
				return plugin.UpdateResponse{Status: resource.StatusUnknown}, fmt.Errorf("update failure for %s", req.URN)
			}
		}

		return plugin.UpdateResponse{
			Properties: req.NewInputs,
			Status:     resource.StatusOK,
		}, nil
	}
}

type ProviderSpecOptions struct {
	CreateAction *rapid.Generator[ProviderCreateSpecAction]
	DeleteAction *rapid.Generator[ProviderDeleteSpecAction]
	DiffAction   *rapid.Generator[ProviderDiffSpecAction]
	ReadAction   *rapid.Generator[ProviderReadSpecAction]
	UpdateAction *rapid.Generator[ProviderUpdateSpecAction]
}

func (pso ProviderSpecOptions) With(overrides ProviderSpecOptions) ProviderSpecOptions {
	if overrides.CreateAction != nil {
		pso.CreateAction = overrides.CreateAction
	}
	if overrides.DeleteAction != nil {
		pso.DeleteAction = overrides.DeleteAction
	}
	if overrides.DiffAction != nil {
		pso.DiffAction = overrides.DiffAction
	}
	if overrides.ReadAction != nil {
		pso.ReadAction = overrides.ReadAction
	}
	if overrides.UpdateAction != nil {
		pso.UpdateAction = overrides.UpdateAction
	}

	return pso
}

var defaultProviderSpecOptions = ProviderSpecOptions{
	CreateAction: rapid.SampledFrom(providerCreateSpecActions),
	DeleteAction: rapid.SampledFrom(providerDeleteSpecActions),
	DiffAction:   rapid.SampledFrom(providerDiffSpecActions),
	ReadAction:   rapid.SampledFrom(providerReadSpecActions),
	UpdateAction: rapid.SampledFrom(providerUpdateSpecActions),
}

var providerCreateSpecActions = []ProviderCreateSpecAction{
	"",
	ProviderCreateFailure,
}

var providerDeleteSpecActions = []ProviderDeleteSpecAction{
	"",
	ProviderDeleteFailure,
}

var providerDiffSpecActions = []ProviderDiffSpecAction{
	"",
	ProviderDiffDeleteBeforeReplace,
	ProviderDiffDeleteAfterReplace,
	ProviderDiffChange,
	ProviderDiffFailure,
}

var providerReadSpecActions = []ProviderReadSpecAction{
	"",
	ProviderReadDeleted,
	ProviderReadFailure,
}

var providerUpdateSpecActions = []ProviderUpdateSpecAction{
	"",
	ProviderUpdateFailure,
}

func GeneratedProviderSpec(progSpec *ProgramSpec, pso ProviderSpecOptions) *rapid.Generator[*ProviderSpec] {
	pso = defaultProviderSpecOptions.With(pso)

	return rapid.Custom(func(t *rapid.T) *ProviderSpec {
		provSpec := &ProviderSpec{
			Packages: map[tokens.Package]bool{},
			Create:   map[resource.URN]ProviderCreateSpecAction{},
			Delete:   map[resource.URN]ProviderDeleteSpecAction{},
			Diff:     map[resource.URN]ProviderDiffSpecAction{},
			Read:     map[resource.URN]ProviderReadSpecAction{},
			Update:   map[resource.URN]ProviderUpdateSpecAction{},
		}

		if len(progSpec.ResourceRegistrations) == 0 && len(progSpec.Drops) == 0 {
			// TODO EXPLAIN that we must draw at least one bit of entropy from Rapid
			return rapid.Just(provSpec).Draw(t, "ProviderSpec.Empty")
		}

		allResources := append(progSpec.ResourceRegistrations, progSpec.Drops...)
		for _, r := range allResources {
			if providers.IsProviderType(r.Type) {
				provSpec.AddPackage(tokens.Package(r.Type.Name()))
			} else {
				provSpec.AddPackage(r.Type.Package())
			}

			updateGeneratedProviderCreateSpec(t, provSpec, pso, r)
			updateGeneratedProviderDeleteSpec(t, provSpec, pso, r)
			updateGeneratedProviderDiffSpec(t, provSpec, pso, r)
			updateGeneratedProviderReadSpec(t, provSpec, pso, r)
			updateGeneratedProviderUpdateSpec(t, provSpec, pso, r)
		}

		return provSpec
	})
}

func updateGeneratedProviderCreateSpec(t *rapid.T, ps *ProviderSpec, pso ProviderSpecOptions, r *ResourceSpec) {
	action := pso.CreateAction.Draw(t, "ProviderCreateSpec.Action")
	if action == "" {
		return
	}

	ps.Create[r.URN()] = action
	AddTag(r, action)
}

func updateGeneratedProviderDeleteSpec(t *rapid.T, ps *ProviderSpec, pso ProviderSpecOptions, r *ResourceSpec) {
	action := pso.DeleteAction.Draw(t, "ProviderDeleteSpec.Action")
	if action == "" {
		return
	}

	ps.Delete[r.URN()] = action
	AddTag(r, action)
}

func updateGeneratedProviderDiffSpec(t *rapid.T, ps *ProviderSpec, pso ProviderSpecOptions, r *ResourceSpec) {
	action := pso.DiffAction.Draw(t, "ProviderDiffSpec.Action")
	if action == "" {
		return
	}

	ps.Diff[r.URN()] = action
	AddTag(r, action)
}

func updateGeneratedProviderReadSpec(t *rapid.T, ps *ProviderSpec, pso ProviderSpecOptions, r *ResourceSpec) {
	action := pso.ReadAction.Draw(t, "ProviderReadSpec.Action")
	if action == "" {
		return
	}

	ps.Read[r.URN()] = action
	AddTag(r, action)
}

func updateGeneratedProviderUpdateSpec(t *rapid.T, ps *ProviderSpec, pso ProviderSpecOptions, r *ResourceSpec) {
	action := pso.UpdateAction.Draw(t, "ProviderUpdateSpec.Action")
	if action == "" {
		return
	}

	ps.Update[r.URN()] = action
	AddTag(r, action)
}
