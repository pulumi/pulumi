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
	"slices"
	"strings"

	"github.com/mitchellh/copystructure"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"golang.org/x/exp/maps"
	"pgregory.net/rapid"
)

// A ResourceSpec specifies the subset of a resource's state that is relevant to fuzzing snapshot integrity issues.
// Generally this encompasses enough to identify a resource (URN, ID, and so on) and any dependencies it may have on
// others.
type ResourceSpec struct {
	Project              tokens.PackageName
	Stack                tokens.QName
	Type                 tokens.Type
	Name                 string
	ID                   resource.ID
	Custom               bool
	Delete               bool
	Protect              bool
	PendingReplacement   bool
	RetainOnDelete       bool
	Provider             string
	Parent               resource.URN
	Dependencies         []resource.URN
	PropertyDependencies map[resource.PropertyKey][]resource.URN
	DeletedWith          resource.URN
	Aliases              []resource.URN

	// A set of tags associated with the resource. These have no bearing on any tests but are included to aid in debugging
	// and identifying the causes of snapshot integrity issues.
	Tags map[string]bool
}

// Creates a ResourceSpec from the given ResourceV3.
func FromResourceV3(r apitype.ResourceV3) *ResourceSpec {
	deps := copystructure.Must(copystructure.Copy(r.Dependencies)).([]resource.URN)
	propDeps := copystructure.Must(copystructure.Copy(r.PropertyDependencies)).(map[resource.PropertyKey][]resource.URN)
	aliases := copystructure.Must(copystructure.Copy(r.Aliases)).([]resource.URN)

	return &ResourceSpec{
		Project:              r.URN.Project(),
		Stack:                r.URN.Stack(),
		Type:                 r.Type,
		Name:                 r.URN.Name(),
		Custom:               r.Custom,
		Delete:               r.Delete,
		ID:                   r.ID,
		Protect:              r.Protect,
		PendingReplacement:   r.PendingReplacement,
		RetainOnDelete:       r.RetainOnDelete,
		Provider:             r.Provider,
		Parent:               r.Parent,
		Dependencies:         deps,
		PropertyDependencies: propDeps,
		DeletedWith:          r.DeletedWith,
		Aliases:              aliases,
		Tags:                 map[string]bool{},
	}
}

// AddTag adds the given tag to the given ResourceSpec. Ideally this would be a generic method on ResourceSpec itself,
// but Go doesn't support generic methods yet.
func AddTag[T ~string](r *ResourceSpec, tag T) {
	if tag == "" {
		return
	}

	if r.Tags == nil {
		r.Tags = map[string]bool{}
	}

	r.Tags[string(tag)] = true
}

// URN returns the URN of this ResourceSpec.
func (r *ResourceSpec) URN() resource.URN {
	var parentType tokens.Type
	if r.Parent != "" {
		parentType = r.Parent.QualifiedType()
	}

	return resource.NewURN(r.Stack, r.Project, parentType, r.Type, r.Name)
}

// Copy returns a deep copy of this ResourceSpec.
func (r *ResourceSpec) Copy() *ResourceSpec {
	deps := copystructure.Must(copystructure.Copy(r.Dependencies)).([]resource.URN)
	propDeps := copystructure.Must(copystructure.Copy(r.PropertyDependencies)).(map[resource.PropertyKey][]resource.URN)
	aliases := copystructure.Must(copystructure.Copy(r.Aliases)).([]resource.URN)
	tags := copystructure.Must(copystructure.Copy(r.Tags)).(map[string]bool)

	return &ResourceSpec{
		Project:              r.Project,
		Stack:                r.Stack,
		Type:                 r.Type,
		Name:                 r.Name,
		Custom:               r.Custom,
		Delete:               r.Delete,
		ID:                   r.ID,
		Protect:              r.Protect,
		PendingReplacement:   r.PendingReplacement,
		RetainOnDelete:       r.RetainOnDelete,
		Parent:               r.Parent,
		Provider:             r.Provider,
		Dependencies:         deps,
		PropertyDependencies: propDeps,
		DeletedWith:          r.DeletedWith,
		Aliases:              aliases,
		Tags:                 tags,
	}
}

