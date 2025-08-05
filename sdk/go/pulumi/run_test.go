// Copyright 2020-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pulumi

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/internal"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// WithDryRun is an internal, test-only option
// that controls whether a Context is in dryRun mode.
func WithDryRun(dryRun bool) RunOption {
	return func(r *RunInfo) {
		r.DryRun = dryRun
	}
}

// WrapResourceMonitorClient is an internal, test-only option
// that wraps the ResourceMonitorClient used by Context.
func WrapResourceMonitorClient(
	wrap func(pulumirpc.ResourceMonitorClient) pulumirpc.ResourceMonitorClient,
) RunOption {
	return func(ri *RunInfo) {
		ri.wrapResourceMonitorClient = wrap
	}
}

type testMonitor struct {
	// Actually an "Invoke" by provider parlance, but is named so to be consistent with the interface.
	CallF func(args MockCallArgs) (resource.PropertyMap, error)
	// Actually an "Call" by provider parlance, but is named so to be consistent with the interface.
	MethodCallF               func(args MockCallArgs) (resource.PropertyMap, error)
	NewResourceF              func(args MockResourceArgs) (string, resource.PropertyMap, error)
	RegisterResourceOutputsF  func() (*emptypb.Empty, error)
	SignalAndWaitForShutdownF func() (*emptypb.Empty, error)
}

func (m *testMonitor) Call(args MockCallArgs) (resource.PropertyMap, error) {
	if m.CallF == nil {
		return resource.PropertyMap{}, nil
	}
	return m.CallF(args)
}

func (m *testMonitor) MethodCall(args MockCallArgs) (resource.PropertyMap, error) {
	if m.MethodCallF == nil {
		return resource.PropertyMap{}, nil
	}
	return m.MethodCallF(args)
}

func (m *testMonitor) NewResource(args MockResourceArgs) (string, resource.PropertyMap, error) {
	if m.NewResourceF == nil {
		return args.Name, resource.PropertyMap{}, nil
	}
	return m.NewResourceF(args)
}

func (m *testMonitor) RegisterResourceOutputs() (*emptypb.Empty, error) {
	if m.RegisterResourceOutputsF == nil {
		return &emptypb.Empty{}, nil
	}
	return m.RegisterResourceOutputsF()
}

func (m *testMonitor) SignalAndWaitForShutdown() (*emptypb.Empty, error) {
	if m.SignalAndWaitForShutdownF == nil {
		return &emptypb.Empty{}, nil
	}
	return m.SignalAndWaitForShutdownF()
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
	t.Parallel()

	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			switch args.TypeToken {
			case "test:resource:type":
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
			case "test:resource:complextype":
				assert.Equal(t, "resB", args.Name)
				assert.True(t, args.Inputs.DeepEquals(resource.NewPropertyMapFromMap(map[string]interface{}{
					"foo":  "oof",
					"bar":  "rab",
					"baz":  "zab",
					"bang": "gnab",
				})))
				assert.Equal(t, "", args.Provider)
				assert.Equal(t, "", args.ID)

				return "someID", resource.PropertyMap{
					"foo":    resource.NewStringProperty("qux"),
					"secret": resource.MakeSecret(resource.NewStringProperty("shh")),
					"output": resource.MakeOutput(resource.NewStringProperty("known unknown")),
				}, nil
			default:
				assert.Fail(t, "Expected a valid resource type, got %v", args.TypeToken)
				return "someID", nil, nil
			}
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
		require.NoError(t, err)

		id, known, secret, deps, err := await(res.ID())
		require.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, []Resource{&res}, deps)
		assert.Equal(t, ID("someID"), id)

		urn, known, secret, deps, err := await(res.URN())
		require.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, []Resource{&res}, deps)
		assert.NotEqual(t, "", urn)

		foo, known, secret, deps, err := await(res.Foo)
		require.NoError(t, err)
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
		require.NoError(t, err)
		require.NotNil(t, res2.rawOutputs)

		id, known, secret, deps, err = await(res2.ID())
		require.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, []Resource{&res2}, deps)
		assert.Equal(t, ID("someID"), id)

		urn, known, secret, deps, err = await(res2.URN())
		require.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, []Resource{&res2}, deps)
		assert.NotEqual(t, "", urn)

		outputs, known, secret, deps, err := await(res2.Outputs)
		require.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, []Resource{&res2}, deps)
		assert.Equal(t, map[string]interface{}{"foo": "qux"}, outputs)

		// Test raw access to property values:
		var res3 testResource3
		err = ctx.RegisterResource("test:resource:complextype", "resB", Map{
			"foo":  String("oof"),
			"bar":  String("rab"),
			"baz":  String("zab"),
			"bang": String("gnab"),
		}, &res3)
		require.NoError(t, err)
		require.NotNil(t, res3.rawOutputs)
		output := InternalGetRawOutputs(&res3.ResourceState)
		rawOutputsTmp, _, _, _, err := await(output)
		require.NoError(t, err)
		rawOutputs, ok := rawOutputsTmp.(resource.PropertyMap)
		assert.True(t, ok)
		assert.True(t, rawOutputs.HasValue("foo"))
		assert.True(t, rawOutputs.HasValue("secret"))
		assert.True(t, rawOutputs.ContainsSecrets())

		return nil
	}, WithMocks("project", "stack", mocks))
	require.NoError(t, err)
}

