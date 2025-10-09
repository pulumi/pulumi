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
	"github.com/stretchr/testify/require"
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
	tree := map[string]any{
		"b":  true,
		"s":  "hello",
		"f":  float64(3.14159265359),
		"ss": []string{"a", "b", "c"},
	}

	// Try some simple primitive decodes.
	var s bag
	err := md.DecodeValue(tree, reflect.TypeOf(bag{}), "b", &s.Bool, false)
	require.NoError(t, err)
	assert.Equal(t, tree["b"], s.Bool)
	err = md.DecodeValue(tree, reflect.TypeOf(bag{}), "b", &s.BoolP, false)
	require.NoError(t, err)
	assert.Equal(t, tree["b"], *s.BoolP)
	err = md.DecodeValue(tree, reflect.TypeOf(bag{}), "s", &s.String, false)
	require.NoError(t, err)
	assert.Equal(t, tree["s"], s.String)
	err = md.DecodeValue(tree, reflect.TypeOf(bag{}), "s", &s.StringP, false)
	require.NoError(t, err)
	assert.Equal(t, tree["s"], *s.StringP)
	err = md.DecodeValue(tree, reflect.TypeOf(bag{}), "f", &s.Float64, false)
	require.NoError(t, err)
	assert.Equal(t, tree["f"], s.Float64)
	err = md.DecodeValue(tree, reflect.TypeOf(bag{}), "f", &s.Float64P, false)
	require.NoError(t, err)
	assert.Equal(t, tree["f"], *s.Float64P)
	err = md.DecodeValue(tree, reflect.TypeOf(bag{}), "ss", &s.Strings, false)
	require.NoError(t, err)
	assert.Equal(t, tree["ss"], s.Strings)
	err = md.DecodeValue(tree, reflect.TypeOf(bag{}), "ss", &s.StringsP, false)
	require.NoError(t, err)
	assert.Equal(t, tree["ss"], *s.StringsP)

	// Ensure any conversions work:
	var sif string
	err = md.DecodeValue(map[string]any{"x": any("hello")},
		reflect.TypeOf(bag{}), "x", &sif, false)
	require.NoError(t, err)
	assert.Equal(t, "hello", sif)

	var sifs []string
	err = md.DecodeValue(map[string]any{"arr": []any{"a", "b", "c"}},
		reflect.TypeOf(bag{}), "arr", &sifs, false)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, sifs)

	// Ensure missing optional fields are ignored:
	s.String = "x"
	err = md.DecodeValue(tree, reflect.TypeOf(bag{}), "missing", &s.String, true)
	require.NoError(t, err)
	assert.Equal(t, "x", s.String)

	// Try some error conditions; first, wrong type:
	s.String = "x"
	err = md.DecodeValue(tree, reflect.TypeOf(bag{}), "b", &s.String, false)
	assert.EqualError(t, err, "Field 'b' on 'mapper.bag' must be a 'string'; got 'bool' instead")
	assert.Equal(t, "x", s.String)

	// Next, missing required field:
	s.String = "x"
	err = md.DecodeValue(tree, reflect.TypeOf(bag{}), "missing", &s.String, false)
	assert.EqualError(t, err, "Missing required field 'missing' on 'mapper.bag'")
	assert.Equal(t, "x", s.String)
}

type bagtag struct {
	String        string         `pulumi:"s"`
	StringSkip    string         `pulumi:"sc,skip"`
	StringOpt     string         `pulumi:"so,optional"`
	StringSkipOpt string         `pulumi:"sco,skip,optional"`
	MapOpt        map[string]any `pulumi:"mo,optional"`
}

type AnInterface interface {
	isAnInterface()
}

func TestMapperEncode(t *testing.T) {
	t.Parallel()
	bag := bagtag{
		String:    "something",
		StringOpt: "ohmv",
		MapOpt: map[string]any{
			"a": "something",
			"b": nil,
		},
	}

	md := &mapper{}
	var err error
	var m map[string]any

	// Nils
	m, err = md.Encode(nil)
	require.NoError(t, err)
	require.Len(t, m, 0)

	// Nil (interface)
	m, err = md.Encode((AnInterface)(nil))
	require.NoError(t, err)
	require.Len(t, m, 0)

	// Structs
	m, err = md.encode(reflect.ValueOf(bag))
	require.NoError(t, err)
	assert.Equal(t, "something", m["s"])
	assert.Equal(t, "ohmv", m["so"])
	assert.Equal(t, map[string]any{"a": "something", "b": nil}, m["mo"])

	// Pointers
	m, err = md.encode(reflect.Zero(reflect.TypeOf(&bag)))
	require.NoError(t, err)
	assert.Nil(t, m)
	m, err = md.encode(reflect.ValueOf(&bag))
	require.NoError(t, err)
	assert.Equal(t, "something", m["s"])
	assert.Equal(t, "ohmv", m["so"])
	assert.Equal(t, map[string]any{"a": "something", "b": nil}, m["mo"])
}

