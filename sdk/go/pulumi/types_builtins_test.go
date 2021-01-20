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
	// Test builtin applies.
	{
		out := newIntOutput()
		go func() { out.resolve(42, true, false, nil) }()

		t.Run("ApplyArchive", func(t *testing.T) {

			o2 := out.ApplyArchiveWithContext(context.Background(), func(_ context.Context, v int) Archive { return *new(Archive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyArchiveArray", func(t *testing.T) {

			o2 := out.ApplyArchiveArrayWithContext(context.Background(), func(_ context.Context, v int) []Archive { return *new([]Archive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyArchiveMap", func(t *testing.T) {

			o2 := out.ApplyArchiveMapWithContext(context.Background(), func(_ context.Context, v int) map[string]Archive { return *new(map[string]Archive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyArchiveArrayMap", func(t *testing.T) {

			o2 := out.ApplyArchiveArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]Archive { return *new(map[string][]Archive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyArchiveMapArray", func(t *testing.T) {

			o2 := out.ApplyArchiveMapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]Archive { return *new([]map[string]Archive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyArchiveMapMap", func(t *testing.T) {

			o2 := out.ApplyArchiveMapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]Archive {
				return *new(map[string]map[string]Archive)
			})
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyArchiveArrayArray", func(t *testing.T) {

			o2 := out.ApplyArchiveArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]Archive { return *new([][]Archive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAsset", func(t *testing.T) {

			o2 := out.ApplyAssetWithContext(context.Background(), func(_ context.Context, v int) Asset { return *new(Asset) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetArray", func(t *testing.T) {

			o2 := out.ApplyAssetArrayWithContext(context.Background(), func(_ context.Context, v int) []Asset { return *new([]Asset) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetMap", func(t *testing.T) {

			o2 := out.ApplyAssetMapWithContext(context.Background(), func(_ context.Context, v int) map[string]Asset { return *new(map[string]Asset) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetArrayMap", func(t *testing.T) {

			o2 := out.ApplyAssetArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]Asset { return *new(map[string][]Asset) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetMapArray", func(t *testing.T) {

			o2 := out.ApplyAssetMapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]Asset { return *new([]map[string]Asset) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetMapMap", func(t *testing.T) {

			o2 := out.ApplyAssetMapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]Asset { return *new(map[string]map[string]Asset) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetArrayArray", func(t *testing.T) {

			o2 := out.ApplyAssetArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]Asset { return *new([][]Asset) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetOrArchive", func(t *testing.T) {

			o2 := out.ApplyAssetOrArchiveWithContext(context.Background(), func(_ context.Context, v int) AssetOrArchive { return *new(AssetOrArchive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetOrArchiveArray", func(t *testing.T) {

			o2 := out.ApplyAssetOrArchiveArrayWithContext(context.Background(), func(_ context.Context, v int) []AssetOrArchive { return *new([]AssetOrArchive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetOrArchiveMap", func(t *testing.T) {

			o2 := out.ApplyAssetOrArchiveMapWithContext(context.Background(), func(_ context.Context, v int) map[string]AssetOrArchive { return *new(map[string]AssetOrArchive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetOrArchiveArrayMap", func(t *testing.T) {

			o2 := out.ApplyAssetOrArchiveArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]AssetOrArchive { return *new(map[string][]AssetOrArchive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetOrArchiveMapArray", func(t *testing.T) {

			o2 := out.ApplyAssetOrArchiveMapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]AssetOrArchive { return *new([]map[string]AssetOrArchive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetOrArchiveMapMap", func(t *testing.T) {

			o2 := out.ApplyAssetOrArchiveMapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]AssetOrArchive {
				return *new(map[string]map[string]AssetOrArchive)
			})
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetOrArchiveArrayArray", func(t *testing.T) {

			o2 := out.ApplyAssetOrArchiveArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]AssetOrArchive { return *new([][]AssetOrArchive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyBool", func(t *testing.T) {

			o2 := out.ApplyBoolWithContext(context.Background(), func(_ context.Context, v int) bool { return *new(bool) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyBoolPtr", func(t *testing.T) {

			o2 := out.ApplyBoolPtrWithContext(context.Background(), func(_ context.Context, v int) *bool { return *new(*bool) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyBoolArray", func(t *testing.T) {

			o2 := out.ApplyBoolArrayWithContext(context.Background(), func(_ context.Context, v int) []bool { return *new([]bool) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyBoolMap", func(t *testing.T) {

			o2 := out.ApplyBoolMapWithContext(context.Background(), func(_ context.Context, v int) map[string]bool { return *new(map[string]bool) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyBoolArrayMap", func(t *testing.T) {

			o2 := out.ApplyBoolArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]bool { return *new(map[string][]bool) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyBoolMapArray", func(t *testing.T) {

			o2 := out.ApplyBoolMapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]bool { return *new([]map[string]bool) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyBoolMapMap", func(t *testing.T) {

			o2 := out.ApplyBoolMapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]bool { return *new(map[string]map[string]bool) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyBoolArrayArray", func(t *testing.T) {

			o2 := out.ApplyBoolArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]bool { return *new([][]bool) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat64", func(t *testing.T) {

			o2 := out.ApplyFloat64WithContext(context.Background(), func(_ context.Context, v int) float64 { return *new(float64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat64Ptr", func(t *testing.T) {

			o2 := out.ApplyFloat64PtrWithContext(context.Background(), func(_ context.Context, v int) *float64 { return *new(*float64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat64Array", func(t *testing.T) {

			o2 := out.ApplyFloat64ArrayWithContext(context.Background(), func(_ context.Context, v int) []float64 { return *new([]float64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat64Map", func(t *testing.T) {

			o2 := out.ApplyFloat64MapWithContext(context.Background(), func(_ context.Context, v int) map[string]float64 { return *new(map[string]float64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat64ArrayMap", func(t *testing.T) {

			o2 := out.ApplyFloat64ArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]float64 { return *new(map[string][]float64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat64MapArray", func(t *testing.T) {

			o2 := out.ApplyFloat64MapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]float64 { return *new([]map[string]float64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat64MapMap", func(t *testing.T) {

			o2 := out.ApplyFloat64MapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]float64 {
				return *new(map[string]map[string]float64)
			})
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat64ArrayArray", func(t *testing.T) {

			o2 := out.ApplyFloat64ArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]float64 { return *new([][]float64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyID", func(t *testing.T) {

			o2 := out.ApplyIDWithContext(context.Background(), func(_ context.Context, v int) ID { return *new(ID) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIDPtr", func(t *testing.T) {

			o2 := out.ApplyIDPtrWithContext(context.Background(), func(_ context.Context, v int) *ID { return *new(*ID) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIDArray", func(t *testing.T) {

			o2 := out.ApplyIDArrayWithContext(context.Background(), func(_ context.Context, v int) []ID { return *new([]ID) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIDMap", func(t *testing.T) {

			o2 := out.ApplyIDMapWithContext(context.Background(), func(_ context.Context, v int) map[string]ID { return *new(map[string]ID) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIDArrayMap", func(t *testing.T) {

			o2 := out.ApplyIDArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]ID { return *new(map[string][]ID) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIDMapArray", func(t *testing.T) {

			o2 := out.ApplyIDMapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]ID { return *new([]map[string]ID) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIDMapMap", func(t *testing.T) {

			o2 := out.ApplyIDMapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]ID { return *new(map[string]map[string]ID) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIDArrayArray", func(t *testing.T) {

			o2 := out.ApplyIDArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]ID { return *new([][]ID) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyArray", func(t *testing.T) {

			o2 := out.ApplyArrayWithContext(context.Background(), func(_ context.Context, v int) []interface{} { return *new([]interface{}) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyMap", func(t *testing.T) {

			o2 := out.ApplyMapWithContext(context.Background(), func(_ context.Context, v int) map[string]interface{} { return *new(map[string]interface{}) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyArrayMap", func(t *testing.T) {

			o2 := out.ApplyArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]interface{} { return *new(map[string][]interface{}) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyMapArray", func(t *testing.T) {

			o2 := out.ApplyMapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]interface{} { return *new([]map[string]interface{}) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyMapMap", func(t *testing.T) {

			o2 := out.ApplyMapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]interface{} {
				return *new(map[string]map[string]interface{})
			})
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyArrayArray", func(t *testing.T) {

			o2 := out.ApplyArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]interface{} { return *new([][]interface{}) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt", func(t *testing.T) {

			o2 := out.ApplyIntWithContext(context.Background(), func(_ context.Context, v int) int { return *new(int) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIntPtr", func(t *testing.T) {

			o2 := out.ApplyIntPtrWithContext(context.Background(), func(_ context.Context, v int) *int { return *new(*int) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIntArray", func(t *testing.T) {

			o2 := out.ApplyIntArrayWithContext(context.Background(), func(_ context.Context, v int) []int { return *new([]int) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIntMap", func(t *testing.T) {

			o2 := out.ApplyIntMapWithContext(context.Background(), func(_ context.Context, v int) map[string]int { return *new(map[string]int) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIntArrayMap", func(t *testing.T) {

			o2 := out.ApplyIntArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]int { return *new(map[string][]int) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIntMapArray", func(t *testing.T) {

			o2 := out.ApplyIntMapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]int { return *new([]map[string]int) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIntMapMap", func(t *testing.T) {

			o2 := out.ApplyIntMapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]int { return *new(map[string]map[string]int) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIntArrayArray", func(t *testing.T) {

			o2 := out.ApplyIntArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]int { return *new([][]int) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyString", func(t *testing.T) {

			o2 := out.ApplyStringWithContext(context.Background(), func(_ context.Context, v int) string { return *new(string) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyStringPtr", func(t *testing.T) {

			o2 := out.ApplyStringPtrWithContext(context.Background(), func(_ context.Context, v int) *string { return *new(*string) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyStringArray", func(t *testing.T) {

			o2 := out.ApplyStringArrayWithContext(context.Background(), func(_ context.Context, v int) []string { return *new([]string) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyStringMap", func(t *testing.T) {

			o2 := out.ApplyStringMapWithContext(context.Background(), func(_ context.Context, v int) map[string]string { return *new(map[string]string) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyStringArrayMap", func(t *testing.T) {

			o2 := out.ApplyStringArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]string { return *new(map[string][]string) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyStringMapArray", func(t *testing.T) {

			o2 := out.ApplyStringMapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]string { return *new([]map[string]string) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyStringMapMap", func(t *testing.T) {

			o2 := out.ApplyStringMapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]string { return *new(map[string]map[string]string) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyStringArrayArray", func(t *testing.T) {

			o2 := out.ApplyStringArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]string { return *new([][]string) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyURN", func(t *testing.T) {

			o2 := out.ApplyURNWithContext(context.Background(), func(_ context.Context, v int) URN { return *new(URN) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyURNPtr", func(t *testing.T) {

			o2 := out.ApplyURNPtrWithContext(context.Background(), func(_ context.Context, v int) *URN { return *new(*URN) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyURNArray", func(t *testing.T) {

			o2 := out.ApplyURNArrayWithContext(context.Background(), func(_ context.Context, v int) []URN { return *new([]URN) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyURNMap", func(t *testing.T) {

			o2 := out.ApplyURNMapWithContext(context.Background(), func(_ context.Context, v int) map[string]URN { return *new(map[string]URN) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyURNArrayMap", func(t *testing.T) {

			o2 := out.ApplyURNArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]URN { return *new(map[string][]URN) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyURNMapArray", func(t *testing.T) {

			o2 := out.ApplyURNMapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]URN { return *new([]map[string]URN) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyURNMapMap", func(t *testing.T) {

			o2 := out.ApplyURNMapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]URN { return *new(map[string]map[string]URN) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyURNArrayArray", func(t *testing.T) {

			o2 := out.ApplyURNArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]URN { return *new([][]URN) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

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

		out2 := StringOutput{newOutputState(reflect.TypeOf(""))}
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

		res5 := All(res3, res4).Apply(func(v interface{}) (interface{}, error) {
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

	out := in.ToArchiveOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveOutputWithContext(context.Background())

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

	out := in.ToArchiveArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveArrayOutputWithContext(context.Background())

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

	out := in.ToArchiveMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveMapOutputWithContext(context.Background())

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

	out := in.ToArchiveArrayMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveArrayMapOutputWithContext(context.Background())

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

	out := in.ToArchiveMapArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveMapArrayOutputWithContext(context.Background())

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

	out := in.ToArchiveMapMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveMapMapOutputWithContext(context.Background())

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

	out := in.ToArchiveArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveArrayArrayOutputWithContext(context.Background())

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

	out := in.ToAssetOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOutputWithContext(context.Background())

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

	out := in.ToAssetArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetArrayOutputWithContext(context.Background())

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

	out := in.ToAssetMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetMapOutputWithContext(context.Background())

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

	out := in.ToAssetArrayMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetArrayMapOutputWithContext(context.Background())

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

	out := in.ToAssetMapArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetMapArrayOutputWithContext(context.Background())

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

	out := in.ToAssetMapMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetMapMapOutputWithContext(context.Background())

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

	out := in.ToAssetArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetArrayArrayOutputWithContext(context.Background())

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

	out := in.ToAssetOrArchiveOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveOutputWithContext(context.Background())

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

	out := in.ToAssetOrArchiveArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveArrayOutputWithContext(context.Background())

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

	out := in.ToAssetOrArchiveMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveMapOutputWithContext(context.Background())

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

	out := in.ToAssetOrArchiveArrayMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveArrayMapOutputWithContext(context.Background())

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

	out := in.ToAssetOrArchiveMapArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveMapArrayOutputWithContext(context.Background())

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

	out := in.ToAssetOrArchiveMapMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveMapMapOutputWithContext(context.Background())

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

	out := in.ToAssetOrArchiveArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveArrayArrayOutputWithContext(context.Background())

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

	out := in.ToBoolOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolOutputWithContext(context.Background())

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

	out := in.ToBoolPtrOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolPtrOutputWithContext(context.Background())

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

	out := in.ToBoolArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolArrayOutputWithContext(context.Background())

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

	out := in.ToBoolMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolMapOutputWithContext(context.Background())

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

	out := in.ToBoolArrayMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolArrayMapOutputWithContext(context.Background())

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

	out := in.ToBoolMapArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolMapArrayOutputWithContext(context.Background())

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

	out := in.ToBoolMapMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolMapMapOutputWithContext(context.Background())

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

	out := in.ToBoolArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolArrayArrayOutputWithContext(context.Background())

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

	out := in.ToFloat64OutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64OutputWithContext(context.Background())

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

	out := in.ToFloat64PtrOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64PtrOutputWithContext(context.Background())

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

	out := in.ToFloat64ArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64ArrayOutputWithContext(context.Background())

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

	out := in.ToFloat64MapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64MapOutputWithContext(context.Background())

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

	out := in.ToFloat64ArrayMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64ArrayMapOutputWithContext(context.Background())

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

	out := in.ToFloat64MapArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64MapArrayOutputWithContext(context.Background())

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

	out := in.ToFloat64MapMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64MapMapOutputWithContext(context.Background())

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

	out := in.ToFloat64ArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64ArrayArrayOutputWithContext(context.Background())

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

	out := in.ToIDOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDOutputWithContext(context.Background())

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

	out := in.ToIDPtrOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDPtrOutputWithContext(context.Background())

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

	out := in.ToIDArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDArrayOutputWithContext(context.Background())

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

	out := in.ToIDMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDMapOutputWithContext(context.Background())

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

	out := in.ToIDArrayMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDArrayMapOutputWithContext(context.Background())

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

	out := in.ToIDMapArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDMapArrayOutputWithContext(context.Background())

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

	out := in.ToIDMapMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDMapMapOutputWithContext(context.Background())

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

	out := in.ToIDArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDArrayArrayOutputWithContext(context.Background())

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

	out := in.ToArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArrayOutputWithContext(context.Background())

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

	out := in.ToMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToMapOutputWithContext(context.Background())

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

	out := in.ToArrayMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArrayMapOutputWithContext(context.Background())

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

	out := in.ToMapArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToMapArrayOutputWithContext(context.Background())

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

	out := in.ToMapMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToMapMapOutputWithContext(context.Background())

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

	out := in.ToArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArrayArrayOutputWithContext(context.Background())

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

func TestToIntOutput(t *testing.T) {
	in := IntInput(Int(42))

	out := in.ToIntOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntOutputWithContext(context.Background())

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

	out := in.ToIntPtrOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntPtrOutputWithContext(context.Background())

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

	out := in.ToIntArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntArrayOutputWithContext(context.Background())

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

	out := in.ToIntMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntMapOutputWithContext(context.Background())

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

	out := in.ToIntArrayMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntArrayMapOutputWithContext(context.Background())

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

	out := in.ToIntMapArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntMapArrayOutputWithContext(context.Background())

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

	out := in.ToIntMapMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntMapMapOutputWithContext(context.Background())

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

	out := in.ToIntArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntArrayArrayOutputWithContext(context.Background())

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

	out := in.ToStringOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringOutputWithContext(context.Background())

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

	out := in.ToStringPtrOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringPtrOutputWithContext(context.Background())

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

	out := in.ToStringArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringArrayOutputWithContext(context.Background())

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

	out := in.ToStringMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringMapOutputWithContext(context.Background())

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

	out := in.ToStringArrayMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringArrayMapOutputWithContext(context.Background())

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

	out := in.ToStringMapArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringMapArrayOutputWithContext(context.Background())

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

	out := in.ToStringMapMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringMapMapOutputWithContext(context.Background())

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

	out := in.ToStringArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringArrayArrayOutputWithContext(context.Background())

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

	out := in.ToURNOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNOutputWithContext(context.Background())

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

	out := in.ToURNPtrOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNPtrOutputWithContext(context.Background())

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

	out := in.ToURNArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNArrayOutputWithContext(context.Background())

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

	out := in.ToURNMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNMapOutputWithContext(context.Background())

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

	out := in.ToURNArrayMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNArrayMapOutputWithContext(context.Background())

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

	out := in.ToURNMapArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNMapArrayOutputWithContext(context.Background())

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

	out := in.ToURNMapMapOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNMapMapOutputWithContext(context.Background())

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

	out := in.ToURNArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNArrayArrayOutputWithContext(context.Background())

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
	assetOrArchiveOut := archiveIn.ToAssetOrArchiveOutputWithContext(context.Background())
	archiveV, known, _, _, err := await(assetOrArchiveOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, archiveIn, archiveV)

	archiveOut := archiveIn.ToArchiveOutputWithContext(context.Background())
	assetOrArchiveOut = archiveOut.ToAssetOrArchiveOutputWithContext(context.Background())
	archiveV, known, _, _, err = await(assetOrArchiveOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, archiveIn, archiveV)

	assetIn := NewFileAsset("foo.zip")
	assetOrArchiveOut = assetIn.ToAssetOrArchiveOutputWithContext(context.Background())
	assetV, known, _, _, err := await(assetOrArchiveOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, assetIn, assetV)

	assetOut := assetIn.ToAssetOutputWithContext(context.Background())
	assetOrArchiveOut = assetOut.ToAssetOrArchiveOutputWithContext(context.Background())
	assetV, known, _, _, err = await(assetOrArchiveOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, assetIn, assetV)

	idIn := ID("foo")
	stringOut := idIn.ToStringOutputWithContext(context.Background())
	stringV, known, _, _, err := await(stringOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, string(idIn), stringV)

	idOut := idIn.ToIDOutputWithContext(context.Background())
	stringOut = idOut.ToStringOutputWithContext(context.Background())
	stringV, known, _, _, err = await(stringOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, string(idIn), stringV)

	urnIn := URN("foo")
	stringOut = urnIn.ToStringOutputWithContext(context.Background())
	stringV, known, _, _, err = await(stringOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, string(urnIn), stringV)

	urnOut := urnIn.ToURNOutputWithContext(context.Background())
	stringOut = urnOut.ToStringOutputWithContext(context.Background())
	stringV, known, _, _, err = await(stringOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, string(urnIn), stringV)
}

// Test pointer types.

func TestBoolPtrElem(t *testing.T) {
	out := (BoolPtr(bool(Bool(true)))).ToBoolPtrOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*bool)), iv)
}

func TestFloat64PtrElem(t *testing.T) {
	out := (Float64Ptr(float64(Float64(999.9)))).ToFloat64PtrOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*float64)), iv)
}

func TestIDPtrElem(t *testing.T) {
	out := (IDPtr(ID(ID("foo")))).ToIDPtrOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*ID)), iv)
}

func TestIntPtrElem(t *testing.T) {
	out := (IntPtr(int(Int(42)))).ToIntPtrOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*int)), iv)
}

func TestStringPtrElem(t *testing.T) {
	out := (StringPtr(string(String("foo")))).ToStringPtrOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*string)), iv)
}

func TestURNPtrElem(t *testing.T) {
	out := (URNPtr(URN(URN("foo")))).ToURNPtrOutputWithContext(context.Background())

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
	out := (ArchiveArray{NewFileArchive("foo.zip")}).ToArchiveArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]Archive)[0], iv)
}

func TestArchiveMapArrayIndex(t *testing.T) {
	out := (ArchiveMapArray{ArchiveMap{"baz": NewFileArchive("foo.zip")}}).ToArchiveMapArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]Archive)[0], iv)
}

func TestArchiveArrayArrayIndex(t *testing.T) {
	out := (ArchiveArrayArray{ArchiveArray{NewFileArchive("foo.zip")}}).ToArchiveArrayArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]Archive)[0], iv)
}

func TestAssetArrayIndex(t *testing.T) {
	out := (AssetArray{NewFileAsset("foo.txt")}).ToAssetArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]Asset)[0], iv)
}

func TestAssetMapArrayIndex(t *testing.T) {
	out := (AssetMapArray{AssetMap{"baz": NewFileAsset("foo.txt")}}).ToAssetMapArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]Asset)[0], iv)
}

func TestAssetArrayArrayIndex(t *testing.T) {
	out := (AssetArrayArray{AssetArray{NewFileAsset("foo.txt")}}).ToAssetArrayArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]Asset)[0], iv)
}

func TestAssetOrArchiveArrayIndex(t *testing.T) {
	out := (AssetOrArchiveArray{NewFileArchive("foo.zip")}).ToAssetOrArchiveArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]AssetOrArchive)[0], iv)
}

func TestAssetOrArchiveMapArrayIndex(t *testing.T) {
	out := (AssetOrArchiveMapArray{AssetOrArchiveMap{"baz": NewFileArchive("foo.zip")}}).ToAssetOrArchiveMapArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]AssetOrArchive)[0], iv)
}

func TestAssetOrArchiveArrayArrayIndex(t *testing.T) {
	out := (AssetOrArchiveArrayArray{AssetOrArchiveArray{NewFileArchive("foo.zip")}}).ToAssetOrArchiveArrayArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]AssetOrArchive)[0], iv)
}

func TestBoolArrayIndex(t *testing.T) {
	out := (BoolArray{Bool(true)}).ToBoolArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]bool)[0], iv)
}

func TestBoolMapArrayIndex(t *testing.T) {
	out := (BoolMapArray{BoolMap{"baz": Bool(true)}}).ToBoolMapArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]bool)[0], iv)
}

func TestBoolArrayArrayIndex(t *testing.T) {
	out := (BoolArrayArray{BoolArray{Bool(true)}}).ToBoolArrayArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]bool)[0], iv)
}

func TestFloat64ArrayIndex(t *testing.T) {
	out := (Float64Array{Float64(999.9)}).ToFloat64ArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]float64)[0], iv)
}

func TestFloat64MapArrayIndex(t *testing.T) {
	out := (Float64MapArray{Float64Map{"baz": Float64(999.9)}}).ToFloat64MapArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]float64)[0], iv)
}

func TestFloat64ArrayArrayIndex(t *testing.T) {
	out := (Float64ArrayArray{Float64Array{Float64(999.9)}}).ToFloat64ArrayArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]float64)[0], iv)
}

func TestIDArrayIndex(t *testing.T) {
	out := (IDArray{ID("foo")}).ToIDArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]ID)[0], iv)
}

func TestIDMapArrayIndex(t *testing.T) {
	out := (IDMapArray{IDMap{"baz": ID("foo")}}).ToIDMapArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]ID)[0], iv)
}

func TestIDArrayArrayIndex(t *testing.T) {
	out := (IDArrayArray{IDArray{ID("foo")}}).ToIDArrayArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]ID)[0], iv)
}

func TestArrayIndex(t *testing.T) {
	out := (Array{String("any")}).ToArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]interface{})[0], iv)
}

func TestMapArrayIndex(t *testing.T) {
	out := (MapArray{Map{"baz": String("any")}}).ToMapArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]interface{})[0], iv)
}

func TestArrayArrayIndex(t *testing.T) {
	out := (ArrayArray{Array{String("any")}}).ToArrayArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]interface{})[0], iv)
}

func TestIntArrayIndex(t *testing.T) {
	out := (IntArray{Int(42)}).ToIntArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]int)[0], iv)
}

func TestIntMapArrayIndex(t *testing.T) {
	out := (IntMapArray{IntMap{"baz": Int(42)}}).ToIntMapArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]int)[0], iv)
}

func TestIntArrayArrayIndex(t *testing.T) {
	out := (IntArrayArray{IntArray{Int(42)}}).ToIntArrayArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]int)[0], iv)
}

