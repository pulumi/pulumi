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

//nolint:lll
package pulumi

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type BoolEnumInput interface {
	Input

	ToBoolEnumOutput() BoolEnumOutput
	ToBoolEnumOutputWithContext(context.Context) BoolEnumOutput
}

type BoolEnumOutput struct{ *OutputState }

func (BoolEnumOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*BoolEnum)(nil)).Elem()
}

func (o BoolEnumOutput) ToBoolEnumOutput() BoolEnumOutput {
	return o
}

func (o BoolEnumOutput) ToBoolEnumOutputWithContext(ctx context.Context) BoolEnumOutput {
	return o
}

type BoolEnum bool

func (BoolEnum) ElementType() reflect.Type {
	return reflect.TypeOf((*BoolEnum)(nil)).Elem()
}

func (e BoolEnum) ToBoolEnumOutput() BoolEnumOutput {
	return e.ToBoolEnumOutputWithContext(context.Background())
}

func (e BoolEnum) ToBoolEnumOutputWithContext(ctx context.Context) BoolEnumOutput {
	return ToOutputWithContext(ctx, e).(BoolEnumOutput)
}

type BoolEnumInputArgs struct {
	Value BoolEnumInput `pulumi:"value"`
}

type IntEnumInput interface {
	Input

	ToIntEnumOutput() IntEnumOutput
	ToIntEnumOutputWithContext(context.Context) IntEnumOutput
}

type IntEnumOutput struct{ *OutputState }

func (IntEnumOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*IntEnum)(nil)).Elem()
}

func (o IntEnumOutput) ToIntEnumOutput() IntEnumOutput {
	return o
}

func (o IntEnumOutput) ToIntEnumOutputWithContext(ctx context.Context) IntEnumOutput {
	return o
}

type IntEnum int

func (IntEnum) ElementType() reflect.Type {
	return reflect.TypeOf((*IntEnum)(nil)).Elem()
}

func (e IntEnum) ToIntEnumOutput() IntEnumOutput {
	return e.ToIntEnumOutputWithContext(context.Background())
}

func (e IntEnum) ToIntEnumOutputWithContext(ctx context.Context) IntEnumOutput {
	return ToOutputWithContext(ctx, e).(IntEnumOutput)
}

type IntEnumInputArgs struct {
	Value IntEnumInput `pulumi:"value"`
}

type FloatEnumInput interface {
	Input

	ToFloatEnumOutput() FloatEnumOutput
	ToFloatEnumOutputWithContext(context.Context) FloatEnumOutput
}

type FloatEnumOutput struct{ *OutputState }

func (FloatEnumOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*FloatEnum)(nil)).Elem()
}

func (o FloatEnumOutput) ToFloatEnumOutput() FloatEnumOutput {
	return o
}

func (o FloatEnumOutput) ToFloatEnumOutputWithContext(ctx context.Context) FloatEnumOutput {
	return o
}

type FloatEnum float64

func (FloatEnum) ElementType() reflect.Type {
	return reflect.TypeOf((*FloatEnum)(nil)).Elem()
}

func (e FloatEnum) ToFloatEnumOutput() FloatEnumOutput {
	return e.ToFloatEnumOutputWithContext(context.Background())
}

func (e FloatEnum) ToFloatEnumOutputWithContext(ctx context.Context) FloatEnumOutput {
	return ToOutputWithContext(ctx, e).(FloatEnumOutput)
}

type FloatEnumInputArgs struct {
	Value FloatEnumInput `pulumi:"value"`
}

type StringEnumInput interface {
	Input

	ToStringEnumOutput() StringEnumOutput
	ToStringEnumOutputWithContext(context.Context) StringEnumOutput
}

type StringEnumOutput struct{ *OutputState }

func (StringEnumOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*StringEnum)(nil)).Elem()
}

func (o StringEnumOutput) ToStringEnumOutput() StringEnumOutput {
	return o
}

func (o StringEnumOutput) ToStringEnumOutputWithContext(ctx context.Context) StringEnumOutput {
	return o
}

type StringEnum string

func (StringEnum) ElementType() reflect.Type {
	return reflect.TypeOf((*StringEnum)(nil)).Elem()
}

func (e StringEnum) ToStringEnumOutput() StringEnumOutput {
	return e.ToStringEnumOutputWithContext(context.Background())
}

func (e StringEnum) ToStringEnumOutputWithContext(ctx context.Context) StringEnumOutput {
	return ToOutputWithContext(ctx, e).(StringEnumOutput)
}

type StringEnumInputArgs struct {
	Value StringEnumInput `pulumi:"value"`
}

type StringInputArgs struct {
	Value StringInput `pulumi:"value"`
}

type StringPtrInputArgs struct {
	Value StringPtrInput `pulumi:"value"`
}

type StringArrayInputArgs struct {
	Value StringArrayInput `pulumi:"value"`
}

type StringMapInputArgs struct {
	Value StringMapInput `pulumi:"value"`
}

type BoolInputArgs struct {
	Value BoolInput `pulumi:"value"`
}

type BoolPtrInputArgs struct {
	Value BoolPtrInput `pulumi:"value"`
}

type NestedOutputArgs struct {
	Value NestedOutput `pulumi:"value"`
}

type NestedPtrOutputArgs struct {
	Value NestedPtrOutput `pulumi:"value"`
}

type NestedArrayOutputArgs struct {
	Value NestedArrayOutput `pulumi:"value"`
}

type NestedMapOutputArgs struct {
	Value NestedMapOutput `pulumi:"value"`
}

type Nested struct {
	Foo string `pulumi:"foo"`
	Bar int    `pulumi:"bar"`
}

type NestedOutput struct {
	*OutputState
}

func (NestedOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*Nested)(nil)).Elem()
}

type NestedPtrOutput struct {
	*OutputState
}

func (NestedPtrOutput) ElementType() reflect.Type {
	return reflect.TypeOf((**Nested)(nil)).Elem()
}

type NestedArrayOutput struct {
	*OutputState
}

func (NestedArrayOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*[]Nested)(nil)).Elem()
}

type NestedMapOutput struct {
	*OutputState
}

func (NestedMapOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*map[string]Nested)(nil)).Elem()
}

type StringArgs struct {
	Value string `pulumi:"value"`
}

type StringPtrArgs struct {
	Value *string `pulumi:"value"`
}

type StringArrayArgs struct {
	Value []string `pulumi:"value"`
}

type StringMapArgs struct {
	Value map[string]string `pulumi:"value"`
}

type IntArgs struct {
	Value int `pulumi:"value"`
}

type NestedArgs struct {
	Value Nested `pulumi:"value"`
}

type NestedPtrArgs struct {
	Value *Nested `pulumi:"value"`
}

type NestedArrayArgs struct {
	Value []Nested `pulumi:"value"`
}

type NestedMapArgs struct {
	Value map[string]Nested `pulumi:"value"`
}

type PlainArrayArgs struct {
	Value []StringInput `pulumi:"value"`
}

type PlainMapArgs struct {
	Value map[string]StringInput `pulumi:"value"`
}

type AssetArgs struct {
	Value Asset `pulumi:"value"`
}

type AssetInputArgs struct {
	Value AssetInput `pulumi:"value"`
}

