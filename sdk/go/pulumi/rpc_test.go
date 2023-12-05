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

//nolint:unused,deadcode,lll
package pulumi

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/blang/semver"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type simpleComponentResource struct {
	ResourceState
}

func newSimpleComponentResource(ctx *Context, urn URN) ComponentResource {
	var res simpleComponentResource
	res.urn.OutputState = ctx.newOutputState(res.urn.ElementType(), &res)
	internal.ResolveOutput(res.urn, urn, true, false, resourcesToInternal(nil))
	return &res
}

type simpleCustomResource struct {
	CustomResourceState
}

func newSimpleCustomResource(ctx *Context, urn URN, id ID) CustomResource {
	var res simpleCustomResource
	res.urn.OutputState = ctx.newOutputState(res.urn.ElementType(), &res)
	res.id.OutputState = ctx.newOutputState(res.id.ElementType(), &res)
	internal.ResolveOutput(res.urn, urn, true, false, resourcesToInternal(nil))
	internal.ResolveOutput(res.id, id, id != "", false, resourcesToInternal(nil))
	return &res
}

type simpleProviderResource struct {
	ProviderResourceState
}

func newSimpleProviderResource(ctx *Context, urn URN, id ID) ProviderResource {
	var res simpleProviderResource
	res.urn.OutputState = ctx.newOutputState(res.urn.ElementType(), &res)
	res.id.OutputState = ctx.newOutputState(res.id.ElementType(), &res)
	internal.ResolveOutput(res.urn, urn, true, false, resourcesToInternal(nil))
	internal.ResolveOutput(res.id, id, id != "", false, resourcesToInternal(nil))
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
	t.Parallel()

	// Create interesting inputs.
	ctx, err := NewContext(context.Background(), RunInfo{})
	assert.NoError(t, err)

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
	internal.FulfillOutput(out2, nil, false, false, resourcesToInternal(nil), nil)
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
	assert.NoError(t, err)

	if assert.NoError(t, err) {
		assert.Equal(t, reflect.TypeOf(inputs).NumField(), len(resolved))
		assert.Equal(t, 10, len(deps))
		assert.Equal(t, 25, len(pdeps))

		// Now just unmarshal and ensure the resulting map matches.
		resV, secret, err := unmarshalPropertyValue(ctx, resource.NewObjectProperty(resolved))
		assert.False(t, secret)
		if assert.NoError(t, err) {
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
	t.Parallel()

	ctx, err := NewContext(context.Background(), RunInfo{})
	assert.NoError(t, err)

	var theResource testResource
	state := ctx.makeResourceState("", "", &theResource, nil, nil, "", "", nil, nil)

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
	assert.NoError(t, err)
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
	assert.NoError(t, err)
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
	t.Parallel()

	ctx, err := NewContext(context.Background(), RunInfo{})
	assert.NoError(t, err)

	secret := resource.MakeSecret(resource.NewPropertyValue("foo"))

	_, isSecret, err := unmarshalPropertyValue(ctx, secret)
	assert.NoError(t, err)
	assert.True(t, isSecret)

	var sv string
	isSecret, err = unmarshalOutput(ctx, secret, reflect.ValueOf(&sv).Elem())
	assert.NoError(t, err)
	assert.Equal(t, "foo", sv)
	assert.True(t, isSecret)
}

func TestUnmarshalInternalMapValue(t *testing.T) {
	t.Parallel()

	ctx, err := NewContext(context.Background(), RunInfo{})
	assert.NoError(t, err)

	m := make(map[string]interface{})
	m["foo"] = "bar"
	m["__default"] = "buzz"
	pmap := resource.NewObjectProperty(resource.NewPropertyMapFromMap(m))

	var mv map[string]string
	_, err = unmarshalOutput(ctx, pmap, reflect.ValueOf(&mv).Elem())
	assert.NoError(t, err)
	val, ok := mv["foo"]
	assert.True(t, ok)
	assert.Equal(t, "bar", val)
	_, ok = mv["__default"]
	assert.False(t, ok)
}

// TestMarshalRoundtripNestedSecret ensures that marshaling a complex structure to and from
// its on-the-wire gRPC format succeeds including a nested secret property.
func TestMarshalRoundtripNestedSecret(t *testing.T) {
	t.Parallel()

	// Create interesting inputs.
	ctx, err := NewContext(context.Background(), RunInfo{})
	assert.NoError(t, err)

	out, resolve, _ := NewOutput()
	resolve("outputty")
	out2 := ctx.newOutputState(reflect.TypeOf(""))
	internal.FulfillOutput(out2, nil, false, true, resourcesToInternal(nil), nil)
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
	assert.NoError(t, err)

	if assert.NoError(t, err) {
		// The value we marshaled above omits the 10 Resource-typed fields, so we don't expect those fields to appear
		// in the unmarshaled value.
		const resourceFields = 10
		assert.Equal(t, reflect.TypeOf(inputs).NumField()-resourceFields, len(resolved))
		assert.Equal(t, 0, len(deps))
		assert.Equal(t, 15, len(pdeps))

		// Now just unmarshal and ensure the resulting map matches.
		resV, secret, err := unmarshalPropertyValue(ctx, resource.NewObjectProperty(resolved))
		assert.True(t, secret)
		if assert.NoError(t, err) {
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

func TestMapInputMarshalling(t *testing.T) {
	t.Parallel()

	var theResource simpleCustomResource
	out := internal.NewOutput(nil, reflect.TypeOf((*StringOutput)(nil)).Elem(), &theResource)
	internal.ResolveOutput(out, "outputty", true, false, resourcesToInternal(nil))

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
		inputs            Input
		depUrns           []string
		expectOutputValue bool
	}{
		{inputs: inputs1, depUrns: []string{""}, expectOutputValue: true},
		{inputs: inputs2, depUrns: nil},
	}

	for _, c := range cases {
		resolved, _, depUrns, err := marshalInputs(c.inputs)
		assert.NoError(t, err)
		if c.expectOutputValue {
			assert.Equal(t, "outputty", resolved["prop"].OutputValue().Element.StringValue())
		} else {
			assert.Equal(t, "outputty", resolved["prop"].StringValue())
		}
		assert.Equal(t, "foo", resolved["nested"].ObjectValue()["foo"].StringValue())
		assert.Equal(t, 42.0, resolved["nested"].ObjectValue()["bar"].NumberValue())
		assert.Equal(t, len(c.depUrns), len(depUrns))
		for i := range c.depUrns {
			assert.Equal(t, URN(c.depUrns[i]), depUrns[i])
		}
	}
}

func TestVersionedMap(t *testing.T) {
	t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pkg, found := resourceModules.Load(tt.pkg, tt.version)
			assert.Equal(t, tt.expectFound, found)
			if tt.expectFound {
				assert.Equal(t, tt.expectedVersion, pkg.Version())
			}
		})
	}
}

func TestRegisterResourcePackage(t *testing.T) {
	t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			RegisterResourcePackage(pkg, tt.resourcePackage)
			assert.Panics(t, func() {
				RegisterResourcePackage(pkg, tt.resourcePackage)
			})
		})
	}
}

