// Copyright 2016 Marapongo, Inc. All rights reserved.

package encoding

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type bag struct {
	Bool     bool
	BoolP    *bool
	String   string
	StringP  *string
	Float64  float64
	Float64P *float64
	Strings  []string
	StringsP *[]string
}

func TestDecodeField(t *testing.T) {
	tree := object{
		"b":  true,
		"s":  "hello",
		"f":  float64(3.14159265359),
		"ss": []string{"a", "b", "c"},
	}

	// Try some simple primitive decodes.
	var s bag
	var err error
	err = decodeField(tree, "bag", "b", &s.Bool, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["b"], s.Bool)
	err = decodeField(tree, "bag", "b", &s.BoolP, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["b"], *s.BoolP)
	err = decodeField(tree, "bag", "s", &s.String, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["s"], s.String)
	err = decodeField(tree, "bag", "s", &s.StringP, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["s"], *s.StringP)
	err = decodeField(tree, "bag", "f", &s.Float64, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["f"], s.Float64)
	err = decodeField(tree, "bag", "f", &s.Float64P, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["f"], *s.Float64P)
	err = decodeField(tree, "bag", "ss", &s.Strings, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["ss"], s.Strings)
	err = decodeField(tree, "bag", "ss", &s.StringsP, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["ss"], *s.StringsP)

	// Ensure interface{} conversions work:
	var sif string
	err = decodeField(object{"x": interface{}("hello")}, "bag", "x", &sif, false)
	assert.Nil(t, err)
	assert.Equal(t, "hello", sif)

	var sifs []string
	err = decodeField(object{"arr": []interface{}{"a", "b", "c"}}, "bag", "arr", &sifs, false)
	assert.Nil(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, sifs)

	// Ensure missing optional fields are ignored:
	s.String = "x"
	err = decodeField(tree, "bag", "missing", &s.String, true)
	assert.Nil(t, err)
	assert.Equal(t, "x", s.String)

	// Try some error conditions; first, wrong type:
	s.String = "x"
	err = decodeField(tree, "bag", "b", &s.String, false)
	assert.NotNil(t, err)
	assert.Equal(t, "bag `b` must be a `string`, got `bool`", err.Error())
	assert.Equal(t, "x", s.String)

	// Next, missing required field:
	s.String = "x"
	err = decodeField(tree, "bag", "missing", &s.String, false)
	assert.NotNil(t, err)
	assert.Equal(t, "Missing required bag field `missing`", err.Error())
	assert.Equal(t, "x", s.String)
}

type bagtag struct {
	String        string `json:"s"`
	StringCust    string `json:"sc,custom"`
	StringOpt     string `json:"so,omitempty"`
	StringCustOpt string `json:"sco,custom,omitempty"`
}

func TestDecode(t *testing.T) {
	var err error

	// First, test the fully populated case.
	var b1 bagtag
	err = decode(object{
		"s":   "something",
		"sc":  "nothing",
		"so":  "ohmy",
		"sco": "ohmynada",
	}, &b1)
	assert.Nil(t, err)
	assert.Equal(t, "something", b1.String)
	assert.Equal(t, "", b1.StringCust)
	assert.Equal(t, "ohmy", b1.StringOpt)
	assert.Equal(t, "", b1.StringCustOpt)

	// Now let optional fields go missing.
	var b2 bagtag
	err = decode(object{
		"s":  "something",
		"sc": "nothing",
	}, &b2)
	assert.Nil(t, err)
	assert.Equal(t, "something", b2.String)
	assert.Equal(t, "", b2.StringCust)
	assert.Equal(t, "", b2.StringOpt)
	assert.Equal(t, "", b2.StringCustOpt)

	// Try some error conditions; first, wrong type:
	var b3 bagtag
	err = decode(object{
		"s":  true,
		"sc": "",
	}, &b3)
	assert.NotNil(t, err)
	assert.Equal(t, "bagtag `s` must be a `string`, got `bool`", err.Error())
	assert.Equal(t, "", b3.String)

	// Next, missing required field:
	var b4 bagtag
	err = decode(object{}, &b4)
	assert.NotNil(t, err)
	assert.Equal(t, "Missing required bagtag field `s`", err.Error())
	assert.Equal(t, "", b4.String)
}

type bog struct {
	Boggers []bogger `json:"boggers"`
}

type bogger struct {
	Num float64 `json:"num"`
}

func TestNestedDecode(t *testing.T) {
	var b bog
	err := decode(object{
		"boggers": []object{
			{"num": float64(1)},
			{"num": float64(2)},
			{"num": float64(42)},
		},
	}, &b)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(b.Boggers))
	assert.Equal(t, float64(1), b.Boggers[0].Num)
	assert.Equal(t, float64(2), b.Boggers[1].Num)
	assert.Equal(t, float64(42), b.Boggers[2].Num)
}

type hasmap struct {
	Entries  map[string]mapentry  `json:"entries"`
	EntriesP map[string]*mapentry `json:"entriesp"`
}

type mapentry struct {
	Title string `json:"title"`
}

func TestMapDecode(t *testing.T) {
	// Ensure we can decode both maps of structs and maps of pointers to structs.
	var hm hasmap
	err := decode(object{
		"entries": object{
			"a": object{"title": "first"},
			"b": object{"title": "second"},
		},
		"entriesp": object{
			"x": object{"title": "firstp"},
			"y": object{"title": "secondp"},
		},
	}, &hm)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(hm.Entries))
	assert.Equal(t, "first", hm.Entries["a"].Title)
	assert.Equal(t, "second", hm.Entries["b"].Title)
	assert.Equal(t, 2, len(hm.EntriesP))
	assert.NotNil(t, hm.EntriesP["x"])
	assert.NotNil(t, hm.EntriesP["y"])
	assert.Equal(t, "firstp", hm.EntriesP["x"].Title)
	assert.Equal(t, "secondp", hm.EntriesP["y"].Title)
}