type ArchiveArgs struct {
	Value Archive `pulumi:"value"`
}

type ArchiveInputArgs struct {
	Value ArchiveInput `pulumi:"value"`
}

type AssetOrArchiveArgs struct {
	Value AssetOrArchive `pulumi:"value"`
}

type AssetOrArchiveInputArgs struct {
	Value AssetOrArchiveInput `pulumi:"value"`
}

type NestedInputty struct {
	Something *string `pulumi:"something"`
}

// This struct implements the Input interface.
type NestedInputtyInputArgs struct {
	Something StringPtrInput `pulumi:"something"`
}

func (NestedInputtyInputArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*NestedInputty)(nil)).Elem()
}

type PlainOptionalNestedInputtyInputArgs struct {
	Value *NestedInputtyInputArgs `pulumi:"value"`
}

// This struct does not implement the Input interface.
type NestedInputtyArgs struct {
	Something StringPtrInput `pulumi:"something"`
}

type PlainOptionalNestedInputtyArgs struct {
	Value *NestedInputtyArgs `pulumi:"value"`
}

type LaunchTemplateOptions struct {
	TagSpecifications []LaunchTemplateTagSpecification `pulumi:"tagSpecifications"`
}

type LaunchTemplateOptionsInput interface {
	Input

	ToLaunchTemplateOptionsOutput() LaunchTemplateOptionsOutput
	ToLaunchTemplateOptionsOutputWithContext(context.Context) LaunchTemplateOptionsOutput
}

type LaunchTemplateOptionsArgs struct {
	TagSpecifications LaunchTemplateTagSpecificationArrayInput `pulumi:"tagSpecifications"`
}

func (LaunchTemplateOptionsArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*LaunchTemplateOptions)(nil)).Elem()
}

func (i LaunchTemplateOptionsArgs) ToLaunchTemplateOptionsOutput() LaunchTemplateOptionsOutput {
	return i.ToLaunchTemplateOptionsOutputWithContext(context.Background())
}

func (i LaunchTemplateOptionsArgs) ToLaunchTemplateOptionsOutputWithContext(ctx context.Context) LaunchTemplateOptionsOutput {
	return ToOutputWithContext(ctx, i).(LaunchTemplateOptionsOutput)
}

func (i LaunchTemplateOptionsArgs) ToLaunchTemplateOptionsPtrOutput() LaunchTemplateOptionsPtrOutput {
	return i.ToLaunchTemplateOptionsPtrOutputWithContext(context.Background())
}

func (i LaunchTemplateOptionsArgs) ToLaunchTemplateOptionsPtrOutputWithContext(ctx context.Context) LaunchTemplateOptionsPtrOutput {
	return ToOutputWithContext(ctx, i).(LaunchTemplateOptionsOutput).ToLaunchTemplateOptionsPtrOutputWithContext(ctx)
}

type LaunchTemplateOptionsPtrInput interface {
	Input

	ToLaunchTemplateOptionsPtrOutput() LaunchTemplateOptionsPtrOutput
	ToLaunchTemplateOptionsPtrOutputWithContext(context.Context) LaunchTemplateOptionsPtrOutput
}

type launchTemplateOptionsPtrType LaunchTemplateOptionsArgs

func LaunchTemplateOptionsPtr(v *LaunchTemplateOptionsArgs) LaunchTemplateOptionsPtrInput {
	return (*launchTemplateOptionsPtrType)(v)
}

func (*launchTemplateOptionsPtrType) ElementType() reflect.Type {
	return reflect.TypeOf((**LaunchTemplateOptions)(nil)).Elem()
}

func (i *launchTemplateOptionsPtrType) ToLaunchTemplateOptionsPtrOutput() LaunchTemplateOptionsPtrOutput {
	return i.ToLaunchTemplateOptionsPtrOutputWithContext(context.Background())
}

func (i *launchTemplateOptionsPtrType) ToLaunchTemplateOptionsPtrOutputWithContext(ctx context.Context) LaunchTemplateOptionsPtrOutput {
	return ToOutputWithContext(ctx, i).(LaunchTemplateOptionsPtrOutput)
}

type LaunchTemplateOptionsOutput struct{ *OutputState }

func (LaunchTemplateOptionsOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*LaunchTemplateOptions)(nil)).Elem()
}

func (o LaunchTemplateOptionsOutput) ToLaunchTemplateOptionsOutput() LaunchTemplateOptionsOutput {
	return o
}

func (o LaunchTemplateOptionsOutput) ToLaunchTemplateOptionsOutputWithContext(ctx context.Context) LaunchTemplateOptionsOutput {
	return o
}

func (o LaunchTemplateOptionsOutput) ToLaunchTemplateOptionsPtrOutput() LaunchTemplateOptionsPtrOutput {
	return o.ToLaunchTemplateOptionsPtrOutputWithContext(context.Background())
}

func (o LaunchTemplateOptionsOutput) ToLaunchTemplateOptionsPtrOutputWithContext(ctx context.Context) LaunchTemplateOptionsPtrOutput {
	return o.ApplyTWithContext(ctx, func(_ context.Context, v LaunchTemplateOptions) *LaunchTemplateOptions {
		return &v
	}).(LaunchTemplateOptionsPtrOutput)
}

type LaunchTemplateOptionsPtrOutput struct{ *OutputState }

func (LaunchTemplateOptionsPtrOutput) ElementType() reflect.Type {
	return reflect.TypeOf((**LaunchTemplateOptions)(nil)).Elem()
}

func (o LaunchTemplateOptionsPtrOutput) ToLaunchTemplateOptionsPtrOutput() LaunchTemplateOptionsPtrOutput {
	return o
}

func (o LaunchTemplateOptionsPtrOutput) ToLaunchTemplateOptionsPtrOutputWithContext(ctx context.Context) LaunchTemplateOptionsPtrOutput {
	return o
}

func (o LaunchTemplateOptionsPtrOutput) Elem() LaunchTemplateOptionsOutput {
	return o.ApplyT(func(v *LaunchTemplateOptions) LaunchTemplateOptions {
		if v != nil {
			return *v
		}
		var ret LaunchTemplateOptions
		return ret
	}).(LaunchTemplateOptionsOutput)
}

type LaunchTemplateTagSpecification struct {
	Tags map[string]string `pulumi:"tags"`
}

type LaunchTemplateTagSpecificationInput interface {
	Input

	ToLaunchTemplateTagSpecificationOutput() LaunchTemplateTagSpecificationOutput
	ToLaunchTemplateTagSpecificationOutputWithContext(context.Context) LaunchTemplateTagSpecificationOutput
}

type LaunchTemplateTagSpecificationArgs struct {
	Tags StringMapInput `pulumi:"tags"`
}

func (LaunchTemplateTagSpecificationArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*LaunchTemplateTagSpecification)(nil)).Elem()
}

func (i LaunchTemplateTagSpecificationArgs) ToLaunchTemplateTagSpecificationOutput() LaunchTemplateTagSpecificationOutput {
	return i.ToLaunchTemplateTagSpecificationOutputWithContext(context.Background())
}

