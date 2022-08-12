// Copyright 2016-2021, Pulumi Corporation.
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

package codegen

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/assert"
)

func TestStringSetContains(t *testing.T) {
	t.Parallel()

	set123 := NewStringSet("1", "2", "3")
	set12 := NewStringSet("1", "2")
	set14 := NewStringSet("1", "4")
	setEmpty := NewStringSet()

	assert.True(t, set123.Contains(set123))
	assert.True(t, set123.Contains(set12))
	assert.False(t, set12.Contains(set123))
	assert.False(t, set123.Contains(set14))
	assert.True(t, set123.Contains(setEmpty))
}

func TestStringSetSubtract(t *testing.T) {
	t.Parallel()

	set1234 := NewStringSet("1", "2", "3", "4")
	set125 := NewStringSet("1", "2", "5")
	set34 := NewStringSet("3", "4")
	setEmpty := NewStringSet()

	assert.Equal(t, set34, set1234.Subtract(set125))
	assert.Equal(t, setEmpty, set1234.Subtract(set1234))
	assert.Equal(t, set1234, set1234.Subtract(setEmpty))
}

func TestSimplifyInputUnion(t *testing.T) {
	t.Parallel()

	u1 := &schema.UnionType{
		ElementTypes: []schema.Type{
			&schema.InputType{ElementType: schema.StringType},
			schema.NumberType,
		},
	}

	u2 := SimplifyInputUnion(u1)
	assert.Equal(t, &schema.UnionType{
		ElementTypes: []schema.Type{
			schema.StringType,
			schema.NumberType,
		},
	}, u2)
}

func TestCamel(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	assert.Equal("", Camel(""))
	assert.Equal("plugh", Camel("plugh"))
	assert.Equal("waldoThudFred", Camel("WaldoThudFred"))
	assert.Equal("graultBaz", Camel("Grault-Baz"))
	assert.Equal("graultBaz", Camel("grault-baz"))
	assert.Equal("graultBaz", Camel("graultBaz"))
	assert.Equal("grault_Baz", Camel("Grault_Baz"))
	assert.Equal("graultBaz", Camel("Grault-baz"))
}

func TestConvertHyphens(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	assert.Equal("", ConvertHyphens("", ""))
	assert.Equal("plugh", ConvertHyphens("plugh", ""))
	assert.Equal("waldoThudFred", ConvertHyphens("waldo-thud-Fred", ""))
	assert.Equal("GarlpyCorgeFred", ConvertHyphens("Garlpy-Corge-Fred", ""))
	assert.Equal("thud", ConvertHyphens("thud-", ""))
	assert.Equal("Thud", ConvertHyphens("-thud", ""))
	assert.Equal("waldo_Thud_Fred", ConvertHyphens("waldo-thud-Fred", "_"))
	assert.Equal("Garlpy_Corge_Fred", ConvertHyphens("Garlpy-Corge-Fred", "_"))
}

func TestSeparate(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	assert.Equal([]string{"a", "b", "c"}, Separate("abc", ""))

	assert.Equal([]string{"Garply", "_", "Plugh", "_", "Waldo"}, Separate("Garply_Plugh_Waldo", "_"))
	assert.Equal([]string{"garply", "_", "plugh", "_", "waldo"}, Separate("garply_plugh_waldo", "_"))

	assert.Equal([]string{"a", "_", "b"}, Separate("a_b", "_"))
	assert.Equal([]string{"a", "_", "b"}, Separate("a_b", "_"))
	assert.Equal([]string{"_", "a"}, Separate("_a", "_"))
	assert.Equal([]string{"a", "_"}, Separate("a_", "_"))
	assert.Equal([]string{"_", "a", "_"}, Separate("_a_", "_"))
	assert.Equal([]string{"a", "_", "b", "_", "c"}, Separate("a_b_c", "_"))
	assert.Equal([]string{"a", "_", "b", "_", "c", "_"}, Separate("a_b_c_", "_"))
	assert.Equal([]string{"_", "a", "_", "b", "_", "c", "_"}, Separate("_a_b_c_", "_"))

	assert.Equal([]string{"a", "_", "_", "b"}, Separate("a__b", "_"))
	assert.Equal([]string{"a", "_", "_", "b", "_", "_"}, Separate("a__b__", "_"))
	assert.Equal([]string{"_", "_", "a", "_", "b", "_", "_"}, Separate("__a_b__", "_"))
}
