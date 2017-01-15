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
	Boggy    bogger     `json:"boggy"`
	BoggyP   *bogger    `json:"boggyp"`
	Boggers  []bogger   `json:"boggers"`
	BoggersP *[]*bogger `json:"boggersp"`
}

type bogger struct {
	Num float64 `json:"num"`
}

func TestNestedDecode(t *testing.T) {
	// Test one level deep nesting (fields, arrays, pointers).
	var b bog
	err := decode(object{
		"boggy":  object{"num": float64(99)},
		"boggyp": object{"num": float64(180)},
		"boggers": []object{
			{"num": float64(1)},
			{"num": float64(2)},
			{"num": float64(42)},
		},
		"boggersp": []object{
			{"num": float64(4)},
			{"num": float64(8)},
			{"num": float64(84)},
		},
	}, &b)
	assert.Nil(t, err)
	assert.Equal(t, float64(99), b.Boggy.Num)
	assert.NotNil(t, b.BoggyP)
	assert.Equal(t, float64(180), b.BoggyP.Num)
	assert.Equal(t, 3, len(b.Boggers))
	assert.Equal(t, float64(1), b.Boggers[0].Num)
	assert.Equal(t, float64(2), b.Boggers[1].Num)
	assert.Equal(t, float64(42), b.Boggers[2].Num)
	assert.NotNil(t, b.BoggersP)
	assert.Equal(t, 3, len(*b.BoggersP))
	assert.NotNil(t, (*b.BoggersP)[0])
	assert.Equal(t, float64(4), (*b.BoggersP)[0].Num)
	assert.NotNil(t, (*b.BoggersP)[1])
	assert.Equal(t, float64(8), (*b.BoggersP)[1].Num)
	assert.NotNil(t, (*b.BoggersP)[2])
	assert.Equal(t, float64(84), (*b.BoggersP)[2].Num)
}

type boggerdybogger struct {
	Bogs  map[string]bog   `json:"bogs"`
	BogsP *map[string]*bog `json:"bogsp"`
}

func TestMultiplyNestedDecode(t *testing.T) {
	// Test multilevel nesting (maps, fields, arrays, pointers).
	var ber boggerdybogger
	err := decode(object{
		"bogs": object{
			"a": object{
				"boggy":  object{"num": float64(99)},
				"boggyp": object{"num": float64(180)},
				"boggers": []object{
					{"num": float64(1)},
					{"num": float64(2)},
					{"num": float64(42)},
				},
				"boggersp": []object{
					{"num": float64(4)},
					{"num": float64(8)},
					{"num": float64(84)},
				},
			},
		},
		"bogsp": object{
			"z": object{
				"boggy":  object{"num": float64(188)},
				"boggyp": object{"num": float64(360)},
				"boggers": []object{
					{"num": float64(2)},
					{"num": float64(4)},
					{"num": float64(84)},
				},
				"boggersp": []object{
					{"num": float64(8)},
					{"num": float64(16)},
					{"num": float64(168)},
				},
			},
		},
	}, &ber)
	assert.Nil(t, err)

	assert.Equal(t, 1, len(ber.Bogs))
	b := ber.Bogs["a"]
	assert.Equal(t, float64(99), b.Boggy.Num)
	assert.NotNil(t, b.BoggyP)
	assert.Equal(t, float64(180), b.BoggyP.Num)
	assert.Equal(t, 3, len(b.Boggers))
	assert.Equal(t, float64(1), b.Boggers[0].Num)
	assert.Equal(t, float64(2), b.Boggers[1].Num)
	assert.Equal(t, float64(42), b.Boggers[2].Num)
	assert.NotNil(t, b.BoggersP)
	assert.Equal(t, 3, len(*b.BoggersP))
	assert.NotNil(t, (*b.BoggersP)[0])
	assert.Equal(t, float64(4), (*b.BoggersP)[0].Num)
	assert.NotNil(t, (*b.BoggersP)[1])
	assert.Equal(t, float64(8), (*b.BoggersP)[1].Num)
	assert.NotNil(t, (*b.BoggersP)[2])
	assert.Equal(t, float64(84), (*b.BoggersP)[2].Num)

	assert.NotNil(t, ber.BogsP)
	assert.Equal(t, 1, len(*ber.BogsP))
	p := (*ber.BogsP)["z"]
	assert.NotNil(t, p)
	assert.Equal(t, float64(188), p.Boggy.Num)
	assert.NotNil(t, p.BoggyP)
	assert.Equal(t, float64(360), p.BoggyP.Num)
	assert.Equal(t, 3, len(p.Boggers))
	assert.Equal(t, float64(2), p.Boggers[0].Num)
	assert.Equal(t, float64(4), p.Boggers[1].Num)
	assert.Equal(t, float64(84), p.Boggers[2].Num)
	assert.NotNil(t, p.BoggersP)
	assert.Equal(t, 3, len(*p.BoggersP))
	assert.NotNil(t, (*p.BoggersP)[0])
	assert.Equal(t, float64(8), (*p.BoggersP)[0].Num)
	assert.NotNil(t, (*p.BoggersP)[1])
	assert.Equal(t, float64(16), (*p.BoggersP)[1].Num)
	assert.NotNil(t, (*p.BoggersP)[2])
	assert.Equal(t, float64(168), (*p.BoggersP)[2].Num)
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