func (i LaunchTemplateTagSpecificationArgs) ToLaunchTemplateTagSpecificationOutputWithContext(ctx context.Context) LaunchTemplateTagSpecificationOutput {
	return ToOutputWithContext(ctx, i).(LaunchTemplateTagSpecificationOutput)
}

type LaunchTemplateTagSpecificationArrayInput interface {
	Input

	ToLaunchTemplateTagSpecificationArrayOutput() LaunchTemplateTagSpecificationArrayOutput
	ToLaunchTemplateTagSpecificationArrayOutputWithContext(context.Context) LaunchTemplateTagSpecificationArrayOutput
}

type LaunchTemplateTagSpecificationArray []LaunchTemplateTagSpecificationInput

func (LaunchTemplateTagSpecificationArray) ElementType() reflect.Type {
	return reflect.TypeOf((*[]LaunchTemplateTagSpecification)(nil)).Elem()
}

func (i LaunchTemplateTagSpecificationArray) ToLaunchTemplateTagSpecificationArrayOutput() LaunchTemplateTagSpecificationArrayOutput {
	return i.ToLaunchTemplateTagSpecificationArrayOutputWithContext(context.Background())
}

func (i LaunchTemplateTagSpecificationArray) ToLaunchTemplateTagSpecificationArrayOutputWithContext(ctx context.Context) LaunchTemplateTagSpecificationArrayOutput {
	return ToOutputWithContext(ctx, i).(LaunchTemplateTagSpecificationArrayOutput)
}

type LaunchTemplateTagSpecificationOutput struct{ *OutputState }

func (LaunchTemplateTagSpecificationOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*LaunchTemplateTagSpecification)(nil)).Elem()
}

func (o LaunchTemplateTagSpecificationOutput) ToLaunchTemplateTagSpecificationOutput() LaunchTemplateTagSpecificationOutput {
	return o
}

func (o LaunchTemplateTagSpecificationOutput) ToLaunchTemplateTagSpecificationOutputWithContext(ctx context.Context) LaunchTemplateTagSpecificationOutput {
	return o
}

type LaunchTemplateTagSpecificationArrayOutput struct{ *OutputState }

func (LaunchTemplateTagSpecificationArrayOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*[]LaunchTemplateTagSpecification)(nil)).Elem()
}

func (o LaunchTemplateTagSpecificationArrayOutput) ToLaunchTemplateTagSpecificationArrayOutput() LaunchTemplateTagSpecificationArrayOutput {
	return o
}

func (o LaunchTemplateTagSpecificationArrayOutput) ToLaunchTemplateTagSpecificationArrayOutputWithContext(ctx context.Context) LaunchTemplateTagSpecificationArrayOutput {
	return o
}

func (o LaunchTemplateTagSpecificationArrayOutput) Index(i IntInput) LaunchTemplateTagSpecificationOutput {
	return All(o, i).ApplyT(func(vs []interface{}) LaunchTemplateTagSpecification {
		return vs[0].([]LaunchTemplateTagSpecification)[vs[1].(int)]
	}).(LaunchTemplateTagSpecificationOutput)
}

type LaunchTemplateArgs struct {
	Value LaunchTemplateOptionsPtrInput `pulumi:"value"`
}

func init() {
	RegisterInputType(reflect.TypeOf((*BoolEnumInput)(nil)).Elem(), BoolEnum(false))
	RegisterInputType(reflect.TypeOf((*IntEnumInput)(nil)).Elem(), IntEnum(0))
	RegisterInputType(reflect.TypeOf((*FloatEnumInput)(nil)).Elem(), FloatEnum(0))
	RegisterInputType(reflect.TypeOf((*StringEnumInput)(nil)).Elem(), StringEnum(""))
}

func assertOutputEqual(t *testing.T, value interface{}, known bool, secret bool, deps urnSet, output interface{}) {
	actualValue, actualKnown, actualSecret, actualDeps, err := await(output.(Output))
	assert.NoError(t, err)
	assert.Equal(t, value, actualValue)
	assert.Equal(t, known, actualKnown)
	assert.Equal(t, secret, actualSecret)

	actualDepsSet := urnSet{}
	for _, res := range actualDeps {
		urn, uknown, usecret, err := res.URN().awaitURN(context.Background())
		assert.NoError(t, err)
		assert.True(t, uknown)
		assert.False(t, usecret)
		actualDepsSet.add(urn)
	}
	assert.Equal(t, deps, actualDepsSet)
}

