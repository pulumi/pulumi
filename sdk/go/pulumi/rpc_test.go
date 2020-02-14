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

// nolint: unused,deadcode
package pulumi

import (
	"reflect"
	"testing"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/stretchr/testify/assert"
)

type test struct {
	S             string                 `pulumi:"s"`
	A             bool                   `pulumi:"a"`
	B             int                    `pulumi:"b"`
	StringAsset   Asset                  `pulumi:"cStringAsset"`
	FileAsset     Asset                  `pulumi:"cFileAsset"`
	RemoteAsset   Asset                  `pulumi:"cRemoteAsset"`
	AssetArchive  Archive                `pulumi:"dAssetArchive"`
	FileArchive   Archive                `pulumi:"dFileArchive"`
	RemoteArchive Archive                `pulumi:"dRemoteArchive"`
	E             interface{}            `pulumi:"e"`
	Array         []interface{}          `pulumi:"fArray"`
	Map           map[string]interface{} `pulumi:"fMap"`
	G             string                 `pulumi:"g"`
	H             string                 `pulumi:"h"`
	I             string                 `pulumi:"i"`
}

type testInputs struct {
	S             StringInput
	A             BoolInput
	B             IntInput
	StringAsset   AssetInput
	FileAsset     AssetInput
	RemoteAsset   AssetInput
	AssetArchive  ArchiveInput
	FileArchive   ArchiveInput
	RemoteArchive ArchiveInput
	E             Input
	Array         ArrayInput
	Map           MapInput
	G             StringInput
	H             StringInput
	I             StringInput
}

func (testInputs) ElementType() reflect.Type {
	return reflect.TypeOf(test{})
}

// TestMarshalRoundtrip ensures that marshaling a complex structure to and from its on-the-wire gRPC format succeeds.
func TestMarshalRoundtrip(t *testing.T) {
	// Create interesting inputs.
	out, resolve, _ := NewOutput()
	resolve("outputty")
	out2 := newOutputState(reflect.TypeOf(""))
	out2.fulfill(nil, false, false, nil)
	inputs := testInputs{
		S:           String("a string"),
		A:           Bool(true),
		B:           Int(42),
		StringAsset: NewStringAsset("put a lime in the coconut"),
		FileAsset:   NewFileAsset("foo.txt"),
		RemoteAsset: NewRemoteAsset("https://pulumi.com/fake/txt"),
		AssetArchive: NewAssetArchive(map[string]interface{}{
			"subAsset":   NewFileAsset("bar.txt"),
			"subArchive": NewFileArchive("bar.zip"),
		}),
		FileArchive:   NewFileArchive("foo.zip"),
		RemoteArchive: NewRemoteArchive("https://pulumi.com/fake/archive.zip"),
		E:             out,
		Array:         Array{Int(0), Float32(1.3), String("x"), Bool(false)},
		Map: Map{
			"x": String("y"),
			"y": Float64(999.9),
			"z": Bool(false),
		},
		G: StringOutput{out2},
		H: URN("foo"),
		I: StringOutput{},
	}

	// Marshal those inputs.
	resolved, pdeps, deps, err := marshalInputs(inputs)
	assert.Nil(t, err)

	if !assert.Nil(t, err) {
		assert.Equal(t, reflect.TypeOf(inputs).NumField(), len(pdeps))
		assert.Equal(t, 0, len(deps))

		// Now just unmarshal and ensure the resulting map matches.
		resV, secret, err := unmarshalPropertyValue(resource.NewObjectProperty(resolved))
		assert.False(t, secret)
		if !assert.Nil(t, err) {
			if !assert.NotNil(t, resV) {
				res := resV.(map[string]interface{})
				assert.Equal(t, "a string", res["s"])
				assert.Equal(t, true, res["a"])
				assert.Equal(t, 42, res["b"])
				assert.Equal(t, "put a lime in the coconut", res["cStringAsset"].(Asset).Text())
				assert.Equal(t, "foo.txt", res["cFileAsset"].(Asset).Path())
				assert.Equal(t, "https://pulumi.com/fake/txt", res["cRemoteAsset"].(Asset).URI())
				ar := res["dAssetArchive"].(Archive).Assets()
				assert.Equal(t, 2, len(ar))
				assert.Equal(t, "bar.txt", ar["subAsset"].(Asset).Path())
				assert.Equal(t, "bar.zip", ar["subrchive"].(Archive).Path())
				assert.Equal(t, "foo.zip", res["dFileArchive"].(Archive).Path())
				assert.Equal(t, "https://pulumi.com/fake/archive.zip", res["dRemoteArchive"].(Archive).URI())
				assert.Equal(t, "outputty", res["e"])
				aa := res["fArray"].([]interface{})
				assert.Equal(t, 4, len(aa))
				assert.Equal(t, 0, aa[0])
				assert.Equal(t, 1.3, aa[1])
				assert.Equal(t, "x", aa[2])
				assert.Equal(t, false, aa[3])
				am := res["fMap"].(map[string]interface{})
				assert.Equal(t, 3, len(am))
				assert.Equal(t, "y", am["x"])
				assert.Equal(t, 999.9, am["y"])
				assert.Equal(t, false, am["z"])
				assert.Equal(t, rpcTokenUnknownValue, res["g"])
				assert.Equal(t, "foo", res["h"])
				assert.Equal(t, rpcTokenUnknownValue, res["i"])
			}
		}
	}
}

