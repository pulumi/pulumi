package pulumi

import (
	"reflect"
	"testing"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/stretchr/testify/assert"
)

type testMonitor struct {
	CallF        func(tok string, args resource.PropertyMap, provider string) (resource.PropertyMap, error)
	NewResourceF func(typeToken, name string, inputs resource.PropertyMap,
		provider, id string) (string, resource.PropertyMap, error)
}

func (m *testMonitor) Call(tok string, args resource.PropertyMap, provider string) (resource.PropertyMap, error) {
	if m.CallF == nil {
		return resource.PropertyMap{}, nil
	}
	return m.CallF(tok, args, provider)
}

func (m *testMonitor) NewResource(typeToken, name string, inputs resource.PropertyMap,
	provider, id string) (string, resource.PropertyMap, error) {

	if m.NewResourceF == nil {
		return name, resource.PropertyMap{}, nil
	}
	return m.NewResourceF(typeToken, name, inputs, provider, id)
}

type testResource2 struct {
	CustomResourceState

	Foo StringOutput `pulumi:"foo"`
}

type testResource2Args struct {
	Foo  string `pulumi:"foo"`
	Bar  string `pulumi:"bar"`
	Baz  string `pulumi:"baz"`
	Bang string `pulumi:"bang"`
}

type testResource2Inputs struct {
	Foo  StringInput
	Bar  StringInput
	Baz  StringInput
	Bang StringInput
}

func (*testResource2Inputs) ElementType() reflect.Type {
	return reflect.TypeOf((*testResource2Args)(nil))
}

type invokeArgs struct {
	Bang string `pulumi:"bang"`
	Bar  string `pulumi:"bar"`
}

type invokeResult struct {
	Foo string `pulumi:"foo"`
	Baz string `pulumi:"baz"`
}

func TestRegisterResource(t *testing.T) {
	mocks := &testMonitor{
		NewResourceF: func(typeToken, name string, inputs resource.PropertyMap,
			provider, id string) (string, resource.PropertyMap, error) {

			assert.Equal(t, "test:resource:type", typeToken)
			assert.Equal(t, "resA", name)
			assert.True(t, inputs.DeepEquals(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo":  "oof",
				"bar":  "rab",
				"baz":  "zab",
				"bang": "gnab",
			})))
			assert.Equal(t, "", provider)
			assert.Equal(t, "", id)

			return "someID", resource.PropertyMap{"foo": resource.NewStringProperty("qux")}, nil
		},
	}

	err := RunErr(func(ctx *Context) error {
		var res testResource2
		err := ctx.RegisterResource("test:resource:type", "resA", &testResource2Inputs{
			Foo:  String("oof"),
			Bar:  String("rab"),
			Baz:  String("zab"),
			Bang: String("gnab"),
		}, &res)
		assert.NoError(t, err)

		id, known, secret, err := await(res.ID())
		assert.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, ID("someID"), id)

		urn, known, secret, err := await(res.URN())
		assert.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.NotEqual(t, "", urn)

		foo, known, secret, err := await(res.Foo)
		assert.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, "qux", foo)

		return nil
	}, WithMocks("project", "stack", mocks))
	assert.NoError(t, err)
}

func TestReadResource(t *testing.T) {
	mocks := &testMonitor{
		NewResourceF: func(typeToken, name string, state resource.PropertyMap,
			provider, id string) (string, resource.PropertyMap, error) {

			assert.Equal(t, "test:resource:type", typeToken)
			assert.Equal(t, "resA", name)
			assert.True(t, state.DeepEquals(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": "oof",
			})))
			assert.Equal(t, "", provider)
			assert.Equal(t, "someID", id)

			return id, resource.PropertyMap{"foo": resource.NewStringProperty("qux")}, nil
		},
	}

	err := RunErr(func(ctx *Context) error {
		var res testResource2
		err := ctx.ReadResource("test:resource:type", "resA", ID("someID"), &testResource2Inputs{
			Foo: String("oof"),
		}, &res)
		assert.NoError(t, err)

		id, known, secret, err := await(res.ID())
		assert.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, ID("someID"), id)

		urn, known, secret, err := await(res.URN())
		assert.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.NotEqual(t, "", urn)

		foo, known, secret, err := await(res.Foo)
		assert.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, "qux", foo)

		return nil
	}, WithMocks("project", "stack", mocks))
	assert.NoError(t, err)
}

func TestInvoke(t *testing.T) {
	mocks := &testMonitor{
		CallF: func(token string, args resource.PropertyMap, provider string) (resource.PropertyMap, error) {
			assert.Equal(t, "test:index:func", token)
			assert.True(t, args.DeepEquals(resource.NewPropertyMapFromMap(map[string]interface{}{
				"bang": "gnab",
				"bar":  "rab",
			})))
			return resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": "oof",
				"baz": "zab",
			}), nil
		},
	}

	err := RunErr(func(ctx *Context) error {
		var result invokeResult
		err := ctx.Invoke("test:index:func", &invokeArgs{
			Bang: "gnab",
			Bar:  "rab",
		}, &result)

		assert.NoError(t, err)
		assert.Equal(t, "oof", result.Foo)
		assert.Equal(t, "zab", result.Baz)

		return nil
	}, WithMocks("project", "stack", mocks))
	assert.NoError(t, err)
}