func TestConstructInputsCopyTo(t *testing.T) {
	t.Parallel()

	stringPtr := func(v string) *string {
		return &v
	}

	tests := []struct {
		name          string
		input         resource.PropertyValue
		deps          urnSet
		args          interface{}
		assert        func(t *testing.T, actual interface{})
		expectedError string
	}{
		// StringArgs
		{
			name:  "string no deps",
			input: resource.NewStringProperty("hello"),
			args:  &StringArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, "hello", actual)
			},
		},
		{
			name:  "string null value no deps",
			input: resource.NewNullProperty(),
			args:  &StringArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, "", actual)
			},
		},
		{
			name:          "string secret no deps",
			input:         resource.MakeSecret(resource.NewStringProperty("hello")),
			args:          &StringArgs{},
			expectedError: "expected destination type to implement pulumi.Input or pulumi.Output, got string",
		},
		{
			name:          "string computed no deps",
			input:         resource.MakeComputed(resource.NewStringProperty("")),
			args:          &StringArgs{},
			expectedError: "expected destination type to implement pulumi.Input or pulumi.Output, got string",
		},
		{
			name: "string output value known no deps",
			input: resource.NewOutputProperty(resource.Output{
				Element: resource.NewStringProperty("hello"),
				Known:   true,
			}),
			args: &StringArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, "hello", actual)
			},
		},
		{
			name: "string output value known secret no deps",
			input: resource.NewOutputProperty(resource.Output{
				Element: resource.NewStringProperty("hello"),
				Known:   true,
				Secret:  true,
			}),
			args:          &StringArgs{},
			expectedError: "expected destination type to implement pulumi.Input or pulumi.Output, got string",
		},
		{
			name:  "string deps",
			input: resource.NewStringProperty("hello"),
			deps:  urnSet{"fakeURN": struct{}{}},
			args:  &StringArgs{},
			expectedError: "pulumi.StringArgs.Value is typed as string but must be a type that implements " +
				"pulumi.Input or pulumi.Output for input with dependencies",
		},

		// StringPtrArgs
		{
			name:  "string pointer no deps",
			input: resource.NewStringProperty("hello"),
			args:  &StringPtrArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, stringPtr("hello"), actual)
			},
		},
		{
			name:          "string pointer secret no deps",
			input:         resource.MakeSecret(resource.NewStringProperty("hello")),
			args:          &StringPtrArgs{},
			expectedError: "expected destination type to implement pulumi.Input or pulumi.Output, got string",
		},
		{
			name:  "string pointer null value no deps",
			input: resource.NewNullProperty(),
			args:  &StringPtrArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Nil(t, actual)
			},
		},

		// StringInputArgs
		{
			name:  "StringInput no deps",
			input: resource.NewStringProperty("hello"),
			args:  &StringInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, String("hello"), actual)
			},
		},
		{
			name:  "StringInput known secret no deps",
			input: resource.MakeSecret(resource.NewStringProperty("hello")),
			args:  &StringInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assertOutputEqual(t, "hello", true, true, urnSet{}, actual)
			},
		},
		{
			name: "StringInput output value known secret no deps",
			input: resource.NewOutputProperty(resource.Output{
				Element: resource.NewStringProperty("hello"),
				Known:   true,
				Secret:  true,
			}),
			args: &StringInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assertOutputEqual(t, "hello", true, true, urnSet{}, actual)
			},
		},
		{
			name:  "StringInput output value unknown no deps",
			input: resource.NewOutputProperty(resource.Output{}),
			args:  &StringInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assertOutputEqual(t, nil, false, false, urnSet{}, actual)
			},
		},
		{
			name: "StringInput output value unknown secret no deps",
			input: resource.NewOutputProperty(resource.Output{
				Secret: true,
			}),
			args: &StringInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assertOutputEqual(t, nil, false, true, urnSet{}, actual)
			},
		},
		{
			name:  "StringInput with deps",
			input: resource.NewStringProperty("hello"),
			deps:  urnSet{"fakeURN": struct{}{}},
			args:  &StringInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assertOutputEqual(t, "hello", true, false, urnSet{"fakeURN": struct{}{}}, actual)
			},
		},

		// StringPtrInputArgs
		{
			name:  "StringPtrInput no deps",
			input: resource.NewStringProperty("hello"),
			args:  &StringPtrInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, String("hello"), actual)
			},
		},
		{
			name:  "StringPtrInput null value no deps",
			input: resource.NewNullProperty(),
			args:  &StringPtrInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Nil(t, actual)
			},
		},

		// BoolInputArgs
		{
			name:  "BoolInput no deps",
			input: resource.NewBoolProperty(true),
			args:  &BoolInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, Bool(true), actual)
			},
		},
		{
			name:  "BoolInput known secret no deps",
			input: resource.MakeSecret(resource.NewBoolProperty(true)),
			args:  &BoolInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assertOutputEqual(t, true, true, true, urnSet{}, actual)
			},
		},
		{
			name: "BoolInput output value known secret no deps",
			input: resource.NewOutputProperty(resource.Output{
				Element: resource.NewBoolProperty(true),
				Known:   true,
				Secret:  true,
			}),
			args: &BoolInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assertOutputEqual(t, true, true, true, urnSet{}, actual)
			},
		},
		{
			name:  "BoolInput output value unknown no deps",
			input: resource.NewOutputProperty(resource.Output{}),
			args:  &BoolInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assertOutputEqual(t, nil, false, false, urnSet{}, actual)
			},
		},
		{
			name: "BoolInput output value unknown secret no deps",
			input: resource.NewOutputProperty(resource.Output{
				Secret: true,
			}),
			args: &BoolInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assertOutputEqual(t, nil, false, true, urnSet{}, actual)
			},
		},

		// BoolPtrInputArgs
		{
			name:  "BoolPtrInput no deps",
			input: resource.NewBoolProperty(true),
			args:  &BoolPtrInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, Bool(true), actual)
			},
		},
		{
			name:  "BoolPtrInput null value no deps",
			input: resource.NewNullProperty(),
			args:  &BoolPtrInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Nil(t, actual)
			},
		},

		// IntArgs
		{
			name:  "int no deps",
			input: resource.NewNumberProperty(42),
			args:  &IntArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, 42, actual)
			},
		},
		{
			name:          "set field typed as int with string value",
			input:         resource.NewStringProperty("foo"),
			args:          &IntArgs{},
			expectedError: "unmarshaling value: expected an int, got a string",
		},

		// BoolEnumInputArgs
		{
			name:  "BoolEnumInput no deps",
			input: resource.NewBoolProperty(true),
			args:  &BoolEnumInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, BoolEnum(true), actual)
			},
		},
		// IntEnumInputArgs
		{
			name:  "IntEnumInput no deps",
			input: resource.NewNumberProperty(42),
			args:  &IntEnumInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, IntEnum(42), actual)
			},
		},
		// FloatEnumInputArgs
		{
			name:  "FloatEnumInput no deps",
			input: resource.NewNumberProperty(42),
			args:  &FloatEnumInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, FloatEnum(42), actual)
			},
		},
		// StringEnumInputArgs
		{
			name:  "StringEnumInput no deps",
			input: resource.NewStringProperty("hello"),
			args:  &StringEnumInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, StringEnum("hello"), actual)
			},
		},

		// StringArrayArgs
		{
			name: "StringArrayArgs no deps",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("hello"),
				resource.NewStringProperty("world"),
			}),
			args: &StringArrayArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, []string{"hello", "world"}, actual)
			},
		},

		// StringArrayInputArgs
		{
			name: "StringArrayInputArgs no deps",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("hello"),
				resource.NewStringProperty("world"),
			}),
			args: &StringArrayInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, StringArray{
					String("hello"),
					String("world"),
				}, actual)
			},
		},
		{
			name: "StringArrayInputArgs no deps nested secret output",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("hello"),
				resource.NewOutputProperty(resource.Output{
					Element: resource.NewStringProperty("world"),
					Known:   true,
					Secret:  true,
				}),
			}),
			args: &StringArrayInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				v, ok := actual.(StringArray)
				assert.True(t, ok)
				assert.Len(t, v, 2)
				assert.Equal(t, String("hello"), v[0])
				assertOutputEqual(t, "world", true, true, urnSet{}, v[1])
			},
		},

		// StringMapArgs
		{
			name: "StringMapArgs no deps",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": "hello",
				"bar": "world",
			})),
			args: &StringMapArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, map[string]string{
					"foo": "hello",
					"bar": "world",
				}, actual)
			},
		},

		// StringMapInputArgs
		{
			name: "StringMapInputArgs no deps",
			input: resource.NewObjectProperty(resource.PropertyMap{
				"foo": resource.NewStringProperty("hello"),
				"bar": resource.NewStringProperty("world"),
			}),
			args: &StringMapInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, StringMap{
					"foo": String("hello"),
					"bar": String("world"),
				}, actual)
			},
		},
		{
			name: "StringMapInputArgs no deps nested secret output",
			input: resource.NewObjectProperty(resource.PropertyMap{
				"foo": resource.NewStringProperty("hello"),
				"bar": resource.NewOutputProperty(resource.Output{
					Element: resource.NewStringProperty("world"),
					Known:   true,
					Secret:  true,
				}),
			}),
			args: &StringMapInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				v, ok := actual.(StringMap)
				assert.True(t, ok)
				assert.Len(t, v, 2)
				assert.Equal(t, String("hello"), v["foo"])
				assertOutputEqual(t, "world", true, true, urnSet{}, v["bar"])
			},
		},
		{
			name: "StringMapInputArgs with deps nested secret output",
			input: resource.NewObjectProperty(resource.PropertyMap{
				"foo": resource.NewStringProperty("hello"),
				"bar": resource.NewOutputProperty(resource.Output{
					Element:      resource.NewStringProperty("world"),
					Known:        true,
					Secret:       true,
					Dependencies: []resource.URN{"fakeURN"},
				}),
			}),
			deps: urnSet{"fakeURN": struct{}{}},
			args: &StringMapInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				v, ok := actual.(StringMap)
				assert.True(t, ok)
				assert.Len(t, v, 2)
				assert.Equal(t, String("hello"), v["foo"])
				assertOutputEqual(t, "world", true, true, urnSet{"fakeURN": struct{}{}}, v["bar"])
			},
		},
		{
			name: "StringMapInputArgs with extra deps nested secret output",
			input: resource.NewObjectProperty(resource.PropertyMap{
				"foo": resource.NewStringProperty("hello"),
				"bar": resource.NewOutputProperty(resource.Output{
					Element:      resource.NewStringProperty("world"),
					Known:        true,
					Secret:       true,
					Dependencies: []resource.URN{"fakeURN1", "fakeURN2"},
				}),
			}),
			deps: urnSet{"fakeURN1": struct{}{}},
			args: &StringMapInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				v, ok := actual.(StringMap)
				assert.True(t, ok)
				assert.Len(t, v, 2)
				assert.Equal(t, String("hello"), v["foo"])
				assertOutputEqual(t, "world", true, true, urnSet{
					"fakeURN1": struct{}{},
					"fakeURN2": struct{}{},
				}, v["bar"])
			},
		},

		// NestedArgs
		{
			name: "NestedArgs no deps",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": "hi",
				"bar": 7,
			})),
			args: &NestedArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, Nested{Foo: "hi", Bar: 7}, actual)
			},
		},
		{
			name: "set field typed as string with number value",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": 500,
				"bar": 42,
			})),
			args:          &NestedArgs{},
			expectedError: "unmarshaling value: expected a string, got a number",
		},
		{
			name: "destination must be typed as input or output for secret value",
			input: resource.MakeSecret(resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": "hello",
				"bar": 42,
			}))),
			args:          &NestedArgs{},
			expectedError: "expected destination type to implement pulumi.Input or pulumi.Output, got pulumi.Nested",
		},
		{
			name: "destination must be typed as input or output for value with dependencies",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": "hello",
				"bar": 42,
			})),
			deps: urnSet{"fakeURN": struct{}{}},
			args: &NestedArgs{},
			expectedError: "pulumi.NestedArgs.Value is typed as pulumi.Nested but must be a type that implements " +
				"pulumi.Input or pulumi.Output for input with dependencies",
		},

		// NestedPtrArgs
		{
			name: "NestedPtrArgs no deps",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": "pointer",
				"bar": 2,
			})),
			args: &NestedPtrArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, &Nested{Foo: "pointer", Bar: 2}, actual)
			},
		},

		// NestedArrayArgs
		{
			name: "NestedArrayArgs no deps",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
					"foo": "1",
					"bar": 1,
				})),
				resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
					"foo": "2",
					"bar": 2,
				})),
			}),
			args: &NestedArrayArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, []Nested{
					{Foo: "1", Bar: 1},
					{Foo: "2", Bar: 2},
				}, actual)
			},
		},

		// NestedMapArgs
		{
			name: "NestedMapArgs no deps",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"a": map[string]interface{}{
					"foo": "3",
					"bar": 3,
				},
				"b": map[string]interface{}{
					"foo": "4",
					"bar": 4,
				},
			})),
			args: &NestedMapArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, map[string]Nested{
					"a": {Foo: "3", Bar: 3},
					"b": {Foo: "4", Bar: 4},
				}, actual)
			},
		},

		// NestedOutputArgs
		{
			name: "NestedOutputArgs no deps",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": "hello",
				"bar": 42,
			})),
			args: &NestedOutputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assertOutputEqual(t, Nested{
					Foo: "hello",
					Bar: 42,
				}, true, false, urnSet{}, actual)
			},
		},

		// NestedPtrOutputArgs
		{
			name: "NestedPtrOutputArgs no deps",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": "world",
				"bar": 100,
			})),
			args: &NestedPtrOutputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assertOutputEqual(t, &Nested{
					Foo: "world",
					Bar: 100,
				}, true, false, urnSet{}, actual)
			},
		},

		// NestedArrayOutputArgs
		{
			name: "NestedArrayOutputArgs no deps",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
					"foo": "a",
					"bar": 1,
				})),
				resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
					"foo": "b",
					"bar": 2,
				})),
			}),
			args: &NestedArrayOutputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assertOutputEqual(t, []Nested{
					{Foo: "a", Bar: 1},
					{Foo: "b", Bar: 2},
				}, true, false, urnSet{}, actual)
			},
		},

		// NestedMapOutputArgs
		{
			name: "NestedMapOutputArgs no deps",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"a": map[string]interface{}{
					"foo": "c",
					"bar": 3,
				},
				"b": map[string]interface{}{
					"foo": "d",
					"bar": 4,
				},
			})),
			args: &NestedMapOutputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assertOutputEqual(t, map[string]Nested{
					"a": {Foo: "c", Bar: 3},
					"b": {Foo: "d", Bar: 4},
				}, true, false, urnSet{}, actual)
			},
		},

		// PlainArrayArgs
		{
			name: "PlainArrayArgs no deps",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("foo"),
				resource.NewStringProperty("bar"),
			}),
			args: &PlainArrayArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, []StringInput{
					String("foo"),
					String("bar"),
				}, actual)
			},
		},
		{
			name: "PlainArrayArgs secret no deps",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("foo"),
				resource.MakeSecret(resource.NewStringProperty("bar")),
			}),
			args: &PlainArrayArgs{},
			assert: func(t *testing.T, actual interface{}) {
				v, ok := actual.([]StringInput)
				assert.True(t, ok)
				assert.Len(t, v, 2)
				assert.Equal(t, String("foo"), v[0])
				assertOutputEqual(t, "bar", true, true, urnSet{}, v[1])
			},
		},
		{
			name: "PlainArrayArgs computed no deps",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("foo"),
				resource.MakeComputed(resource.NewStringProperty("")),
			}),
			args: &PlainArrayArgs{},
			assert: func(t *testing.T, actual interface{}) {
				v, ok := actual.([]StringInput)
				assert.True(t, ok)
				assert.Len(t, v, 2)
				assert.Equal(t, String("foo"), v[0])
				assertOutputEqual(t, nil, false, false, urnSet{}, v[1])
			},
		},
		{
			name: "PlainArrayArgs output value known no deps",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("foo"),
				resource.NewOutputProperty(resource.Output{
					Element: resource.NewStringProperty("bar"),
					Known:   true,
				}),
			}),
			args: &PlainArrayArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, []StringInput{
					String("foo"),
					String("bar"),
				}, actual)
			},
		},
		{
			name: "PlainArrayArgs output value known secret no deps",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("foo"),
				resource.NewOutputProperty(resource.Output{
					Element: resource.NewStringProperty("bar"),
					Known:   true,
					Secret:  true,
				}),
			}),
			args: &PlainArrayArgs{},
			assert: func(t *testing.T, actual interface{}) {
				v, ok := actual.([]StringInput)
				assert.True(t, ok)
				assert.Len(t, v, 2)
				assert.Equal(t, String("foo"), v[0])
				assertOutputEqual(t, "bar", true, true, urnSet{}, v[1])
			},
		},
		{
			name: "PlainArrayArgs with deps",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("foo"),
				resource.NewStringProperty("bar"),
			}),
			deps: urnSet{"fakeURN": struct{}{}},
			args: &PlainArrayArgs{},
			assert: func(t *testing.T, actual interface{}) {
				v, ok := actual.([]StringInput)
				assert.True(t, ok)
				assert.Len(t, v, 2)
				assertOutputEqual(t, "foo", true, false, urnSet{"fakeURN": struct{}{}}, v[0])
				assertOutputEqual(t, "bar", true, false, urnSet{"fakeURN": struct{}{}}, v[1])
			},
		},

		// PlainMapArgs
		{
			name: "PlainMapArgs no deps",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": "bar",
				"baz": "qux",
			})),
			args: &PlainMapArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, map[string]StringInput{
					"foo": String("bar"),
					"baz": String("qux"),
				}, actual)
			},
		},
		{
			name: "PlainMapArgs secret no deps",
			input: resource.NewObjectProperty(resource.PropertyMap{
				"foo": resource.NewStringProperty("bar"),
				"baz": resource.MakeSecret(resource.NewStringProperty("qux")),
			}),
			args: &PlainMapArgs{},
			assert: func(t *testing.T, actual interface{}) {
				v, ok := actual.(map[string]StringInput)
				assert.True(t, ok)
				assert.Len(t, v, 2)
				assert.Equal(t, String("bar"), v["foo"])
				assertOutputEqual(t, "qux", true, true, urnSet{}, v["baz"])
			},
		},
		{
			name: "PlainMapArgs computed no deps",
			input: resource.NewObjectProperty(resource.PropertyMap{
				"foo": resource.NewStringProperty("bar"),
				"baz": resource.MakeComputed(resource.NewStringProperty("")),
			}),
			args: &PlainMapArgs{},
			assert: func(t *testing.T, actual interface{}) {
				v, ok := actual.(map[string]StringInput)
				assert.True(t, ok)
				assert.Len(t, v, 2)
				assert.Equal(t, String("bar"), v["foo"])
				assertOutputEqual(t, nil, false, false, urnSet{}, v["baz"])
			},
		},
		{
			name: "PlainMapArgs output value known no deps",
			input: resource.NewObjectProperty(resource.PropertyMap{
				"foo": resource.NewStringProperty("bar"),
				"baz": resource.NewOutputProperty(resource.Output{
					Element: resource.NewStringProperty("qux"),
					Known:   true,
				}),
			}),
			args: &PlainMapArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, map[string]StringInput{
					"foo": String("bar"),
					"baz": String("qux"),
				}, actual)
			},
		},
		{
			name: "PlainMapArgs output value known secret no deps",
			input: resource.NewObjectProperty(resource.PropertyMap{
				"foo": resource.NewStringProperty("bar"),
				"baz": resource.NewOutputProperty(resource.Output{
					Element: resource.NewStringProperty("qux"),
					Known:   true,
					Secret:  true,
				}),
			}),
			args: &PlainMapArgs{},
			assert: func(t *testing.T, actual interface{}) {
				v, ok := actual.(map[string]StringInput)
				assert.True(t, ok)
				assert.Len(t, v, 2)
				assert.Equal(t, String("bar"), v["foo"])
				assertOutputEqual(t, "qux", true, true, urnSet{}, v["baz"])
			},
		},
		{
			name: "PlainMapArgs with deps",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": "bar",
				"baz": "qux",
			})),
			deps: urnSet{"fakeURN": struct{}{}},
			args: &PlainMapArgs{},
			assert: func(t *testing.T, actual interface{}) {
				v, ok := actual.(map[string]StringInput)
				assert.True(t, ok)
				assert.Len(t, v, 2)
				assertOutputEqual(t, "bar", true, false, urnSet{"fakeURN": struct{}{}}, v["foo"])
				assertOutputEqual(t, "qux", true, false, urnSet{"fakeURN": struct{}{}}, v["baz"])
			},
		},

		// PlainOptionalNestedInputtyInputArgs
		{
			name:  "PlainOptionalNestedInputtyInputArgs empty no deps",
			input: resource.NewObjectProperty(resource.PropertyMap{}),
			args:  &PlainOptionalNestedInputtyInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, &NestedInputtyInputArgs{}, actual)
			},
		},
		{
			name: "PlainOptionalNestedInputtyInputArgs value no deps",
			input: resource.NewObjectProperty(resource.PropertyMap{
				"something": resource.NewStringProperty("anything"),
			}),
			args: &PlainOptionalNestedInputtyInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, &NestedInputtyInputArgs{
					Something: String("anything"),
				}, actual)
			},
		},
		{
			name: "PlainOptionalNestedInputtyInputArgs secret no deps",
			input: resource.NewObjectProperty(resource.PropertyMap{
				"something": resource.MakeSecret(resource.NewStringProperty("anything")),
			}),
			args: &PlainOptionalNestedInputtyInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				v, ok := actual.(*NestedInputtyInputArgs)
				assert.True(t, ok)
				assertOutputEqual(t, "anything", true, true, urnSet{}, v.Something)
			},
		},
		{
			name: "PlainOptionalNestedInputtyInputArgs computed no deps",
			input: resource.NewObjectProperty(resource.PropertyMap{
				"something": resource.MakeComputed(resource.NewStringProperty("")),
			}),
			args: &PlainOptionalNestedInputtyInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				v, ok := actual.(*NestedInputtyInputArgs)
				assert.True(t, ok)
				assertOutputEqual(t, nil, false, false, urnSet{}, v.Something)
			},
		},
		{
			name: "PlainOptionalNestedInputtyInputArgs output value known no deps",
			input: resource.NewObjectProperty(resource.PropertyMap{
				"something": resource.NewOutputProperty(resource.Output{
					Element: resource.NewStringProperty("anything"),
					Known:   true,
				}),
			}),
			args: &PlainOptionalNestedInputtyInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, &NestedInputtyInputArgs{
					Something: String("anything"),
				}, actual)
			},
		},
		{
			name: "PlainOptionalNestedInputtyInputArgs output value known secret no deps",
			input: resource.NewObjectProperty(resource.PropertyMap{
				"something": resource.NewOutputProperty(resource.Output{
					Element: resource.NewStringProperty("anything"),
					Known:   true,
					Secret:  true,
				}),
			}),
			args: &PlainOptionalNestedInputtyInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				v, ok := actual.(*NestedInputtyInputArgs)
				assert.True(t, ok)
				assertOutputEqual(t, "anything", true, true, urnSet{}, v.Something)
			},
		},
		{
			name: "PlainOptionalNestedInputtyInputArgs output value known secret with deps",
			input: resource.NewObjectProperty(resource.PropertyMap{
				"something": resource.NewOutputProperty(resource.Output{
					Element:      resource.NewStringProperty("anything"),
					Known:        true,
					Secret:       true,
					Dependencies: []resource.URN{"fakeURN"},
				}),
			}),
			deps: urnSet{"fakeURN": struct{}{}},
			args: &PlainOptionalNestedInputtyInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				v, ok := actual.(*NestedInputtyInputArgs)
				assert.True(t, ok)
				assertOutputEqual(t, "anything", true, true, urnSet{"fakeURN": struct{}{}}, v.Something)
			},
		},

		// PlainOptionalNestedInputtyArgs
		{
			name:  "PlainOptionalNestedInputtyArgs empty no deps",
			input: resource.NewObjectProperty(resource.PropertyMap{}),
			args:  &PlainOptionalNestedInputtyArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, &NestedInputtyArgs{}, actual)
			},
		},
		{
			name: "PlainOptionalNestedInputtyArgs value no deps",
			input: resource.NewObjectProperty(resource.PropertyMap{
				"something": resource.NewStringProperty("anything"),
			}),
			args: &PlainOptionalNestedInputtyArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, &NestedInputtyArgs{
					Something: String("anything"),
				}, actual)
			},
		},
		{
			name: "PlainOptionalNestedInputtyArgs secret no deps",
			input: resource.NewObjectProperty(resource.PropertyMap{
				"something": resource.MakeSecret(resource.NewStringProperty("anything")),
			}),
			args: &PlainOptionalNestedInputtyArgs{},
			assert: func(t *testing.T, actual interface{}) {
				v, ok := actual.(*NestedInputtyArgs)
				assert.True(t, ok)
				assertOutputEqual(t, "anything", true, true, urnSet{}, v.Something)
			},
		},
		{
			name: "PlainOptionalNestedInputtyArgs computed no deps",
			input: resource.NewObjectProperty(resource.PropertyMap{
				"something": resource.MakeComputed(resource.NewStringProperty("")),
			}),
			args: &PlainOptionalNestedInputtyArgs{},
			assert: func(t *testing.T, actual interface{}) {
				v, ok := actual.(*NestedInputtyArgs)
				assert.True(t, ok)
				assertOutputEqual(t, nil, false, false, urnSet{}, v.Something)
			},
		},
		{
			name: "PlainOptionalNestedInputtyArgs output value known no deps",
			input: resource.NewObjectProperty(resource.PropertyMap{
				"something": resource.NewOutputProperty(resource.Output{
					Element: resource.NewStringProperty("anything"),
					Known:   true,
				}),
			}),
			args: &PlainOptionalNestedInputtyArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, &NestedInputtyArgs{
					Something: String("anything"),
				}, actual)
			},
		},
		{
			name: "PlainOptionalNestedInputtyArgs output value known secret no deps",
			input: resource.NewObjectProperty(resource.PropertyMap{
				"something": resource.NewOutputProperty(resource.Output{
					Element: resource.NewStringProperty("anything"),
					Known:   true,
					Secret:  true,
				}),
			}),
			args: &PlainOptionalNestedInputtyArgs{},
			assert: func(t *testing.T, actual interface{}) {
				v, ok := actual.(*NestedInputtyArgs)
				assert.True(t, ok)
				assertOutputEqual(t, "anything", true, true, urnSet{}, v.Something)
			},
		},
		{
			name: "PlainOptionalNestedInputtyArgs output value known secret with deps",
			input: resource.NewObjectProperty(resource.PropertyMap{
				"something": resource.NewOutputProperty(resource.Output{
					Element:      resource.NewStringProperty("anything"),
					Known:        true,
					Secret:       true,
					Dependencies: []resource.URN{"fakeURN"},
				}),
			}),
			deps: urnSet{"fakeURN": struct{}{}},
			args: &PlainOptionalNestedInputtyArgs{},
			assert: func(t *testing.T, actual interface{}) {
				v, ok := actual.(*NestedInputtyArgs)
				assert.True(t, ok)
				assertOutputEqual(t, "anything", true, true, urnSet{"fakeURN": struct{}{}}, v.Something)
			},
		},

		// AssetArgs
		{
			name:  "AssetArgs no deps",
			input: resource.NewAssetProperty(&resource.Asset{Text: "hello"}),
			args:  &AssetArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, NewStringAsset("hello"), actual)
			},
		},

		// AssetInputArgs
		{
			name:  "AssetInputArgs no deps",
			input: resource.NewAssetProperty(&resource.Asset{Text: "hello"}),
			args:  &AssetInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, NewStringAsset("hello"), actual)
			},
		},

		// ArchiveArgs
		{
			name:  "ArchiveArgs no deps",
			input: resource.NewArchiveProperty(&resource.Archive{Path: "path"}),
			args:  &ArchiveArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, NewFileArchive("path"), actual)
			},
		},

		// ArchiveInputArgs
		{
			name:  "ArchiveInputArgs no deps",
			input: resource.NewArchiveProperty(&resource.Archive{Path: "path"}),
			args:  &ArchiveInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, NewFileArchive("path"), actual)
			},
		},

		// AssetOrArchiveArgs
		{
			name:  "AssetOrArchiveArgs no deps",
			input: resource.NewAssetProperty(&resource.Asset{Text: "hello"}),
			args:  &AssetOrArchiveArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, NewStringAsset("hello"), actual)
			},
		},

		// AssetOrArchiveInputArgs
		{
			name:  "AssetOrArchiveInputArgs no deps",
			input: resource.NewAssetProperty(&resource.Asset{Text: "hello"}),
			args:  &AssetOrArchiveInputArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assert.Equal(t, NewStringAsset("hello"), actual)
			},
		},

		// LaunchTemplateArgs
		{
			name: "LaunchTemplateArgs input types not registered nested known output value",
			input: resource.NewObjectProperty(resource.PropertyMap{
				"tagSpecifications": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewObjectProperty(resource.PropertyMap{
						"tags": resource.NewObjectProperty(resource.PropertyMap{
							"Name": resource.NewOutputProperty(resource.Output{
								Element: resource.NewStringProperty("Worker Node"),
								Known:   true,
							}),
							"Test Name": resource.NewStringProperty("test name"),
						}),
					}),
				}),
			}),
			args: &LaunchTemplateArgs{},
			assert: func(t *testing.T, actual interface{}) {
				assertOutputEqual(t, &LaunchTemplateOptions{
					TagSpecifications: []LaunchTemplateTagSpecification{
						{
							Tags: map[string]string{
								"Name":      "Worker Node",
								"Test Name": "test name",
							},
						},
					},
				}, true, false, urnSet{}, actual)
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, err := NewContext(context.Background(), RunInfo{})
			require.NoError(t, err)

			inputs := map[string]interface{}{
				"value": &constructInput{value: test.input, deps: test.deps},
			}
			err = constructInputsCopyTo(ctx, inputs, test.args)
			if test.expectedError != "" {
				assert.EqualError(t, err, "copying input \"value\": "+test.expectedError)
			} else {
				assert.NoError(t, err)
				actual := reflect.ValueOf(test.args).Elem().FieldByName("Value").Interface()
				test.assert(t, actual)
			}
		})
	}
}

