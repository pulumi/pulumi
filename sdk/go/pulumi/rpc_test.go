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
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
)

type simpleComponentResource struct {
	ResourceState
}

func newSimpleComponentResource(ctx *Context, urn URN) ComponentResource {
	var res simpleComponentResource
	res.urn.OutputState = ctx.newOutputState(res.urn.ElementType(), &res)
	res.urn.resolve(urn, true, false, nil)
	return &res
}

type simpleCustomResource struct {
	CustomResourceState
}

func newSimpleCustomResource(ctx *Context, urn URN, id ID) CustomResource {
	var res simpleCustomResource
	res.urn.OutputState = ctx.newOutputState(res.urn.ElementType(), &res)
	res.id.OutputState = ctx.newOutputState(res.id.ElementType(), &res)
	res.urn.resolve(urn, true, false, nil)
	res.id.resolve(id, id != "", false, nil)
	return &res
}

type simpleProviderResource struct {
	ProviderResourceState
}

func newSimpleProviderResource(ctx *Context, urn URN, id ID) ProviderResource {
	var res simpleProviderResource
	res.urn.OutputState = ctx.newOutputState(res.urn.ElementType(), &res)
	res.id.OutputState = ctx.newOutputState(res.id.ElementType(), &res)
	res.urn.resolve(urn, true, false, nil)
	res.id.resolve(id, id != "", false, nil)
	res.pkg = string(resource.URN(urn).Type().Name())
	return &res
}

type testResourcePackage struct {
	version semver.Version
}

func (rp *testResourcePackage) ConstructProvider(ctx *Context, name, typ, urn string) (ProviderResource, error) {
	if typ != "pulumi:providers:test" {
		return nil, fmt.Errorf("unknown provider type %v", typ)
	}
	id := "id"
	if resource.URN(urn).Name() == "preview" {
		id = ""
	}
	return newSimpleProviderResource(ctx, URN(urn), ID(id)), nil
}

func (rp *testResourcePackage) Version() semver.Version {
	return rp.version
}

type testResourceModule struct {
	version semver.Version
}

func (rm *testResourceModule) Construct(ctx *Context, name, typ, urn string) (Resource, error) {
	switch typ {
	case "test:index:custom":
		id := "id"
		if resource.URN(urn).Name() == "preview" {
			id = ""
		}
		return newSimpleCustomResource(ctx, URN(urn), ID(id)), nil
	case "test:index:component":
		return newSimpleComponentResource(ctx, URN(urn)), nil
	default:
		return nil, fmt.Errorf("unknown resource type %v", typ)
	}
}

func (rm *testResourceModule) Version() semver.Version {
	return rm.version
}

type test struct {
	S                              string                 `pulumi:"s"`
	A                              bool                   `pulumi:"a"`
	B                              int                    `pulumi:"b"`
	StringAsset                    Asset                  `pulumi:"cStringAsset"`
	FileAsset                      Asset                  `pulumi:"cFileAsset"`
	RemoteAsset                    Asset                  `pulumi:"cRemoteAsset"`
	AssetArchive                   Archive                `pulumi:"dAssetArchive"`
	FileArchive                    Archive                `pulumi:"dFileArchive"`
	RemoteArchive                  Archive                `pulumi:"dRemoteArchive"`
	E                              interface{}            `pulumi:"e"`
	Array                          []interface{}          `pulumi:"fArray"`
	Map                            map[string]interface{} `pulumi:"fMap"`
	G                              string                 `pulumi:"g"`
	H                              string                 `pulumi:"h"`
	I                              string                 `pulumi:"i"`
	CustomResource                 CustomResource         `pulumi:"jCustomResource"`
	ComponentResource              ComponentResource      `pulumi:"jComponentResource"`
	ProviderResource               ProviderResource       `pulumi:"jProviderResource"`
	PreviewCustomResource          CustomResource         `pulumi:"jPreviewCustomResource"`
	PreviewProviderResource        ProviderResource       `pulumi:"jPreviewProviderResource"`
	MissingCustomResource          CustomResource         `pulumi:"kCustomResource"`
	MissingComponentResource       ComponentResource      `pulumi:"kComponentResource"`
	MissingProviderResource        ProviderResource       `pulumi:"kProviderResource"`
	MissingPreviewCustomResource   CustomResource         `pulumi:"kPreviewCustomResource"`
	MissingPreviewProviderResource ProviderResource       `pulumi:"kPreviewProviderResource"`
}

