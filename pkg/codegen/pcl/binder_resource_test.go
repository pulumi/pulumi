// Copyright 2016, Pulumi Corporation.
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

package pcl

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestBindResourceOptions(t *testing.T) {
	t.Parallel()

	fooPkg := schema.Package{
		Name: "foo",
		Provider: &schema.Resource{
			Token: "foo:index:Foo",
			InputProperties: []*schema.Property{
				{Name: "property", Type: schema.StringType},
			},
			Properties: []*schema.Property{
				{Name: "property", Type: schema.StringType},
			},
		},
		Resources: []*schema.Resource{
			{
				Token: "foo:index:Foo",
				InputProperties: []*schema.Property{
					{Name: "property", Type: schema.StringType},
				},
				Properties: []*schema.Property{
					{Name: "property", Type: schema.StringType},
				},
			},
		},
	}

	tests := []struct {
		name string // ResourceOptions field name
		src  string // line in options block
		want cty.Value
	}{
		{
			name: "Range",
			src:  "range = 42",
			want: cty.NumberIntVal(42),
		},
		{
			name: "Protect",
			src:  "protect = true",
			want: cty.True,
		},
		{
			name: "RetainOnDelete",
			src:  "retainOnDelete = true",
			want: cty.True,
		},
		{
			name: "Version",
			src:  `version = "1.2.3"`,
			want: cty.StringVal("1.2.3"),
		},
		{
			name: "PluginDownloadURL",
			src:  `pluginDownloadURL = "https://example.com/whatever"`,
			want: cty.StringVal("https://example.com/whatever"),
		},
		{
			name: "ImportID",
			src:  `import = "abc123"`,
			want: cty.StringVal("abc123"),
		},
		{
			name: "IgnoreChanges",
			src:  `ignoreChanges = [property]`,
			want: cty.TupleVal([]cty.Value{cty.StringVal("property")}),
		},
		{
			name: "HideDiffs",
			src:  `hideDiffs = [property]`,
			want: cty.TupleVal([]cty.Value{cty.StringVal("property")}),
		},
		{
			name: "DeletedWith",
			src:  `deletedWith = "abc123"`,
			want: cty.StringVal("abc123"),
		},
		{
			name: "AdditionalSecretOutputs",
			src:  `additionalSecretOutputs = [property]`,
			want: cty.TupleVal([]cty.Value{cty.StringVal("property")}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var sb strings.Builder
			fmt.Fprintln(&sb, `resource foo "foo:index:Foo" {`)
			fmt.Fprintln(&sb, "	property = \"42\"")
			fmt.Fprintln(&sb, "	options {")
			fmt.Fprintln(&sb, "		"+tt.src)
			fmt.Fprintln(&sb, "	}")
			fmt.Fprintln(&sb, `}`)
			src := sb.String()
			defer func() {
				// If the test fails, print the source code
				// for easier debugging.
				if t.Failed() {
					t.Logf("source:\n%s", src)
				}
			}()

			parser := syntax.NewParser()
			require.NoError(t,
				parser.ParseFile(strings.NewReader(src), "test.pcl"),
				"parse failed")

			prog, diag, err := BindProgram(parser.Files, Loader(&stubSchemaLoader{
				Package: &fooPkg,
			}))
			require.NoError(t, err, "bind failed")
			require.Empty(t, diag, "bind failed")

			require.Len(t, prog.Nodes, 1, "expected one node")
			require.IsType(t, &Resource{}, prog.Nodes[0], "expected a resource")
			res := prog.Nodes[0].(*Resource)

			expr := reflect.ValueOf(res.Options).Elem().
				FieldByName(tt.name).Interface().(model.Expression)

			v, diag := expr.Evaluate(&hcl.EvalContext{})
			require.Empty(t, diag, "evaluation failed")

			// We can't use assert.Equal here.
			// cty.Value internals can differ, but still be equal.
			if !v.RawEquals(tt.want) {
				t.Errorf("unexpected value: %#v != %#v", tt.want, v)
			}
		})
	}
}