func TestRegisterResourceModule(t *testing.T) {
	t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			RegisterResourceModule(pkg, mod, tt.resourceModule)
			assert.Panics(t, func() {
				RegisterResourceModule(pkg, mod, tt.resourceModule)
			})
		})
	}
}

func TestInvalidAsset(t *testing.T) {
	t.Parallel()

	ctx, err := NewContext(context.Background(), RunInfo{})
	assert.NoError(t, err)

	var d Asset
	_, err = unmarshalOutput(ctx, resource.NewStringProperty("foo"), reflect.ValueOf(&d).Elem())
	require.NoError(t, err)
	require.NotNil(t, d)
	require.True(t, d.(*asset).invalid)

	_, _, err = marshalInput(d, assetType, true)
	assert.Error(t, err)
}

func TestInvalidArchive(t *testing.T) {
	t.Parallel()

	ctx, err := NewContext(context.Background(), RunInfo{})
	assert.NoError(t, err)

	var d Archive
	_, err = unmarshalOutput(ctx, resource.NewStringProperty("foo"), reflect.ValueOf(&d).Elem())
	require.NoError(t, err)
	require.NotNil(t, d)
	require.True(t, d.(*archive).invalid)

	_, _, err = marshalInput(d, archiveType, true)
	assert.Error(t, err)
}

func TestDependsOnComponent(t *testing.T) {
	t.Parallel()

	ctx, err := NewContext(context.Background(), RunInfo{})
	assert.NoError(t, err)

	registerResource := func(name string, res Resource, custom bool, options ...ResourceOption) (Resource, []string) {
		opts := merge(options...)
		state := ctx.makeResourceState("", "", res, nil, nil, "", "", nil, nil)
		state.resolve(ctx, nil, nil, name, "", &structpb.Struct{}, nil)

		inputs, err := ctx.prepareResourceInputs(res, Map{}, "", opts, state, false, custom)
		require.NoError(t, err)

		return res, inputs.deps
	}

	newResource := func(name string, options ...ResourceOption) (Resource, []string) {
		var res testResource
		return registerResource(name, &res, true, options...)
	}

	newComponent := func(name string, options ...ResourceOption) (Resource, []string) {
		var res simpleComponentResource
		return registerResource(name, &res, false, options...)
	}

	resA, _ := newResource("resA", nil)
	comp1, _ := newComponent("comp1", nil)
	resB, _ := newResource("resB", Parent(comp1))
	newResource("resC", Parent(resB))
	comp2, _ := newComponent("comp2", Parent(comp1))

	resD, deps := newResource("resD", DependsOn([]Resource{resA}), Parent(comp2))
	assert.Equal(t, []string{"resA"}, deps)

	_, deps = newResource("resE", DependsOn([]Resource{resD}), Parent(comp2))
	assert.Equal(t, []string{"resD"}, deps)

	_, deps = newResource("resF", DependsOn([]Resource{resA}))
	assert.Equal(t, []string{"resA"}, deps)

	resG, deps := newResource("resG", DependsOn([]Resource{comp1}))
	assert.Equal(t, []string{"resB", "resD", "resE"}, deps)

	_, deps = newResource("resH", DependsOn([]Resource{comp2}))
	assert.Equal(t, []string{"resD", "resE"}, deps)

	_, deps = newResource("resI", DependsOn([]Resource{resG}))
	assert.Equal(t, []string{"resG"}, deps)
}

