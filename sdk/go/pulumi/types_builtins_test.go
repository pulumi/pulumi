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
			o2 := out.ApplyArchive(func(v int) Archive { return *new(Archive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyArchiveWithContext(context.Background(), func(_ context.Context, v int) Archive { return *new(Archive) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyArchiveArray", func(t *testing.T) {
			o2 := out.ApplyArchiveArray(func(v int) []Archive { return *new([]Archive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyArchiveArrayWithContext(context.Background(), func(_ context.Context, v int) []Archive { return *new([]Archive) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyArchiveMap", func(t *testing.T) {
			o2 := out.ApplyArchiveMap(func(v int) map[string]Archive { return *new(map[string]Archive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyArchiveMapWithContext(context.Background(), func(_ context.Context, v int) map[string]Archive { return *new(map[string]Archive) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyArchiveArrayMap", func(t *testing.T) {
			o2 := out.ApplyArchiveArrayMap(func(v int) map[string][]Archive { return *new(map[string][]Archive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyArchiveArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]Archive { return *new(map[string][]Archive) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyArchiveMapArray", func(t *testing.T) {
			o2 := out.ApplyArchiveMapArray(func(v int) []map[string]Archive { return *new([]map[string]Archive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyArchiveMapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]Archive { return *new([]map[string]Archive) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyArchiveMapMap", func(t *testing.T) {
			o2 := out.ApplyArchiveMapMap(func(v int) map[string]map[string]Archive { return *new(map[string]map[string]Archive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyArchiveMapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]Archive {
				return *new(map[string]map[string]Archive)
			})
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyArchiveArrayArray", func(t *testing.T) {
			o2 := out.ApplyArchiveArrayArray(func(v int) [][]Archive { return *new([][]Archive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyArchiveArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]Archive { return *new([][]Archive) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAsset", func(t *testing.T) {
			o2 := out.ApplyAsset(func(v int) Asset { return *new(Asset) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyAssetWithContext(context.Background(), func(_ context.Context, v int) Asset { return *new(Asset) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetArray", func(t *testing.T) {
			o2 := out.ApplyAssetArray(func(v int) []Asset { return *new([]Asset) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyAssetArrayWithContext(context.Background(), func(_ context.Context, v int) []Asset { return *new([]Asset) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetMap", func(t *testing.T) {
			o2 := out.ApplyAssetMap(func(v int) map[string]Asset { return *new(map[string]Asset) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyAssetMapWithContext(context.Background(), func(_ context.Context, v int) map[string]Asset { return *new(map[string]Asset) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetArrayMap", func(t *testing.T) {
			o2 := out.ApplyAssetArrayMap(func(v int) map[string][]Asset { return *new(map[string][]Asset) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyAssetArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]Asset { return *new(map[string][]Asset) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetMapArray", func(t *testing.T) {
			o2 := out.ApplyAssetMapArray(func(v int) []map[string]Asset { return *new([]map[string]Asset) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyAssetMapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]Asset { return *new([]map[string]Asset) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetMapMap", func(t *testing.T) {
			o2 := out.ApplyAssetMapMap(func(v int) map[string]map[string]Asset { return *new(map[string]map[string]Asset) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyAssetMapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]Asset { return *new(map[string]map[string]Asset) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetArrayArray", func(t *testing.T) {
			o2 := out.ApplyAssetArrayArray(func(v int) [][]Asset { return *new([][]Asset) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyAssetArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]Asset { return *new([][]Asset) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetOrArchive", func(t *testing.T) {
			o2 := out.ApplyAssetOrArchive(func(v int) AssetOrArchive { return *new(AssetOrArchive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyAssetOrArchiveWithContext(context.Background(), func(_ context.Context, v int) AssetOrArchive { return *new(AssetOrArchive) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetOrArchiveArray", func(t *testing.T) {
			o2 := out.ApplyAssetOrArchiveArray(func(v int) []AssetOrArchive { return *new([]AssetOrArchive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyAssetOrArchiveArrayWithContext(context.Background(), func(_ context.Context, v int) []AssetOrArchive { return *new([]AssetOrArchive) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetOrArchiveMap", func(t *testing.T) {
			o2 := out.ApplyAssetOrArchiveMap(func(v int) map[string]AssetOrArchive { return *new(map[string]AssetOrArchive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyAssetOrArchiveMapWithContext(context.Background(), func(_ context.Context, v int) map[string]AssetOrArchive { return *new(map[string]AssetOrArchive) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetOrArchiveArrayMap", func(t *testing.T) {
			o2 := out.ApplyAssetOrArchiveArrayMap(func(v int) map[string][]AssetOrArchive { return *new(map[string][]AssetOrArchive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyAssetOrArchiveArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]AssetOrArchive { return *new(map[string][]AssetOrArchive) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetOrArchiveMapArray", func(t *testing.T) {
			o2 := out.ApplyAssetOrArchiveMapArray(func(v int) []map[string]AssetOrArchive { return *new([]map[string]AssetOrArchive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyAssetOrArchiveMapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]AssetOrArchive { return *new([]map[string]AssetOrArchive) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetOrArchiveMapMap", func(t *testing.T) {
			o2 := out.ApplyAssetOrArchiveMapMap(func(v int) map[string]map[string]AssetOrArchive { return *new(map[string]map[string]AssetOrArchive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyAssetOrArchiveMapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]AssetOrArchive {
				return *new(map[string]map[string]AssetOrArchive)
			})
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetOrArchiveArrayArray", func(t *testing.T) {
			o2 := out.ApplyAssetOrArchiveArrayArray(func(v int) [][]AssetOrArchive { return *new([][]AssetOrArchive) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyAssetOrArchiveArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]AssetOrArchive { return *new([][]AssetOrArchive) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyBool", func(t *testing.T) {
			o2 := out.ApplyBool(func(v int) bool { return *new(bool) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyBoolWithContext(context.Background(), func(_ context.Context, v int) bool { return *new(bool) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyBoolPtr", func(t *testing.T) {
			o2 := out.ApplyBoolPtr(func(v int) *bool { return *new(*bool) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyBoolPtrWithContext(context.Background(), func(_ context.Context, v int) *bool { return *new(*bool) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyBoolArray", func(t *testing.T) {
			o2 := out.ApplyBoolArray(func(v int) []bool { return *new([]bool) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyBoolArrayWithContext(context.Background(), func(_ context.Context, v int) []bool { return *new([]bool) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyBoolMap", func(t *testing.T) {
			o2 := out.ApplyBoolMap(func(v int) map[string]bool { return *new(map[string]bool) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyBoolMapWithContext(context.Background(), func(_ context.Context, v int) map[string]bool { return *new(map[string]bool) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyBoolArrayMap", func(t *testing.T) {
			o2 := out.ApplyBoolArrayMap(func(v int) map[string][]bool { return *new(map[string][]bool) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyBoolArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]bool { return *new(map[string][]bool) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyBoolMapArray", func(t *testing.T) {
			o2 := out.ApplyBoolMapArray(func(v int) []map[string]bool { return *new([]map[string]bool) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyBoolMapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]bool { return *new([]map[string]bool) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyBoolMapMap", func(t *testing.T) {
			o2 := out.ApplyBoolMapMap(func(v int) map[string]map[string]bool { return *new(map[string]map[string]bool) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyBoolMapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]bool { return *new(map[string]map[string]bool) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyBoolArrayArray", func(t *testing.T) {
			o2 := out.ApplyBoolArrayArray(func(v int) [][]bool { return *new([][]bool) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyBoolArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]bool { return *new([][]bool) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat32", func(t *testing.T) {
			o2 := out.ApplyFloat32(func(v int) float32 { return *new(float32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat32WithContext(context.Background(), func(_ context.Context, v int) float32 { return *new(float32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat32Ptr", func(t *testing.T) {
			o2 := out.ApplyFloat32Ptr(func(v int) *float32 { return *new(*float32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat32PtrWithContext(context.Background(), func(_ context.Context, v int) *float32 { return *new(*float32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat32Array", func(t *testing.T) {
			o2 := out.ApplyFloat32Array(func(v int) []float32 { return *new([]float32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat32ArrayWithContext(context.Background(), func(_ context.Context, v int) []float32 { return *new([]float32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat32Map", func(t *testing.T) {
			o2 := out.ApplyFloat32Map(func(v int) map[string]float32 { return *new(map[string]float32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat32MapWithContext(context.Background(), func(_ context.Context, v int) map[string]float32 { return *new(map[string]float32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat32ArrayMap", func(t *testing.T) {
			o2 := out.ApplyFloat32ArrayMap(func(v int) map[string][]float32 { return *new(map[string][]float32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat32ArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]float32 { return *new(map[string][]float32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat32MapArray", func(t *testing.T) {
			o2 := out.ApplyFloat32MapArray(func(v int) []map[string]float32 { return *new([]map[string]float32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat32MapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]float32 { return *new([]map[string]float32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat32MapMap", func(t *testing.T) {
			o2 := out.ApplyFloat32MapMap(func(v int) map[string]map[string]float32 { return *new(map[string]map[string]float32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat32MapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]float32 {
				return *new(map[string]map[string]float32)
			})
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat32ArrayArray", func(t *testing.T) {
			o2 := out.ApplyFloat32ArrayArray(func(v int) [][]float32 { return *new([][]float32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat32ArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]float32 { return *new([][]float32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat64", func(t *testing.T) {
			o2 := out.ApplyFloat64(func(v int) float64 { return *new(float64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat64WithContext(context.Background(), func(_ context.Context, v int) float64 { return *new(float64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat64Ptr", func(t *testing.T) {
			o2 := out.ApplyFloat64Ptr(func(v int) *float64 { return *new(*float64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat64PtrWithContext(context.Background(), func(_ context.Context, v int) *float64 { return *new(*float64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat64Array", func(t *testing.T) {
			o2 := out.ApplyFloat64Array(func(v int) []float64 { return *new([]float64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat64ArrayWithContext(context.Background(), func(_ context.Context, v int) []float64 { return *new([]float64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat64Map", func(t *testing.T) {
			o2 := out.ApplyFloat64Map(func(v int) map[string]float64 { return *new(map[string]float64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat64MapWithContext(context.Background(), func(_ context.Context, v int) map[string]float64 { return *new(map[string]float64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat64ArrayMap", func(t *testing.T) {
			o2 := out.ApplyFloat64ArrayMap(func(v int) map[string][]float64 { return *new(map[string][]float64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat64ArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]float64 { return *new(map[string][]float64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat64MapArray", func(t *testing.T) {
			o2 := out.ApplyFloat64MapArray(func(v int) []map[string]float64 { return *new([]map[string]float64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat64MapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]float64 { return *new([]map[string]float64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat64MapMap", func(t *testing.T) {
			o2 := out.ApplyFloat64MapMap(func(v int) map[string]map[string]float64 { return *new(map[string]map[string]float64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat64MapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]float64 {
				return *new(map[string]map[string]float64)
			})
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat64ArrayArray", func(t *testing.T) {
			o2 := out.ApplyFloat64ArrayArray(func(v int) [][]float64 { return *new([][]float64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat64ArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]float64 { return *new([][]float64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyID", func(t *testing.T) {
			o2 := out.ApplyID(func(v int) ID { return *new(ID) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIDWithContext(context.Background(), func(_ context.Context, v int) ID { return *new(ID) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIDPtr", func(t *testing.T) {
			o2 := out.ApplyIDPtr(func(v int) *ID { return *new(*ID) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIDPtrWithContext(context.Background(), func(_ context.Context, v int) *ID { return *new(*ID) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIDArray", func(t *testing.T) {
			o2 := out.ApplyIDArray(func(v int) []ID { return *new([]ID) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIDArrayWithContext(context.Background(), func(_ context.Context, v int) []ID { return *new([]ID) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIDMap", func(t *testing.T) {
			o2 := out.ApplyIDMap(func(v int) map[string]ID { return *new(map[string]ID) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIDMapWithContext(context.Background(), func(_ context.Context, v int) map[string]ID { return *new(map[string]ID) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIDArrayMap", func(t *testing.T) {
			o2 := out.ApplyIDArrayMap(func(v int) map[string][]ID { return *new(map[string][]ID) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIDArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]ID { return *new(map[string][]ID) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIDMapArray", func(t *testing.T) {
			o2 := out.ApplyIDMapArray(func(v int) []map[string]ID { return *new([]map[string]ID) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIDMapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]ID { return *new([]map[string]ID) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIDMapMap", func(t *testing.T) {
			o2 := out.ApplyIDMapMap(func(v int) map[string]map[string]ID { return *new(map[string]map[string]ID) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIDMapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]ID { return *new(map[string]map[string]ID) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIDArrayArray", func(t *testing.T) {
			o2 := out.ApplyIDArrayArray(func(v int) [][]ID { return *new([][]ID) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIDArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]ID { return *new([][]ID) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyArray", func(t *testing.T) {
			o2 := out.ApplyArray(func(v int) []interface{} { return *new([]interface{}) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyArrayWithContext(context.Background(), func(_ context.Context, v int) []interface{} { return *new([]interface{}) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyMap", func(t *testing.T) {
			o2 := out.ApplyMap(func(v int) map[string]interface{} { return *new(map[string]interface{}) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyMapWithContext(context.Background(), func(_ context.Context, v int) map[string]interface{} { return *new(map[string]interface{}) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyArrayMap", func(t *testing.T) {
			o2 := out.ApplyArrayMap(func(v int) map[string][]interface{} { return *new(map[string][]interface{}) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]interface{} { return *new(map[string][]interface{}) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyMapArray", func(t *testing.T) {
			o2 := out.ApplyMapArray(func(v int) []map[string]interface{} { return *new([]map[string]interface{}) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyMapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]interface{} { return *new([]map[string]interface{}) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyMapMap", func(t *testing.T) {
			o2 := out.ApplyMapMap(func(v int) map[string]map[string]interface{} { return *new(map[string]map[string]interface{}) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyMapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]interface{} {
				return *new(map[string]map[string]interface{})
			})
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyArrayArray", func(t *testing.T) {
			o2 := out.ApplyArrayArray(func(v int) [][]interface{} { return *new([][]interface{}) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]interface{} { return *new([][]interface{}) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt", func(t *testing.T) {
			o2 := out.ApplyInt(func(v int) int { return *new(int) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIntWithContext(context.Background(), func(_ context.Context, v int) int { return *new(int) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIntPtr", func(t *testing.T) {
			o2 := out.ApplyIntPtr(func(v int) *int { return *new(*int) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIntPtrWithContext(context.Background(), func(_ context.Context, v int) *int { return *new(*int) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIntArray", func(t *testing.T) {
			o2 := out.ApplyIntArray(func(v int) []int { return *new([]int) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIntArrayWithContext(context.Background(), func(_ context.Context, v int) []int { return *new([]int) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIntMap", func(t *testing.T) {
			o2 := out.ApplyIntMap(func(v int) map[string]int { return *new(map[string]int) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIntMapWithContext(context.Background(), func(_ context.Context, v int) map[string]int { return *new(map[string]int) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIntArrayMap", func(t *testing.T) {
			o2 := out.ApplyIntArrayMap(func(v int) map[string][]int { return *new(map[string][]int) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIntArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]int { return *new(map[string][]int) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIntMapArray", func(t *testing.T) {
			o2 := out.ApplyIntMapArray(func(v int) []map[string]int { return *new([]map[string]int) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIntMapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]int { return *new([]map[string]int) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIntMapMap", func(t *testing.T) {
			o2 := out.ApplyIntMapMap(func(v int) map[string]map[string]int { return *new(map[string]map[string]int) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIntMapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]int { return *new(map[string]map[string]int) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIntArrayArray", func(t *testing.T) {
			o2 := out.ApplyIntArrayArray(func(v int) [][]int { return *new([][]int) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIntArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]int { return *new([][]int) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt16", func(t *testing.T) {
			o2 := out.ApplyInt16(func(v int) int16 { return *new(int16) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt16WithContext(context.Background(), func(_ context.Context, v int) int16 { return *new(int16) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt16Ptr", func(t *testing.T) {
			o2 := out.ApplyInt16Ptr(func(v int) *int16 { return *new(*int16) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt16PtrWithContext(context.Background(), func(_ context.Context, v int) *int16 { return *new(*int16) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt16Array", func(t *testing.T) {
			o2 := out.ApplyInt16Array(func(v int) []int16 { return *new([]int16) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt16ArrayWithContext(context.Background(), func(_ context.Context, v int) []int16 { return *new([]int16) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt16Map", func(t *testing.T) {
			o2 := out.ApplyInt16Map(func(v int) map[string]int16 { return *new(map[string]int16) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt16MapWithContext(context.Background(), func(_ context.Context, v int) map[string]int16 { return *new(map[string]int16) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt16ArrayMap", func(t *testing.T) {
			o2 := out.ApplyInt16ArrayMap(func(v int) map[string][]int16 { return *new(map[string][]int16) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt16ArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]int16 { return *new(map[string][]int16) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt16MapArray", func(t *testing.T) {
			o2 := out.ApplyInt16MapArray(func(v int) []map[string]int16 { return *new([]map[string]int16) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt16MapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]int16 { return *new([]map[string]int16) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt16MapMap", func(t *testing.T) {
			o2 := out.ApplyInt16MapMap(func(v int) map[string]map[string]int16 { return *new(map[string]map[string]int16) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt16MapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]int16 { return *new(map[string]map[string]int16) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt16ArrayArray", func(t *testing.T) {
			o2 := out.ApplyInt16ArrayArray(func(v int) [][]int16 { return *new([][]int16) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt16ArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]int16 { return *new([][]int16) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt32", func(t *testing.T) {
			o2 := out.ApplyInt32(func(v int) int32 { return *new(int32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt32WithContext(context.Background(), func(_ context.Context, v int) int32 { return *new(int32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt32Ptr", func(t *testing.T) {
			o2 := out.ApplyInt32Ptr(func(v int) *int32 { return *new(*int32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt32PtrWithContext(context.Background(), func(_ context.Context, v int) *int32 { return *new(*int32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt32Array", func(t *testing.T) {
			o2 := out.ApplyInt32Array(func(v int) []int32 { return *new([]int32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt32ArrayWithContext(context.Background(), func(_ context.Context, v int) []int32 { return *new([]int32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt32Map", func(t *testing.T) {
			o2 := out.ApplyInt32Map(func(v int) map[string]int32 { return *new(map[string]int32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt32MapWithContext(context.Background(), func(_ context.Context, v int) map[string]int32 { return *new(map[string]int32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt32ArrayMap", func(t *testing.T) {
			o2 := out.ApplyInt32ArrayMap(func(v int) map[string][]int32 { return *new(map[string][]int32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt32ArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]int32 { return *new(map[string][]int32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt32MapArray", func(t *testing.T) {
			o2 := out.ApplyInt32MapArray(func(v int) []map[string]int32 { return *new([]map[string]int32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt32MapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]int32 { return *new([]map[string]int32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt32MapMap", func(t *testing.T) {
			o2 := out.ApplyInt32MapMap(func(v int) map[string]map[string]int32 { return *new(map[string]map[string]int32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt32MapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]int32 { return *new(map[string]map[string]int32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt32ArrayArray", func(t *testing.T) {
			o2 := out.ApplyInt32ArrayArray(func(v int) [][]int32 { return *new([][]int32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt32ArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]int32 { return *new([][]int32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt64", func(t *testing.T) {
			o2 := out.ApplyInt64(func(v int) int64 { return *new(int64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt64WithContext(context.Background(), func(_ context.Context, v int) int64 { return *new(int64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt64Ptr", func(t *testing.T) {
			o2 := out.ApplyInt64Ptr(func(v int) *int64 { return *new(*int64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt64PtrWithContext(context.Background(), func(_ context.Context, v int) *int64 { return *new(*int64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt64Array", func(t *testing.T) {
			o2 := out.ApplyInt64Array(func(v int) []int64 { return *new([]int64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt64ArrayWithContext(context.Background(), func(_ context.Context, v int) []int64 { return *new([]int64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt64Map", func(t *testing.T) {
			o2 := out.ApplyInt64Map(func(v int) map[string]int64 { return *new(map[string]int64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt64MapWithContext(context.Background(), func(_ context.Context, v int) map[string]int64 { return *new(map[string]int64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt64ArrayMap", func(t *testing.T) {
			o2 := out.ApplyInt64ArrayMap(func(v int) map[string][]int64 { return *new(map[string][]int64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt64ArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]int64 { return *new(map[string][]int64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt64MapArray", func(t *testing.T) {
			o2 := out.ApplyInt64MapArray(func(v int) []map[string]int64 { return *new([]map[string]int64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt64MapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]int64 { return *new([]map[string]int64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt64MapMap", func(t *testing.T) {
			o2 := out.ApplyInt64MapMap(func(v int) map[string]map[string]int64 { return *new(map[string]map[string]int64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt64MapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]int64 { return *new(map[string]map[string]int64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt64ArrayArray", func(t *testing.T) {
			o2 := out.ApplyInt64ArrayArray(func(v int) [][]int64 { return *new([][]int64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt64ArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]int64 { return *new([][]int64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt8", func(t *testing.T) {
			o2 := out.ApplyInt8(func(v int) int8 { return *new(int8) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt8WithContext(context.Background(), func(_ context.Context, v int) int8 { return *new(int8) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt8Ptr", func(t *testing.T) {
			o2 := out.ApplyInt8Ptr(func(v int) *int8 { return *new(*int8) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt8PtrWithContext(context.Background(), func(_ context.Context, v int) *int8 { return *new(*int8) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt8Array", func(t *testing.T) {
			o2 := out.ApplyInt8Array(func(v int) []int8 { return *new([]int8) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt8ArrayWithContext(context.Background(), func(_ context.Context, v int) []int8 { return *new([]int8) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt8Map", func(t *testing.T) {
			o2 := out.ApplyInt8Map(func(v int) map[string]int8 { return *new(map[string]int8) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt8MapWithContext(context.Background(), func(_ context.Context, v int) map[string]int8 { return *new(map[string]int8) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt8ArrayMap", func(t *testing.T) {
			o2 := out.ApplyInt8ArrayMap(func(v int) map[string][]int8 { return *new(map[string][]int8) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt8ArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]int8 { return *new(map[string][]int8) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt8MapArray", func(t *testing.T) {
			o2 := out.ApplyInt8MapArray(func(v int) []map[string]int8 { return *new([]map[string]int8) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt8MapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]int8 { return *new([]map[string]int8) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt8MapMap", func(t *testing.T) {
			o2 := out.ApplyInt8MapMap(func(v int) map[string]map[string]int8 { return *new(map[string]map[string]int8) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt8MapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]int8 { return *new(map[string]map[string]int8) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt8ArrayArray", func(t *testing.T) {
			o2 := out.ApplyInt8ArrayArray(func(v int) [][]int8 { return *new([][]int8) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt8ArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]int8 { return *new([][]int8) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyString", func(t *testing.T) {
			o2 := out.ApplyString(func(v int) string { return *new(string) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyStringWithContext(context.Background(), func(_ context.Context, v int) string { return *new(string) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyStringPtr", func(t *testing.T) {
			o2 := out.ApplyStringPtr(func(v int) *string { return *new(*string) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyStringPtrWithContext(context.Background(), func(_ context.Context, v int) *string { return *new(*string) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyStringArray", func(t *testing.T) {
			o2 := out.ApplyStringArray(func(v int) []string { return *new([]string) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyStringArrayWithContext(context.Background(), func(_ context.Context, v int) []string { return *new([]string) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyStringMap", func(t *testing.T) {
			o2 := out.ApplyStringMap(func(v int) map[string]string { return *new(map[string]string) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyStringMapWithContext(context.Background(), func(_ context.Context, v int) map[string]string { return *new(map[string]string) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyStringArrayMap", func(t *testing.T) {
			o2 := out.ApplyStringArrayMap(func(v int) map[string][]string { return *new(map[string][]string) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyStringArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]string { return *new(map[string][]string) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyStringMapArray", func(t *testing.T) {
			o2 := out.ApplyStringMapArray(func(v int) []map[string]string { return *new([]map[string]string) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyStringMapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]string { return *new([]map[string]string) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyStringMapMap", func(t *testing.T) {
			o2 := out.ApplyStringMapMap(func(v int) map[string]map[string]string { return *new(map[string]map[string]string) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyStringMapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]string { return *new(map[string]map[string]string) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyStringArrayArray", func(t *testing.T) {
			o2 := out.ApplyStringArrayArray(func(v int) [][]string { return *new([][]string) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyStringArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]string { return *new([][]string) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyURN", func(t *testing.T) {
			o2 := out.ApplyURN(func(v int) URN { return *new(URN) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyURNWithContext(context.Background(), func(_ context.Context, v int) URN { return *new(URN) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyURNPtr", func(t *testing.T) {
			o2 := out.ApplyURNPtr(func(v int) *URN { return *new(*URN) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyURNPtrWithContext(context.Background(), func(_ context.Context, v int) *URN { return *new(*URN) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyURNArray", func(t *testing.T) {
			o2 := out.ApplyURNArray(func(v int) []URN { return *new([]URN) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyURNArrayWithContext(context.Background(), func(_ context.Context, v int) []URN { return *new([]URN) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyURNMap", func(t *testing.T) {
			o2 := out.ApplyURNMap(func(v int) map[string]URN { return *new(map[string]URN) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyURNMapWithContext(context.Background(), func(_ context.Context, v int) map[string]URN { return *new(map[string]URN) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyURNArrayMap", func(t *testing.T) {
			o2 := out.ApplyURNArrayMap(func(v int) map[string][]URN { return *new(map[string][]URN) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyURNArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]URN { return *new(map[string][]URN) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyURNMapArray", func(t *testing.T) {
			o2 := out.ApplyURNMapArray(func(v int) []map[string]URN { return *new([]map[string]URN) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyURNMapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]URN { return *new([]map[string]URN) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyURNMapMap", func(t *testing.T) {
			o2 := out.ApplyURNMapMap(func(v int) map[string]map[string]URN { return *new(map[string]map[string]URN) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyURNMapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]URN { return *new(map[string]map[string]URN) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyURNArrayArray", func(t *testing.T) {
			o2 := out.ApplyURNArrayArray(func(v int) [][]URN { return *new([][]URN) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyURNArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]URN { return *new([][]URN) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint", func(t *testing.T) {
			o2 := out.ApplyUint(func(v int) uint { return *new(uint) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUintWithContext(context.Background(), func(_ context.Context, v int) uint { return *new(uint) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUintPtr", func(t *testing.T) {
			o2 := out.ApplyUintPtr(func(v int) *uint { return *new(*uint) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUintPtrWithContext(context.Background(), func(_ context.Context, v int) *uint { return *new(*uint) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUintArray", func(t *testing.T) {
			o2 := out.ApplyUintArray(func(v int) []uint { return *new([]uint) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUintArrayWithContext(context.Background(), func(_ context.Context, v int) []uint { return *new([]uint) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUintMap", func(t *testing.T) {
			o2 := out.ApplyUintMap(func(v int) map[string]uint { return *new(map[string]uint) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUintMapWithContext(context.Background(), func(_ context.Context, v int) map[string]uint { return *new(map[string]uint) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUintArrayMap", func(t *testing.T) {
			o2 := out.ApplyUintArrayMap(func(v int) map[string][]uint { return *new(map[string][]uint) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUintArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]uint { return *new(map[string][]uint) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUintMapArray", func(t *testing.T) {
			o2 := out.ApplyUintMapArray(func(v int) []map[string]uint { return *new([]map[string]uint) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUintMapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]uint { return *new([]map[string]uint) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUintMapMap", func(t *testing.T) {
			o2 := out.ApplyUintMapMap(func(v int) map[string]map[string]uint { return *new(map[string]map[string]uint) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUintMapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]uint { return *new(map[string]map[string]uint) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUintArrayArray", func(t *testing.T) {
			o2 := out.ApplyUintArrayArray(func(v int) [][]uint { return *new([][]uint) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUintArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]uint { return *new([][]uint) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint16", func(t *testing.T) {
			o2 := out.ApplyUint16(func(v int) uint16 { return *new(uint16) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint16WithContext(context.Background(), func(_ context.Context, v int) uint16 { return *new(uint16) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint16Ptr", func(t *testing.T) {
			o2 := out.ApplyUint16Ptr(func(v int) *uint16 { return *new(*uint16) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint16PtrWithContext(context.Background(), func(_ context.Context, v int) *uint16 { return *new(*uint16) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint16Array", func(t *testing.T) {
			o2 := out.ApplyUint16Array(func(v int) []uint16 { return *new([]uint16) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint16ArrayWithContext(context.Background(), func(_ context.Context, v int) []uint16 { return *new([]uint16) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint16Map", func(t *testing.T) {
			o2 := out.ApplyUint16Map(func(v int) map[string]uint16 { return *new(map[string]uint16) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint16MapWithContext(context.Background(), func(_ context.Context, v int) map[string]uint16 { return *new(map[string]uint16) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint16ArrayMap", func(t *testing.T) {
			o2 := out.ApplyUint16ArrayMap(func(v int) map[string][]uint16 { return *new(map[string][]uint16) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint16ArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]uint16 { return *new(map[string][]uint16) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint16MapArray", func(t *testing.T) {
			o2 := out.ApplyUint16MapArray(func(v int) []map[string]uint16 { return *new([]map[string]uint16) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint16MapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]uint16 { return *new([]map[string]uint16) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint16MapMap", func(t *testing.T) {
			o2 := out.ApplyUint16MapMap(func(v int) map[string]map[string]uint16 { return *new(map[string]map[string]uint16) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint16MapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]uint16 { return *new(map[string]map[string]uint16) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint16ArrayArray", func(t *testing.T) {
			o2 := out.ApplyUint16ArrayArray(func(v int) [][]uint16 { return *new([][]uint16) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint16ArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]uint16 { return *new([][]uint16) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint32", func(t *testing.T) {
			o2 := out.ApplyUint32(func(v int) uint32 { return *new(uint32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint32WithContext(context.Background(), func(_ context.Context, v int) uint32 { return *new(uint32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint32Ptr", func(t *testing.T) {
			o2 := out.ApplyUint32Ptr(func(v int) *uint32 { return *new(*uint32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint32PtrWithContext(context.Background(), func(_ context.Context, v int) *uint32 { return *new(*uint32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint32Array", func(t *testing.T) {
			o2 := out.ApplyUint32Array(func(v int) []uint32 { return *new([]uint32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint32ArrayWithContext(context.Background(), func(_ context.Context, v int) []uint32 { return *new([]uint32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint32Map", func(t *testing.T) {
			o2 := out.ApplyUint32Map(func(v int) map[string]uint32 { return *new(map[string]uint32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint32MapWithContext(context.Background(), func(_ context.Context, v int) map[string]uint32 { return *new(map[string]uint32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint32ArrayMap", func(t *testing.T) {
			o2 := out.ApplyUint32ArrayMap(func(v int) map[string][]uint32 { return *new(map[string][]uint32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint32ArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]uint32 { return *new(map[string][]uint32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint32MapArray", func(t *testing.T) {
			o2 := out.ApplyUint32MapArray(func(v int) []map[string]uint32 { return *new([]map[string]uint32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint32MapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]uint32 { return *new([]map[string]uint32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint32MapMap", func(t *testing.T) {
			o2 := out.ApplyUint32MapMap(func(v int) map[string]map[string]uint32 { return *new(map[string]map[string]uint32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint32MapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]uint32 { return *new(map[string]map[string]uint32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint32ArrayArray", func(t *testing.T) {
			o2 := out.ApplyUint32ArrayArray(func(v int) [][]uint32 { return *new([][]uint32) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint32ArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]uint32 { return *new([][]uint32) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint64", func(t *testing.T) {
			o2 := out.ApplyUint64(func(v int) uint64 { return *new(uint64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint64WithContext(context.Background(), func(_ context.Context, v int) uint64 { return *new(uint64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint64Ptr", func(t *testing.T) {
			o2 := out.ApplyUint64Ptr(func(v int) *uint64 { return *new(*uint64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint64PtrWithContext(context.Background(), func(_ context.Context, v int) *uint64 { return *new(*uint64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint64Array", func(t *testing.T) {
			o2 := out.ApplyUint64Array(func(v int) []uint64 { return *new([]uint64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint64ArrayWithContext(context.Background(), func(_ context.Context, v int) []uint64 { return *new([]uint64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint64Map", func(t *testing.T) {
			o2 := out.ApplyUint64Map(func(v int) map[string]uint64 { return *new(map[string]uint64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint64MapWithContext(context.Background(), func(_ context.Context, v int) map[string]uint64 { return *new(map[string]uint64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint64ArrayMap", func(t *testing.T) {
			o2 := out.ApplyUint64ArrayMap(func(v int) map[string][]uint64 { return *new(map[string][]uint64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint64ArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]uint64 { return *new(map[string][]uint64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint64MapArray", func(t *testing.T) {
			o2 := out.ApplyUint64MapArray(func(v int) []map[string]uint64 { return *new([]map[string]uint64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint64MapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]uint64 { return *new([]map[string]uint64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint64MapMap", func(t *testing.T) {
			o2 := out.ApplyUint64MapMap(func(v int) map[string]map[string]uint64 { return *new(map[string]map[string]uint64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint64MapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]uint64 { return *new(map[string]map[string]uint64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint64ArrayArray", func(t *testing.T) {
			o2 := out.ApplyUint64ArrayArray(func(v int) [][]uint64 { return *new([][]uint64) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint64ArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]uint64 { return *new([][]uint64) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint8", func(t *testing.T) {
			o2 := out.ApplyUint8(func(v int) uint8 { return *new(uint8) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint8WithContext(context.Background(), func(_ context.Context, v int) uint8 { return *new(uint8) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint8Ptr", func(t *testing.T) {
			o2 := out.ApplyUint8Ptr(func(v int) *uint8 { return *new(*uint8) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint8PtrWithContext(context.Background(), func(_ context.Context, v int) *uint8 { return *new(*uint8) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint8Array", func(t *testing.T) {
			o2 := out.ApplyUint8Array(func(v int) []uint8 { return *new([]uint8) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint8ArrayWithContext(context.Background(), func(_ context.Context, v int) []uint8 { return *new([]uint8) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint8Map", func(t *testing.T) {
			o2 := out.ApplyUint8Map(func(v int) map[string]uint8 { return *new(map[string]uint8) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint8MapWithContext(context.Background(), func(_ context.Context, v int) map[string]uint8 { return *new(map[string]uint8) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint8ArrayMap", func(t *testing.T) {
			o2 := out.ApplyUint8ArrayMap(func(v int) map[string][]uint8 { return *new(map[string][]uint8) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint8ArrayMapWithContext(context.Background(), func(_ context.Context, v int) map[string][]uint8 { return *new(map[string][]uint8) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint8MapArray", func(t *testing.T) {
			o2 := out.ApplyUint8MapArray(func(v int) []map[string]uint8 { return *new([]map[string]uint8) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint8MapArrayWithContext(context.Background(), func(_ context.Context, v int) []map[string]uint8 { return *new([]map[string]uint8) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint8MapMap", func(t *testing.T) {
			o2 := out.ApplyUint8MapMap(func(v int) map[string]map[string]uint8 { return *new(map[string]map[string]uint8) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint8MapMapWithContext(context.Background(), func(_ context.Context, v int) map[string]map[string]uint8 { return *new(map[string]map[string]uint8) })
			_, known, _, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint8ArrayArray", func(t *testing.T) {
			o2 := out.ApplyUint8ArrayArray(func(v int) [][]uint8 { return *new([][]uint8) })
			_, known, _, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint8ArrayArrayWithContext(context.Background(), func(_ context.Context, v int) [][]uint8 { return *new([][]uint8) })
			_, known, _, _, err = await(o2)
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

		t.Run("ApplyT::Float32ArrayMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string][]float32 { return *new(map[string][]float32) }).(Float32ArrayMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Float32MapArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []map[string]float32 { return *new([]map[string]float32) }).(Float32MapArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Float32MapMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]map[string]float32 { return *new(map[string]map[string]float32) }).(Float32MapMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Float32ArrayArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) [][]float32 { return *new([][]float32) }).(Float32ArrayArrayOutput)
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

		t.Run("ApplyT::Int16ArrayMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string][]int16 { return *new(map[string][]int16) }).(Int16ArrayMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int16MapArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []map[string]int16 { return *new([]map[string]int16) }).(Int16MapArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int16MapMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]map[string]int16 { return *new(map[string]map[string]int16) }).(Int16MapMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int16ArrayArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) [][]int16 { return *new([][]int16) }).(Int16ArrayArrayOutput)
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

		t.Run("ApplyT::Int32ArrayMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string][]int32 { return *new(map[string][]int32) }).(Int32ArrayMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int32MapArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []map[string]int32 { return *new([]map[string]int32) }).(Int32MapArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int32MapMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]map[string]int32 { return *new(map[string]map[string]int32) }).(Int32MapMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int32ArrayArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) [][]int32 { return *new([][]int32) }).(Int32ArrayArrayOutput)
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

		t.Run("ApplyT::Int64ArrayMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string][]int64 { return *new(map[string][]int64) }).(Int64ArrayMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int64MapArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []map[string]int64 { return *new([]map[string]int64) }).(Int64MapArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int64MapMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]map[string]int64 { return *new(map[string]map[string]int64) }).(Int64MapMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int64ArrayArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) [][]int64 { return *new([][]int64) }).(Int64ArrayArrayOutput)
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

		t.Run("ApplyT::Int8ArrayMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string][]int8 { return *new(map[string][]int8) }).(Int8ArrayMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int8MapArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []map[string]int8 { return *new([]map[string]int8) }).(Int8MapArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int8MapMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]map[string]int8 { return *new(map[string]map[string]int8) }).(Int8MapMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Int8ArrayArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) [][]int8 { return *new([][]int8) }).(Int8ArrayArrayOutput)
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

		t.Run("ApplyT::UintArrayMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string][]uint { return *new(map[string][]uint) }).(UintArrayMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::UintMapArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []map[string]uint { return *new([]map[string]uint) }).(UintMapArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::UintMapMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]map[string]uint { return *new(map[string]map[string]uint) }).(UintMapMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::UintArrayArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) [][]uint { return *new([][]uint) }).(UintArrayArrayOutput)
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

		t.Run("ApplyT::Uint16ArrayMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string][]uint16 { return *new(map[string][]uint16) }).(Uint16ArrayMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint16MapArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []map[string]uint16 { return *new([]map[string]uint16) }).(Uint16MapArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint16MapMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]map[string]uint16 { return *new(map[string]map[string]uint16) }).(Uint16MapMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint16ArrayArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) [][]uint16 { return *new([][]uint16) }).(Uint16ArrayArrayOutput)
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

		t.Run("ApplyT::Uint32ArrayMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string][]uint32 { return *new(map[string][]uint32) }).(Uint32ArrayMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint32MapArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []map[string]uint32 { return *new([]map[string]uint32) }).(Uint32MapArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint32MapMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]map[string]uint32 { return *new(map[string]map[string]uint32) }).(Uint32MapMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint32ArrayArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) [][]uint32 { return *new([][]uint32) }).(Uint32ArrayArrayOutput)
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

		t.Run("ApplyT::Uint64ArrayMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string][]uint64 { return *new(map[string][]uint64) }).(Uint64ArrayMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint64MapArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []map[string]uint64 { return *new([]map[string]uint64) }).(Uint64MapArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint64MapMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]map[string]uint64 { return *new(map[string]map[string]uint64) }).(Uint64MapMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint64ArrayArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) [][]uint64 { return *new([][]uint64) }).(Uint64ArrayArrayOutput)
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

		t.Run("ApplyT::Uint8ArrayMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string][]uint8 { return *new(map[string][]uint8) }).(Uint8ArrayMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint8MapArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) []map[string]uint8 { return *new([]map[string]uint8) }).(Uint8MapArrayOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint8MapMapOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) map[string]map[string]uint8 { return *new(map[string]map[string]uint8) }).(Uint8MapMapOutput)
			assert.True(t, ok)
		})

		t.Run("ApplyT::Uint8ArrayArrayOutput", func(t *testing.T) {
			_, ok := out.ApplyT(func(v int) [][]uint8 { return *new([][]uint8) }).(Uint8ArrayArrayOutput)
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

func TestToOutputFloat32(t *testing.T) {
	out := ToOutput(Float32(1.3))
	_, ok := out.(Float32Input)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float32Input)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat32Ptr(t *testing.T) {
	out := ToOutput(Float32Ptr(float32(Float32(1.3))))
	_, ok := out.(Float32PtrInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float32PtrInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat32Array(t *testing.T) {
	out := ToOutput(Float32Array{Float32(1.3)})
	_, ok := out.(Float32ArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float32ArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat32Map(t *testing.T) {
	out := ToOutput(Float32Map{"baz": Float32(1.3)})
	_, ok := out.(Float32MapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float32MapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat32ArrayMap(t *testing.T) {
	out := ToOutput(Float32ArrayMap{"baz": Float32Array{Float32(1.3)}})
	_, ok := out.(Float32ArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float32ArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat32MapArray(t *testing.T) {
	out := ToOutput(Float32MapArray{Float32Map{"baz": Float32(1.3)}})
	_, ok := out.(Float32MapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float32MapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat32MapMap(t *testing.T) {
	out := ToOutput(Float32MapMap{"baz": Float32Map{"baz": Float32(1.3)}})
	_, ok := out.(Float32MapMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float32MapMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat32ArrayArray(t *testing.T) {
	out := ToOutput(Float32ArrayArray{Float32Array{Float32(1.3)}})
	_, ok := out.(Float32ArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float32ArrayArrayInput)
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

func TestToOutputInt16(t *testing.T) {
	out := ToOutput(Int16(33))
	_, ok := out.(Int16Input)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int16Input)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt16Ptr(t *testing.T) {
	out := ToOutput(Int16Ptr(int16(Int16(33))))
	_, ok := out.(Int16PtrInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int16PtrInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt16Array(t *testing.T) {
	out := ToOutput(Int16Array{Int16(33)})
	_, ok := out.(Int16ArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int16ArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt16Map(t *testing.T) {
	out := ToOutput(Int16Map{"baz": Int16(33)})
	_, ok := out.(Int16MapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int16MapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt16ArrayMap(t *testing.T) {
	out := ToOutput(Int16ArrayMap{"baz": Int16Array{Int16(33)}})
	_, ok := out.(Int16ArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int16ArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt16MapArray(t *testing.T) {
	out := ToOutput(Int16MapArray{Int16Map{"baz": Int16(33)}})
	_, ok := out.(Int16MapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int16MapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt16MapMap(t *testing.T) {
	out := ToOutput(Int16MapMap{"baz": Int16Map{"baz": Int16(33)}})
	_, ok := out.(Int16MapMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int16MapMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt16ArrayArray(t *testing.T) {
	out := ToOutput(Int16ArrayArray{Int16Array{Int16(33)}})
	_, ok := out.(Int16ArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int16ArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt32(t *testing.T) {
	out := ToOutput(Int32(24))
	_, ok := out.(Int32Input)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int32Input)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt32Ptr(t *testing.T) {
	out := ToOutput(Int32Ptr(int32(Int32(24))))
	_, ok := out.(Int32PtrInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int32PtrInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt32Array(t *testing.T) {
	out := ToOutput(Int32Array{Int32(24)})
	_, ok := out.(Int32ArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int32ArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt32Map(t *testing.T) {
	out := ToOutput(Int32Map{"baz": Int32(24)})
	_, ok := out.(Int32MapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int32MapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt32ArrayMap(t *testing.T) {
	out := ToOutput(Int32ArrayMap{"baz": Int32Array{Int32(24)}})
	_, ok := out.(Int32ArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int32ArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt32MapArray(t *testing.T) {
	out := ToOutput(Int32MapArray{Int32Map{"baz": Int32(24)}})
	_, ok := out.(Int32MapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int32MapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt32MapMap(t *testing.T) {
	out := ToOutput(Int32MapMap{"baz": Int32Map{"baz": Int32(24)}})
	_, ok := out.(Int32MapMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int32MapMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt32ArrayArray(t *testing.T) {
	out := ToOutput(Int32ArrayArray{Int32Array{Int32(24)}})
	_, ok := out.(Int32ArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int32ArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt64(t *testing.T) {
	out := ToOutput(Int64(15))
	_, ok := out.(Int64Input)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int64Input)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt64Ptr(t *testing.T) {
	out := ToOutput(Int64Ptr(int64(Int64(15))))
	_, ok := out.(Int64PtrInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int64PtrInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt64Array(t *testing.T) {
	out := ToOutput(Int64Array{Int64(15)})
	_, ok := out.(Int64ArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int64ArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt64Map(t *testing.T) {
	out := ToOutput(Int64Map{"baz": Int64(15)})
	_, ok := out.(Int64MapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int64MapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt64ArrayMap(t *testing.T) {
	out := ToOutput(Int64ArrayMap{"baz": Int64Array{Int64(15)}})
	_, ok := out.(Int64ArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int64ArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt64MapArray(t *testing.T) {
	out := ToOutput(Int64MapArray{Int64Map{"baz": Int64(15)}})
	_, ok := out.(Int64MapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int64MapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt64MapMap(t *testing.T) {
	out := ToOutput(Int64MapMap{"baz": Int64Map{"baz": Int64(15)}})
	_, ok := out.(Int64MapMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int64MapMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt64ArrayArray(t *testing.T) {
	out := ToOutput(Int64ArrayArray{Int64Array{Int64(15)}})
	_, ok := out.(Int64ArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int64ArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt8(t *testing.T) {
	out := ToOutput(Int8(6))
	_, ok := out.(Int8Input)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int8Input)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt8Ptr(t *testing.T) {
	out := ToOutput(Int8Ptr(int8(Int8(6))))
	_, ok := out.(Int8PtrInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int8PtrInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt8Array(t *testing.T) {
	out := ToOutput(Int8Array{Int8(6)})
	_, ok := out.(Int8ArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int8ArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt8Map(t *testing.T) {
	out := ToOutput(Int8Map{"baz": Int8(6)})
	_, ok := out.(Int8MapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int8MapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt8ArrayMap(t *testing.T) {
	out := ToOutput(Int8ArrayMap{"baz": Int8Array{Int8(6)}})
	_, ok := out.(Int8ArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int8ArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt8MapArray(t *testing.T) {
	out := ToOutput(Int8MapArray{Int8Map{"baz": Int8(6)}})
	_, ok := out.(Int8MapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int8MapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt8MapMap(t *testing.T) {
	out := ToOutput(Int8MapMap{"baz": Int8Map{"baz": Int8(6)}})
	_, ok := out.(Int8MapMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int8MapMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt8ArrayArray(t *testing.T) {
	out := ToOutput(Int8ArrayArray{Int8Array{Int8(6)}})
	_, ok := out.(Int8ArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int8ArrayArrayInput)
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

func TestToOutputUint(t *testing.T) {
	out := ToOutput(Uint(42))
	_, ok := out.(UintInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(UintInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUintPtr(t *testing.T) {
	out := ToOutput(UintPtr(uint(Uint(42))))
	_, ok := out.(UintPtrInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(UintPtrInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUintArray(t *testing.T) {
	out := ToOutput(UintArray{Uint(42)})
	_, ok := out.(UintArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(UintArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUintMap(t *testing.T) {
	out := ToOutput(UintMap{"baz": Uint(42)})
	_, ok := out.(UintMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(UintMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUintArrayMap(t *testing.T) {
	out := ToOutput(UintArrayMap{"baz": UintArray{Uint(42)}})
	_, ok := out.(UintArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(UintArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUintMapArray(t *testing.T) {
	out := ToOutput(UintMapArray{UintMap{"baz": Uint(42)}})
	_, ok := out.(UintMapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(UintMapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUintMapMap(t *testing.T) {
	out := ToOutput(UintMapMap{"baz": UintMap{"baz": Uint(42)}})
	_, ok := out.(UintMapMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(UintMapMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUintArrayArray(t *testing.T) {
	out := ToOutput(UintArrayArray{UintArray{Uint(42)}})
	_, ok := out.(UintArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(UintArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint16(t *testing.T) {
	out := ToOutput(Uint16(33))
	_, ok := out.(Uint16Input)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint16Input)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint16Ptr(t *testing.T) {
	out := ToOutput(Uint16Ptr(uint16(Uint16(33))))
	_, ok := out.(Uint16PtrInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint16PtrInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint16Array(t *testing.T) {
	out := ToOutput(Uint16Array{Uint16(33)})
	_, ok := out.(Uint16ArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint16ArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint16Map(t *testing.T) {
	out := ToOutput(Uint16Map{"baz": Uint16(33)})
	_, ok := out.(Uint16MapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint16MapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint16ArrayMap(t *testing.T) {
	out := ToOutput(Uint16ArrayMap{"baz": Uint16Array{Uint16(33)}})
	_, ok := out.(Uint16ArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint16ArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint16MapArray(t *testing.T) {
	out := ToOutput(Uint16MapArray{Uint16Map{"baz": Uint16(33)}})
	_, ok := out.(Uint16MapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint16MapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint16MapMap(t *testing.T) {
	out := ToOutput(Uint16MapMap{"baz": Uint16Map{"baz": Uint16(33)}})
	_, ok := out.(Uint16MapMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint16MapMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint16ArrayArray(t *testing.T) {
	out := ToOutput(Uint16ArrayArray{Uint16Array{Uint16(33)}})
	_, ok := out.(Uint16ArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint16ArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint32(t *testing.T) {
	out := ToOutput(Uint32(24))
	_, ok := out.(Uint32Input)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint32Input)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint32Ptr(t *testing.T) {
	out := ToOutput(Uint32Ptr(uint32(Uint32(24))))
	_, ok := out.(Uint32PtrInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint32PtrInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint32Array(t *testing.T) {
	out := ToOutput(Uint32Array{Uint32(24)})
	_, ok := out.(Uint32ArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint32ArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint32Map(t *testing.T) {
	out := ToOutput(Uint32Map{"baz": Uint32(24)})
	_, ok := out.(Uint32MapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint32MapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint32ArrayMap(t *testing.T) {
	out := ToOutput(Uint32ArrayMap{"baz": Uint32Array{Uint32(24)}})
	_, ok := out.(Uint32ArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint32ArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint32MapArray(t *testing.T) {
	out := ToOutput(Uint32MapArray{Uint32Map{"baz": Uint32(24)}})
	_, ok := out.(Uint32MapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint32MapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint32MapMap(t *testing.T) {
	out := ToOutput(Uint32MapMap{"baz": Uint32Map{"baz": Uint32(24)}})
	_, ok := out.(Uint32MapMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint32MapMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint32ArrayArray(t *testing.T) {
	out := ToOutput(Uint32ArrayArray{Uint32Array{Uint32(24)}})
	_, ok := out.(Uint32ArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint32ArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint64(t *testing.T) {
	out := ToOutput(Uint64(15))
	_, ok := out.(Uint64Input)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint64Input)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint64Ptr(t *testing.T) {
	out := ToOutput(Uint64Ptr(uint64(Uint64(15))))
	_, ok := out.(Uint64PtrInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint64PtrInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint64Array(t *testing.T) {
	out := ToOutput(Uint64Array{Uint64(15)})
	_, ok := out.(Uint64ArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint64ArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint64Map(t *testing.T) {
	out := ToOutput(Uint64Map{"baz": Uint64(15)})
	_, ok := out.(Uint64MapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint64MapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint64ArrayMap(t *testing.T) {
	out := ToOutput(Uint64ArrayMap{"baz": Uint64Array{Uint64(15)}})
	_, ok := out.(Uint64ArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint64ArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint64MapArray(t *testing.T) {
	out := ToOutput(Uint64MapArray{Uint64Map{"baz": Uint64(15)}})
	_, ok := out.(Uint64MapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint64MapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint64MapMap(t *testing.T) {
	out := ToOutput(Uint64MapMap{"baz": Uint64Map{"baz": Uint64(15)}})
	_, ok := out.(Uint64MapMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint64MapMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint64ArrayArray(t *testing.T) {
	out := ToOutput(Uint64ArrayArray{Uint64Array{Uint64(15)}})
	_, ok := out.(Uint64ArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint64ArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint8(t *testing.T) {
	out := ToOutput(Uint8(6))
	_, ok := out.(Uint8Input)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint8Input)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint8Ptr(t *testing.T) {
	out := ToOutput(Uint8Ptr(uint8(Uint8(6))))
	_, ok := out.(Uint8PtrInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint8PtrInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint8Array(t *testing.T) {
	out := ToOutput(Uint8Array{Uint8(6)})
	_, ok := out.(Uint8ArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint8ArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint8Map(t *testing.T) {
	out := ToOutput(Uint8Map{"baz": Uint8(6)})
	_, ok := out.(Uint8MapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint8MapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint8ArrayMap(t *testing.T) {
	out := ToOutput(Uint8ArrayMap{"baz": Uint8Array{Uint8(6)}})
	_, ok := out.(Uint8ArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint8ArrayMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint8MapArray(t *testing.T) {
	out := ToOutput(Uint8MapArray{Uint8Map{"baz": Uint8(6)}})
	_, ok := out.(Uint8MapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint8MapArrayInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint8MapMap(t *testing.T) {
	out := ToOutput(Uint8MapMap{"baz": Uint8Map{"baz": Uint8(6)}})
	_, ok := out.(Uint8MapMapInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint8MapMapInput)
	assert.True(t, ok)

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint8ArrayArray(t *testing.T) {
	out := ToOutput(Uint8ArrayArray{Uint8Array{Uint8(6)}})
	_, ok := out.(Uint8ArrayArrayInput)
	assert.True(t, ok)

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint8ArrayArrayInput)
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

func TestToFloat32Output(t *testing.T) {
	in := Float32Input(Float32(1.3))

	out := in.ToFloat32Output()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32Output()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat32OutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32OutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat32PtrOutput(t *testing.T) {
	in := Float32PtrInput(Float32Ptr(float32(Float32(1.3))))

	out := in.ToFloat32PtrOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32PtrOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat32PtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32PtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat32ArrayOutput(t *testing.T) {
	in := Float32ArrayInput(Float32Array{Float32(1.3)})

	out := in.ToFloat32ArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32ArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat32ArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32ArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat32MapOutput(t *testing.T) {
	in := Float32MapInput(Float32Map{"baz": Float32(1.3)})

	out := in.ToFloat32MapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32MapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat32MapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32MapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat32ArrayMapOutput(t *testing.T) {
	in := Float32ArrayMapInput(Float32ArrayMap{"baz": Float32Array{Float32(1.3)}})

	out := in.ToFloat32ArrayMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32ArrayMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat32ArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32ArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat32MapArrayOutput(t *testing.T) {
	in := Float32MapArrayInput(Float32MapArray{Float32Map{"baz": Float32(1.3)}})

	out := in.ToFloat32MapArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32MapArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat32MapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32MapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat32MapMapOutput(t *testing.T) {
	in := Float32MapMapInput(Float32MapMap{"baz": Float32Map{"baz": Float32(1.3)}})

	out := in.ToFloat32MapMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32MapMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat32MapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32MapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat32ArrayArrayOutput(t *testing.T) {
	in := Float32ArrayArrayInput(Float32ArrayArray{Float32Array{Float32(1.3)}})

	out := in.ToFloat32ArrayArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32ArrayArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat32ArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32ArrayArrayOutputWithContext(context.Background())

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

func TestToInt16Output(t *testing.T) {
	in := Int16Input(Int16(33))

	out := in.ToInt16Output()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16Output()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt16OutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16OutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt16PtrOutput(t *testing.T) {
	in := Int16PtrInput(Int16Ptr(int16(Int16(33))))

	out := in.ToInt16PtrOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16PtrOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt16PtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16PtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt16ArrayOutput(t *testing.T) {
	in := Int16ArrayInput(Int16Array{Int16(33)})

	out := in.ToInt16ArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16ArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt16ArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16ArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt16MapOutput(t *testing.T) {
	in := Int16MapInput(Int16Map{"baz": Int16(33)})

	out := in.ToInt16MapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16MapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt16MapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16MapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt16ArrayMapOutput(t *testing.T) {
	in := Int16ArrayMapInput(Int16ArrayMap{"baz": Int16Array{Int16(33)}})

	out := in.ToInt16ArrayMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16ArrayMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt16ArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16ArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt16MapArrayOutput(t *testing.T) {
	in := Int16MapArrayInput(Int16MapArray{Int16Map{"baz": Int16(33)}})

	out := in.ToInt16MapArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16MapArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt16MapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16MapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt16MapMapOutput(t *testing.T) {
	in := Int16MapMapInput(Int16MapMap{"baz": Int16Map{"baz": Int16(33)}})

	out := in.ToInt16MapMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16MapMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt16MapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16MapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt16ArrayArrayOutput(t *testing.T) {
	in := Int16ArrayArrayInput(Int16ArrayArray{Int16Array{Int16(33)}})

	out := in.ToInt16ArrayArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16ArrayArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt16ArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16ArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt32Output(t *testing.T) {
	in := Int32Input(Int32(24))

	out := in.ToInt32Output()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32Output()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt32OutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32OutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt32PtrOutput(t *testing.T) {
	in := Int32PtrInput(Int32Ptr(int32(Int32(24))))

	out := in.ToInt32PtrOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32PtrOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt32PtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32PtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt32ArrayOutput(t *testing.T) {
	in := Int32ArrayInput(Int32Array{Int32(24)})

	out := in.ToInt32ArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32ArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt32ArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32ArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt32MapOutput(t *testing.T) {
	in := Int32MapInput(Int32Map{"baz": Int32(24)})

	out := in.ToInt32MapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32MapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt32MapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32MapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt32ArrayMapOutput(t *testing.T) {
	in := Int32ArrayMapInput(Int32ArrayMap{"baz": Int32Array{Int32(24)}})

	out := in.ToInt32ArrayMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32ArrayMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt32ArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32ArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt32MapArrayOutput(t *testing.T) {
	in := Int32MapArrayInput(Int32MapArray{Int32Map{"baz": Int32(24)}})

	out := in.ToInt32MapArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32MapArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt32MapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32MapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt32MapMapOutput(t *testing.T) {
	in := Int32MapMapInput(Int32MapMap{"baz": Int32Map{"baz": Int32(24)}})

	out := in.ToInt32MapMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32MapMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt32MapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32MapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt32ArrayArrayOutput(t *testing.T) {
	in := Int32ArrayArrayInput(Int32ArrayArray{Int32Array{Int32(24)}})

	out := in.ToInt32ArrayArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32ArrayArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt32ArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32ArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt64Output(t *testing.T) {
	in := Int64Input(Int64(15))

	out := in.ToInt64Output()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64Output()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt64OutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64OutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt64PtrOutput(t *testing.T) {
	in := Int64PtrInput(Int64Ptr(int64(Int64(15))))

	out := in.ToInt64PtrOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64PtrOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt64PtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64PtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt64ArrayOutput(t *testing.T) {
	in := Int64ArrayInput(Int64Array{Int64(15)})

	out := in.ToInt64ArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64ArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt64ArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64ArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt64MapOutput(t *testing.T) {
	in := Int64MapInput(Int64Map{"baz": Int64(15)})

	out := in.ToInt64MapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64MapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt64MapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64MapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt64ArrayMapOutput(t *testing.T) {
	in := Int64ArrayMapInput(Int64ArrayMap{"baz": Int64Array{Int64(15)}})

	out := in.ToInt64ArrayMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64ArrayMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt64ArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64ArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt64MapArrayOutput(t *testing.T) {
	in := Int64MapArrayInput(Int64MapArray{Int64Map{"baz": Int64(15)}})

	out := in.ToInt64MapArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64MapArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt64MapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64MapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt64MapMapOutput(t *testing.T) {
	in := Int64MapMapInput(Int64MapMap{"baz": Int64Map{"baz": Int64(15)}})

	out := in.ToInt64MapMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64MapMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt64MapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64MapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt64ArrayArrayOutput(t *testing.T) {
	in := Int64ArrayArrayInput(Int64ArrayArray{Int64Array{Int64(15)}})

	out := in.ToInt64ArrayArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64ArrayArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt64ArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64ArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt8Output(t *testing.T) {
	in := Int8Input(Int8(6))

	out := in.ToInt8Output()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8Output()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt8OutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8OutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt8PtrOutput(t *testing.T) {
	in := Int8PtrInput(Int8Ptr(int8(Int8(6))))

	out := in.ToInt8PtrOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8PtrOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt8PtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8PtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt8ArrayOutput(t *testing.T) {
	in := Int8ArrayInput(Int8Array{Int8(6)})

	out := in.ToInt8ArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8ArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt8ArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8ArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt8MapOutput(t *testing.T) {
	in := Int8MapInput(Int8Map{"baz": Int8(6)})

	out := in.ToInt8MapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8MapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt8MapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8MapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt8ArrayMapOutput(t *testing.T) {
	in := Int8ArrayMapInput(Int8ArrayMap{"baz": Int8Array{Int8(6)}})

	out := in.ToInt8ArrayMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8ArrayMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt8ArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8ArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt8MapArrayOutput(t *testing.T) {
	in := Int8MapArrayInput(Int8MapArray{Int8Map{"baz": Int8(6)}})

	out := in.ToInt8MapArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8MapArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt8MapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8MapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt8MapMapOutput(t *testing.T) {
	in := Int8MapMapInput(Int8MapMap{"baz": Int8Map{"baz": Int8(6)}})

	out := in.ToInt8MapMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8MapMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt8MapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8MapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt8ArrayArrayOutput(t *testing.T) {
	in := Int8ArrayArrayInput(Int8ArrayArray{Int8Array{Int8(6)}})

	out := in.ToInt8ArrayArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8ArrayArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt8ArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8ArrayArrayOutputWithContext(context.Background())

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

func TestToUintOutput(t *testing.T) {
	in := UintInput(Uint(42))

	out := in.ToUintOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUintOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUintPtrOutput(t *testing.T) {
	in := UintPtrInput(UintPtr(uint(Uint(42))))

	out := in.ToUintPtrOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintPtrOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUintPtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintPtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUintArrayOutput(t *testing.T) {
	in := UintArrayInput(UintArray{Uint(42)})

	out := in.ToUintArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUintArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUintMapOutput(t *testing.T) {
	in := UintMapInput(UintMap{"baz": Uint(42)})

	out := in.ToUintMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUintMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUintArrayMapOutput(t *testing.T) {
	in := UintArrayMapInput(UintArrayMap{"baz": UintArray{Uint(42)}})

	out := in.ToUintArrayMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintArrayMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUintArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUintMapArrayOutput(t *testing.T) {
	in := UintMapArrayInput(UintMapArray{UintMap{"baz": Uint(42)}})

	out := in.ToUintMapArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintMapArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUintMapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintMapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUintMapMapOutput(t *testing.T) {
	in := UintMapMapInput(UintMapMap{"baz": UintMap{"baz": Uint(42)}})

	out := in.ToUintMapMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintMapMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUintMapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintMapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUintArrayArrayOutput(t *testing.T) {
	in := UintArrayArrayInput(UintArrayArray{UintArray{Uint(42)}})

	out := in.ToUintArrayArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintArrayArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUintArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint16Output(t *testing.T) {
	in := Uint16Input(Uint16(33))

	out := in.ToUint16Output()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16Output()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint16OutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16OutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint16PtrOutput(t *testing.T) {
	in := Uint16PtrInput(Uint16Ptr(uint16(Uint16(33))))

	out := in.ToUint16PtrOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16PtrOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint16PtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16PtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint16ArrayOutput(t *testing.T) {
	in := Uint16ArrayInput(Uint16Array{Uint16(33)})

	out := in.ToUint16ArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16ArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint16ArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16ArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint16MapOutput(t *testing.T) {
	in := Uint16MapInput(Uint16Map{"baz": Uint16(33)})

	out := in.ToUint16MapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16MapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint16MapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16MapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint16ArrayMapOutput(t *testing.T) {
	in := Uint16ArrayMapInput(Uint16ArrayMap{"baz": Uint16Array{Uint16(33)}})

	out := in.ToUint16ArrayMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16ArrayMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint16ArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16ArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint16MapArrayOutput(t *testing.T) {
	in := Uint16MapArrayInput(Uint16MapArray{Uint16Map{"baz": Uint16(33)}})

	out := in.ToUint16MapArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16MapArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint16MapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16MapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint16MapMapOutput(t *testing.T) {
	in := Uint16MapMapInput(Uint16MapMap{"baz": Uint16Map{"baz": Uint16(33)}})

	out := in.ToUint16MapMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16MapMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint16MapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16MapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint16ArrayArrayOutput(t *testing.T) {
	in := Uint16ArrayArrayInput(Uint16ArrayArray{Uint16Array{Uint16(33)}})

	out := in.ToUint16ArrayArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16ArrayArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint16ArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16ArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint32Output(t *testing.T) {
	in := Uint32Input(Uint32(24))

	out := in.ToUint32Output()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32Output()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint32OutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32OutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint32PtrOutput(t *testing.T) {
	in := Uint32PtrInput(Uint32Ptr(uint32(Uint32(24))))

	out := in.ToUint32PtrOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32PtrOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint32PtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32PtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint32ArrayOutput(t *testing.T) {
	in := Uint32ArrayInput(Uint32Array{Uint32(24)})

	out := in.ToUint32ArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32ArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint32ArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32ArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint32MapOutput(t *testing.T) {
	in := Uint32MapInput(Uint32Map{"baz": Uint32(24)})

	out := in.ToUint32MapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32MapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint32MapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32MapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint32ArrayMapOutput(t *testing.T) {
	in := Uint32ArrayMapInput(Uint32ArrayMap{"baz": Uint32Array{Uint32(24)}})

	out := in.ToUint32ArrayMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32ArrayMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint32ArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32ArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint32MapArrayOutput(t *testing.T) {
	in := Uint32MapArrayInput(Uint32MapArray{Uint32Map{"baz": Uint32(24)}})

	out := in.ToUint32MapArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32MapArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint32MapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32MapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint32MapMapOutput(t *testing.T) {
	in := Uint32MapMapInput(Uint32MapMap{"baz": Uint32Map{"baz": Uint32(24)}})

	out := in.ToUint32MapMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32MapMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint32MapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32MapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint32ArrayArrayOutput(t *testing.T) {
	in := Uint32ArrayArrayInput(Uint32ArrayArray{Uint32Array{Uint32(24)}})

	out := in.ToUint32ArrayArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32ArrayArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint32ArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32ArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint64Output(t *testing.T) {
	in := Uint64Input(Uint64(15))

	out := in.ToUint64Output()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64Output()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint64OutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64OutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint64PtrOutput(t *testing.T) {
	in := Uint64PtrInput(Uint64Ptr(uint64(Uint64(15))))

	out := in.ToUint64PtrOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64PtrOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint64PtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64PtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint64ArrayOutput(t *testing.T) {
	in := Uint64ArrayInput(Uint64Array{Uint64(15)})

	out := in.ToUint64ArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64ArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint64ArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64ArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint64MapOutput(t *testing.T) {
	in := Uint64MapInput(Uint64Map{"baz": Uint64(15)})

	out := in.ToUint64MapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64MapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint64MapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64MapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint64ArrayMapOutput(t *testing.T) {
	in := Uint64ArrayMapInput(Uint64ArrayMap{"baz": Uint64Array{Uint64(15)}})

	out := in.ToUint64ArrayMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64ArrayMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint64ArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64ArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint64MapArrayOutput(t *testing.T) {
	in := Uint64MapArrayInput(Uint64MapArray{Uint64Map{"baz": Uint64(15)}})

	out := in.ToUint64MapArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64MapArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint64MapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64MapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint64MapMapOutput(t *testing.T) {
	in := Uint64MapMapInput(Uint64MapMap{"baz": Uint64Map{"baz": Uint64(15)}})

	out := in.ToUint64MapMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64MapMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint64MapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64MapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint64ArrayArrayOutput(t *testing.T) {
	in := Uint64ArrayArrayInput(Uint64ArrayArray{Uint64Array{Uint64(15)}})

	out := in.ToUint64ArrayArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64ArrayArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint64ArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64ArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint8Output(t *testing.T) {
	in := Uint8Input(Uint8(6))

	out := in.ToUint8Output()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8Output()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint8OutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8OutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint8PtrOutput(t *testing.T) {
	in := Uint8PtrInput(Uint8Ptr(uint8(Uint8(6))))

	out := in.ToUint8PtrOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8PtrOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint8PtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8PtrOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint8ArrayOutput(t *testing.T) {
	in := Uint8ArrayInput(Uint8Array{Uint8(6)})

	out := in.ToUint8ArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8ArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint8ArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8ArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint8MapOutput(t *testing.T) {
	in := Uint8MapInput(Uint8Map{"baz": Uint8(6)})

	out := in.ToUint8MapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8MapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint8MapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8MapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint8ArrayMapOutput(t *testing.T) {
	in := Uint8ArrayMapInput(Uint8ArrayMap{"baz": Uint8Array{Uint8(6)}})

	out := in.ToUint8ArrayMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8ArrayMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint8ArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8ArrayMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint8MapArrayOutput(t *testing.T) {
	in := Uint8MapArrayInput(Uint8MapArray{Uint8Map{"baz": Uint8(6)}})

	out := in.ToUint8MapArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8MapArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint8MapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8MapArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint8MapMapOutput(t *testing.T) {
	in := Uint8MapMapInput(Uint8MapMap{"baz": Uint8Map{"baz": Uint8(6)}})

	out := in.ToUint8MapMapOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8MapMapOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint8MapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8MapMapOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint8ArrayArrayOutput(t *testing.T) {
	in := Uint8ArrayArrayInput(Uint8ArrayArray{Uint8Array{Uint8(6)}})

	out := in.ToUint8ArrayArrayOutput()

	_, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8ArrayArrayOutput()

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint8ArrayArrayOutputWithContext(context.Background())

	_, known, _, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8ArrayArrayOutputWithContext(context.Background())

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

func TestFloat32PtrElem(t *testing.T) {
	out := (Float32Ptr(float32(Float32(1.3)))).ToFloat32PtrOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*float32)), iv)
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

func TestInt16PtrElem(t *testing.T) {
	out := (Int16Ptr(int16(Int16(33)))).ToInt16PtrOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*int16)), iv)
}

func TestInt32PtrElem(t *testing.T) {
	out := (Int32Ptr(int32(Int32(24)))).ToInt32PtrOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*int32)), iv)
}

func TestInt64PtrElem(t *testing.T) {
	out := (Int64Ptr(int64(Int64(15)))).ToInt64PtrOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*int64)), iv)
}

func TestInt8PtrElem(t *testing.T) {
	out := (Int8Ptr(int8(Int8(6)))).ToInt8PtrOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*int8)), iv)
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

func TestUintPtrElem(t *testing.T) {
	out := (UintPtr(uint(Uint(42)))).ToUintPtrOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*uint)), iv)
}

func TestUint16PtrElem(t *testing.T) {
	out := (Uint16Ptr(uint16(Uint16(33)))).ToUint16PtrOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*uint16)), iv)
}

func TestUint32PtrElem(t *testing.T) {
	out := (Uint32Ptr(uint32(Uint32(24)))).ToUint32PtrOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*uint32)), iv)
}

func TestUint64PtrElem(t *testing.T) {
	out := (Uint64Ptr(uint64(Uint64(15)))).ToUint64PtrOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*uint64)), iv)
}

func TestUint8PtrElem(t *testing.T) {
	out := (Uint8Ptr(uint8(Uint8(6)))).ToUint8PtrOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*uint8)), iv)
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
}

func TestFloat32ArrayIndex(t *testing.T) {
	out := (Float32Array{Float32(1.3)}).ToFloat32ArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]float32)[0], iv)
}

func TestFloat32MapArrayIndex(t *testing.T) {
	out := (Float32MapArray{Float32Map{"baz": Float32(1.3)}}).ToFloat32MapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]float32)[0], iv)
}

func TestFloat32ArrayArrayIndex(t *testing.T) {
	out := (Float32ArrayArray{Float32Array{Float32(1.3)}}).ToFloat32ArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]float32)[0], iv)
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
}

func TestInt16ArrayIndex(t *testing.T) {
	out := (Int16Array{Int16(33)}).ToInt16ArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]int16)[0], iv)
}

func TestInt16MapArrayIndex(t *testing.T) {
	out := (Int16MapArray{Int16Map{"baz": Int16(33)}}).ToInt16MapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]int16)[0], iv)
}

func TestInt16ArrayArrayIndex(t *testing.T) {
	out := (Int16ArrayArray{Int16Array{Int16(33)}}).ToInt16ArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]int16)[0], iv)
}

func TestInt32ArrayIndex(t *testing.T) {
	out := (Int32Array{Int32(24)}).ToInt32ArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]int32)[0], iv)
}

func TestInt32MapArrayIndex(t *testing.T) {
	out := (Int32MapArray{Int32Map{"baz": Int32(24)}}).ToInt32MapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]int32)[0], iv)
}

func TestInt32ArrayArrayIndex(t *testing.T) {
	out := (Int32ArrayArray{Int32Array{Int32(24)}}).ToInt32ArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]int32)[0], iv)
}

func TestInt64ArrayIndex(t *testing.T) {
	out := (Int64Array{Int64(15)}).ToInt64ArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]int64)[0], iv)
}

func TestInt64MapArrayIndex(t *testing.T) {
	out := (Int64MapArray{Int64Map{"baz": Int64(15)}}).ToInt64MapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]int64)[0], iv)
}

func TestInt64ArrayArrayIndex(t *testing.T) {
	out := (Int64ArrayArray{Int64Array{Int64(15)}}).ToInt64ArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]int64)[0], iv)
}

func TestInt8ArrayIndex(t *testing.T) {
	out := (Int8Array{Int8(6)}).ToInt8ArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]int8)[0], iv)
}

func TestInt8MapArrayIndex(t *testing.T) {
	out := (Int8MapArray{Int8Map{"baz": Int8(6)}}).ToInt8MapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]int8)[0], iv)
}

func TestInt8ArrayArrayIndex(t *testing.T) {
	out := (Int8ArrayArray{Int8Array{Int8(6)}}).ToInt8ArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]int8)[0], iv)
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
}

func TestUintArrayIndex(t *testing.T) {
	out := (UintArray{Uint(42)}).ToUintArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]uint)[0], iv)
}

func TestUintMapArrayIndex(t *testing.T) {
	out := (UintMapArray{UintMap{"baz": Uint(42)}}).ToUintMapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]uint)[0], iv)
}

func TestUintArrayArrayIndex(t *testing.T) {
	out := (UintArrayArray{UintArray{Uint(42)}}).ToUintArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]uint)[0], iv)
}

func TestUint16ArrayIndex(t *testing.T) {
	out := (Uint16Array{Uint16(33)}).ToUint16ArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]uint16)[0], iv)
}

func TestUint16MapArrayIndex(t *testing.T) {
	out := (Uint16MapArray{Uint16Map{"baz": Uint16(33)}}).ToUint16MapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]uint16)[0], iv)
}

func TestUint16ArrayArrayIndex(t *testing.T) {
	out := (Uint16ArrayArray{Uint16Array{Uint16(33)}}).ToUint16ArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]uint16)[0], iv)
}

func TestUint32ArrayIndex(t *testing.T) {
	out := (Uint32Array{Uint32(24)}).ToUint32ArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]uint32)[0], iv)
}

func TestUint32MapArrayIndex(t *testing.T) {
	out := (Uint32MapArray{Uint32Map{"baz": Uint32(24)}}).ToUint32MapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]uint32)[0], iv)
}

func TestUint32ArrayArrayIndex(t *testing.T) {
	out := (Uint32ArrayArray{Uint32Array{Uint32(24)}}).ToUint32ArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]uint32)[0], iv)
}

func TestUint64ArrayIndex(t *testing.T) {
	out := (Uint64Array{Uint64(15)}).ToUint64ArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]uint64)[0], iv)
}

func TestUint64MapArrayIndex(t *testing.T) {
	out := (Uint64MapArray{Uint64Map{"baz": Uint64(15)}}).ToUint64MapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]uint64)[0], iv)
}

func TestUint64ArrayArrayIndex(t *testing.T) {
	out := (Uint64ArrayArray{Uint64Array{Uint64(15)}}).ToUint64ArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]uint64)[0], iv)
}

func TestUint8ArrayIndex(t *testing.T) {
	out := (Uint8Array{Uint8(6)}).ToUint8ArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]uint8)[0], iv)
}

func TestUint8MapArrayIndex(t *testing.T) {
	out := (Uint8MapArray{Uint8Map{"baz": Uint8(6)}}).ToUint8MapArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]map[string]uint8)[0], iv)
}

func TestUint8ArrayArrayIndex(t *testing.T) {
	out := (Uint8ArrayArray{Uint8Array{Uint8(6)}}).ToUint8ArrayArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([][]uint8)[0], iv)
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
}

func TestFloat32MapIndex(t *testing.T) {
	out := (Float32Map{"baz": Float32(1.3)}).ToFloat32MapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]float32)["baz"], iv)
}

func TestFloat32ArrayMapIndex(t *testing.T) {
	out := (Float32ArrayMap{"baz": Float32Array{Float32(1.3)}}).ToFloat32ArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]float32)["baz"], iv)
}

func TestFloat32MapMapIndex(t *testing.T) {
	out := (Float32MapMap{"baz": Float32Map{"baz": Float32(1.3)}}).ToFloat32MapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]float32)["baz"], iv)
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
}

func TestInt16MapIndex(t *testing.T) {
	out := (Int16Map{"baz": Int16(33)}).ToInt16MapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]int16)["baz"], iv)
}

func TestInt16ArrayMapIndex(t *testing.T) {
	out := (Int16ArrayMap{"baz": Int16Array{Int16(33)}}).ToInt16ArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]int16)["baz"], iv)
}

func TestInt16MapMapIndex(t *testing.T) {
	out := (Int16MapMap{"baz": Int16Map{"baz": Int16(33)}}).ToInt16MapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]int16)["baz"], iv)
}

func TestInt32MapIndex(t *testing.T) {
	out := (Int32Map{"baz": Int32(24)}).ToInt32MapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]int32)["baz"], iv)
}

func TestInt32ArrayMapIndex(t *testing.T) {
	out := (Int32ArrayMap{"baz": Int32Array{Int32(24)}}).ToInt32ArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]int32)["baz"], iv)
}

func TestInt32MapMapIndex(t *testing.T) {
	out := (Int32MapMap{"baz": Int32Map{"baz": Int32(24)}}).ToInt32MapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]int32)["baz"], iv)
}

func TestInt64MapIndex(t *testing.T) {
	out := (Int64Map{"baz": Int64(15)}).ToInt64MapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]int64)["baz"], iv)
}

func TestInt64ArrayMapIndex(t *testing.T) {
	out := (Int64ArrayMap{"baz": Int64Array{Int64(15)}}).ToInt64ArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]int64)["baz"], iv)
}

func TestInt64MapMapIndex(t *testing.T) {
	out := (Int64MapMap{"baz": Int64Map{"baz": Int64(15)}}).ToInt64MapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]int64)["baz"], iv)
}

func TestInt8MapIndex(t *testing.T) {
	out := (Int8Map{"baz": Int8(6)}).ToInt8MapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]int8)["baz"], iv)
}

func TestInt8ArrayMapIndex(t *testing.T) {
	out := (Int8ArrayMap{"baz": Int8Array{Int8(6)}}).ToInt8ArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]int8)["baz"], iv)
}

func TestInt8MapMapIndex(t *testing.T) {
	out := (Int8MapMap{"baz": Int8Map{"baz": Int8(6)}}).ToInt8MapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]int8)["baz"], iv)
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
}

func TestUintMapIndex(t *testing.T) {
	out := (UintMap{"baz": Uint(42)}).ToUintMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]uint)["baz"], iv)
}

func TestUintArrayMapIndex(t *testing.T) {
	out := (UintArrayMap{"baz": UintArray{Uint(42)}}).ToUintArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]uint)["baz"], iv)
}

func TestUintMapMapIndex(t *testing.T) {
	out := (UintMapMap{"baz": UintMap{"baz": Uint(42)}}).ToUintMapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]uint)["baz"], iv)
}

func TestUint16MapIndex(t *testing.T) {
	out := (Uint16Map{"baz": Uint16(33)}).ToUint16MapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]uint16)["baz"], iv)
}

func TestUint16ArrayMapIndex(t *testing.T) {
	out := (Uint16ArrayMap{"baz": Uint16Array{Uint16(33)}}).ToUint16ArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]uint16)["baz"], iv)
}

func TestUint16MapMapIndex(t *testing.T) {
	out := (Uint16MapMap{"baz": Uint16Map{"baz": Uint16(33)}}).ToUint16MapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]uint16)["baz"], iv)
}

func TestUint32MapIndex(t *testing.T) {
	out := (Uint32Map{"baz": Uint32(24)}).ToUint32MapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]uint32)["baz"], iv)
}

func TestUint32ArrayMapIndex(t *testing.T) {
	out := (Uint32ArrayMap{"baz": Uint32Array{Uint32(24)}}).ToUint32ArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]uint32)["baz"], iv)
}

func TestUint32MapMapIndex(t *testing.T) {
	out := (Uint32MapMap{"baz": Uint32Map{"baz": Uint32(24)}}).ToUint32MapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]uint32)["baz"], iv)
}

func TestUint64MapIndex(t *testing.T) {
	out := (Uint64Map{"baz": Uint64(15)}).ToUint64MapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]uint64)["baz"], iv)
}

func TestUint64ArrayMapIndex(t *testing.T) {
	out := (Uint64ArrayMap{"baz": Uint64Array{Uint64(15)}}).ToUint64ArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]uint64)["baz"], iv)
}

func TestUint64MapMapIndex(t *testing.T) {
	out := (Uint64MapMap{"baz": Uint64Map{"baz": Uint64(15)}}).ToUint64MapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]uint64)["baz"], iv)
}

func TestUint8MapIndex(t *testing.T) {
	out := (Uint8Map{"baz": Uint8(6)}).ToUint8MapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]uint8)["baz"], iv)
}

func TestUint8ArrayMapIndex(t *testing.T) {
	out := (Uint8ArrayMap{"baz": Uint8Array{Uint8(6)}}).ToUint8ArrayMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string][]uint8)["baz"], iv)
}

func TestUint8MapMapIndex(t *testing.T) {
	out := (Uint8MapMap{"baz": Uint8Map{"baz": Uint8(6)}}).ToUint8MapMapOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]map[string]uint8)["baz"], iv)
}
