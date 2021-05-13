//nolint: goconst
package pulumi

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

type testMonitor struct {
	CallF        func(args MockCallArgs) (resource.PropertyMap, error)
	NewResourceF func(args MockResourceArgs) (string, resource.PropertyMap, error)
}

func (m *testMonitor) Call(args MockCallArgs) (resource.PropertyMap, error) {
	if m.CallF == nil {
		return resource.PropertyMap{}, nil
	}
	return m.CallF(args)
}

func (m *testMonitor) NewResource(args MockResourceArgs) (string, resource.PropertyMap, error) {

	if m.NewResourceF == nil {
		return args.Name, resource.PropertyMap{}, nil
	}
	return m.NewResourceF(args)
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

type testResource3 struct {
	CustomResourceState

	Outputs MapOutput `pulumi:""`
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
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {

			assert.Equal(t, "test:resource:type", args.TypeToken)
			assert.Equal(t, "resA", args.Name)
			assert.True(t, args.Inputs.DeepEquals(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo":  "oof",
				"bar":  "rab",
				"baz":  "zab",
				"bang": "gnab",
			})))
			assert.Equal(t, "", args.Provider)
			assert.Equal(t, "", args.ID)

			return "someID", resource.PropertyMap{"foo": resource.NewStringProperty("qux")}, nil
		},
	}

	err := RunErr(func(ctx *Context) error {
		// Test struct-tag-based marshaling.
		var res testResource2
		err := ctx.RegisterResource("test:resource:type", "resA", &testResource2Inputs{
			Foo:  String("oof"),
			Bar:  String("rab"),
			Baz:  String("zab"),
			Bang: String("gnab"),
		}, &res)
		assert.NoError(t, err)

		id, known, secret, deps, err := await(res.ID())
		assert.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, []Resource{&res}, deps)
		assert.Equal(t, ID("someID"), id)

		urn, known, secret, deps, err := await(res.URN())
		assert.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, []Resource{&res}, deps)
		assert.NotEqual(t, "", urn)

		foo, known, secret, deps, err := await(res.Foo)
		assert.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, []Resource{&res}, deps)
		assert.Equal(t, "qux", foo)

		// Test map marshaling.
		var res2 testResource3
		err = ctx.RegisterResource("test:resource:type", "resA", Map{
			"foo":  String("oof"),
			"bar":  String("rab"),
			"baz":  String("zab"),
			"bang": String("gnab"),
		}, &res2)
		assert.NoError(t, err)

		id, known, secret, deps, err = await(res2.ID())
		assert.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, []Resource{&res2}, deps)
		assert.Equal(t, ID("someID"), id)

		urn, known, secret, deps, err = await(res2.URN())
		assert.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, []Resource{&res2}, deps)
		assert.NotEqual(t, "", urn)

		outputs, known, secret, deps, err := await(res2.Outputs)
		assert.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, []Resource{&res2}, deps)
		assert.Equal(t, map[string]interface{}{"foo": "qux"}, outputs)

		return nil
	}, WithMocks("project", "stack", mocks))
	assert.NoError(t, err)
}

func TestReadResource(t *testing.T) {
	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {

			assert.Equal(t, "test:resource:type", args.TypeToken)
			assert.Equal(t, "resA", args.Name)
			assert.True(t, args.Inputs.DeepEquals(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": "oof",
			})))
			assert.Equal(t, "", args.Provider)
			assert.Equal(t, "someID", args.ID)

			return args.ID, resource.PropertyMap{"foo": resource.NewStringProperty("qux")}, nil
		},
	}

	err := RunErr(func(ctx *Context) error {
		// Test struct-tag-based marshaling.
		var res testResource2
		err := ctx.ReadResource("test:resource:type", "resA", ID("someID"), &testResource2Inputs{
			Foo: String("oof"),
		}, &res)
		assert.NoError(t, err)

		id, known, secret, deps, err := await(res.ID())
		assert.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, []Resource{&res}, deps)
		assert.Equal(t, ID("someID"), id)

		urn, known, secret, deps, err := await(res.URN())
		assert.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, []Resource{&res}, deps)
		assert.NotEqual(t, "", urn)

		foo, known, secret, deps, err := await(res.Foo)
		assert.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, []Resource{&res}, deps)
		assert.Equal(t, "qux", foo)

		// Test map marshaling.
		var res2 testResource2
		err = ctx.ReadResource("test:resource:type", "resA", ID("someID"), Map{
			"foo": String("oof"),
		}, &res2)
		assert.NoError(t, err)

		foo, known, secret, deps, err = await(res2.Foo)
		assert.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, []Resource{&res2}, deps)
		assert.Equal(t, "qux", foo)

		return nil
	}, WithMocks("project", "stack", mocks))
	assert.NoError(t, err)
}

