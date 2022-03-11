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

package resource

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMappable ensures that we properly convert from resource property maps to their "weakly typed" JSON-like
// equivalents.
func TestMappable(t *testing.T) {
	t.Parallel()

	ma1 := map[string]interface{}{
		"a": float64(42.3),
		"b": false,
		"c": "foobar",
		"d": []interface{}{"x", float64(99), true},
		"e": map[string]interface{}{
			"e.1": "z",
			"e.n": float64(676.767),
			"e.^": []interface{}{"bbb"},
		},
		"f": []interface{}{},
	}
	ma1p := NewPropertyMapFromMap(ma1)
	assert.Equal(t, len(ma1), len(ma1p))
	ma1mm := ma1p.Mappable()
	assert.Equal(t, ma1, ma1mm)
}

// TestMapReplValues ensures that we properly convert from resource property maps to their "weakly typed" JSON-like
// equivalents, but with additional and optional functions that replace values inline as we go.
func TestMapReplValues(t *testing.T) {
	t.Parallel()

	// First, no replacements (nil repl).
	ma1 := map[string]interface{}{
		"a": float64(42.3),
		"b": false,
		"c": "foobar",
		"d": []interface{}{"x", float64(99), true},
		"e": map[string]interface{}{
			"e.1": "z",
			"e.n": float64(676.767),
			"e.^": []interface{}{"bbb"},
		},
	}
	ma1p := NewPropertyMapFromMap(ma1)
	assert.Equal(t, len(ma1), len(ma1p))
	ma1mm := ma1p.MapRepl(nil, nil)
	assert.Equal(t, ma1, ma1mm)

	// First, no replacements (false-returning repl).
	ma2 := map[string]interface{}{
		"a": float64(42.3),
		"b": false,
		"c": "foobar",
		"d": []interface{}{"x", float64(99), true},
		"e": map[string]interface{}{
			"e.1": "z",
			"e.n": float64(676.767),
			"e.^": []interface{}{"bbb"},
		},
	}
	ma2p := NewPropertyMapFromMap(ma2)
	assert.Equal(t, len(ma2), len(ma2p))
	ma2mm := ma2p.MapRepl(nil, func(v PropertyValue) (interface{}, bool) {
		return nil, false
	})
	assert.Equal(t, ma2, ma2mm)

	// Finally, actually replace some numbers with ints.
	ma3 := map[string]interface{}{
		"a": float64(42.3),
		"b": false,
		"c": "foobar",
		"d": []interface{}{"x", float64(99), true},
		"e": map[string]interface{}{
			"e.1": "z",
			"e.n": float64(676.767),
			"e.^": []interface{}{"bbb"},
		},
	}
	ma3p := NewPropertyMapFromMap(ma3)
	assert.Equal(t, len(ma3), len(ma3p))
	ma3mm := ma3p.MapRepl(nil, func(v PropertyValue) (interface{}, bool) {
		if v.IsNumber() {
			return int(v.NumberValue()), true
		}
		return nil, false
	})
	// patch the original map so it can compare easily
	ma3["a"] = int(ma3["a"].(float64))
	ma3["d"].([]interface{})[1] = int(ma3["d"].([]interface{})[1].(float64))
	ma3["e"].(map[string]interface{})["e.n"] = int(ma3["e"].(map[string]interface{})["e.n"].(float64))
	assert.Equal(t, ma3, ma3mm)
}

func TestMapReplKeys(t *testing.T) {
	t.Parallel()

	m := map[string]interface{}{
		"a": float64(42.3),
		"b": false,
		"c": "foobar",
		"d": []interface{}{"x", float64(99), true},
		"e": map[string]interface{}{
			"e.1": "z",
			"e.n": float64(676.767),
			"e.^": []interface{}{"bbb"},
		},
	}
	ma := NewPropertyMapFromMap(m)
	assert.Equal(t, len(m), len(ma))
	mam := ma.MapRepl(func(k string) (string, bool) {
		return strings.ToUpper(k), true
	}, nil)
	assert.Equal(t, m["a"], mam["A"])
	assert.Equal(t, m["b"], mam["B"])
	assert.Equal(t, m["c"], mam["C"])
	assert.Equal(t, m["d"], mam["D"])
	assert.Equal(t, m["e"].(map[string]interface{})["e.1"], mam["E"].(map[string]interface{})["E.1"])
	assert.Equal(t, m["e"].(map[string]interface{})["e.n"], mam["E"].(map[string]interface{})["E.N"])
	assert.Equal(t, m["e"].(map[string]interface{})["e.^"], mam["E"].(map[string]interface{})["E.^"])
}

func TestMapReplComputedOutput(t *testing.T) {
	t.Parallel()

	m := make(PropertyMap)
	m["a"] = NewComputedProperty(Computed{Element: NewStringProperty("X")})
	m["b"] = NewOutputProperty(Output{Element: NewNumberProperty(46)})
	mm := m.MapRepl(nil, nil)
	assert.Equal(t, len(m), len(mm))
	m2 := NewPropertyMapFromMap(mm)
	assert.Equal(t, m, m2)
}