func TestReadResource(t *testing.T) {
	t.Parallel()

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
		require.NoError(t, err)

		id, known, secret, deps, err := await(res.ID())
		require.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, []Resource{&res}, deps)
		assert.Equal(t, ID("someID"), id)

		urn, known, secret, deps, err := await(res.URN())
		require.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, []Resource{&res}, deps)
		assert.NotEqual(t, "", urn)

		foo, known, secret, deps, err := await(res.Foo)
		require.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, []Resource{&res}, deps)
		assert.Equal(t, "qux", foo)

		// Test map marshaling.
		var res2 testResource2
		err = ctx.ReadResource("test:resource:type", "resA", ID("someID"), Map{
			"foo": String("oof"),
		}, &res2)
		require.NoError(t, err)

		foo, known, secret, deps, err = await(res2.Foo)
		require.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, []Resource{&res2}, deps)
		assert.Equal(t, "qux", foo)

		return nil
	}, WithMocks("project", "stack", mocks))
	require.NoError(t, err)
}

func TestInvoke(t *testing.T) {
	t.Parallel()

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
		require.NoError(t, err)
		assert.Equal(t, "oof", result.Foo)
		assert.Equal(t, "zab", result.Baz)

		// Test map unmarshaling.
		var result2 map[string]interface{}
		err = ctx.Invoke("test:index:func", &invokeArgs{
			Bang: "gnab",
			Bar:  "rab",
		}, &result2)
		require.NoError(t, err)
		assert.Equal(t, "oof", result2["foo"].(string))
		assert.Equal(t, "zab", result2["baz"].(string))

		return nil
	}, WithMocks("project", "stack", mocks))
	require.NoError(t, err)
}

func TestSignalAndWaitForShutdownNotImplemented(t *testing.T) {
	t.Parallel()

	mocks := &testMonitor{
		// Simulate an old CLI without SignalAndWaitForShutdown
		SignalAndWaitForShutdownF: func() (*emptypb.Empty, error) {
			return &emptypb.Empty{}, status.Error(codes.Unimplemented, "SignalAndWaitForShutdown is not implemented")
		},
	}

	logError := func(ctx *Context, err error) {
		require.Fail(t, "The `Unimplemented` error should be handled gracefully and not reported", err)
	}

	err := runErrInner(func(ctx *Context) error {
		return nil
	}, logError, WithMocks("project", "stack", mocks))

	require.NoError(t, err)
}

