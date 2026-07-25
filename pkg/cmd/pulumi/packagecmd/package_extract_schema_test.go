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

package packagecmd

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errLoader is a schema.Loader that fails any load. Same-package extends schemas resolve their bases locally and never
// reach the loader, so it also asserts that these tests stay hermetic (no external package is fetched).
type errLoader struct{ t *testing.T }

func (l errLoader) LoadPackage(pkg string, version *semver.Version) (*schema.Package, error) {
	l.t.Fatalf("unexpected LoadPackage(%s)", pkg)
	return nil, nil
}

func (l errLoader) LoadPackageV2(
	ctx context.Context, descriptor *schema.PackageDescriptor,
) (*schema.Package, error) {
	l.t.Fatalf("unexpected LoadPackageV2(%s)", descriptor.Name)
	return nil, nil
}

func TestSchemaUsesInheritance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec schema.PackageSpec
		want bool
	}{
		{
			name: "plain package",
			spec: schema.PackageSpec{Name: "test", Version: "1.0.0"},
			want: false,
		},
		{
			name: "resource with extends",
			spec: schema.PackageSpec{
				Name:    "test",
				Version: "1.0.0",
				Resources: map[string]schema.ResourceSpec{
					"test:index:Derived": {
						IsComponent: true,
						Extends:     &schema.TypeSpec{Ref: "#/resources/test:index:Base"},
					},
				},
			},
			want: true,
		},
		{
			name: "requiredFeatures inheritance marker",
			spec: schema.PackageSpec{
				Name:             "test",
				Version:          "1.0.0",
				RequiredFeatures: []string{"inheritance"},
			},
			want: true,
		},
		{
			name: "unrelated requiredFeatures",
			spec: schema.PackageSpec{
				Name:             "test",
				Version:          "1.0.0",
				RequiredFeatures: []string{"some-other-feature"},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, schemaUsesInheritance(&tt.spec))
		})
	}
}

// plainSchema is a package that does not use inheritance. Its get-schema output must be a byte-for-byte passthrough.
func plainSchema() schema.PackageSpec {
	return schema.PackageSpec{
		Name:    "test",
		Version: "1.0.0",
		Resources: map[string]schema.ResourceSpec{
			"test:index:Widget": {
				IsComponent: true,
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"name": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
				},
				InputProperties: map[string]schema.PropertySpec{
					"name": {TypeSpec: schema.TypeSpec{Type: "string"}},
				},
			},
		},
	}
}

// sparseInheritanceSchema is a same-package base + derived pair in the sparse form an analyzer emits: the derived lists
// only its own member and relies on the binder to materialize the inherited one, so it carries the requiredFeatures
// marker. get-schema must normalize this to the flattened canonical form.
func sparseInheritanceSchema() schema.PackageSpec {
	return schema.PackageSpec{
		Name:             "test",
		Version:          "1.0.0",
		RequiredFeatures: []string{"inheritance"},
		Resources: map[string]schema.ResourceSpec{
			"test:index:Base": {
				IsComponent: true,
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"host": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
				},
				InputProperties: map[string]schema.PropertySpec{
					"host": {TypeSpec: schema.TypeSpec{Type: "string"}},
				},
			},
			"test:index:Derived": {
				IsComponent: true,
				Extends:     &schema.TypeSpec{Ref: "#/resources/test:index:Base"},
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"replicas": {TypeSpec: schema.TypeSpec{Type: "integer"}},
					},
				},
				InputProperties: map[string]schema.PropertySpec{
					"replicas": {TypeSpec: schema.TypeSpec{Type: "integer"}},
				},
			},
		},
	}
}