type testInputs struct {
	S                              StringInput
	A                              BoolInput
	B                              IntInput
	StringAsset                    AssetInput
	FileAsset                      AssetInput
	RemoteAsset                    AssetInput
	AssetArchive                   ArchiveInput
	FileArchive                    ArchiveInput
	RemoteArchive                  ArchiveInput
	E                              Input
	Array                          ArrayInput
	Map                            MapInput
	G                              StringInput
	H                              StringInput
	I                              StringInput
	CustomResource                 CustomResource
	ComponentResource              ComponentResource
	ProviderResource               ProviderResource
	PreviewCustomResource          CustomResource
	PreviewProviderResource        ProviderResource
	MissingCustomResource          CustomResource
	MissingComponentResource       ComponentResource
	MissingProviderResource        ProviderResource
	MissingPreviewCustomResource   CustomResource
	MissingPreviewProviderResource ProviderResource
}

func (testInputs) ElementType() reflect.Type {
	return reflect.TypeOf(test{})
}

// TestMarshalRoundtrip ensures that marshaling a complex structure to and from its on-the-wire gRPC format succeeds.
func TestMarshalRoundtrip(t *testing.T) {
	// Create interesting inputs.
	ctx, err := NewContext(context.Background(), RunInfo{})
	assert.Nil(t, err)

	customURN := resource.NewURN("stack", "project", "", "test:index:custom", "test")
	componentURN := resource.NewURN("stack", "project", "", "test:index:component", "test")
	providerURN := resource.NewURN("stack", "project", "", "pulumi:providers:test", "test")
	previewCustomURN := resource.NewURN("stack", "project", "", "test:index:custom", "preview")
	previewProviderURN := resource.NewURN("stack", "project", "", "pulumi:providers:test", "preview")

	missingCustomURN := resource.NewURN("stack", "project", "", "missing:index:custom", "test")
	missingComponentURN := resource.NewURN("stack", "project", "", "missing:index:component", "test")
	missingProviderURN := resource.NewURN("stack", "project", "", "pulumi:providers:missing", "test")
	missingPreviewCustomURN := resource.NewURN("stack", "project", "", "missing:index:custom", "preview")
	missingPreviewProviderURN := resource.NewURN("stack", "project", "", "pulumi:providers:missing", "preview")

	RegisterResourcePackage("test", &testResourcePackage{})
	RegisterResourceModule("test", "index", &testResourceModule{})

	out, resolve, _ := ctx.NewOutput()
	resolve("outputty")
	out2 := ctx.newOutputState(reflect.TypeOf(""))
	out2.fulfill(nil, false, false, nil, nil)
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
		Array:         Array{Int(0), Float64(1.3), String("x"), Bool(false)},
		Map: Map{
			"x": String("y"),
			"y": Float64(999.9),
			"z": Bool(false),
		},
		G:                              StringOutput{out2},
		H:                              URN("foo"),
		I:                              StringOutput{},
		CustomResource:                 newSimpleCustomResource(ctx, URN(customURN), "id"),
		ComponentResource:              ctx.newDependencyResource(URN(componentURN)),
		ProviderResource:               newSimpleProviderResource(ctx, URN(providerURN), "id"),
		PreviewCustomResource:          newSimpleCustomResource(ctx, URN(previewCustomURN), ""),
		PreviewProviderResource:        newSimpleProviderResource(ctx, URN(previewProviderURN), ""),
		MissingCustomResource:          newSimpleCustomResource(ctx, URN(missingCustomURN), "id"),
		MissingComponentResource:       ctx.newDependencyResource(URN(missingComponentURN)),
		MissingProviderResource:        newSimpleProviderResource(ctx, URN(missingProviderURN), "id"),
		MissingPreviewCustomResource:   newSimpleCustomResource(ctx, URN(missingPreviewCustomURN), ""),
		MissingPreviewProviderResource: newSimpleProviderResource(ctx, URN(missingPreviewProviderURN), ""),
	}

	// Marshal those inputs.
	resolved, pdeps, deps, err := marshalInputs(inputs)
	assert.Nil(t, err)

	if assert.Nil(t, err) {
		assert.Equal(t, reflect.TypeOf(inputs).NumField(), len(resolved))
		assert.Equal(t, 10, len(deps))
		assert.Equal(t, 10, len(pdeps))

		// Now just unmarshal and ensure the resulting map matches.
		resV, secret, err := unmarshalPropertyValue(ctx, resource.NewObjectProperty(resolved))
		assert.False(t, secret)
		if assert.Nil(t, err) {
			if assert.NotNil(t, resV) {
				res := resV.(map[string]interface{})
				assert.Equal(t, "a string", res["s"])
				assert.Equal(t, true, res["a"])
				assert.Equal(t, 42.0, res["b"])
				assert.Equal(t, "put a lime in the coconut", res["cStringAsset"].(Asset).Text())
				assert.Equal(t, "foo.txt", res["cFileAsset"].(Asset).Path())
				assert.Equal(t, "https://pulumi.com/fake/txt", res["cRemoteAsset"].(Asset).URI())
				ar := res["dAssetArchive"].(Archive).Assets()
				assert.Equal(t, 2, len(ar))
				assert.Equal(t, "bar.txt", ar["subAsset"].(Asset).Path())
				assert.Equal(t, "bar.zip", ar["subArchive"].(Archive).Path())
				assert.Equal(t, "foo.zip", res["dFileArchive"].(Archive).Path())
				assert.Equal(t, "https://pulumi.com/fake/archive.zip", res["dRemoteArchive"].(Archive).URI())
				assert.Equal(t, "outputty", res["e"])
				aa := res["fArray"].([]interface{})
				assert.Equal(t, 4, len(aa))
				assert.Equal(t, 0.0, aa[0])
				assert.Less(t, 1.3-aa[1].(float64), 0.00001)
				assert.Equal(t, 1.3-aa[1].(float64), 0.0)
				assert.Equal(t, "x", aa[2])
				assert.Equal(t, false, aa[3])
				am := res["fMap"].(map[string]interface{})
				assert.Equal(t, 3, len(am))
				assert.Equal(t, "y", am["x"])
				assert.Equal(t, 999.9, am["y"])
				assert.Equal(t, false, am["z"])
				assert.Equal(t, nil, res["g"])
				assert.Equal(t, "foo", res["h"])
				assert.Equal(t, nil, res["i"])
				custom := res["jCustomResource"].(*simpleCustomResource)
				urn, _, _, _ := custom.URN().awaitURN(context.Background())
				assert.Equal(t, URN(customURN), urn)
				id, _, _, _ := custom.ID().awaitID(context.Background())
				assert.Equal(t, ID("id"), id)
				component := res["jComponentResource"].(*simpleComponentResource)
				urn, _, _, _ = component.URN().awaitURN(context.Background())
				assert.Equal(t, URN(componentURN), urn)
				provider := res["jProviderResource"].(*simpleProviderResource)
				urn, _, _, _ = provider.URN().awaitURN(context.Background())
				assert.Equal(t, URN(providerURN), urn)
				id, _, _, _ = provider.ID().awaitID(context.Background())
				assert.Equal(t, ID("id"), id)
				previewCustom := res["jPreviewCustomResource"].(*simpleCustomResource)
				urn, _, _, _ = previewCustom.URN().awaitURN(context.Background())
				assert.Equal(t, URN(previewCustomURN), urn)
				_, known, _, _ := previewCustom.ID().awaitID(context.Background())
				assert.False(t, known)
				previewProvider := res["jPreviewProviderResource"].(*simpleProviderResource)
				urn, _, _, _ = previewProvider.URN().awaitURN(context.Background())
				assert.Equal(t, URN(previewProviderURN), urn)
				_, known, _, _ = previewProvider.ID().awaitID(context.Background())
				assert.False(t, known)
				missingCustom := res["kCustomResource"].(CustomResource)
				urn, _, _, _ = missingCustom.URN().awaitURN(context.Background())
				assert.Equal(t, URN(missingCustomURN), urn)
				id, _, _, _ = missingCustom.ID().awaitID(context.Background())
				assert.Equal(t, ID("id"), id)
				missingComponent := res["kComponentResource"].(ComponentResource)
				urn, _, _, _ = missingComponent.URN().awaitURN(context.Background())
				assert.Equal(t, URN(missingComponentURN), urn)
				missingProvider := res["kProviderResource"].(ProviderResource)
				urn, _, _, _ = missingProvider.URN().awaitURN(context.Background())
				assert.Equal(t, URN(missingProviderURN), urn)
				id, _, _, _ = missingProvider.ID().awaitID(context.Background())
				assert.Equal(t, ID("id"), id)
				missingPreviewCustom := res["kPreviewCustomResource"].(CustomResource)
				urn, _, _, _ = missingPreviewCustom.URN().awaitURN(context.Background())
				assert.Equal(t, URN(missingPreviewCustomURN), urn)
				_, known, _, _ = missingPreviewCustom.ID().awaitID(context.Background())
				assert.False(t, known)
				missingPreviewProvider := res["kPreviewProviderResource"].(ProviderResource)
				urn, _, _, _ = missingPreviewProvider.URN().awaitURN(context.Background())
				assert.Equal(t, URN(missingPreviewProviderURN), urn)
				_, known, _, _ = missingPreviewProvider.ID().awaitID(context.Background())
				assert.False(t, known)
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
	Float64 float64                `pulumi:"float64"`
	Int     int                    `pulumi:"int"`
	Map     map[string]interface{} `pulumi:"map"`
	String  string                 `pulumi:"string"`

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
	Float64 Float64Input
	Int     IntInput
	Map     MapInput
	String  StringInput

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
	Float64 Float64Output `pulumi:"float64"`
	Int     IntOutput     `pulumi:"int"`
	Map     MapOutput     `pulumi:"map"`
	String  StringOutput  `pulumi:"string"`

	Nested nestedTypeOutput `pulumi:"nested"`
}

func TestResourceState(t *testing.T) {
	ctx, err := NewContext(context.Background(), RunInfo{})
	assert.Nil(t, err)

	var theResource testResource
	state := ctx.makeResourceState("", "", &theResource, nil, nil, "", nil, nil)

	resolved, _, _, _ := marshalInputs(&testResourceInputs{
		Any:     String("foo"),
		Archive: NewRemoteArchive("https://pulumi.com/fake/archive.zip"),
		Array:   Array{String("foo")},
		Asset:   NewStringAsset("put a lime in the coconut"),
		Bool:    Bool(true),
		Float64: Float64(3.14),
		Int:     Int(-1),
		Map:     Map{"foo": String("bar")},
		String:  String("qux"),

		Nested: nestedTypeInputs{
			Foo: String("bar"),
			Bar: Int(42),
		},
	})
	s, err := plugin.MarshalProperties(
		resolved,
		plugin.MarshalOptions{KeepUnknowns: true})
	assert.NoError(t, err)
	state.resolve(ctx, nil, nil, "foo", "bar", s, nil)

	input := &testResourceInputs{
		URN:     theResource.URN(),
		ID:      theResource.ID(),
		Any:     theResource.Any,
		Archive: theResource.Archive,
		Array:   theResource.Array,
		Asset:   theResource.Asset,
		Bool:    theResource.Bool,
		Float64: theResource.Float64,
		Int:     theResource.Int,
		Map:     theResource.Map,
		String:  theResource.String,
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
		"float64": {"foo"},
		"int":     {"foo"},
		"map":     {"foo"},
		"string":  {"foo"},
		"nested":  {"foo"},
	}, pdeps)
	assert.Equal(t, []URN{"foo"}, deps)

	res, secret, err := unmarshalPropertyValue(ctx, resource.NewObjectProperty(resolved))
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
		"float64": 3.14,
		"int":     -1.0,
		"map":     map[string]interface{}{"foo": "bar"},
		"string":  "qux",
		"nested": map[string]interface{}{
			"foo": "bar",
			"bar": 42.0,
		},
	}, res)
}

