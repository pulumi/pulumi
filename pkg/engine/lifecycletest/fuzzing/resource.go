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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"golang.org/x/exp/maps"
	"pgregory.net/rapid"
)

type ResourceSpec struct {
	Stack                tokens.QName
	Project              tokens.PackageName
	Type                 tokens.Type
	Name                 string
	ID                   resource.ID
	Custom               bool
	Delete               bool
	Protect              bool
	PendingReplacement   bool
	RetainOnDelete       bool
	Provider             string
	Dependencies         []resource.URN
	PropertyDependencies map[resource.PropertyKey][]resource.URN
	DeletedWith          resource.URN
	Tags                 map[string]bool
}

func AddTag[T ~string](r *ResourceSpec, tag T) {
	if tag == "" {
		return
	}

	if r.Tags == nil {
		r.Tags = map[string]bool{}
	}

	r.Tags[string(tag)] = true
}

func (r *ResourceSpec) URN() resource.URN {
	return resource.NewURN(r.Stack, r.Project, "", r.Type, r.Name)
}

func (r *ResourceSpec) Copy() *ResourceSpec {
	deps := copystructure.Must(copystructure.Copy(r.Dependencies)).([]resource.URN)
	propDeps := copystructure.Must(copystructure.Copy(r.PropertyDependencies)).(map[resource.PropertyKey][]resource.URN)
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
		Provider:             r.Provider,
		Dependencies:         deps,
		PropertyDependencies: propDeps,
		DeletedWith:          r.DeletedWith,
		Tags:                 tags,
	}
}

func (r *ResourceSpec) AsResource() *resource.State {
	deps := copystructure.Must(copystructure.Copy(r.Dependencies)).([]resource.URN)
	propDeps := copystructure.Must(copystructure.Copy(r.PropertyDependencies)).(map[resource.PropertyKey][]resource.URN)

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
		Dependencies:         deps,
		PropertyDependencies: propDeps,
		DeletedWith:          r.DeletedWith,
		SourcePosition:       strings.Join(tags, ", "),
	}

	if !providers.IsProviderType(r.Type) {
		s.Inputs = resource.PropertyMap{
			"__id": resource.NewStringProperty(r.ID.String()),
		}
	}

	return s
}

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

	rendered := fmt.Sprintf(`%[1]s%[2]s [%[3]s%[4]s%[5]s]%[6]s
%[1]s  Protect:             %[7]v
%[1]s  Pending replacement: %[8]v
%[1]s  Retain on delete:    %[9]v%[10]s%[11]s%[12]s%[13]s`,
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
		deps,
		propDeps,
		deletedWith,
	)

	return rendered
}

func GeneratedResourceType(pkg tokens.Package) *rapid.Generator[tokens.Type] {
	return rapid.Custom(func(t *rapid.T) tokens.Type {
		mod := rapid.StringMatching("^mod-[a-z][A-Za-z0-9]{3}$").Draw(t, "ResourceType.Module")
		typ := rapid.StringMatching("^type-[a-z][A-Za-z0-9]{3}$").Draw(t, "ResourceType.Type")
		return tokens.Type(fmt.Sprintf("%s:%s:%s", pkg, mod, typ))
	})
}

var GeneratedProviderType = rapid.Custom(func(t *rapid.T) tokens.Type {
	pkg := rapid.StringMatching("^pkg-[a-z][A-Za-z0-9]{3}$").Draw(t, "ProviderType.Package")
	return tokens.Type("pulumi:providers:%s" + pkg)
})

var GeneratedResourceName = rapid.Custom(func(t *rapid.T) string {
	name := rapid.StringMatching("^res-[a-z][A-Za-z0-9]{11}$").Draw(t, "ResourceName")
	return name
})

var GeneratedResourceID = rapid.Custom(func(t *rapid.T) resource.ID {
	id := rapid.StringMatching("^id-[a-z][A-Za-z0-9]{11}$").Draw(t, "ResourceID")
	return resource.ID(id)
})

type ResourceSpecOptions struct {
	Custom             *rapid.Generator[bool]
	Protect            *rapid.Generator[bool]
	PendingReplacement *rapid.Generator[bool]
	RetainOnDelete     *rapid.Generator[bool]
}

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

var defaultResourceSpecOptions = ResourceSpecOptions{
	Custom:             rapid.Bool(),
	Protect:            rapid.Bool(),
	PendingReplacement: rapid.Bool(),
	RetainOnDelete:     rapid.Bool(),
}

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

			Tags: map[string]bool{},
		}

		return r
	})
}

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

			Tags: map[string]bool{},
		}

		return r
	})
}