// Returns a resource.State representation of this ResourceSpec, suitable for inclusion in e.g. a snapshot.
func (r *ResourceSpec) AsResource() *resource.State {
	deps := copystructure.Must(copystructure.Copy(r.Dependencies)).([]resource.URN)
	propDeps := copystructure.Must(copystructure.Copy(r.PropertyDependencies)).(map[resource.PropertyKey][]resource.URN)
	aliases := copystructure.Must(copystructure.Copy(r.Aliases)).([]resource.URN)

	tags := maps.Keys(r.Tags)
	slices.Sort(tags)

	s := &resource.State{
		Type:                 r.Type,
		URN:                  r.URN(),
		Custom:               r.Custom,
		Delete:               r.Delete,
		ID:                   r.ID,
		Protect:              r.Protect,
		PendingReplacement:   r.PendingReplacement,
		RetainOnDelete:       r.RetainOnDelete,
		Provider:             r.Provider,
		Parent:               r.Parent,
		Dependencies:         deps,
		PropertyDependencies: propDeps,
		DeletedWith:          r.DeletedWith,
		Aliases:              aliases,
		SourcePosition:       strings.Join(tags, ", "),
	}

	// In order to allow us to control generated resource IDs (e.g. such as those returned by a provider Create call),
	// we'll set the ResourceSpec's ID field as an input property.
	if !providers.IsProviderType(r.Type) {
		s.Inputs = resource.PropertyMap{
			"__id": resource.NewStringProperty(r.ID.String()),
		}
	}

	return s
}

// Implements PrettySpec.Pretty. Returns a human-readable representation of this ResourceSpec, suitable for use in
// debugging output and error messages.
//
// <urn> [provider, custom]
//
//	Tags:                program.updated, snapshot.provider.initial
//
//	Protect:             false
//	Pending replacement: false
//	Retain on delete:    false
//
//	Dependencies (1):
//	  <urn>
func (r *ResourceSpec) Pretty(indent string) string {
	var providerHint string
	if providers.IsProviderType(r.Type) {
		providerHint = "provider, "
	}

	var customOrComponent string
	if r.Custom {
		customOrComponent = "custom"
	} else {
		customOrComponent = "component"
	}

	var deleted string
	if r.Delete {
		deleted = ", deleted"
	}

	var tags string
	if len(r.Tags) > 0 {
		ks := maps.Keys(r.Tags)
		slices.Sort(ks)
		tags = fmt.Sprintf("\n%s  Tags:                %s\n", indent, strings.Join(ks, ", "))
	}

	var provider string
	if r.Provider != "" {
		provRef, err := providers.ParseReference(r.Provider)
		if err != nil {
			provider = fmt.Sprintf("\n%s  Provider:            %s", indent, r.Provider)
		} else {
			provider = fmt.Sprintf("\n%s  Provider:            %s::%s", indent, Colored(provRef.URN()), provRef.ID())
		}
	}

	var parent string
	if r.Parent != "" {
		parent = fmt.Sprintf("\n%s  Parent:              %s", indent, Colored(r.Parent))
	}

	var deps string
	if len(r.Dependencies) > 0 {
		deps = fmt.Sprintf("\n\n%s  Dependencies (%d):", indent, len(r.Dependencies))
		for _, d := range r.Dependencies {
			deps += fmt.Sprintf("\n%s    %s", indent, Colored(d))
		}
	}

	var propDeps string
	if len(r.PropertyDependencies) > 0 {
		propDeps = fmt.Sprintf("\n\n%s  Property dependencies (%d key[s]):", indent, len(r.PropertyDependencies))
		for k, deps := range r.PropertyDependencies {
			propDeps += fmt.Sprintf("\n%s    %s", indent, k)
			for _, d := range deps {
				propDeps += fmt.Sprintf("\n%s      %s", indent, Colored(d))
			}
		}
	}

	var deletedWith string
	if r.DeletedWith != "" {
		deletedWith = fmt.Sprintf("\n\n%s  Deleted with:        %s", indent, Colored(r.DeletedWith))
	}

	var aliases string
	if len(r.Aliases) > 0 {
		aliases = fmt.Sprintf("\n\n%s  Aliases (%d):", indent, len(r.Aliases))
		for _, a := range r.Aliases {
			aliases += fmt.Sprintf("\n%s    %s", indent, Colored(a))
		}
	}

	rendered := fmt.Sprintf(`%[1]s%[2]s [%[3]s%[4]s%[5]s]%[6]s
%[1]s  Protect:             %[7]v
%[1]s  Pending replacement: %[8]v
%[1]s  Retain on delete:    %[9]v%[10]s%[11]s%[12]s%[13]s%[14]s%[15]s`,
		indent,

		Colored(r.URN()),
		providerHint,
		customOrComponent,
		deleted,

		tags,

		r.Protect,
		r.PendingReplacement,
		r.RetainOnDelete,

		provider,
		parent,
		deps,
		propDeps,
		deletedWith,
		aliases,
	)

	return rendered
}

