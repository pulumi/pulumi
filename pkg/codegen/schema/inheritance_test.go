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

package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test helpers -----------------------------------------------------------

func pStr() PropertySpec  { return PropertySpec{TypeSpec: TypeSpec{Type: "string"}} }
func pInt() PropertySpec  { return PropertySpec{TypeSpec: TypeSpec{Type: "integer"}} }
func pBool() PropertySpec { return PropertySpec{TypeSpec: TypeSpec{Type: "boolean"}} }

// selfInput builds a method input object carrying only the __self__ receiver.
func selfInput(resourceToken string) *ObjectTypeSpec {
	return &ObjectTypeSpec{
		Properties: map[string]PropertySpec{
			"__self__": {TypeSpec: TypeSpec{Ref: "#/resources/" + resourceToken}},
		},
	}
}

// objectOut builds a method output object with the given properties.
func objectOut(props map[string]PropertySpec) *ObjectTypeSpec {
	return &ObjectTypeSpec{Type: "object", Properties: props}
}

func bindOptions() ValidationOptions {
	return ValidationOptions{AllowDanglingReferences: true}
}

func findResource(pkg *Package, token string) *Resource {
	for _, r := range pkg.Resources {
		if r.Token == token {
			return r
		}
	}
	return nil
}

func propNames(props []*Property) []string {
	names := make([]string, len(props))
	for i, p := range props {
		names[i] = p.Name
	}
	return names
}

// bindOK binds a spec expecting no errors and returns the package.
func bindOK(t *testing.T, spec PackageSpec, loader Loader) *Package {
	t.Helper()
	pkg, diags, err := BindSpec(spec, loader, bindOptions())
	require.NoError(t, err)
	require.Falsef(t, diags.HasErrors(), "unexpected errors: %s", diags.Error())
	return pkg
}

// bindErr binds a spec expecting an error diagnostic containing want.
func bindErr(t *testing.T, spec PackageSpec, loader Loader, want string) {
	t.Helper()
	_, diags, err := BindSpec(spec, loader, bindOptions())
	require.NoError(t, err)
	require.Truef(t, diags.HasErrors(), "expected an error containing %q, got none", want)
	assert.Containsf(t, diags.Error(), want, "diagnostics: %s", diags.Error())
}

// --- Same-package extends ---------------------------------------------------

func TestExtendsSamePackage(t *testing.T) {
	t.Parallel()

	// Two components, base + derived, with the derived token sorting *before* the base token to exercise the
	// order-independence of same-package extends resolution (finishResources sorts by token, so the derived is
	// flattened first and must recurse into its later-sorting base).
	spec := PackageSpec{
		Name:             "test",
		Version:          "1.0.0",
		RequiredFeatures: []string{featureInheritance},
		Resources: map[string]ResourceSpec{
			"test:index:Zeta": {
				IsComponent:     true,
				ObjectTypeSpec:  ObjectTypeSpec{Properties: map[string]PropertySpec{"host": pStr()}},
				InputProperties: map[string]PropertySpec{"host": pStr()},
			},
			"test:index:Alpha": {
				IsComponent:     true,
				Extends:         &TypeSpec{Ref: "#/resources/test:index:Zeta"},
				ObjectTypeSpec:  ObjectTypeSpec{Properties: map[string]PropertySpec{"replicas": pInt()}},
				InputProperties: map[string]PropertySpec{"replicas": pInt()},
			},
		},
	}

	pkg := bindOK(t, spec, noOpLoader{})
	derived := findResource(pkg, "test:index:Alpha")
	require.NotNil(t, derived)
	base := findResource(pkg, "test:index:Zeta")
	require.NotNil(t, base)

	require.Same(t, base, derived.BaseResource)
	// The flattened bound model carries both members.
	assert.ElementsMatch(t, []string{"host", "replicas"}, propNames(derived.Properties))
	assert.ElementsMatch(t, []string{"host", "replicas"}, propNames(derived.InputProperties))
	// Own vs. inherited partition the flattened set.
	assert.ElementsMatch(t, []string{"replicas"}, propNames(derived.OwnProperties()))
	assert.ElementsMatch(t, []string{"host"}, propNames(derived.InheritedProperties()))
	assert.ElementsMatch(t, []string{"replicas"}, propNames(derived.OwnInputProperties()))
	assert.ElementsMatch(t, []string{"host"}, propNames(derived.InheritedInputProperties()))

	// The base has no base of its own.
	assert.Nil(t, base.BaseResource)
	assert.ElementsMatch(t, []string{"host"}, propNames(base.OwnProperties()))
	assert.Empty(t, base.InheritedProperties())
}