func TestCopy(t *testing.T) {
	t.Parallel()

	src := NewPropertyMapFromMap(map[string]interface{}{
		"a": "str",
		"b": 42,
	})
	dst := src.Copy()
	assert.NotNil(t, dst)
	assert.Equal(t, len(src), len(dst))
	assert.Equal(t, src["a"], dst["a"])
	assert.Equal(t, src["b"], dst["b"])
	src["a"] = NewNullProperty()
	assert.Equal(t, NewStringProperty("str"), dst["a"])
	src["c"] = NewNumberProperty(99.99)
	assert.Equal(t, 2, len(dst))
}

func TestSecretUnknown(t *testing.T) {
	t.Parallel()

	o := NewOutputProperty(Output{Element: NewNumberProperty(46)})
	so := MakeSecret(o)
	assert.True(t, o.ContainsUnknowns())
	assert.True(t, so.ContainsUnknowns())
	c := NewComputedProperty(Computed{Element: NewStringProperty("X")})
	co := MakeSecret(so)
	assert.True(t, c.ContainsUnknowns())
	assert.True(t, co.ContainsUnknowns())
}

func TestTypeString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		prop     PropertyValue
		expected string
	}{
		{
			prop:     MakeComputed(NewStringProperty("")),
			expected: "output<string>",
		},
		{
			prop:     MakeSecret(NewStringProperty("")),
			expected: "secret<string>",
		},
		{
			prop:     MakeOutput(NewStringProperty("")),
			expected: "output<string>",
		},
		{
			prop: NewOutputProperty(Output{
				Element: NewStringProperty(""),
				Known:   true,
			}),
			expected: "string",
		},
		{
			prop: NewOutputProperty(Output{
				Element: NewStringProperty(""),
				Known:   true,
				Secret:  true,
			}),
			expected: "secret<string>",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.expected, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, tt.prop.TypeString())
		})
	}
}

func TestString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		prop     PropertyValue
		expected string
	}{
		{
			prop:     MakeComputed(NewStringProperty("")),
			expected: "output<string>{}",
		},
		{
			prop:     MakeSecret(NewStringProperty("shh")),
			expected: "{&{{shh}}}",
		},
		{
			prop:     MakeOutput(NewStringProperty("")),
			expected: "output<string>{}",
		},
		{
			prop: NewOutputProperty(Output{
				Element: NewStringProperty("hello"),
				Known:   true,
			}),
			expected: "{hello}",
		},
		{
			prop: NewOutputProperty(Output{
				Element: NewStringProperty("shh"),
				Known:   true,
				Secret:  true,
			}),
			expected: "{&{{shh}}}",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.expected, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, tt.prop.String())
		})
	}
}

func TestContainsUnknowns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		prop     PropertyValue
		expected bool
	}{
		{
			name:     "computed unknown",
			prop:     MakeComputed(NewStringProperty("")),
			expected: true,
		},
		{
			name:     "output unknown",
			prop:     MakeOutput(NewStringProperty("")),
			expected: true,
		},
		{
			name: "output known",
			prop: NewOutputProperty(Output{
				Element: NewStringProperty(""),
				Known:   true,
			}),
			expected: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, tt.prop.ContainsUnknowns())
		})
	}
}

func TestContainsSecrets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		prop     PropertyValue
		expected bool
	}{
		{
			name:     "secret",
			prop:     MakeSecret(NewStringProperty("")),
			expected: true,
		},
		{
			name:     "output unknown",
			prop:     MakeOutput(NewStringProperty("")),
			expected: false,
		},
		{
			name:     "output unknown containing secret",
			prop:     MakeOutput(MakeSecret(NewStringProperty(""))),
			expected: true,
		},
		{
			name: "output unknown secret",
			prop: NewOutputProperty(Output{
				Element: NewStringProperty(""),
				Secret:  true,
			}),
			expected: true,
		},
		{
			name: "output known secret",
			prop: NewOutputProperty(Output{
				Element: NewStringProperty(""),
				Known:   true,
				Secret:  true,
			}),
			expected: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, tt.prop.ContainsSecrets())
		})
	}
}

func TestHasValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		prop     PropertyValue
		expected bool
	}{
		{
			name:     "null",
			prop:     NewNullProperty(),
			expected: false,
		},
		{
			name:     "string",
			prop:     NewStringProperty(""),
			expected: true,
		},
		{
			name:     "output unknown",
			prop:     MakeOutput(NewStringProperty("")),
			expected: false,
		},
		{
			name: "output known",
			prop: NewOutputProperty(Output{
				Element: NewStringProperty(""),
				Known:   true,
			}),
			expected: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, tt.prop.HasValue())
		})
	}
}