func TestOutputValueMarshalling(t *testing.T) {
	t.Parallel()

	ctx, err := NewContext(context.Background(), RunInfo{})
	assert.NoError(t, err)

	values := []struct {
		value    interface{}
		expected resource.PropertyValue
	}{
		{value: nil, expected: resource.NewNullProperty()},
		{value: 0, expected: resource.NewNumberProperty(0)},
		{value: 1, expected: resource.NewNumberProperty(1)},
		{value: "", expected: resource.NewStringProperty("")},
		{value: "hi", expected: resource.NewStringProperty("hi")},
		{value: map[string]string{}, expected: resource.NewObjectProperty(resource.PropertyMap{})},
		{value: []string{}, expected: resource.NewArrayProperty(nil)},
	}
	//nolint:paralleltest // parallel parent, would require refactor to silence lint
	for _, value := range values {
		for _, deps := range [][]resource.URN{nil, {"fakeURN1", "fakeURN2"}} {
			for _, known := range []bool{true, false} {
				for _, secret := range []bool{true, false} {
					var resources []Resource
					if len(deps) > 0 {
						for _, dep := range deps {
							resources = append(resources, ctx.newDependencyResource(URN(dep)))
						}
					}

					out := ctx.newOutput(anyOutputType, resources...)
					internal.ResolveOutput(out, value.value, known, secret, resourcesToInternal(nil))
					inputs := Map{"value": out}

					expectedValue := value.expected
					if !known || secret || len(deps) > 0 {
						v := resource.Output{
							Known:        known,
							Secret:       secret,
							Dependencies: deps,
						}
						if known {
							v.Element = value.expected
						}
						expectedValue = resource.NewOutputProperty(v)
					}

					expected := resource.PropertyMap{"value": expectedValue}
					if value.value == nil && known && !secret && len(deps) == 0 {
						// marshalInputs excludes plain nil values.
						expected = resource.PropertyMap{}
					}

					name := fmt.Sprintf("value=%v, known=%v, secret=%v, deps=%v", value, known, secret, deps)
					//nolint:paralleltest // very small test, parallel parent
					t.Run(name, func(t *testing.T) {
						actual, _, _, err := marshalInputs(inputs)
						assert.NoError(t, err)
						assert.Equal(t, expected, actual)
					})
				}
			}
		}
	}
}

type foo struct {
	TemplateOptions *TemplateOptions `pulumi:"templateOptions"`
}

type fooArgs struct {
	TemplateOptions TemplateOptionsPtrInput
}

func (fooArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*foo)(nil)).Elem()
}

type TemplateOptions struct {
	Description       *string                    `pulumi:"description"`
	TagSpecifications []TemplateTagSpecification `pulumi:"tagSpecifications"`
}

type TemplateOptionsInput interface {
	Input

	ToTemplateOptionsOutput() TemplateOptionsOutput
	ToTemplateOptionsOutputWithContext(context.Context) TemplateOptionsOutput
}

type TemplateOptionsArgs struct {
	Description       StringPtrInput
	TagSpecifications TemplateTagSpecificationArrayInput
}

func (TemplateOptionsArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*TemplateOptions)(nil)).Elem()
}

func (i TemplateOptionsArgs) ToTemplateOptionsOutput() TemplateOptionsOutput {
	return i.ToTemplateOptionsOutputWithContext(context.Background())
}

func (i TemplateOptionsArgs) ToTemplateOptionsOutputWithContext(ctx context.Context) TemplateOptionsOutput {
	return ToOutputWithContext(ctx, i).(TemplateOptionsOutput)
}

func (i TemplateOptionsArgs) ToTemplateOptionsPtrOutput() TemplateOptionsPtrOutput {
	return i.ToTemplateOptionsPtrOutputWithContext(context.Background())
}

func (i TemplateOptionsArgs) ToTemplateOptionsPtrOutputWithContext(ctx context.Context) TemplateOptionsPtrOutput {
	return ToOutputWithContext(ctx, i).(TemplateOptionsOutput).ToTemplateOptionsPtrOutputWithContext(ctx)
}

type TemplateOptionsPtrInput interface {
	Input

	ToTemplateOptionsPtrOutput() TemplateOptionsPtrOutput
	ToTemplateOptionsPtrOutputWithContext(context.Context) TemplateOptionsPtrOutput
}

type templateOptionsPtrType TemplateOptionsArgs

func TemplateOptionsPtr(v *TemplateOptionsArgs) TemplateOptionsPtrInput {
	return (*templateOptionsPtrType)(v)
}

func (*templateOptionsPtrType) ElementType() reflect.Type {
	return reflect.TypeOf((**TemplateOptions)(nil)).Elem()
}