func TestUnmarshalSecret(t *testing.T) {
	ctx, err := NewContext(context.Background(), RunInfo{})
	assert.Nil(t, err)

	secret := resource.MakeSecret(resource.NewPropertyValue("foo"))

	_, isSecret, err := unmarshalPropertyValue(ctx, secret)
	assert.Nil(t, err)
	assert.True(t, isSecret)

	var sv string
	isSecret, err = unmarshalOutput(ctx, secret, reflect.ValueOf(&sv).Elem())
	assert.Nil(t, err)
	assert.Equal(t, "foo", sv)
	assert.True(t, isSecret)
}

func TestUnmarshalInternalMapValue(t *testing.T) {
	ctx, err := NewContext(context.Background(), RunInfo{})
	assert.Nil(t, err)

	m := make(map[string]interface{})
	m["foo"] = "bar"
	m["__default"] = "buzz"
	pmap := resource.NewObjectProperty(resource.NewPropertyMapFromMap(m))

	var mv map[string]string
	_, err = unmarshalOutput(ctx, pmap, reflect.ValueOf(&mv).Elem())
	assert.Nil(t, err)
	val, ok := mv["foo"]
	assert.True(t, ok)
	assert.Equal(t, "bar", val)
	_, ok = mv["__default"]
	assert.False(t, ok)
}

