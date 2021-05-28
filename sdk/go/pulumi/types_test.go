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

// nolint: lll
package pulumi

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func await(out Output) (interface{}, bool, bool, []Resource, error) {
	return out.getState().await(context.Background())
}

func assertApplied(t *testing.T, out Output) {
	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.Nil(t, err)
}

func newIntOutput() IntOutput {
	return IntOutput{newOutputState(nil, reflect.TypeOf(42))}
}

func TestBasicOutputs(t *testing.T) {
	// Just test basic resolve and reject functionality.
	{
		out, resolve, _ := NewOutput()
		go func() {
			resolve(42)
		}()
		v, known, secret, deps, err := await(out)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Nil(t, deps)
		assert.NotNil(t, v)
		assert.Equal(t, 42, v.(int))
	}
	{
		out, _, reject := NewOutput()
		go func() {
			reject(errors.New("boom"))
		}()
		v, _, _, _, err := await(out)
		assert.NotNil(t, err)
		assert.Nil(t, v)
	}
}

func TestArrayOutputs(t *testing.T) {
	out := ArrayOutput{newOutputState(nil, reflect.TypeOf([]interface{}{}))}
	go func() {
		out.resolve([]interface{}{nil, 0, "x"}, true, false, nil)
	}()
	{
		assertApplied(t, out.ApplyT(func(arr []interface{}) (interface{}, error) {
			assert.NotNil(t, arr)
			if assert.Equal(t, 3, len(arr)) {
				assert.Equal(t, nil, arr[0])
				assert.Equal(t, 0, arr[1])
				assert.Equal(t, "x", arr[2])
			}
			return nil, nil
		}))
	}
}

func TestBoolOutputs(t *testing.T) {
	out := BoolOutput{newOutputState(nil, reflect.TypeOf(false))}
	go func() {
		out.resolve(true, true, false, nil)
	}()
	{
		assertApplied(t, out.ApplyT(func(v bool) (interface{}, error) {
			assert.True(t, v)
			return nil, nil
		}))
	}
}

func TestMapOutputs(t *testing.T) {
	out := MapOutput{newOutputState(nil, reflect.TypeOf(map[string]interface{}{}))}
	go func() {
		out.resolve(map[string]interface{}{
			"x": 1,
			"y": false,
			"z": "abc",
		}, true, false, nil)
	}()
	{
		assertApplied(t, out.ApplyT(func(v map[string]interface{}) (interface{}, error) {
			assert.NotNil(t, v)
			assert.Equal(t, 1, v["x"])
			assert.Equal(t, false, v["y"])
			assert.Equal(t, "abc", v["z"])
			return nil, nil
		}))
	}
}

func TestNumberOutputs(t *testing.T) {
	out := Float64Output{newOutputState(nil, reflect.TypeOf(float64(0)))}
	go func() {
		out.resolve(42.345, true, false, nil)
	}()
	{
		assertApplied(t, out.ApplyT(func(v float64) (interface{}, error) {
			assert.Equal(t, 42.345, v)
			return nil, nil
		}))
	}
}

func TestStringOutputs(t *testing.T) {
	out := StringOutput{newOutputState(nil, reflect.TypeOf(""))}
	go func() {
		out.resolve("a stringy output", true, false, nil)
	}()
	{
		assertApplied(t, out.ApplyT(func(v string) (interface{}, error) {
			assert.Equal(t, "a stringy output", v)
			return nil, nil
		}))
	}
}

func TestResolveOutputToOutput(t *testing.T) {
	// Test that resolving an output to an output yields the value, not the output.
	{
		out, resolve, _ := NewOutput()
		go func() {
			other, resolveOther, _ := NewOutput()
			resolve(other)
			go func() { resolveOther(99) }()
		}()
		assertApplied(t, out.ApplyT(func(v interface{}) (interface{}, error) {
			assert.Equal(t, v, 99)
			return nil, nil
		}))
	}
	// Similarly, test that resolving an output to a rejected output yields an error.
	{
		out, resolve, _ := NewOutput()
		go func() {
			other, _, rejectOther := NewOutput()
			resolve(other)
			go func() { rejectOther(errors.New("boom")) }()
		}()
		v, _, _, _, err := await(out)
		assert.NotNil(t, err)
		assert.Nil(t, v)
	}
}