func TestExtendsMultiLevelSamePackage(t *testing.T) {
	t.Parallel()

	spec := PackageSpec{
		Name:             "test",
		Version:          "1.0.0",
		RequiredFeatures: []string{featureInheritance},
		Resources: map[string]ResourceSpec{
			"test:index:L0": {
				IsComponent:     true,
				ObjectTypeSpec:  ObjectTypeSpec{Properties: map[string]PropertySpec{"a": pStr()}},
				InputProperties: map[string]PropertySpec{"a": pStr()},
			},
			"test:index:L1": {
				IsComponent:     true,
				Extends:         &TypeSpec{Ref: "#/resources/test:index:L0"},
				ObjectTypeSpec:  ObjectTypeSpec{Properties: map[string]PropertySpec{"b": pStr()}},
				InputProperties: map[string]PropertySpec{"b": pStr()},
			},
			"test:index:L2": {
				IsComponent:     true,
				Extends:         &TypeSpec{Ref: "#/resources/test:index:L1"},
				ObjectTypeSpec:  ObjectTypeSpec{Properties: map[string]PropertySpec{"c": pStr()}},
				InputProperties: map[string]PropertySpec{"c": pStr()},
			},
		},
	}

	pkg := bindOK(t, spec, noOpLoader{})
	l2 := findResource(pkg, "test:index:L2")
	require.NotNil(t, l2)
	// The full chain is flattened all the way to the root.
	assert.ElementsMatch(t, []string{"a", "b", "c"}, propNames(l2.Properties))
	assert.ElementsMatch(t, []string{"c"}, propNames(l2.OwnProperties()))
	// InheritedProperties reflects only the immediate base's flattened set (a + b).
	assert.ElementsMatch(t, []string{"a", "b"}, propNames(l2.InheritedProperties()))
	// BaseDescriptor points at the immediate base only.
	require.NotNil(t, l2.BaseDescriptor())
	assert.Equal(t, "test", l2.BaseDescriptor().Name)
}

// --- Cross-package extends --------------------------------------------------

// bindBasePackage binds a standalone "base" package exporting a single component with a method, and returns it plus a
// loader that resolves it for a derived package.
func bindBasePackage(t *testing.T) (*Package, Loader) {
	t.Helper()
	baseSpec := PackageSpec{
		Name:    "base",
		Version: "1.0.0",
		Resources: map[string]ResourceSpec{
			"base:index:Service": {
				IsComponent:     true,
				ObjectTypeSpec:  ObjectTypeSpec{Properties: map[string]PropertySpec{"endpoint": pStr()}},
				InputProperties: map[string]PropertySpec{"port": pInt()},
				Methods:         map[string]string{"getStatus": "base:index:Service/getStatus"},
			},
		},
		Functions: map[string]FunctionSpec{
			"base:index:Service/getStatus": {
				Inputs:  selfInput("base:index:Service"),
				Outputs: objectOut(map[string]PropertySpec{"status": pStr()}),
			},
		},
	}
	base := bindOK(t, baseSpec, noOpLoader{})
	baseRef := base.Reference()
	loader := &mockLoader{
		GetPackageF: func(_ context.Context, desc *PackageDescriptor) (PackageReference, error) {
			if desc.Name == "base" {
				return baseRef, nil
			}
			return nil, fmt.Errorf("unknown package %q", desc.Name)
		},
	}
	return base, loader
}

func TestExtendsCrossPackage(t *testing.T) {
	t.Parallel()

	_, loader := bindBasePackage(t)

	v := semver.MustParse("1.0.0")
	spec := PackageSpec{
		Name:             "derived",
		Version:          "2.0.0",
		RequiredFeatures: []string{featureInheritance},
		Dependencies:     []PackageDescriptor{{Name: "base", Version: &v, DownloadURL: "https://example.com/base"}},
		Resources: map[string]ResourceSpec{
			"derived:index:WebService": {
				IsComponent:     true,
				Extends:         &TypeSpec{Ref: "/base/v1.0.0/schema.json#/resources/base:index:Service"},
				ObjectTypeSpec:  ObjectTypeSpec{Properties: map[string]PropertySpec{"replicas": pInt()}},
				InputProperties: map[string]PropertySpec{"replicas": pInt()},
			},
		},
	}

	pkg := bindOK(t, spec, loader)
	derived := findResource(pkg, "derived:index:WebService")
	require.NotNil(t, derived)
	require.NotNil(t, derived.BaseResource)
	assert.Equal(t, "base:index:Service", derived.BaseResource.Token)

	// The base's flattened members are materialized into the derived (endpoint output, port input).
	assert.ElementsMatch(t, []string{"endpoint", "replicas"}, propNames(derived.Properties))
	assert.ElementsMatch(t, []string{"port", "replicas"}, propNames(derived.InputProperties))
	assert.ElementsMatch(t, []string{"endpoint"}, propNames(derived.InheritedProperties()))
	assert.ElementsMatch(t, []string{"port"}, propNames(derived.InheritedInputProperties()))

	// The immediate-base descriptor carries the resolved dependency's identity.
	desc := derived.BaseDescriptor()
	require.NotNil(t, desc)
	assert.Equal(t, "base", desc.Name)
	require.NotNil(t, desc.Version)
	assert.Equal(t, "1.0.0", desc.Version.String())
	assert.Equal(t, "https://example.com/base", desc.DownloadURL)

	// The inherited method surfaces through AllMethods with the base's token.
	methods := derived.AllMethods()
	require.Len(t, methods, 1)
	assert.Equal(t, "getStatus", methods[0].Name)
	assert.Equal(t, "base:index:Service/getStatus", methods[0].Function.Token)
}