func (i *templateOptionsPtrType) ToTemplateOptionsPtrOutput() TemplateOptionsPtrOutput {
	return i.ToTemplateOptionsPtrOutputWithContext(context.Background())
}

func (i *templateOptionsPtrType) ToTemplateOptionsPtrOutputWithContext(ctx context.Context) TemplateOptionsPtrOutput {
	return ToOutputWithContext(ctx, i).(TemplateOptionsPtrOutput)
}

type TemplateOptionsOutput struct{ *OutputState }

func (TemplateOptionsOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*TemplateOptions)(nil)).Elem()
}

func (o TemplateOptionsOutput) ToTemplateOptionsOutput() TemplateOptionsOutput {
	return o
}

func (o TemplateOptionsOutput) ToTemplateOptionsOutputWithContext(ctx context.Context) TemplateOptionsOutput {
	return o
}

func (o TemplateOptionsOutput) ToTemplateOptionsPtrOutput() TemplateOptionsPtrOutput {
	return o.ToTemplateOptionsPtrOutputWithContext(context.Background())
}

func (o TemplateOptionsOutput) ToTemplateOptionsPtrOutputWithContext(ctx context.Context) TemplateOptionsPtrOutput {
	return o.ApplyTWithContext(ctx, func(_ context.Context, v TemplateOptions) *TemplateOptions {
		return &v
	}).(TemplateOptionsPtrOutput)
}

type TemplateOptionsPtrOutput struct{ *OutputState }

func (TemplateOptionsPtrOutput) ElementType() reflect.Type {
	return reflect.TypeOf((**TemplateOptions)(nil)).Elem()
}

func (o TemplateOptionsPtrOutput) ToTemplateOptionsPtrOutput() TemplateOptionsPtrOutput {
	return o
}

func (o TemplateOptionsPtrOutput) ToTemplateOptionsPtrOutputWithContext(ctx context.Context) TemplateOptionsPtrOutput {
	return o
}

type TemplateTagSpecification struct {
	Name *string           `pulumi:"name"`
	Tags map[string]string `pulumi:"tags"`
}

type TemplateTagSpecificationInput interface {
	Input

	ToTemplateTagSpecificationOutput() TemplateTagSpecificationOutput
	ToTemplateTagSpecificationOutputWithContext(context.Context) TemplateTagSpecificationOutput
}

type TemplateTagSpecificationArgs struct {
	Name StringPtrInput `pulumi:"name"`
	Tags StringMapInput `pulumi:"tags"`
}

func (TemplateTagSpecificationArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*TemplateTagSpecification)(nil)).Elem()
}

func (i TemplateTagSpecificationArgs) ToTemplateTagSpecificationOutput() TemplateTagSpecificationOutput {
	return i.ToTemplateTagSpecificationOutputWithContext(context.Background())
}

func (i TemplateTagSpecificationArgs) ToTemplateTagSpecificationOutputWithContext(ctx context.Context) TemplateTagSpecificationOutput {
	return ToOutputWithContext(ctx, i).(TemplateTagSpecificationOutput)
}

type TemplateTagSpecificationArrayInput interface {
	Input

	ToTemplateTagSpecificationArrayOutput() TemplateTagSpecificationArrayOutput
	ToTemplateTagSpecificationArrayOutputWithContext(context.Context) TemplateTagSpecificationArrayOutput
}

type TemplateTagSpecificationArray []TemplateTagSpecificationInput

func (TemplateTagSpecificationArray) ElementType() reflect.Type {
	return reflect.TypeOf((*[]TemplateTagSpecification)(nil)).Elem()
}

func (i TemplateTagSpecificationArray) ToTemplateTagSpecificationArrayOutput() TemplateTagSpecificationArrayOutput {
	return i.ToTemplateTagSpecificationArrayOutputWithContext(context.Background())
}

func (i TemplateTagSpecificationArray) ToTemplateTagSpecificationArrayOutputWithContext(ctx context.Context) TemplateTagSpecificationArrayOutput {
	return ToOutputWithContext(ctx, i).(TemplateTagSpecificationArrayOutput)
}

type TemplateTagSpecificationOutput struct{ *OutputState }

func (TemplateTagSpecificationOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*TemplateTagSpecification)(nil)).Elem()
}

func (o TemplateTagSpecificationOutput) ToTemplateTagSpecificationOutput() TemplateTagSpecificationOutput {
	return o
}

func (o TemplateTagSpecificationOutput) ToTemplateTagSpecificationOutputWithContext(ctx context.Context) TemplateTagSpecificationOutput {
	return o
}

type TemplateTagSpecificationArrayOutput struct{ *OutputState }

func (TemplateTagSpecificationArrayOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*[]TemplateTagSpecification)(nil)).Elem()
}

func (o TemplateTagSpecificationArrayOutput) ToTemplateTagSpecificationArrayOutput() TemplateTagSpecificationArrayOutput {
	return o
}

func (o TemplateTagSpecificationArrayOutput) ToTemplateTagSpecificationArrayOutputWithContext(ctx context.Context) TemplateTagSpecificationArrayOutput {
	return o
}

type bucketObjectArgs struct {
	Source AssetOrArchive `pulumi:"source"`
}