func TestInvoke(t *testing.T) {
	mocks := &testMonitor{
		CallF: func(args MockCallArgs) (resource.PropertyMap, error) {
			assert.Equal(t, "test:index:func", args.Token)
			assert.True(t, args.Args.DeepEquals(resource.NewPropertyMapFromMap(map[string]interface{}{
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
		// Test struct unmarshaling.
		var result invokeResult
		err := ctx.Invoke("test:index:func", &invokeArgs{
			Bang: "gnab",
			Bar:  "rab",
		}, &result)
		assert.NoError(t, err)
		assert.Equal(t, "oof", result.Foo)
		assert.Equal(t, "zab", result.Baz)

		// Test map unmarshaling.
		var result2 map[string]interface{}
		err = ctx.Invoke("test:index:func", &invokeArgs{
			Bang: "gnab",
			Bar:  "rab",
		}, &result2)
		assert.NoError(t, err)
		assert.Equal(t, "oof", result2["foo"].(string))
		assert.Equal(t, "zab", result2["baz"].(string))

		return nil
	}, WithMocks("project", "stack", mocks))
	assert.NoError(t, err)
}

type testInstanceResource struct {
	CustomResourceState
}

type testInstanceResourceArgs struct {
}

type testInstanceResourceInputs struct {
}

func (*testInstanceResourceInputs) ElementType() reflect.Type {
	return reflect.TypeOf((*testInstanceResourceArgs)(nil)).Elem()
}

type testInstanceResourceInput interface {
	Input

	ToTestInstanceResourceOutput() testInstanceResourceOutput
	ToTestInstanceResourceOutputWithContext(ctx context.Context) testInstanceResourceOutput
}

func (*testInstanceResource) ElementType() reflect.Type {
	return reflect.TypeOf((*testInstanceResource)(nil)).Elem()
}

func (i *testInstanceResource) ToTestInstanceResourceOutput() testInstanceResourceOutput {
	return i.ToTestInstanceResourceOutputWithContext(context.Background())
}

func (i *testInstanceResource) ToTestInstanceResourceOutputWithContext(ctx context.Context) testInstanceResourceOutput {
	return ToOutputWithContext(ctx, i).(testInstanceResourceOutput)
}

type testInstanceResourceOutput struct {
	*OutputState
}

func (testInstanceResourceOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*testInstanceResource)(nil)).Elem()
}

func (o testInstanceResourceOutput) ToTestInstanceResourceOutput() testInstanceResourceOutput {
	return o
}

func (o testInstanceResourceOutput) ToTestInstanceResourceOutputWithContext(
	ctx context.Context) testInstanceResourceOutput {
	return o
}

type testMyCustomResource struct {
	CustomResourceState

	Instance testInstanceResourceOutput `pulumi:"instance"`
}

type testMyCustomResourceArgs struct {
	Instance testInstanceResource `pulumi:"instance"`
}

type testMyCustomResourceInputs struct {
	Instance testInstanceResourceInput
}

func (testMyCustomResourceInputs) ElementType() reflect.Type {
	return reflect.TypeOf((*testMyCustomResourceArgs)(nil)).Elem()
}

type module int

func (module) Construct(ctx *Context, name, typ, urn string) (Resource, error) {
	switch typ {
	case "pkg:index:Instance":
		var instance testInstanceResource
		return &instance, nil
	default:
		return nil, errors.Errorf("unknown resource type %s", typ)
	}
}

func (module) Version() semver.Version {
	return semver.Version{}
}

func TestRegisterResourceWithResourceReferences(t *testing.T) {
	RegisterOutputType(testInstanceResourceOutput{})

	RegisterResourceModule("pkg", "index", module(0))

	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {

			switch args.TypeToken {
			case "pkg:index:Instance":
				return "i-1234567890abcdef0", resource.PropertyMap{}, nil
			case "pkg:index:MyCustom":
				return args.Name + "_id", args.Inputs, nil
			default:
				return "", nil, errors.Errorf("unknown resource %s", args.TypeToken)
			}
		},
	}

	err := RunErr(func(ctx *Context) error {
		var instance testInstanceResource
		err := ctx.RegisterResource("pkg:index:Instance", "instance", &testInstanceResourceInputs{}, &instance)
		assert.NoError(t, err)

		var mycustom testMyCustomResource
		err = ctx.RegisterResource("pkg:index:MyCustom", "mycustom", &testMyCustomResourceInputs{
			Instance: &instance,
		}, &mycustom)
		assert.NoError(t, err)

		_, _, secret, _, err := await(mycustom.Instance)
		assert.NoError(t, err)
		assert.False(t, secret)

		return nil
	}, WithMocks("project", "stack", mocks))
	assert.NoError(t, err)
}

func TestWaitOrphanedApply(t *testing.T) {
	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			return "someID", resource.PropertyMap{"foo": resource.NewStringProperty("qux")}, nil
		},
	}

	var theID ID
	err := RunErr(func(ctx *Context) error {
		var res testResource2
		err := ctx.RegisterResource("test:resource:type", "resA", &testResource2Inputs{
			Foo: String("oof"),
		}, &res)
		assert.NoError(t, err)

		res.ID().ApplyT(func(id ID) int {
			theID = id
			return 0
		})

		return nil
	}, WithMocks("project", "stack", mocks))
	assert.NoError(t, err)

	assert.Equal(t, ID("someID"), theID)
}