// Given a package name, returns a rapid.Generator that yields random resource types within that package.
//
//	GeneratedResourceType("pkg-xyz").Draw(t, "ResourceType") = "pkg-xyz:<mod>:<type>"
func GeneratedResourceType(pkg tokens.Package) *rapid.Generator[tokens.Type] {
	return rapid.Custom(func(t *rapid.T) tokens.Type {
		mod := rapid.StringMatching("^mod-[a-z][A-Za-z0-9]{3}$").Draw(t, "ResourceType.Module")
		typ := rapid.StringMatching("^type-[a-z][A-Za-z0-9]{3}$").Draw(t, "ResourceType.Type")
		return tokens.Type(fmt.Sprintf("%s:%s:%s", pkg, mod, typ))
	})
}

// A rapid.Generator that yields random provider types.
//
//	GeneratedProviderType.Draw(t, "ProviderType") = "pulumi:providers:<pkg>"
var GeneratedProviderType = rapid.Custom(func(t *rapid.T) tokens.Type {
	pkg := rapid.StringMatching("^pkg-[a-z][A-Za-z0-9]{3}$").Draw(t, "ProviderType.Package")
	return tokens.Type("pulumi:providers:" + pkg)
})

// A rapid.Generator that yields random resource names.
//
//	GeneratedResourceName.Draw(t, "ResourceName") = "res-<random>"
var GeneratedResourceName = rapid.Custom(func(t *rapid.T) string {
	name := rapid.StringMatching("^res-[a-z][A-Za-z0-9]{3}$").Draw(t, "ResourceName")
	return name
})

// A rapid.Generator that yields random resource IDs.
//
//	GeneratedResourceID.Draw(t, "ResourceID") = "id-<random>"
var GeneratedResourceID = rapid.Custom(func(t *rapid.T) resource.ID {
	id := rapid.StringMatching("^id-[a-z][A-Za-z0-9]{11}$").Draw(t, "ResourceID")
	return resource.ID(id)
})

// A set of options for configuring the generation of a ResourceSpec.
type ResourceSpecOptions struct {
	Custom             *rapid.Generator[bool]
	Protect            *rapid.Generator[bool]
	PendingReplacement *rapid.Generator[bool]
	RetainOnDelete     *rapid.Generator[bool]
}