type BucketObjectArgs struct {
	Source AssetOrArchiveInput
}

func (BucketObjectArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*bucketObjectArgs)(nil)).Elem()
}

type myResourceArgs struct {
	Res Resource `pulumi:"res"`
}

type MyResourceArgs struct {
	Res ResourceInput
}

func (MyResourceArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*myResourceArgs)(nil)).Elem()
}

type myNestedOutputArgs struct {
	Nested interface{} `pulumi:"nested"`
}

type MyNestedOutputArgs struct {
	Nested Input
}

func (MyNestedOutputArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*myNestedOutputArgs)(nil)).Elem()
}

func TestOutputValueMarshallingNested(t *testing.T) {
	t.Parallel()

	ctx, err := NewContext(context.Background(), RunInfo{})
	assert.NoError(t, err)

	RegisterOutputType(TemplateOptionsOutput{})
	RegisterOutputType(TemplateOptionsPtrOutput{})
	RegisterOutputType(TemplateTagSpecificationOutput{})
	RegisterOutputType(TemplateTagSpecificationArrayOutput{})

	templateOptionsPtrOutputType := reflect.TypeOf((*TemplateOptionsPtrOutput)(nil)).Elem()
	unknownTemplateOptionsPtrOutput := ctx.newOutput(templateOptionsPtrOutputType).(TemplateOptionsPtrOutput)
	internal.ResolveOutput(unknownTemplateOptionsPtrOutput, nil, false, false, resourcesToInternal(nil)) /*known*/ /*secret*/

	unknownSecretTemplateOptionsPtrOutput := ctx.newOutput(templateOptionsPtrOutputType).(TemplateOptionsPtrOutput)
	internal.ResolveOutput(unknownSecretTemplateOptionsPtrOutput, nil, false, true, resourcesToInternal(nil)) /*known*/ /*secret*/

	stringOutputType := reflect.TypeOf((*StringOutput)(nil)).Elem()
	unknownStringOutput := ctx.newOutput(stringOutputType).(StringOutput)
	internal.ResolveOutput(unknownStringOutput, "", false, false, resourcesToInternal(nil)) /*known*/ /*secret*/

	assetOutputType := reflect.TypeOf((*AssetOutput)(nil)).Elem()
	fileAssetOutput := ctx.newOutput(assetOutputType).(AssetOutput)
	internal.ResolveOutput(fileAssetOutput, &asset{path: "foo.txt"}, true, false, resourcesToInternal(nil)) /*known*/ /*secret*/
	fileAssetSecretOutput := ctx.newOutput(assetOutputType).(AssetOutput)
	internal.ResolveOutput(fileAssetSecretOutput, &asset{path: "foo.txt"}, true, true, resourcesToInternal(nil)) /*known*/ /*secret*/
	fileAssetOutputDeps := ctx.newOutput(assetOutputType).(AssetOutput)
	internal.ResolveOutput(fileAssetOutputDeps, &asset{path: "foo.txt"}, true, false, resourcesToInternal([]Resource{newSimpleCustomResource(ctx, "fakeURN", "fakeID")})) /*known*/ /*secret*/

	anyOutputType := reflect.TypeOf((*AnyOutput)(nil)).Elem()

	nestedOutput := ctx.newOutput(anyOutputType).(AnyOutput)
	internal.ResolveOutput(nestedOutput, fileAssetOutput, true, false, resourcesToInternal(nil)) /*known*/ /*secret*/

	nestedPtrOutput := ctx.newOutput(anyOutputType).(AnyOutput)
	internal.ResolveOutput(nestedPtrOutput, &fileAssetOutput, true, false, resourcesToInternal(nil)) /*known*/ /*secret*/

	nestedNestedOutput := ctx.newOutput(anyOutputType).(AnyOutput)
	internal.ResolveOutput(nestedNestedOutput, nestedOutput, true, false, resourcesToInternal(nil)) /*known*/ /*secret*/

	tests := []struct {
		name     string
		input    Input
		expected resource.PropertyValue
	}{
		{
			name:     "empty",
			input:    fooArgs{},
			expected: resource.NewObjectProperty(resource.PropertyMap{}),
		},
		{
			name: "options empty",
			input: fooArgs{
				TemplateOptions: TemplateOptionsArgs{},
			},
			expected: resource.NewObjectProperty(resource.PropertyMap{
				"templateOptions": resource.NewObjectProperty(resource.PropertyMap{}),
			}),
		},
		{
			name: "options unknown",
			input: fooArgs{
				TemplateOptions: unknownTemplateOptionsPtrOutput,
			},
			expected: resource.NewObjectProperty(resource.PropertyMap{
				"templateOptions": resource.NewOutputProperty(resource.Output{}),
			}),
		},
		{
			name: "options unknown secret",
			input: fooArgs{
				TemplateOptions: unknownSecretTemplateOptionsPtrOutput,
			},
			expected: resource.NewObjectProperty(resource.PropertyMap{
				"templateOptions": resource.NewOutputProperty(resource.Output{
					Secret: true,
				}),
			}),
		},
		{
			name: "options plain known description",
			input: fooArgs{
				TemplateOptions: TemplateOptionsArgs{
					Description: String("hello"),
				},
			},
			expected: resource.NewObjectProperty(resource.PropertyMap{
				"templateOptions": resource.NewObjectProperty(resource.PropertyMap{
					"description": resource.NewStringProperty("hello"),
				}),
			}),
		},
		{
			name: "options plain known secret description",
			input: fooArgs{
				TemplateOptions: TemplateOptionsArgs{
					Description: ToSecret(String("hello")).(StringOutput),
				},
			},
			expected: resource.NewObjectProperty(resource.PropertyMap{
				"templateOptions": resource.NewObjectProperty(resource.PropertyMap{
					"description": resource.NewOutputProperty(resource.Output{
						Element: resource.NewStringProperty("hello"),
						Known:   true,
						Secret:  true,
					}),
				}),
			}),
		},
		{
			name: "options output known secret description",
			input: fooArgs{
				TemplateOptions: TemplateOptionsArgs{
					Description: ToSecret(String("hello")).(StringOutput),
				}.ToTemplateOptionsOutput(),
			},
			expected: resource.NewObjectProperty(resource.PropertyMap{
				"templateOptions": resource.NewOutputProperty(resource.Output{
					Element: resource.NewObjectProperty(resource.PropertyMap{
						"description": resource.NewStringProperty("hello"),
					}),
					Known:  true,
					Secret: true,
				}),
			}),
		},
		{
			name: "options plain unknown description",
			input: fooArgs{
				TemplateOptions: TemplateOptionsArgs{
					Description: unknownStringOutput,
				},
			},
			expected: resource.NewObjectProperty(resource.PropertyMap{
				"templateOptions": resource.NewObjectProperty(resource.PropertyMap{
					"description": resource.NewOutputProperty(resource.Output{}),
				}),
			}),
		},
		{
			name: "options tag specifications nested unknown",
			input: fooArgs{
				TemplateOptions: TemplateOptionsArgs{
					TagSpecifications: TemplateTagSpecificationArray{
						TemplateTagSpecificationArgs{
							Name: String("hello"),
							Tags: StringMap{
								"first": String("second"),
								"third": unknownStringOutput,
							},
						},
					},
				},
			},
			expected: resource.NewObjectProperty(resource.PropertyMap{
				"templateOptions": resource.NewObjectProperty(resource.PropertyMap{
					"tagSpecifications": resource.NewArrayProperty([]resource.PropertyValue{
						resource.NewObjectProperty(resource.PropertyMap{
							"name": resource.NewStringProperty("hello"),
							"tags": resource.NewObjectProperty(resource.PropertyMap{
								"first": resource.NewStringProperty("second"),
								"third": resource.NewOutputProperty(resource.Output{}),
							}),
						}),
					}),
				}),
			}),
		},
		{
			name: "bucket object with file asset",
			input: &BucketObjectArgs{
				Source: NewFileAsset("foo.txt"),
			},
			expected: resource.NewObjectProperty(resource.PropertyMap{
				"source": resource.NewAssetProperty(&resource.Asset{
					Path: "foo.txt",
				}),
			}),
		},
		{
			name: "bucket object with file archive",
			input: &BucketObjectArgs{
				Source: NewFileArchive("bar.zip"),
			},
			expected: resource.NewObjectProperty(resource.PropertyMap{
				"source": resource.NewArchiveProperty(&resource.Archive{
					Path: "bar.zip",
				}),
			}),
		},
		{
			name: "bucket object with file asset output",
			input: &BucketObjectArgs{
				Source: fileAssetOutput,
			},
			expected: resource.NewObjectProperty(resource.PropertyMap{
				"source": resource.NewAssetProperty(&resource.Asset{
					Path: "foo.txt",
				}),
			}),
		},
		{
			name: "bucket object with file asset secret output",
			input: &BucketObjectArgs{
				Source: fileAssetSecretOutput,
			},
			expected: resource.NewObjectProperty(resource.PropertyMap{
				"source": resource.NewOutputProperty(resource.Output{
					Element: resource.NewAssetProperty(&resource.Asset{
						Path: "foo.txt",
					}),
					Known:  true,
					Secret: true,
				}),
			}),
		},
		{
			name: "bucket object with file asset with deps",
			input: &BucketObjectArgs{
				Source: fileAssetOutputDeps,
			},
			expected: resource.NewObjectProperty(resource.PropertyMap{
				"source": resource.NewOutputProperty(resource.Output{
					Element: resource.NewAssetProperty(&resource.Asset{
						Path: "foo.txt",
					}),
					Known:        true,
					Dependencies: []resource.URN{"fakeURN"},
				}),
			}),
		},
		{
			name: "resource",
			input: &MyResourceArgs{
				Res: NewResourceInput(newSimpleCustomResource(ctx, "fakeURN", "fakeID")),
			},
			expected: resource.NewObjectProperty(resource.PropertyMap{
				"res": resource.NewResourceReferenceProperty(resource.ResourceReference{
					URN: "fakeURN",
					ID:  resource.NewStringProperty("fakeID"),
				}),
			}),
		},
		{
			name: "nested output",
			input: &MyNestedOutputArgs{
				Nested: nestedOutput,
			},
			expected: resource.NewObjectProperty(resource.PropertyMap{
				"nested": resource.NewAssetProperty(&resource.Asset{
					Path: "foo.txt",
				}),
			}),
		},
		{
			name: "nested ptr output",
			input: &MyNestedOutputArgs{
				Nested: nestedPtrOutput,
			},
			expected: resource.NewObjectProperty(resource.PropertyMap{
				"nested": resource.NewAssetProperty(&resource.Asset{
					Path: "foo.txt",
				}),
			}),
		},
		{
			name: "nested nested output",
			input: &MyNestedOutputArgs{
				Nested: nestedNestedOutput,
			},
			expected: resource.NewObjectProperty(resource.PropertyMap{
				"nested": resource.NewAssetProperty(&resource.Asset{
					Path: "foo.txt",
				}),
			}),
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inputs := Map{"value": tt.input}
			expected := resource.PropertyMap{"value": tt.expected}

			actual, _, _, err := marshalInputs(inputs)
			assert.NoError(t, err)
			assert.Equal(t, expected, actual)
		})
	}
}