func TestWaitOrphanedNestedApply(t *testing.T) {
	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			return "someID", resource.PropertyMap{"foo": resource.NewStringProperty("qux")}, nil
		},
	}

	var theID ID
	err := RunErr(func(ctx *Context) error {
		var res testResource2
		err := ctx.RegisterResource("test:resource:type", "resA", &testResource2Inputs{
			Foo: String("oof"),
		}, &res)
		assert.NoError(t, err)

		ctx.Export("urn", res.URN().ApplyT(func(urn URN) URN {
			res.ID().ApplyT(func(id ID) int {
				theID = id
				return 0
			})
			return urn
		}))

		return nil
	}, WithMocks("project", "stack", mocks))
	assert.NoError(t, err)

	assert.Equal(t, ID("someID"), theID)
}

func TestWaitOrphanedAllApply(t *testing.T) {
	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			return "someID", resource.PropertyMap{"foo": resource.NewStringProperty("qux")}, nil
		},
	}

	var theURN URN
	var theID ID
	err := RunErr(func(ctx *Context) error {
		var res testResource2
		err := ctx.RegisterResource("test:resource:type", "resA", &testResource2Inputs{
			Foo: String("oof"),
		}, &res)
		assert.NoError(t, err)

		All(res.URN(), res.ID()).ApplyT(func(vs []interface{}) int {
			theURN, _ = vs[0].(URN)
			theID, _ = vs[1].(ID)
			return 0
		})

		return nil
	}, WithMocks("project", "stack", mocks))
	assert.NoError(t, err)

	assert.NotEqual(t, URN(""), theURN)
	assert.Equal(t, ID("someID"), theID)
}

