// Copyright 2016-2018, Pulumi Corporation.
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
	t.Parallel()

	md := New(nil)
	tree := map[string]interface{}{
		"b":  true,
		"s":  "hello",
		"f":  float64(3.14159265359),
		"ss": []string{"a", "b", "c"},
	}

	// Try some simple primitive decodes.
	var s bag
	err := md.DecodeValue(tree, reflect.TypeOf(bag{}), "b", &s.Bool, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["b"], s.Bool)
	err = md.DecodeValue(tree, reflect.TypeOf(bag{}), "b", &s.BoolP, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["b"], *s.BoolP)
	err = md.DecodeValue(tree, reflect.TypeOf(bag{}), "s", &s.String, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["s"], s.String)
	err = md.DecodeValue(tree, reflect.TypeOf(bag{}), "s", &s.StringP, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["s"], *s.StringP)
	err = md.DecodeValue(tree, reflect.TypeOf(bag{}), "f", &s.Float64, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["f"], s.Float64)
	err = md.DecodeValue(tree, reflect.TypeOf(bag{}), "f", &s.Float64P, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["f"], *s.Float64P)
	err = md.DecodeValue(tree, reflect.TypeOf(bag{}), "ss", &s.Strings, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["ss"], s.Strings)
	err = md.DecodeValue(tree, reflect.TypeOf(bag{}), "ss", &s.StringsP, false)
	assert.Nil(t, err)
	assert.Equal(t, tree["ss"], *s.StringsP)

	// Ensure interface{} conversions work:
	var sif string
	err = md.DecodeValue(map[string]interface{}{"x": interface{}("hello")},
		reflect.TypeOf(bag{}), "x", &sif, false)
	assert.Nil(t, err)
	assert.Equal(t, "hello", sif)

	var sifs []string
	err = md.DecodeValue(map[string]interface{}{"arr": []interface{}{"a", "b", "c"}},
		reflect.TypeOf(bag{}), "arr", &sifs, false)
	assert.Nil(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, sifs)

	// Ensure missing optional fields are ignored:
	s.String = "x"
	err = md.DecodeValue(tree, reflect.TypeOf(bag{}), "missing", &s.String, true)
	assert.Nil(t, err)
	assert.Equal(t, "x", s.String)

	// Try some error conditions; first, wrong type:
	s.String = "x"
	err = md.DecodeValue(tree, reflect.TypeOf(bag{}), "b", &s.String, false)
	assert.NotNil(t, err)
	assert.Equal(t, "Field 'b' on 'mapper.bag' must be a 'string'; got 'bool' instead", err.Error())
	assert.Equal(t, "x", s.String)

	// Next, missing required field:
	s.String = "x"
	err = md.DecodeValue(tree, reflect.TypeOf(bag{}), "missing", &s.String, false)
	assert.NotNil(t, err)
	assert.Equal(t, "Missing required field 'missing' on 'mapper.bag'", err.Error())
	assert.Equal(t, "x", s.String)
}

type bagtag struct {
	String        string `pulumi:"s"`
	StringSkip    string `pulumi:"sc,skip"`
	StringOpt     string `pulumi:"so,optional"`
	StringSkipOpt string `pulumi:"sco,skip,optional"`
}

