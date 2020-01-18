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

// nolint: lll, unconvert
package pulumi

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestOutputApply(t *testing.T) {
	// Test that resolved outputs lead to applies being run.
	{
		out := newIntOutput()
		go func() { out.resolve(42, true) }()
		var ranApp bool
		app := out.ApplyT(func(v int) (interface{}, error) {
			ranApp = true
			return v + 1, nil
		})
		v, known, err := await(app)
		assert.True(t, ranApp)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, v, 43)
	}
	// Test that resolved, but unknown outputs, skip the running of applies.
	{
		out := newIntOutput()
		go func() { out.resolve(42, false) }()
		var ranApp bool
		app := out.ApplyT(func(v int) (interface{}, error) {
			ranApp = true
			return v + 1, nil
		})
		_, known, err := await(app)
		assert.False(t, ranApp)
		assert.Nil(t, err)
		assert.False(t, known)
	}
	// Test that rejected outputs do not run the apply, and instead flow the error.
	{
		out := newIntOutput()
		go func() { out.reject(errors.New("boom")) }()
		var ranApp bool
		app := out.ApplyT(func(v int) (interface{}, error) {
			ranApp = true
			return v + 1, nil
		})
		v, _, err := await(app)
		assert.False(t, ranApp)
		assert.NotNil(t, err)
		assert.Nil(t, v)
	}
	// Test that an an apply that returns an output returns the resolution of that output, not the output itself.
	{
		out := newIntOutput()
		go func() { out.resolve(42, true) }()
		var ranApp bool
		app := out.ApplyT(func(v int) (interface{}, error) {
			other, resolveOther, _ := NewOutput()
			go func() { resolveOther(v + 1) }()
			ranApp = true
			return other, nil
		})
		v, known, err := await(app)
		assert.True(t, ranApp)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, v, 43)

		app = out.ApplyT(func(v int) (interface{}, error) {
			other, resolveOther, _ := NewOutput()
			go func() { resolveOther(v + 2) }()
			ranApp = true
			return other, nil
		})
		v, known, err = await(app)
		assert.True(t, ranApp)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, v, 44)
	}
	// Test that an an apply that reject an output returns the rejection of that output, not the output itself.
	{
		out := newIntOutput()
		go func() { out.resolve(42, true) }()
		var ranApp bool
		app := out.ApplyT(func(v int) (interface{}, error) {
			other, _, rejectOther := NewOutput()
			go func() { rejectOther(errors.New("boom")) }()
			ranApp = true
			return other, nil
		})
		v, _, err := await(app)
		assert.True(t, ranApp)
		assert.NotNil(t, err)
		assert.Nil(t, v)

		app = out.ApplyT(func(v int) (interface{}, error) {
			other, _, rejectOther := NewOutput()
			go func() { rejectOther(errors.New("boom")) }()
			ranApp = true
			return other, nil
		})
		v, _, err = await(app)
		assert.True(t, ranApp)
		assert.NotNil(t, err)
		assert.Nil(t, v)
	}
	// Test that applies return appropriate concrete implementations of Output based on the callback type
	{
		out := newIntOutput()
		go func() { out.resolve(42, true) }()

		t.Run("ApplyT::ArchiveOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) Archive { return *new(Archive) }).(ArchiveOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::ArchiveArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []Archive { return *new([]Archive) }).(ArchiveArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::ArchiveMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]Archive { return *new(map[string]Archive) }).(ArchiveMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::AssetOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) Asset { return *new(Asset) }).(AssetOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::AssetArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []Asset { return *new([]Asset) }).(AssetArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::AssetMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]Asset { return *new(map[string]Asset) }).(AssetMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::AssetOrArchiveOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) AssetOrArchive { return *new(AssetOrArchive) }).(AssetOrArchiveOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::AssetOrArchiveArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []AssetOrArchive { return *new([]AssetOrArchive) }).(AssetOrArchiveArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::AssetOrArchiveMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]AssetOrArchive { return *new(map[string]AssetOrArchive) }).(AssetOrArchiveMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::BoolOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) bool { return *new(bool) }).(BoolOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::BoolPtrOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) *bool { return *new(*bool) }).(BoolPtrOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::BoolArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []bool { return *new([]bool) }).(BoolArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::BoolMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]bool { return *new(map[string]bool) }).(BoolMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Float32Output", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) float32 { return *new(float32) }).(Float32Output)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Float32PtrOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) *float32 { return *new(*float32) }).(Float32PtrOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Float32ArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []float32 { return *new([]float32) }).(Float32ArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Float32MapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]float32 { return *new(map[string]float32) }).(Float32MapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Float64Output", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) float64 { return *new(float64) }).(Float64Output)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Float64PtrOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) *float64 { return *new(*float64) }).(Float64PtrOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Float64ArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []float64 { return *new([]float64) }).(Float64ArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Float64MapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]float64 { return *new(map[string]float64) }).(Float64MapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::IDOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) ID { return *new(ID) }).(IDOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::IDPtrOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) *ID { return *new(*ID) }).(IDPtrOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::IDArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []ID { return *new([]ID) }).(IDArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::IDMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]ID { return *new(map[string]ID) }).(IDMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::ArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []interface{} { return *new([]interface{}) }).(ArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::MapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]interface{} { return *new(map[string]interface{}) }).(MapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::IntOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) int { return *new(int) }).(IntOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::IntPtrOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) *int { return *new(*int) }).(IntPtrOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::IntArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []int { return *new([]int) }).(IntArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::IntMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]int { return *new(map[string]int) }).(IntMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int16Output", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) int16 { return *new(int16) }).(Int16Output)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int16PtrOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) *int16 { return *new(*int16) }).(Int16PtrOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int16ArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []int16 { return *new([]int16) }).(Int16ArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int16MapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]int16 { return *new(map[string]int16) }).(Int16MapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int32Output", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) int32 { return *new(int32) }).(Int32Output)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int32PtrOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) *int32 { return *new(*int32) }).(Int32PtrOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int32ArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []int32 { return *new([]int32) }).(Int32ArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int32MapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]int32 { return *new(map[string]int32) }).(Int32MapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int64Output", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) int64 { return *new(int64) }).(Int64Output)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int64PtrOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) *int64 { return *new(*int64) }).(Int64PtrOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int64ArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []int64 { return *new([]int64) }).(Int64ArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int64MapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]int64 { return *new(map[string]int64) }).(Int64MapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int8Output", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) int8 { return *new(int8) }).(Int8Output)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int8PtrOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) *int8 { return *new(*int8) }).(Int8PtrOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int8ArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []int8 { return *new([]int8) }).(Int8ArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int8MapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]int8 { return *new(map[string]int8) }).(Int8MapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::StringOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) string { return *new(string) }).(StringOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::StringPtrOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) *string { return *new(*string) }).(StringPtrOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::StringArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []string { return *new([]string) }).(StringArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::StringMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]string { return *new(map[string]string) }).(StringMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::URNOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) URN { return *new(URN) }).(URNOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::URNPtrOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) *URN { return *new(*URN) }).(URNPtrOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::URNArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []URN { return *new([]URN) }).(URNArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::URNMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]URN { return *new(map[string]URN) }).(URNMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::UintOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) uint { return *new(uint) }).(UintOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::UintPtrOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) *uint { return *new(*uint) }).(UintPtrOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::UintArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []uint { return *new([]uint) }).(UintArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::UintMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]uint { return *new(map[string]uint) }).(UintMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint16Output", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) uint16 { return *new(uint16) }).(Uint16Output)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint16PtrOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) *uint16 { return *new(*uint16) }).(Uint16PtrOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint16ArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []uint16 { return *new([]uint16) }).(Uint16ArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint16MapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]uint16 { return *new(map[string]uint16) }).(Uint16MapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint32Output", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) uint32 { return *new(uint32) }).(Uint32Output)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint32PtrOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) *uint32 { return *new(*uint32) }).(Uint32PtrOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint32ArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []uint32 { return *new([]uint32) }).(Uint32ArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint32MapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]uint32 { return *new(map[string]uint32) }).(Uint32MapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint64Output", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) uint64 { return *new(uint64) }).(Uint64Output)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint64PtrOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) *uint64 { return *new(*uint64) }).(Uint64PtrOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint64ArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []uint64 { return *new([]uint64) }).(Uint64ArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint64MapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]uint64 { return *new(map[string]uint64) }).(Uint64MapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint8Output", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) uint8 { return *new(uint8) }).(Uint8Output)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint8PtrOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) *uint8 { return *new(*uint8) }).(Uint8PtrOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint8ArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []uint8 { return *new([]uint8) }).(Uint8ArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint8MapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]uint8 { return *new(map[string]uint8) }).(Uint8MapOutput)
			assert.True(t, ok)
		})

	}
	// Test some chained applies.
	{
		type myStructType struct {
			foo int
			bar string
		}

		out := newIntOutput()
		go func() { out.resolve(42, true) }()

		out2 := StringOutput{newOutputState(reflect.TypeOf(""))}
		go func() { out2.resolve("hello", true) }()

		res := out.
			ApplyT(func(v int) myStructType {
				return myStructType{foo: v, bar: "qux,zed"}
			}).
			ApplyT(func(v interface{}) (string, error) {
				bar := v.(myStructType).bar
				if bar != "qux,zed" {
					return "", errors.New("unexpected value")
				}
				return bar, nil
			}).
			ApplyT(func(v string) ([]string, error) {
				strs := strings.Split(v, ",")
				if len(strs) != 2 {
					return nil, errors.New("unexpected value")
				}
				return []string{strs[0], strs[1]}, nil
			})

		res2 := out.
			ApplyT(func(v int) myStructType {
				return myStructType{foo: v, bar: "foo,bar"}
			}).
			ApplyT(func(v interface{}) (string, error) {
				bar := v.(myStructType).bar
				if bar != "foo,bar" {
					return "", errors.New("unexpected value")
				}
				return bar, nil
			}).
			ApplyT(func(v string) ([]string, error) {
				strs := strings.Split(v, ",")
				if len(strs) != 2 {
					return nil, errors.New("unexpected value")
				}
				return []string{strs[0], strs[1]}, nil
			})

		res3 := All(res, res2).ApplyT(func(v []interface{}) string {
			res, res2 := v[0].([]string), v[1].([]string)
			return strings.Join(append(res2, res...), ",")
		})

		res4 := All(out, out2).ApplyT(func(v []interface{}) *myStructType {
			return &myStructType{
				foo: v[0].(int),
				bar: v[1].(string),
			}
		})

		res5 := All(res3, res4).Apply(func(v interface{}) (interface{}, error) {
			vs := v.([]interface{})
			res3 := vs[0].(string)
			res4 := vs[1].(*myStructType)
			return fmt.Sprintf("%v;%v;%v", res3, res4.foo, res4.bar), nil
		})

		_, ok := res.(StringArrayOutput)
		assert.True(t, ok)

		v, known, err := await(res)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, []string{"qux", "zed"}, v)

		_, ok = res2.(StringArrayOutput)
		assert.True(t, ok)

		v, known, err = await(res2)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, []string{"foo", "bar"}, v)

		_, ok = res3.(StringOutput)
		assert.True(t, ok)

		v, known, err = await(res3)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, "foo,bar,qux,zed", v)

		_, ok = res4.(AnyOutput)
		assert.True(t, ok)

		v, known, err = await(res4)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, &myStructType{foo: 42, bar: "hello"}, v)

		v, known, err = await(res5)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, "foo,bar,qux,zed;42;hello", v)
	}
}