func TestSignalAndWaitForShutdownError(t *testing.T) {
	t.Parallel()

	mocks := &testMonitor{
		// Simulate a CLI that returns an error when calling SignalAndWaitForShutdown
		SignalAndWaitForShutdownF: func() (*emptypb.Empty, error) {
			return &emptypb.Empty{}, status.Error(codes.Unknown, "SignalAndWaitForShutdown returned some error")
		},
	}

	errorReported := false

	logError := func(ctx *Context, err error) {
		require.ErrorContains(t, err, "SignalAndWaitForShutdown returned some error")
		errorReported = true
	}

	err := runErrInner(func(ctx *Context) error {
		return nil
	}, logError, WithMocks("project", "stack", mocks))

	require.ErrorContains(t, err, "SignalAndWaitForShutdown returned some error")
	require.True(t, errorReported, "The error should have been reported")
}

type testInstanceResource struct {
	CustomResourceState
}

type testInstanceResourceArgs struct{}

type testInstanceResourceInputs struct{}

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
	ctx context.Context,
) testInstanceResourceOutput {
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
		return nil, fmt.Errorf("unknown resource type %s", typ)
	}
}

func (module) Version() semver.Version {
	return semver.Version{}
}

func TestRegisterResourceWithResourceReferences(t *testing.T) {
	t.Parallel()

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
				return "", nil, fmt.Errorf("unknown resource %s", args.TypeToken)
			}
		},
	}

	err := RunErr(func(ctx *Context) error {
		var instance testInstanceResource
		err := ctx.RegisterResource("pkg:index:Instance", "instance", &testInstanceResourceInputs{}, &instance)
		require.NoError(t, err)

		var mycustom testMyCustomResource
		err = ctx.RegisterResource("pkg:index:MyCustom", "mycustom", &testMyCustomResourceInputs{
			Instance: &instance,
		}, &mycustom)
		require.NoError(t, err)

		_, _, secret, _, err := await(mycustom.Instance)
		require.NoError(t, err)
		assert.False(t, secret)

		return nil
	}, WithMocks("project", "stack", mocks))
	require.NoError(t, err)
}

type testMyRemoteComponentArgs struct {
	Inprop string `pulumi:"inprop"`
}

type testMyRemoteComponentInputs struct {
	Inprop StringInput
}

func (testMyRemoteComponentInputs) ElementType() reflect.Type {
	return reflect.TypeOf((*testMyRemoteComponentArgs)(nil)).Elem()
}

type testMyRemoteComponent struct {
	ResourceState

	Outprop StringOutput `pulumi:"outprop"`
}

func TestRemoteComponent(t *testing.T) {
	t.Parallel()

	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			switch args.TypeToken {
			case "pkg:index:Instance":
				return "i-1234567890abcdef0", resource.PropertyMap{}, nil
			case "pkg:index:MyRemoteComponent":
				outprop := resource.NewStringProperty("output: " + args.Inputs["inprop"].StringValue())
				return args.Name + "_id", resource.PropertyMap{
					"inprop":  args.Inputs["inprop"],
					"outprop": outprop,
				}, nil
			default:
				return "", nil, fmt.Errorf("unknown resource %s", args.TypeToken)
			}
		},
	}

	err := RunErr(func(ctx *Context) error {
		var instance testInstanceResource
		err := ctx.RegisterResource("pkg:index:Instance", "instance", &testInstanceResourceInputs{}, &instance)
		require.NoError(t, err)

		var myremotecomponent testMyRemoteComponent
		err = ctx.RegisterRemoteComponentResource(
			"pkg:index:MyRemoteComponent", "myremotecomponent", &testMyRemoteComponentInputs{
				Inprop: Sprintf("hello: %v", instance.id),
			}, &myremotecomponent)
		require.NoError(t, err)

		val, known, secret, deps, err := await(myremotecomponent.Outprop)
		require.NoError(t, err)
		stringVal, ok := val.(string)
		assert.True(t, ok)
		assert.True(t, strings.HasPrefix(stringVal, "output: hello: "))
		assert.True(t, known)
		assert.False(t, secret)
		assert.Equal(t, []Resource{&myremotecomponent}, deps)

		return nil
	}, WithMocks("project", "stack", mocks))
	require.NoError(t, err)
}

func TestWaitOrphanedApply(t *testing.T) {
	t.Parallel()

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
		require.NoError(t, err)

		res.ID().ApplyT(func(id ID) int {
			theID = id
			return 0
		})

		return nil
	}, WithMocks("project", "stack", mocks))
	require.NoError(t, err)

	assert.Equal(t, ID("someID"), theID)
}