type nestedTypeInput interface {
	Input
}

var nestedTypeType = reflect.TypeOf((*nestedType)(nil)).Elem()

type nestedType struct {
	Foo string `pulumi:"foo"`
	Bar int    `pulumi:"bar"`
}

type nestedTypeInputs struct {
	Foo StringInput `pulumi:"foo"`
	Bar IntInput    `pulumi:"bar"`
}

func (nestedTypeInputs) ElementType() reflect.Type {
	return nestedTypeType
}

func (nestedTypeInputs) isNestedType() {}

type nestedTypeOutput struct{ *OutputState }

func (nestedTypeOutput) ElementType() reflect.Type {
	return nestedTypeType
}

func (nestedTypeOutput) isNestedType() {}

func init() {
	RegisterOutputType(nestedTypeOutput{})
}

type testResourceArgs struct {
	URN URN `pulumi:"urn"`
	ID  ID  `pulumi:"id"`

	Any     interface{}            `pulumi:"any"`
	Archive Archive                `pulumi:"archive"`
	Array   []interface{}          `pulumi:"array"`
	Asset   Asset                  `pulumi:"asset"`
	Bool    bool                   `pulumi:"bool"`
	Float32 float32                `pulumi:"float32"`
	Float64 float64                `pulumi:"float64"`
	Int     int                    `pulumi:"int"`
	Int8    int8                   `pulumi:"int8"`
	Int16   int16                  `pulumi:"int16"`
	Int32   int32                  `pulumi:"int32"`
	Int64   int64                  `pulumi:"int64"`
	Map     map[string]interface{} `pulumi:"map"`
	String  string                 `pulumi:"string"`
	Uint    uint                   `pulumi:"uint"`
	Uint8   uint8                  `pulumi:"uint8"`
	Uint16  uint16                 `pulumi:"uint16"`
	Uint32  uint32                 `pulumi:"uint32"`
	Uint64  uint64                 `pulumi:"uint64"`

	Nested nestedType `pulumi:"nested"`
}

type testResourceInputs struct {
	URN URNInput
	ID  IDInput

	Any     Input
	Archive ArchiveInput
	Array   ArrayInput
	Asset   AssetInput
	Bool    BoolInput
	Float32 Float32Input
	Float64 Float64Input
	Int     IntInput
	Int8    Int8Input
	Int16   Int16Input
	Int32   Int32Input
	Int64   Int64Input
	Map     MapInput
	String  StringInput
	Uint    UintInput
	Uint8   Uint8Input
	Uint16  Uint16Input
	Uint32  Uint32Input
	Uint64  Uint64Input

	Nested nestedTypeInput
}

func (*testResourceInputs) ElementType() reflect.Type {
	return reflect.TypeOf((*testResourceArgs)(nil))
}

type testResource struct {
	CustomResourceState

	Any     AnyOutput     `pulumi:"any"`
	Archive ArchiveOutput `pulumi:"archive"`
	Array   ArrayOutput   `pulumi:"array"`
	Asset   AssetOutput   `pulumi:"asset"`
	Bool    BoolOutput    `pulumi:"bool"`
	Float32 Float32Output `pulumi:"float32"`
	Float64 Float64Output `pulumi:"float64"`
	Int     IntOutput     `pulumi:"int"`
	Int8    Int8Output    `pulumi:"int8"`
	Int16   Int16Output   `pulumi:"int16"`
	Int32   Int32Output   `pulumi:"int32"`
	Int64   Int64Output   `pulumi:"int64"`
	Map     MapOutput     `pulumi:"map"`
	String  StringOutput  `pulumi:"string"`
	Uint    UintOutput    `pulumi:"uint"`
	Uint8   Uint8Output   `pulumi:"uint8"`
	Uint16  Uint16Output  `pulumi:"uint16"`
	Uint32  Uint32Output  `pulumi:"uint32"`
	Uint64  Uint64Output  `pulumi:"uint64"`

	Nested nestedTypeOutput `pulumi:"nested"`
}