func TestWaitOrphanedAnyApply(t *testing.T) {
	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			return "someID", resource.PropertyMap{"foo": resource.NewStringProperty("qux")}, nil
		},
	}

	var theURN URN
	var theID ID
	err := RunErr(func(ctx *Context) error {
		var res testResource2
		err := ctx.RegisterResource("test:resource:type", "resA", &testResource2Inputs{
			Foo: String("oof"),
		}, &res)
		assert.NoError(t, err)

		Any(map[string]Output{
			"urn": res.URN(),
			"id":  res.ID(),
		}).ApplyT(func(v interface{}) int {
			m := v.(map[string]Output)
			m["urn"].ApplyT(func(urn URN) int {
				theURN = urn
				return 0
			})
			m["id"].ApplyT(func(id ID) int {
				theID = id
				return 0
			})
			return 0
		})

		return nil
	}, WithMocks("project", "stack", mocks))
	assert.NoError(t, err)

	assert.NotEqual(t, URN(""), theURN)
	assert.Equal(t, ID("someID"), theID)
}

func TestWaitOrphanedContextAllApply(t *testing.T) {
	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			return "someID", resource.PropertyMap{"foo": resource.NewStringProperty("qux")}, nil
		},
	}

	var theURN URN
	var theID ID
	err := RunErr(func(ctx *Context) error {
		var res testResource2
		err := ctx.RegisterResource("test:resource:type", "resA", &testResource2Inputs{
			Foo: String("oof"),
		}, &res)
		assert.NoError(t, err)

		All(res.URN(), res.ID()).ApplyT(func(vs []interface{}) int {
			theURN, _ = vs[0].(URN)
			theID, _ = vs[1].(ID)
			return 0
		})

		return nil
	}, WithMocks("project", "stack", mocks))
	assert.NoError(t, err)

	assert.NotEqual(t, URN(""), theURN)
	assert.Equal(t, ID("someID"), theID)
}

func TestWaitOrphanedContextAnyApply(t *testing.T) {
	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			return "someID", resource.PropertyMap{"foo": resource.NewStringProperty("qux")}, nil
		},
	}

	var theURN URN
	var theID ID
	err := RunErr(func(ctx *Context) error {
		var res testResource2
		err := ctx.RegisterResource("test:resource:type", "resA", &testResource2Inputs{
			Foo: String("oof"),
		}, &res)
		assert.NoError(t, err)

		Any(map[string]Output{
			"urn": res.URN(),
			"id":  res.ID(),
		}).ApplyT(func(v interface{}) int {
			m := v.(map[string]Output)
			m["urn"].ApplyT(func(urn URN) int {
				theURN = urn
				return 0
			})
			m["id"].ApplyT(func(id ID) int {
				theID = id
				return 0
			})
			return 0
		})

		return nil
	}, WithMocks("project", "stack", mocks))
	assert.NoError(t, err)

	assert.NotEqual(t, URN(""), theURN)
	assert.Equal(t, ID("someID"), theID)
}

func TestWaitOrphanedResource(t *testing.T) {
	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			return "someID", resource.PropertyMap{"foo": resource.NewStringProperty("qux")}, nil
		},
	}

	var res testResource2
	err := RunErr(func(ctx *Context) error {
		err := ctx.RegisterResource("test:resource:type", "resA", &testResource2Inputs{
			Foo: String("oof"),
		}, &res)
		assert.NoError(t, err)

		return nil
	}, WithMocks("project", "stack", mocks))
	assert.NoError(t, err)

	assert.Equal(t, uint32(outputResolved), res.urn.state)
	assert.Equal(t, uint32(outputResolved), res.id.state)
}