func TestMapperEncodeValue(t *testing.T) {
	t.Parallel()
	strdata := "something"
	bag := bagtag{
		String:    "something",
		StringOpt: "ohmv",
	}
	slice := []string{"something"}
	mapdata := map[string]any{
		"a": "something",
		"b": nil,
	}
	anyType := reflect.TypeOf((*any)(nil)).Elem()
	assert.Equal(t, reflect.Interface, anyType.Kind())

	md := &mapper{}
	var err error
	var v any

	// Nils
	v, err = md.EncodeValue(nil)
	require.NoError(t, err)
	assert.Nil(t, v)

	// Bools
	v, err = md.encodeValue(reflect.ValueOf(true))
	require.NoError(t, err)
	assert.Equal(t, true, v)

	// Ints
	v, err = md.encodeValue(reflect.ValueOf(int(1)))
	require.NoError(t, err)
	assert.Equal(t, float64(1), v)

	// Uints
	v, err = md.encodeValue(reflect.ValueOf(uint(1)))
	require.NoError(t, err)
	assert.Equal(t, float64(1), v)

	// Floats
	v, err = md.encodeValue(reflect.ValueOf(float32(1.0)))
	require.NoError(t, err)
	assert.Equal(t, float64(1.0), v)

	// Pointers
	v, err = md.encodeValue(reflect.Zero(reflect.TypeOf(&strdata)))
	require.NoError(t, err)
	assert.Nil(t, v)
	v, err = md.encodeValue(reflect.ValueOf(&strdata))
	require.NoError(t, err)
	assert.Equal(t, "something", v)

	// Slices
	v, err = md.encodeValue(reflect.Zero(reflect.TypeOf(slice)))
	require.NoError(t, err)
	assert.Nil(t, v)
	v, err = md.encodeValue(reflect.ValueOf(slice))
	require.NoError(t, err)
	assert.Equal(t, []any{"something"}, v)

	// Maps
	v, err = md.encodeValue(reflect.Zero(reflect.TypeOf(mapdata)))
	require.NoError(t, err)
	assert.Nil(t, v)
	v, err = md.encodeValue(reflect.ValueOf(mapdata))
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"a": "something", "b": nil}, v)

	// Structs
	v, err = md.encodeValue(reflect.ValueOf(bag))
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"s": "something", "so": "ohmv"}, v)

	// Interfaces
	v, err = md.encodeValue(reflect.Zero(anyType))
	require.NoError(t, err)
	assert.Nil(t, v)
	v, err = md.encodeValue(reflect.ValueOf("something").Convert(anyType))
	require.NoError(t, err)
	assert.Equal(t, "something", v)
}

