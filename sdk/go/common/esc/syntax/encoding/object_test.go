// Copyright 2023, Pulumi Corporation.
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

package encoding

import (
	"encoding/json"
	"testing"

	"github.com/pulumi/esc/syntax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func some[T any](v T) *T {
	return &v
}

func TestObjectBasic(t *testing.T) {
	type Embedded struct {
		Field string `syntax:"embedded"`
	}

	type basic struct {
		Embedded

		Null      any               `syntax:"null"`
		Bool      bool              `syntax:"bool"`
		Int       int               `syntax:"int"`
		Int8      int8              `syntax:"int8"`
		Int16     int16             `syntax:"int16"`
		Int32     int32             `syntax:"int32"`
		Int64     int64             `syntax:"int64"`
		Uint      uint              `syntax:"uint"`
		Uint8     uint8             `syntax:"uint8"`
		Uint16    uint16            `syntax:"uint16"`
		Uint32    uint32            `syntax:"uint32"`
		Uint64    uint64            `syntax:"uint64"`
		Float32   float32           `syntax:"float32"`
		Float64   float64           `syntax:"float64"`
		Number    json.Number       `syntax:"number"`
		String    string            `syntax:"string"`
		Array     []string          `syntax:"array"`
		NilArray  []string          `syntax:"nilArray"`
		Object    map[string]string `syntax:"object"`
		NilObject map[string]string `syntax:"nilObject"`
		Ptr       *string           `syntax:"ptr"`
		Any       any               `syntax:"any"`
	}

	v := basic{
		Embedded: Embedded{Field: "embedded-field"},
		Null:     nil,
		Bool:     true,
		Int:      1,
		Int8:     2,
		Int16:    3,
		Int32:    4,
		Int64:    5,
		Uint:     6,
		Uint8:    7,
		Uint16:   8,
		Uint32:   9,
		Uint64:   10,
		Float32:  3.14,
		Float64:  2.72,
		Number:   json.Number("42"),
		String:   "syntax",
		Array:    []string{"hello", "world"},
		Object:   map[string]string{"hello": "world"},
		Ptr:      some("syntax"),
		Any:      "value",
	}

	decoded, diags := DecodeValue(v)

	expected := syntax.Object(
		syntax.ObjectProperty(syntax.String("embedded"), syntax.String("embedded-field")),
		syntax.ObjectProperty(syntax.String("bool"), syntax.Boolean(true)),
		syntax.ObjectProperty(syntax.String("int"), syntax.Number(json.Number("1"))),
		syntax.ObjectProperty(syntax.String("int8"), syntax.Number(json.Number("2"))),
		syntax.ObjectProperty(syntax.String("int16"), syntax.Number(json.Number("3"))),
		syntax.ObjectProperty(syntax.String("int32"), syntax.Number(json.Number("4"))),
		syntax.ObjectProperty(syntax.String("int64"), syntax.Number(json.Number("5"))),
		syntax.ObjectProperty(syntax.String("uint"), syntax.Number(json.Number("6"))),
		syntax.ObjectProperty(syntax.String("uint8"), syntax.Number(json.Number("7"))),
		syntax.ObjectProperty(syntax.String("uint16"), syntax.Number(json.Number("8"))),
		syntax.ObjectProperty(syntax.String("uint32"), syntax.Number(json.Number("9"))),
		syntax.ObjectProperty(syntax.String("uint64"), syntax.Number(json.Number("10"))),
		syntax.ObjectProperty(syntax.String("float32"), syntax.Number(json.Number("3.140000104904175"))),
		syntax.ObjectProperty(syntax.String("float64"), syntax.Number(json.Number("2.72"))),
		syntax.ObjectProperty(syntax.String("number"), syntax.Number(json.Number("42"))),
		syntax.ObjectProperty(syntax.String("string"), syntax.String("syntax")),
		syntax.ObjectProperty(syntax.String("array"), syntax.Array(
			syntax.String("hello"), syntax.String("world"))),
		syntax.ObjectProperty(syntax.String("object"), syntax.Object(
			syntax.ObjectProperty(syntax.String("hello"), syntax.String("world")),
		)),
		syntax.ObjectProperty(syntax.String("ptr"), syntax.String("syntax")),
		syntax.ObjectProperty(syntax.String("any"), syntax.String("value")),
	)

	assert.Equal(t, expected, decoded)
	assert.Len(t, diags, 0)

	var encoded basic
	diags = EncodeValue(decoded, &encoded)
	assert.Equal(t, v, encoded)
	assert.Empty(t, diags)
}

func TestEncodeNodeField(t *testing.T) {
	var encoded struct {
		Node syntax.Node `syntax:"-"`

		String string `syntax:"string"`
	}
	node := syntax.Object(syntax.ObjectProperty(syntax.String("string"), syntax.String("syntax")))
	diags := EncodeValue(node, &encoded)
	require.Empty(t, diags)
	assert.Equal(t, "syntax", encoded.String)
	assert.Equal(t, encoded.Node, node)

	decoded, diags := DecodeValue(encoded)
	require.Empty(t, diags)
	assert.Equal(t, node, decoded)
}

func TestEncodeNull(t *testing.T) {
	var v any
	diags := EncodeValue(syntax.Null(), &v)
	assert.Empty(t, diags)
	assert.Nil(t, v)
}

func TestEncodeNode(t *testing.T) {
	var n syntax.Node
	diags := EncodeValue(syntax.String("hello"), &n)
	assert.Empty(t, diags)
	assert.Equal(t, syntax.String("hello"), n)
}

func TestEncodeAny(t *testing.T) {
	cases := []struct {
		node     syntax.Node
		expected any
	}{
		{syntax.Null(), nil},
		{syntax.Boolean(true), true},
		{syntax.Number(json.Number("3.14")), json.Number("3.14")},
		{syntax.String("syntax"), "syntax"},
		{syntax.Array(syntax.String("hello"), syntax.String("world")), []any{"hello", "world"}},
		{syntax.Object(syntax.ObjectProperty(syntax.String("hello"), syntax.String("world"))), map[string]any{"hello": "world"}},
	}
	for _, c := range cases {
		t.Run(c.node.String(), func(t *testing.T) {
			var v any
			diags := EncodeValue(c.node, &v)
			require.Empty(t, diags)
			assert.Equal(t, c.expected, v)
		})
	}
}

func TestInvalidObject(t *testing.T) {
	_, diags := DecodeValue(map[int]bool{42: true})
	assert.NotEmpty(t, diags)
}

func TestInvalidEncode(t *testing.T) {
	cases := []struct {
		node syntax.Node
		into any
	}{
		{syntax.Boolean(true), some(42)},
		{syntax.Number(json.Number("3.14")), some(42)},
		{syntax.Number(json.Number("3.14")), some("hello")},
		{syntax.String("syntax"), some(42)},
		{syntax.Array(syntax.String("hello"), syntax.String("world")), some(42)},
		{syntax.Object(syntax.ObjectProperty(syntax.String("hello"), syntax.String("world"))), some(42)},
	}
	for _, c := range cases {
		t.Run(c.node.String(), func(t *testing.T) {
			diags := EncodeValue(c.node, c.into)
			assert.NotEmpty(t, diags)
		})
	}

}
