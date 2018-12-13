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

package pulumi

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

type mockResource struct {
	urn *URNOutput
}

func (r *mockResource) URN() *URNOutput {
	return r.urn
}

func newMockResource(urn string) Resource {
	r := &mockResource{}
	urnOutput, resolve, _ := NewOutput([]Resource{r})
	resolve(urn, true)
	r.urn = (*URNOutput)(urnOutput)
	return r
}

func assertSameDependencies(t *testing.T, expected []Resource, actual []Resource) {
	expectedDeps := make(map[URN]bool)
	for _, r := range expected {
		urn, _ := r.URN().Value()
		expectedDeps[urn] = true
	}

	actualDeps := make(map[URN]bool)
	for _, r := range actual {
		urn, _ := r.URN().Value()
		actualDeps[urn] = true
	}

	assert.Equal(t, expectedDeps, actualDeps)
}

func TestBasicOutputs(t *testing.T) {
	// Just test basic resolve and reject functionality.
	{
		out, resolve, _ := NewOutput(nil)
		go func() {
			resolve(42, true)
		}()
		v, known, deps, err := out.value()
		assert.Nil(t, err)
		assert.True(t, known)
		assert.NotNil(t, v)
		assert.Equal(t, 42, v.(int))
		assert.Empty(t, deps)
	}
	{
		out, _, reject := NewOutput(nil)
		go func() {
			reject(errors.New("boom"))
		}()
		v, _, err := out.Value()
		assert.NotNil(t, err)
		assert.Nil(t, v)
	}
}

func TestArrayOutputs(t *testing.T) {
	out, resolve, _ := NewOutput(nil)
	go func() {
		resolve([]interface{}{nil, 0, "x"}, true)
	}()
	{
		v, known, err := out.Array()
		assert.Nil(t, err)
		assert.True(t, known)
		assert.NotNil(t, v)
		if assert.Equal(t, 3, len(v)) {
			assert.Equal(t, nil, v[0])
			assert.Equal(t, 0, v[1])
			assert.Equal(t, "x", v[2])
		}
	}
	{
		arr := (*ArrayOutput)(out)
		v, _, err := arr.Value()
		assert.Nil(t, err)
		assert.NotNil(t, v)
		if assert.Equal(t, 3, len(v)) {
			assert.Equal(t, nil, v[0])
			assert.Equal(t, 0, v[1])
			assert.Equal(t, "x", v[2])
		}
	}
}

func TestBoolOutputs(t *testing.T) {
	out, resolve, _ := NewOutput(nil)
	go func() {
		resolve(true, true)
	}()
	{
		v, known, err := out.Bool()
		assert.Nil(t, err)
		assert.True(t, known)
		assert.True(t, v)
	}
	{
		b := (*BoolOutput)(out)
		v, known, err := b.Value()
		assert.Nil(t, err)
		assert.True(t, known)
		assert.True(t, v)
	}
}

func TestMapOutputs(t *testing.T) {
	out, resolve, _ := NewOutput(nil)
	go func() {
		resolve(map[string]interface{}{
			"x": 1,
			"y": false,
			"z": "abc",
		}, true)
	}()
	{
		v, known, err := out.Map()
		assert.Nil(t, err)
		assert.True(t, known)
		assert.NotNil(t, v)
		assert.Equal(t, 1, v["x"])
		assert.Equal(t, false, v["y"])
		assert.Equal(t, "abc", v["z"])
	}
	{
		b := (*MapOutput)(out)
		v, known, err := b.Value()
		assert.Nil(t, err)
		assert.True(t, known)
		assert.NotNil(t, v)
		assert.Equal(t, 1, v["x"])
		assert.Equal(t, false, v["y"])
		assert.Equal(t, "abc", v["z"])
	}
}

func TestNumberOutputs(t *testing.T) {
	out, resolve, _ := NewOutput(nil)
	go func() {
		resolve(42.345, true)
	}()
	{
		v, known, err := out.Float64()
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, 42.345, v)
	}
	{
		b := (*Float64Output)(out)
		v, known, err := b.Value()
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, 42.345, v)
	}
}

func TestStringOutputs(t *testing.T) {
	out, resolve, _ := NewOutput(nil)
	go func() {
		resolve("a stringy output", true)
	}()
	{
		v, known, err := out.String()
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, "a stringy output", v)
	}
	{
		b := (*StringOutput)(out)
		v, known, err := b.Value()
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, "a stringy output", v)
	}
}