// TestMarshalRoundtripNestedSecret ensures that marshaling a complex structure to and from
// its on-the-wire gRPC format succeeds including a nested secret property.
func TestMarshalRoundtripNestedSecret(t *testing.T) {
	// Create interesting inputs.
	ctx, err := NewContext(context.Background(), RunInfo{})
	assert.Nil(t, err)

	out, resolve, _ := NewOutput()
	resolve("outputty")
	out2 := ctx.newOutputState(reflect.TypeOf(""))
	out2.fulfill(nil, false, true, nil, nil)
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
		Array:         Array{Int(0), Float64(1.3), String("x"), Bool(false)},
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

	if assert.Nil(t, err) {
		// The value we marshaled above omits the 10 Resource-typed fields, so we don't expect those fields to appear
		// in the unmarshaled value.
		const resourceFields = 10
		assert.Equal(t, reflect.TypeOf(inputs).NumField()-resourceFields, len(resolved))
		assert.Equal(t, 0, len(deps))
		assert.Equal(t, 0, len(pdeps))

		// Now just unmarshal and ensure the resulting map matches.
		resV, secret, err := unmarshalPropertyValue(ctx, resource.NewObjectProperty(resolved))
		assert.True(t, secret)
		if assert.Nil(t, err) {
			if assert.NotNil(t, resV) {
				res := resV.(map[string]interface{})
				assert.Equal(t, "a string", res["s"])
				assert.Equal(t, true, res["a"])
				assert.Equal(t, 42.0, res["b"])
				assert.Equal(t, "put a lime in the coconut", res["cStringAsset"].(Asset).Text())
				assert.Equal(t, "foo.txt", res["cFileAsset"].(Asset).Path())
				assert.Equal(t, "https://pulumi.com/fake/txt", res["cRemoteAsset"].(Asset).URI())
				ar := res["dAssetArchive"].(Archive).Assets()
				assert.Equal(t, 2, len(ar))
				assert.Equal(t, "bar.txt", ar["subAsset"].(Asset).Path())
				assert.Equal(t, "bar.zip", ar["subArchive"].(Archive).Path())
				assert.Equal(t, "foo.zip", res["dFileArchive"].(Archive).Path())
				assert.Equal(t, "https://pulumi.com/fake/archive.zip", res["dRemoteArchive"].(Archive).URI())
				assert.Equal(t, "outputty", res["e"])
				aa := res["fArray"].([]interface{})
				assert.Equal(t, 4, len(aa))
				assert.Equal(t, 0.0, aa[0])
				assert.Less(t, 1.3-aa[1].(float64), 0.00001)
				assert.Equal(t, 1.3-aa[1].(float64), 0.0)
				assert.Equal(t, "x", aa[2])
				assert.Equal(t, false, aa[3])
				am := res["fMap"].(map[string]interface{})
				assert.Equal(t, 3, len(am))
				assert.Equal(t, "y", am["x"])
				assert.Equal(t, 999.9, am["y"])
				assert.Equal(t, false, am["z"])
				assert.Equal(t, nil, res["g"])
				assert.Equal(t, "foo", res["h"])
				assert.Equal(t, nil, res["i"])
			}
		}
	}
}