func TestBindReadResourceOptions(t *testing.T) {
	t.Parallel()

	fooPkg := schema.Package{
		Name: "foo",
		Provider: &schema.Resource{
			Token: "foo:index:Foo",
			InputProperties: []*schema.Property{
				{Name: "property", Type: schema.StringType},
			},
			Properties: []*schema.Property{
				{Name: "property", Type: schema.StringType},
			},
		},
		Resources: []*schema.Resource{
			{
				Token: "foo:index:Foo",
				InputProperties: []*schema.Property{
					{Name: "property", Type: schema.StringType},
				},
				Properties: []*schema.Property{
					{Name: "property", Type: schema.StringType},
				},
			},
		},
	}

	tests := []struct {
		name string // ResourceOptions field name
		src  string // line in options block
		want cty.Value
	}{
		{
			name: "Protect",
			src:  "protect = true",
			want: cty.True,
		},
		{
			name: "RetainOnDelete",
			src:  "retainOnDelete = true",
			want: cty.True,
		},
		{
			name: "ImportID",
			src:  `import = "abc123"`,
			want: cty.StringVal("abc123"),
		},
		{
			name: "CustomTimeouts",
			src:  `customTimeouts = { create = "5m" }`,
			want: cty.ObjectVal(map[string]cty.Value{
				"create": cty.StringVal("5m"),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var sb strings.Builder
			fmt.Fprintln(&sb, `read foo "foo:index:Foo" {`)
			fmt.Fprintln(&sb, `	id = "42"`)
			fmt.Fprintln(&sb, "	options {")
			fmt.Fprintln(&sb, "		"+tt.src)
			fmt.Fprintln(&sb, "	}")
			fmt.Fprintln(&sb, `}`)
			src := sb.String()
			defer func() {
				if t.Failed() {
					t.Logf("source:\n%s", src)
				}
			}()

			parser := syntax.NewParser()
			require.NoError(t,
				parser.ParseFile(strings.NewReader(src), "test.pcl"),
				"parse failed")

			prog, diag, err := BindProgram(parser.Files, Loader(&stubSchemaLoader{
				Package: &fooPkg,
			}))
			require.NoError(t, err, "bind failed")
			require.Empty(t, diag, "bind failed")

			require.Len(t, prog.Nodes, 1, "expected one node")
			require.IsType(t, &ReadResource{}, prog.Nodes[0], "expected a read resource")
			res := prog.Nodes[0].(*ReadResource)

			expr := reflect.ValueOf(res.Options).Elem().
				FieldByName(tt.name).Interface().(model.Expression)

			v, diag := expr.Evaluate(&hcl.EvalContext{})
			require.Empty(t, diag, "evaluation failed")

			if !v.RawEquals(tt.want) {
				t.Errorf("unexpected value: %#v != %#v", tt.want, v)
			}
		})
	}
}

func TestBindReadComponentResourceFails(t *testing.T) {
	t.Parallel()

	fooPkgSpec := schema.PackageSpec{
		Name:    "foo",
		Version: "1.0.0",
		Provider: schema.ResourceSpec{
			ObjectTypeSpec: schema.ObjectTypeSpec{
				Type: "object",
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"foo:index:Component": {
				IsComponent: true,
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"id": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
				},
				StateInputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"lookup": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
				},
			},
		},
	}
	fooPkg, diags, err := schema.BindSpec(fooPkgSpec, nil, schema.ValidationOptions{})
	require.NoError(t, err)
	require.False(t, diags.HasErrors())

	src := `read comp "foo:index:Component" {
	id = "existing-id"
	lookup = "existing-key"
}`

	parser := syntax.NewParser()
	require.NoError(t,
		parser.ParseFile(strings.NewReader(src), "test.pcl"),
		"parse failed")

	prog, diags, err := BindProgram(parser.Files, Loader(&stubSchemaLoader{
		Package: fooPkg,
	}))
	require.Error(t, err, "bind should fail")
	require.True(t, diags.HasErrors(), "expected bind diagnostics")
	require.Nil(t, prog)
	require.Contains(t, diags.Error(), "component resources cannot be read")
	require.Contains(t, diags.Error(), "Component")
}