type rubberTreeArgs struct {
	Size *TreeSize `pulumi:"size"`
}
type RubberTreeArgs struct {
	Size TreeSizePtrInput
}

func (RubberTreeArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*rubberTreeArgs)(nil)).Elem()
}

type TreeSize string

const (
	TreeSizeSmall  = TreeSize("small")
	TreeSizeMedium = TreeSize("medium")
	TreeSizeLarge  = TreeSize("large")
)

func (TreeSize) ElementType() reflect.Type {
	return reflect.TypeOf((*TreeSize)(nil)).Elem()
}

func (e TreeSize) ToTreeSizeOutput() TreeSizeOutput {
	return ToOutput(e).(TreeSizeOutput)
}

func (e TreeSize) ToTreeSizeOutputWithContext(ctx context.Context) TreeSizeOutput {
	return ToOutputWithContext(ctx, e).(TreeSizeOutput)
}

func (e TreeSize) ToTreeSizePtrOutput() TreeSizePtrOutput {
	return e.ToTreeSizePtrOutputWithContext(context.Background())
}

func (e TreeSize) ToTreeSizePtrOutputWithContext(ctx context.Context) TreeSizePtrOutput {
	return e.ToTreeSizeOutputWithContext(ctx).ToTreeSizePtrOutputWithContext(ctx)
}

