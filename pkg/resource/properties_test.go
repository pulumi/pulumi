// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMappable ensures that we properly convert from resource property maps to their "weakly typed" JSON-like
// equivalents.
func TestMappable(t *testing.T) {
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
	ma1mm := ma1p.Mappable()
	assert.Equal(t, ma1, ma1mm)
}

// TestReplace ensures that we properly convert from resource property maps to their "weakly typed" JSON-like
// equivalents, but with additional and optional functions that replace values inline as we go.
func TestMapReplace(t *testing.T) {
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
	ma1mm := ma1p.MapReplace(nil)
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
	ma2mm := ma2p.MapReplace(func(v PropertyValue) (interface{}, bool) {
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
	ma3mm := ma3p.MapReplace(func(v PropertyValue) (interface{}, bool) {
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