func TestMapper(t *testing.T) {
	t.Parallel()

	var err error
	md := New(nil)

	// First, test the fully populated case.
	var b1 bagtag
	err = md.Decode(map[string]interface{}{
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
	err = md.Decode(map[string]interface{}{
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
	err = md.Decode(map[string]interface{}{
		"s":  true,
		"sc": "",
	}, &b3)
	assert.NotNil(t, err)
	assert.Equal(t, "1 failures decoding:\n"+
		"\ts: Field 's' on 'mapper.bagtag' must be a 'string'; got 'bool' instead", err.Error())
	assert.Equal(t, "", b3.String)

	// Next, missing required field:
	var b4 bagtag
	err = md.Decode(map[string]interface{}{}, &b4)
	assert.NotNil(t, err)
	assert.Equal(t, "1 failures decoding:\n"+
		"\ts: Missing required field 's' on 'mapper.bagtag'", err.Error())
	assert.Equal(t, "", b4.String)
}

type bog struct {
	Boggy    bogger     `pulumi:"boggy"`
	BoggyP   *bogger    `pulumi:"boggyp"`
	Boggers  []bogger   `pulumi:"boggers"`
	BoggersP *[]*bogger `pulumi:"boggersp"`
}

type bogger struct {
	Num float64 `pulumi:"num"`
}

func TestNestedMapper(t *testing.T) {
	t.Parallel()

	md := New(nil)

	// Test one level deep nesting (fields, arrays, pointers).
	var b bog
	err := md.Decode(map[string]interface{}{
		"boggy":  map[string]interface{}{"num": float64(99)},
		"boggyp": map[string]interface{}{"num": float64(180)},
		"boggers": []map[string]interface{}{
			{"num": float64(1)},
			{"num": float64(2)},
			{"num": float64(42)},
		},
		"boggersp": []map[string]interface{}{
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
	Bogs  map[string]bog   `pulumi:"bogs"`
	BogsP *map[string]*bog `pulumi:"bogsp"`
}

func TestMultiplyNestedMapper(t *testing.T) {
	t.Parallel()
	md := New(nil)

	// Test multilevel nesting (maps, fields, arrays, pointers).
	var ber boggerdybogger
	err := md.Decode(map[string]interface{}{
		"bogs": map[string]interface{}{
			"a": map[string]interface{}{
				"boggy":  map[string]interface{}{"num": float64(99)},
				"boggyp": map[string]interface{}{"num": float64(180)},
				"boggers": []map[string]interface{}{
					{"num": float64(1)},
					{"num": float64(2)},
					{"num": float64(42)},
				},
				"boggersp": []map[string]interface{}{
					{"num": float64(4)},
					{"num": float64(8)},
					{"num": float64(84)},
				},
			},
		},
		"bogsp": map[string]interface{}{
			"z": map[string]interface{}{
				"boggy":  map[string]interface{}{"num": float64(188)},
				"boggyp": map[string]interface{}{"num": float64(360)},
				"boggers": []map[string]interface{}{
					{"num": float64(2)},
					{"num": float64(4)},
					{"num": float64(84)},
				},
				"boggersp": []map[string]interface{}{
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
	Entries  map[string]mapentry  `pulumi:"entries"`
	EntriesP map[string]*mapentry `pulumi:"entriesp"`
}

type mapentry struct {
	Title string `pulumi:"title"`
}

func TestMapMapper(t *testing.T) {
	t.Parallel()

	md := New(nil)

	// Ensure we can decode both maps of structs and maps of pointers to structs.
	var hm hasmap
	err := md.Decode(map[string]interface{}{
		"entries": map[string]interface{}{
			"a": map[string]interface{}{"title": "first"},
			"b": map[string]interface{}{"title": "second"},
		},
		"entriesp": map[string]interface{}{
			"x": map[string]interface{}{"title": "firstp"},
			"y": map[string]interface{}{"title": "secondp"},
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
	C  customStruct    `pulumi:"c"`
	CI customInterface `pulumi:"ci"`
}

type customInterface interface {
	GetX() float64
	GetY() float64
}

type customStruct struct {
	X float64 `pulumi:"x"`
	Y float64 `pulumi:"y"`
}

func (s *customStruct) GetX() float64 { return s.X }
func (s *customStruct) GetY() float64 { return s.Y }

func TestCustomMapper(t *testing.T) {
	t.Parallel()

	md := New(&Opts{
		CustomDecoders: Decoders{
			reflect.TypeOf((*customInterface)(nil)).Elem(): decodeCustomInterface,
			reflect.TypeOf(customStruct{}):                 decodeCustomStruct,
		},
	})

	var w wrap
	err := md.Decode(map[string]interface{}{
		"c": map[string]interface{}{
			"x": float64(-99.2),
			"y": float64(127.127),
		},
		"ci": map[string]interface{}{
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

func decodeCustomInterface(m Mapper, tree map[string]interface{}) (interface{}, error) {
	var s customStruct
	if err := m.DecodeValue(tree, reflect.TypeOf(s), "x", &s.X, false); err != nil {
		return nil, err
	}
	if err := m.DecodeValue(tree, reflect.TypeOf(s), "y", &s.Y, false); err != nil {
		return nil, err
	}
	return customInterface(&s), nil
}

func decodeCustomStruct(m Mapper, tree map[string]interface{}) (interface{}, error) {
	var s customStruct
	if err := m.DecodeValue(tree, reflect.TypeOf(s), "x", &s.X, false); err != nil {
		return nil, err
	}
	if err := m.DecodeValue(tree, reflect.TypeOf(s), "y", &s.Y, false); err != nil {
		return nil, err
	}
	return s, nil
}

type outer struct {
	Inners *[]inner `pulumi:"inners,optional"`
}

type inner struct {
	A string   `pulumi:"a"`
	B *string  `pulumi:"b,optional"`
	C *string  `pulumi:"c,optional"`
	D float64  `pulumi:"d"`
	E *float64 `pulumi:"e,optional"`
	F *float64 `pulumi:"f,optional"`
	G *inner   `pulumi:"g,optional"`
	H *[]inner `pulumi:"h,optional"`
}

func TestBasicUnmap(t *testing.T) {
	t.Parallel()

	v2 := "v2"
	v5 := float64(5)
	i1v2 := "i1v2"
	i1v5 := float64(15)
	i2v2 := "i2v2"
	i2v5 := float64(25)
	i3v2 := "i3v2"
	i3v5 := float64(35)
	o := outer{
		Inners: &[]inner{
			{
				A: "v1",
				B: &v2,
				C: nil,
				D: float64(4),
				E: &v5,
				F: nil,
				G: &inner{
					A: "i1v1",
					B: &i1v2,
					C: nil,
					D: float64(14),
					E: &i1v5,
					F: nil,
					G: nil,
					H: nil,
				},
				H: &[]inner{
					{
						A: "i2v1",
						B: &i2v2,
						C: nil,
						D: float64(24),
						E: &i2v5,
						F: nil,
						G: nil,
						H: nil,
					},
					{
						A: "i3v1",
						B: &i3v2,
						C: nil,
						D: float64(34),
						E: &i3v5,
						F: nil,
						G: nil,
						H: nil,
					},
				},
			},
		},
	}

	// Unmap returns a JSON-like dictionary object representing the above structure.
	for _, e := range []interface{}{o, &o} {
		um, err := Unmap(e)
		assert.Nil(t, err)
		assert.NotNil(t, um)

		// check outer:
		assert.NotNil(t, um["inners"])
		arr := um["inners"].([]interface{})
		assert.Equal(t, len(arr), 1)

		// check outer.inner:
		inn := arr[0].(map[string]interface{})
		assert.Equal(t, inn["a"], "v1")
		assert.Equal(t, inn["b"], "v2")
		_, hasc := inn["c"]
		assert.False(t, hasc)
		assert.Equal(t, inn["d"], float64(4))
		assert.Equal(t, inn["e"], float64(5))
		_, hasf := inn["f"]
		assert.False(t, hasf)
		assert.NotNil(t, inn["g"])

		// check outer.inner.inner:
		inng := inn["g"].(map[string]interface{})
		assert.Equal(t, inng["a"], "i1v1")
		assert.Equal(t, inng["b"], "i1v2")
		_, hasgc := inng["c"]
		assert.False(t, hasgc)
		assert.Equal(t, inng["d"], float64(14))
		assert.Equal(t, inng["e"], float64(15))
		_, hasgf := inng["f"]
		assert.False(t, hasgf)
		_, hasgg := inng["g"]
		assert.False(t, hasgg)
		_, hasgh := inng["h"]
		assert.False(t, hasgh)

		// check outer.inner.inners[0]:
		innh := inn["h"].([]interface{})
		assert.Equal(t, len(innh), 2)
		innh0 := innh[0].(map[string]interface{})
		assert.Equal(t, innh0["a"], "i2v1")
		assert.Equal(t, innh0["b"], "i2v2")
		_, hash0c := inng["c"]
		assert.False(t, hash0c)
		assert.Equal(t, innh0["d"], float64(24))
		assert.Equal(t, innh0["e"], float64(25))
		_, hash0f := inng["f"]
		assert.False(t, hash0f)
		_, hash0g := inng["g"]
		assert.False(t, hash0g)
		_, hash0h := inng["h"]
		assert.False(t, hash0h)

		// check outer.inner.inners[1]:
		innh1 := innh[1].(map[string]interface{})
		assert.Equal(t, innh1["a"], "i3v1")
		assert.Equal(t, innh1["b"], "i3v2")
		_, hash1c := inng["c"]
		assert.False(t, hash1c)
		assert.Equal(t, innh1["d"], float64(34))
		assert.Equal(t, innh1["e"], float64(35))
		_, hash1f := inng["f"]
		assert.False(t, hash1f)
		_, hash1g := inng["g"]
		assert.False(t, hash1g)
		_, hash1h := inng["h"]
		assert.False(t, hash1h)
	}
}
