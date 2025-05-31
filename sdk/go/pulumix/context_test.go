// Copyright 2016-2023, Pulumi Corporation.
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

package pulumix_test // to avoid import cycles

import (
	"reflect"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumix"
	"github.com/stretchr/testify/require"
)

type testResourceInputs struct {
	PuxString pulumix.Input[string]
	PuString  pulumi.StringInput

	PuxStringPtr pulumix.Input[*string]
	PuStringPtr  pulumi.StringPtrInput

	PuxIntArray pulumix.Input[[]int]
	PuIntArray  pulumi.IntArrayInput

	PuxIntMap pulumix.Input[map[string]int]
	PuIntMap  pulumi.IntMapInput
}

func (*testResourceInputs) ElementType() reflect.Type {
	return reflect.TypeOf((*testResourceArgs)(nil))
}

type testResourceArgs struct {
	PuxString string `pulumi:"puxString"`
	PuString  string `pulumi:"puString"`

	PuxStringPtr *string `pulumi:"puxStringPtr"`
	PuStringPtr  *string `pulumi:"puStringPtr"`

	PuxIntArray []int `pulumi:"puxIntArray"`
	PuIntArray  []int `pulumi:"puIntArray"`

	PuxIntMap map[string]int `pulumi:"puxIntMap"`
	PuIntMap  map[string]int `pulumi:"puIntMap"`
}