func TestWaitOrphanedNestedApply(t *testing.T) {
	t.Parallel()

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
		require.NoError(t, err)

		ctx.Export("urn", res.URN().ApplyT(func(urn URN) URN {
			res.ID().ApplyT(func(id ID) int {
				theID = id
				return 0
			})
			return urn
		}))

		return nil
	}, WithMocks("project", "stack", mocks))
	require.NoError(t, err)

	assert.Equal(t, ID("someID"), theID)
}

func TestWaitOrphanedAllApply(t *testing.T) {
	t.Parallel()

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
		require.NoError(t, err)

		All(res.URN(), res.ID()).ApplyT(func(vs []interface{}) int {
			theURN, _ = vs[0].(URN)
			theID, _ = vs[1].(ID)
			return 0
		})

		return nil
	}, WithMocks("project", "stack", mocks))
	require.NoError(t, err)

	assert.NotEqual(t, URN(""), theURN)
	assert.Equal(t, ID("someID"), theID)
}

func TestWaitOrphanedAnyApply(t *testing.T) {
	t.Parallel()

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
		require.NoError(t, err)

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
	require.NoError(t, err)

	assert.NotEqual(t, URN(""), theURN)
	assert.Equal(t, ID("someID"), theID)
}

func TestWaitOrphanedContextAllApply(t *testing.T) {
	t.Parallel()

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
		require.NoError(t, err)

		All(res.URN(), res.ID()).ApplyT(func(vs []interface{}) int {
			theURN, _ = vs[0].(URN)
			theID, _ = vs[1].(ID)
			return 0
		})

		return nil
	}, WithMocks("project", "stack", mocks))
	require.NoError(t, err)

	assert.NotEqual(t, URN(""), theURN)
	assert.Equal(t, ID("someID"), theID)
}

func TestWaitOrphanedContextAnyApply(t *testing.T) {
	t.Parallel()

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
		require.NoError(t, err)

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
	require.NoError(t, err)

	assert.NotEqual(t, URN(""), theURN)
	assert.Equal(t, ID("someID"), theID)
}

func TestWaitOrphanedResource(t *testing.T) {
	t.Parallel()

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
		require.NoError(t, err)

		return nil
	}, WithMocks("project", "stack", mocks))
	require.NoError(t, err)

	assert.Equal(t, internal.OutputResolved, internal.GetOutputStatus(res.urn))
	assert.Equal(t, internal.OutputResolved, internal.GetOutputStatus(res.id))
}

func TestWaitResourceInsideApply(t *testing.T) {
	t.Parallel()

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
		require.NoError(t, err)

		outerRes.ID().ApplyT(func(_ ID) error {
			return ctx.RegisterResource("test:resource:type", "resB", &testResource2Inputs{
				Foo: String("foo"),
			}, &innerRes)
		})

		return nil
	}, WithMocks("project", "stack", mocks))
	require.NoError(t, err)

	assert.Equal(t, internal.OutputResolved, internal.GetOutputStatus(innerRes.urn))
	assert.Equal(t, internal.OutputResolved, internal.GetOutputStatus(innerRes.id))
}

func TestWaitOrphanedApplyOnResourceInsideApply(t *testing.T) {
	t.Parallel()

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
		require.NoError(t, err)

		outerRes.ID().ApplyT(func(_ ID) int {
			var innerRes testResource2
			err := ctx.RegisterResource("test:resource:type", "resB", &testResource2Inputs{
				Foo: String("foo"),
			}, &innerRes)
			require.NoError(t, err)

			innerRes.ID().ApplyT(func(id ID) int {
				theID = id
				return 0
			})
			return 0
		})

		return nil
	}, WithMocks("project", "stack", mocks))
	require.NoError(t, err)

	assert.Equal(t, ID("someID"), theID)
}

func TestWaitRecursiveApply(t *testing.T) {
	t.Parallel()

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
			Foo: String(strconv.Itoa(n)),
		}, &res)
		require.NoError(t, err)

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
	require.NoError(t, err)

	assert.Equal(t, 10, resources)
}

