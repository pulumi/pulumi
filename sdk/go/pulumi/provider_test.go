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

package pulumi

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

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

func TestConstructInputsCopyTo(t *testing.T) {
	var ctx Context

	bar := "bar"
	dep := ctx.newDependencyResource(URN(resource.NewURN("stack", "project", "", "test:index:custom", "test")))
	tests := []struct {
		input          resource.PropertyValue
		deps           []Resource
		args           interface{}
		expectedType   interface{}
		expectedValue  interface{}
		expectedSecret bool
		expectedDeps   []Resource
		typeOnly       bool
	}{
		{
			input:         resource.NewStringProperty("foo"),
			args:          &StringInputArgs{},
			expectedType:  StringOutput{},
			expectedValue: "foo",
		},
		{
			input:          resource.MakeSecret(resource.NewStringProperty("foo")),
			args:           &StringInputArgs{},
			expectedType:   StringOutput{},
			expectedValue:  "foo",
			expectedSecret: true,
		},
		{
			input:         resource.NewStringProperty("foo"),
			deps:          []Resource{dep},
			args:          &StringInputArgs{},
			expectedType:  StringOutput{},
			expectedValue: "foo",
			expectedDeps:  []Resource{dep},
		},
		{
			input:         resource.NewStringProperty("bar"),
			args:          &StringPtrInputArgs{},
			expectedType:  StringPtrOutput{},
			expectedValue: &bar,
		},
		{
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("hello"),
				resource.NewStringProperty("world"),
			}),
			args:          &StringArrayInputArgs{},
			expectedType:  StringArrayOutput{},
			expectedValue: []string{"hello", "world"},
		},
		{
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": "hello",
				"bar": "world",
			})),
			args:         &StringMapInputArgs{},
			expectedType: StringMapOutput{},
			expectedValue: map[string]string{
				"foo": "hello",
				"bar": "world",
			},
		},
		{
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": "hello",
				"bar": 42,
			})),
			args:          &NestedOutputArgs{},
			expectedType:  NestedOutput{},
			expectedValue: Nested{Foo: "hello", Bar: 42},
		},
		{
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": "world",
				"bar": 100,
			})),
			args:          &NestedPtrOutputArgs{},
			expectedType:  NestedPtrOutput{},
			expectedValue: &Nested{Foo: "world", Bar: 100},
		},
		{
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
			args:         &NestedArrayOutputArgs{},
			expectedType: NestedArrayOutput{},
			expectedValue: []Nested{
				{Foo: "a", Bar: 1},
				{Foo: "b", Bar: 2},
			},
		},
		{
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
			args:         &NestedMapOutputArgs{},
			expectedType: NestedMapOutput{},
			expectedValue: map[string]Nested{
				"a": {Foo: "c", Bar: 3},
				"b": {Foo: "d", Bar: 4},
			},
		},
		{
			input:         resource.NewStringProperty("foo"),
			args:          &StringArgs{},
			expectedType:  string(""),
			expectedValue: "foo",
		},
		{
			input:         resource.NewStringProperty("bar"),
			args:          &StringPtrArgs{},
			expectedType:  &bar,
			expectedValue: &bar,
		},
		{
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("hello"),
				resource.NewStringProperty("world"),
			}),
			args:          &StringArrayArgs{},
			expectedType:  []string{},
			expectedValue: []string{"hello", "world"},
		},
		{
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": "hello",
				"bar": "world",
			})),
			args:         &StringMapArgs{},
			expectedType: map[string]string{},
			expectedValue: map[string]string{
				"foo": "hello",
				"bar": "world",
			},
		},
		{
			input:         resource.NewNumberProperty(42),
			args:          &IntArgs{},
			expectedType:  int(0),
			expectedValue: 42,
		},
		{
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": "hi",
				"bar": 7,
			})),
			args:          &NestedArgs{},
			expectedType:  Nested{},
			expectedValue: Nested{Foo: "hi", Bar: 7},
		},
		{
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": "pointer",
				"bar": 2,
			})),
			args:          &NestedPtrArgs{},
			expectedType:  &Nested{},
			expectedValue: &Nested{Foo: "pointer", Bar: 2},
		},
		{
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
			args:         &NestedArrayArgs{},
			expectedType: []Nested{},
			expectedValue: []Nested{
				{Foo: "1", Bar: 1},
				{Foo: "2", Bar: 2},
			},
		},
		{
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
			args:         &NestedMapArgs{},
			expectedType: map[string]Nested{},
			expectedValue: map[string]Nested{
				"a": {Foo: "3", Bar: 3},
				"b": {Foo: "4", Bar: 4},
			},
		},
		{
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("foo"),
				resource.NewStringProperty("bar"),
			}),
			args:         &PlainArrayArgs{},
			expectedType: []StringInput{},
			typeOnly:     true,
		},
		{
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": "bar",
				"baz": "qux",
			})),
			args:         &PlainMapArgs{},
			expectedType: map[string]StringInput{},
			typeOnly:     true,
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%T-%v-%T", test.args, test.input, test.expectedType), func(t *testing.T) {
			ctx, err := NewContext(context.Background(), RunInfo{})
			require.NoError(t, err)

			inputs := map[string]interface{}{
				"value": &constructInput{value: test.input, deps: test.deps},
			}
			err = constructInputsCopyTo(ctx, inputs, test.args)
			require.NoError(t, err)

			result := reflect.ValueOf(test.args).Elem().FieldByName("Value").Interface()
			require.IsType(t, test.expectedType, result)

			if test.typeOnly {
				return
			}

			if out, ok := result.(Output); ok {
				value, known, secret, deps, err := await(out)
				assert.NoError(t, err)
				assert.Equal(t, test.expectedValue, value)
				assert.True(t, known)
				assert.Equal(t, test.expectedSecret, secret)
				assert.Equal(t, test.expectedDeps, deps)
			} else {
				if test.expectedValue == nil {
					assert.Nil(t, result)
					assert.True(t, false)
				} else {
					assert.Equal(t, test.expectedValue, result)
				}
			}
		})
	}
}