func TestStringArrayIndex(t *testing.T) {
	out := (StringArray{String("foo")}).ToStringArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]string)[0], iv)
}

func TestStringMapArrayIndex(t *testing.T) {
	out := (StringMapArray{StringMap{"baz": String("foo")}}).ToStringMapArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]string)[0], iv)
}

func TestStringArrayArrayIndex(t *testing.T) {
	out := (StringArrayArray{StringArray{String("foo")}}).ToStringArrayArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]string)[0], iv)
}

func TestURNArrayIndex(t *testing.T) {
	out := (URNArray{URN("foo")}).ToURNArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]URN)[0], iv)
}

func TestURNMapArrayIndex(t *testing.T) {
	out := (URNMapArray{URNMap{"baz": URN("foo")}}).ToURNMapArrayOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]URN)[0], iv)
}

func TestURNArrayArrayIndex(t *testing.T) {
	out := (URNArrayArray{URNArray{URN("foo")}}).ToURNArrayArrayOutputWithContext(context.Background())

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
	out := (ArchiveMap{"baz": NewFileArchive("foo.zip")}).ToArchiveMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]Archive)["baz"], iv)
}

func TestArchiveArrayMapIndex(t *testing.T) {
	out := (ArchiveArrayMap{"baz": ArchiveArray{NewFileArchive("foo.zip")}}).ToArchiveArrayMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]Archive)["baz"], iv)
}