func TestBindResourceIgnoreChangesNameCollision(t *testing.T) {
	t.Parallel()

	pkgSpec := schema.PackageSpec{
		Name:    "foo",
		Version: "1.0.0",
		Provider: schema.ResourceSpec{
			ObjectTypeSpec: schema.ObjectTypeSpec{Type: "object"},
		},
		Resources: map[string]schema.ResourceSpec{
			"foo:index:Foo": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"property": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					Required: []string{"property"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"property": {TypeSpec: schema.TypeSpec{Type: "string"}},
				},
				RequiredInputs: []string{"property"},
			},
			// Input named "ignoreChanges" so the user-attribute subtest can
			// exercise the top-level case where the name collides.
			"foo:index:Bar": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"ignoreChanges": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					Required: []string{"ignoreChanges"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"ignoreChanges": {TypeSpec: schema.TypeSpec{Type: "string"}},
				},
				RequiredInputs: []string{"ignoreChanges"},
			},
		},
	}
	pkg, diags, err := schema.BindSpec(pkgSpec, nil, schema.ValidationOptions{})
	require.NoError(t, err, "BindSpec failed")
	require.False(t, diags.HasErrors(), "BindSpec diagnostics: %v", diags.Error())

	tests := []struct {
		name     string
		src      string
		subject  string
		wantDeps []string
	}{
		{
			name: "ignoreChanges entry shadows the resource's own name",
			src: `
resource property "foo:index:Foo" {
	property = "p"
	options {
		ignoreChanges = [property]
	}
}`,
			subject: "property",
		},
		{
			name: "ignoreChanges entry shadows a sibling resource's name",
			src: `
resource property "foo:index:Foo" {
	property = "s"
}
resource other "foo:index:Foo" {
	property = "p"
	options {
		ignoreChanges = [property]
	}
}`,
			subject: "other",
		},
		{
			name: "all four shadowing attributes are suppressed",
			src: `
resource property "foo:index:Foo" {
	property = "p"
	options {
		ignoreChanges           = [property]
		hideDiffs               = [property]
		replaceOnChanges        = [property]
		additionalSecretOutputs = [property]
	}
}`,
			subject: "property",
		},
		{
			// Top-level "ignoreChanges" is a user property, not the options
			// attribute, so its references must still resolve at root scope.
			name: "user attribute named ignoreChanges still tracks deps",
			src: `
target = "t"
resource bar "foo:index:Bar" {
	ignoreChanges = target
}`,
			subject:  "bar",
			wantDeps: []string{"target"},
		},
		{
			// Guards against an over-broad fix that would suppress every
			// options-block lookup; dependsOn is not a property-name attribute.
			name: "options.dependsOn still registers root references",
			src: `
resource target "foo:index:Foo" {
	property = "t"
}
resource property "foo:index:Foo" {
	property = "p"
	options {
		dependsOn     = [target]
		ignoreChanges = [property]
	}
}`,
			subject:  "property",
			wantDeps: []string{"target"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			defer func() {
				if t.Failed() {
					t.Logf("source:\n%s", tt.src)
				}
			}()

			parser := syntax.NewParser()
			require.NoError(t,
				parser.ParseFile(strings.NewReader(tt.src), "test.pp"),
				"parse failed")

			prog, bindDiags, err := BindProgram(parser.Files, Loader(&stubSchemaLoader{
				Package: pkg,
			}))
			require.NoError(t, err, "bind returned error")
			require.Falsef(t, bindDiags.HasErrors(), "bind diagnostics: %v", bindDiags.Error())
			require.NotNil(t, prog)

			var subject Node
			for _, n := range prog.Nodes {
				if n.Name() == tt.subject {
					subject = n
					break
				}
			}
			require.NotNilf(t, subject, "no node named %q in program", tt.subject)

			deps := subject.GetDependencies()
			gotDeps := make([]string, 0, len(deps))
			for _, d := range deps {
				gotDeps = append(gotDeps, d.Name())
			}
			require.ElementsMatch(t, tt.wantDeps, gotDeps,
				"unexpected dependency set for %q", subject.Name())
		})
	}
}

