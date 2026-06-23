// Copyright 2026, Pulumi Corporation.
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

package property

import (
	"archive/tar"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func assertDeepEqualsIffEmptyDiff(t *testing.T, val1, val2 Value) {
	diff := val1.Diff(val2)
	equals := reflect.DeepEqual(val1, val2)
	assert.Equal(t, diff == nil, equals, "Equal <--> empty diff")
}

func TestNullPropertyValueDiffs(t *testing.T) {
	t.Parallel()
	d1 := New(Null).Diff(New(Null))
	assert.Nil(t, d1)
	d2 := New(Null).Diff(New(true))
	require.NotNil(t, d2)
	assert.Nil(t, d2.Array)
	assert.Nil(t, d2.Object)
	assert.True(t, d2.Old.IsNull())
	assert.True(t, d2.New.IsBool())
	assert.Equal(t, true, d2.New.AsBool())
}

func TestBoolPropertyValueDiffs(t *testing.T) {
	t.Parallel()
	d1 := New(true).Diff(New(true))
	assert.Nil(t, d1)
	d2 := New(true).Diff(New(false))
	require.NotNil(t, d2)
	assert.Nil(t, d2.Array)
	assert.Nil(t, d2.Object)
	assert.True(t, d2.Old.IsBool())
	assert.Equal(t, true, d2.Old.AsBool())
	assert.True(t, d2.New.IsBool())
	assert.Equal(t, false, d2.New.AsBool())
	d3 := New(true).Diff(New(Null))
	require.NotNil(t, d3)
	assert.Nil(t, d3.Array)
	assert.Nil(t, d3.Object)
	assert.True(t, d3.Old.IsBool())
	assert.Equal(t, true, d3.Old.AsBool())
	assert.True(t, d3.New.IsNull())
}

func TestNumberPropertyValueDiffs(t *testing.T) {
	t.Parallel()
	d1 := New(42.0).Diff(New(42.0))
	assert.Nil(t, d1)
	d2 := New(42.0).Diff(New(66.0))
	require.NotNil(t, d2)
	assert.Nil(t, d2.Array)
	assert.Nil(t, d2.Object)
	assert.True(t, d2.Old.IsNumber())
	assert.Equal(t, float64(42), d2.Old.AsNumber())
	assert.True(t, d2.New.IsNumber())
	assert.Equal(t, float64(66), d2.New.AsNumber())
	d3 := New(88.0).Diff(New(true))
	require.NotNil(t, d3)
	assert.Nil(t, d3.Array)
	assert.Nil(t, d3.Object)
	assert.True(t, d3.Old.IsNumber())
	assert.Equal(t, float64(88), d3.Old.AsNumber())
	assert.True(t, d3.New.IsBool())
	assert.Equal(t, true, d3.New.AsBool())
}

func TestStringPropertyValueDiffs(t *testing.T) {
	t.Parallel()
	d1 := New("a string").Diff(New("a string"))
	assert.Nil(t, d1)
	d2 := New("a string").Diff(New("some other string"))
	require.NotNil(t, d2)
	assert.True(t, d2.Old.IsString())
	assert.Equal(t, "a string", d2.Old.AsString())
	assert.True(t, d2.New.IsString())
	assert.Equal(t, "some other string", d2.New.AsString())
	d3 := New("what a string").Diff(New(973.0))
	require.NotNil(t, d3)
	assert.Nil(t, d3.Array)
	assert.Nil(t, d3.Object)
	assert.True(t, d3.Old.IsString())
	assert.Equal(t, "what a string", d3.Old.AsString(), "what a string")
	assert.True(t, d3.New.IsNumber())
	assert.Equal(t, float64(973), d3.New.AsNumber())
}

func TestArrayPropertyValueDiffs(t *testing.T) {
	t.Parallel()
	// no diffs:
	d1 := New([]Value{}).Diff(New([]Value{}))
	assert.Nil(t, d1)
	d2 := New([]Value{
		New("element one"), New(2.0), New(Null),
	}).Diff(New([]Value{
		New("element one"), New(2.0), New(Null),
	}))
	assert.Nil(t, d2)
	// all updates:
	d3a1 := New([]Value{
		New("element one"), New(2.0), New(Null),
	})
	d3a2 := New([]Value{
		New(1.0), New(Null), New("element three"),
	})
	assertDeepEqualsIffEmptyDiff(t, d3a1, d3a2)
	d3 := d3a1.Diff(d3a2)
	require.NotNil(t, d3)
	require.NotNil(t, d3.Array)
	assert.Nil(t, d3.Object)
	assert.Empty(t, d3.Array.Adds)
	assert.Empty(t, d3.Array.Deletes)
	assert.Empty(t, d3.Array.Sames)
	require.Len(t, d3.Array.Updates, 3)
	for i, update := range d3.Array.Updates {
		assert.Equal(t, d3a1.AsArray().Get(i), update.Old)
		assert.Equal(t, d3a2.AsArray().Get(i), update.New)
	}
	// update one, keep one, delete one:
	d4a1 := New([]Value{
		New("element one"), New(2.0), New(true),
	})
	d4a2 := New([]Value{
		New("element 1"), New(2.0),
	})
	assertDeepEqualsIffEmptyDiff(t, d4a1, d4a2)
	d4 := d4a1.Diff(d4a2)
	require.NotNil(t, d4)
	require.NotNil(t, d4.Array)
	assert.Nil(t, d4.Object)
	assert.Empty(t, d4.Array.Adds)
	require.Len(t, d4.Array.Deletes, 1)
	for i, delete := range d4.Array.Deletes {
		assert.Equal(t, 2, i)
		assert.Equal(t, d4a1.AsArray().Get(i), delete)
	}
	require.Len(t, d4.Array.Sames, 1)
	for i, same := range d4.Array.Sames {
		assert.Equal(t, 1, i)
		assert.Equal(t, d4a1.AsArray().Get(i), same)
		assert.Equal(t, d4a2.AsArray().Get(i), same)
	}
	require.Len(t, d4.Array.Updates, 1)
	for i, update := range d4.Array.Updates {
		assert.Equal(t, 0, i)
		assert.Equal(t, d4a1.AsArray().Get(i), update.Old)
		assert.Equal(t, d4a2.AsArray().Get(i), update.New)
	}
	// keep one, update one, add one:
	d5a1 := New([]Value{
		New("element one"), New(2.0),
	})
	d5a2 := New([]Value{
		New("element 1"), New(2.0), New(true),
	})
	assertDeepEqualsIffEmptyDiff(t, d5a1, d5a2)
	d5 := d5a1.Diff(d5a2)
	require.NotNil(t, d5)
	require.NotNil(t, d5.Array)
	assert.Nil(t, d5.Object)
	require.Len(t, d5.Array.Adds, 1)
	for i, add := range d5.Array.Adds {
		assert.Equal(t, 2, i)
		assert.Equal(t, d5a2.AsArray().Get(i), add)
	}
	assert.Empty(t, d5.Array.Deletes)
	require.Len(t, d5.Array.Sames, 1)
	for i, same := range d5.Array.Sames {
		assert.Equal(t, 1, i)
		assert.Equal(t, d5a1.AsArray().Get(i), same)
		assert.Equal(t, d5a2.AsArray().Get(i), same)
	}
	require.Len(t, d5.Array.Updates, 1)
	for i, update := range d5.Array.Updates {
		assert.Equal(t, 0, i)
		assert.Equal(t, d5a1.AsArray().Get(i), update.Old)
		assert.Equal(t, d5a2.AsArray().Get(i), update.New)
	}
	// from nil to empty array:
	d6 := New(Null).Diff(New([]Value{}))
	require.NotNil(t, d6)
}

func TestObjectPropertyValueDiffs(t *testing.T) {
	t.Parallel()
	// no diffs:
	d1 := Map{}.Diff(Map{})
	assert.Nil(t, d1)
	d2 := NewMap(map[string]Value{
		"a": New(true),
	}).Diff(NewMap(map[string]Value{
		"a": New(true),
	}))
	assert.Nil(t, d2)
	// all updates:
	{
		obj1 := NewMap(map[string]Value{
			"prop-a": New(true),
			"prop-b": New("bbb"),
			"prop-c": New(NewMap(map[string]Value{
				"inner-prop-a": New(673.0),
			})),
		})
		obj2 := NewMap(map[string]Value{
			"prop-a": New(false),
			"prop-b": New(89.0),
			"prop-c": New(NewMap(map[string]Value{
				"inner-prop-a": New(672.0),
			})),
		})
		assertDeepEqualsIffEmptyDiff(t, New(obj1), New(obj2))
		d3 := obj1.Diff(obj2)
		require.NotNil(t, d3)
		assert.Empty(t, d3.Adds)
		assert.Empty(t, d3.Deletes)
		assert.Empty(t, d3.Sames)
		require.Len(t, d3.Updates, 3)
		d3pa := d3.Updates["prop-a"]
		assert.Nil(t, d3pa.Array)
		assert.Nil(t, d3pa.Object)
		assert.True(t, d3pa.Old.IsBool())
		assert.Equal(t, true, d3pa.Old.AsBool())
		assert.True(t, d3pa.Old.IsBool())
		assert.Equal(t, false, d3pa.New.AsBool())
		d3pb := d3.Updates["prop-b"]
		assert.Nil(t, d3pb.Array)
		assert.Nil(t, d3pb.Object)
		assert.True(t, d3pb.Old.IsString())
		assert.Equal(t, "bbb", d3pb.Old.AsString())
		assert.True(t, d3pb.New.IsNumber())
		assert.Equal(t, float64(89), d3pb.New.AsNumber())
		d3pc := d3.Updates["prop-c"]
		assert.Nil(t, d3pc.Array)
		require.NotNil(t, d3pc.Object)
		assert.Empty(t, d3pc.Object.Adds)
		assert.Empty(t, d3pc.Object.Deletes)
		assert.Empty(t, d3pc.Object.Sames)
		require.Len(t, d3pc.Object.Updates, 1)
		d3pcu := d3pc.Object.Updates["inner-prop-a"]
		assert.True(t, d3pcu.Old.IsNumber())
		assert.Equal(t, float64(673), d3pcu.Old.AsNumber())
		assert.True(t, d3pcu.New.IsNumber())
		assert.Equal(t, float64(672), d3pcu.New.AsNumber())
	}
	// add one (1 missing key), update two, keep two, delete two (1 missing key, 1 null).
	{
		obj1 := NewMap(map[string]Value{
			"prop-a-2": New(Null),
			"prop-b":   New("bbb"),
			"prop-c-1": New(6767.0),
			"prop-c-2": New(Null),
			"prop-d-1": New(true),
			"prop-d-2": New(false),
		})
		obj2 := NewMap(map[string]Value{
			"prop-a-1": New("a fresh value"),
			"prop-a-2": New("a non-nil value"),
			"prop-b":   New(89.0),
			"prop-c-1": New(6767.0),
			"prop-c-2": New(Null),
			"prop-d-2": New(Null),
		})
		assertDeepEqualsIffEmptyDiff(t, New(obj1), New(obj2))
		d4 := obj1.Diff(obj2)
		require.NotNil(t, d4)
		require.Len(t, d4.Adds, 2)
		assert.Equal(t, obj2.Get("prop-a-1"), d4.Adds["prop-a-1"])
		assert.Equal(t, obj2.Get("prop-a-2"), d4.Adds["prop-a-2"])
		require.Len(t, d4.Deletes, 2)
		assert.Equal(t, obj1.Get("prop-d-1"), d4.Deletes["prop-d-1"])
		assert.Equal(t, obj1.Get("prop-d-2"), d4.Deletes["prop-d-2"])
		require.Len(t, d4.Sames, 2)
		assert.Equal(t, obj1.Get("prop-c-1"), d4.Sames["prop-c-1"])
		assert.Equal(t, obj1.Get("prop-c-2"), d4.Sames["prop-c-2"])
		assert.Equal(t, obj2.Get("prop-c-1"), d4.Sames["prop-c-1"])
		assert.Equal(t, obj2.Get("prop-c-2"), d4.Sames["prop-c-2"])
		require.Len(t, d4.Updates, 1)
		assert.Equal(t, obj1.Get("prop-b"), d4.Updates["prop-b"].Old)
		assert.Equal(t, obj2.Get("prop-b"), d4.Updates["prop-b"].New)
	}
}

func TestAssetPropertyValueDiffs(t *testing.T) {
	t.Parallel()
	a1, err := asset.FromText("test")
	require.NoError(t, err)
	d1 := New(a1).Diff(New(a1))
	assert.Nil(t, d1)
	a2, err := asset.FromText("test2")
	require.NoError(t, err)
	d2 := New(a1).Diff(New(a2))
	require.NotNil(t, d2)
	assert.Nil(t, d2.Array)
	assert.Nil(t, d2.Object)
	assert.True(t, d2.Old.IsAsset())
	assert.Equal(t, "test", d2.Old.AsAsset().Text)
	assert.True(t, d2.New.IsAsset())
	assert.Equal(t, "test2", d2.New.AsAsset().Text)
	d3 := New(a1).Diff(New(Null))
	require.NotNil(t, d3)
	assert.Nil(t, d3.Array)
	assert.Nil(t, d3.Object)
	assert.True(t, d3.Old.IsAsset())
	assert.Equal(t, "test", d3.Old.AsAsset().Text)
	assert.True(t, d3.New.IsNull())
}

func tempArchive(prefix string, fill bool) (string, error) {
	for {
		path := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%x.tar", prefix, rand.Uint32())) //nolint:gosec
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		switch {
		case os.IsExist(err):
			continue
		case err != nil:
			return "", err
		default:
			defer contract.IgnoreClose(f)

			// write out a tar file. if `fill` is true, add a single empty file.
			if fill {
				w := tar.NewWriter(f)
				defer contract.IgnoreClose(w)

				err = w.WriteHeader(&tar.Header{
					Name: "file",
					Mode: 0o600,
					Size: 0,
				})
			}
			return path, err
		}
	}
}

func TestArchivePropertyValueDiffs(t *testing.T) {
	t.Parallel()
	path, err := tempArchive("test", false)
	require.NoError(t, err)
	defer func() { contract.IgnoreError(os.Remove(path)) }()
	a1, err := archive.FromPath(path)
	require.NoError(t, err)
	d1 := New(a1).Diff(New(a1))
	assert.Nil(t, d1)
	path2, err := tempArchive("test2", true)
	require.NoError(t, err)
	defer func() { contract.IgnoreError(os.Remove(path)) }()
	a2, err := archive.FromPath(path2)
	require.NoError(t, err)
	d2 := New(a1).Diff(New(a2))
	require.NotNil(t, d2)
	assert.Nil(t, d2.Array)
	assert.Nil(t, d2.Object)
	assert.True(t, d2.Old.IsArchive())
	assert.Equal(t, path, d2.Old.AsArchive().Path)
	assert.True(t, d2.New.IsArchive())
	assert.Equal(t, path2, d2.New.AsArchive().Path)
	d3 := New(a1).Diff(New(Null))
	require.NotNil(t, d3)
	assert.Nil(t, d3.Array)
	assert.Nil(t, d3.Object)
	assert.True(t, d3.Old.IsArchive())
	assert.Equal(t, path, d3.Old.AsArchive().Path)
	assert.True(t, d3.New.IsNull())
}

func TestMismatchedPropertyValueDiff(t *testing.T) {
	t.Parallel()

	s1 := New([]Value{New("a"), New("b"), New("c")}).WithSecret(true)
	s2 := New([]Value{New("a"), New("b"), New("c")}).WithSecret(true)

	assert.True(t, reflect.DeepEqual(s2, s1))
	assert.True(t, reflect.DeepEqual(s1, s2))
}

func TestComputedProperyValueDiff(t *testing.T) {
	t.Parallel()

	a1 := New(Computed)
	a2 := New(Computed)
	assert.True(t, reflect.DeepEqual(a1, a2))

	a3 := New("a")
	assert.False(t, reflect.DeepEqual(a1, a3))
}