func TestWaitResourceInsideApply(t *testing.T) {
	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			return "someID", resource.PropertyMap{"foo": resource.NewStringProperty("qux")}, nil
		},
	}

	var innerRes testResource2
	err := RunErr(func(ctx *Context) error {
		var outerRes testResource2
		err := ctx.RegisterResource("test:resource:type", "resA", &testResource2Inputs{
			Foo: String("oof"),
		}, &outerRes)
		assert.NoError(t, err)

		outerRes.ID().ApplyT(func(_ ID) error {
			return ctx.RegisterResource("test:resource:type", "resB", &testResource2Inputs{
				Foo: String("foo"),
			}, &innerRes)
		})

		return nil
	}, WithMocks("project", "stack", mocks))
	assert.NoError(t, err)

	assert.Equal(t, uint32(outputResolved), innerRes.urn.state)
	assert.Equal(t, uint32(outputResolved), innerRes.id.state)
}

func TestWaitOrphanedApplyOnResourceInsideApply(t *testing.T) {
	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			return "someID", resource.PropertyMap{"foo": resource.NewStringProperty("qux")}, nil
		},
	}

	var theID ID
	err := RunErr(func(ctx *Context) error {
		var outerRes testResource2
		err := ctx.RegisterResource("test:resource:type", "resA", &testResource2Inputs{
			Foo: String("oof"),
		}, &outerRes)
		assert.NoError(t, err)

		outerRes.ID().ApplyT(func(_ ID) int {
			var innerRes testResource2
			err := ctx.RegisterResource("test:resource:type", "resB", &testResource2Inputs{
				Foo: String("foo"),
			}, &innerRes)
			assert.NoError(t, err)

			innerRes.ID().ApplyT(func(id ID) int {
				theID = id
				return 0
			})
			return 0
		})

		return nil
	}, WithMocks("project", "stack", mocks))
	assert.NoError(t, err)

	assert.Equal(t, ID("someID"), theID)
}

func TestWaitRecursiveApply(t *testing.T) {
	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			return "someID", resource.PropertyMap{"foo": resource.NewStringProperty("qux")}, nil
		},
	}

	resources := 0

	var newResource func(ctx *Context, n int)
	newResource = func(ctx *Context, n int) {
		if n == 0 {
			return
		}

		var res testResource2
		err := ctx.RegisterResource("test:resource:type", fmt.Sprintf("res%d", n), &testResource2Inputs{
			Foo: String(fmt.Sprintf("%d", n)),
		}, &res)
		assert.NoError(t, err)

		resources++
		res.ID().ApplyT(func(_ ID) int {
			newResource(ctx, n-1)
			return 0
		})
	}

	err := RunErr(func(ctx *Context) error {
		newResource(ctx, 10)
		return nil
	}, WithMocks("project", "stack", mocks))
	assert.NoError(t, err)

	assert.Equal(t, 10, resources)
}

func TestWaitOrphanedManualOutput(t *testing.T) {
	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			return "someID", resource.PropertyMap{"foo": resource.NewStringProperty("qux")}, nil
		},
	}

	output := make(chan Output)
	doResolve := make(chan bool)
	done := make(chan bool)
	go func() {
		err := RunErr(func(ctx *Context) error {
			out, resolve, _ := ctx.NewOutput()
			go func() {
				<-doResolve
				resolve("foo")
			}()
			output <- out

			return nil
		}, WithMocks("project", "stack", mocks))
		assert.NoError(t, err)

		close(done)
	}()

	state := (<-output).getState()
	assert.Equal(t, uint32(outputPending), state.state)
	close(doResolve)

	<-done
	assert.Equal(t, uint32(outputResolved), state.state)
	assert.Equal(t, "foo", state.value)
}

func TestWaitOrphanedDeprecatedOutput(t *testing.T) {
	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			return "someID", resource.PropertyMap{"foo": resource.NewStringProperty("qux")}, nil
		},
	}

	var output Output
	err := RunErr(func(ctx *Context) error {
		output, _, _ = NewOutput()

		return nil
	}, WithMocks("project", "stack", mocks))
	assert.NoError(t, err)

	state := output.getState()
	assert.Equal(t, uint32(outputPending), state.state)
}