// Test that ToOutput works with a struct type.
func TestToOutputStruct(t *testing.T) {
	out := ToOutput(nestedTypeInputs{Foo: String("bar"), Bar: Int(42)})
	_, ok := out.(nestedTypeOutput)
	assert.True(t, ok)

	v, known, secret, deps, err := await(out)
	assert.True(t, known)
	assert.False(t, secret)
	assert.Nil(t, deps)
	assert.NoError(t, err)
	assert.Equal(t, nestedType{Foo: "bar", Bar: 42}, v)

	out = ToOutput(out)
	_, ok = out.(nestedTypeOutput)
	assert.True(t, ok)

	v, known, secret, deps, err = await(out)
	assert.True(t, known)
	assert.False(t, secret)
	assert.Nil(t, deps)

	assert.NoError(t, err)
	assert.Equal(t, nestedType{Foo: "bar", Bar: 42}, v)

	out = ToOutput(nestedTypeInputs{Foo: ToOutput(String("bar")).(StringInput), Bar: ToOutput(Int(42)).(IntInput)})
	_, ok = out.(nestedTypeOutput)
	assert.True(t, ok)

	v, known, secret, deps, err = await(out)
	assert.True(t, known)
	assert.False(t, secret)
	assert.Nil(t, deps)
	assert.NoError(t, err)
	assert.Equal(t, nestedType{Foo: "bar", Bar: 42}, v)
}

type arrayLenInput Array

func (arrayLenInput) ElementType() reflect.Type {
	return Array{}.ElementType()
}

func (i arrayLenInput) ToIntOutput() IntOutput {
	return i.ToIntOutputWithContext(context.Background())
}

func (i arrayLenInput) ToIntOutputWithContext(ctx context.Context) IntOutput {
	return ToOutput(i).ApplyT(func(arr []interface{}) int {
		return len(arr)
	}).(IntOutput)
}

func (i arrayLenInput) ToIntPtrOutput() IntPtrOutput {
	return i.ToIntPtrOutputWithContext(context.Background())
}

func (i arrayLenInput) ToIntPtrOutputWithContext(ctx context.Context) IntPtrOutput {
	return ToOutput(i).ApplyT(func(arr []interface{}) *int {
		v := len(arr)
		return &v
	}).(IntPtrOutput)
}

// Test that ToOutput converts inputs appropriately.
func TestToOutputConvert(t *testing.T) {
	out := ToOutput(nestedTypeInputs{Foo: ID("bar"), Bar: arrayLenInput{Int(42)}})
	_, ok := out.(nestedTypeOutput)
	assert.True(t, ok)

	v, known, secret, deps, err := await(out)
	assert.True(t, known)
	assert.False(t, secret)
	assert.Nil(t, deps)
	assert.NoError(t, err)
	assert.Equal(t, nestedType{Foo: "bar", Bar: 1}, v)
}