func TestExtendsCrossPackageMaterializesExternalRefs(t *testing.T) {
	t.Parallel()

	// A base package whose component has a member typed as one of the base's own named types. When that member is
	// materialized into a cross-package derived, its type reference must be re-anchored to the base's external form.
	baseSpec := PackageSpec{
		Name:    "base",
		Version: "1.0.0",
		Types: map[string]ComplexTypeSpec{
			"base:index:Config": {
				ObjectTypeSpec: ObjectTypeSpec{
					Type:       "object",
					Properties: map[string]PropertySpec{"tier": pStr()},
				},
			},
		},
		Resources: map[string]ResourceSpec{
			"base:index:Service": {
				IsComponent: true,
				ObjectTypeSpec: ObjectTypeSpec{
					Properties: map[string]PropertySpec{
						"config": {TypeSpec: TypeSpec{Ref: "#/types/base:index:Config"}},
					},
				},
			},
		},
	}
	base := bindOK(t, baseSpec, noOpLoader{})
	baseRef := base.Reference()
	loader := &mockLoader{
		GetPackageF: func(_ context.Context, desc *PackageDescriptor) (PackageReference, error) {
			if desc.Name == "base" {
				return baseRef, nil
			}
			return nil, fmt.Errorf("unknown package %q", desc.Name)
		},
	}

	v := semver.MustParse("1.0.0")
	derivedSpec := PackageSpec{
		Name:             "derived",
		Version:          "2.0.0",
		RequiredFeatures: []string{featureInheritance},
		Dependencies:     []PackageDescriptor{{Name: "base", Version: &v}},
		Resources: map[string]ResourceSpec{
			"derived:index:WebService": {
				IsComponent: true,
				Extends:     &TypeSpec{Ref: "/base/v1.0.0/schema.json#/resources/base:index:Service"},
			},
		},
	}

	pkg := bindOK(t, derivedSpec, loader)
	marshaled, err := pkg.MarshalSpec()
	require.NoError(t, err)

	ws := marshaled.Resources["derived:index:WebService"]
	require.Contains(t, ws.Properties, "config")
	// The materialized member's type ref is re-anchored to the base package's external form.
	assert.Equal(t, "/base/v1.0.0/schema.json#/types/base:index:Config", ws.Properties["config"].Ref)
	// The extends ref is likewise external.
	require.NotNil(t, ws.Extends)
	assert.Equal(t, "/base/v1.0.0/schema.json#/resources/base:index:Service", ws.Extends.Ref)
}

func TestExtendsCrossPackageUnresolvableIsError(t *testing.T) {
	t.Parallel()

	// A loader that fails to resolve the base. Even under AllowDanglingReferences a dangling base is an error.
	loader := &mockLoader{
		GetPackageF: func(_ context.Context, desc *PackageDescriptor) (PackageReference, error) {
			return nil, fmt.Errorf("no such package %q", desc.Name)
		},
	}
	spec := PackageSpec{
		Name:    "derived",
		Version: "1.0.0",
		Resources: map[string]ResourceSpec{
			"derived:index:WebService": {
				IsComponent: true,
				Extends:     &TypeSpec{Ref: "/base/v1.0.0/schema.json#/resources/base:index:Service"},
			},
		},
	}
	bindErr(t, spec, loader, "resolving base package base")
}

// --- Invalid extends targets ------------------------------------------------

func TestExtendsInvalidTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		resources map[string]ResourceSpec
		want      string
	}{
		{
			name: "extends-custom-resource",
			resources: map[string]ResourceSpec{
				"test:index:Custom": {
					ObjectTypeSpec: ObjectTypeSpec{Properties: map[string]PropertySpec{"x": pStr()}},
				},
				"test:index:Comp": {
					IsComponent: true,
					Extends:     &TypeSpec{Ref: "#/resources/test:index:Custom"},
				},
			},
			want: "cannot extend test:index:Custom because test:index:Custom is not a component",
		},
		{
			name: "extends-provider",
			resources: map[string]ResourceSpec{
				"test:index:Comp": {
					IsComponent: true,
					Extends:     &TypeSpec{Ref: "#/provider"},
				},
			},
			want: "extends must reference a resource",
		},
		{
			name: "extends-nonexistent",
			resources: map[string]ResourceSpec{
				"test:index:Comp": {
					IsComponent: true,
					Extends:     &TypeSpec{Ref: "#/resources/test:index:Missing"},
				},
			},
			want: "base resource test:index:Missing not found",
		},
		{
			name: "non-component-extender",
			resources: map[string]ResourceSpec{
				"test:index:Base": {
					IsComponent:    true,
					ObjectTypeSpec: ObjectTypeSpec{Properties: map[string]PropertySpec{"x": pStr()}},
				},
				"test:index:CustomExtender": {
					// Not a component, so it may not extend.
					Extends: &TypeSpec{Ref: "#/resources/test:index:Base"},
				},
			},
			want: "only components may extend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			bindErr(t, PackageSpec{Name: "test", Version: "1.0.0", Resources: tt.resources}, noOpLoader{}, tt.want)
		})
	}
}

// --- Cycles -----------------------------------------------------------------