func TestMapperDecode(t *testing.T) {
	t.Parallel()

	var err error
	md := New(nil)

	// First, test the fully populated case.
	var b1 bagtag
	err = md.Decode(map[string]any{
		"s":   "something",
		"sc":  "nothing",
		"so":  "ohmy",
		"sco": "ohmynada",
		"mo": map[string]any{
			"a": "something",
			"b": nil,
		},
	}, &b1)
	require.NoError(t, err)
	assert.Equal(t, "something", b1.String)
	assert.Equal(t, "", b1.StringSkip)
	assert.Equal(t, "ohmy", b1.StringOpt)
	assert.Equal(t, "", b1.StringSkipOpt)
	assert.Equal(t, map[string]any{"a": "something", "b": nil}, b1.MapOpt)

	// Now let optional fields go missing.
	var b2 bagtag
	err = md.Decode(map[string]any{
		"s":  "something",
		"sc": "nothing",
	}, &b2)
	require.NoError(t, err)
	assert.Equal(t, "something", b2.String)
	assert.Equal(t, "", b2.StringSkip)
	assert.Equal(t, "", b2.StringOpt)
	assert.Equal(t, "", b2.StringSkipOpt)

	// Try some error conditions; first, wrong type:
	var b3 bagtag
	err = md.Decode(map[string]any{
		"s":  true,
		"sc": "",
	}, &b3)
	assert.EqualError(t, err, "1 failures decoding:\n"+
		"\ts: Field 's' on 'mapper.bagtag' must be a 'string'; got 'bool' instead")
	assert.Equal(t, "", b3.String)

	// Next, missing required field:
	var b4 bagtag
	err = md.Decode(map[string]any{}, &b4)
	assert.EqualError(t, err, "1 failures decoding:\n"+
		"\ts: Missing required field 's' on 'mapper.bagtag'")
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
	err := md.Decode(map[string]any{
		"boggy":  map[string]any{"num": float64(99)},
		"boggyp": map[string]any{"num": float64(180)},
		"boggers": []map[string]any{
			{"num": float64(1)},
			{"num": float64(2)},
			{"num": float64(42)},
		},
		"boggersp": []map[string]any{
			{"num": float64(4)},
			{"num": float64(8)},
			{"num": float64(84)},
		},
	}, &b)
	require.NoError(t, err)
	assert.Equal(t, float64(99), b.Boggy.Num)
	require.NotNil(t, b.BoggyP)
	assert.Equal(t, float64(180), b.BoggyP.Num)
	require.Len(t, b.Boggers, 3)
	assert.Equal(t, float64(1), b.Boggers[0].Num)
	assert.Equal(t, float64(2), b.Boggers[1].Num)
	assert.Equal(t, float64(42), b.Boggers[2].Num)
	require.NotNil(t, b.BoggersP)
	require.Len(t, *b.BoggersP, 3)
	require.NotNil(t, (*b.BoggersP)[0])
	assert.Equal(t, float64(4), (*b.BoggersP)[0].Num)
	require.NotNil(t, (*b.BoggersP)[1])
	assert.Equal(t, float64(8), (*b.BoggersP)[1].Num)
	require.NotNil(t, (*b.BoggersP)[2])
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
	err := md.Decode(map[string]any{
		"bogs": map[string]any{
			"a": map[string]any{
				"boggy":  map[string]any{"num": float64(99)},
				"boggyp": map[string]any{"num": float64(180)},
				"boggers": []map[string]any{
					{"num": float64(1)},
					{"num": float64(2)},
					{"num": float64(42)},
				},
				"boggersp": []map[string]any{
					{"num": float64(4)},
					{"num": float64(8)},
					{"num": float64(84)},
				},
			},
		},
		"bogsp": map[string]any{
			"z": map[string]any{
				"boggy":  map[string]any{"num": float64(188)},
				"boggyp": map[string]any{"num": float64(360)},
				"boggers": []map[string]any{
					{"num": float64(2)},
					{"num": float64(4)},
					{"num": float64(84)},
				},
				"boggersp": []map[string]any{
					{"num": float64(8)},
					{"num": float64(16)},
					{"num": float64(168)},
				},
			},
		},
	}, &ber)
	require.NoError(t, err)

	require.Len(t, ber.Bogs, 1)
	b := ber.Bogs["a"]
	assert.Equal(t, float64(99), b.Boggy.Num)
	require.NotNil(t, b.BoggyP)
	assert.Equal(t, float64(180), b.BoggyP.Num)
	require.Len(t, b.Boggers, 3)
	assert.Equal(t, float64(1), b.Boggers[0].Num)
	assert.Equal(t, float64(2), b.Boggers[1].Num)
	assert.Equal(t, float64(42), b.Boggers[2].Num)
	require.NotNil(t, b.BoggersP)
	require.Len(t, *b.BoggersP, 3)
	require.NotNil(t, (*b.BoggersP)[0])
	assert.Equal(t, float64(4), (*b.BoggersP)[0].Num)
	require.NotNil(t, (*b.BoggersP)[1])
	assert.Equal(t, float64(8), (*b.BoggersP)[1].Num)
	require.NotNil(t, (*b.BoggersP)[2])
	assert.Equal(t, float64(84), (*b.BoggersP)[2].Num)

	require.NotNil(t, ber.BogsP)
	require.Len(t, *ber.BogsP, 1)
	p := (*ber.BogsP)["z"]
	require.NotNil(t, p)
	assert.Equal(t, float64(188), p.Boggy.Num)
	require.NotNil(t, p.BoggyP)
	assert.Equal(t, float64(360), p.BoggyP.Num)
	require.Len(t, p.Boggers, 3)
	assert.Equal(t, float64(2), p.Boggers[0].Num)
	assert.Equal(t, float64(4), p.Boggers[1].Num)
	assert.Equal(t, float64(84), p.Boggers[2].Num)
	require.NotNil(t, p.BoggersP)
	require.Len(t, *p.BoggersP, 3)
	require.NotNil(t, (*p.BoggersP)[0])
	assert.Equal(t, float64(8), (*p.BoggersP)[0].Num)
	require.NotNil(t, (*p.BoggersP)[1])
	assert.Equal(t, float64(16), (*p.BoggersP)[1].Num)
	require.NotNil(t, (*p.BoggersP)[2])
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
	err := md.Decode(map[string]any{
		"entries": map[string]any{
			"a": map[string]any{"title": "first"},
			"b": map[string]any{"title": "second"},
		},
		"entriesp": map[string]any{
			"x": map[string]any{"title": "firstp"},
			"y": map[string]any{"title": "secondp"},
		},
	}, &hm)
	require.NoError(t, err)
	require.Len(t, hm.Entries, 2)
	assert.Equal(t, "first", hm.Entries["a"].Title)
	assert.Equal(t, "second", hm.Entries["b"].Title)
	require.Len(t, hm.EntriesP, 2)
	require.NotNil(t, hm.EntriesP["x"])
	require.NotNil(t, hm.EntriesP["y"])
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
	err := md.Decode(map[string]any{
		"c": map[string]any{
			"x": float64(-99.2),
			"y": float64(127.127),
		},
		"ci": map[string]any{
			"x": float64(42.6),
			"y": float64(247.9),
		},
	}, &w)
	require.NoError(t, err)
	assert.Equal(t, float64(-99.2), w.C.X)
	assert.Equal(t, float64(127.127), w.C.Y)
	require.NotNil(t, w.CI)
	assert.Equal(t, float64(42.6), w.CI.GetX())
	assert.Equal(t, float64(247.9), w.CI.GetY())
}