func TestArchiveMapMapIndex(t *testing.T) {
	out := (ArchiveMapMap{"baz": ArchiveMap{"baz": NewFileArchive("foo.zip")}}).ToArchiveMapMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]Archive)["baz"], iv)
}

func TestAssetMapIndex(t *testing.T) {
	out := (AssetMap{"baz": NewFileAsset("foo.txt")}).ToAssetMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]Asset)["baz"], iv)
}

func TestAssetArrayMapIndex(t *testing.T) {
	out := (AssetArrayMap{"baz": AssetArray{NewFileAsset("foo.txt")}}).ToAssetArrayMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]Asset)["baz"], iv)
}

func TestAssetMapMapIndex(t *testing.T) {
	out := (AssetMapMap{"baz": AssetMap{"baz": NewFileAsset("foo.txt")}}).ToAssetMapMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]Asset)["baz"], iv)
}

func TestAssetOrArchiveMapIndex(t *testing.T) {
	out := (AssetOrArchiveMap{"baz": NewFileArchive("foo.zip")}).ToAssetOrArchiveMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]AssetOrArchive)["baz"], iv)
}

func TestAssetOrArchiveArrayMapIndex(t *testing.T) {
	out := (AssetOrArchiveArrayMap{"baz": AssetOrArchiveArray{NewFileArchive("foo.zip")}}).ToAssetOrArchiveArrayMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]AssetOrArchive)["baz"], iv)
}