func TestExtendsCycles(t *testing.T) {
	t.Parallel()

	comp := func(extends string) ResourceSpec {
		return ResourceSpec{IsComponent: true, Extends: &TypeSpec{Ref: "#/resources/" + extends}}
	}

	tests := []struct {
		name      string
		resources map[string]ResourceSpec
	}{
		{
			name: "self",
			resources: map[string]ResourceSpec{
				"test:index:A": comp("test:index:A"),
			},
		},
		{
			name: "two-cycle",
			resources: map[string]ResourceSpec{
				"test:index:A": comp("test:index:B"),
				"test:index:B": comp("test:index:A"),
			},
		},
		{
			name: "three-cycle",
			resources: map[string]ResourceSpec{
				"test:index:A": comp("test:index:B"),
				"test:index:B": comp("test:index:C"),
				"test:index:C": comp("test:index:A"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			bindErr(t, PackageSpec{Name: "test", Version: "1.0.0", Resources: tt.resources}, noOpLoader{},
				"extends cycle detected")
		})
	}
}

// --- Abstract ---------------------------------------------------------------

func TestAbstractNonComponentErrors(t *testing.T) {
	t.Parallel()

	spec := PackageSpec{
		Name:    "test",
		Version: "1.0.0",
		Resources: map[string]ResourceSpec{
			"test:index:Custom": {
				Abstract:       true,
				ObjectTypeSpec: ObjectTypeSpec{Properties: map[string]PropertySpec{"x": pStr()}},
			},
		},
	}
	bindErr(t, spec, noOpLoader{}, "only components may be declared abstract")
}

func TestAbstractBaseConcreteDerived(t *testing.T) {
	t.Parallel()

	spec := PackageSpec{
		Name:             "test",
		Version:          "1.0.0",
		RequiredFeatures: []string{featureInheritance},
		Resources: map[string]ResourceSpec{
			"test:index:Base": {
				IsComponent:     true,
				Abstract:        true,
				ObjectTypeSpec:  ObjectTypeSpec{Properties: map[string]PropertySpec{"host": pStr()}},
				InputProperties: map[string]PropertySpec{"host": pStr()},
			},
			"test:index:Derived": {
				IsComponent:     true,
				Extends:         &TypeSpec{Ref: "#/resources/test:index:Base"},
				ObjectTypeSpec:  ObjectTypeSpec{Properties: map[string]PropertySpec{"replicas": pInt()}},
				InputProperties: map[string]PropertySpec{"replicas": pInt()},
			},
		},
	}

	pkg := bindOK(t, spec, noOpLoader{})
	base := findResource(pkg, "test:index:Base")
	derived := findResource(pkg, "test:index:Derived")
	require.NotNil(t, base)
	require.NotNil(t, derived)
	assert.True(t, base.Abstract)
	assert.False(t, derived.Abstract)
	require.Same(t, base, derived.BaseResource)
	assert.ElementsMatch(t, []string{"host", "replicas"}, propNames(derived.Properties))
}

// --- Flattened-view consistency ---------------------------------------------

func TestFlattenedViewMatchesChain(t *testing.T) {
	t.Parallel()

	// The base is fixed; each case varies how the derived (re)declares inherited members.
	base := ResourceSpec{
		IsComponent:     true,
		ObjectTypeSpec:  ObjectTypeSpec{Properties: map[string]PropertySpec{"endpoint": pStr()}},
		InputProperties: map[string]PropertySpec{"port": pInt()},
		RequiredInputs:  []string{"port"},
	}

	pkgWith := func(derived ResourceSpec, requiredFeatures []string) PackageSpec {
		return PackageSpec{
			Name:             "test",
			Version:          "1.0.0",
			RequiredFeatures: requiredFeatures,
			Resources: map[string]ResourceSpec{
				"test:index:Base":    base,
				"test:index:Derived": derived,
			},
		}
	}

	t.Run("exact-match", func(t *testing.T) {
		t.Parallel()
		// The derived re-lists every inherited member exactly (fully flattened) plus its own.
		derived := ResourceSpec{
			IsComponent: true,
			Extends:     &TypeSpec{Ref: "#/resources/test:index:Base"},
			ObjectTypeSpec: ObjectTypeSpec{
				Properties: map[string]PropertySpec{"endpoint": pStr(), "replicas": pInt()},
			},
			InputProperties: map[string]PropertySpec{"port": pInt(), "replicas": pInt()},
			RequiredInputs:  []string{"port"},
		}
		pkg := bindOK(t, pkgWith(derived, nil), noOpLoader{})
		d := findResource(pkg, "test:index:Derived")
		assert.ElementsMatch(t, []string{"endpoint", "replicas"}, propNames(d.Properties))
		assert.ElementsMatch(t, []string{"port", "replicas"}, propNames(d.InputProperties))
	})

	t.Run("mismatched-type", func(t *testing.T) {
		t.Parallel()
		// The derived re-lists inherited "endpoint" with a different type.
		derived := ResourceSpec{
			IsComponent: true,
			Extends:     &TypeSpec{Ref: "#/resources/test:index:Base"},
			ObjectTypeSpec: ObjectTypeSpec{
				Properties: map[string]PropertySpec{"endpoint": pInt()},
			},
		}
		bindErr(t, pkgWith(derived, nil), noOpLoader{},
			`inherited property "endpoint" does not match base test:index:Base`)
	})

	t.Run("extra-member-claiming-inheritance", func(t *testing.T) {
		t.Parallel()
		// The derived re-lists inherited "endpoint" but flips its secret flag: a copy that no longer matches the base.
		endpoint := pStr()
		endpoint.Secret = true
		derived := ResourceSpec{
			IsComponent: true,
			Extends:     &TypeSpec{Ref: "#/resources/test:index:Base"},
			ObjectTypeSpec: ObjectTypeSpec{
				Properties: map[string]PropertySpec{"endpoint": endpoint},
			},
		}
		bindErr(t, pkgWith(derived, nil), noOpLoader{},
			`inherited property "endpoint" does not match base test:index:Base`)
	})

	t.Run("missing-member-materialization", func(t *testing.T) {
		t.Parallel()
		// The derived omits every inherited member (sparse) and declares the feature; the binder materializes them.
		derived := ResourceSpec{
			IsComponent: true,
			Extends:     &TypeSpec{Ref: "#/resources/test:index:Base"},
			ObjectTypeSpec: ObjectTypeSpec{
				Properties: map[string]PropertySpec{"replicas": pInt()},
			},
			InputProperties: map[string]PropertySpec{"replicas": pInt()},
		}
		pkg := bindOK(t, pkgWith(derived, []string{featureInheritance}), noOpLoader{})
		d := findResource(pkg, "test:index:Derived")
		assert.ElementsMatch(t, []string{"endpoint", "replicas"}, propNames(d.Properties))
		assert.ElementsMatch(t, []string{"port", "replicas"}, propNames(d.InputProperties))
		// The materialized "port" retains its required-ness from the base.
		for _, p := range d.InputProperties {
			if p.Name == "port" {
				assert.True(t, p.IsRequired())
			}
		}
	})
}

// --- requiredFeatures -------------------------------------------------------

func TestRequiredFeaturesUnknownIsError(t *testing.T) {
	t.Parallel()
	spec := PackageSpec{
		Name:             "test",
		Version:          "1.0.0",
		RequiredFeatures: []string{"quantum-entanglement"},
		Resources: map[string]ResourceSpec{
			"test:index:R": {ObjectTypeSpec: ObjectTypeSpec{Properties: map[string]PropertySpec{"x": pStr()}}},
		},
	}
	bindErr(t, spec, noOpLoader{},
		`this schema requires feature "quantum-entanglement" which this version of Pulumi does not support`)
}

func TestRequiredFeaturesSparseWithoutDeclarationIsError(t *testing.T) {
	t.Parallel()
	// Sparse derived (omits inherited "host") without declaring the inheritance feature.
	spec := PackageSpec{
		Name:    "test",
		Version: "1.0.0",
		Resources: map[string]ResourceSpec{
			"test:index:Base": {
				IsComponent:    true,
				ObjectTypeSpec: ObjectTypeSpec{Properties: map[string]PropertySpec{"host": pStr()}},
			},
			"test:index:Derived": {
				IsComponent:    true,
				Extends:        &TypeSpec{Ref: "#/resources/test:index:Base"},
				ObjectTypeSpec: ObjectTypeSpec{Properties: map[string]PropertySpec{"replicas": pInt()}},
			},
		},
	}
	bindErr(t, spec, noOpLoader{}, "must declare requiredFeatures")
}

func TestRequiredFeaturesDroppedOnMarshalWhenFlattened(t *testing.T) {
	t.Parallel()
	// A fully flattened schema may still declare the inheritance feature (harmless); on marshal it is dropped because
	// the bound model is already flattened.
	spec := PackageSpec{
		Name:             "test",
		Version:          "1.0.0",
		RequiredFeatures: []string{featureInheritance},
		Resources: map[string]ResourceSpec{
			"test:index:Base": {
				IsComponent:    true,
				ObjectTypeSpec: ObjectTypeSpec{Properties: map[string]PropertySpec{"host": pStr()}},
			},
			"test:index:Derived": {
				IsComponent: true,
				Extends:     &TypeSpec{Ref: "#/resources/test:index:Base"},
				// Fully flattened: re-lists the inherited member.
				ObjectTypeSpec: ObjectTypeSpec{Properties: map[string]PropertySpec{"host": pStr(), "replicas": pInt()}},
			},
		},
	}
	pkg := bindOK(t, spec, noOpLoader{})
	marshaled, err := pkg.MarshalSpec()
	require.NoError(t, err)
	assert.Empty(t, marshaled.RequiredFeatures)
}

// --- Method overrides -------------------------------------------------------

func TestMethodOverrideValid(t *testing.T) {
	t.Parallel()

	// Base declares getStatus + getMetrics; derived overrides getStatus with an exact-signature, derived-owned token.
	spec := PackageSpec{
		Name:    "test",
		Version: "1.0.0",
		Resources: map[string]ResourceSpec{
			"test:index:Base": {
				IsComponent:    true,
				ObjectTypeSpec: ObjectTypeSpec{Properties: map[string]PropertySpec{"host": pStr()}},
				Methods: map[string]string{
					"getStatus":  "test:index:Base/getStatus",
					"getMetrics": "test:index:Base/getMetrics",
				},
			},
			"test:index:Derived": {
				IsComponent:    true,
				Extends:        &TypeSpec{Ref: "#/resources/test:index:Base"},
				ObjectTypeSpec: ObjectTypeSpec{Properties: map[string]PropertySpec{"host": pStr()}},
				Methods:        map[string]string{"getStatus": "test:index:Derived/getStatus"},
			},
		},
		Functions: map[string]FunctionSpec{
			"test:index:Base/getStatus": {
				Inputs:  selfInput("test:index:Base"),
				Outputs: objectOut(map[string]PropertySpec{"status": pStr()}),
			},
			"test:index:Base/getMetrics": {
				Inputs:  selfInput("test:index:Base"),
				Outputs: objectOut(map[string]PropertySpec{"series": pStr()}),
			},
			"test:index:Derived/getStatus": {
				Inputs:  selfInput("test:index:Derived"),
				Outputs: objectOut(map[string]PropertySpec{"status": pStr()}),
			},
		},
	}

	pkg := bindOK(t, spec, noOpLoader{})
	derived := findResource(pkg, "test:index:Derived")
	require.NotNil(t, derived)

	methods := derived.AllMethods()
	byName := map[string]*Method{}
	for _, m := range methods {
		byName[m.Name] = m
	}
	require.Contains(t, byName, "getStatus")
	require.Contains(t, byName, "getMetrics")
	// Nearest-level-wins: the derived's override shadows the base's getStatus.
	assert.Equal(t, "test:index:Derived/getStatus", byName["getStatus"].Function.Token)
	// The non-overridden method dispatches on the base token.
	assert.Equal(t, "test:index:Base/getMetrics", byName["getMetrics"].Function.Token)
}

func TestMethodOverrideSignatureMismatch(t *testing.T) {
	t.Parallel()

	spec := PackageSpec{
		Name:    "test",
		Version: "1.0.0",
		Resources: map[string]ResourceSpec{
			"test:index:Base": {
				IsComponent:    true,
				ObjectTypeSpec: ObjectTypeSpec{Properties: map[string]PropertySpec{"host": pStr()}},
				Methods:        map[string]string{"getStatus": "test:index:Base/getStatus"},
			},
			"test:index:Derived": {
				IsComponent:    true,
				Extends:        &TypeSpec{Ref: "#/resources/test:index:Base"},
				ObjectTypeSpec: ObjectTypeSpec{Properties: map[string]PropertySpec{"host": pStr()}},
				Methods:        map[string]string{"getStatus": "test:index:Derived/getStatus"},
			},
		},
		Functions: map[string]FunctionSpec{
			"test:index:Base/getStatus": {
				Inputs:  selfInput("test:index:Base"),
				Outputs: objectOut(map[string]PropertySpec{"status": pStr()}),
			},
			"test:index:Derived/getStatus": {
				// Different output shape: an override may not change the signature.
				Inputs:  selfInput("test:index:Derived"),
				Outputs: objectOut(map[string]PropertySpec{"status": pInt()}),
			},
		},
	}
	bindErr(t, spec, noOpLoader{}, `override of method "getStatus" changes its signature`)
}

// --- Cross-level collisions -------------------------------------------------

func TestPropertyMethodCollisions(t *testing.T) {
	t.Parallel()

	t.Run("own-method-collides-with-inherited-property", func(t *testing.T) {
		t.Parallel()
		spec := PackageSpec{
			Name:    "test",
			Version: "1.0.0",
			Resources: map[string]ResourceSpec{
				"test:index:Base": {
					IsComponent:    true,
					ObjectTypeSpec: ObjectTypeSpec{Properties: map[string]PropertySpec{"status": pStr()}},
				},
				"test:index:Derived": {
					IsComponent: true,
					Extends:     &TypeSpec{Ref: "#/resources/test:index:Base"},
					Methods:     map[string]string{"status": "test:index:Derived/status"},
				},
			},
			Functions: map[string]FunctionSpec{
				"test:index:Derived/status": {
					Inputs:  selfInput("test:index:Derived"),
					Outputs: objectOut(map[string]PropertySpec{"result": pStr()}),
				},
			},
		}
		bindErr(t, spec, noOpLoader{}, `method "status" collides with a property inherited from test:index:Base`)
	})

	t.Run("own-property-collides-with-inherited-method", func(t *testing.T) {
		t.Parallel()
		spec := PackageSpec{
			Name:    "test",
			Version: "1.0.0",
			Resources: map[string]ResourceSpec{
				"test:index:Base": {
					IsComponent:    true,
					ObjectTypeSpec: ObjectTypeSpec{Properties: map[string]PropertySpec{"host": pStr()}},
					Methods:        map[string]string{"getStatus": "test:index:Base/getStatus"},
				},
				"test:index:Derived": {
					IsComponent:    true,
					Extends:        &TypeSpec{Ref: "#/resources/test:index:Base"},
					ObjectTypeSpec: ObjectTypeSpec{Properties: map[string]PropertySpec{"getStatus": pStr()}},
				},
			},
			Functions: map[string]FunctionSpec{
				"test:index:Base/getStatus": {
					Inputs:  selfInput("test:index:Base"),
					Outputs: objectOut(map[string]PropertySpec{"status": pStr()}),
				},
			},
		}
		bindErr(t, spec, noOpLoader{}, `property "getStatus" collides with a method inherited from test:index:Base`)
	})
}

// --- Round-trip / marshal ---------------------------------------------------

func TestInheritanceMarshalPreservesExtendsAndFlattens(t *testing.T) {
	t.Parallel()

	// A sparse derived: on marshal it must become flattened (all members) and preserve the extends ref.
	spec := PackageSpec{
		Name:             "test",
		Version:          "1.0.0",
		RequiredFeatures: []string{featureInheritance},
		Resources: map[string]ResourceSpec{
			"test:index:Base": {
				IsComponent:     true,
				ObjectTypeSpec:  ObjectTypeSpec{Properties: map[string]PropertySpec{"host": pStr()}},
				InputProperties: map[string]PropertySpec{"host": pStr()},
			},
			"test:index:Derived": {
				IsComponent:     true,
				Extends:         &TypeSpec{Ref: "#/resources/test:index:Base"},
				ObjectTypeSpec:  ObjectTypeSpec{Properties: map[string]PropertySpec{"replicas": pInt()}},
				InputProperties: map[string]PropertySpec{"replicas": pInt()},
			},
		},
	}

	pkg := bindOK(t, spec, noOpLoader{})
	marshaled, err := pkg.MarshalSpec()
	require.NoError(t, err)

	derivedSpec := marshaled.Resources["test:index:Derived"]
	// extends preserved.
	require.NotNil(t, derivedSpec.Extends)
	assert.Equal(t, "#/resources/test:index:Base", derivedSpec.Extends.Ref)
	// Flattened: the inherited member is now present in the marshaled spec.
	assert.Contains(t, derivedSpec.Properties, "host")
	assert.Contains(t, derivedSpec.Properties, "replicas")
	assert.Contains(t, derivedSpec.InputProperties, "host")
	assert.Contains(t, derivedSpec.InputProperties, "replicas")
	// requiredFeatures dropped because the marshaled form is flattened.
	assert.Empty(t, marshaled.RequiredFeatures)
}

func TestInheritanceMarshalAbstract(t *testing.T) {
	t.Parallel()
	spec := PackageSpec{
		Name:    "test",
		Version: "1.0.0",
		Resources: map[string]ResourceSpec{
			"test:index:Base": {
				IsComponent:    true,
				Abstract:       true,
				ObjectTypeSpec: ObjectTypeSpec{Properties: map[string]PropertySpec{"host": pStr()}},
			},
		},
	}
	pkg := bindOK(t, spec, noOpLoader{})
	marshaled, err := pkg.MarshalSpec()
	require.NoError(t, err)
	assert.True(t, marshaled.Resources["test:index:Base"].Abstract)
}

func TestInheritanceRoundtripFixpoint(t *testing.T) {
	t.Parallel()

	// A three-level same-package chain, sparse at each level, round-trips to a stable flattened fixpoint.
	spec := PackageSpec{
		Name:             "test",
		Version:          "1.0.0",
		RequiredFeatures: []string{featureInheritance},
		Resources: map[string]ResourceSpec{
			"test:index:L0": {
				IsComponent:     true,
				ObjectTypeSpec:  ObjectTypeSpec{Properties: map[string]PropertySpec{"a": pStr()}},
				InputProperties: map[string]PropertySpec{"a": pStr()},
			},
			"test:index:L1": {
				IsComponent:     true,
				Extends:         &TypeSpec{Ref: "#/resources/test:index:L0"},
				ObjectTypeSpec:  ObjectTypeSpec{Properties: map[string]PropertySpec{"b": pBool()}},
				InputProperties: map[string]PropertySpec{"b": pBool()},
			},
			"test:index:L2": {
				IsComponent:     true,
				Extends:         &TypeSpec{Ref: "#/resources/test:index:L1"},
				ObjectTypeSpec:  ObjectTypeSpec{Properties: map[string]PropertySpec{"c": pInt()}},
				InputProperties: map[string]PropertySpec{"c": pInt()},
			},
		},
	}

	pkg := bindOK(t, spec, noOpLoader{})
	first, err := pkg.MarshalSpec()
	require.NoError(t, err)

	// Re-binding the flattened marshal must succeed without the inheritance feature (it is no longer sparse) and
	// marshal to a byte-identical spec.
	pkg2 := bindOK(t, *first, noOpLoader{})
	second, err := pkg2.MarshalSpec()
	require.NoError(t, err)

	firstJSON, err := json.Marshal(first)
	require.NoError(t, err)
	secondJSON, err := json.Marshal(second)
	require.NoError(t, err)
	assert.JSONEq(t, string(firstJSON), string(secondJSON))

	// The re-marshaled derived is fully flattened.
	l2 := second.Resources["test:index:L2"]
	assert.ElementsMatch(t, []string{"a", "b", "c"}, keys(l2.Properties))
	require.NotNil(t, l2.Extends)
	assert.Equal(t, "#/resources/test:index:L1", l2.Extends.Ref)
}

func keys(m map[string]PropertySpec) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

// --- Bind-path uniformity (partial packages) --------------------------------

// loadPartial imports a spec as a PartialPackage — the lazy, bind-on-demand form the plugin loader and LoaderClient
// return — by round-tripping it through JSON exactly as those loaders do.
func loadPartial(t *testing.T, spec PackageSpec, loader Loader) *PartialPackage {
	t.Helper()
	raw, err := json.Marshal(spec)
	require.NoError(t, err)
	var partial PartialPackageSpec
	require.NoError(t, json.Unmarshal(raw, &partial))
	p, err := ImportPartialSpec(partial, nil, loader)
	require.NoError(t, err)
	return p
}

// inheritanceSpec is a base component carrying a method plus a derived component that extends it. The derived re-lists
// the base's own property (flattened form, as a published schema would) and adds its own.
func inheritanceSpec() PackageSpec {
	return PackageSpec{
		Name:    "test",
		Version: "1.0.0",
		Resources: map[string]ResourceSpec{
			"test:index:Base": {
				IsComponent:    true,
				ObjectTypeSpec: ObjectTypeSpec{Properties: map[string]PropertySpec{"host": pStr()}},
				Methods:        map[string]string{"getStatus": "test:index:Base/getStatus"},
			},
			"test:index:Derived": {
				IsComponent: true,
				Extends:     &TypeSpec{Ref: "#/resources/test:index:Base"},
				ObjectTypeSpec: ObjectTypeSpec{
					Properties: map[string]PropertySpec{"host": pStr(), "replicas": pInt()},
				},
			},
		},
		Functions: map[string]FunctionSpec{
			"test:index:Base/getStatus": {
				Inputs:  selfInput("test:index:Base"),
				Outputs: objectOut(map[string]PropertySpec{"status": pStr()}),
			},
		},
	}
}

func assertDerivedResolved(t *testing.T, derived *Resource) {
	t.Helper()
	require.NotNil(t, derived)
	// Regression: before inheritance resolution was made uniform across bind paths, the lazily-loaded derived came
	// back with BaseResource == nil and no inherited methods.
	require.NotNil(t, derived.BaseResource, "BaseResource must be wired")
	assert.Equal(t, "test:index:Base", derived.BaseResource.Token)
	assert.ElementsMatch(t, []string{"host"}, propNames(derived.InheritedProperties()))
	assert.ElementsMatch(t, []string{"replicas"}, propNames(derived.OwnProperties()))

	methods := derived.AllMethods()
	require.Len(t, methods, 1, "AllMethods must include the inherited method")
	assert.Equal(t, "getStatus", methods[0].Name)
	assert.Equal(t, "test:index:Base/getStatus", methods[0].Function.Token)
}

func TestPartialPackageResolvesInheritanceOnGet(t *testing.T) {
	t.Parallel()
	// The lazy path: Resources().Get binds a single resource on demand. It must still resolve that resource's extends.
	p := loadPartial(t, inheritanceSpec(), noOpLoader{})
	derived, ok, err := p.Resources().Get("test:index:Derived")
	require.NoError(t, err)
	require.True(t, ok)
	assertDerivedResolved(t, derived)
}

func TestPartialPackageResolvesInheritanceViaGetType(t *testing.T) {
	t.Parallel()
	// GetType is the accessor codegen's external-ref path uses; it must resolve inheritance too.
	p := loadPartial(t, inheritanceSpec(), noOpLoader{})
	typ, ok, err := p.Resources().GetType("test:index:Derived")
	require.NoError(t, err)
	require.True(t, ok)
	assertDerivedResolved(t, typ.Resource)
}

func TestPartialPackageResolvesInheritanceOnDefinition(t *testing.T) {
	t.Parallel()
	// The full-materialization path on a partial package: Definition() finishes binding every member.
	p := loadPartial(t, inheritanceSpec(), noOpLoader{})
	def, err := p.Definition()
	require.NoError(t, err)
	assertDerivedResolved(t, findResource(def, "test:index:Derived"))
}

func TestPartialPackageResolvesCrossPackageInheritance(t *testing.T) {
	t.Parallel()

	// A base package with a component + method, resolved cross-package through the loader — the plugin-loader shape.
	_, baseLoader := bindBasePackage(t)

	v := semver.MustParse("1.0.0")
	derivedSpec := PackageSpec{
		Name:         "derived",
		Version:      "2.0.0",
		Dependencies: []PackageDescriptor{{Name: "base", Version: &v}},
		Resources: map[string]ResourceSpec{
			"derived:index:WebService": {
				IsComponent: true,
				Extends:     &TypeSpec{Ref: "/base/v1.0.0/schema.json#/resources/base:index:Service"},
				ObjectTypeSpec: ObjectTypeSpec{
					Properties: map[string]PropertySpec{"endpoint": pStr(), "replicas": pInt()},
				},
			},
		},
	}

	p := loadPartial(t, derivedSpec, baseLoader)
	derived, ok, err := p.Resources().Get("derived:index:WebService")
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, derived.BaseResource)
	assert.Equal(t, "base:index:Service", derived.BaseResource.Token)
	// The inherited method dispatches on the base package's token.
	methods := derived.AllMethods()
	require.Len(t, methods, 1)
	assert.Equal(t, "base:index:Service/getStatus", methods[0].Function.Token)
	// BaseDescriptor carries the cross-package identity generated stubs need.
	require.NotNil(t, derived.BaseDescriptor())
	assert.Equal(t, "base", derived.BaseDescriptor().Name)
}