// Test that ToOutput works with all builtin input types

func TestToOutputArchive(t *testing.T) {
	out := ToOutput(NewFileArchive("foo.zip"))
	_, ok := out.(ArchiveInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(ArchiveInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputArchiveArray(t *testing.T) {
	out := ToOutput(ArchiveArray{NewFileArchive("foo.zip")})
	_, ok := out.(ArchiveArrayInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(ArchiveArrayInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputArchiveMap(t *testing.T) {
	out := ToOutput(ArchiveMap{"baz": NewFileArchive("foo.zip")})
	_, ok := out.(ArchiveMapInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(ArchiveMapInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAsset(t *testing.T) {
	out := ToOutput(NewFileAsset("foo.txt"))
	_, ok := out.(AssetInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetArray(t *testing.T) {
	out := ToOutput(AssetArray{NewFileAsset("foo.txt")})
	_, ok := out.(AssetArrayInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetArrayInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetMap(t *testing.T) {
	out := ToOutput(AssetMap{"baz": NewFileAsset("foo.txt")})
	_, ok := out.(AssetMapInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetMapInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetOrArchive(t *testing.T) {
	out := ToOutput(NewFileArchive("foo.zip"))
	_, ok := out.(AssetOrArchiveInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetOrArchiveInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetOrArchiveArray(t *testing.T) {
	out := ToOutput(AssetOrArchiveArray{NewFileArchive("foo.zip")})
	_, ok := out.(AssetOrArchiveArrayInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetOrArchiveArrayInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetOrArchiveMap(t *testing.T) {
	out := ToOutput(AssetOrArchiveMap{"baz": NewFileArchive("foo.zip")})
	_, ok := out.(AssetOrArchiveMapInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetOrArchiveMapInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputBool(t *testing.T) {
	out := ToOutput(Bool(true))
	_, ok := out.(BoolInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(BoolInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputBoolPtr(t *testing.T) {
	out := ToOutput(BoolPtr(bool(Bool(true))))
	_, ok := out.(BoolPtrInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(BoolPtrInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputBoolArray(t *testing.T) {
	out := ToOutput(BoolArray{Bool(true)})
	_, ok := out.(BoolArrayInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(BoolArrayInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputBoolMap(t *testing.T) {
	out := ToOutput(BoolMap{"baz": Bool(true)})
	_, ok := out.(BoolMapInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(BoolMapInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat32(t *testing.T) {
	out := ToOutput(Float32(1.3))
	_, ok := out.(Float32Input)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float32Input)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat32Ptr(t *testing.T) {
	out := ToOutput(Float32Ptr(float32(Float32(1.3))))
	_, ok := out.(Float32PtrInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float32PtrInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat32Array(t *testing.T) {
	out := ToOutput(Float32Array{Float32(1.3)})
	_, ok := out.(Float32ArrayInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float32ArrayInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat32Map(t *testing.T) {
	out := ToOutput(Float32Map{"baz": Float32(1.3)})
	_, ok := out.(Float32MapInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float32MapInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat64(t *testing.T) {
	out := ToOutput(Float64(999.9))
	_, ok := out.(Float64Input)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float64Input)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat64Ptr(t *testing.T) {
	out := ToOutput(Float64Ptr(float64(Float64(999.9))))
	_, ok := out.(Float64PtrInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float64PtrInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat64Array(t *testing.T) {
	out := ToOutput(Float64Array{Float64(999.9)})
	_, ok := out.(Float64ArrayInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float64ArrayInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat64Map(t *testing.T) {
	out := ToOutput(Float64Map{"baz": Float64(999.9)})
	_, ok := out.(Float64MapInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float64MapInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputID(t *testing.T) {
	out := ToOutput(ID("foo"))
	_, ok := out.(IDInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IDInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIDPtr(t *testing.T) {
	out := ToOutput(IDPtr(ID(ID("foo"))))
	_, ok := out.(IDPtrInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IDPtrInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIDArray(t *testing.T) {
	out := ToOutput(IDArray{ID("foo")})
	_, ok := out.(IDArrayInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IDArrayInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIDMap(t *testing.T) {
	out := ToOutput(IDMap{"baz": ID("foo")})
	_, ok := out.(IDMapInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IDMapInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputArray(t *testing.T) {
	out := ToOutput(Array{String("any")})
	_, ok := out.(ArrayInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(ArrayInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputMap(t *testing.T) {
	out := ToOutput(Map{"baz": String("any")})
	_, ok := out.(MapInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(MapInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt(t *testing.T) {
	out := ToOutput(Int(42))
	_, ok := out.(IntInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IntInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIntPtr(t *testing.T) {
	out := ToOutput(IntPtr(int(Int(42))))
	_, ok := out.(IntPtrInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IntPtrInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIntArray(t *testing.T) {
	out := ToOutput(IntArray{Int(42)})
	_, ok := out.(IntArrayInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IntArrayInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIntMap(t *testing.T) {
	out := ToOutput(IntMap{"baz": Int(42)})
	_, ok := out.(IntMapInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IntMapInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt16(t *testing.T) {
	out := ToOutput(Int16(33))
	_, ok := out.(Int16Input)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int16Input)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt16Ptr(t *testing.T) {
	out := ToOutput(Int16Ptr(int16(Int16(33))))
	_, ok := out.(Int16PtrInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int16PtrInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt16Array(t *testing.T) {
	out := ToOutput(Int16Array{Int16(33)})
	_, ok := out.(Int16ArrayInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int16ArrayInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt16Map(t *testing.T) {
	out := ToOutput(Int16Map{"baz": Int16(33)})
	_, ok := out.(Int16MapInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int16MapInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt32(t *testing.T) {
	out := ToOutput(Int32(24))
	_, ok := out.(Int32Input)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int32Input)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt32Ptr(t *testing.T) {
	out := ToOutput(Int32Ptr(int32(Int32(24))))
	_, ok := out.(Int32PtrInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int32PtrInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt32Array(t *testing.T) {
	out := ToOutput(Int32Array{Int32(24)})
	_, ok := out.(Int32ArrayInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int32ArrayInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt32Map(t *testing.T) {
	out := ToOutput(Int32Map{"baz": Int32(24)})
	_, ok := out.(Int32MapInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int32MapInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt64(t *testing.T) {
	out := ToOutput(Int64(15))
	_, ok := out.(Int64Input)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int64Input)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt64Ptr(t *testing.T) {
	out := ToOutput(Int64Ptr(int64(Int64(15))))
	_, ok := out.(Int64PtrInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int64PtrInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt64Array(t *testing.T) {
	out := ToOutput(Int64Array{Int64(15)})
	_, ok := out.(Int64ArrayInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int64ArrayInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt64Map(t *testing.T) {
	out := ToOutput(Int64Map{"baz": Int64(15)})
	_, ok := out.(Int64MapInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int64MapInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt8(t *testing.T) {
	out := ToOutput(Int8(6))
	_, ok := out.(Int8Input)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int8Input)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt8Ptr(t *testing.T) {
	out := ToOutput(Int8Ptr(int8(Int8(6))))
	_, ok := out.(Int8PtrInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int8PtrInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt8Array(t *testing.T) {
	out := ToOutput(Int8Array{Int8(6)})
	_, ok := out.(Int8ArrayInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int8ArrayInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt8Map(t *testing.T) {
	out := ToOutput(Int8Map{"baz": Int8(6)})
	_, ok := out.(Int8MapInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int8MapInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputString(t *testing.T) {
	out := ToOutput(String("foo"))
	_, ok := out.(StringInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(StringInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputStringPtr(t *testing.T) {
	out := ToOutput(StringPtr(string(String("foo"))))
	_, ok := out.(StringPtrInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(StringPtrInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputStringArray(t *testing.T) {
	out := ToOutput(StringArray{String("foo")})
	_, ok := out.(StringArrayInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(StringArrayInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputStringMap(t *testing.T) {
	out := ToOutput(StringMap{"baz": String("foo")})
	_, ok := out.(StringMapInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(StringMapInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputURN(t *testing.T) {
	out := ToOutput(URN("foo"))
	_, ok := out.(URNInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(URNInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputURNPtr(t *testing.T) {
	out := ToOutput(URNPtr(URN(URN("foo"))))
	_, ok := out.(URNPtrInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(URNPtrInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputURNArray(t *testing.T) {
	out := ToOutput(URNArray{URN("foo")})
	_, ok := out.(URNArrayInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(URNArrayInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputURNMap(t *testing.T) {
	out := ToOutput(URNMap{"baz": URN("foo")})
	_, ok := out.(URNMapInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(URNMapInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint(t *testing.T) {
	out := ToOutput(Uint(42))
	_, ok := out.(UintInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(UintInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUintPtr(t *testing.T) {
	out := ToOutput(UintPtr(uint(Uint(42))))
	_, ok := out.(UintPtrInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(UintPtrInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUintArray(t *testing.T) {
	out := ToOutput(UintArray{Uint(42)})
	_, ok := out.(UintArrayInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(UintArrayInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUintMap(t *testing.T) {
	out := ToOutput(UintMap{"baz": Uint(42)})
	_, ok := out.(UintMapInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(UintMapInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint16(t *testing.T) {
	out := ToOutput(Uint16(33))
	_, ok := out.(Uint16Input)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint16Input)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint16Ptr(t *testing.T) {
	out := ToOutput(Uint16Ptr(uint16(Uint16(33))))
	_, ok := out.(Uint16PtrInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint16PtrInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint16Array(t *testing.T) {
	out := ToOutput(Uint16Array{Uint16(33)})
	_, ok := out.(Uint16ArrayInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint16ArrayInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint16Map(t *testing.T) {
	out := ToOutput(Uint16Map{"baz": Uint16(33)})
	_, ok := out.(Uint16MapInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint16MapInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint32(t *testing.T) {
	out := ToOutput(Uint32(24))
	_, ok := out.(Uint32Input)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint32Input)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint32Ptr(t *testing.T) {
	out := ToOutput(Uint32Ptr(uint32(Uint32(24))))
	_, ok := out.(Uint32PtrInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint32PtrInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint32Array(t *testing.T) {
	out := ToOutput(Uint32Array{Uint32(24)})
	_, ok := out.(Uint32ArrayInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint32ArrayInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint32Map(t *testing.T) {
	out := ToOutput(Uint32Map{"baz": Uint32(24)})
	_, ok := out.(Uint32MapInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint32MapInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint64(t *testing.T) {
	out := ToOutput(Uint64(15))
	_, ok := out.(Uint64Input)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint64Input)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint64Ptr(t *testing.T) {
	out := ToOutput(Uint64Ptr(uint64(Uint64(15))))
	_, ok := out.(Uint64PtrInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint64PtrInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint64Array(t *testing.T) {
	out := ToOutput(Uint64Array{Uint64(15)})
	_, ok := out.(Uint64ArrayInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint64ArrayInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint64Map(t *testing.T) {
	out := ToOutput(Uint64Map{"baz": Uint64(15)})
	_, ok := out.(Uint64MapInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint64MapInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint8(t *testing.T) {
	out := ToOutput(Uint8(6))
	_, ok := out.(Uint8Input)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint8Input)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint8Ptr(t *testing.T) {
	out := ToOutput(Uint8Ptr(uint8(Uint8(6))))
	_, ok := out.(Uint8PtrInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint8PtrInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint8Array(t *testing.T) {
	out := ToOutput(Uint8Array{Uint8(6)})
	_, ok := out.(Uint8ArrayInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint8ArrayInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint8Map(t *testing.T) {
	out := ToOutput(Uint8Map{"baz": Uint8(6)})
	_, ok := out.(Uint8MapInput)
	assert.True(t, ok)

	_, known, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint8MapInput)
	assert.True(t, ok)

	_, known, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}