// flattenedInheritanceSchema is a same-package base + derived pair in the flattened form the analyzers emit for a
// local base: the derived carries the full member set (its own plus the base's, copied verbatim) alongside the
// extends ref, and no requiredFeatures marker. Binding it exercises the binder's flattened-copy validation, so this
// fixture also proves the analyzers' local-case emission shape is one the binder accepts.
func flattenedInheritanceSchema() schema.PackageSpec {
	return schema.PackageSpec{
		Name:    "test",
		Version: "1.0.0",
		Resources: map[string]schema.ResourceSpec{
			"test:index:Base": {
				IsComponent: true,
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"host": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
				},
				InputProperties: map[string]schema.PropertySpec{
					"host": {TypeSpec: schema.TypeSpec{Type: "string"}},
				},
			},
			"test:index:Derived": {
				IsComponent: true,
				Extends:     &schema.TypeSpec{Ref: "#/resources/test:index:Base"},
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"host":     {TypeSpec: schema.TypeSpec{Type: "string"}},
						"replicas": {TypeSpec: schema.TypeSpec{Type: "integer"}},
					},
				},
				InputProperties: map[string]schema.PropertySpec{
					"host":     {TypeSpec: schema.TypeSpec{Type: "string"}},
					"replicas": {TypeSpec: schema.TypeSpec{Type: "integer"}},
				},
			},
		},
	}
}

// TestWriteExtractedSchemaPassthrough verifies that a schema which does not use inheritance is emitted byte-for-byte,
// with no re-binding or re-marshaling through the bound model.
func TestWriteExtractedSchemaPassthrough(t *testing.T) {
	t.Parallel()

	spec := plainSchema()
	want, err := json.MarshalIndent(&spec, "", "  ")
	require.NoError(t, err)
	want = append(want, '\n')

	var out bytes.Buffer
	require.NoError(t, writeExtractedSchema(&out, &spec, errLoader{t}))
	assert.Equal(t, string(want), out.String())
}

// TestWriteExtractedSchemaNormalizes verifies that a sparse inheritance-bearing schema is normalized to the flattened
// canonical form: inherited members are materialized, the extends reference is preserved, and the transient
// requiredFeatures marker is dropped.
func TestWriteExtractedSchemaNormalizes(t *testing.T) {
	t.Parallel()

	spec := sparseInheritanceSchema()

	var out bytes.Buffer
	require.NoError(t, writeExtractedSchema(&out, &spec, errLoader{t}))

	// The normalized output must be a schema that no longer relies on inheritance materialization.
	var got schema.PackageSpec
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Empty(t, got.RequiredFeatures, "requiredFeatures must be dropped after normalization")

	derived, ok := got.Resources["test:index:Derived"]
	require.True(t, ok)
	// The extends reference is preserved in the normalized (flattened) form.
	require.NotNil(t, derived.Extends)
	assert.Equal(t, "#/resources/test:index:Base", derived.Extends.Ref)
	// The inherited member is materialized alongside the derived's own member (flattened).
	assert.Contains(t, derived.Properties, "host", "inherited output must be materialized")
	assert.Contains(t, derived.Properties, "replicas")
	assert.Contains(t, derived.InputProperties, "host", "inherited input must be materialized")
	assert.Contains(t, derived.InputProperties, "replicas")

	// The input spec really was sparse: proving the assertions above exercised materialization rather than passing on
	// an already-flattened schema.
	require.NotContains(t, spec.Resources["test:index:Derived"].Properties, "host")
}

// TestWriteExtractedSchemaFlattenedLocal binds the flattened-local shape the analyzers emit for a same-package base
// (extends + full member set, no requiredFeatures). Binding validates the flattened copies against the base, so a
// clean pass proves the analyzers' local emission is a shape the binder accepts and round-trips.
func TestWriteExtractedSchemaFlattenedLocal(t *testing.T) {
	t.Parallel()

	spec := flattenedInheritanceSchema()

	var out bytes.Buffer
	require.NoError(t, writeExtractedSchema(&out, &spec, errLoader{t}))

	var got schema.PackageSpec
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Empty(t, got.RequiredFeatures)

	derived, ok := got.Resources["test:index:Derived"]
	require.True(t, ok)
	require.NotNil(t, derived.Extends)
	assert.Equal(t, "#/resources/test:index:Base", derived.Extends.Ref)
	assert.Contains(t, derived.Properties, "host")
	assert.Contains(t, derived.Properties, "replicas")
	assert.Contains(t, derived.InputProperties, "host")
	assert.Contains(t, derived.InputProperties, "replicas")
}