func TestAssetOrArchiveMapMapIndex(t *testing.T) {
	out := (AssetOrArchiveMapMap{"baz": AssetOrArchiveMap{"baz": NewFileArchive("foo.zip")}}).ToAssetOrArchiveMapMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]AssetOrArchive)["baz"], iv)
}

func TestBoolMapIndex(t *testing.T) {
	out := (BoolMap{"baz": Bool(true)}).ToBoolMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]bool)["baz"], iv)
}

func TestBoolArrayMapIndex(t *testing.T) {
	out := (BoolArrayMap{"baz": BoolArray{Bool(true)}}).ToBoolArrayMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]bool)["baz"], iv)
}

func TestBoolMapMapIndex(t *testing.T) {
	out := (BoolMapMap{"baz": BoolMap{"baz": Bool(true)}}).ToBoolMapMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]bool)["baz"], iv)
}

func TestFloat64MapIndex(t *testing.T) {
	out := (Float64Map{"baz": Float64(999.9)}).ToFloat64MapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]float64)["baz"], iv)
}

func TestFloat64ArrayMapIndex(t *testing.T) {
	out := (Float64ArrayMap{"baz": Float64Array{Float64(999.9)}}).ToFloat64ArrayMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]float64)["baz"], iv)
}

func TestFloat64MapMapIndex(t *testing.T) {
	out := (Float64MapMap{"baz": Float64Map{"baz": Float64(999.9)}}).ToFloat64MapMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]float64)["baz"], iv)
}

