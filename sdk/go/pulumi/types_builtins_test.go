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
		go func() { out.resolve(42, true, false) }()
		var ranApp bool
		app := out.ApplyT(func(v int) (interface{}, error) {
			ranApp = true
			return v + 1, nil
		})
		v, known, _, err := await(app)
		assert.True(t, ranApp)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, v, 43)
	}
	// Test that resolved, but unknown outputs, skip the running of applies.
	{
		out := newIntOutput()
		go func() { out.resolve(42, false, false) }()
		var ranApp bool
		app := out.ApplyT(func(v int) (interface{}, error) {
			ranApp = true
			return v + 1, nil
		})
		_, known, _, err := await(app)
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
		v, _, _, err := await(app)
		assert.False(t, ranApp)
		assert.NotNil(t, err)
		assert.Nil(t, v)
	}
	// Test that an an apply that returns an output returns the resolution of that output, not the output itself.
	{
		out := newIntOutput()
		go func() { out.resolve(42, true, false) }()
		var ranApp bool
		app := out.ApplyT(func(v int) (interface{}, error) {
			other, resolveOther, _ := NewOutput()
			go func() { resolveOther(v + 1) }()
			ranApp = true
			return other, nil
		})
		v, known, _, err := await(app)
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
		v, known, _, err = await(app)
		assert.True(t, ranApp)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, v, 44)
	}
	// Test that an an apply that reject an output returns the rejection of that output, not the output itself.
	{
		out := newIntOutput()
		go func() { out.resolve(42, true, false) }()
		var ranApp bool
		app := out.ApplyT(func(v int) (interface{}, error) {
			other, _, rejectOther := NewOutput()
			go func() { rejectOther(errors.New("boom")) }()
			ranApp = true
			return other, nil
		})
		v, _, _, err := await(app)
		assert.True(t, ranApp)
		assert.NotNil(t, err)
		assert.Nil(t, v)

		app = out.ApplyT(func(v int) (interface{}, error) {
			other, _, rejectOther := NewOutput()
			go func() { rejectOther(errors.New("boom")) }()
			ranApp = true
			return other, nil
		})
		v, _, _, err = await(app)
		assert.True(t, ranApp)
		assert.NotNil(t, err)
		assert.Nil(t, v)
	}
	// Test builtin applies.
	{
		out := newIntOutput()
		go func() { out.resolve(42, true, false) }()

		t.Run("ApplyArchive", func(t *testing.T) {
			o2 := out.ApplyArchive(func(v int) Archive { return *new(Archive) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyArchiveWithContext(context.Background(), func(_ context.Context, v int) Archive { return *new(Archive) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyArchiveArray", func(t *testing.T) {
			o2 := out.ApplyArchiveArray(func(v int) []Archive { return *new([]Archive) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyArchiveArrayWithContext(context.Background(), func(_ context.Context, v int) []Archive { return *new([]Archive) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyArchiveMap", func(t *testing.T) {
			o2 := out.ApplyArchiveMap(func(v int) map[string]Archive { return *new(map[string]Archive) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyArchiveMapWithContext(context.Background(), func(_ context.Context, v int) map[string]Archive { return *new(map[string]Archive) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAsset", func(t *testing.T) {
			o2 := out.ApplyAsset(func(v int) Asset { return *new(Asset) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyAssetWithContext(context.Background(), func(_ context.Context, v int) Asset { return *new(Asset) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetArray", func(t *testing.T) {
			o2 := out.ApplyAssetArray(func(v int) []Asset { return *new([]Asset) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyAssetArrayWithContext(context.Background(), func(_ context.Context, v int) []Asset { return *new([]Asset) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetMap", func(t *testing.T) {
			o2 := out.ApplyAssetMap(func(v int) map[string]Asset { return *new(map[string]Asset) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyAssetMapWithContext(context.Background(), func(_ context.Context, v int) map[string]Asset { return *new(map[string]Asset) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetOrArchive", func(t *testing.T) {
			o2 := out.ApplyAssetOrArchive(func(v int) AssetOrArchive { return *new(AssetOrArchive) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyAssetOrArchiveWithContext(context.Background(), func(_ context.Context, v int) AssetOrArchive { return *new(AssetOrArchive) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetOrArchiveArray", func(t *testing.T) {
			o2 := out.ApplyAssetOrArchiveArray(func(v int) []AssetOrArchive { return *new([]AssetOrArchive) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyAssetOrArchiveArrayWithContext(context.Background(), func(_ context.Context, v int) []AssetOrArchive { return *new([]AssetOrArchive) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyAssetOrArchiveMap", func(t *testing.T) {
			o2 := out.ApplyAssetOrArchiveMap(func(v int) map[string]AssetOrArchive { return *new(map[string]AssetOrArchive) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyAssetOrArchiveMapWithContext(context.Background(), func(_ context.Context, v int) map[string]AssetOrArchive { return *new(map[string]AssetOrArchive) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyBool", func(t *testing.T) {
			o2 := out.ApplyBool(func(v int) bool { return *new(bool) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyBoolWithContext(context.Background(), func(_ context.Context, v int) bool { return *new(bool) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyBoolPtr", func(t *testing.T) {
			o2 := out.ApplyBoolPtr(func(v int) *bool { return *new(*bool) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyBoolPtrWithContext(context.Background(), func(_ context.Context, v int) *bool { return *new(*bool) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyBoolArray", func(t *testing.T) {
			o2 := out.ApplyBoolArray(func(v int) []bool { return *new([]bool) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyBoolArrayWithContext(context.Background(), func(_ context.Context, v int) []bool { return *new([]bool) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyBoolMap", func(t *testing.T) {
			o2 := out.ApplyBoolMap(func(v int) map[string]bool { return *new(map[string]bool) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyBoolMapWithContext(context.Background(), func(_ context.Context, v int) map[string]bool { return *new(map[string]bool) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat32", func(t *testing.T) {
			o2 := out.ApplyFloat32(func(v int) float32 { return *new(float32) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat32WithContext(context.Background(), func(_ context.Context, v int) float32 { return *new(float32) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat32Ptr", func(t *testing.T) {
			o2 := out.ApplyFloat32Ptr(func(v int) *float32 { return *new(*float32) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat32PtrWithContext(context.Background(), func(_ context.Context, v int) *float32 { return *new(*float32) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat32Array", func(t *testing.T) {
			o2 := out.ApplyFloat32Array(func(v int) []float32 { return *new([]float32) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat32ArrayWithContext(context.Background(), func(_ context.Context, v int) []float32 { return *new([]float32) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat32Map", func(t *testing.T) {
			o2 := out.ApplyFloat32Map(func(v int) map[string]float32 { return *new(map[string]float32) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat32MapWithContext(context.Background(), func(_ context.Context, v int) map[string]float32 { return *new(map[string]float32) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat64", func(t *testing.T) {
			o2 := out.ApplyFloat64(func(v int) float64 { return *new(float64) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat64WithContext(context.Background(), func(_ context.Context, v int) float64 { return *new(float64) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat64Ptr", func(t *testing.T) {
			o2 := out.ApplyFloat64Ptr(func(v int) *float64 { return *new(*float64) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat64PtrWithContext(context.Background(), func(_ context.Context, v int) *float64 { return *new(*float64) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat64Array", func(t *testing.T) {
			o2 := out.ApplyFloat64Array(func(v int) []float64 { return *new([]float64) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat64ArrayWithContext(context.Background(), func(_ context.Context, v int) []float64 { return *new([]float64) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyFloat64Map", func(t *testing.T) {
			o2 := out.ApplyFloat64Map(func(v int) map[string]float64 { return *new(map[string]float64) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyFloat64MapWithContext(context.Background(), func(_ context.Context, v int) map[string]float64 { return *new(map[string]float64) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyID", func(t *testing.T) {
			o2 := out.ApplyID(func(v int) ID { return *new(ID) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIDWithContext(context.Background(), func(_ context.Context, v int) ID { return *new(ID) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIDPtr", func(t *testing.T) {
			o2 := out.ApplyIDPtr(func(v int) *ID { return *new(*ID) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIDPtrWithContext(context.Background(), func(_ context.Context, v int) *ID { return *new(*ID) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIDArray", func(t *testing.T) {
			o2 := out.ApplyIDArray(func(v int) []ID { return *new([]ID) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIDArrayWithContext(context.Background(), func(_ context.Context, v int) []ID { return *new([]ID) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIDMap", func(t *testing.T) {
			o2 := out.ApplyIDMap(func(v int) map[string]ID { return *new(map[string]ID) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIDMapWithContext(context.Background(), func(_ context.Context, v int) map[string]ID { return *new(map[string]ID) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyArray", func(t *testing.T) {
			o2 := out.ApplyArray(func(v int) []interface{} { return *new([]interface{}) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyArrayWithContext(context.Background(), func(_ context.Context, v int) []interface{} { return *new([]interface{}) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyMap", func(t *testing.T) {
			o2 := out.ApplyMap(func(v int) map[string]interface{} { return *new(map[string]interface{}) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyMapWithContext(context.Background(), func(_ context.Context, v int) map[string]interface{} { return *new(map[string]interface{}) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt", func(t *testing.T) {
			o2 := out.ApplyInt(func(v int) int { return *new(int) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIntWithContext(context.Background(), func(_ context.Context, v int) int { return *new(int) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIntPtr", func(t *testing.T) {
			o2 := out.ApplyIntPtr(func(v int) *int { return *new(*int) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIntPtrWithContext(context.Background(), func(_ context.Context, v int) *int { return *new(*int) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIntArray", func(t *testing.T) {
			o2 := out.ApplyIntArray(func(v int) []int { return *new([]int) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIntArrayWithContext(context.Background(), func(_ context.Context, v int) []int { return *new([]int) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyIntMap", func(t *testing.T) {
			o2 := out.ApplyIntMap(func(v int) map[string]int { return *new(map[string]int) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyIntMapWithContext(context.Background(), func(_ context.Context, v int) map[string]int { return *new(map[string]int) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt16", func(t *testing.T) {
			o2 := out.ApplyInt16(func(v int) int16 { return *new(int16) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt16WithContext(context.Background(), func(_ context.Context, v int) int16 { return *new(int16) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt16Ptr", func(t *testing.T) {
			o2 := out.ApplyInt16Ptr(func(v int) *int16 { return *new(*int16) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt16PtrWithContext(context.Background(), func(_ context.Context, v int) *int16 { return *new(*int16) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt16Array", func(t *testing.T) {
			o2 := out.ApplyInt16Array(func(v int) []int16 { return *new([]int16) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt16ArrayWithContext(context.Background(), func(_ context.Context, v int) []int16 { return *new([]int16) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt16Map", func(t *testing.T) {
			o2 := out.ApplyInt16Map(func(v int) map[string]int16 { return *new(map[string]int16) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt16MapWithContext(context.Background(), func(_ context.Context, v int) map[string]int16 { return *new(map[string]int16) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt32", func(t *testing.T) {
			o2 := out.ApplyInt32(func(v int) int32 { return *new(int32) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt32WithContext(context.Background(), func(_ context.Context, v int) int32 { return *new(int32) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt32Ptr", func(t *testing.T) {
			o2 := out.ApplyInt32Ptr(func(v int) *int32 { return *new(*int32) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt32PtrWithContext(context.Background(), func(_ context.Context, v int) *int32 { return *new(*int32) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt32Array", func(t *testing.T) {
			o2 := out.ApplyInt32Array(func(v int) []int32 { return *new([]int32) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt32ArrayWithContext(context.Background(), func(_ context.Context, v int) []int32 { return *new([]int32) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt32Map", func(t *testing.T) {
			o2 := out.ApplyInt32Map(func(v int) map[string]int32 { return *new(map[string]int32) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt32MapWithContext(context.Background(), func(_ context.Context, v int) map[string]int32 { return *new(map[string]int32) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt64", func(t *testing.T) {
			o2 := out.ApplyInt64(func(v int) int64 { return *new(int64) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt64WithContext(context.Background(), func(_ context.Context, v int) int64 { return *new(int64) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt64Ptr", func(t *testing.T) {
			o2 := out.ApplyInt64Ptr(func(v int) *int64 { return *new(*int64) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt64PtrWithContext(context.Background(), func(_ context.Context, v int) *int64 { return *new(*int64) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt64Array", func(t *testing.T) {
			o2 := out.ApplyInt64Array(func(v int) []int64 { return *new([]int64) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt64ArrayWithContext(context.Background(), func(_ context.Context, v int) []int64 { return *new([]int64) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt64Map", func(t *testing.T) {
			o2 := out.ApplyInt64Map(func(v int) map[string]int64 { return *new(map[string]int64) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt64MapWithContext(context.Background(), func(_ context.Context, v int) map[string]int64 { return *new(map[string]int64) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt8", func(t *testing.T) {
			o2 := out.ApplyInt8(func(v int) int8 { return *new(int8) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt8WithContext(context.Background(), func(_ context.Context, v int) int8 { return *new(int8) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt8Ptr", func(t *testing.T) {
			o2 := out.ApplyInt8Ptr(func(v int) *int8 { return *new(*int8) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt8PtrWithContext(context.Background(), func(_ context.Context, v int) *int8 { return *new(*int8) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt8Array", func(t *testing.T) {
			o2 := out.ApplyInt8Array(func(v int) []int8 { return *new([]int8) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt8ArrayWithContext(context.Background(), func(_ context.Context, v int) []int8 { return *new([]int8) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyInt8Map", func(t *testing.T) {
			o2 := out.ApplyInt8Map(func(v int) map[string]int8 { return *new(map[string]int8) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyInt8MapWithContext(context.Background(), func(_ context.Context, v int) map[string]int8 { return *new(map[string]int8) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyString", func(t *testing.T) {
			o2 := out.ApplyString(func(v int) string { return *new(string) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyStringWithContext(context.Background(), func(_ context.Context, v int) string { return *new(string) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyStringPtr", func(t *testing.T) {
			o2 := out.ApplyStringPtr(func(v int) *string { return *new(*string) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyStringPtrWithContext(context.Background(), func(_ context.Context, v int) *string { return *new(*string) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyStringArray", func(t *testing.T) {
			o2 := out.ApplyStringArray(func(v int) []string { return *new([]string) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyStringArrayWithContext(context.Background(), func(_ context.Context, v int) []string { return *new([]string) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyStringMap", func(t *testing.T) {
			o2 := out.ApplyStringMap(func(v int) map[string]string { return *new(map[string]string) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyStringMapWithContext(context.Background(), func(_ context.Context, v int) map[string]string { return *new(map[string]string) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyURN", func(t *testing.T) {
			o2 := out.ApplyURN(func(v int) URN { return *new(URN) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyURNWithContext(context.Background(), func(_ context.Context, v int) URN { return *new(URN) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyURNPtr", func(t *testing.T) {
			o2 := out.ApplyURNPtr(func(v int) *URN { return *new(*URN) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyURNPtrWithContext(context.Background(), func(_ context.Context, v int) *URN { return *new(*URN) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyURNArray", func(t *testing.T) {
			o2 := out.ApplyURNArray(func(v int) []URN { return *new([]URN) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyURNArrayWithContext(context.Background(), func(_ context.Context, v int) []URN { return *new([]URN) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyURNMap", func(t *testing.T) {
			o2 := out.ApplyURNMap(func(v int) map[string]URN { return *new(map[string]URN) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyURNMapWithContext(context.Background(), func(_ context.Context, v int) map[string]URN { return *new(map[string]URN) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint", func(t *testing.T) {
			o2 := out.ApplyUint(func(v int) uint { return *new(uint) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUintWithContext(context.Background(), func(_ context.Context, v int) uint { return *new(uint) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUintPtr", func(t *testing.T) {
			o2 := out.ApplyUintPtr(func(v int) *uint { return *new(*uint) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUintPtrWithContext(context.Background(), func(_ context.Context, v int) *uint { return *new(*uint) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUintArray", func(t *testing.T) {
			o2 := out.ApplyUintArray(func(v int) []uint { return *new([]uint) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUintArrayWithContext(context.Background(), func(_ context.Context, v int) []uint { return *new([]uint) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUintMap", func(t *testing.T) {
			o2 := out.ApplyUintMap(func(v int) map[string]uint { return *new(map[string]uint) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUintMapWithContext(context.Background(), func(_ context.Context, v int) map[string]uint { return *new(map[string]uint) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint16", func(t *testing.T) {
			o2 := out.ApplyUint16(func(v int) uint16 { return *new(uint16) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint16WithContext(context.Background(), func(_ context.Context, v int) uint16 { return *new(uint16) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint16Ptr", func(t *testing.T) {
			o2 := out.ApplyUint16Ptr(func(v int) *uint16 { return *new(*uint16) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint16PtrWithContext(context.Background(), func(_ context.Context, v int) *uint16 { return *new(*uint16) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint16Array", func(t *testing.T) {
			o2 := out.ApplyUint16Array(func(v int) []uint16 { return *new([]uint16) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint16ArrayWithContext(context.Background(), func(_ context.Context, v int) []uint16 { return *new([]uint16) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint16Map", func(t *testing.T) {
			o2 := out.ApplyUint16Map(func(v int) map[string]uint16 { return *new(map[string]uint16) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint16MapWithContext(context.Background(), func(_ context.Context, v int) map[string]uint16 { return *new(map[string]uint16) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint32", func(t *testing.T) {
			o2 := out.ApplyUint32(func(v int) uint32 { return *new(uint32) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint32WithContext(context.Background(), func(_ context.Context, v int) uint32 { return *new(uint32) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint32Ptr", func(t *testing.T) {
			o2 := out.ApplyUint32Ptr(func(v int) *uint32 { return *new(*uint32) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint32PtrWithContext(context.Background(), func(_ context.Context, v int) *uint32 { return *new(*uint32) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint32Array", func(t *testing.T) {
			o2 := out.ApplyUint32Array(func(v int) []uint32 { return *new([]uint32) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint32ArrayWithContext(context.Background(), func(_ context.Context, v int) []uint32 { return *new([]uint32) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint32Map", func(t *testing.T) {
			o2 := out.ApplyUint32Map(func(v int) map[string]uint32 { return *new(map[string]uint32) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint32MapWithContext(context.Background(), func(_ context.Context, v int) map[string]uint32 { return *new(map[string]uint32) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint64", func(t *testing.T) {
			o2 := out.ApplyUint64(func(v int) uint64 { return *new(uint64) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint64WithContext(context.Background(), func(_ context.Context, v int) uint64 { return *new(uint64) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint64Ptr", func(t *testing.T) {
			o2 := out.ApplyUint64Ptr(func(v int) *uint64 { return *new(*uint64) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint64PtrWithContext(context.Background(), func(_ context.Context, v int) *uint64 { return *new(*uint64) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint64Array", func(t *testing.T) {
			o2 := out.ApplyUint64Array(func(v int) []uint64 { return *new([]uint64) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint64ArrayWithContext(context.Background(), func(_ context.Context, v int) []uint64 { return *new([]uint64) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint64Map", func(t *testing.T) {
			o2 := out.ApplyUint64Map(func(v int) map[string]uint64 { return *new(map[string]uint64) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint64MapWithContext(context.Background(), func(_ context.Context, v int) map[string]uint64 { return *new(map[string]uint64) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint8", func(t *testing.T) {
			o2 := out.ApplyUint8(func(v int) uint8 { return *new(uint8) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint8WithContext(context.Background(), func(_ context.Context, v int) uint8 { return *new(uint8) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint8Ptr", func(t *testing.T) {
			o2 := out.ApplyUint8Ptr(func(v int) *uint8 { return *new(*uint8) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint8PtrWithContext(context.Background(), func(_ context.Context, v int) *uint8 { return *new(*uint8) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint8Array", func(t *testing.T) {
			o2 := out.ApplyUint8Array(func(v int) []uint8 { return *new([]uint8) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint8ArrayWithContext(context.Background(), func(_ context.Context, v int) []uint8 { return *new([]uint8) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

		t.Run("ApplyUint8Map", func(t *testing.T) {
			o2 := out.ApplyUint8Map(func(v int) map[string]uint8 { return *new(map[string]uint8) })
			_, known, _, err := await(o2)
			assert.True(t, known)
			assert.NoError(t, err)

			o2 = out.ApplyUint8MapWithContext(context.Background(), func(_ context.Context, v int) map[string]uint8 { return *new(map[string]uint8) })
			_, known, _, err = await(o2)
			assert.True(t, known)
			assert.NoError(t, err)
		})

	}
	// Test that applies return appropriate concrete implementations of Output based on the callback type
	{
		out := newIntOutput()
		go func() { out.resolve(42, true, false) }()

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
		go func() { out.resolve(42, true, false) }()

		out2 := StringOutput{newOutputState(reflect.TypeOf(""))}
		go func() { out2.resolve("hello", true, false) }()

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

		v, known, _, err := await(res)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, []string{"qux", "zed"}, v)

		_, ok = res2.(StringArrayOutput)
		assert.True(t, ok)

		v, known, _, err = await(res2)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, []string{"foo", "bar"}, v)

		_, ok = res3.(StringOutput)
		assert.True(t, ok)

		v, known, _, err = await(res3)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, "foo,bar,qux,zed", v)

		_, ok = res4.(AnyOutput)
		assert.True(t, ok)

		v, known, _, err = await(res4)
		assert.Nil(t, err)
		assert.True(t, known)
		assert.Equal(t, &myStructType{foo: 42, bar: "hello"}, v)

		v, known, _, err = await(res5)
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

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(ArchiveInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputArchiveArray(t *testing.T) {
	out := ToOutput(ArchiveArray{NewFileArchive("foo.zip")})
	_, ok := out.(ArchiveArrayInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(ArchiveArrayInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputArchiveMap(t *testing.T) {
	out := ToOutput(ArchiveMap{"baz": NewFileArchive("foo.zip")})
	_, ok := out.(ArchiveMapInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(ArchiveMapInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAsset(t *testing.T) {
	out := ToOutput(NewFileAsset("foo.txt"))
	_, ok := out.(AssetInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetArray(t *testing.T) {
	out := ToOutput(AssetArray{NewFileAsset("foo.txt")})
	_, ok := out.(AssetArrayInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetArrayInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetMap(t *testing.T) {
	out := ToOutput(AssetMap{"baz": NewFileAsset("foo.txt")})
	_, ok := out.(AssetMapInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetMapInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetOrArchive(t *testing.T) {
	out := ToOutput(NewFileArchive("foo.zip"))
	_, ok := out.(AssetOrArchiveInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetOrArchiveInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetOrArchiveArray(t *testing.T) {
	out := ToOutput(AssetOrArchiveArray{NewFileArchive("foo.zip")})
	_, ok := out.(AssetOrArchiveArrayInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetOrArchiveArrayInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputAssetOrArchiveMap(t *testing.T) {
	out := ToOutput(AssetOrArchiveMap{"baz": NewFileArchive("foo.zip")})
	_, ok := out.(AssetOrArchiveMapInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(AssetOrArchiveMapInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputBool(t *testing.T) {
	out := ToOutput(Bool(true))
	_, ok := out.(BoolInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(BoolInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputBoolPtr(t *testing.T) {
	out := ToOutput(BoolPtr(bool(Bool(true))))
	_, ok := out.(BoolPtrInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(BoolPtrInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputBoolArray(t *testing.T) {
	out := ToOutput(BoolArray{Bool(true)})
	_, ok := out.(BoolArrayInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(BoolArrayInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputBoolMap(t *testing.T) {
	out := ToOutput(BoolMap{"baz": Bool(true)})
	_, ok := out.(BoolMapInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(BoolMapInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat32(t *testing.T) {
	out := ToOutput(Float32(1.3))
	_, ok := out.(Float32Input)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float32Input)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat32Ptr(t *testing.T) {
	out := ToOutput(Float32Ptr(float32(Float32(1.3))))
	_, ok := out.(Float32PtrInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float32PtrInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat32Array(t *testing.T) {
	out := ToOutput(Float32Array{Float32(1.3)})
	_, ok := out.(Float32ArrayInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float32ArrayInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat32Map(t *testing.T) {
	out := ToOutput(Float32Map{"baz": Float32(1.3)})
	_, ok := out.(Float32MapInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float32MapInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat64(t *testing.T) {
	out := ToOutput(Float64(999.9))
	_, ok := out.(Float64Input)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float64Input)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat64Ptr(t *testing.T) {
	out := ToOutput(Float64Ptr(float64(Float64(999.9))))
	_, ok := out.(Float64PtrInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float64PtrInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat64Array(t *testing.T) {
	out := ToOutput(Float64Array{Float64(999.9)})
	_, ok := out.(Float64ArrayInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float64ArrayInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputFloat64Map(t *testing.T) {
	out := ToOutput(Float64Map{"baz": Float64(999.9)})
	_, ok := out.(Float64MapInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Float64MapInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputID(t *testing.T) {
	out := ToOutput(ID("foo"))
	_, ok := out.(IDInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IDInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIDPtr(t *testing.T) {
	out := ToOutput(IDPtr(ID(ID("foo"))))
	_, ok := out.(IDPtrInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IDPtrInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIDArray(t *testing.T) {
	out := ToOutput(IDArray{ID("foo")})
	_, ok := out.(IDArrayInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IDArrayInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIDMap(t *testing.T) {
	out := ToOutput(IDMap{"baz": ID("foo")})
	_, ok := out.(IDMapInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IDMapInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputArray(t *testing.T) {
	out := ToOutput(Array{String("any")})
	_, ok := out.(ArrayInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(ArrayInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputMap(t *testing.T) {
	out := ToOutput(Map{"baz": String("any")})
	_, ok := out.(MapInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(MapInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt(t *testing.T) {
	out := ToOutput(Int(42))
	_, ok := out.(IntInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IntInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIntPtr(t *testing.T) {
	out := ToOutput(IntPtr(int(Int(42))))
	_, ok := out.(IntPtrInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IntPtrInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIntArray(t *testing.T) {
	out := ToOutput(IntArray{Int(42)})
	_, ok := out.(IntArrayInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IntArrayInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputIntMap(t *testing.T) {
	out := ToOutput(IntMap{"baz": Int(42)})
	_, ok := out.(IntMapInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(IntMapInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt16(t *testing.T) {
	out := ToOutput(Int16(33))
	_, ok := out.(Int16Input)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int16Input)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt16Ptr(t *testing.T) {
	out := ToOutput(Int16Ptr(int16(Int16(33))))
	_, ok := out.(Int16PtrInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int16PtrInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt16Array(t *testing.T) {
	out := ToOutput(Int16Array{Int16(33)})
	_, ok := out.(Int16ArrayInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int16ArrayInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt16Map(t *testing.T) {
	out := ToOutput(Int16Map{"baz": Int16(33)})
	_, ok := out.(Int16MapInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int16MapInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt32(t *testing.T) {
	out := ToOutput(Int32(24))
	_, ok := out.(Int32Input)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int32Input)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt32Ptr(t *testing.T) {
	out := ToOutput(Int32Ptr(int32(Int32(24))))
	_, ok := out.(Int32PtrInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int32PtrInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt32Array(t *testing.T) {
	out := ToOutput(Int32Array{Int32(24)})
	_, ok := out.(Int32ArrayInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int32ArrayInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt32Map(t *testing.T) {
	out := ToOutput(Int32Map{"baz": Int32(24)})
	_, ok := out.(Int32MapInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int32MapInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt64(t *testing.T) {
	out := ToOutput(Int64(15))
	_, ok := out.(Int64Input)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int64Input)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt64Ptr(t *testing.T) {
	out := ToOutput(Int64Ptr(int64(Int64(15))))
	_, ok := out.(Int64PtrInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int64PtrInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt64Array(t *testing.T) {
	out := ToOutput(Int64Array{Int64(15)})
	_, ok := out.(Int64ArrayInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int64ArrayInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt64Map(t *testing.T) {
	out := ToOutput(Int64Map{"baz": Int64(15)})
	_, ok := out.(Int64MapInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int64MapInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt8(t *testing.T) {
	out := ToOutput(Int8(6))
	_, ok := out.(Int8Input)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int8Input)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt8Ptr(t *testing.T) {
	out := ToOutput(Int8Ptr(int8(Int8(6))))
	_, ok := out.(Int8PtrInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int8PtrInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt8Array(t *testing.T) {
	out := ToOutput(Int8Array{Int8(6)})
	_, ok := out.(Int8ArrayInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int8ArrayInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputInt8Map(t *testing.T) {
	out := ToOutput(Int8Map{"baz": Int8(6)})
	_, ok := out.(Int8MapInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Int8MapInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputString(t *testing.T) {
	out := ToOutput(String("foo"))
	_, ok := out.(StringInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(StringInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputStringPtr(t *testing.T) {
	out := ToOutput(StringPtr(string(String("foo"))))
	_, ok := out.(StringPtrInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(StringPtrInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputStringArray(t *testing.T) {
	out := ToOutput(StringArray{String("foo")})
	_, ok := out.(StringArrayInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(StringArrayInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputStringMap(t *testing.T) {
	out := ToOutput(StringMap{"baz": String("foo")})
	_, ok := out.(StringMapInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(StringMapInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputURN(t *testing.T) {
	out := ToOutput(URN("foo"))
	_, ok := out.(URNInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(URNInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputURNPtr(t *testing.T) {
	out := ToOutput(URNPtr(URN(URN("foo"))))
	_, ok := out.(URNPtrInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(URNPtrInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputURNArray(t *testing.T) {
	out := ToOutput(URNArray{URN("foo")})
	_, ok := out.(URNArrayInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(URNArrayInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputURNMap(t *testing.T) {
	out := ToOutput(URNMap{"baz": URN("foo")})
	_, ok := out.(URNMapInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(URNMapInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint(t *testing.T) {
	out := ToOutput(Uint(42))
	_, ok := out.(UintInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(UintInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUintPtr(t *testing.T) {
	out := ToOutput(UintPtr(uint(Uint(42))))
	_, ok := out.(UintPtrInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(UintPtrInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUintArray(t *testing.T) {
	out := ToOutput(UintArray{Uint(42)})
	_, ok := out.(UintArrayInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(UintArrayInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUintMap(t *testing.T) {
	out := ToOutput(UintMap{"baz": Uint(42)})
	_, ok := out.(UintMapInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(UintMapInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint16(t *testing.T) {
	out := ToOutput(Uint16(33))
	_, ok := out.(Uint16Input)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint16Input)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint16Ptr(t *testing.T) {
	out := ToOutput(Uint16Ptr(uint16(Uint16(33))))
	_, ok := out.(Uint16PtrInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint16PtrInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint16Array(t *testing.T) {
	out := ToOutput(Uint16Array{Uint16(33)})
	_, ok := out.(Uint16ArrayInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint16ArrayInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint16Map(t *testing.T) {
	out := ToOutput(Uint16Map{"baz": Uint16(33)})
	_, ok := out.(Uint16MapInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint16MapInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint32(t *testing.T) {
	out := ToOutput(Uint32(24))
	_, ok := out.(Uint32Input)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint32Input)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint32Ptr(t *testing.T) {
	out := ToOutput(Uint32Ptr(uint32(Uint32(24))))
	_, ok := out.(Uint32PtrInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint32PtrInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint32Array(t *testing.T) {
	out := ToOutput(Uint32Array{Uint32(24)})
	_, ok := out.(Uint32ArrayInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint32ArrayInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint32Map(t *testing.T) {
	out := ToOutput(Uint32Map{"baz": Uint32(24)})
	_, ok := out.(Uint32MapInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint32MapInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint64(t *testing.T) {
	out := ToOutput(Uint64(15))
	_, ok := out.(Uint64Input)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint64Input)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint64Ptr(t *testing.T) {
	out := ToOutput(Uint64Ptr(uint64(Uint64(15))))
	_, ok := out.(Uint64PtrInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint64PtrInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint64Array(t *testing.T) {
	out := ToOutput(Uint64Array{Uint64(15)})
	_, ok := out.(Uint64ArrayInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint64ArrayInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint64Map(t *testing.T) {
	out := ToOutput(Uint64Map{"baz": Uint64(15)})
	_, ok := out.(Uint64MapInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint64MapInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint8(t *testing.T) {
	out := ToOutput(Uint8(6))
	_, ok := out.(Uint8Input)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint8Input)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint8Ptr(t *testing.T) {
	out := ToOutput(Uint8Ptr(uint8(Uint8(6))))
	_, ok := out.(Uint8PtrInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint8PtrInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint8Array(t *testing.T) {
	out := ToOutput(Uint8Array{Uint8(6)})
	_, ok := out.(Uint8ArrayInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint8ArrayInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToOutputUint8Map(t *testing.T) {
	out := ToOutput(Uint8Map{"baz": Uint8(6)})
	_, ok := out.(Uint8MapInput)
	assert.True(t, ok)

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = ToOutput(out)
	_, ok = out.(Uint8MapInput)
	assert.True(t, ok)

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

// Test that type-specific ToOutput methods work with all builtin input and output types

func TestToArchiveOutput(t *testing.T) {
	in := ArchiveInput(NewFileArchive("foo.zip"))

	out := in.ToArchiveOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToArchiveOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToArchiveArrayOutput(t *testing.T) {
	in := ArchiveArrayInput(ArchiveArray{NewFileArchive("foo.zip")})

	out := in.ToArchiveArrayOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveArrayOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToArchiveArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToArchiveMapOutput(t *testing.T) {
	in := ArchiveMapInput(ArchiveMap{"baz": NewFileArchive("foo.zip")})

	out := in.ToArchiveMapOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveMapOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToArchiveMapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArchiveMapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToAssetOutput(t *testing.T) {
	in := AssetInput(NewFileAsset("foo.txt"))

	out := in.ToAssetOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToAssetOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToAssetArrayOutput(t *testing.T) {
	in := AssetArrayInput(AssetArray{NewFileAsset("foo.txt")})

	out := in.ToAssetArrayOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetArrayOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToAssetArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToAssetMapOutput(t *testing.T) {
	in := AssetMapInput(AssetMap{"baz": NewFileAsset("foo.txt")})

	out := in.ToAssetMapOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetMapOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToAssetMapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetMapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToAssetOrArchiveOutput(t *testing.T) {
	in := AssetOrArchiveInput(NewFileArchive("foo.zip"))

	out := in.ToAssetOrArchiveOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToAssetOrArchiveOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToAssetOrArchiveArrayOutput(t *testing.T) {
	in := AssetOrArchiveArrayInput(AssetOrArchiveArray{NewFileArchive("foo.zip")})

	out := in.ToAssetOrArchiveArrayOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveArrayOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToAssetOrArchiveArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToAssetOrArchiveMapOutput(t *testing.T) {
	in := AssetOrArchiveMapInput(AssetOrArchiveMap{"baz": NewFileArchive("foo.zip")})

	out := in.ToAssetOrArchiveMapOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveMapOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToAssetOrArchiveMapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToAssetOrArchiveMapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToBoolOutput(t *testing.T) {
	in := BoolInput(Bool(true))

	out := in.ToBoolOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToBoolOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToBoolPtrOutput(t *testing.T) {
	in := BoolPtrInput(BoolPtr(bool(Bool(true))))

	out := in.ToBoolPtrOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolPtrOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToBoolPtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolPtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToBoolArrayOutput(t *testing.T) {
	in := BoolArrayInput(BoolArray{Bool(true)})

	out := in.ToBoolArrayOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolArrayOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToBoolArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToBoolMapOutput(t *testing.T) {
	in := BoolMapInput(BoolMap{"baz": Bool(true)})

	out := in.ToBoolMapOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolMapOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToBoolMapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToBoolMapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat32Output(t *testing.T) {
	in := Float32Input(Float32(1.3))

	out := in.ToFloat32Output()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32Output()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat32OutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32OutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat32PtrOutput(t *testing.T) {
	in := Float32PtrInput(Float32Ptr(float32(Float32(1.3))))

	out := in.ToFloat32PtrOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32PtrOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat32PtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32PtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat32ArrayOutput(t *testing.T) {
	in := Float32ArrayInput(Float32Array{Float32(1.3)})

	out := in.ToFloat32ArrayOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32ArrayOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat32ArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32ArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat32MapOutput(t *testing.T) {
	in := Float32MapInput(Float32Map{"baz": Float32(1.3)})

	out := in.ToFloat32MapOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32MapOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat32MapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat32MapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat64Output(t *testing.T) {
	in := Float64Input(Float64(999.9))

	out := in.ToFloat64Output()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64Output()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat64OutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64OutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat64PtrOutput(t *testing.T) {
	in := Float64PtrInput(Float64Ptr(float64(Float64(999.9))))

	out := in.ToFloat64PtrOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64PtrOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat64PtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64PtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat64ArrayOutput(t *testing.T) {
	in := Float64ArrayInput(Float64Array{Float64(999.9)})

	out := in.ToFloat64ArrayOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64ArrayOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat64ArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64ArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToFloat64MapOutput(t *testing.T) {
	in := Float64MapInput(Float64Map{"baz": Float64(999.9)})

	out := in.ToFloat64MapOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64MapOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToFloat64MapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToFloat64MapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIDOutput(t *testing.T) {
	in := IDInput(ID("foo"))

	out := in.ToIDOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIDOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIDPtrOutput(t *testing.T) {
	in := IDPtrInput(IDPtr(ID(ID("foo"))))

	out := in.ToIDPtrOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDPtrOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIDPtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDPtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIDArrayOutput(t *testing.T) {
	in := IDArrayInput(IDArray{ID("foo")})

	out := in.ToIDArrayOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDArrayOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIDArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIDMapOutput(t *testing.T) {
	in := IDMapInput(IDMap{"baz": ID("foo")})

	out := in.ToIDMapOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDMapOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIDMapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIDMapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToArrayOutput(t *testing.T) {
	in := ArrayInput(Array{String("any")})

	out := in.ToArrayOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArrayOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToMapOutput(t *testing.T) {
	in := MapInput(Map{"baz": String("any")})

	out := in.ToMapOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToMapOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToMapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToMapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIntOutput(t *testing.T) {
	in := IntInput(Int(42))

	out := in.ToIntOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIntOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIntPtrOutput(t *testing.T) {
	in := IntPtrInput(IntPtr(int(Int(42))))

	out := in.ToIntPtrOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntPtrOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIntPtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntPtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIntArrayOutput(t *testing.T) {
	in := IntArrayInput(IntArray{Int(42)})

	out := in.ToIntArrayOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntArrayOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIntArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToIntMapOutput(t *testing.T) {
	in := IntMapInput(IntMap{"baz": Int(42)})

	out := in.ToIntMapOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntMapOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToIntMapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToIntMapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt16Output(t *testing.T) {
	in := Int16Input(Int16(33))

	out := in.ToInt16Output()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16Output()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt16OutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16OutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt16PtrOutput(t *testing.T) {
	in := Int16PtrInput(Int16Ptr(int16(Int16(33))))

	out := in.ToInt16PtrOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16PtrOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt16PtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16PtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt16ArrayOutput(t *testing.T) {
	in := Int16ArrayInput(Int16Array{Int16(33)})

	out := in.ToInt16ArrayOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16ArrayOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt16ArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16ArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt16MapOutput(t *testing.T) {
	in := Int16MapInput(Int16Map{"baz": Int16(33)})

	out := in.ToInt16MapOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16MapOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt16MapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt16MapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt32Output(t *testing.T) {
	in := Int32Input(Int32(24))

	out := in.ToInt32Output()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32Output()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt32OutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32OutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt32PtrOutput(t *testing.T) {
	in := Int32PtrInput(Int32Ptr(int32(Int32(24))))

	out := in.ToInt32PtrOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32PtrOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt32PtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32PtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt32ArrayOutput(t *testing.T) {
	in := Int32ArrayInput(Int32Array{Int32(24)})

	out := in.ToInt32ArrayOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32ArrayOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt32ArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32ArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt32MapOutput(t *testing.T) {
	in := Int32MapInput(Int32Map{"baz": Int32(24)})

	out := in.ToInt32MapOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32MapOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt32MapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt32MapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt64Output(t *testing.T) {
	in := Int64Input(Int64(15))

	out := in.ToInt64Output()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64Output()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt64OutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64OutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt64PtrOutput(t *testing.T) {
	in := Int64PtrInput(Int64Ptr(int64(Int64(15))))

	out := in.ToInt64PtrOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64PtrOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt64PtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64PtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt64ArrayOutput(t *testing.T) {
	in := Int64ArrayInput(Int64Array{Int64(15)})

	out := in.ToInt64ArrayOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64ArrayOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt64ArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64ArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt64MapOutput(t *testing.T) {
	in := Int64MapInput(Int64Map{"baz": Int64(15)})

	out := in.ToInt64MapOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64MapOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt64MapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt64MapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt8Output(t *testing.T) {
	in := Int8Input(Int8(6))

	out := in.ToInt8Output()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8Output()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt8OutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8OutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt8PtrOutput(t *testing.T) {
	in := Int8PtrInput(Int8Ptr(int8(Int8(6))))

	out := in.ToInt8PtrOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8PtrOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt8PtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8PtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt8ArrayOutput(t *testing.T) {
	in := Int8ArrayInput(Int8Array{Int8(6)})

	out := in.ToInt8ArrayOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8ArrayOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt8ArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8ArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToInt8MapOutput(t *testing.T) {
	in := Int8MapInput(Int8Map{"baz": Int8(6)})

	out := in.ToInt8MapOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8MapOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToInt8MapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToInt8MapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToStringOutput(t *testing.T) {
	in := StringInput(String("foo"))

	out := in.ToStringOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToStringOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToStringPtrOutput(t *testing.T) {
	in := StringPtrInput(StringPtr(string(String("foo"))))

	out := in.ToStringPtrOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringPtrOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToStringPtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringPtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToStringArrayOutput(t *testing.T) {
	in := StringArrayInput(StringArray{String("foo")})

	out := in.ToStringArrayOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringArrayOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToStringArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToStringMapOutput(t *testing.T) {
	in := StringMapInput(StringMap{"baz": String("foo")})

	out := in.ToStringMapOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringMapOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToStringMapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToStringMapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToURNOutput(t *testing.T) {
	in := URNInput(URN("foo"))

	out := in.ToURNOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToURNOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToURNPtrOutput(t *testing.T) {
	in := URNPtrInput(URNPtr(URN(URN("foo"))))

	out := in.ToURNPtrOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNPtrOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToURNPtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNPtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToURNArrayOutput(t *testing.T) {
	in := URNArrayInput(URNArray{URN("foo")})

	out := in.ToURNArrayOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNArrayOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToURNArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToURNMapOutput(t *testing.T) {
	in := URNMapInput(URNMap{"baz": URN("foo")})

	out := in.ToURNMapOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNMapOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToURNMapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToURNMapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUintOutput(t *testing.T) {
	in := UintInput(Uint(42))

	out := in.ToUintOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUintOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUintPtrOutput(t *testing.T) {
	in := UintPtrInput(UintPtr(uint(Uint(42))))

	out := in.ToUintPtrOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintPtrOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUintPtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintPtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUintArrayOutput(t *testing.T) {
	in := UintArrayInput(UintArray{Uint(42)})

	out := in.ToUintArrayOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintArrayOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUintArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUintMapOutput(t *testing.T) {
	in := UintMapInput(UintMap{"baz": Uint(42)})

	out := in.ToUintMapOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintMapOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUintMapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUintMapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint16Output(t *testing.T) {
	in := Uint16Input(Uint16(33))

	out := in.ToUint16Output()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16Output()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint16OutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16OutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint16PtrOutput(t *testing.T) {
	in := Uint16PtrInput(Uint16Ptr(uint16(Uint16(33))))

	out := in.ToUint16PtrOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16PtrOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint16PtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16PtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint16ArrayOutput(t *testing.T) {
	in := Uint16ArrayInput(Uint16Array{Uint16(33)})

	out := in.ToUint16ArrayOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16ArrayOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint16ArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16ArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint16MapOutput(t *testing.T) {
	in := Uint16MapInput(Uint16Map{"baz": Uint16(33)})

	out := in.ToUint16MapOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16MapOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint16MapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint16MapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint32Output(t *testing.T) {
	in := Uint32Input(Uint32(24))

	out := in.ToUint32Output()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32Output()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint32OutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32OutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint32PtrOutput(t *testing.T) {
	in := Uint32PtrInput(Uint32Ptr(uint32(Uint32(24))))

	out := in.ToUint32PtrOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32PtrOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint32PtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32PtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint32ArrayOutput(t *testing.T) {
	in := Uint32ArrayInput(Uint32Array{Uint32(24)})

	out := in.ToUint32ArrayOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32ArrayOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint32ArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32ArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint32MapOutput(t *testing.T) {
	in := Uint32MapInput(Uint32Map{"baz": Uint32(24)})

	out := in.ToUint32MapOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32MapOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint32MapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint32MapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint64Output(t *testing.T) {
	in := Uint64Input(Uint64(15))

	out := in.ToUint64Output()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64Output()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint64OutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64OutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint64PtrOutput(t *testing.T) {
	in := Uint64PtrInput(Uint64Ptr(uint64(Uint64(15))))

	out := in.ToUint64PtrOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64PtrOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint64PtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64PtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint64ArrayOutput(t *testing.T) {
	in := Uint64ArrayInput(Uint64Array{Uint64(15)})

	out := in.ToUint64ArrayOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64ArrayOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint64ArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64ArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint64MapOutput(t *testing.T) {
	in := Uint64MapInput(Uint64Map{"baz": Uint64(15)})

	out := in.ToUint64MapOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64MapOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint64MapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint64MapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint8Output(t *testing.T) {
	in := Uint8Input(Uint8(6))

	out := in.ToUint8Output()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8Output()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint8OutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8OutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint8PtrOutput(t *testing.T) {
	in := Uint8PtrInput(Uint8Ptr(uint8(Uint8(6))))

	out := in.ToUint8PtrOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8PtrOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint8PtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8PtrOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint8ArrayOutput(t *testing.T) {
	in := Uint8ArrayInput(Uint8Array{Uint8(6)})

	out := in.ToUint8ArrayOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8ArrayOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint8ArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8ArrayOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

func TestToUint8MapOutput(t *testing.T) {
	in := Uint8MapInput(Uint8Map{"baz": Uint8(6)})

	out := in.ToUint8MapOutput()

	_, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8MapOutput()

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = in.ToUint8MapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	out = out.ToUint8MapOutputWithContext(context.Background())

	_, known, _, err = await(out)
	assert.True(t, known)
	assert.NoError(t, err)
}

// Test type-specific ToOutput methods for builtins that implement other builtin input types.
func TestBuiltinConversions(t *testing.T) {
	archiveIn := NewFileArchive("foo.zip")
	assetOrArchiveOut := archiveIn.ToAssetOrArchiveOutput()
	archiveV, known, _, err := await(assetOrArchiveOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, archiveIn, archiveV)

	archiveOut := archiveIn.ToArchiveOutput()
	assetOrArchiveOut = archiveOut.ToAssetOrArchiveOutput()
	archiveV, known, _, err = await(assetOrArchiveOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, archiveIn, archiveV)

	assetIn := NewFileAsset("foo.zip")
	assetOrArchiveOut = assetIn.ToAssetOrArchiveOutput()
	assetV, known, _, err := await(assetOrArchiveOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, assetIn, assetV)

	assetOut := assetIn.ToAssetOutput()
	assetOrArchiveOut = assetOut.ToAssetOrArchiveOutput()
	assetV, known, _, err = await(assetOrArchiveOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, assetIn, assetV)

	idIn := ID("foo")
	stringOut := idIn.ToStringOutput()
	stringV, known, _, err := await(stringOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, string(idIn), stringV)

	idOut := idIn.ToIDOutput()
	stringOut = idOut.ToStringOutput()
	stringV, known, _, err = await(stringOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, string(idIn), stringV)

	urnIn := URN("foo")
	stringOut = urnIn.ToStringOutput()
	stringV, known, _, err = await(stringOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, string(urnIn), stringV)

	urnOut := urnIn.ToURNOutput()
	stringOut = urnOut.ToStringOutput()
	stringV, known, _, err = await(stringOut)
	assert.True(t, known)
	assert.NoError(t, err)
	assert.Equal(t, string(urnIn), stringV)
}

// Test pointer types.

func TestBoolPtrElem(t *testing.T) {
	out := (BoolPtr(bool(Bool(true)))).ToBoolPtrOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*bool)), iv)
}

func TestFloat32PtrElem(t *testing.T) {
	out := (Float32Ptr(float32(Float32(1.3)))).ToFloat32PtrOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*float32)), iv)
}

func TestFloat64PtrElem(t *testing.T) {
	out := (Float64Ptr(float64(Float64(999.9)))).ToFloat64PtrOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*float64)), iv)
}

func TestIDPtrElem(t *testing.T) {
	out := (IDPtr(ID(ID("foo")))).ToIDPtrOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*ID)), iv)
}

func TestIntPtrElem(t *testing.T) {
	out := (IntPtr(int(Int(42)))).ToIntPtrOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*int)), iv)
}

func TestInt16PtrElem(t *testing.T) {
	out := (Int16Ptr(int16(Int16(33)))).ToInt16PtrOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*int16)), iv)
}

func TestInt32PtrElem(t *testing.T) {
	out := (Int32Ptr(int32(Int32(24)))).ToInt32PtrOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*int32)), iv)
}

func TestInt64PtrElem(t *testing.T) {
	out := (Int64Ptr(int64(Int64(15)))).ToInt64PtrOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*int64)), iv)
}

func TestInt8PtrElem(t *testing.T) {
	out := (Int8Ptr(int8(Int8(6)))).ToInt8PtrOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*int8)), iv)
}

func TestStringPtrElem(t *testing.T) {
	out := (StringPtr(string(String("foo")))).ToStringPtrOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*string)), iv)
}

func TestURNPtrElem(t *testing.T) {
	out := (URNPtr(URN(URN("foo")))).ToURNPtrOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*URN)), iv)
}

func TestUintPtrElem(t *testing.T) {
	out := (UintPtr(uint(Uint(42)))).ToUintPtrOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*uint)), iv)
}

func TestUint16PtrElem(t *testing.T) {
	out := (Uint16Ptr(uint16(Uint16(33)))).ToUint16PtrOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*uint16)), iv)
}

func TestUint32PtrElem(t *testing.T) {
	out := (Uint32Ptr(uint32(Uint32(24)))).ToUint32PtrOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*uint32)), iv)
}

func TestUint64PtrElem(t *testing.T) {
	out := (Uint64Ptr(uint64(Uint64(15)))).ToUint64PtrOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*uint64)), iv)
}

func TestUint8PtrElem(t *testing.T) {
	out := (Uint8Ptr(uint8(Uint8(6)))).ToUint8PtrOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Elem())
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, *(av.(*uint8)), iv)
}

// Test array indexers.

func TestArchiveArrayIndex(t *testing.T) {
	out := (ArchiveArray{NewFileArchive("foo.zip")}).ToArchiveArrayOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]Archive)[0], iv)
}

func TestAssetArrayIndex(t *testing.T) {
	out := (AssetArray{NewFileAsset("foo.txt")}).ToAssetArrayOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]Asset)[0], iv)
}

func TestAssetOrArchiveArrayIndex(t *testing.T) {
	out := (AssetOrArchiveArray{NewFileArchive("foo.zip")}).ToAssetOrArchiveArrayOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]AssetOrArchive)[0], iv)
}

func TestBoolArrayIndex(t *testing.T) {
	out := (BoolArray{Bool(true)}).ToBoolArrayOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]bool)[0], iv)
}

func TestFloat32ArrayIndex(t *testing.T) {
	out := (Float32Array{Float32(1.3)}).ToFloat32ArrayOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]float32)[0], iv)
}

func TestFloat64ArrayIndex(t *testing.T) {
	out := (Float64Array{Float64(999.9)}).ToFloat64ArrayOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]float64)[0], iv)
}

func TestIDArrayIndex(t *testing.T) {
	out := (IDArray{ID("foo")}).ToIDArrayOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]ID)[0], iv)
}

func TestArrayIndex(t *testing.T) {
	out := (Array{String("any")}).ToArrayOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]interface{})[0], iv)
}

func TestIntArrayIndex(t *testing.T) {
	out := (IntArray{Int(42)}).ToIntArrayOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]int)[0], iv)
}

func TestInt16ArrayIndex(t *testing.T) {
	out := (Int16Array{Int16(33)}).ToInt16ArrayOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]int16)[0], iv)
}

func TestInt32ArrayIndex(t *testing.T) {
	out := (Int32Array{Int32(24)}).ToInt32ArrayOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]int32)[0], iv)
}

func TestInt64ArrayIndex(t *testing.T) {
	out := (Int64Array{Int64(15)}).ToInt64ArrayOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]int64)[0], iv)
}

func TestInt8ArrayIndex(t *testing.T) {
	out := (Int8Array{Int8(6)}).ToInt8ArrayOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]int8)[0], iv)
}

func TestStringArrayIndex(t *testing.T) {
	out := (StringArray{String("foo")}).ToStringArrayOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]string)[0], iv)
}

func TestURNArrayIndex(t *testing.T) {
	out := (URNArray{URN("foo")}).ToURNArrayOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]URN)[0], iv)
}

func TestUintArrayIndex(t *testing.T) {
	out := (UintArray{Uint(42)}).ToUintArrayOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]uint)[0], iv)
}

func TestUint16ArrayIndex(t *testing.T) {
	out := (Uint16Array{Uint16(33)}).ToUint16ArrayOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]uint16)[0], iv)
}

func TestUint32ArrayIndex(t *testing.T) {
	out := (Uint32Array{Uint32(24)}).ToUint32ArrayOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]uint32)[0], iv)
}

func TestUint64ArrayIndex(t *testing.T) {
	out := (Uint64Array{Uint64(15)}).ToUint64ArrayOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]uint64)[0], iv)
}

func TestUint8ArrayIndex(t *testing.T) {
	out := (Uint8Array{Uint8(6)}).ToUint8ArrayOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.Index(Int(0)))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.([]uint8)[0], iv)
}

// Test map indexers.

func TestArchiveMapIndex(t *testing.T) {
	out := (ArchiveMap{"baz": NewFileArchive("foo.zip")}).ToArchiveMapOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]Archive)["baz"], iv)
}

func TestAssetMapIndex(t *testing.T) {
	out := (AssetMap{"baz": NewFileAsset("foo.txt")}).ToAssetMapOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]Asset)["baz"], iv)
}

func TestAssetOrArchiveMapIndex(t *testing.T) {
	out := (AssetOrArchiveMap{"baz": NewFileArchive("foo.zip")}).ToAssetOrArchiveMapOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]AssetOrArchive)["baz"], iv)
}

func TestBoolMapIndex(t *testing.T) {
	out := (BoolMap{"baz": Bool(true)}).ToBoolMapOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]bool)["baz"], iv)
}

func TestFloat32MapIndex(t *testing.T) {
	out := (Float32Map{"baz": Float32(1.3)}).ToFloat32MapOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]float32)["baz"], iv)
}

func TestFloat64MapIndex(t *testing.T) {
	out := (Float64Map{"baz": Float64(999.9)}).ToFloat64MapOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]float64)["baz"], iv)
}

func TestIDMapIndex(t *testing.T) {
	out := (IDMap{"baz": ID("foo")}).ToIDMapOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]ID)["baz"], iv)
}

func TestMapIndex(t *testing.T) {
	out := (Map{"baz": String("any")}).ToMapOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]interface{})["baz"], iv)
}

func TestIntMapIndex(t *testing.T) {
	out := (IntMap{"baz": Int(42)}).ToIntMapOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]int)["baz"], iv)
}

func TestInt16MapIndex(t *testing.T) {
	out := (Int16Map{"baz": Int16(33)}).ToInt16MapOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]int16)["baz"], iv)
}

func TestInt32MapIndex(t *testing.T) {
	out := (Int32Map{"baz": Int32(24)}).ToInt32MapOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]int32)["baz"], iv)
}

func TestInt64MapIndex(t *testing.T) {
	out := (Int64Map{"baz": Int64(15)}).ToInt64MapOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]int64)["baz"], iv)
}

func TestInt8MapIndex(t *testing.T) {
	out := (Int8Map{"baz": Int8(6)}).ToInt8MapOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]int8)["baz"], iv)
}

func TestStringMapIndex(t *testing.T) {
	out := (StringMap{"baz": String("foo")}).ToStringMapOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]string)["baz"], iv)
}

func TestURNMapIndex(t *testing.T) {
	out := (URNMap{"baz": URN("foo")}).ToURNMapOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]URN)["baz"], iv)
}

func TestUintMapIndex(t *testing.T) {
	out := (UintMap{"baz": Uint(42)}).ToUintMapOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]uint)["baz"], iv)
}

func TestUint16MapIndex(t *testing.T) {
	out := (Uint16Map{"baz": Uint16(33)}).ToUint16MapOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]uint16)["baz"], iv)
}

func TestUint32MapIndex(t *testing.T) {
	out := (Uint32Map{"baz": Uint32(24)}).ToUint32MapOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]uint32)["baz"], iv)
}

func TestUint64MapIndex(t *testing.T) {
	out := (Uint64Map{"baz": Uint64(15)}).ToUint64MapOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]uint64)["baz"], iv)
}

func TestUint8MapIndex(t *testing.T) {
	out := (Uint8Map{"baz": Uint8(6)}).ToUint8MapOutput()

	av, known, _, err := await(out)
	assert.True(t, known)
	assert.NoError(t, err)

	iv, known, _, err := await(out.MapIndex(String("baz")))
	assert.True(t, known)
	assert.NoError(t, err)

	assert.EqualValues(t, av.(map[string]uint8)["baz"], iv)
}
