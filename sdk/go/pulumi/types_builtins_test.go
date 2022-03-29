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
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOutputApply(t *testing.T) {
	// Test that resolved outputs lead to applies being run.
	{
		out := newIntOutput()
		go func() { out.resolve(42, true, false, nil) }()
		var ranApp bool
		app := out.ApplyT(func(v int) (interface{}, error) {
			ranApp = true
			return v + 1, nil
		})
		v, known, _, _, err := await(app)
		assert.True(t, ranApp)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, v, 43)
	}
	// Test that resolved, but unknown outputs, skip the running of applies.
	{
		out := newIntOutput()
		go func() { out.resolve(42, false, false, nil) }()
		var ranApp bool
		app := out.ApplyT(func(v int) (interface{}, error) {
			ranApp = true
			return v + 1, nil
		})
		_, known, _, _, err := await(app)
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
		v, _, _, _, err := await(app)
		assert.False(t, ranApp)
		assert.NotNil(t, err)
		assert.Nil(t, v)
	}
	// Test that an an apply that returns an output returns the resolution of that output, not the output itself.
	{
		out := newIntOutput()
		go func() { out.resolve(42, true, false, nil) }()
		var ranApp bool
		app := out.ApplyT(func(v int) (interface{}, error) {
			other, resolveOther, _ := NewOutput()
			go func() { resolveOther(v + 1) }()
			ranApp = true
			return other, nil
		})
		v, known, _, _, err := await(app)
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
		v, known, _, _, err = await(app)
		assert.True(t, ranApp)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, v, 44)
	}
	// Test that an an apply that reject an output returns the rejection of that output, not the output itself.
	{
		out := newIntOutput()
		go func() { out.resolve(42, true, false, nil) }()
		var ranApp bool
		app := out.ApplyT(func(v int) (interface{}, error) {
			other, _, rejectOther := NewOutput()
			go func() { rejectOther(errors.New("boom")) }()
			ranApp = true
			return other, nil
		})
		v, _, _, _, err := await(app)
		assert.True(t, ranApp)
		assert.NotNil(t, err)
		assert.Nil(t, v)

		app = out.ApplyT(func(v int) (interface{}, error) {
			other, _, rejectOther := NewOutput()
			go func() { rejectOther(errors.New("boom")) }()
			ranApp = true
			return other, nil
		})
		v, _, _, _, err = await(app)
		assert.True(t, ranApp)
		assert.NotNil(t, err)
		assert.Nil(t, v)
	}
	// Test that applies return appropriate concrete implementations of Output based on the callback type
	{
		out := newIntOutput()
		go func() { out.resolve(42, true, false, nil) }()

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

		t.Run("ApplyT::ArchiveArrayMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string][]Archive { return *new(map[string][]Archive) }).(ArchiveArrayMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::ArchiveMapArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []map[string]Archive { return *new([]map[string]Archive) }).(ArchiveMapArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::ArchiveMapMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]map[string]Archive { return *new(map[string]map[string]Archive) }).(ArchiveMapMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::ArchiveArrayArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) [][]Archive { return *new([][]Archive) }).(ArchiveArrayArrayOutput)
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

		t.Run("ApplyT::AssetArrayMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string][]Asset { return *new(map[string][]Asset) }).(AssetArrayMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::AssetMapArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []map[string]Asset { return *new([]map[string]Asset) }).(AssetMapArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::AssetMapMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]map[string]Asset { return *new(map[string]map[string]Asset) }).(AssetMapMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::AssetArrayArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) [][]Asset { return *new([][]Asset) }).(AssetArrayArrayOutput)
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

		t.Run("ApplyT::AssetOrArchiveArrayMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string][]AssetOrArchive { return *new(map[string][]AssetOrArchive) }).(AssetOrArchiveArrayMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::AssetOrArchiveMapArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []map[string]AssetOrArchive { return *new([]map[string]AssetOrArchive) }).(AssetOrArchiveMapArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::AssetOrArchiveMapMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]map[string]AssetOrArchive { return *new(map[string]map[string]AssetOrArchive) }).(AssetOrArchiveMapMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::AssetOrArchiveArrayArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) [][]AssetOrArchive { return *new([][]AssetOrArchive) }).(AssetOrArchiveArrayArrayOutput)
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

		t.Run("ApplyT::BoolArrayMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string][]bool { return *new(map[string][]bool) }).(BoolArrayMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::BoolMapArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []map[string]bool { return *new([]map[string]bool) }).(BoolMapArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::BoolMapMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]map[string]bool { return *new(map[string]map[string]bool) }).(BoolMapMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::BoolArrayArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) [][]bool { return *new([][]bool) }).(BoolArrayArrayOutput)
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

		t.Run("ApplyT::Float64ArrayMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string][]float64 { return *new(map[string][]float64) }).(Float64ArrayMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Float64MapArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []map[string]float64 { return *new([]map[string]float64) }).(Float64MapArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Float64MapMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]map[string]float64 { return *new(map[string]map[string]float64) }).(Float64MapMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Float64ArrayArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) [][]float64 { return *new([][]float64) }).(Float64ArrayArrayOutput)
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

		t.Run("ApplyT::IDArrayMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string][]ID { return *new(map[string][]ID) }).(IDArrayMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::IDMapArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []map[string]ID { return *new([]map[string]ID) }).(IDMapArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::IDMapMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]map[string]ID { return *new(map[string]map[string]ID) }).(IDMapMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::IDArrayArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) [][]ID { return *new([][]ID) }).(IDArrayArrayOutput)
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

		t.Run("ApplyT::ArrayMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string][]interface{} { return *new(map[string][]interface{}) }).(ArrayMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::MapArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []map[string]interface{} { return *new([]map[string]interface{}) }).(MapArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::MapMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]map[string]interface{} { return *new(map[string]map[string]interface{}) }).(MapMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::ArrayArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) [][]interface{} { return *new([][]interface{}) }).(ArrayArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::ArrayArrayMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string][][]interface{} { return *new(map[string][][]interface{}) }).(ArrayArrayMapOutput)
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

		t.Run("ApplyT::IntArrayMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string][]int { return *new(map[string][]int) }).(IntArrayMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::IntMapArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []map[string]int { return *new([]map[string]int) }).(IntMapArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::IntMapMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]map[string]int { return *new(map[string]map[string]int) }).(IntMapMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::IntArrayArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) [][]int { return *new([][]int) }).(IntArrayArrayOutput)
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

		t.Run("ApplyT::StringArrayMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string][]string { return *new(map[string][]string) }).(StringArrayMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::StringMapArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []map[string]string { return *new([]map[string]string) }).(StringMapArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::StringMapMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]map[string]string { return *new(map[string]map[string]string) }).(StringMapMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::StringArrayArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) [][]string { return *new([][]string) }).(StringArrayArrayOutput)
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

		t.Run("ApplyT::URNArrayMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string][]URN { return *new(map[string][]URN) }).(URNArrayMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::URNMapArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []map[string]URN { return *new([]map[string]URN) }).(URNMapArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::URNMapMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]map[string]URN { return *new(map[string]map[string]URN) }).(URNMapMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::URNArrayArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) [][]URN { return *new([][]URN) }).(URNArrayArrayOutput)
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
		go func() { out.resolve(42, true, false, nil) }()

		out2 := StringOutput{newOutputState(nil, reflect.TypeOf(""))}
		go func() { out2.resolve("hello", true, false, nil) }()

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

		res5 := All(res3, res4).ApplyT(func(v interface{}) (interface{}, error) {
			vs := v.([]interface{})
			res3 := vs[0].(string)
			res4 := vs[1].(*myStructType)
			return fmt.Sprintf("%v;%v;%v", res3, res4.foo, res4.bar), nil
		})

		_, ok := res.(StringArrayOutput)
		assert.True(t, ok)

		v, known, _, _, err := await(res)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, []string{"qux", "zed"}, v)

		_, ok = res2.(StringArrayOutput)
		assert.True(t, ok)

		v, known, _, _, err = await(res2)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, []string{"foo", "bar"}, v)

		_, ok = res3.(StringOutput)
		assert.True(t, ok)

		v, known, _, _, err = await(res3)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, "foo,bar,qux,zed", v)

		_, ok = res4.(AnyOutput)
		assert.True(t, ok)

		v, known, _, _, err = await(res4)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, &myStructType{foo: 42, bar: "hello"}, v)

		v, known, _, _, err = await(res5)
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

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(ArchiveInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputArchiveArray(t *testing.T) {
	out := ToOutput(ArchiveArray{NewFileArchive("foo.zip")})
	_, ok := out.(ArchiveArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(ArchiveArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputArchiveMap(t *testing.T) {
	out := ToOutput(ArchiveMap{"baz": NewFileArchive("foo.zip")})
	_, ok := out.(ArchiveMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(ArchiveMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputArchiveArrayMap(t *testing.T) {
	out := ToOutput(ArchiveArrayMap{"baz": ArchiveArray{NewFileArchive("foo.zip")}})
	_, ok := out.(ArchiveArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(ArchiveArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputArchiveMapArray(t *testing.T) {
	out := ToOutput(ArchiveMapArray{ArchiveMap{"baz": NewFileArchive("foo.zip")}})
	_, ok := out.(ArchiveMapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(ArchiveMapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputArchiveMapMap(t *testing.T) {
	out := ToOutput(ArchiveMapMap{"baz": ArchiveMap{"baz": NewFileArchive("foo.zip")}})
	_, ok := out.(ArchiveMapMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(ArchiveMapMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputArchiveArrayArray(t *testing.T) {
	out := ToOutput(ArchiveArrayArray{ArchiveArray{NewFileArchive("foo.zip")}})
	_, ok := out.(ArchiveArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(ArchiveArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAsset(t *testing.T) {
	out := ToOutput(NewFileAsset("foo.txt"))
	_, ok := out.(AssetInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetArray(t *testing.T) {
	out := ToOutput(AssetArray{NewFileAsset("foo.txt")})
	_, ok := out.(AssetArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetMap(t *testing.T) {
	out := ToOutput(AssetMap{"baz": NewFileAsset("foo.txt")})
	_, ok := out.(AssetMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetArrayMap(t *testing.T) {
	out := ToOutput(AssetArrayMap{"baz": AssetArray{NewFileAsset("foo.txt")}})
	_, ok := out.(AssetArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetMapArray(t *testing.T) {
	out := ToOutput(AssetMapArray{AssetMap{"baz": NewFileAsset("foo.txt")}})
	_, ok := out.(AssetMapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetMapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetMapMap(t *testing.T) {
	out := ToOutput(AssetMapMap{"baz": AssetMap{"baz": NewFileAsset("foo.txt")}})
	_, ok := out.(AssetMapMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetMapMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetArrayArray(t *testing.T) {
	out := ToOutput(AssetArrayArray{AssetArray{NewFileAsset("foo.txt")}})
	_, ok := out.(AssetArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetOrArchive(t *testing.T) {
	out := ToOutput(NewFileArchive("foo.zip"))
	_, ok := out.(AssetOrArchiveInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetOrArchiveInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetOrArchiveArray(t *testing.T) {
	out := ToOutput(AssetOrArchiveArray{NewFileArchive("foo.zip")})
	_, ok := out.(AssetOrArchiveArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetOrArchiveArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetOrArchiveMap(t *testing.T) {
	out := ToOutput(AssetOrArchiveMap{"baz": NewFileArchive("foo.zip")})
	_, ok := out.(AssetOrArchiveMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetOrArchiveMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetOrArchiveArrayMap(t *testing.T) {
	out := ToOutput(AssetOrArchiveArrayMap{"baz": AssetOrArchiveArray{NewFileArchive("foo.zip")}})
	_, ok := out.(AssetOrArchiveArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetOrArchiveArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetOrArchiveMapArray(t *testing.T) {
	out := ToOutput(AssetOrArchiveMapArray{AssetOrArchiveMap{"baz": NewFileArchive("foo.zip")}})
	_, ok := out.(AssetOrArchiveMapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetOrArchiveMapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetOrArchiveMapMap(t *testing.T) {
	out := ToOutput(AssetOrArchiveMapMap{"baz": AssetOrArchiveMap{"baz": NewFileArchive("foo.zip")}})
	_, ok := out.(AssetOrArchiveMapMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetOrArchiveMapMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetOrArchiveArrayArray(t *testing.T) {
	out := ToOutput(AssetOrArchiveArrayArray{AssetOrArchiveArray{NewFileArchive("foo.zip")}})
	_, ok := out.(AssetOrArchiveArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetOrArchiveArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputBool(t *testing.T) {
	out := ToOutput(Bool(true))
	_, ok := out.(BoolInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(BoolInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputBoolPtr(t *testing.T) {
	out := ToOutput(BoolPtr(bool(Bool(true))))
	_, ok := out.(BoolPtrInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(BoolPtrInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputBoolArray(t *testing.T) {
	out := ToOutput(BoolArray{Bool(true)})
	_, ok := out.(BoolArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(BoolArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputBoolMap(t *testing.T) {
	out := ToOutput(BoolMap{"baz": Bool(true)})
	_, ok := out.(BoolMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(BoolMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputBoolArrayMap(t *testing.T) {
	out := ToOutput(BoolArrayMap{"baz": BoolArray{Bool(true)}})
	_, ok := out.(BoolArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(BoolArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputBoolMapArray(t *testing.T) {
	out := ToOutput(BoolMapArray{BoolMap{"baz": Bool(true)}})
	_, ok := out.(BoolMapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(BoolMapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputBoolMapMap(t *testing.T) {
	out := ToOutput(BoolMapMap{"baz": BoolMap{"baz": Bool(true)}})
	_, ok := out.(BoolMapMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(BoolMapMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputBoolArrayArray(t *testing.T) {
	out := ToOutput(BoolArrayArray{BoolArray{Bool(true)}})
	_, ok := out.(BoolArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(BoolArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat64(t *testing.T) {
	out := ToOutput(Float64(999.9))
	_, ok := out.(Float64Input)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float64Input)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat64Ptr(t *testing.T) {
	out := ToOutput(Float64Ptr(float64(Float64(999.9))))
	_, ok := out.(Float64PtrInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float64PtrInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat64Array(t *testing.T) {
	out := ToOutput(Float64Array{Float64(999.9)})
	_, ok := out.(Float64ArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float64ArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat64Map(t *testing.T) {
	out := ToOutput(Float64Map{"baz": Float64(999.9)})
	_, ok := out.(Float64MapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float64MapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat64ArrayMap(t *testing.T) {
	out := ToOutput(Float64ArrayMap{"baz": Float64Array{Float64(999.9)}})
	_, ok := out.(Float64ArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float64ArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat64MapArray(t *testing.T) {
	out := ToOutput(Float64MapArray{Float64Map{"baz": Float64(999.9)}})
	_, ok := out.(Float64MapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float64MapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat64MapMap(t *testing.T) {
	out := ToOutput(Float64MapMap{"baz": Float64Map{"baz": Float64(999.9)}})
	_, ok := out.(Float64MapMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float64MapMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat64ArrayArray(t *testing.T) {
	out := ToOutput(Float64ArrayArray{Float64Array{Float64(999.9)}})
	_, ok := out.(Float64ArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float64ArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputID(t *testing.T) {
	out := ToOutput(ID("foo"))
	_, ok := out.(IDInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IDInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIDPtr(t *testing.T) {
	out := ToOutput(IDPtr(ID(ID("foo"))))
	_, ok := out.(IDPtrInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IDPtrInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIDArray(t *testing.T) {
	out := ToOutput(IDArray{ID("foo")})
	_, ok := out.(IDArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IDArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIDMap(t *testing.T) {
	out := ToOutput(IDMap{"baz": ID("foo")})
	_, ok := out.(IDMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IDMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIDArrayMap(t *testing.T) {
	out := ToOutput(IDArrayMap{"baz": IDArray{ID("foo")}})
	_, ok := out.(IDArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IDArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIDMapArray(t *testing.T) {
	out := ToOutput(IDMapArray{IDMap{"baz": ID("foo")}})
	_, ok := out.(IDMapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IDMapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIDMapMap(t *testing.T) {
	out := ToOutput(IDMapMap{"baz": IDMap{"baz": ID("foo")}})
	_, ok := out.(IDMapMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IDMapMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIDArrayArray(t *testing.T) {
	out := ToOutput(IDArrayArray{IDArray{ID("foo")}})
	_, ok := out.(IDArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IDArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputArray(t *testing.T) {
	out := ToOutput(Array{String("any")})
	_, ok := out.(ArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(ArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputMap(t *testing.T) {
	out := ToOutput(Map{"baz": String("any")})
	_, ok := out.(MapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(MapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputArrayMap(t *testing.T) {
	out := ToOutput(ArrayMap{"baz": Array{String("any")}})
	_, ok := out.(ArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(ArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputMapArray(t *testing.T) {
	out := ToOutput(MapArray{Map{"baz": String("any")}})
	_, ok := out.(MapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(MapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputMapMap(t *testing.T) {
	out := ToOutput(MapMap{"baz": Map{"baz": String("any")}})
	_, ok := out.(MapMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(MapMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputArrayArray(t *testing.T) {
	out := ToOutput(ArrayArray{Array{String("any")}})
	_, ok := out.(ArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(ArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputArrayArrayMap(t *testing.T) {
	out := ToOutput(ArrayArrayMap{"baz": ArrayArray{Array{String("any")}}})
	_, ok := out.(ArrayArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(ArrayArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt(t *testing.T) {
	out := ToOutput(Int(42))
	_, ok := out.(IntInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IntInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIntPtr(t *testing.T) {
	out := ToOutput(IntPtr(int(Int(42))))
	_, ok := out.(IntPtrInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IntPtrInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIntArray(t *testing.T) {
	out := ToOutput(IntArray{Int(42)})
	_, ok := out.(IntArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IntArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIntMap(t *testing.T) {
	out := ToOutput(IntMap{"baz": Int(42)})
	_, ok := out.(IntMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IntMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIntArrayMap(t *testing.T) {
	out := ToOutput(IntArrayMap{"baz": IntArray{Int(42)}})
	_, ok := out.(IntArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IntArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIntMapArray(t *testing.T) {
	out := ToOutput(IntMapArray{IntMap{"baz": Int(42)}})
	_, ok := out.(IntMapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IntMapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIntMapMap(t *testing.T) {
	out := ToOutput(IntMapMap{"baz": IntMap{"baz": Int(42)}})
	_, ok := out.(IntMapMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IntMapMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIntArrayArray(t *testing.T) {
	out := ToOutput(IntArrayArray{IntArray{Int(42)}})
	_, ok := out.(IntArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IntArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputString(t *testing.T) {
	out := ToOutput(String("foo"))
	_, ok := out.(StringInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(StringInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputStringPtr(t *testing.T) {
	out := ToOutput(StringPtr(string(String("foo"))))
	_, ok := out.(StringPtrInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(StringPtrInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputStringArray(t *testing.T) {
	out := ToOutput(StringArray{String("foo")})
	_, ok := out.(StringArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(StringArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputStringMap(t *testing.T) {
	out := ToOutput(StringMap{"baz": String("foo")})
	_, ok := out.(StringMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(StringMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputStringArrayMap(t *testing.T) {
	out := ToOutput(StringArrayMap{"baz": StringArray{String("foo")}})
	_, ok := out.(StringArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(StringArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputStringMapArray(t *testing.T) {
	out := ToOutput(StringMapArray{StringMap{"baz": String("foo")}})
	_, ok := out.(StringMapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(StringMapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputStringMapMap(t *testing.T) {
	out := ToOutput(StringMapMap{"baz": StringMap{"baz": String("foo")}})
	_, ok := out.(StringMapMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(StringMapMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputStringArrayArray(t *testing.T) {
	out := ToOutput(StringArrayArray{StringArray{String("foo")}})
	_, ok := out.(StringArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(StringArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputURN(t *testing.T) {
	out := ToOutput(URN("foo"))
	_, ok := out.(URNInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(URNInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputURNPtr(t *testing.T) {
	out := ToOutput(URNPtr(URN(URN("foo"))))
	_, ok := out.(URNPtrInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(URNPtrInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputURNArray(t *testing.T) {
	out := ToOutput(URNArray{URN("foo")})
	_, ok := out.(URNArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(URNArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputURNMap(t *testing.T) {
	out := ToOutput(URNMap{"baz": URN("foo")})
	_, ok := out.(URNMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(URNMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputURNArrayMap(t *testing.T) {
	out := ToOutput(URNArrayMap{"baz": URNArray{URN("foo")}})
	_, ok := out.(URNArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(URNArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputURNMapArray(t *testing.T) {
	out := ToOutput(URNMapArray{URNMap{"baz": URN("foo")}})
	_, ok := out.(URNMapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(URNMapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputURNMapMap(t *testing.T) {
	out := ToOutput(URNMapMap{"baz": URNMap{"baz": URN("foo")}})
	_, ok := out.(URNMapMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(URNMapMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputURNArrayArray(t *testing.T) {
	out := ToOutput(URNArrayArray{URNArray{URN("foo")}})
	_, ok := out.(URNArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(URNArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

// Test that type-specific ToOutput methods work with all builtin input and output types

func TestToArchiveOutput(t *testing.T) {
	in := ArchiveInput(NewFileArchive("foo.zip"))

	out := in.ToArchiveOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToArchiveOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToArchiveArrayOutput(t *testing.T) {
	in := ArchiveArrayInput(ArchiveArray{NewFileArchive("foo.zip")})

	out := in.ToArchiveArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToArchiveArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToArchiveMapOutput(t *testing.T) {
	in := ArchiveMapInput(ArchiveMap{"baz": NewFileArchive("foo.zip")})

	out := in.ToArchiveMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToArchiveMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToArchiveArrayMapOutput(t *testing.T) {
	in := ArchiveArrayMapInput(ArchiveArrayMap{"baz": ArchiveArray{NewFileArchive("foo.zip")}})

	out := in.ToArchiveArrayMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveArrayMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToArchiveArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToArchiveMapArrayOutput(t *testing.T) {
	in := ArchiveMapArrayInput(ArchiveMapArray{ArchiveMap{"baz": NewFileArchive("foo.zip")}})

	out := in.ToArchiveMapArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveMapArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToArchiveMapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveMapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToArchiveMapMapOutput(t *testing.T) {
	in := ArchiveMapMapInput(ArchiveMapMap{"baz": ArchiveMap{"baz": NewFileArchive("foo.zip")}})

	out := in.ToArchiveMapMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveMapMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToArchiveMapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveMapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToArchiveArrayArrayOutput(t *testing.T) {
	in := ArchiveArrayArrayInput(ArchiveArrayArray{ArchiveArray{NewFileArchive("foo.zip")}})

	out := in.ToArchiveArrayArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveArrayArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToArchiveArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToAssetOutput(t *testing.T) {
	in := AssetInput(NewFileAsset("foo.txt"))

	out := in.ToAssetOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToAssetOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToAssetArrayOutput(t *testing.T) {
	in := AssetArrayInput(AssetArray{NewFileAsset("foo.txt")})

	out := in.ToAssetArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToAssetArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToAssetMapOutput(t *testing.T) {
	in := AssetMapInput(AssetMap{"baz": NewFileAsset("foo.txt")})

	out := in.ToAssetMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToAssetMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToAssetArrayMapOutput(t *testing.T) {
	in := AssetArrayMapInput(AssetArrayMap{"baz": AssetArray{NewFileAsset("foo.txt")}})

	out := in.ToAssetArrayMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetArrayMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToAssetArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToAssetMapArrayOutput(t *testing.T) {
	in := AssetMapArrayInput(AssetMapArray{AssetMap{"baz": NewFileAsset("foo.txt")}})

	out := in.ToAssetMapArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetMapArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToAssetMapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetMapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToAssetMapMapOutput(t *testing.T) {
	in := AssetMapMapInput(AssetMapMap{"baz": AssetMap{"baz": NewFileAsset("foo.txt")}})

	out := in.ToAssetMapMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetMapMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToAssetMapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetMapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToAssetArrayArrayOutput(t *testing.T) {
	in := AssetArrayArrayInput(AssetArrayArray{AssetArray{NewFileAsset("foo.txt")}})

	out := in.ToAssetArrayArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetArrayArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToAssetArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToAssetOrArchiveOutput(t *testing.T) {
	in := AssetOrArchiveInput(NewFileArchive("foo.zip"))

	out := in.ToAssetOrArchiveOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToAssetOrArchiveOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToAssetOrArchiveArrayOutput(t *testing.T) {
	in := AssetOrArchiveArrayInput(AssetOrArchiveArray{NewFileArchive("foo.zip")})

	out := in.ToAssetOrArchiveArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToAssetOrArchiveArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToAssetOrArchiveMapOutput(t *testing.T) {
	in := AssetOrArchiveMapInput(AssetOrArchiveMap{"baz": NewFileArchive("foo.zip")})

	out := in.ToAssetOrArchiveMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToAssetOrArchiveMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToAssetOrArchiveArrayMapOutput(t *testing.T) {
	in := AssetOrArchiveArrayMapInput(AssetOrArchiveArrayMap{"baz": AssetOrArchiveArray{NewFileArchive("foo.zip")}})

	out := in.ToAssetOrArchiveArrayMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveArrayMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToAssetOrArchiveArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToAssetOrArchiveMapArrayOutput(t *testing.T) {
	in := AssetOrArchiveMapArrayInput(AssetOrArchiveMapArray{AssetOrArchiveMap{"baz": NewFileArchive("foo.zip")}})

	out := in.ToAssetOrArchiveMapArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveMapArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToAssetOrArchiveMapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveMapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToAssetOrArchiveMapMapOutput(t *testing.T) {
	in := AssetOrArchiveMapMapInput(AssetOrArchiveMapMap{"baz": AssetOrArchiveMap{"baz": NewFileArchive("foo.zip")}})

	out := in.ToAssetOrArchiveMapMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveMapMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToAssetOrArchiveMapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveMapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToAssetOrArchiveArrayArrayOutput(t *testing.T) {
	in := AssetOrArchiveArrayArrayInput(AssetOrArchiveArrayArray{AssetOrArchiveArray{NewFileArchive("foo.zip")}})

	out := in.ToAssetOrArchiveArrayArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveArrayArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToAssetOrArchiveArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToBoolOutput(t *testing.T) {
	in := BoolInput(Bool(true))

	out := in.ToBoolOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToBoolOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToBoolPtrOutput(t *testing.T) {
	in := BoolPtrInput(BoolPtr(bool(Bool(true))))

	out := in.ToBoolPtrOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolPtrOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToBoolPtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolPtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToBoolArrayOutput(t *testing.T) {
	in := BoolArrayInput(BoolArray{Bool(true)})

	out := in.ToBoolArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToBoolArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToBoolMapOutput(t *testing.T) {
	in := BoolMapInput(BoolMap{"baz": Bool(true)})

	out := in.ToBoolMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToBoolMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToBoolArrayMapOutput(t *testing.T) {
	in := BoolArrayMapInput(BoolArrayMap{"baz": BoolArray{Bool(true)}})

	out := in.ToBoolArrayMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolArrayMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToBoolArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToBoolMapArrayOutput(t *testing.T) {
	in := BoolMapArrayInput(BoolMapArray{BoolMap{"baz": Bool(true)}})

	out := in.ToBoolMapArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolMapArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToBoolMapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolMapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToBoolMapMapOutput(t *testing.T) {
	in := BoolMapMapInput(BoolMapMap{"baz": BoolMap{"baz": Bool(true)}})

	out := in.ToBoolMapMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolMapMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToBoolMapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolMapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToBoolArrayArrayOutput(t *testing.T) {
	in := BoolArrayArrayInput(BoolArrayArray{BoolArray{Bool(true)}})

	out := in.ToBoolArrayArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolArrayArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToBoolArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat64Output(t *testing.T) {
	in := Float64Input(Float64(999.9))

	out := in.ToFloat64Output()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64Output()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat64OutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64OutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat64PtrOutput(t *testing.T) {
	in := Float64PtrInput(Float64Ptr(float64(Float64(999.9))))

	out := in.ToFloat64PtrOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64PtrOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat64PtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64PtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat64ArrayOutput(t *testing.T) {
	in := Float64ArrayInput(Float64Array{Float64(999.9)})

	out := in.ToFloat64ArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64ArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat64ArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64ArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat64MapOutput(t *testing.T) {
	in := Float64MapInput(Float64Map{"baz": Float64(999.9)})

	out := in.ToFloat64MapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64MapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat64MapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64MapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat64ArrayMapOutput(t *testing.T) {
	in := Float64ArrayMapInput(Float64ArrayMap{"baz": Float64Array{Float64(999.9)}})

	out := in.ToFloat64ArrayMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64ArrayMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat64ArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64ArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat64MapArrayOutput(t *testing.T) {
	in := Float64MapArrayInput(Float64MapArray{Float64Map{"baz": Float64(999.9)}})

	out := in.ToFloat64MapArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64MapArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat64MapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64MapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat64MapMapOutput(t *testing.T) {
	in := Float64MapMapInput(Float64MapMap{"baz": Float64Map{"baz": Float64(999.9)}})

	out := in.ToFloat64MapMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64MapMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat64MapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64MapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat64ArrayArrayOutput(t *testing.T) {
	in := Float64ArrayArrayInput(Float64ArrayArray{Float64Array{Float64(999.9)}})

	out := in.ToFloat64ArrayArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64ArrayArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat64ArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64ArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIDOutput(t *testing.T) {
	in := IDInput(ID("foo"))

	out := in.ToIDOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIDOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIDPtrOutput(t *testing.T) {
	in := IDPtrInput(IDPtr(ID(ID("foo"))))

	out := in.ToIDPtrOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDPtrOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIDPtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDPtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIDArrayOutput(t *testing.T) {
	in := IDArrayInput(IDArray{ID("foo")})

	out := in.ToIDArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIDArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIDMapOutput(t *testing.T) {
	in := IDMapInput(IDMap{"baz": ID("foo")})

	out := in.ToIDMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIDMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIDArrayMapOutput(t *testing.T) {
	in := IDArrayMapInput(IDArrayMap{"baz": IDArray{ID("foo")}})

	out := in.ToIDArrayMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDArrayMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIDArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIDMapArrayOutput(t *testing.T) {
	in := IDMapArrayInput(IDMapArray{IDMap{"baz": ID("foo")}})

	out := in.ToIDMapArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDMapArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIDMapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDMapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIDMapMapOutput(t *testing.T) {
	in := IDMapMapInput(IDMapMap{"baz": IDMap{"baz": ID("foo")}})

	out := in.ToIDMapMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDMapMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIDMapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDMapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIDArrayArrayOutput(t *testing.T) {
	in := IDArrayArrayInput(IDArrayArray{IDArray{ID("foo")}})

	out := in.ToIDArrayArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDArrayArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIDArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToArrayOutput(t *testing.T) {
	in := ArrayInput(Array{String("any")})

	out := in.ToArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToMapOutput(t *testing.T) {
	in := MapInput(Map{"baz": String("any")})

	out := in.ToMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToArrayMapOutput(t *testing.T) {
	in := ArrayMapInput(ArrayMap{"baz": Array{String("any")}})

	out := in.ToArrayMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArrayMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToMapArrayOutput(t *testing.T) {
	in := MapArrayInput(MapArray{Map{"baz": String("any")}})

	out := in.ToMapArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToMapArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToMapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToMapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToMapMapOutput(t *testing.T) {
	in := MapMapInput(MapMap{"baz": Map{"baz": String("any")}})

	out := in.ToMapMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToMapMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToMapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToMapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToArrayArrayOutput(t *testing.T) {
	in := ArrayArrayInput(ArrayArray{Array{String("any")}})

	out := in.ToArrayArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArrayArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToArrayArrayMapOutput(t *testing.T) {
	in := ArrayArrayMapInput(ArrayArrayMap{"baz": ArrayArray{Array{String("any")}}})

	out := in.ToArrayArrayMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArrayArrayMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToArrayArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArrayArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIntOutput(t *testing.T) {
	in := IntInput(Int(42))

	out := in.ToIntOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIntOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIntPtrOutput(t *testing.T) {
	in := IntPtrInput(IntPtr(int(Int(42))))

	out := in.ToIntPtrOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntPtrOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIntPtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntPtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIntArrayOutput(t *testing.T) {
	in := IntArrayInput(IntArray{Int(42)})

	out := in.ToIntArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIntArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIntMapOutput(t *testing.T) {
	in := IntMapInput(IntMap{"baz": Int(42)})

	out := in.ToIntMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIntMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIntArrayMapOutput(t *testing.T) {
	in := IntArrayMapInput(IntArrayMap{"baz": IntArray{Int(42)}})

	out := in.ToIntArrayMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntArrayMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIntArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIntMapArrayOutput(t *testing.T) {
	in := IntMapArrayInput(IntMapArray{IntMap{"baz": Int(42)}})

	out := in.ToIntMapArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntMapArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIntMapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntMapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIntMapMapOutput(t *testing.T) {
	in := IntMapMapInput(IntMapMap{"baz": IntMap{"baz": Int(42)}})

	out := in.ToIntMapMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntMapMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIntMapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntMapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIntArrayArrayOutput(t *testing.T) {
	in := IntArrayArrayInput(IntArrayArray{IntArray{Int(42)}})

	out := in.ToIntArrayArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntArrayArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIntArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToStringOutput(t *testing.T) {
	in := StringInput(String("foo"))

	out := in.ToStringOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToStringOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToStringPtrOutput(t *testing.T) {
	in := StringPtrInput(StringPtr(string(String("foo"))))

	out := in.ToStringPtrOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringPtrOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToStringPtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringPtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToStringArrayOutput(t *testing.T) {
	in := StringArrayInput(StringArray{String("foo")})

	out := in.ToStringArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToStringArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToStringMapOutput(t *testing.T) {
	in := StringMapInput(StringMap{"baz": String("foo")})

	out := in.ToStringMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToStringMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToStringArrayMapOutput(t *testing.T) {
	in := StringArrayMapInput(StringArrayMap{"baz": StringArray{String("foo")}})

	out := in.ToStringArrayMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringArrayMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToStringArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToStringMapArrayOutput(t *testing.T) {
	in := StringMapArrayInput(StringMapArray{StringMap{"baz": String("foo")}})

	out := in.ToStringMapArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringMapArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToStringMapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringMapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToStringMapMapOutput(t *testing.T) {
	in := StringMapMapInput(StringMapMap{"baz": StringMap{"baz": String("foo")}})

	out := in.ToStringMapMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringMapMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToStringMapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringMapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToStringArrayArrayOutput(t *testing.T) {
	in := StringArrayArrayInput(StringArrayArray{StringArray{String("foo")}})

	out := in.ToStringArrayArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringArrayArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToStringArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToURNOutput(t *testing.T) {
	in := URNInput(URN("foo"))

	out := in.ToURNOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToURNOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToURNPtrOutput(t *testing.T) {
	in := URNPtrInput(URNPtr(URN(URN("foo"))))

	out := in.ToURNPtrOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNPtrOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToURNPtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNPtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToURNArrayOutput(t *testing.T) {
	in := URNArrayInput(URNArray{URN("foo")})

	out := in.ToURNArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToURNArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToURNMapOutput(t *testing.T) {
	in := URNMapInput(URNMap{"baz": URN("foo")})

	out := in.ToURNMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToURNMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToURNArrayMapOutput(t *testing.T) {
	in := URNArrayMapInput(URNArrayMap{"baz": URNArray{URN("foo")}})

	out := in.ToURNArrayMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNArrayMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToURNArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToURNMapArrayOutput(t *testing.T) {
	in := URNMapArrayInput(URNMapArray{URNMap{"baz": URN("foo")}})

	out := in.ToURNMapArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNMapArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToURNMapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNMapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToURNMapMapOutput(t *testing.T) {
	in := URNMapMapInput(URNMapMap{"baz": URNMap{"baz": URN("foo")}})

	out := in.ToURNMapMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNMapMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToURNMapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNMapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToURNArrayArrayOutput(t *testing.T) {
	in := URNArrayArrayInput(URNArrayArray{URNArray{URN("foo")}})

	out := in.ToURNArrayArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNArrayArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToURNArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

// Test type-specific ToOutput methods for builtins that implement other builtin input types.
func TestBuiltinConversions(t *testing.T) {
	archiveIn := NewFileArchive("foo.zip")
	assetOrArchiveOut := archiveIn.ToAssetOrArchiveOutput()
	archiveV, known, _, _, err := await(assetOrArchiveOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, archiveIn, archiveV)

	archiveOut := archiveIn.ToArchiveOutput()
	assetOrArchiveOut = archiveOut.ToAssetOrArchiveOutput()
	archiveV, known, _, _, err = await(assetOrArchiveOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, archiveIn, archiveV)

	assetIn := NewFileAsset("foo.zip")
	assetOrArchiveOut = assetIn.ToAssetOrArchiveOutput()
	assetV, known, _, _, err := await(assetOrArchiveOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, assetIn, assetV)

	assetOut := assetIn.ToAssetOutput()
	assetOrArchiveOut = assetOut.ToAssetOrArchiveOutput()
	assetV, known, _, _, err = await(assetOrArchiveOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, assetIn, assetV)

	idIn := ID("foo")
	stringOut := idIn.ToStringOutput()
	stringV, known, _, _, err := await(stringOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, string(idIn), stringV)

	idOut := idIn.ToIDOutput()
	stringOut = idOut.ToStringOutput()
	stringV, known, _, _, err = await(stringOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, string(idIn), stringV)

	urnIn := URN("foo")
	stringOut = urnIn.ToStringOutput()
	stringV, known, _, _, err = await(stringOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, string(urnIn), stringV)

	urnOut := urnIn.ToURNOutput()
	stringOut = urnOut.ToStringOutput()
	stringV, known, _, _, err = await(stringOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, string(urnIn), stringV)
}

// Test pointer types.

func TestBoolPtrElem(t *testing.T) {
	out := (BoolPtr(bool(Bool(true)))).ToBoolPtrOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*bool)), iv)
}

func TestFloat64PtrElem(t *testing.T) {
	out := (Float64Ptr(float64(Float64(999.9)))).ToFloat64PtrOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*float64)), iv)
}

func TestIDPtrElem(t *testing.T) {
	out := (IDPtr(ID(ID("foo")))).ToIDPtrOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*ID)), iv)
}

func TestIntPtrElem(t *testing.T) {
	out := (IntPtr(int(Int(42)))).ToIntPtrOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*int)), iv)
}

func TestStringPtrElem(t *testing.T) {
	out := (StringPtr(string(String("foo")))).ToStringPtrOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*string)), iv)
}

func TestURNPtrElem(t *testing.T) {
	out := (URNPtr(URN(URN("foo")))).ToURNPtrOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*URN)), iv)
}

// Test array indexers.

func TestArchiveArrayIndex(t *testing.T) {
	out := (ArchiveArray{NewFileArchive("foo.zip")}).ToArchiveArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([]Archive)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToArchiveArray(t *testing.T) {
	out := ToArchiveArray([]Archive{NewFileArchive("foo.zip")}).ToArchiveArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]Archive)[0], iv)
}

func TestTopLevelToArchiveArrayOutput(t *testing.T) {
	out := ToArchiveArrayOutput([]ArchiveOutput{ToOutput(NewFileArchive("foo.zip")).(ArchiveOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]Archive)[0], iv)
}

func TestArchiveMapArrayIndex(t *testing.T) {
	out := (ArchiveMapArray{ArchiveMap{"baz": NewFileArchive("foo.zip")}}).ToArchiveMapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([]map[string]Archive)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToArchiveMapArray(t *testing.T) {
	out := ToArchiveMapArray([]map[string]Archive{{"baz": NewFileArchive("foo.zip")}}).ToArchiveMapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]Archive)[0], iv)
}

func TestTopLevelToArchiveMapArrayOutput(t *testing.T) {
	out := ToArchiveMapArrayOutput([]ArchiveMapOutput{ToOutput(ArchiveMap{"baz": NewFileArchive("foo.zip")}).(ArchiveMapOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]Archive)[0], iv)
}

func TestArchiveArrayArrayIndex(t *testing.T) {
	out := (ArchiveArrayArray{ArchiveArray{NewFileArchive("foo.zip")}}).ToArchiveArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([][]Archive)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToArchiveArrayArray(t *testing.T) {
	out := ToArchiveArrayArray([][]Archive{{NewFileArchive("foo.zip")}}).ToArchiveArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]Archive)[0], iv)
}

func TestTopLevelToArchiveArrayArrayOutput(t *testing.T) {
	out := ToArchiveArrayArrayOutput([]ArchiveArrayOutput{ToOutput(ArchiveArray{NewFileArchive("foo.zip")}).(ArchiveArrayOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]Archive)[0], iv)
}

func TestAssetArrayIndex(t *testing.T) {
	out := (AssetArray{NewFileAsset("foo.txt")}).ToAssetArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([]Asset)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToAssetArray(t *testing.T) {
	out := ToAssetArray([]Asset{NewFileAsset("foo.txt")}).ToAssetArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]Asset)[0], iv)
}

func TestTopLevelToAssetArrayOutput(t *testing.T) {
	out := ToAssetArrayOutput([]AssetOutput{ToOutput(NewFileAsset("foo.txt")).(AssetOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]Asset)[0], iv)
}

func TestAssetMapArrayIndex(t *testing.T) {
	out := (AssetMapArray{AssetMap{"baz": NewFileAsset("foo.txt")}}).ToAssetMapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([]map[string]Asset)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToAssetMapArray(t *testing.T) {
	out := ToAssetMapArray([]map[string]Asset{{"baz": NewFileAsset("foo.txt")}}).ToAssetMapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]Asset)[0], iv)
}

func TestTopLevelToAssetMapArrayOutput(t *testing.T) {
	out := ToAssetMapArrayOutput([]AssetMapOutput{ToOutput(AssetMap{"baz": NewFileAsset("foo.txt")}).(AssetMapOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]Asset)[0], iv)
}

func TestAssetArrayArrayIndex(t *testing.T) {
	out := (AssetArrayArray{AssetArray{NewFileAsset("foo.txt")}}).ToAssetArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([][]Asset)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToAssetArrayArray(t *testing.T) {
	out := ToAssetArrayArray([][]Asset{{NewFileAsset("foo.txt")}}).ToAssetArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]Asset)[0], iv)
}

func TestTopLevelToAssetArrayArrayOutput(t *testing.T) {
	out := ToAssetArrayArrayOutput([]AssetArrayOutput{ToOutput(AssetArray{NewFileAsset("foo.txt")}).(AssetArrayOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]Asset)[0], iv)
}

func TestAssetOrArchiveArrayIndex(t *testing.T) {
	out := (AssetOrArchiveArray{NewFileArchive("foo.zip")}).ToAssetOrArchiveArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([]AssetOrArchive)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestAssetOrArchiveMapArrayIndex(t *testing.T) {
	out := (AssetOrArchiveMapArray{AssetOrArchiveMap{"baz": NewFileArchive("foo.zip")}}).ToAssetOrArchiveMapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([]map[string]AssetOrArchive)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestAssetOrArchiveArrayArrayIndex(t *testing.T) {
	out := (AssetOrArchiveArrayArray{AssetOrArchiveArray{NewFileArchive("foo.zip")}}).ToAssetOrArchiveArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([][]AssetOrArchive)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestBoolArrayIndex(t *testing.T) {
	out := (BoolArray{Bool(true)}).ToBoolArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([]bool)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToBoolArray(t *testing.T) {
	out := ToBoolArray([]bool{true}).ToBoolArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]bool)[0], iv)
}

func TestTopLevelToBoolArrayOutput(t *testing.T) {
	out := ToBoolArrayOutput([]BoolOutput{ToOutput(Bool(true)).(BoolOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]bool)[0], iv)
}

func TestBoolMapArrayIndex(t *testing.T) {
	out := (BoolMapArray{BoolMap{"baz": Bool(true)}}).ToBoolMapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([]map[string]bool)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToBoolMapArray(t *testing.T) {
	out := ToBoolMapArray([]map[string]bool{{"baz": true}}).ToBoolMapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]bool)[0], iv)
}

func TestTopLevelToBoolMapArrayOutput(t *testing.T) {
	out := ToBoolMapArrayOutput([]BoolMapOutput{ToOutput(BoolMap{"baz": Bool(true)}).(BoolMapOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]bool)[0], iv)
}

func TestBoolArrayArrayIndex(t *testing.T) {
	out := (BoolArrayArray{BoolArray{Bool(true)}}).ToBoolArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([][]bool)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToBoolArrayArray(t *testing.T) {
	out := ToBoolArrayArray([][]bool{{true}}).ToBoolArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]bool)[0], iv)
}

func TestTopLevelToBoolArrayArrayOutput(t *testing.T) {
	out := ToBoolArrayArrayOutput([]BoolArrayOutput{ToOutput(BoolArray{Bool(true)}).(BoolArrayOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]bool)[0], iv)
}

func TestFloat64ArrayIndex(t *testing.T) {
	out := (Float64Array{Float64(999.9)}).ToFloat64ArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([]float64)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToFloat64Array(t *testing.T) {
	out := ToFloat64Array([]float64{999.9}).ToFloat64ArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]float64)[0], iv)
}

func TestTopLevelToFloat64ArrayOutput(t *testing.T) {
	out := ToFloat64ArrayOutput([]Float64Output{ToOutput(Float64(999.9)).(Float64Output)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]float64)[0], iv)
}

func TestFloat64MapArrayIndex(t *testing.T) {
	out := (Float64MapArray{Float64Map{"baz": Float64(999.9)}}).ToFloat64MapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([]map[string]float64)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToFloat64MapArray(t *testing.T) {
	out := ToFloat64MapArray([]map[string]float64{{"baz": 999.9}}).ToFloat64MapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]float64)[0], iv)
}

func TestTopLevelToFloat64MapArrayOutput(t *testing.T) {
	out := ToFloat64MapArrayOutput([]Float64MapOutput{ToOutput(Float64Map{"baz": Float64(999.9)}).(Float64MapOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]float64)[0], iv)
}

func TestFloat64ArrayArrayIndex(t *testing.T) {
	out := (Float64ArrayArray{Float64Array{Float64(999.9)}}).ToFloat64ArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([][]float64)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToFloat64ArrayArray(t *testing.T) {
	out := ToFloat64ArrayArray([][]float64{{999.9}}).ToFloat64ArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]float64)[0], iv)
}

func TestTopLevelToFloat64ArrayArrayOutput(t *testing.T) {
	out := ToFloat64ArrayArrayOutput([]Float64ArrayOutput{ToOutput(Float64Array{Float64(999.9)}).(Float64ArrayOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]float64)[0], iv)
}

func TestIDArrayIndex(t *testing.T) {
	out := (IDArray{ID("foo")}).ToIDArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([]ID)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToIDArray(t *testing.T) {
	out := ToIDArray([]ID{ID("foo")}).ToIDArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]ID)[0], iv)
}

func TestTopLevelToIDArrayOutput(t *testing.T) {
	out := ToIDArrayOutput([]IDOutput{ToOutput(ID("foo")).(IDOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]ID)[0], iv)
}

func TestIDMapArrayIndex(t *testing.T) {
	out := (IDMapArray{IDMap{"baz": ID("foo")}}).ToIDMapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([]map[string]ID)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToIDMapArray(t *testing.T) {
	out := ToIDMapArray([]map[string]ID{{"baz": ID("foo")}}).ToIDMapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]ID)[0], iv)
}

func TestTopLevelToIDMapArrayOutput(t *testing.T) {
	out := ToIDMapArrayOutput([]IDMapOutput{ToOutput(IDMap{"baz": ID("foo")}).(IDMapOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]ID)[0], iv)
}

func TestIDArrayArrayIndex(t *testing.T) {
	out := (IDArrayArray{IDArray{ID("foo")}}).ToIDArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([][]ID)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToIDArrayArray(t *testing.T) {
	out := ToIDArrayArray([][]ID{{ID("foo")}}).ToIDArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]ID)[0], iv)
}

func TestTopLevelToIDArrayArrayOutput(t *testing.T) {
	out := ToIDArrayArrayOutput([]IDArrayOutput{ToOutput(IDArray{ID("foo")}).(IDArrayOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]ID)[0], iv)
}

func TestArrayIndex(t *testing.T) {
	out := (Array{String("any")}).ToArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([]interface{})[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToArray(t *testing.T) {
	out := ToArray([]interface{}{String("any")}).ToArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]interface{})[0], iv)
}

func TestTopLevelToArrayOutput(t *testing.T) {
	out := ToArrayOutput([]Output{ToOutput(String("any")).(Output)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]interface{})[0], iv)
}

func TestMapArrayIndex(t *testing.T) {
	out := (MapArray{Map{"baz": String("any")}}).ToMapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([]map[string]interface{})[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToMapArray(t *testing.T) {
	out := ToMapArray([]map[string]interface{}{{"baz": String("any")}}).ToMapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]interface{})[0], iv)
}

func TestTopLevelToMapArrayOutput(t *testing.T) {
	out := ToMapArrayOutput([]MapOutput{ToOutput(Map{"baz": String("any")}).(MapOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]interface{})[0], iv)
}

func TestArrayArrayIndex(t *testing.T) {
	out := (ArrayArray{Array{String("any")}}).ToArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([][]interface{})[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToArrayArray(t *testing.T) {
	out := ToArrayArray([][]interface{}{{String("any")}}).ToArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]interface{})[0], iv)
}

func TestTopLevelToArrayArrayOutput(t *testing.T) {
	out := ToArrayArrayOutput([]ArrayOutput{ToOutput(Array{String("any")}).(ArrayOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]interface{})[0], iv)
}

func TestIntArrayIndex(t *testing.T) {
	out := (IntArray{Int(42)}).ToIntArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([]int)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToIntArray(t *testing.T) {
	out := ToIntArray([]int{42}).ToIntArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]int)[0], iv)
}

func TestTopLevelToIntArrayOutput(t *testing.T) {
	out := ToIntArrayOutput([]IntOutput{ToOutput(Int(42)).(IntOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]int)[0], iv)
}

func TestIntMapArrayIndex(t *testing.T) {
	out := (IntMapArray{IntMap{"baz": Int(42)}}).ToIntMapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([]map[string]int)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToIntMapArray(t *testing.T) {
	out := ToIntMapArray([]map[string]int{{"baz": 42}}).ToIntMapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]int)[0], iv)
}

func TestTopLevelToIntMapArrayOutput(t *testing.T) {
	out := ToIntMapArrayOutput([]IntMapOutput{ToOutput(IntMap{"baz": Int(42)}).(IntMapOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]int)[0], iv)
}

func TestIntArrayArrayIndex(t *testing.T) {
	out := (IntArrayArray{IntArray{Int(42)}}).ToIntArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([][]int)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToIntArrayArray(t *testing.T) {
	out := ToIntArrayArray([][]int{{42}}).ToIntArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]int)[0], iv)
}

func TestTopLevelToIntArrayArrayOutput(t *testing.T) {
	out := ToIntArrayArrayOutput([]IntArrayOutput{ToOutput(IntArray{Int(42)}).(IntArrayOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]int)[0], iv)
}

func TestStringArrayIndex(t *testing.T) {
	out := (StringArray{String("foo")}).ToStringArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([]string)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToStringArray(t *testing.T) {
	out := ToStringArray([]string{"foo"}).ToStringArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]string)[0], iv)
}

func TestTopLevelToStringArrayOutput(t *testing.T) {
	out := ToStringArrayOutput([]StringOutput{ToOutput(String("foo")).(StringOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]string)[0], iv)
}

func TestStringMapArrayIndex(t *testing.T) {
	out := (StringMapArray{StringMap{"baz": String("foo")}}).ToStringMapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([]map[string]string)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToStringMapArray(t *testing.T) {
	out := ToStringMapArray([]map[string]string{{"baz": "foo"}}).ToStringMapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]string)[0], iv)
}

func TestTopLevelToStringMapArrayOutput(t *testing.T) {
	out := ToStringMapArrayOutput([]StringMapOutput{ToOutput(StringMap{"baz": String("foo")}).(StringMapOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]string)[0], iv)
}

func TestStringArrayArrayIndex(t *testing.T) {
	out := (StringArrayArray{StringArray{String("foo")}}).ToStringArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([][]string)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToStringArrayArray(t *testing.T) {
	out := ToStringArrayArray([][]string{{"foo"}}).ToStringArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]string)[0], iv)
}

func TestTopLevelToStringArrayArrayOutput(t *testing.T) {
	out := ToStringArrayArrayOutput([]StringArrayOutput{ToOutput(StringArray{String("foo")}).(StringArrayOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]string)[0], iv)
}

func TestURNArrayIndex(t *testing.T) {
	out := (URNArray{URN("foo")}).ToURNArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([]URN)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToURNArray(t *testing.T) {
	out := ToURNArray([]URN{URN("foo")}).ToURNArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]URN)[0], iv)
}

func TestTopLevelToURNArrayOutput(t *testing.T) {
	out := ToURNArrayOutput([]URNOutput{ToOutput(URN("foo")).(URNOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]URN)[0], iv)
}

func TestURNMapArrayIndex(t *testing.T) {
	out := (URNMapArray{URNMap{"baz": URN("foo")}}).ToURNMapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([]map[string]URN)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToURNMapArray(t *testing.T) {
	out := ToURNMapArray([]map[string]URN{{"baz": URN("foo")}}).ToURNMapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]URN)[0], iv)
}

func TestTopLevelToURNMapArrayOutput(t *testing.T) {
	out := ToURNMapArrayOutput([]URNMapOutput{ToOutput(URNMap{"baz": URN("foo")}).(URNMapOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]URN)[0], iv)
}

func TestURNArrayArrayIndex(t *testing.T) {
	out := (URNArrayArray{URNArray{URN("foo")}}).ToURNArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.([][]URN)[0], iv)

	iv, known, _, _, err = await(out.Index(Int(-1)))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToURNArrayArray(t *testing.T) {
	out := ToURNArrayArray([][]URN{{URN("foo")}}).ToURNArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]URN)[0], iv)
}

func TestTopLevelToURNArrayArrayOutput(t *testing.T) {
	out := ToURNArrayArrayOutput([]URNArrayOutput{ToOutput(URNArray{URN("foo")}).(URNArrayOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]URN)[0], iv)
}

// Test map indexers.

func TestArchiveMapIndex(t *testing.T) {
	out := (ArchiveMap{"baz": NewFileArchive("foo.zip")}).ToArchiveMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string]Archive)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToArchiveMap(t *testing.T) {
	out := ToArchiveMap(map[string]Archive{"baz": NewFileArchive("foo.zip")}).ToArchiveMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]Archive)["baz"], iv)
}

func TestTopLevelToArchiveMapOutput(t *testing.T) {
	out := ToArchiveMapOutput(map[string]ArchiveOutput{"baz": ToOutput(NewFileArchive("foo.zip")).(ArchiveOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]Archive)["baz"], iv)
}

func TestArchiveArrayMapIndex(t *testing.T) {
	out := (ArchiveArrayMap{"baz": ArchiveArray{NewFileArchive("foo.zip")}}).ToArchiveArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string][]Archive)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToArchiveArrayMap(t *testing.T) {
	out := ToArchiveArrayMap(map[string][]Archive{"baz": {NewFileArchive("foo.zip")}}).ToArchiveArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]Archive)["baz"], iv)
}

func TestTopLevelToArchiveArrayMapOutput(t *testing.T) {
	out := ToArchiveArrayMapOutput(map[string]ArchiveArrayOutput{"baz": ToOutput(ArchiveArray{NewFileArchive("foo.zip")}).(ArchiveArrayOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]Archive)["baz"], iv)
}

func TestArchiveMapMapIndex(t *testing.T) {
	out := (ArchiveMapMap{"baz": ArchiveMap{"baz": NewFileArchive("foo.zip")}}).ToArchiveMapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string]map[string]Archive)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToArchiveMapMap(t *testing.T) {
	out := ToArchiveMapMap(map[string]map[string]Archive{"baz": {"baz": NewFileArchive("foo.zip")}}).ToArchiveMapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]Archive)["baz"], iv)
}

func TestTopLevelToArchiveMapMapOutput(t *testing.T) {
	out := ToArchiveMapMapOutput(map[string]ArchiveMapOutput{"baz": ToOutput(ArchiveMap{"baz": NewFileArchive("foo.zip")}).(ArchiveMapOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]Archive)["baz"], iv)
}

func TestAssetMapIndex(t *testing.T) {
	out := (AssetMap{"baz": NewFileAsset("foo.txt")}).ToAssetMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string]Asset)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToAssetMap(t *testing.T) {
	out := ToAssetMap(map[string]Asset{"baz": NewFileAsset("foo.txt")}).ToAssetMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]Asset)["baz"], iv)
}

func TestTopLevelToAssetMapOutput(t *testing.T) {
	out := ToAssetMapOutput(map[string]AssetOutput{"baz": ToOutput(NewFileAsset("foo.txt")).(AssetOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]Asset)["baz"], iv)
}

func TestAssetArrayMapIndex(t *testing.T) {
	out := (AssetArrayMap{"baz": AssetArray{NewFileAsset("foo.txt")}}).ToAssetArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string][]Asset)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToAssetArrayMap(t *testing.T) {
	out := ToAssetArrayMap(map[string][]Asset{"baz": {NewFileAsset("foo.txt")}}).ToAssetArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]Asset)["baz"], iv)
}

func TestTopLevelToAssetArrayMapOutput(t *testing.T) {
	out := ToAssetArrayMapOutput(map[string]AssetArrayOutput{"baz": ToOutput(AssetArray{NewFileAsset("foo.txt")}).(AssetArrayOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]Asset)["baz"], iv)
}

func TestAssetMapMapIndex(t *testing.T) {
	out := (AssetMapMap{"baz": AssetMap{"baz": NewFileAsset("foo.txt")}}).ToAssetMapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string]map[string]Asset)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToAssetMapMap(t *testing.T) {
	out := ToAssetMapMap(map[string]map[string]Asset{"baz": {"baz": NewFileAsset("foo.txt")}}).ToAssetMapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]Asset)["baz"], iv)
}

func TestTopLevelToAssetMapMapOutput(t *testing.T) {
	out := ToAssetMapMapOutput(map[string]AssetMapOutput{"baz": ToOutput(AssetMap{"baz": NewFileAsset("foo.txt")}).(AssetMapOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]Asset)["baz"], iv)
}

func TestAssetOrArchiveMapIndex(t *testing.T) {
	out := (AssetOrArchiveMap{"baz": NewFileArchive("foo.zip")}).ToAssetOrArchiveMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string]AssetOrArchive)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestAssetOrArchiveArrayMapIndex(t *testing.T) {
	out := (AssetOrArchiveArrayMap{"baz": AssetOrArchiveArray{NewFileArchive("foo.zip")}}).ToAssetOrArchiveArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string][]AssetOrArchive)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestAssetOrArchiveMapMapIndex(t *testing.T) {
	out := (AssetOrArchiveMapMap{"baz": AssetOrArchiveMap{"baz": NewFileArchive("foo.zip")}}).ToAssetOrArchiveMapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string]map[string]AssetOrArchive)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestBoolMapIndex(t *testing.T) {
	out := (BoolMap{"baz": Bool(true)}).ToBoolMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string]bool)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToBoolMap(t *testing.T) {
	out := ToBoolMap(map[string]bool{"baz": true}).ToBoolMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]bool)["baz"], iv)
}

func TestTopLevelToBoolMapOutput(t *testing.T) {
	out := ToBoolMapOutput(map[string]BoolOutput{"baz": ToOutput(Bool(true)).(BoolOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]bool)["baz"], iv)
}

func TestBoolArrayMapIndex(t *testing.T) {
	out := (BoolArrayMap{"baz": BoolArray{Bool(true)}}).ToBoolArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string][]bool)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToBoolArrayMap(t *testing.T) {
	out := ToBoolArrayMap(map[string][]bool{"baz": {true}}).ToBoolArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]bool)["baz"], iv)
}

func TestTopLevelToBoolArrayMapOutput(t *testing.T) {
	out := ToBoolArrayMapOutput(map[string]BoolArrayOutput{"baz": ToOutput(BoolArray{Bool(true)}).(BoolArrayOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]bool)["baz"], iv)
}

func TestBoolMapMapIndex(t *testing.T) {
	out := (BoolMapMap{"baz": BoolMap{"baz": Bool(true)}}).ToBoolMapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string]map[string]bool)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToBoolMapMap(t *testing.T) {
	out := ToBoolMapMap(map[string]map[string]bool{"baz": {"baz": true}}).ToBoolMapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]bool)["baz"], iv)
}

func TestTopLevelToBoolMapMapOutput(t *testing.T) {
	out := ToBoolMapMapOutput(map[string]BoolMapOutput{"baz": ToOutput(BoolMap{"baz": Bool(true)}).(BoolMapOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]bool)["baz"], iv)
}

func TestFloat64MapIndex(t *testing.T) {
	out := (Float64Map{"baz": Float64(999.9)}).ToFloat64MapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string]float64)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToFloat64Map(t *testing.T) {
	out := ToFloat64Map(map[string]float64{"baz": 999.9}).ToFloat64MapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]float64)["baz"], iv)
}

func TestTopLevelToFloat64MapOutput(t *testing.T) {
	out := ToFloat64MapOutput(map[string]Float64Output{"baz": ToOutput(Float64(999.9)).(Float64Output)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]float64)["baz"], iv)
}

func TestFloat64ArrayMapIndex(t *testing.T) {
	out := (Float64ArrayMap{"baz": Float64Array{Float64(999.9)}}).ToFloat64ArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string][]float64)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToFloat64ArrayMap(t *testing.T) {
	out := ToFloat64ArrayMap(map[string][]float64{"baz": {999.9}}).ToFloat64ArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]float64)["baz"], iv)
}

func TestTopLevelToFloat64ArrayMapOutput(t *testing.T) {
	out := ToFloat64ArrayMapOutput(map[string]Float64ArrayOutput{"baz": ToOutput(Float64Array{Float64(999.9)}).(Float64ArrayOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]float64)["baz"], iv)
}

func TestFloat64MapMapIndex(t *testing.T) {
	out := (Float64MapMap{"baz": Float64Map{"baz": Float64(999.9)}}).ToFloat64MapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string]map[string]float64)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToFloat64MapMap(t *testing.T) {
	out := ToFloat64MapMap(map[string]map[string]float64{"baz": {"baz": 999.9}}).ToFloat64MapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]float64)["baz"], iv)
}

func TestTopLevelToFloat64MapMapOutput(t *testing.T) {
	out := ToFloat64MapMapOutput(map[string]Float64MapOutput{"baz": ToOutput(Float64Map{"baz": Float64(999.9)}).(Float64MapOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]float64)["baz"], iv)
}

func TestIDMapIndex(t *testing.T) {
	out := (IDMap{"baz": ID("foo")}).ToIDMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string]ID)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToIDMap(t *testing.T) {
	out := ToIDMap(map[string]ID{"baz": ID("foo")}).ToIDMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]ID)["baz"], iv)
}

func TestTopLevelToIDMapOutput(t *testing.T) {
	out := ToIDMapOutput(map[string]IDOutput{"baz": ToOutput(ID("foo")).(IDOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]ID)["baz"], iv)
}

func TestIDArrayMapIndex(t *testing.T) {
	out := (IDArrayMap{"baz": IDArray{ID("foo")}}).ToIDArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string][]ID)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToIDArrayMap(t *testing.T) {
	out := ToIDArrayMap(map[string][]ID{"baz": {ID("foo")}}).ToIDArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]ID)["baz"], iv)
}

func TestTopLevelToIDArrayMapOutput(t *testing.T) {
	out := ToIDArrayMapOutput(map[string]IDArrayOutput{"baz": ToOutput(IDArray{ID("foo")}).(IDArrayOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]ID)["baz"], iv)
}

func TestIDMapMapIndex(t *testing.T) {
	out := (IDMapMap{"baz": IDMap{"baz": ID("foo")}}).ToIDMapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string]map[string]ID)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToIDMapMap(t *testing.T) {
	out := ToIDMapMap(map[string]map[string]ID{"baz": {"baz": ID("foo")}}).ToIDMapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]ID)["baz"], iv)
}

func TestTopLevelToIDMapMapOutput(t *testing.T) {
	out := ToIDMapMapOutput(map[string]IDMapOutput{"baz": ToOutput(IDMap{"baz": ID("foo")}).(IDMapOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]ID)["baz"], iv)
}

func TestMapIndex(t *testing.T) {
	out := (Map{"baz": String("any")}).ToMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string]interface{})["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToMap(t *testing.T) {
	out := ToMap(map[string]interface{}{"baz": String("any")}).ToMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]interface{})["baz"], iv)
}

func TestTopLevelToMapOutput(t *testing.T) {
	out := ToMapOutput(map[string]Output{"baz": ToOutput(String("any")).(Output)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]interface{})["baz"], iv)
}

func TestArrayMapIndex(t *testing.T) {
	out := (ArrayMap{"baz": Array{String("any")}}).ToArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string][]interface{})["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToArrayMap(t *testing.T) {
	out := ToArrayMap(map[string][]interface{}{"baz": {String("any")}}).ToArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]interface{})["baz"], iv)
}

func TestTopLevelToArrayMapOutput(t *testing.T) {
	out := ToArrayMapOutput(map[string]ArrayOutput{"baz": ToOutput(Array{String("any")}).(ArrayOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]interface{})["baz"], iv)
}

func TestMapMapIndex(t *testing.T) {
	out := (MapMap{"baz": Map{"baz": String("any")}}).ToMapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string]map[string]interface{})["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToMapMap(t *testing.T) {
	out := ToMapMap(map[string]map[string]interface{}{"baz": {"baz": String("any")}}).ToMapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]interface{})["baz"], iv)
}

func TestTopLevelToMapMapOutput(t *testing.T) {
	out := ToMapMapOutput(map[string]MapOutput{"baz": ToOutput(Map{"baz": String("any")}).(MapOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]interface{})["baz"], iv)
}

func TestArrayArrayMapIndex(t *testing.T) {
	out := (ArrayArrayMap{"baz": ArrayArray{Array{String("any")}}}).ToArrayArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string][][]interface{})["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToArrayArrayMap(t *testing.T) {
	out := ToArrayArrayMap(map[string][][]interface{}{"baz": {{String("any")}}}).ToArrayArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][][]interface{})["baz"], iv)
}

func TestTopLevelToArrayArrayMapOutput(t *testing.T) {
	out := ToArrayArrayMapOutput(map[string]ArrayArrayOutput{"baz": ToOutput(ArrayArray{Array{String("any")}}).(ArrayArrayOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][][]interface{})["baz"], iv)
}

func TestIntMapIndex(t *testing.T) {
	out := (IntMap{"baz": Int(42)}).ToIntMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string]int)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToIntMap(t *testing.T) {
	out := ToIntMap(map[string]int{"baz": 42}).ToIntMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]int)["baz"], iv)
}

func TestTopLevelToIntMapOutput(t *testing.T) {
	out := ToIntMapOutput(map[string]IntOutput{"baz": ToOutput(Int(42)).(IntOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]int)["baz"], iv)
}

func TestIntArrayMapIndex(t *testing.T) {
	out := (IntArrayMap{"baz": IntArray{Int(42)}}).ToIntArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string][]int)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToIntArrayMap(t *testing.T) {
	out := ToIntArrayMap(map[string][]int{"baz": {42}}).ToIntArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]int)["baz"], iv)
}

func TestTopLevelToIntArrayMapOutput(t *testing.T) {
	out := ToIntArrayMapOutput(map[string]IntArrayOutput{"baz": ToOutput(IntArray{Int(42)}).(IntArrayOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]int)["baz"], iv)
}

func TestIntMapMapIndex(t *testing.T) {
	out := (IntMapMap{"baz": IntMap{"baz": Int(42)}}).ToIntMapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string]map[string]int)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToIntMapMap(t *testing.T) {
	out := ToIntMapMap(map[string]map[string]int{"baz": {"baz": 42}}).ToIntMapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]int)["baz"], iv)
}

func TestTopLevelToIntMapMapOutput(t *testing.T) {
	out := ToIntMapMapOutput(map[string]IntMapOutput{"baz": ToOutput(IntMap{"baz": Int(42)}).(IntMapOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]int)["baz"], iv)
}

func TestStringMapIndex(t *testing.T) {
	out := (StringMap{"baz": String("foo")}).ToStringMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string]string)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToStringMap(t *testing.T) {
	out := ToStringMap(map[string]string{"baz": "foo"}).ToStringMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]string)["baz"], iv)
}

func TestTopLevelToStringMapOutput(t *testing.T) {
	out := ToStringMapOutput(map[string]StringOutput{"baz": ToOutput(String("foo")).(StringOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]string)["baz"], iv)
}

func TestStringArrayMapIndex(t *testing.T) {
	out := (StringArrayMap{"baz": StringArray{String("foo")}}).ToStringArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string][]string)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToStringArrayMap(t *testing.T) {
	out := ToStringArrayMap(map[string][]string{"baz": {"foo"}}).ToStringArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]string)["baz"], iv)
}

func TestTopLevelToStringArrayMapOutput(t *testing.T) {
	out := ToStringArrayMapOutput(map[string]StringArrayOutput{"baz": ToOutput(StringArray{String("foo")}).(StringArrayOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]string)["baz"], iv)
}

func TestStringMapMapIndex(t *testing.T) {
	out := (StringMapMap{"baz": StringMap{"baz": String("foo")}}).ToStringMapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string]map[string]string)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToStringMapMap(t *testing.T) {
	out := ToStringMapMap(map[string]map[string]string{"baz": {"baz": "foo"}}).ToStringMapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]string)["baz"], iv)
}

func TestTopLevelToStringMapMapOutput(t *testing.T) {
	out := ToStringMapMapOutput(map[string]StringMapOutput{"baz": ToOutput(StringMap{"baz": String("foo")}).(StringMapOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]string)["baz"], iv)
}

func TestURNMapIndex(t *testing.T) {
	out := (URNMap{"baz": URN("foo")}).ToURNMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string]URN)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToURNMap(t *testing.T) {
	out := ToURNMap(map[string]URN{"baz": URN("foo")}).ToURNMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]URN)["baz"], iv)
}

func TestTopLevelToURNMapOutput(t *testing.T) {
	out := ToURNMapOutput(map[string]URNOutput{"baz": ToOutput(URN("foo")).(URNOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]URN)["baz"], iv)
}

func TestURNArrayMapIndex(t *testing.T) {
	out := (URNArrayMap{"baz": URNArray{URN("foo")}}).ToURNArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string][]URN)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToURNArrayMap(t *testing.T) {
	out := ToURNArrayMap(map[string][]URN{"baz": {URN("foo")}}).ToURNArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]URN)["baz"], iv)
}

func TestTopLevelToURNArrayMapOutput(t *testing.T) {
	out := ToURNArrayMapOutput(map[string]URNArrayOutput{"baz": ToOutput(URNArray{URN("foo")}).(URNArrayOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]URN)["baz"], iv)
}

func TestURNMapMapIndex(t *testing.T) {
	out := (URNMapMap{"baz": URNMap{"baz": URN("foo")}}).ToURNMapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.EqualValues(t, av.(map[string]map[string]URN)["baz"], iv)

	iv, known, _, _, err = await(out.MapIndex(String("notfound")))
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Zero(t, iv)
}

func TestToURNMapMap(t *testing.T) {
	out := ToURNMapMap(map[string]map[string]URN{"baz": {"baz": URN("foo")}}).ToURNMapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]URN)["baz"], iv)
}

func TestTopLevelToURNMapMapOutput(t *testing.T) {
	out := ToURNMapMapOutput(map[string]URNMapOutput{"baz": ToOutput(URNMap{"baz": URN("foo")}).(URNMapOutput)})

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]URN)["baz"], iv)
}