func TestRegisterResource_inputSerialization(t *testing.T) {
	t.Parallel()

	j := "j"

	tests := []struct {
		desc string
		give pulumi.Input
		want resource.PropertyMap
	}{
		// --- pulumix.Input[string] ---
		{
			desc: "pux.String/pu.String",
			give: &testResourceInputs{PuxString: pulumi.String("a")},
			want: resource.PropertyMap{"puxString": resource.NewStringProperty("a")},
		},
		{
			desc: "pux.String/pu.StringOutput",
			give: &testResourceInputs{PuxString: pulumi.String("b").ToStringOutput()},
			want: resource.PropertyMap{"puxString": resource.NewStringProperty("b")},
		},
		{
			desc: "pux.String/pux.Output[string]",
			give: &testResourceInputs{PuxString: pulumix.Val("c")},
			want: resource.PropertyMap{"puxString": resource.NewStringProperty("c")},
		},

		// --- pulumi.StringInput ---
		{
			desc: "pu.String/pu.String",
			give: &testResourceInputs{PuString: pulumi.String("d")},
			want: resource.PropertyMap{"puString": resource.NewStringProperty("d")},
		},
		{
			desc: "pu.String/pu.StringOutput",
			give: &testResourceInputs{PuString: pulumi.String("e").ToStringOutput()},
			want: resource.PropertyMap{"puString": resource.NewStringProperty("e")},
		},
		{
			desc: "pu.String/pux.Output[string] untyped",
			give: &testResourceInputs{PuString: pulumix.Val("f").Untyped().(pulumi.StringOutput)},
			want: resource.PropertyMap{"puString": resource.NewStringProperty("f")},
		},

		// --- pulumix.Input[*string] ---
		{
			desc: "pux.StringPtr/pu.String PtrOf",
			give: &testResourceInputs{PuxStringPtr: pulumix.PtrOf[string](pulumi.String("g"))},
			want: resource.PropertyMap{"puxStringPtr": resource.NewStringProperty("g")},
		},
		{
			desc: "pux.StringPtr/pu.StringPtrOutput",
			give: &testResourceInputs{PuxStringPtr: pulumi.String("h").ToStringPtrOutput()},
			want: resource.PropertyMap{"puxStringPtr": resource.NewStringProperty("h")},
		},
		{
			desc: "pux.StringPtr/pux.PtrOutput[string]",
			give: &testResourceInputs{PuxStringPtr: pulumix.Ptr("i")},
			want: resource.PropertyMap{"puxStringPtr": resource.NewStringProperty("i")},
		},
		{
			desc: "pux.StringPtr/pux.Output[*string]",
			give: &testResourceInputs{PuxStringPtr: pulumix.Val[*string](&j)},
			want: resource.PropertyMap{"puxStringPtr": resource.NewStringProperty("j")},
		},

		// --- pulumi.StringPtrInput ---
		{
			desc: "pu.StringPtr/pu.StringPtr",
			give: &testResourceInputs{PuStringPtr: pulumi.StringPtr("j")},
			want: resource.PropertyMap{"puStringPtr": resource.NewStringProperty("j")},
		},
		{
			desc: "pu.StringPtr/pu.String",
			give: &testResourceInputs{PuStringPtr: pulumi.String("k")},
			want: resource.PropertyMap{"puStringPtr": resource.NewStringProperty("k")},
		},
		{
			desc: "pu.StringPtr/pu.StringPtrOutput",
			give: &testResourceInputs{PuStringPtr: pulumi.String("l").ToStringPtrOutput()},
			want: resource.PropertyMap{"puStringPtr": resource.NewStringProperty("l")},
		},
		{
			desc: "pu.StringPtr/pux.PtrOutput[string] untyped",
			give: &testResourceInputs{PuStringPtr: pulumix.Ptr("m").Untyped().(pulumi.StringPtrOutput)},
			want: resource.PropertyMap{"puStringPtr": resource.NewStringProperty("m")},
		},
		{
			desc: "pu.StringPtr/pux.GPtrOutput[string] untyped",
			give: &testResourceInputs{
				PuStringPtr: pulumix.Cast[pulumix.GPtrOutput[string, pulumi.StringOutput], *string](
					pulumix.Ptr("n"),
				).Untyped().(pulumi.StringPtrOutput),
			},
			want: resource.PropertyMap{"puStringPtr": resource.NewStringProperty("n")},
		},

		// --- pulumix.Input[[]int] ---
		{
			desc: "pux.IntArray/pu.IntArray",
			give: &testResourceInputs{
				PuxIntArray: pulumi.IntArray{
					pulumi.Int(1),
					pulumi.Int(2),
					pulumi.Int(3),
				},
			},
			want: resource.PropertyMap{
				"puxIntArray": resource.NewArrayProperty(
					[]resource.PropertyValue{
						resource.NewNumberProperty(1),
						resource.NewNumberProperty(2),
						resource.NewNumberProperty(3),
					},
				),
			},
		},
		{
			desc: "pux.IntArray/pu.IntArrayOutput",
			give: &testResourceInputs{
				PuxIntArray: pulumi.IntArray{
					pulumi.Int(4),
					pulumi.Int(5),
					pulumi.Int(6),
				}.ToIntArrayOutput(),
			},
			want: resource.PropertyMap{
				"puxIntArray": resource.NewArrayProperty(
					[]resource.PropertyValue{
						resource.NewNumberProperty(4),
						resource.NewNumberProperty(5),
						resource.NewNumberProperty(6),
					},
				),
			},
		},
		{
			desc: "pux.IntArray/pux.Output[[]int]",
			give: &testResourceInputs{
				PuxIntArray: pulumix.Val([]int{7, 8, 9}),
			},
			want: resource.PropertyMap{
				"puxIntArray": resource.NewArrayProperty(
					[]resource.PropertyValue{
						resource.NewNumberProperty(7),
						resource.NewNumberProperty(8),
						resource.NewNumberProperty(9),
					},
				),
			},
		},
		{
			desc: "pux.IntArray/pux.ArrayOutput[int]",
			give: &testResourceInputs{
				PuxIntArray: pulumix.Cast[pulumix.ArrayOutput[int], []int](
					pulumix.Val([]int{10, 11, 12}),
				),
			},
			want: resource.PropertyMap{
				"puxIntArray": resource.NewArrayProperty(
					[]resource.PropertyValue{
						resource.NewNumberProperty(10),
						resource.NewNumberProperty(11),
						resource.NewNumberProperty(12),
					},
				),
			},
		},
		{
			desc: "pux.IntArray/pux.GArrayOutput",
			give: &testResourceInputs{
				PuxIntArray: pulumix.Cast[pulumix.GArrayOutput[int, pulumi.IntOutput], []int](
					pulumix.Val([]int{13, 14, 15}),
				),
			},
			want: resource.PropertyMap{
				"puxIntArray": resource.NewArrayProperty(
					[]resource.PropertyValue{
						resource.NewNumberProperty(13),
						resource.NewNumberProperty(14),
						resource.NewNumberProperty(15),
					},
				),
			},
		},

		// --- pulumi.IntArrayInput ---
		{
			desc: "pu.IntArray/pux.Output[[]int] untyped",
			give: &testResourceInputs{
				PuIntArray: pulumix.Val([]int{1, 2, 3}).Untyped().(pulumi.IntArrayOutput),
			},
			want: resource.PropertyMap{
				"puIntArray": resource.NewArrayProperty(
					[]resource.PropertyValue{
						resource.NewNumberProperty(1),
						resource.NewNumberProperty(2),
						resource.NewNumberProperty(3),
					},
				),
			},
		},
		{
			desc: "pu.IntArray/pux.ArrayOutput[int] untyped",
			give: &testResourceInputs{
				PuIntArray: pulumix.Cast[pulumix.ArrayOutput[int], []int](
					pulumix.Val([]int{4, 5, 6}),
				).Untyped().(pulumi.IntArrayOutput),
			},
			want: resource.PropertyMap{
				"puIntArray": resource.NewArrayProperty(
					[]resource.PropertyValue{
						resource.NewNumberProperty(4),
						resource.NewNumberProperty(5),
						resource.NewNumberProperty(6),
					},
				),
			},
		},
		{
			desc: "pu.IntArray/pux.GArrayOutput untyped",
			give: &testResourceInputs{
				PuIntArray: pulumix.Cast[pulumix.GArrayOutput[int, pulumi.IntOutput], []int](
					pulumix.Val([]int{7, 8, 9}),
				).Untyped().(pulumi.IntArrayOutput),
			},
			want: resource.PropertyMap{
				"puIntArray": resource.NewArrayProperty(
					[]resource.PropertyValue{
						resource.NewNumberProperty(7),
						resource.NewNumberProperty(8),
						resource.NewNumberProperty(9),
					},
				),
			},
		},

		// --- pulumix.Input[map[string]int] ---
		{
			desc: "pux.IntMap/pu.IntMap",
			give: &testResourceInputs{
				PuxIntMap: pulumi.IntMap{"a": pulumi.Int(1), "b": pulumi.Int(2)},
			},
			want: resource.PropertyMap{
				"puxIntMap": resource.NewObjectProperty(
					resource.PropertyMap{
						"a": resource.NewNumberProperty(1),
						"b": resource.NewNumberProperty(2),
					},
				),
			},
		},
		{
			desc: "pux.IntMap/pu.IntMapOutput",
			give: &testResourceInputs{
				PuxIntMap: pulumi.IntMap{"c": pulumi.Int(3), "d": pulumi.Int(4)}.ToIntMapOutput(),
			},
			want: resource.PropertyMap{
				"puxIntMap": resource.NewObjectProperty(
					resource.PropertyMap{
						"c": resource.NewNumberProperty(3),
						"d": resource.NewNumberProperty(4),
					},
				),
			},
		},
		{
			desc: "pux.IntMap/pux.Output[map[string]int]",
			give: &testResourceInputs{
				PuxIntMap: pulumix.Val(map[string]int{"e": 5, "f": 6}),
			},
			want: resource.PropertyMap{
				"puxIntMap": resource.NewObjectProperty(
					resource.PropertyMap{
						"e": resource.NewNumberProperty(5),
						"f": resource.NewNumberProperty(6),
					},
				),
			},
		},
		{
			desc: "pux.IntMap/pux.MapOutput[int]",
			give: &testResourceInputs{
				PuxIntMap: pulumix.Cast[pulumix.MapOutput[int], map[string]int](
					pulumix.Val(map[string]int{"g": 7, "h": 8}),
				),
			},
			want: resource.PropertyMap{
				"puxIntMap": resource.NewObjectProperty(
					resource.PropertyMap{
						"g": resource.NewNumberProperty(7),
						"h": resource.NewNumberProperty(8),
					},
				),
			},
		},
		{
			desc: "pux.IntMap/pux.GMapOutput",
			give: &testResourceInputs{
				PuxIntMap: pulumix.Cast[pulumix.GMapOutput[int, pulumi.IntOutput], map[string]int](
					pulumix.Val(map[string]int{"i": 9, "j": 10}),
				),
			},
			want: resource.PropertyMap{
				"puxIntMap": resource.NewObjectProperty(
					resource.PropertyMap{
						"i": resource.NewNumberProperty(9),
						"j": resource.NewNumberProperty(10),
					},
				),
			},
		},

		// --- pulumi.IntMapInput ---
		{
			desc: "pu.IntMap/pu.IntMap",
			give: &testResourceInputs{
				PuIntMap: pulumi.IntMap{"a": pulumi.Int(1), "b": pulumi.Int(2)},
			},
			want: resource.PropertyMap{
				"puIntMap": resource.NewObjectProperty(
					resource.PropertyMap{
						"a": resource.NewNumberProperty(1),
						"b": resource.NewNumberProperty(2),
					},
				),
			},
		},
		{
			desc: "pu.IntMap/pux.Output[map[string]int] untyped",
			give: &testResourceInputs{
				PuIntMap: pulumix.Val(map[string]int{"c": 3, "d": 4}).Untyped().(pulumi.IntMapOutput),
			},
			want: resource.PropertyMap{
				"puIntMap": resource.NewObjectProperty(
					resource.PropertyMap{
						"c": resource.NewNumberProperty(3),
						"d": resource.NewNumberProperty(4),
					},
				),
			},
		},
		{
			desc: "pu.IntMap/pux.MapOutput[int] untyped",
			give: &testResourceInputs{
				PuIntMap: pulumix.Cast[pulumix.MapOutput[int], map[string]int](
					pulumix.Val(map[string]int{"e": 5, "f": 6}),
				).Untyped().(pulumi.IntMapOutput),
			},
			want: resource.PropertyMap{
				"puIntMap": resource.NewObjectProperty(
					resource.PropertyMap{
						"e": resource.NewNumberProperty(5),
						"f": resource.NewNumberProperty(6),
					},
				),
			},
		},
		{
			desc: "pu.IntMap/pux.GMapOutput untyped",
			give: &testResourceInputs{
				PuIntMap: pulumix.Cast[pulumix.GMapOutput[int, pulumi.IntOutput], map[string]int](
					pulumix.Val(map[string]int{"g": 7, "h": 8}),
				).Untyped().(pulumi.IntMapOutput),
			},
			want: resource.PropertyMap{
				"puIntMap": resource.NewObjectProperty(
					resource.PropertyMap{
						"g": resource.NewNumberProperty(7),
						"h": resource.NewNumberProperty(8),
					},
				),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			var (
				got resource.PropertyMap
				ok  bool
			)
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				var res pulumi.CustomResourceState
				require.NoError(t,
					ctx.RegisterResource("test:index:MyResource", "testResource", tt.give, &res))
				return nil
			}, pulumi.WithMocks("project", "stack", &mockResourceMonitor{
				CallF: func(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
					t.Fatalf("unexpected Call(%#v)", args)
					return nil, nil
				},
				NewResourceF: func(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
					if args.TypeToken == "test:index:MyResource" {
						got = args.Inputs
						ok = true
					} else {
						t.Fatalf("unexpected NewResource(%#v)", args)
					}
					return args.Name + "_id", nil, nil
				},
			}))
			require.NoError(t, err)
			require.True(t, ok)

			require.Equal(t, tt.want, got)
		})
	}
}

func TestRegisterResourceOutputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		give pulumi.Input
		// TODO: find a way to intercept RegisterResourceOutput calls.
	}{
		{"pu.String", pulumi.String("a")},
		{"pu.StringOutput", pulumi.String("b").ToStringOutput()},
		{"pux.Output[string]", pulumix.Val("c")},
		{"pux.Output[string]/untyped", pulumix.Val("c").Untyped()},
		{"pux.PtrOf", pulumix.PtrOf[string](pulumi.String("d"))},
		{"pu.StringPtr", pulumi.StringPtr("e")},
		{"pu.StringPtrOutput", pulumi.String("f").ToStringPtrOutput()},
		{"pux.Output[*string]", pulumix.Ptr("g")},
		{"pux.Output[*string]/untyped", pulumix.Ptr("h").Untyped()},
		{"pux.Output[[]int]", pulumix.Val([]int{1, 2, 3})},
		{
			"pux.ArrayOutput[int]",
			pulumix.Cast[pulumix.ArrayOutput[int], []int](
				pulumix.Val([]int{4, 5, 6}),
			),
		},
		{
			"pux.GArrayOutput",
			pulumix.Cast[pulumix.GArrayOutput[int, pulumi.IntOutput], []int](
				pulumix.Val([]int{7, 8, 9}),
			),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				ctx.Export("x", tt.give)
				return nil
			}, pulumi.WithMocks("project", "stack", &mockResourceMonitor{}))
			require.NoError(t, err)
		})
	}
}

type mockResourceMonitor struct {
	// Actually an "Invoke" by provider parlance, but is named so to be consistent with the interface.
	CallF func(pulumi.MockCallArgs) (resource.PropertyMap, error)
	// Actually an "Call" by provider parlance, but is named so to be consistent with the interface.
	MethodCallF  func(pulumi.MockCallArgs) (resource.PropertyMap, error)
	NewResourceF func(pulumi.MockResourceArgs) (string, resource.PropertyMap, error)
}

var (
	// Ensure we implement the appropriate interfaces for testing.
	_ pulumi.MockResourceMonitor               = (*mockResourceMonitor)(nil)
	_ pulumi.MockResourceMonitorWithMethodCall = (*mockResourceMonitor)(nil)
)

func (m *mockResourceMonitor) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return m.CallF(args)
}

func (m *mockResourceMonitor) MethodCall(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return m.MethodCallF(args)
}

func (m *mockResourceMonitor) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	return m.NewResourceF(args)
}