func decodeCustomInterface(m Mapper, tree map[string]any) (any, error) {
	var s customStruct
	if err := m.DecodeValue(tree, reflect.TypeOf(s), "x", &s.X, false); err != nil {
		return nil, err
	}
	if err := m.DecodeValue(tree, reflect.TypeOf(s), "y", &s.Y, false); err != nil {
		return nil, err
	}
	return customInterface(&s), nil
}

func decodeCustomStruct(m Mapper, tree map[string]any) (any, error) {
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
	for _, e := range []any{o, &o} {
		um, err := Unmap(e)
		require.NoError(t, err)
		require.NotNil(t, um)

		// check outer:
		require.NotNil(t, um["inners"])
		arr := um["inners"].([]any)
		assert.Equal(t, len(arr), 1)

		// check outer.inner:
		inn := arr[0].(map[string]any)
		assert.Equal(t, inn["a"], "v1")
		assert.Equal(t, inn["b"], "v2")
		_, hasc := inn["c"]
		assert.False(t, hasc)
		assert.Equal(t, inn["d"], float64(4))
		assert.Equal(t, inn["e"], float64(5))
		_, hasf := inn["f"]
		assert.False(t, hasf)
		require.NotNil(t, inn["g"])

		// check outer.inner.inner:
		inng := inn["g"].(map[string]any)
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
		innh := inn["h"].([]any)
		assert.Equal(t, len(innh), 2)
		innh0 := innh[0].(map[string]any)
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
		innh1 := innh[1].(map[string]any)
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

func TestReproduceMapStringPointerTurnaroundIssue(t *testing.T) {
	t.Parallel()

	type X struct {
		Args map[string]*string `pulumi:"args,optional"`
	}

	xToMap := func(build X) (map[string]any, error) {
		m, err := New(nil).Encode(build)
		if err != nil {
			return nil, err
		}
		return m, nil
	}

	xFromMap := func(pm map[string]any) (X, error) {
		var build X
		err := New(nil).Decode(pm, &build)
		if err != nil {
			return X{}, err
		}
		return build, nil
	}

	value := "value"
	expected := X{
		Args: map[string]*string{
			"key": &value,
		},
	}

	encodedMap, err := xToMap(expected)
	require.NoError(t, err)
	t.Logf("encodedMap: %v", encodedMap)

	back, err2 := xFromMap(encodedMap)
	require.NoError(t, err2)

	require.Equal(t, expected, back)
}