func TestResolveOutputToOutput(t *testing.T) {
	// Test that resolving an output to an output yields the value, not the output.
	{
		out, resolve, _ := NewOutput(nil)
		go func() {
			other, resolveOther, _ := NewOutput(nil)
			resolve(other, true)
			go func() { resolveOther(99, true) }()
		}()
		v, known, err := out.Value()
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, v, 99)
	}
	// Similarly, test that resolving an output to a rejected output yields an error.
	{
		out, resolve, _ := NewOutput(nil)
		go func() {
			other, _, rejectOther := NewOutput(nil)
			resolve(other, true)
			go func() { rejectOther(errors.New("boom")) }()
		}()
		v, _, err := out.Value()
		assert.NotNil(t, err)
		assert.Nil(t, v)
	}
	// Test that resolving an output to another output properly unions dependencies
	{
		outerDep := newMockResource("outer")
		innerDep := newMockResource("inner")

		outer, resolveOuter, _ := NewOutput([]Resource{outerDep})
		go func() {
			inner, resolveInner, _ := NewOutput([]Resource{innerDep})
			resolveOuter(inner, true)
			go func() { resolveInner(99, true) }()
		}()

		v, known, deps, err := outer.value()
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, v, 99)

		assertSameDependencies(t, []Resource{outerDep, innerDep}, deps)
	}
}

func TestOutputApply(t *testing.T) {
	// Test that resolved outputs lead to applies being run.
	{
		out, resolve, _ := NewOutput(nil)
		go func() { resolve(42, true) }()
		var ranApp bool
		b := (*IntOutput)(out)
		app := b.Apply(func(v int) (interface{}, error) {
			ranApp = true
			return v + 1, nil
		})
		v, known, err := app.Value()
		assert.True(t, ranApp)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, v, 43)
	}
	// Test that resolved, but known outputs, skip the running of applies.
	{
		out, resolve, _ := NewOutput(nil)
		go func() { resolve(42, false) }()
		var ranApp bool
		b := (*IntOutput)(out)
		app := b.Apply(func(v int) (interface{}, error) {
			ranApp = true
			return v + 1, nil
		})
		_, known, err := app.Value()
		assert.False(t, ranApp)
		assert.Nil(t, err)
		assert.False(t, known)
	}
	// Test that rejected outputs do not run the apply, and instead flow the error.
	{
		out, _, reject := NewOutput(nil)
		go func() { reject(errors.New("boom")) }()
		var ranApp bool
		b := (*IntOutput)(out)
		app := b.Apply(func(v int) (interface{}, error) {
			ranApp = true
			return v + 1, nil
		})
		v, _, err := app.Value()
		assert.False(t, ranApp)
		assert.NotNil(t, err)
		assert.Nil(t, v)
	}
	// Test that an an apply that returns an output returns the resolution of that output, not the output itself.
	{
		outerDep := newMockResource("outer")
		innerDep := newMockResource("inner")

		out, resolve, _ := NewOutput([]Resource{outerDep})
		go func() { resolve(42, true) }()
		var ranApp bool
		b := (*IntOutput)(out)
		app := b.Apply(func(v int) (interface{}, error) {
			other, resolveOther, _ := NewOutput([]Resource{innerDep})
			go func() { resolveOther(v+1, true) }()
			ranApp = true
			return other, nil
		})
		v, known, deps, err := app.value()
		assert.True(t, ranApp)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, v, 43)

		assertSameDependencies(t, []Resource{outerDep, innerDep}, deps)
	}
	// Test that an an apply that reject an output returns the rejection of that output, not the output itself.
	{
		out, resolve, _ := NewOutput(nil)
		go func() { resolve(42, true) }()
		var ranApp bool
		b := (*IntOutput)(out)
		app := b.Apply(func(v int) (interface{}, error) {
			other, _, rejectOther := NewOutput(nil)
			go func() { rejectOther(errors.New("boom")) }()
			ranApp = true
			return other, nil
		})
		v, _, err := app.Value()
		assert.True(t, ranApp)
		assert.NotNil(t, err)
		assert.Nil(t, v)
	}
}