// Test that ToOutput correctly handles nested inputs and outputs when the argument is an input or interface{}.
func TestToOutputAny(t *testing.T) {
	type args struct {
		S StringInput
		I IntInput
		A Input
	}

	out := ToOutput(&args{
		S: ID("hello"),
		I: Int(42).ToIntOutput(),
		A: Map{"world": Bool(true).ToBoolOutput()},
	})
	_, ok := out.(AnyOutput)
	assert.True(t, ok)

	v, known, secret, deps, err := await(out)
	assert.True(t, known)
	assert.False(t, secret)
	assert.Nil(t, deps)
	assert.NoError(t, err)

	argsV := v.(*args)

	si, ok := argsV.S.(ID)
	assert.True(t, ok)
	assert.Equal(t, ID("hello"), si)

	io, ok := argsV.I.(IntOutput)
	assert.True(t, ok)
	assert.Equal(t, uint32(outputResolved), io.state)
	assert.Equal(t, 42, io.value)

	ai, ok := argsV.A.(Map)
	assert.True(t, ok)

	bo, ok := ai["world"].(BoolOutput)
	assert.True(t, ok)
	assert.Equal(t, uint32(outputResolved), bo.getState().state)
	assert.Equal(t, true, bo.value)
}

func TestToOutputAnyDeps(t *testing.T) {
	type args struct {
		S StringInput
		I IntInput
		A Input
		R Resource
	}

	stringDep1, stringDep2 := &ResourceState{}, &ResourceState{}
	stringOut := StringOutput{newOutputState(nil, reflect.TypeOf(""), stringDep1)}
	go func() {
		stringOut.resolve("a stringy output", true, false, []Resource{stringDep2})
	}()

	intDep1, intDep2 := &ResourceState{}, &ResourceState{}
	intOut := IntOutput{newOutputState(nil, reflect.TypeOf(0), intDep1)}
	go func() {
		intOut.resolve(42, true, false, []Resource{intDep2})
	}()

	boolDep1, boolDep2 := &ResourceState{}, &ResourceState{}
	boolOut := BoolOutput{newOutputState(nil, reflect.TypeOf(true), boolDep1)}
	go func() {
		boolOut.resolve(true, true, false, []Resource{boolDep2})
	}()

	res := &ResourceState{}

	out := ToOutput(&args{
		S: stringOut,
		I: intOut,
		A: Map{"world": boolOut},
		R: res,
	})
	_, ok := out.(AnyOutput)
	assert.True(t, ok)

	v, known, secret, deps, err := await(out)
	assert.True(t, known)
	assert.False(t, secret)
	assert.ElementsMatch(t, []Resource{stringDep1, stringDep2, intDep1, intDep2, boolDep1, boolDep2, res}, deps)
	assert.NoError(t, err)

	argsV := v.(*args)

	so, ok := argsV.S.(StringOutput)
	assert.True(t, ok)
	assert.Equal(t, uint32(outputResolved), so.state)
	assert.Equal(t, "a stringy output", so.value)
	assert.ElementsMatch(t, []Resource{stringDep1, stringDep2}, so.deps)

	io, ok := argsV.I.(IntOutput)
	assert.True(t, ok)
	assert.Equal(t, uint32(outputResolved), io.state)
	assert.Equal(t, 42, io.value)
	assert.ElementsMatch(t, []Resource{intDep1, intDep2}, io.deps)

	ai, ok := argsV.A.(Map)
	assert.True(t, ok)

	bo, ok := ai["world"].(BoolOutput)
	assert.True(t, ok)
	assert.Equal(t, uint32(outputResolved), bo.getState().state)
	assert.Equal(t, true, bo.value)
	assert.ElementsMatch(t, []Resource{boolDep1, boolDep2}, bo.deps)
}

type args struct {
	S string
	I int
	A interface{}
}

type argsInputs struct {
	S StringInput
	I IntInput
	A Input
}

func (*argsInputs) ElementType() reflect.Type {
	return reflect.TypeOf((*args)(nil))
}

// Test that ToOutput correctly handles nested inputs when the argument is an input with no corresponding output type.
func TestToOutputInputAny(t *testing.T) {
	out := ToOutput(&argsInputs{
		S: ID("hello"),
		I: Int(42),
		A: Map{"world": Bool(true).ToBoolOutput()},
	})
	_, ok := out.(AnyOutput)
	assert.True(t, ok)

	v, known, secret, deps, err := await(out)
	assert.True(t, known)
	assert.False(t, secret)
	assert.Nil(t, deps)
	assert.NoError(t, err)

	assert.Equal(t, &args{
		S: "hello",
		I: 42,
		A: map[string]interface{}{"world": true},
	}, v)
}