// Returns a copy of the given ResourceSpecOptions with the given overrides applied.
func (rso ResourceSpecOptions) With(overrides ResourceSpecOptions) ResourceSpecOptions {
	if overrides.Custom != nil {
		rso.Custom = overrides.Custom
	}
	if overrides.Protect != nil {
		rso.Protect = overrides.Protect
	}
	if overrides.PendingReplacement != nil {
		rso.PendingReplacement = overrides.PendingReplacement
	}
	if overrides.RetainOnDelete != nil {
		rso.RetainOnDelete = overrides.RetainOnDelete
	}

	return rso
}

// A default set of ResourceSpecOptions. By default, all configurations are equally likely.
var defaultResourceSpecOptions = ResourceSpecOptions{
	Custom:             rapid.Bool(),
	Protect:            rapid.Bool(),
	PendingReplacement: rapid.Bool(),
	RetainOnDelete:     rapid.Bool(),
}

// Given a set of StackSpecOptions, returns a rapid.Generator that yields random provider ResourceSpecs with no
// dependencies. Provider resources are always custom and never deleted.
func GeneratedProviderResourceSpec(
	sso StackSpecOptions,
) *rapid.Generator[*ResourceSpec] {
	sso = defaultStackSpecOptions.With(sso)

	return rapid.Custom(func(t *rapid.T) *ResourceSpec {
		typ := GeneratedProviderType.Draw(t, "ProviderResourceSpec.Type")
		name := GeneratedResourceName.Draw(t, "ProviderResourceSpec.Name")
		id := GeneratedResourceID.Draw(t, "ProviderResourceSpec.ID")

		r := &ResourceSpec{
			Project: tokens.PackageName(sso.Project),
			Stack:   tokens.QName(sso.Stack),
			Type:    typ,
			Name:    name,
			ID:      id,

			Custom:             true,
			Delete:             false,
			Protect:            false,
			PendingReplacement: false,
			RetainOnDelete:     false,

			Dependencies:         []resource.URN{},
			PropertyDependencies: map[resource.PropertyKey][]resource.URN{},

			Aliases: []resource.URN{},

			Tags: map[string]bool{},
		}

		return r
	})
}

// Given a set of StackSpecOptions, ResourceSpecOptions, and a map of package names to provider resources, returns a
// rapid.Generator that yields random ResourceSpecs with no dependencies.
func GeneratedResourceSpec(
	sso StackSpecOptions,
	rso ResourceSpecOptions,
	provs map[tokens.Package]*ResourceSpec,
) *rapid.Generator[*ResourceSpec] {
	sso = defaultStackSpecOptions.With(sso)
	rso = defaultResourceSpecOptions.With(rso)

	return rapid.Custom(func(t *rapid.T) *ResourceSpec {
		pkg := rapid.SampledFrom(maps.Keys(provs)).Draw(t, "ResourceSpec.Package")
		provider := provs[pkg]

		typ := GeneratedResourceType(pkg).Draw(t, "ResourceSpec.Type")
		name := GeneratedResourceName.Draw(t, "ResourceSpec.Name")
		id := GeneratedResourceID.Draw(t, "ResourceSpec.ID")

		r := &ResourceSpec{
			Project: tokens.PackageName(sso.Project),
			Stack:   tokens.QName(sso.Stack),
			Type:    typ,
			Name:    name,
			ID:      id,

			Custom:             rso.Custom.Draw(t, "ResourceSpec.Custom"),
			Delete:             false,
			Protect:            rso.Protect.Draw(t, "ResourceSpec.Protect"),
			PendingReplacement: rso.PendingReplacement.Draw(t, "ResourceSpec.PendingReplacement"),
			RetainOnDelete:     rso.RetainOnDelete.Draw(t, "ResourceSpec.RetainOnDelete"),

			Provider:             fmt.Sprintf("%s::%s", provider.URN(), provider.ID),
			Dependencies:         []resource.URN{},
			PropertyDependencies: map[resource.PropertyKey][]resource.URN{},

			Aliases: []resource.URN{},

			Tags: map[string]bool{},
		}

		return r
	})
}
