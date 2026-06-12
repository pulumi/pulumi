// Copyright 2026, Pulumi Corporation.
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

// Package rapidimporter provides rapid (property-based) generators for
// resource state values used by the importer round-trip test.
//
// A drawn Sample contains a [resource.State] for a randomly chosen resource
// in the supplied schema package, plus a Snapshot containing all other
// resources its envelope refers to (provider, optional satellites used as
// parent / dependency / deletedWith / replaceWith / propertyDependencies
// targets). The state's Inputs are drawn via the rapidresource generator and
// converted to [resource.PropertyMap] via [resource.ToResourcePropertyMap].
package rapidimporter

import (
	"fmt"

	"github.com/blang/semver"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils/rapidresource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Sample is a generated resource state and the snapshot needed to import it.
// Snapshot always contains the provider resource referenced by State.Provider,
// plus any satellite resources referenced by State's envelope.
type Sample struct {
	// State is the generated resource being imported.
	State *resource.State
	// Snapshot is the surrounding stack snapshot. It contains the provider
	// resource referenced by State.Provider, and any other resources
	// referenced by Parent, Dependencies, DeletedWith, ReplaceWith, or
	// PropertyDependencies. It does not contain State itself.
	Snapshot []*resource.State
}

// State returns a generator that picks a random custom, non-provider resource
// from pkg, draws schema-conforming inputs for it, and produces a Sample with
// a matching snapshot.
//
// State panics if pkg declares no eligible resource (custom and non-provider).
// Callers that draw a package from a generator should filter for eligibility
// before constructing State.
func State(pkg *schema.Package) *rapid.Generator[*Sample] {
	pickable := selectableResources(pkg)
	if len(pickable) == 0 {
		panic(fmt.Sprintf("rapidimporter.State: package %q declares no custom non-provider resources", pkg.Name))
	}
	return rapid.Custom(func(t *rapid.T) *Sample {
		return drawSample(t, pkg, pickable)
	})
}

// selectableResources keeps only the resources that drawSample knows how to
// emit a state for: custom (non-component) and non-provider.
func selectableResources(pkg *schema.Package) []*schema.Resource {
	out := make([]*schema.Resource, 0, len(pkg.Resources))
	for _, r := range pkg.Resources {
		if r.IsProvider || r.IsComponent {
			continue
		}
		out = append(out, r)
	}
	return out
}

const (
	stackName   = tokens.QName("test-stack")
	projectName = tokens.PackageName("test-project")
)

func drawSample(t *rapid.T, pkg *schema.Package, pickable []*schema.Resource) *Sample {
	r := pickable[rapid.IntRange(0, len(pickable)-1).Draw(t, "resource-index")]

	provider := drawProviderState(t, pkg)

	inputs := rapidresource.ResourceInputs(r).Draw(t, "inputs")

	typ := tokens.Type(r.Token)
	name := drawIdentifier(t, "resource-name")
	state := &resource.State{
		Type:                typ,
		URN:                 resource.NewURN(stackName, projectName, "", typ, name),
		Custom:              true,
		ID:                  drawResourceID(t, "resource-id"),
		Inputs:              resource.ToResourcePropertyMap(inputs),
		Provider:            providerRefString(provider),
		Protect:             rapid.Bool().Draw(t, "protect"),
		RetainOnDelete:      rapid.Bool().Draw(t, "retain-on-delete"),
		PendingReplacement:  rapid.Bool().Draw(t, "pending-replacement"),
		External:            rapid.Bool().Draw(t, "external"),
		RefreshBeforeUpdate: rapid.Bool().Draw(t, "refresh-before-update"),
		ImportID:            drawOptionalResourceID(t, "import-id"),
		IgnoreChanges:       drawPropertyNames(t, "ignore-changes", r.InputProperties),
		ReplaceOnChanges:    drawPropertyNames(t, "replace-on-changes", r.InputProperties),
		Aliases:             drawAliases(t, typ),
	}

	satelliteCount := rapid.IntRange(0, 3).Draw(t, "satellite-count")
	taken := map[resource.URN]bool{state.URN: true, provider.URN: true}
	satellites := make([]*resource.State, 0, satelliteCount)
	for i := 0; i < satelliteCount; i++ {
		s := drawSatellite(t, pickable, provider, taken, i)
		taken[s.URN] = true
		satellites = append(satellites, s)
	}
	snapshot := make([]*resource.State, 0, 1+len(satellites))
	snapshot = append(snapshot, provider)
	snapshot = append(snapshot, satellites...)

	if len(satellites) > 0 {
		if rapid.Bool().Draw(t, "use-parent") {
			state.Parent = pickURN(t, satellites, "parent-idx")
		}
		state.Dependencies = drawDistinctURNs(t, satellites, "dep")
		if rapid.Bool().Draw(t, "use-deleted-with") {
			state.DeletedWith = pickURN(t, satellites, "deleted-with-idx")
		}
		state.ReplaceWith = drawDistinctURNs(t, satellites, "replace-with")
		state.PropertyDependencies = drawPropertyDependencies(t, r.InputProperties, satellites)
	}

	return &Sample{State: state, Snapshot: snapshot}
}

func drawSatellite(
	t *rapid.T, pickable []*schema.Resource, provider *resource.State,
	taken map[resource.URN]bool, i int,
) *resource.State {
	sR := pickable[rapid.IntRange(0, len(pickable)-1).Draw(t, fmt.Sprintf("satellite-%d-resource-idx", i))]
	sTyp := tokens.Type(sR.Token)
	urn := rapid.Custom(func(t *rapid.T) resource.URN {
		name := drawIdentifier(t, fmt.Sprintf("satellite-%d-name", i))
		return resource.NewURN(stackName, projectName, "", sTyp, name)
	}).Filter(func(u resource.URN) bool { return !taken[u] }).
		Draw(t, fmt.Sprintf("satellite-%d-urn", i))
	return &resource.State{
		Type:     sTyp,
		URN:      urn,
		Custom:   true,
		ID:       drawResourceID(t, fmt.Sprintf("satellite-%d-id", i)),
		Provider: providerRefString(provider),
		Inputs:   resource.PropertyMap{},
	}
}

func drawProviderState(t *rapid.T, pkg *schema.Package) *resource.State {
	pkgName := tokens.Package(pkg.Name)
	typ := tokens.Type("pulumi:providers:" + string(pkgName))
	name := drawIdentifier(t, "provider-name")
	id := drawResourceID(t, "provider-id")

	inputs := resource.PropertyMap{}
	version := pkg.Version
	if version == nil {
		v := semver.MustParse("1.0.0")
		version = &v
	}
	providers.SetProviderVersion(inputs, version)

	return &resource.State{
		Type:   typ,
		URN:    resource.NewURN(stackName, projectName, "", typ, name),
		Custom: true,
		ID:     id,
		Inputs: inputs,
	}
}

func providerRefString(provider *resource.State) string {
	return string(provider.URN) + resource.URNNameDelimiter + string(provider.ID)
}

func drawIdentifier(t *rapid.T, label string) string {
	return rapid.StringMatching(`^[a-zA-Z][a-zA-Z0-9_-]{0,15}$`).Draw(t, label)
}

func drawResourceID(t *rapid.T, label string) resource.ID {
	return resource.ID(rapid.StringMatching(`^[a-zA-Z0-9_-]{1,32}$`).Draw(t, label))
}

func drawOptionalResourceID(t *rapid.T, label string) resource.ID {
	if !rapid.Bool().Draw(t, label+":present") {
		return ""
	}
	return drawResourceID(t, label)
}

func drawPropertyNames(t *rapid.T, label string, props []*schema.Property) []string {
	if len(props) == 0 {
		return nil
	}
	n := rapid.IntRange(0, 2).Draw(t, label+":n")
	if n == 0 {
		return nil
	}
	names := make([]string, n)
	for i := 0; i < n; i++ {
		idx := rapid.IntRange(0, len(props)-1).Draw(t, fmt.Sprintf("%s:%d", label, i))
		names[i] = props[idx].Name
	}
	return names
}

func drawAliases(t *rapid.T, typ tokens.Type) []resource.URN {
	n := rapid.IntRange(0, 2).Draw(t, "alias-count")
	if n == 0 {
		return nil
	}
	out := make([]resource.URN, n)
	for i := 0; i < n; i++ {
		name := drawIdentifier(t, fmt.Sprintf("alias-%d-name", i))
		out[i] = resource.NewURN(stackName, projectName, "", typ, name)
	}
	return out
}

func pickURN(t *rapid.T, satellites []*resource.State, label string) resource.URN {
	return satellites[rapid.IntRange(0, len(satellites)-1).Draw(t, label)].URN
}

func drawDistinctURNs(t *rapid.T, satellites []*resource.State, label string) []resource.URN {
	n := rapid.IntRange(0, len(satellites)).Draw(t, label+":n")
	if n == 0 {
		return nil
	}
	seen := map[resource.URN]bool{}
	out := []resource.URN{}
	for i := 0; i < n; i++ {
		u := satellites[rapid.IntRange(0, len(satellites)-1).Draw(t, fmt.Sprintf("%s:%d", label, i))].URN
		if seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func drawPropertyDependencies(
	t *rapid.T,
	props []*schema.Property,
	satellites []*resource.State,
) map[resource.PropertyKey][]resource.URN {
	if len(props) == 0 {
		return nil
	}
	count := rapid.IntRange(0, 2).Draw(t, "propdep-count")
	if count == 0 {
		return nil
	}
	out := map[resource.PropertyKey][]resource.URN{}
	for i := 0; i < count; i++ {
		pi := rapid.IntRange(0, len(props)-1).Draw(t, fmt.Sprintf("propdep-%d-prop", i))
		k := resource.PropertyKey(props[pi].Name)
		if _, has := out[k]; has {
			continue
		}
		urns := drawDistinctURNs(t, satellites, fmt.Sprintf("propdep-%d-urns", i))
		if len(urns) == 0 {
			continue
		}
		out[k] = urns
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