func (e TreeSize) ToStringOutput() StringOutput {
	return ToOutput(String(e)).(StringOutput)
}

func (e TreeSize) ToStringOutputWithContext(ctx context.Context) StringOutput {
	return ToOutputWithContext(ctx, String(e)).(StringOutput)
}

func (e TreeSize) ToStringPtrOutput() StringPtrOutput {
	return String(e).ToStringPtrOutputWithContext(context.Background())
}

func (e TreeSize) ToStringPtrOutputWithContext(ctx context.Context) StringPtrOutput {
	return String(e).ToStringOutputWithContext(ctx).ToStringPtrOutputWithContext(ctx)
}

type TreeSizeOutput struct{ *OutputState }

func (TreeSizeOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*TreeSize)(nil)).Elem()
}

func (o TreeSizeOutput) ToTreeSizeOutput() TreeSizeOutput {
	return o
}

func (o TreeSizeOutput) ToTreeSizeOutputWithContext(ctx context.Context) TreeSizeOutput {
	return o
}

func (o TreeSizeOutput) ToTreeSizePtrOutput() TreeSizePtrOutput {
	return o.ToTreeSizePtrOutputWithContext(context.Background())
}

func (o TreeSizeOutput) ToTreeSizePtrOutputWithContext(ctx context.Context) TreeSizePtrOutput {
	return o.ApplyTWithContext(ctx, func(_ context.Context, v TreeSize) *TreeSize {
		return &v
	}).(TreeSizePtrOutput)
}

func (o TreeSizeOutput) ToStringOutput() StringOutput {
	return o.ToStringOutputWithContext(context.Background())
}

func (o TreeSizeOutput) ToStringOutputWithContext(ctx context.Context) StringOutput {
	return o.ApplyTWithContext(ctx, func(_ context.Context, e TreeSize) string {
		return string(e)
	}).(StringOutput)
}

func (o TreeSizeOutput) ToStringPtrOutput() StringPtrOutput {
	return o.ToStringPtrOutputWithContext(context.Background())
}

func (o TreeSizeOutput) ToStringPtrOutputWithContext(ctx context.Context) StringPtrOutput {
	return o.ApplyTWithContext(ctx, func(_ context.Context, e TreeSize) *string {
		v := string(e)
		return &v
	}).(StringPtrOutput)
}

type TreeSizePtrOutput struct{ *OutputState }

func (TreeSizePtrOutput) ElementType() reflect.Type {
	return reflect.TypeOf((**TreeSize)(nil)).Elem()
}

func (o TreeSizePtrOutput) ToTreeSizePtrOutput() TreeSizePtrOutput {
	return o
}