func TestIDMapIndex(t *testing.T) {
	out := (IDMap{"baz": ID("foo")}).ToIDMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]ID)["baz"], iv)
}

func TestIDArrayMapIndex(t *testing.T) {
	out := (IDArrayMap{"baz": IDArray{ID("foo")}}).ToIDArrayMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]ID)["baz"], iv)
}

func TestIDMapMapIndex(t *testing.T) {
	out := (IDMapMap{"baz": IDMap{"baz": ID("foo")}}).ToIDMapMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]ID)["baz"], iv)
}

func TestMapIndex(t *testing.T) {
	out := (Map{"baz": String("any")}).ToMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]interface{})["baz"], iv)
}

func TestArrayMapIndex(t *testing.T) {
	out := (ArrayMap{"baz": Array{String("any")}}).ToArrayMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]interface{})["baz"], iv)
}

func TestMapMapIndex(t *testing.T) {
	out := (MapMap{"baz": Map{"baz": String("any")}}).ToMapMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]interface{})["baz"], iv)
}

func TestIntMapIndex(t *testing.T) {
	out := (IntMap{"baz": Int(42)}).ToIntMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]int)["baz"], iv)
}

func TestIntArrayMapIndex(t *testing.T) {
	out := (IntArrayMap{"baz": IntArray{Int(42)}}).ToIntArrayMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]int)["baz"], iv)
}

