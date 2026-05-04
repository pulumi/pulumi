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