func (o TreeSizePtrOutput) ToTreeSizePtrOutputWithContext(ctx context.Context) TreeSizePtrOutput {
	return o
}

func (o TreeSizePtrOutput) Elem() TreeSizeOutput {
	return o.ApplyT(func(v *TreeSize) TreeSize {
		if v != nil {
			return *v
		}
		var ret TreeSize
		return ret
	}).(TreeSizeOutput)
}

func (o TreeSizePtrOutput) ToStringPtrOutput() StringPtrOutput {
	return o.ToStringPtrOutputWithContext(context.Background())
}

func (o TreeSizePtrOutput) ToStringPtrOutputWithContext(ctx context.Context) StringPtrOutput {
	return o.ApplyTWithContext(ctx, func(_ context.Context, e *TreeSize) *string {
		if e == nil {
			return nil
		}
		v := string(*e)
		return &v
	}).(StringPtrOutput)
}

type TreeSizeInput interface {
	Input

	ToTreeSizeOutput() TreeSizeOutput
	ToTreeSizeOutputWithContext(context.Context) TreeSizeOutput
}

var treeSizePtrType = reflect.TypeOf((**TreeSize)(nil)).Elem()

type TreeSizePtrInput interface {
	Input

	ToTreeSizePtrOutput() TreeSizePtrOutput
	ToTreeSizePtrOutputWithContext(context.Context) TreeSizePtrOutput
}

type treeSizePtr string

func TreeSizePtr(v string) TreeSizePtrInput {
	return (*treeSizePtr)(&v)
}

func (*treeSizePtr) ElementType() reflect.Type {
	return treeSizePtrType
}

func (in *treeSizePtr) ToTreeSizePtrOutput() TreeSizePtrOutput {
	return ToOutput(in).(TreeSizePtrOutput)
}

func (in *treeSizePtr) ToTreeSizePtrOutputWithContext(ctx context.Context) TreeSizePtrOutput {
	return ToOutputWithContext(ctx, in).(TreeSizePtrOutput)
}

type TreeSizeMapInput interface {
	Input

	ToTreeSizeMapOutput() TreeSizeMapOutput
	ToTreeSizeMapOutputWithContext(context.Context) TreeSizeMapOutput
}

type TreeSizeMap map[string]TreeSize

func (TreeSizeMap) ElementType() reflect.Type {
	return reflect.TypeOf((*map[string]TreeSize)(nil)).Elem()
}

func (i TreeSizeMap) ToTreeSizeMapOutput() TreeSizeMapOutput {
	return i.ToTreeSizeMapOutputWithContext(context.Background())
}

func (i TreeSizeMap) ToTreeSizeMapOutputWithContext(ctx context.Context) TreeSizeMapOutput {
	return ToOutputWithContext(ctx, i).(TreeSizeMapOutput)
}

type TreeSizeMapOutput struct{ *OutputState }

func (TreeSizeMapOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*map[string]TreeSize)(nil)).Elem()
}

func (o TreeSizeMapOutput) ToTreeSizeMapOutput() TreeSizeMapOutput {
	return o
}

func (o TreeSizeMapOutput) ToTreeSizeMapOutputWithContext(ctx context.Context) TreeSizeMapOutput {
	return o
}

func (o TreeSizeMapOutput) MapIndex(k StringInput) TreeSizeOutput {
	return All(o, k).ApplyT(func(vs []interface{}) TreeSize {
		return vs[0].(map[string]TreeSize)[vs[1].(string)]
	}).(TreeSizeOutput)
}

func TestOutputValueMarshallingEnums(t *testing.T) {
	t.Parallel()

	_, err := NewContext(context.Background(), RunInfo{})
	assert.NoError(t, err)

	RegisterOutputType(TreeSizeOutput{})
	RegisterOutputType(TreeSizePtrOutput{})
	RegisterOutputType(TreeSizeMapOutput{})

	tests := []struct {
		name     string
		input    Input
		expected resource.PropertyValue
	}{
		{
			name: "empty",
			input: &RubberTreeArgs{
				Size: TreeSize("medium"),
			},
			expected: resource.NewObjectProperty(resource.PropertyMap{
				"size": resource.NewStringProperty("medium"),
			}),
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inputs := Map{"value": tt.input}
			expected := resource.PropertyMap{"value": tt.expected}

			actual, _, _, err := marshalInputs(inputs)
			assert.NoError(t, err)
			assert.Equal(t, expected, actual)
		})
	}
}

func TestMarshalInputsPropertyDependencies(t *testing.T) {
	t.Parallel()

	pmap, pdeps, deps, err := marshalInputs(testInputs{
		S: String("a string"),
		A: Bool(true),
	})
	assert.NoError(t, err)
	assert.Equal(t, resource.PropertyMap{
		"s": resource.NewStringProperty("a string"),
		"a": resource.NewBoolProperty(true),
	}, pmap)
	assert.Nil(t, deps)
	// Expect a non-empty property deps map, even when there aren't any deps.
	assert.Equal(t, map[string][]URN{"s": nil, "a": nil}, pdeps)
}
