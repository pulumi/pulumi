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

package model

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"
)

func TestTupleTypeTraverseIndex(t *testing.T) {
	t.Parallel()

	tuple := NewTupleType(BoolType, StringType)
	rng := hcl.Range{Filename: "test.pp"}

	cases := []struct {
		name  string
		index int64
		typ   Traversable
		diags hcl.Diagnostics
	}{
		{name: "first", index: 0, typ: BoolType},
		{name: "last", index: 1, typ: StringType},
		{name: "length", index: 2, typ: DynamicType, diags: hcl.Diagnostics{tupleIndexOutOfRange(2, rng)}},
		{name: "past length", index: 3, typ: DynamicType, diags: hcl.Diagnostics{tupleIndexOutOfRange(2, rng)}},
		{name: "negative", index: -1, typ: DynamicType, diags: hcl.Diagnostics{tupleIndexOutOfRange(2, rng)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			typ, diags := tuple.Traverse(hcl.TraverseIndex{Key: cty.NumberIntVal(c.index), SrcRange: rng})
			assert.Equal(t, c.typ, typ)
			assert.Equal(t, c.diags, diags)
		})
	}
}