type MyComponent struct {
	ResourceState

	Foo        string       `pulumi:"foo"`
	SomeValue  StringOutput `pulumi:"someValue"`
	Nope       StringOutput
	unexported StringOutput `pulumi:"unexported"`
}

func TestConstructResult(t *testing.T) {
	t.Parallel()

	someOutput := String("something").ToStringOutput()

	component := &MyComponent{
		Foo:        "hi",
		SomeValue:  someOutput,
		Nope:       String("nope").ToStringOutput(),
		unexported: String("nope").ToStringOutput(),
	}

	_, state, err := newConstructResult(component)
	assert.NoError(t, err)

	resolvedProps, _, _, err := marshalInputs(state)
	assert.NoError(t, err)

	assert.Equal(t, resource.PropertyMap{
		"foo":       resource.NewStringProperty("hi"),
		"someValue": resource.NewStringProperty("something"),
	}, resolvedProps)
}

type MyComponentWithResource struct {
	ResourceState

	// This is not realistic, but it replicates what you get when you send a `nil` pointer
	// over RPC for a resource.
	Component Resource `pulumi:"component"`
}

func TestSerdeNilNestedResource(t *testing.T) {
	t.Parallel()

	component := &MyComponentWithResource{
		Component: (*MyComponent)(nil),
	}
	_, state, err := newConstructResult(component)
	assert.NoError(t, err)

	_, _, _, err = marshalInputs(state)
	assert.NoError(t, err)
}

