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
	"math"
	"reflect"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// asAnySlice converts []T to []interface{} by reflection, simulating covariance.
func asAnySlice(t *testing.T, values interface{}) []interface{} {
	v := reflect.ValueOf(values)
	// use reflect.valueOf to iterate over items of values
	require.Equalf(t, v.Kind(), reflect.Slice, "expected a slice, got %v", v.Type())

	out := slice.Prealloc[interface{}](v.Len())
	for i := 0; i < v.Len(); i++ {
		out = append(out, v.Index(i).Interface())
	}

	return out
}

func TestArchiveArrayContravariance(t *testing.T) {
	t.Parallel()

	plain := []Archive{NewAssetArchive(make(map[string]interface{}))}

	anyout := Any(asAnySlice(t, plain))
	out := anyout.AsArchiveArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	require.NoError(t, err)

	assert.EqualValues(t, av, plain)
}

func TestAssetArrayContravariance(t *testing.T) {
	t.Parallel()

	plain := []Asset{NewStringAsset("foo")}

	anyout := Any(asAnySlice(t, plain))
	out := anyout.AsAssetArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	require.NoError(t, err)

	assert.EqualValues(t, av, plain)
}

func TestAssetOrArchiveArrayContravariance(t *testing.T) {
	t.Parallel()

	plain := []AssetOrArchive{NewStringAsset("foo"), NewAssetArchive(make(map[string]interface{}))}

	anyout := Any(asAnySlice(t, plain))
	out := anyout.AsAssetOrArchiveArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	require.NoError(t, err)

	assert.EqualValues(t, av, plain)
}

func TestBoolArrayContravariance(t *testing.T) {
	t.Parallel()

	plain := []bool{true, false} // an exhaustive test

	anyout := Any(asAnySlice(t, plain))
	out := anyout.AsBoolArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	require.NoError(t, err)

	assert.EqualValues(t, av, plain)
}

func TestFloat64ArrayContravariance(t *testing.T) {
	t.Parallel()

	plain := []float64{0.0, 3.141592653589, -1.0, math.Inf(1)}

	anyout := Any(asAnySlice(t, plain))
	out := anyout.AsFloat64ArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	require.NoError(t, err)

	assert.EqualValues(t, av, plain)
}

func TestIDArrayContravariance(t *testing.T) {
	t.Parallel()

	plain := []ID{ID("foo")}

	anyout := Any(asAnySlice(t, plain))
	out := anyout.AsIDArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	require.NoError(t, err)

	assert.EqualValues(t, av, plain)
}

func TestIntArrayContravariance(t *testing.T) {
	t.Parallel()

	plain := []int{0, 1, 2147483647, -2147483648}

	anyout := Any(asAnySlice(t, plain))
	out := anyout.AsIntArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	require.NoError(t, err)

	assert.EqualValues(t, av, plain)
}

func TestStringArrayContravariance(t *testing.T) {
	t.Parallel()

	plain := []string{"foo"}

	anyout := Any(asAnySlice(t, plain))
	out := anyout.AsStringArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	require.NoError(t, err)

	assert.EqualValues(t, av, plain)
}

func TestURNArrayContravariance(t *testing.T) {
	t.Parallel()

	plain := []URN{URN("foo")}

	anyout := Any(asAnySlice(t, plain))
	out := anyout.AsURNArrayOutput()

	av, known, _, _, err := await(out)
	assert.True(t, known)
	require.NoError(t, err)

	assert.EqualValues(t, av, plain)
}
