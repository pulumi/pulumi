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

package formal_test

import (
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
)

// corpusType is the JSON encoding of a type in corpus.json (README.md).
type corpusType struct {
	Kind    string                `json:"kind"`
	Value   *corpusValue          `json:"value"`
	Token   string                `json:"token"`
	Base    *corpusType           `json:"base"`
	Values  []corpusValue         `json:"values"`
	Elem    *corpusType           `json:"elem"`
	Elems   []corpusType          `json:"elems"`
	Props   map[string]corpusType `json:"props"`
	Members []corpusType          `json:"members"`
}

type corpusValue struct {
	Bool   *bool   `json:"bool"`
	Int    *int64  `json:"int"`
	Number *string `json:"number"`
	String *string `json:"string"`
}

type corpusEntry struct {
	A      corpusType `json:"a"`
	B      corpusType `json:"b"`
	ConvAB string     `json:"conv_ab"`
	ConvBA string     `json:"conv_ba"`
	Safe   corpusType `json:"safe"`
	Unsafe corpusType `json:"unsafe"`
}

func (v corpusValue) decode(t *testing.T) (cty.Value, model.Type) {
	switch {
	case v.Bool != nil:
		return cty.BoolVal(*v.Bool), model.BoolType
	case v.Int != nil:
		return cty.NumberIntVal(*v.Int), model.IntType
	case v.Number != nil:
		f, _, err := big.ParseFloat(*v.Number, 10, 512, big.ToNearestEven)
		require.NoError(t, err)
		return cty.NumberVal(f), model.NumberType
	case v.String != nil:
		return cty.StringVal(*v.String), model.StringType
	}
	t.Fatalf("empty corpus value")
	return cty.NilVal, nil
}

func (c corpusType) decode(t *testing.T) model.Type {
	switch c.Kind {
	case "bool":
		return model.BoolType
	case "int":
		return model.IntType
	case "number":
		return model.NumberType
	case "string":
		return model.StringType
	case "id":
		return model.IDType
	case "dynamic":
		return model.DynamicType
	case "none":
		return model.NoneType
	case "const":
		value, base := c.Value.decode(t)
		return model.NewConstType(base, value)
	case "enum":
		values := make([]cty.Value, len(c.Values))
		for i, v := range c.Values {
			values[i], _ = v.decode(t)
		}
		return model.NewEnumType(c.Token, c.Base.decode(t), values)
	case "list":
		return model.NewListType(c.Elem.decode(t))
	case "set":
		return model.NewSetType(c.Elem.decode(t))
	case "map":
		return model.NewMapType(c.Elem.decode(t))
	case "output":
		return model.NewOutputType(c.Elem.decode(t))
	case "promise":
		return model.NewPromiseType(c.Elem.decode(t))
	case "tuple":
		elems := make([]model.Type, len(c.Elems))
		for i, e := range c.Elems {
			elems[i] = e.decode(t)
		}
		return model.NewTupleType(elems...)
	case "object":
		props := map[string]model.Type{}
		for k, p := range c.Props {
			props[k] = p.decode(t)
		}
		return model.NewObjectType(props)
	case "union":
		members := make([]model.Type, len(c.Members))
		for i, m := range c.Members {
			members[i] = m.decode(t)
		}
		return model.NewUnionType(members...)
	}
	t.Fatalf("unknown corpus kind %q", c.Kind)
	return nil
}

func decodeKind(t *testing.T, kind string) model.ConversionKind {
	switch kind {
	case "safe":
		return model.SafeConversion
	case "unsafe":
		return model.UnsafeConversion
	case "no":
		return model.NoConversion
	}
	t.Fatalf("unknown conversion kind %q", kind)
	return model.NoConversion
}

// TestCorpus checks ConversionFrom and UnifyTypes against every entry of the golden corpus that the Lean model
// exports (corpus.json).
func TestCorpus(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("corpus.json")
	require.NoError(t, err)
	var entries []corpusEntry
	require.NoError(t, json.Unmarshal(data, &entries))
	require.NotEmpty(t, entries)

	for _, entry := range entries {
		a, b := entry.A.decode(t), entry.B.decode(t)
		expected := entry.Safe.decode(t)
		assert.True(t, expected.Equals(entry.Unsafe.decode(t)), "corpus entry with distinct safe and unsafe types")

		assert.Equal(t, decodeKind(t, entry.ConvAB), a.ConversionFrom(b), "%v <- %v", a, b)
		assert.Equal(t, decodeKind(t, entry.ConvBA), b.ConversionFrom(a), "%v <- %v", b, a)
		actual := model.UnifyTypes(a, b)
		assert.True(t, expected.Equals(actual), "UnifyTypes(%v, %v): expected %v, got %v", a, b, expected, actual)
	}
}