func TestIntMapMapIndex(t *testing.T) {
	out := (IntMapMap{"baz": IntMap{"baz": Int(42)}}).ToIntMapMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]int)["baz"], iv)
}

func TestStringMapIndex(t *testing.T) {
	out := (StringMap{"baz": String("foo")}).ToStringMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]string)["baz"], iv)
}

func TestStringArrayMapIndex(t *testing.T) {
	out := (StringArrayMap{"baz": StringArray{String("foo")}}).ToStringArrayMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]string)["baz"], iv)
}

func TestStringMapMapIndex(t *testing.T) {
	out := (StringMapMap{"baz": StringMap{"baz": String("foo")}}).ToStringMapMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]string)["baz"], iv)
}

func TestURNMapIndex(t *testing.T) {
	out := (URNMap{"baz": URN("foo")}).ToURNMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]URN)["baz"], iv)
}

func TestURNArrayMapIndex(t *testing.T) {
	out := (URNArrayMap{"baz": URNArray{URN("foo")}}).ToURNArrayMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]URN)["baz"], iv)
}

func TestURNMapMapIndex(t *testing.T) {
	out := (URNMapMap{"baz": URNMap{"baz": URN("foo")}}).ToURNMapMapOutputWithContext(context.Background())

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]URN)["baz"], iv)
}