func TestResolveUnionOfObjectsToleratesNonStringDiscriminatorValue(t *testing.T) {
	t.Parallel()

	// resolveUnionOfObjects looks up the value at the discriminator key in
	// the object expression. When that value is a non-string literal (e.g.
	// `false` against a discriminator whose union mapping is keyed by
	// string), AsString() used to panic with "not a string." The function
	// must instead skip over the non-string value and leave the union
	// unresolved so binding can continue.
	obj := &model.ObjectConsExpression{
		Items: []model.ObjectConsItem{{
			Key: &model.LiteralValueExpression{
				Value: cty.StringVal("kind"),
			},
			Value: &model.LiteralValueExpression{
				Value: cty.False,
			},
		}},
	}
	union := &schema.UnionType{
		Discriminator: "kind",
		Mapping:       map[string]string{"a": "pkg:index:A"},
		ElementTypes: []schema.Type{
			&schema.ObjectType{Token: "pkg:index:A"},
			&schema.ObjectType{Token: "pkg:index:B"},
		},
	}

	require.NotPanics(t, func() {
		got := resolveUnionOfObjects(obj, union)
		// With a non-string discriminator value, the union cannot be
		// reduced; the function returns the union unchanged.
		require.Same(t, union, got)
	})
}

func TestResolveUnionOfObjectsResolvesStringDiscriminatorValue(t *testing.T) {
	t.Parallel()

	// Sanity check that the non-panic path still resolves a well-formed
	// discriminator. A string literal value at the discriminator key
	// selects the corresponding object type via union.Mapping.
	objA := &schema.ObjectType{Token: "pkg:index:A"}
	objB := &schema.ObjectType{Token: "pkg:index:B"}
	union := &schema.UnionType{
		Discriminator: "kind",
		Mapping:       map[string]string{"a": "pkg:index:A", "b": "pkg:index:B"},
		ElementTypes:  []schema.Type{objA, objB},
	}

	obj := &model.ObjectConsExpression{
		Items: []model.ObjectConsItem{{
			Key:   &model.LiteralValueExpression{Value: cty.StringVal("kind")},
			Value: &model.LiteralValueExpression{Value: cty.StringVal("b")},
		}},
	}
	require.Same(t, objB, resolveUnionOfObjects(obj, union))

	// Same input, expressed as a one-part TemplateExpression — the
	// importer round-trips literal strings through TemplateExpression.
	objTpl := &model.ObjectConsExpression{
		Items: []model.ObjectConsItem{{
			Key: &model.LiteralValueExpression{Value: cty.StringVal("kind")},
			Value: &model.TemplateExpression{Parts: []model.Expression{
				&model.LiteralValueExpression{Value: cty.StringVal("a")},
			}},
		}},
	}
	require.Same(t, objA, resolveUnionOfObjects(objTpl, union))
}

type stubSchemaLoader struct {
	Package *schema.Package
}

var _ schema.Loader = (*stubSchemaLoader)(nil)

func (l *stubSchemaLoader) LoadPackage(pkg string, ver *semver.Version) (*schema.Package, error) {
	return l.Package, nil
}

func (l *stubSchemaLoader) LoadPackageV2(
	ctx context.Context, descriptor *schema.PackageDescriptor,
) (*schema.Package, error) {
	return l.Package, nil
}