// Test that Unsecret will return an Output that has an unwrapped secret
func TestUnsecret(t *testing.T) {
	s := ToSecret(String("foo"))
	// assert that secret is immediately secret
	assert.True(t, IsSecret(s))

	unS := Unsecret(s)
	// assert that we do not have a secret
	assert.False(t, IsSecret(unS))

	errChan := make(chan error)
	resultChan := make(chan string)
	secretChan := make(chan bool)

	unS.ApplyT(func(v interface{}) (string, error) {
		// assert secretness after the output resolves
		secretChan <- IsSecret(unS)
		val := v.(string)
		if val == "foo" {
			// validate the value
			resultChan <- val
		} else {
			errChan <- fmt.Errorf("Invalid result: %v", val)
		}
		return val, nil
	})

	for i := 0; i < 2; i++ {
		select {
		case err := <-errChan:
			assert.Nil(t, err)
			break
		case r := <-resultChan:
			assert.Equal(t, "foo", r)
			break
		case isSecret := <-secretChan:
			assert.False(t, isSecret)
			break
		}
	}
}

// Test that SecretT sets appropriate internal state and that IsSecret appropriately reads it.
func TestSecrets(t *testing.T) {
	s := ToSecret(String("foo"))
	// assert that secret is immediately secret
	assert.True(t, IsSecret(s))

	errChan := make(chan error)
	resultChan := make(chan string)
	secretChan := make(chan bool)

	s.ApplyT(func(v interface{}) (string, error) {
		// assert secretness after the output resolves
		secretChan <- IsSecret(s)
		val := v.(string)
		if val == "foo" {
			// validate the value
			resultChan <- val
		} else {
			errChan <- fmt.Errorf("Invalid result: %v", val)
		}
		return val, nil
	})

	for i := 0; i < 2; i++ {
		select {
		case err := <-errChan:
			assert.Nil(t, err)
			break
		case r := <-resultChan:
			assert.Equal(t, "foo", r)
			break
		case isSecret := <-secretChan:
			assert.True(t, isSecret)
			break
		}
	}

}

// Test that secretness is properly bubbled up with all/apply.
func TestSecretApply(t *testing.T) {
	s1 := ToSecret(String("foo"))
	// assert that secret is immediately secret
	assert.True(t, IsSecret(s1))
	s2 := StringInput(String("bar"))

	errChan := make(chan error)
	resultChan := make(chan string)
	secretChan := make(chan bool)

	s := All(s1, s2).ApplyT(func(v interface{}) (string, error) {
		val := v.([]interface{})
		return val[0].(string) + val[1].(string), nil
	})
	s.ApplyT(func(v interface{}) (string, error) {
		// assert secretness after the output resolves
		secretChan <- IsSecret(s)
		val := v.(string)
		if val == "foobar" {
			// validate the value
			resultChan <- val
		} else {
			errChan <- fmt.Errorf("Invalid result: %v", val)
		}
		return val, nil
	})

	for i := 0; i < 2; i++ {
		select {
		case err := <-errChan:
			assert.Nil(t, err)
			break
		case r := <-resultChan:
			assert.Equal(t, "foobar", r)
			break
		case isSecret := <-secretChan:
			assert.True(t, isSecret)
			break
		}
	}

}