func TestConstruct_resourceOptionsSnapshot(t *testing.T) {
	t.Parallel()

	// Runs Construct with the given request and a fake constructor,
	// and returns a snapshot (ResourceOptions) of the resulting options.
	//
	// Fails the test if any operation fails.
	snapshotFromRequest := func(t *testing.T, req *pulumirpc.ConstructRequest) *ResourceOptions {
		// Keep test cases simple:
		req.Stack = "mystack"
		req.Project = "myproject"

		var got *ResourceOptions
		ctx := context.Background()
		_, err := construct(ctx, req, nil, func(
			ctx *Context,
			typ, name string,
			inputs map[string]interface{},
			opts ResourceOption,
		) (URNInput, Input, error) {
			urn := resource.NewURN(
				tokens.QName(ctx.Stack()),
				tokens.PackageName(ctx.Project()),
				"", // parent
				tokens.Type(typ),
				name,
			)

			snap, err := NewResourceOptions(opts)
			require.NoError(t, err, "failed to create snapshot")
			got = snap

			return URN(urn), nil, nil
		})
		require.NoError(t, err, "failed to construct")
		require.NotNil(t, got, "Construct was never called")
		return got
	}

	t.Run("Aliases", func(t *testing.T) {
		t.Parallel()

		snap := snapshotFromRequest(t, &pulumirpc.ConstructRequest{
			Aliases: []string{"test"},
		})
		assert.Len(t, snap.Aliases, 1, "aliases were not set")
	})

	t.Run("DependsOn", func(t *testing.T) {
		t.Parallel()

		snap := snapshotFromRequest(t, &pulumirpc.ConstructRequest{
			Dependencies: []string{"test"},
		})
		assert.Len(t, snap.DependsOn, 1, "dependencies were not set")
	})

	t.Run("Protect", func(t *testing.T) {
		t.Parallel()

		snap := snapshotFromRequest(t, &pulumirpc.ConstructRequest{
			Protect: true,
		})
		assert.True(t, snap.Protect, "protect was not set")
	})

	t.Run("Providers", func(t *testing.T) {
		t.Parallel()

		urn := resource.NewURN("mystack", "myproject", "", "pulumi:providers:foo", "bar")
		snap := snapshotFromRequest(t, &pulumirpc.ConstructRequest{
			Providers: map[string]string{
				"baz": string(urn) + "::qux",
			},
		})
		assert.Len(t, snap.Providers, 1, "providers were not set")
	})

	t.Run("Parent", func(t *testing.T) {
		t.Parallel()

		urn := resource.NewURN("mystack", "myproject", "", "pulumi:providers:foo", "bar")
		snap := snapshotFromRequest(t, &pulumirpc.ConstructRequest{
			Parent: string(urn),
		})
		assert.NotNil(t, snap.Parent, "parent was not set")
	})

	t.Run("AdditionalSecretOutputs", func(t *testing.T) {
		t.Parallel()

		snap := snapshotFromRequest(t, &pulumirpc.ConstructRequest{
			AdditionalSecretOutputs: []string{"foo"},
		})
		assert.Equal(t, []string{"foo"}, snap.AdditionalSecretOutputs)
	})

	t.Run("CustomTimeouts", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give *pulumirpc.ConstructRequest_CustomTimeouts
			want *CustomTimeouts
		}{
			{
				name: "Create",
				give: &pulumirpc.ConstructRequest_CustomTimeouts{Create: "1m"},
				want: &CustomTimeouts{Create: "1m"},
			},
			{
				name: "Update",
				give: &pulumirpc.ConstructRequest_CustomTimeouts{Update: "2m"},
				want: &CustomTimeouts{Update: "2m"},
			},
			{
				name: "Delete",
				give: &pulumirpc.ConstructRequest_CustomTimeouts{Delete: "3m"},
				want: &CustomTimeouts{Delete: "3m"},
			},
			{
				name: "all",
				give: &pulumirpc.ConstructRequest_CustomTimeouts{
					Create: "1m",
					Update: "2m",
					Delete: "3m",
				},
				want: &CustomTimeouts{
					Create: "1m",
					Update: "2m",
					Delete: "3m",
				},
			},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				snap := snapshotFromRequest(t, &pulumirpc.ConstructRequest{
					CustomTimeouts: tt.give,
				})

				assert.Equal(t, tt.want, snap.CustomTimeouts)
			})
		}
	})

	t.Run("DeletedWith", func(t *testing.T) {
		t.Parallel()

		urn := resource.NewURN("mystack", "myproject", "", "foo:bar:Baz", "qux")
		snap := snapshotFromRequest(t, &pulumirpc.ConstructRequest{
			DeletedWith: string(urn),
		})
		assert.NotNil(t, snap.DeletedWith, "deletedWith was not set")
	})

	t.Run("DeleteBeforeReplace", func(t *testing.T) {
		t.Parallel()

		snap := snapshotFromRequest(t, &pulumirpc.ConstructRequest{
			DeleteBeforeReplace: true,
		})
		assert.True(t, snap.DeleteBeforeReplace, "deleteBeforeReplace was not set")
	})

	t.Run("IgnoreChanges", func(t *testing.T) {
		t.Parallel()

		snap := snapshotFromRequest(t, &pulumirpc.ConstructRequest{
			IgnoreChanges: []string{"foo"},
		})
		assert.Equal(t, []string{"foo"}, snap.IgnoreChanges)
	})

	t.Run("ReplaceOnChanges", func(t *testing.T) {
		t.Parallel()

		snap := snapshotFromRequest(t, &pulumirpc.ConstructRequest{
			ReplaceOnChanges: []string{"foo"},
		})
		assert.Equal(t, []string{"foo"}, snap.ReplaceOnChanges)
	})

	t.Run("RetainOnDelete", func(t *testing.T) {
		t.Parallel()

		snap := snapshotFromRequest(t, &pulumirpc.ConstructRequest{
			RetainOnDelete: true,
		})
		assert.True(t, snap.RetainOnDelete, "retainOnDelete was not set")
	})
}