type UntypedArgs map[string]interface{}

func (UntypedArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*map[string]interface{})(nil)).Elem()
}

func TestMapInputMarhsalling(t *testing.T) {
	var theResource simpleCustomResource
	out := newOutput(nil, reflect.TypeOf((*StringOutput)(nil)).Elem(), &theResource)
	out.getState().resolve("outputty", true, false, nil)

	inputs1 := Map(map[string]Input{
		"prop": out,
		"nested": Map(map[string]Input{
			"foo": String("foo"),
			"bar": Int(42),
		}),
	})

	inputs2 := UntypedArgs(map[string]interface{}{
		"prop": "outputty",
		"nested": map[string]interface{}{
			"foo": "foo",
			"bar": 42,
		},
	})

	cases := []struct {
		inputs  Input
		depUrns []string
	}{
		{inputs: inputs1, depUrns: []string{""}},
		{inputs: inputs2, depUrns: nil},
	}

	for _, c := range cases {
		resolved, _, depUrns, err := marshalInputs(c.inputs)
		assert.NoError(t, err)
		assert.Equal(t, "outputty", resolved["prop"].StringValue())
		assert.Equal(t, "foo", resolved["nested"].ObjectValue()["foo"].StringValue())
		assert.Equal(t, 42.0, resolved["nested"].ObjectValue()["bar"].NumberValue())
		assert.Equal(t, len(c.depUrns), len(depUrns))
		for i := range c.depUrns {
			assert.Equal(t, URN(c.depUrns[i]), depUrns[i])
		}
	}
}