func TestNil(t *testing.T) {
	ao := Any(nil)
	v, known, secret, deps, err := await(ao)
	assert.True(t, known)
	assert.False(t, secret)
	assert.Nil(t, deps)
	assert.NoError(t, err)
	assert.Equal(t, nil, v)

	o := ToOutput(nil)
	v, known, secret, deps, err = await(o)
	assert.True(t, known)
	assert.False(t, secret)
	assert.Nil(t, deps)
	assert.NoError(t, err)
	assert.Equal(t, nil, v)

	o = ToOutput(ao)
	v, known, secret, deps, err = await(o)
	assert.True(t, known)
	assert.False(t, secret)
	assert.Nil(t, deps)
	assert.NoError(t, err)
	assert.Equal(t, nil, v)

	ao = ToOutput("").ApplyT(func(v string) interface{} {
		return nil
	}).(AnyOutput)
	v, known, secret, deps, err = await(ao)
	assert.True(t, known)
	assert.False(t, secret)
	assert.Nil(t, deps)
	assert.NoError(t, err)
	assert.Equal(t, nil, v)

	bo := ao.ApplyT(func(x interface{}) bool {
		return x == nil
	})
	v, known, secret, deps, err = await(bo)
	assert.True(t, known)
	assert.False(t, secret)
	assert.Nil(t, deps)
	assert.NoError(t, err)
	assert.Equal(t, true, v)
}

// Test that dependencies flow through all/apply.
func TestDeps(t *testing.T) {
	stringDep1, stringDep2 := &ResourceState{}, &ResourceState{}
	stringOut := StringOutput{newOutputState(nil, reflect.TypeOf(""), stringDep1)}
	assert.ElementsMatch(t, []Resource{stringDep1}, stringOut.deps)
	go func() {
		stringOut.resolve("hello", true, false, []Resource{stringDep2})
	}()

	boolDep1, boolDep2 := &ResourceState{}, &ResourceState{}
	boolOut := BoolOutput{newOutputState(nil, reflect.TypeOf(true), boolDep1)}
	assert.ElementsMatch(t, []Resource{boolDep1}, boolOut.deps)
	go func() {
		boolOut.resolve(true, true, false, []Resource{boolDep2})
	}()

	a := All(stringOut, boolOut).ApplyT(func(args []interface{}) (string, error) {
		s := args[0].(string)
		b := args[1].(bool)
		return fmt.Sprintf("%s: %v", s, b), nil
	})

	v, known, secret, deps, err := await(a)
	assert.Equal(t, "hello: true", v)
	assert.True(t, known)
	assert.False(t, secret)
	assert.ElementsMatch(t, []Resource{stringDep1, stringDep2, boolDep1, boolDep2}, deps)
	assert.NoError(t, err)
}

func testMixedWaitGroups(t *testing.T, combine func(o1, o2 Output) Output) {
	var wg1, wg2 sync.WaitGroup

	o1 := newOutput(&wg1, anyOutputType)
	o2 := newOutput(&wg2, anyOutputType)

	gate := make(chan chan bool)
	combine(o1, o2).ApplyT(func(_ interface{}) interface{} {
		<-gate <- true
		return 0
	})

	wg1Done, wg2Done := false, false
	go func() {
		wg1.Wait()
		wg1Done = true
		wg2.Wait()
		wg2Done = true
	}()

	o1.getState().resolve(0, true, true, nil)
	o2.getState().resolve(0, true, true, nil)

	c := make(chan bool)
	gate <- c
	assert.False(t, wg1Done)
	assert.False(t, wg2Done)
	<-c
	wg1.Wait()
	wg2.Wait()

}

func TestMixedWaitGroupsAll(t *testing.T) {
	testMixedWaitGroups(t, func(o1, o2 Output) Output {
		return All(o1, o2)
	})
}

func TestMixedWaitGroupsAny(t *testing.T) {
	testMixedWaitGroups(t, func(o1, o2 Output) Output {
		return Any(struct{ O1, O2 Output }{o1, o2})
	})
}

func TestMixedWaitGroupsApply(t *testing.T) {
	testMixedWaitGroups(t, func(o1, o2 Output) Output {
		return o1.ApplyT(func(_ interface{}) interface{} {
			return o2
		})
	})
}