func TestConstructInputsCopyToError(t *testing.T) {
	var ctx Context

	tests := []struct {
		input         resource.PropertyValue
		deps          []Resource
		args          interface{}
		expectedError string
	}{
		{
			input:         resource.NewStringProperty("foo"),
			args:          &IntArgs{},
			expectedError: "unmarshaling input value: expected an int, got a string",
		},
		{
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": 500,
				"bar": 42,
			})),
			args:          &NestedArgs{},
			expectedError: "unmarshaling input value: expected a string, got a number",
		},
		{
			input: resource.MakeSecret(resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": "hello",
				"bar": 42,
			}))),
			args: &NestedArgs{},
			expectedError: "pulumi.NestedArgs.Value is typed as pulumi.Nested but must be typed as Input or " +
				"Output for secret input \"value\"",
		},
		{
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": "hello",
				"bar": 42,
			})),
			deps: []Resource{
				ctx.newDependencyResource(URN(resource.NewURN("stack", "project", "", "test:index:custom", "test"))),
			},
			args: &NestedArgs{},
			expectedError: "pulumi.NestedArgs.Value is typed as pulumi.Nested but must be typed as Input or " +
				"Output for input \"value\" with dependencies",
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%T-%v", test.args, test.input), func(t *testing.T) {
			ctx, err := NewContext(context.Background(), RunInfo{})
			assert.NoError(t, err)

			inputs := map[string]interface{}{
				"value": &constructInput{value: test.input, deps: test.deps},
			}
			err = constructInputsCopyTo(ctx, inputs, test.args)
			if assert.Error(t, err) {
				assert.Equal(t, test.expectedError, err.Error())
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

	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo":       "hi",
		"someValue": "something",
	}), resolvedProps)
}