func TestWaitOrphanedManualOutput(t *testing.T) {
	t.Parallel()

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
		require.NoError(t, err)

		close(done)
	}()

	state := internal.GetOutputState(<-output)
	assert.Equal(t, internal.OutputPending, internal.GetOutputStatus(state))
	close(doResolve)

	<-done
	assert.Equal(t, internal.OutputResolved, internal.GetOutputStatus(state))
	assert.Equal(t, "foo", internal.GetOutputValue(state))
}

func TestWaitOrphanedDeprecatedOutput(t *testing.T) {
	t.Parallel()

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
	require.NoError(t, err)

	status := internal.GetOutputStatus(output)
	assert.Equal(t, internal.OutputPending, status)
}

func TestExportResource(t *testing.T) {
	t.Parallel()

	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			return "someID", resource.PropertyMap{"foo": resource.NewStringProperty("qux")}, nil
		},
	}

	var anyout Output
	err := RunErr(func(ctx *Context) error {
		var res testResource2
		err := ctx.RegisterResource("test:resource:type", "resA", &testResource2Inputs{
			Foo: String("oof"),
		}, &res)
		require.NoError(t, err)

		anyout = Any(&res)

		ctx.Export("any", anyout)
		return nil
	}, WithMocks("project", "stack", mocks))
	require.NoError(t, err)

	require.NotNil(t, internal.GetOutputValue(anyout))
}

type testResource2Input interface {
	Input

	ToTestResource2Output() testResource2Output
	ToTestResource2OutputWithContext(ctx context.Context) testResource2Output
}

func (*testResource2) ElementType() reflect.Type {
	return reflect.TypeOf((**testResource2)(nil)).Elem()
}

func (r *testResource2) ToTestResource2Output() testResource2Output {
	return r.ToTestResource2OutputWithContext(context.Background())
}

func (r *testResource2) ToTestResource2OutputWithContext(ctx context.Context) testResource2Output {
	return ToOutputWithContext(ctx, r).(testResource2Output)
}

type testResource2Output struct{ *OutputState }

func (testResource2Output) ElementType() reflect.Type {
	return reflect.TypeOf((**testResource2)(nil)).Elem()
}

func (o testResource2Output) ToTestResource2Output() testResource2Output {
	return o
}

func (o testResource2Output) ToTestResource2OutputWithContext(ctx context.Context) testResource2Output {
	return o
}

type testResource4Args struct {
	Inprop *testResource2 `pulumi:"inprop"`
}

type testResource4Inputs struct {
	Inprop testResource2Input
}

func (testResource4Inputs) ElementType() reflect.Type {
	return reflect.TypeOf((*testResource4Args)(nil)).Elem()
}

type testResource4 struct {
	ResourceState

	Outprop StringOutput `pulumi:"outprop"`
}

func TestResourceInput(t *testing.T) {
	t.Parallel()

	RegisterOutputType(testResource2Output{})

	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			switch args.TypeToken {
			case "test:resource:type":
				return "someID", resource.PropertyMap{"foo": resource.NewStringProperty("qux")}, nil
			case "pkg:index:MyRemoteComponent":
				return args.Name + "_id", resource.PropertyMap{
					"outprop": resource.NewStringProperty("bar"),
				}, nil
			default:
				return "", nil, fmt.Errorf("unknown resource %s", args.TypeToken)
			}
		},
	}

	err := RunErr(func(ctx *Context) error {
		var res testResource2
		err := ctx.RegisterResource("test:resource:type", "resA", &testResource2Inputs{
			Foo: String("oof"),
		}, &res)
		require.NoError(t, err)

		var myremotecomponent testResource4
		err = ctx.RegisterRemoteComponentResource("pkg:index:MyRemoteComponent", "myremotecomponent",
			&testResource4Inputs{
				Inprop: res.ToTestResource2Output(),
			}, &myremotecomponent)
		require.NoError(t, err)

		ctx.Export("outprop", myremotecomponent.Outprop)
		return nil
	}, WithMocks("project", "stack", mocks))
	require.NoError(t, err)
}
