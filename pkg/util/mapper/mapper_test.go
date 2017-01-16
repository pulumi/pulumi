// Copyright 2016 Marapongo, Inc. All rights reserved.

package mapper

import (
	"reflect"
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

func TestFieldMapper(t *testing.T) {
	md := New(nil)
	tree := Object{
		"b":  true,
		"s":  "hello",
		"f":  float64(3.14159265359),
		"ss": []string{"a", "b", "c"},
	}

	// Try some simple primitive decodes.
	var s bag
	var err error
	err = md.DecodeField(tree, reflect.TypeOf(bag{}), "b", &s.Bool, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["b"], s.Bool)
	err = md.DecodeField(tree, reflect.TypeOf(bag{}), "b", &s.BoolP, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["b"], *s.BoolP)
	err = md.DecodeField(tree, reflect.TypeOf(bag{}), "s", &s.String, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["s"], s.String)
	err = md.DecodeField(tree, reflect.TypeOf(bag{}), "s", &s.StringP, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["s"], *s.StringP)
	err = md.DecodeField(tree, reflect.TypeOf(bag{}), "f", &s.Float64, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["f"], s.Float64)
	err = md.DecodeField(tree, reflect.TypeOf(bag{}), "f", &s.Float64P, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["f"], *s.Float64P)
	err = md.DecodeField(tree, reflect.TypeOf(bag{}), "ss", &s.Strings, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["ss"], s.Strings)
	err = md.DecodeField(tree, reflect.TypeOf(bag{}), "ss", &s.StringsP, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["ss"], *s.StringsP)

	// Ensure interface{} conversions work:
	var sif string
	err = md.DecodeField(Object{"x": interface{}("hello")}, reflect.TypeOf(bag{}), "x", &sif, false)
	assert.Nil(t, err)
	assert.Equal(t, "hello", sif)

	var sifs []string
	err = md.DecodeField(Object{"arr": []interface{}{"a", "b", "c"}}, reflect.TypeOf(bag{}), "arr", &sifs, false)
	assert.Nil(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, sifs)

	// Ensure missing optional fields are ignored:
	s.String = "x"
	err = md.DecodeField(tree, reflect.TypeOf(bag{}), "missing", &s.String, true)
	assert.Nil(t, err)
	assert.Equal(t, "x", s.String)

	// Try some error conditions; first, wrong type:
	s.String = "x"
	err = md.DecodeField(tree, reflect.TypeOf(bag{}), "b", &s.String, false)
	assert.NotNil(t, err)
	assert.Equal(t, "mapper.bag `b` must be a `string`, got `bool`", err.Error())
	assert.Equal(t, "x", s.String)

	// Next, missing required field:
	s.String = "x"
	err = md.DecodeField(tree, reflect.TypeOf(bag{}), "missing", &s.String, false)
	assert.NotNil(t, err)
	assert.Equal(t, "Missing required mapper.bag field `missing`", err.Error())
	assert.Equal(t, "x", s.String)
}

type bagtag struct {
	String        string `json:"s"`
	StringSkip    string `json:"sc,skip"`
	StringOpt     string `json:"so,omitempty"`
	StringSkipOpt string `json:"sco,skip,omitempty"`
}

func TestMapper(t *testing.T) {
	var err error
	md := New(nil)

	// First, test the fully populated case.
	var b1 bagtag
	err = md.Decode(Object{
		"s":   "something",
		"sc":  "nothing",
		"so":  "ohmy",
		"sco": "ohmynada",
	}, &b1)
	assert.Nil(t, err)
	assert.Equal(t, "something", b1.String)
	assert.Equal(t, "", b1.StringSkip)
	assert.Equal(t, "ohmy", b1.StringOpt)
	assert.Equal(t, "", b1.StringSkipOpt)

	// Now let optional fields go missing.
	var b2 bagtag
	err = md.Decode(Object{
		"s":  "something",
		"sc": "nothing",
	}, &b2)
	assert.Nil(t, err)
	assert.Equal(t, "something", b2.String)
	assert.Equal(t, "", b2.StringSkip)
	assert.Equal(t, "", b2.StringOpt)
	assert.Equal(t, "", b2.StringSkipOpt)

	// Try some error conditions; first, wrong type:
	var b3 bagtag
	err = md.Decode(Object{
		"s":  true,
		"sc": "",
	}, &b3)
	assert.NotNil(t, err)
	assert.Equal(t, "mapper.bagtag `s` must be a `string`, got `bool`", err.Error())
	assert.Equal(t, "", b3.String)

	// Next, missing required field:
	var b4 bagtag
	err = md.Decode(Object{}, &b4)
	assert.NotNil(t, err)
	assert.Equal(t, "Missing required mapper.bagtag field `s`", err.Error())
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

func TestNestedMapper(t *testing.T) {
	md := New(nil)

	// Test one level deep nesting (fields, arrays, pointers).
	var b bog
	err := md.Decode(Object{
		"boggy":  Object{"num": float64(99)},
		"boggyp": Object{"num": float64(180)},
		"boggers": []Object{
			{"num": float64(1)},
			{"num": float64(2)},
			{"num": float64(42)},
		},
		"boggersp": []Object{
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

func TestMultiplyNestedMapper(t *testing.T) {
	md := New(nil)

	// Test multilevel nesting (maps, fields, arrays, pointers).
	var ber boggerdybogger
	err := md.Decode(Object{
		"bogs": Object{
			"a": Object{
				"boggy":  Object{"num": float64(99)},
				"boggyp": Object{"num": float64(180)},
				"boggers": []Object{
					{"num": float64(1)},
					{"num": float64(2)},
					{"num": float64(42)},
				},
				"boggersp": []Object{
					{"num": float64(4)},
					{"num": float64(8)},
					{"num": float64(84)},
				},
			},
		},
		"bogsp": Object{
			"z": Object{
				"boggy":  Object{"num": float64(188)},
				"boggyp": Object{"num": float64(360)},
				"boggers": []Object{
					{"num": float64(2)},
					{"num": float64(4)},
					{"num": float64(84)},
				},
				"boggersp": []Object{
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

func TestMapMapper(t *testing.T) {
	md := New(nil)

	// Ensure we can decode both maps of structs and maps of pointers to structs.
	var hm hasmap
	err := md.Decode(Object{
		"entries": Object{
			"a": Object{"title": "first"},
			"b": Object{"title": "second"},
		},
		"entriesp": Object{
			"x": Object{"title": "firstp"},
			"y": Object{"title": "secondp"},
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

type wrap struct {
	C  customStruct    `json:"c"`
	CI customInterface `json:"ci"`
}

type customInterface interface {
	GetX() float64
	GetY() float64
}

type customStruct struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (s *customStruct) GetX() float64 { return s.X }
func (s *customStruct) GetY() float64 { return s.Y }

func TestCustomMapper(t *testing.T) {
	var md Mapper
	md = New(Decoders{
		reflect.TypeOf((*customInterface)(nil)).Elem(): decodeCustomInterface,
		reflect.TypeOf(customStruct{}):                 decodeCustomStruct,
	})

	var w wrap
	err := md.Decode(Object{
		"c": Object{
			"x": float64(-99.2),
			"y": float64(127.127),
		},
		"ci": Object{
			"x": float64(42.6),
			"y": float64(247.9),
		},
	}, &w)
	assert.Nil(t, err)
	assert.Equal(t, float64(-99.2), w.C.X)
	assert.Equal(t, float64(127.127), w.C.Y)
	assert.NotNil(t, w.CI)
	assert.Equal(t, float64(42.6), w.CI.GetX())
	assert.Equal(t, float64(247.9), w.CI.GetY())
}

func decodeCustomInterface(m Mapper, tree Object) (interface{}, error) {
	var s customStruct
	if err := m.DecodeField(tree, reflect.TypeOf(s), "x", &s.X, false); err != nil {
		return nil, err
	}
	if err := m.DecodeField(tree, reflect.TypeOf(s), "y", &s.Y, false); err != nil {
		return nil, err
	}
	return customInterface(&s), nil
}

func decodeCustomStruct(m Mapper, tree Object) (interface{}, error) {
	var s customStruct
	if err := m.DecodeField(tree, reflect.TypeOf(s), "x", &s.X, false); err != nil {
		return nil, err
	}
	if err := m.DecodeField(tree, reflect.TypeOf(s), "y", &s.Y, false); err != nil {
		return nil, err
	}
	return s, nil
}