func TestResourceState(t *testing.T) {
	var theResource testResource
	state := makeResourceState("", "", &theResource, nil, nil)

	resolved, _, _, _ := marshalInputs(&testResourceInputs{
		Any:     String("foo"),
		Archive: NewRemoteArchive("https://pulumi.com/fake/archive.zip"),
		Array:   Array{String("foo")},
		Asset:   NewStringAsset("put a lime in the coconut"),
		Bool:    Bool(true),
		Float32: Float32(42.0),
		Float64: Float64(3.14),
		Int:     Int(-1),
		Int8:    Int8(-2),
		Int16:   Int16(-3),
		Int32:   Int32(-4),
		Int64:   Int64(-5),
		Map:     Map{"foo": String("bar")},
		String:  String("qux"),
		Uint:    Uint(1),
		Uint8:   Uint8(2),
		Uint16:  Uint16(3),
		Uint32:  Uint32(4),
		Uint64:  Uint64(5),

		Nested: nestedTypeInputs{
			Foo: String("bar"),
			Bar: Int(42),
		},
	})
	s, err := plugin.MarshalProperties(
		resolved,
		plugin.MarshalOptions{KeepUnknowns: true})
	assert.NoError(t, err)
	state.resolve(false, nil, nil, "foo", "bar", s)

	input := &testResourceInputs{
		URN:     theResource.URN(),
		ID:      theResource.ID(),
		Any:     theResource.Any,
		Archive: theResource.Archive,
		Array:   theResource.Array,
		Asset:   theResource.Asset,
		Bool:    theResource.Bool,
		Float32: theResource.Float32,
		Float64: theResource.Float64,
		Int:     theResource.Int,
		Int8:    theResource.Int8,
		Int16:   theResource.Int16,
		Int32:   theResource.Int32,
		Int64:   theResource.Int64,
		Map:     theResource.Map,
		String:  theResource.String,
		Uint:    theResource.Uint,
		Uint8:   theResource.Uint8,
		Uint16:  theResource.Uint16,
		Uint32:  theResource.Uint32,
		Uint64:  theResource.Uint64,
		Nested:  theResource.Nested,
	}
	resolved, pdeps, deps, err := marshalInputs(input)
	assert.Nil(t, err)
	assert.Equal(t, map[string][]URN{
		"urn":     {"foo"},
		"id":      {"foo"},
		"any":     {"foo"},
		"archive": {"foo"},
		"array":   {"foo"},
		"asset":   {"foo"},
		"bool":    {"foo"},
		"float32": {"foo"},
		"float64": {"foo"},
		"int":     {"foo"},
		"int8":    {"foo"},
		"int16":   {"foo"},
		"int32":   {"foo"},
		"int64":   {"foo"},
		"map":     {"foo"},
		"string":  {"foo"},
		"uint":    {"foo"},
		"uint8":   {"foo"},
		"uint16":  {"foo"},
		"uint32":  {"foo"},
		"uint64":  {"foo"},
		"nested":  {"foo"},
	}, pdeps)
	assert.Equal(t, []URN{"foo"}, deps)

	res, secret, err := unmarshalPropertyValue(resource.NewObjectProperty(resolved))
	assert.Nil(t, err)
	assert.False(t, secret)
	assert.Equal(t, map[string]interface{}{
		"urn":     "foo",
		"id":      "bar",
		"any":     "foo",
		"archive": NewRemoteArchive("https://pulumi.com/fake/archive.zip"),
		"array":   []interface{}{"foo"},
		"asset":   NewStringAsset("put a lime in the coconut"),
		"bool":    true,
		"float32": 42.0,
		"float64": 3.14,
		"int":     -1.0,
		"int8":    -2.0,
		"int16":   -3.0,
		"int32":   -4.0,
		"int64":   -5.0,
		"map":     map[string]interface{}{"foo": "bar"},
		"string":  "qux",
		"uint":    1.0,
		"uint8":   2.0,
		"uint16":  3.0,
		"uint32":  4.0,
		"uint64":  5.0,
		"nested": map[string]interface{}{
			"foo": "bar",
			"bar": 42.0,
		},
	}, res)
}

// TODO(evanboyle) add cases for bubbling up nested secretness.
func TestUnmarshalSecret(t *testing.T) {
	secret := resource.MakeSecret(resource.NewPropertyValue("foo"))

	_, isSecret, err := unmarshalPropertyValue(secret)
	assert.Nil(t, err)
	assert.True(t, isSecret)

	var sv string
	isSecret, err = unmarshalOutput(secret, reflect.ValueOf(&sv).Elem())
	assert.Nil(t, err)
	assert.Equal(t, "foo", sv)
	assert.True(t, isSecret)
}
