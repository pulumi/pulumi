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

func TestBasicOutputs(t *testing.T) {
	// Just test basic resolve and reject functionality.
	{
		out, resolve, _ := NewOutput(nil)
		go func() {
			resolve(42)
		}()
		v, err := out.Value()
		assert.Nil(t, err)
		assert.NotNil(t, v)
		assert.Equal(t, 42, v.(int))
	}
	{
		out, _, reject := NewOutput(nil)
		go func() {
			reject(errors.New("boom"))
		}()
		v, err := out.Value()
		assert.NotNil(t, err)
		assert.Nil(t, v)
	}
}

func TestArrayOutputs(t *testing.T) {
	out, resolve, _ := NewOutput(nil)
	go func() {
		resolve([]interface{}{nil, 0, "x"})
	}()
	{
		v, err := out.Array()
		assert.Nil(t, err)
		assert.NotNil(t, v)
		if assert.Equal(t, 3, len(v)) {
			assert.Equal(t, nil, v[0])
			assert.Equal(t, 0, v[1])
			assert.Equal(t, "x", v[2])
		}
	}
	{
		arr := (*ArrayOutput)(out)
		v, err := arr.Value()
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
		resolve(true)
	}()
	{
		v, err := out.Bool()
		assert.Nil(t, err)
		assert.True(t, v)
	}
	{
		b := (*BoolOutput)(out)
		v, err := b.Value()
		assert.Nil(t, err)
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
		})
	}()
	{
		v, err := out.Map()
		assert.Nil(t, err)
		assert.NotNil(t, v)
		assert.Equal(t, 1, v["x"])
		assert.Equal(t, false, v["y"])
		assert.Equal(t, "abc", v["z"])
	}
	{
		b := (*MapOutput)(out)
		v, err := b.Value()
		assert.Nil(t, err)
		assert.NotNil(t, v)
		assert.Equal(t, 1, v["x"])
		assert.Equal(t, false, v["y"])
		assert.Equal(t, "abc", v["z"])
	}
}

func TestNumberOutputs(t *testing.T) {
	out, resolve, _ := NewOutput(nil)
	go func() {
		resolve(42.345)
	}()
	{
		v, err := out.Number()
		assert.Nil(t, err)
		assert.Equal(t, 42.345, v)
	}
	{
		b := (*NumberOutput)(out)
		v, err := b.Value()
		assert.Nil(t, err)
		assert.Equal(t, 42.345, v)
	}
}

func TestStringOutputs(t *testing.T) {
	out, resolve, _ := NewOutput(nil)
	go func() {
		resolve("a stringy output")
	}()
	{
		v, err := out.String()
		assert.Nil(t, err)
		assert.Equal(t, "a stringy output", v)
	}
	{
		b := (*StringOutput)(out)
		v, err := b.Value()
		assert.Nil(t, err)
		assert.Equal(t, "a stringy output", v)
	}
}
