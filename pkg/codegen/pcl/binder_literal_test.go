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

package pcl_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
)

// TestBindNestedCollectionLiterals binds programs whose collection literals nest literals of
// different shapes. Each scalar literal has its own const type, and the binder must unify
// those into one collection type rather than a union of collections.
func TestBindNestedCollectionLiterals(t *testing.T) {
	t.Parallel()

	cf, ct := model.NewConstType(model.BoolType, cty.False), model.NewConstType(model.BoolType, cty.True)
	optional := model.NewOptionalType

	cases := []struct {
		name   string
		source string
		typ    model.Type
	}{
		{
			name:   "for over tuples of different constants",
			source: `o = [for x in [[1], [2]] : length(x)]`,
			typ:    model.NewListType(model.IntType),
		},
		{
			name:   "try over nested tuples of different lengths",
			source: `o = try([[[false, false, false]]][length([[[false, false, false]]])], [[true], [false]])`,
			typ: model.NewTupleType(
				model.NewTupleType(model.NewUnionType(cf, ct), optional(cf), optional(cf)),
				optional(model.NewTupleType(cf)),
			),
		},
		{
			name:   "splat over objects with tuples of different lengths",
			source: `o = [{"a" = [false], "b" = [false, false]}, {"a" = [false, false], "b" = [true]}][*].a`,
			typ:    model.NewListType(model.NewTupleType(cf, optional(cf))),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			program, diags, err := ParseAndBindProgram(t, c.source, "main.pp")
			require.NoError(t, err)
			require.False(t, diags.HasErrors(), diags.Error())
			require.Len(t, program.Nodes, 1)
			local := program.Nodes[0].(*pcl.LocalVariable)
			require.True(t, c.typ.Equals(local.Type()), "expected %v, got %v", c.typ, local.Type())
		})
	}
}
