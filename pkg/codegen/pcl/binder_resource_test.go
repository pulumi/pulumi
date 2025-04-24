// Copyright 2016-2023, Pulumi Corporation.
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
			want: cty.TupleVal([]cty.Value{cty.DynamicVal}),
		},
		{
			name: "DeletedWith",
			src:  `deletedWith = "abc123"`,
			want: cty.StringVal("abc123"),
		},
	}

	for _, tt := range tests {
		tt := tt
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