func TestVersionedMap(t *testing.T) {
	resourceModules := versionedMap{
		versions: map[string][]Versioned{},
	}
	_ = resourceModules.Store("test", &testResourcePackage{version: semver.MustParse("1.0.1-alpha1")})
	_ = resourceModules.Store("test", &testResourcePackage{version: semver.MustParse("1.0.2")})
	_ = resourceModules.Store("test", &testResourcePackage{version: semver.MustParse("2.2.0")})
	_ = resourceModules.Store("unrelated", &testResourcePackage{version: semver.MustParse("1.0.3")})
	_ = resourceModules.Store("wild", &testResourcePackage{})
	_ = resourceModules.Store("unreleased", &testResourcePackage{version: semver.MustParse("1.0.0-alpha1")})
	_ = resourceModules.Store("unreleased", &testResourcePackage{version: semver.MustParse("1.0.0-beta1")})

	tests := []struct {
		name            string
		pkg             string
		version         semver.Version
		expectFound     bool
		expectedVersion semver.Version
	}{
		{
			name:        "unknown not found",
			pkg:         "unknown",
			version:     semver.Version{},
			expectFound: false,
		},
		{
			name:        "unknown not found",
			pkg:         "unknown",
			version:     semver.MustParse("0.0.1"),
			expectFound: false,
		},
		{
			name:        "different major version not found",
			pkg:         "test",
			version:     semver.MustParse("0.0.1"),
			expectFound: false,
		},
		{
			name:        "different major version not found",
			pkg:         "test",
			version:     semver.MustParse("3.0.0"),
			expectFound: false,
		},
		{
			name:            "wildcard returns highest version",
			pkg:             "test",
			version:         semver.Version{},
			expectFound:     true,
			expectedVersion: semver.MustParse("2.2.0"),
		},
		{
			name:            "major version respected 1.0.0",
			pkg:             "test",
			version:         semver.MustParse("1.0.0"),
			expectFound:     true,
			expectedVersion: semver.MustParse("1.0.2"),
		},
		{
			name:            "major version respected 2.0.0",
			pkg:             "test",
			version:         semver.MustParse("2.0.0"),
			expectFound:     true,
			expectedVersion: semver.MustParse("2.2.0"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg, found := resourceModules.Load(tt.pkg, tt.version)
			assert.Equal(t, tt.expectFound, found)
			if tt.expectFound {
				assert.Equal(t, tt.expectedVersion, pkg.Version())
			}
		})
	}
}

func TestRegisterResourcePackage(t *testing.T) {
	pkg := "testPkg"

	tests := []struct {
		name            string
		resourcePackage ResourcePackage
	}{
		{
			name:            "wildcard version",
			resourcePackage: &testResourcePackage{},
		},
		{
			name:            "version",
			resourcePackage: &testResourcePackage{version: semver.MustParse("1.2.3")},
		},
		{
			name:            "alpha version",
			resourcePackage: &testResourcePackage{version: semver.MustParse("1.0.0-alpha1")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RegisterResourcePackage(pkg, tt.resourcePackage)
			assert.Panics(t, func() {
				RegisterResourcePackage(pkg, tt.resourcePackage)
			})
		})
	}
}

func TestRegisterResourceModule(t *testing.T) {
	pkg := "testPkg"
	mod := "testMod"

	tests := []struct {
		name           string
		resourceModule ResourceModule
	}{
		{
			name:           "wildcard version",
			resourceModule: &testResourceModule{},
		},
		{
			name:           "version",
			resourceModule: &testResourceModule{version: semver.MustParse("1.2.3")},
		},
		{
			name:           "alpha version",
			resourceModule: &testResourceModule{version: semver.MustParse("1.0.0-alpha1")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RegisterResourceModule(pkg, mod, tt.resourceModule)
			assert.Panics(t, func() {
				RegisterResourceModule(pkg, mod, tt.resourceModule)
			})
		})
	}
}
