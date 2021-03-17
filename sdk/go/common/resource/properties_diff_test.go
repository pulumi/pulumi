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

package resource

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func TestNullPropertyValueDiffs(t *testing.T) {
	t.Parallel()
	d1 := NewNullProperty().Diff(NewNullProperty())
	assert.Nil(t, d1)
	d2 := NewNullProperty().Diff(NewBoolProperty(true))
	assert.NotNil(t, d2)
	assert.Nil(t, d2.Array)
	assert.Nil(t, d2.Object)
	assert.True(t, d2.Old.IsNull())
	assert.True(t, d2.New.IsBool())
	assert.Equal(t, true, d2.New.BoolValue())
}

func TestBoolPropertyValueDiffs(t *testing.T) {
	t.Parallel()
	d1 := NewBoolProperty(true).Diff(NewBoolProperty(true))
	assert.Nil(t, d1)
	d2 := NewBoolProperty(true).Diff(NewBoolProperty(false))
	assert.NotNil(t, d2)
	assert.Nil(t, d2.Array)
	assert.Nil(t, d2.Object)
	assert.True(t, d2.Old.IsBool())
	assert.Equal(t, true, d2.Old.BoolValue())
	assert.True(t, d2.New.IsBool())
	assert.Equal(t, false, d2.New.BoolValue())
	d3 := NewBoolProperty(true).Diff(NewNullProperty())
	assert.NotNil(t, d3)
	assert.Nil(t, d3.Array)
	assert.Nil(t, d3.Object)
	assert.True(t, d3.Old.IsBool())
	assert.Equal(t, true, d3.Old.BoolValue())
	assert.True(t, d3.New.IsNull())
}

func TestNumberPropertyValueDiffs(t *testing.T) {
	t.Parallel()
	d1 := NewNumberProperty(42).Diff(NewNumberProperty(42))
	assert.Nil(t, d1)
	d2 := NewNumberProperty(42).Diff(NewNumberProperty(66))
	assert.NotNil(t, d2)
	assert.Nil(t, d2.Array)
	assert.Nil(t, d2.Object)
	assert.True(t, d2.Old.IsNumber())
	assert.Equal(t, float64(42), d2.Old.NumberValue())
	assert.True(t, d2.New.IsNumber())
	assert.Equal(t, float64(66), d2.New.NumberValue())
	d3 := NewNumberProperty(88).Diff(NewBoolProperty(true))
	assert.NotNil(t, d3)
	assert.Nil(t, d3.Array)
	assert.Nil(t, d3.Object)
	assert.True(t, d3.Old.IsNumber())
	assert.Equal(t, float64(88), d3.Old.NumberValue())
	assert.True(t, d3.New.IsBool())
	assert.Equal(t, true, d3.New.BoolValue())
}

func TestStringPropertyValueDiffs(t *testing.T) {
	t.Parallel()
	d1 := NewStringProperty("a string").Diff(NewStringProperty("a string"))
	assert.Nil(t, d1)
	d2 := NewStringProperty("a string").Diff(NewStringProperty("some other string"))
	assert.NotNil(t, d2)
	assert.True(t, d2.Old.IsString())
	assert.Equal(t, "a string", d2.Old.StringValue())
	assert.True(t, d2.New.IsString())
	assert.Equal(t, "some other string", d2.New.StringValue())
	d3 := NewStringProperty("what a string").Diff(NewNumberProperty(973))
	assert.NotNil(t, d3)
	assert.Nil(t, d3.Array)
	assert.Nil(t, d3.Object)
	assert.True(t, d3.Old.IsString())
	assert.Equal(t, "what a string", d3.Old.StringValue(), "what a string")
	assert.True(t, d3.New.IsNumber())
	assert.Equal(t, float64(973), d3.New.NumberValue())
}

func TestArrayPropertyValueDiffs(t *testing.T) {
	t.Parallel()
	// no diffs:
	d1 := NewArrayProperty([]PropertyValue{}).Diff(NewArrayProperty([]PropertyValue{}))
	assert.Nil(t, d1)
	d2 := NewArrayProperty([]PropertyValue{
		NewStringProperty("element one"), NewNumberProperty(2), NewNullProperty(),
	}).Diff(NewArrayProperty([]PropertyValue{
		NewStringProperty("element one"), NewNumberProperty(2), NewNullProperty(),
	}))
	assert.Nil(t, d2)
	// all updates:
	d3a1 := NewArrayProperty([]PropertyValue{
		NewStringProperty("element one"), NewNumberProperty(2), NewNullProperty()})
	d3a2 := NewArrayProperty([]PropertyValue{
		NewNumberProperty(1), NewNullProperty(), NewStringProperty("element three")})
	d3 := d3a1.Diff(d3a2)
	assert.NotNil(t, d3)
	assert.NotNil(t, d3.Array)
	assert.Nil(t, d3.Object)
	assert.Equal(t, 0, len(d3.Array.Adds))
	assert.Equal(t, 0, len(d3.Array.Deletes))
	assert.Equal(t, 0, len(d3.Array.Sames))
	assert.Equal(t, 3, len(d3.Array.Updates))
	for i, update := range d3.Array.Updates {
		assert.Equal(t, d3a1.ArrayValue()[i], update.Old)
		assert.Equal(t, d3a2.ArrayValue()[i], update.New)
	}
	// update one, keep one, delete one:
	d4a1 := NewArrayProperty([]PropertyValue{
		NewStringProperty("element one"), NewNumberProperty(2), NewBoolProperty(true)})
	d4a2 := NewArrayProperty([]PropertyValue{
		NewStringProperty("element 1"), NewNumberProperty(2)})
	d4 := d4a1.Diff(d4a2)
	assert.NotNil(t, d4)
	assert.NotNil(t, d4.Array)
	assert.Nil(t, d4.Object)
	assert.Equal(t, 0, len(d4.Array.Adds))
	assert.Equal(t, 1, len(d4.Array.Deletes))
	for i, delete := range d4.Array.Deletes {
		assert.Equal(t, 2, i)
		assert.Equal(t, d4a1.ArrayValue()[i], delete)
	}
	assert.Equal(t, 1, len(d4.Array.Sames))
	for i, same := range d4.Array.Sames {
		assert.Equal(t, 1, i)
		assert.Equal(t, d4a1.ArrayValue()[i], same)
		assert.Equal(t, d4a2.ArrayValue()[i], same)
	}
	assert.Equal(t, 1, len(d4.Array.Updates))
	for i, update := range d4.Array.Updates {
		assert.Equal(t, 0, i)
		assert.Equal(t, d4a1.ArrayValue()[i], update.Old)
		assert.Equal(t, d4a2.ArrayValue()[i], update.New)
	}
	// keep one, update one, add one:
	d5a1 := NewArrayProperty([]PropertyValue{
		NewStringProperty("element one"), NewNumberProperty(2)})
	d5a2 := NewArrayProperty([]PropertyValue{
		NewStringProperty("element 1"), NewNumberProperty(2), NewBoolProperty(true)})
	d5 := d5a1.Diff(d5a2)
	assert.NotNil(t, d5)
	assert.NotNil(t, d5.Array)
	assert.Nil(t, d5.Object)
	assert.Equal(t, 1, len(d5.Array.Adds))
	for i, add := range d5.Array.Adds {
		assert.Equal(t, 2, i)
		assert.Equal(t, d5a2.ArrayValue()[i], add)
	}
	assert.Equal(t, 0, len(d5.Array.Deletes))
	assert.Equal(t, 1, len(d5.Array.Sames))
	for i, same := range d5.Array.Sames {
		assert.Equal(t, 1, i)
		assert.Equal(t, d5a1.ArrayValue()[i], same)
		assert.Equal(t, d5a2.ArrayValue()[i], same)
	}
	assert.Equal(t, 1, len(d5.Array.Updates))
	for i, update := range d5.Array.Updates {
		assert.Equal(t, 0, i)
		assert.Equal(t, d5a1.ArrayValue()[i], update.Old)
		assert.Equal(t, d5a2.ArrayValue()[i], update.New)
	}
	// from nil to empty array:
	d6 := NewNullProperty().Diff(NewArrayProperty([]PropertyValue{}))
	assert.NotNil(t, d6)
}

func TestObjectPropertyValueDiffs(t *testing.T) {
	t.Parallel()
	// no diffs:
	d1 := PropertyMap{}.Diff(PropertyMap{})
	assert.Nil(t, d1)
	d2 := PropertyMap{
		PropertyKey("a"): NewBoolProperty(true),
	}.Diff(PropertyMap{
		PropertyKey("a"): NewBoolProperty(true),
	})
	assert.Nil(t, d2)
	// all updates:
	{
		obj1 := PropertyMap{
			PropertyKey("prop-a"): NewBoolProperty(true),
			PropertyKey("prop-b"): NewStringProperty("bbb"),
			PropertyKey("prop-c"): NewObjectProperty(PropertyMap{
				PropertyKey("inner-prop-a"): NewNumberProperty(673),
			}),
		}
		obj2 := PropertyMap{
			PropertyKey("prop-a"): NewBoolProperty(false),
			PropertyKey("prop-b"): NewNumberProperty(89),
			PropertyKey("prop-c"): NewObjectProperty(PropertyMap{
				PropertyKey("inner-prop-a"): NewNumberProperty(672),
			}),
		}
		d3 := obj1.Diff(obj2)
		assert.NotNil(t, d3)
		assert.Equal(t, 0, len(d3.Adds))
		assert.Equal(t, 0, len(d3.Deletes))
		assert.Equal(t, 0, len(d3.Sames))
		assert.Equal(t, 3, len(d3.Updates))
		d3pa := d3.Updates[PropertyKey("prop-a")]
		assert.Nil(t, d3pa.Array)
		assert.Nil(t, d3pa.Object)
		assert.True(t, d3pa.Old.IsBool())
		assert.Equal(t, true, d3pa.Old.BoolValue())
		assert.True(t, d3pa.Old.IsBool())
		assert.Equal(t, false, d3pa.New.BoolValue())
		d3pb := d3.Updates[PropertyKey("prop-b")]
		assert.Nil(t, d3pb.Array)
		assert.Nil(t, d3pb.Object)
		assert.True(t, d3pb.Old.IsString())
		assert.Equal(t, "bbb", d3pb.Old.StringValue())
		assert.True(t, d3pb.New.IsNumber())
		assert.Equal(t, float64(89), d3pb.New.NumberValue())
		d3pc := d3.Updates[PropertyKey("prop-c")]
		assert.Nil(t, d3pc.Array)
		assert.NotNil(t, d3pc.Object)
		assert.Equal(t, 0, len(d3pc.Object.Adds))
		assert.Equal(t, 0, len(d3pc.Object.Deletes))
		assert.Equal(t, 0, len(d3pc.Object.Sames))
		assert.Equal(t, 1, len(d3pc.Object.Updates))
		d3pcu := d3pc.Object.Updates[PropertyKey("inner-prop-a")]
		assert.True(t, d3pcu.Old.IsNumber())
		assert.Equal(t, float64(673), d3pcu.Old.NumberValue())
		assert.True(t, d3pcu.New.IsNumber())
		assert.Equal(t, float64(672), d3pcu.New.NumberValue())
	}
	// add two (1 missing key, 1 null), update one, keep two, delete two (1 missing key, 1 null).
	{
		obj1 := PropertyMap{
			PropertyKey("prop-a-2"): NewNullProperty(),
			PropertyKey("prop-b"):   NewStringProperty("bbb"),
			PropertyKey("prop-c-1"): NewNumberProperty(6767),
			PropertyKey("prop-c-2"): NewNullProperty(),
			PropertyKey("prop-d-1"): NewBoolProperty(true),
			PropertyKey("prop-d-2"): NewBoolProperty(false),
		}
		obj2 := PropertyMap{
			PropertyKey("prop-a-1"): NewStringProperty("a fresh value"),
			PropertyKey("prop-a-2"): NewStringProperty("a non-nil value"),
			PropertyKey("prop-b"):   NewNumberProperty(89),
			PropertyKey("prop-c-1"): NewNumberProperty(6767),
			PropertyKey("prop-c-2"): NewNullProperty(),
			PropertyKey("prop-d-2"): NewNullProperty(),
		}
		d4 := obj1.Diff(obj2)
		assert.NotNil(t, d4)
		assert.Equal(t, 2, len(d4.Adds))
		assert.Equal(t, obj2[PropertyKey("prop-a-1")], d4.Adds[PropertyKey("prop-a-1")])
		assert.Equal(t, obj2[PropertyKey("prop-a-2")], d4.Adds[PropertyKey("prop-a-2")])
		assert.Equal(t, 2, len(d4.Deletes))
		assert.Equal(t, obj1[PropertyKey("prop-d-1")], d4.Deletes[PropertyKey("prop-d-1")])
		assert.Equal(t, obj1[PropertyKey("prop-d-2")], d4.Deletes[PropertyKey("prop-d-2")])
		assert.Equal(t, 2, len(d4.Sames))
		assert.Equal(t, obj1[PropertyKey("prop-c-1")], d4.Sames[PropertyKey("prop-c-1")])
		assert.Equal(t, obj1[PropertyKey("prop-c-2")], d4.Sames[PropertyKey("prop-c-2")])
		assert.Equal(t, obj2[PropertyKey("prop-c-1")], d4.Sames[PropertyKey("prop-c-1")])
		assert.Equal(t, obj2[PropertyKey("prop-c-2")], d4.Sames[PropertyKey("prop-c-2")])
		assert.Equal(t, 1, len(d4.Updates))
		assert.Equal(t, obj1[PropertyKey("prop-b")], d4.Updates[PropertyKey("prop-b")].Old)
		assert.Equal(t, obj2[PropertyKey("prop-b")], d4.Updates[PropertyKey("prop-b")].New)
	}
}

func TestAssetPropertyValueDiffs(t *testing.T) {
	t.Parallel()
	a1, err := NewTextAsset("test")
	assert.Nil(t, err)
	d1 := NewAssetProperty(a1).Diff(NewAssetProperty(a1))
	assert.Nil(t, d1)
	a2, err := NewTextAsset("test2")
	assert.Nil(t, err)
	d2 := NewAssetProperty(a1).Diff(NewAssetProperty(a2))
	assert.NotNil(t, d2)
	assert.Nil(t, d2.Array)
	assert.Nil(t, d2.Object)
	assert.True(t, d2.Old.IsAsset())
	assert.Equal(t, "test", d2.Old.AssetValue().Text)
	assert.True(t, d2.New.IsAsset())
	assert.Equal(t, "test2", d2.New.AssetValue().Text)
	d3 := NewAssetProperty(a1).Diff(NewNullProperty())
	assert.NotNil(t, d3)
	assert.Nil(t, d3.Array)
	assert.Nil(t, d3.Object)
	assert.True(t, d3.Old.IsAsset())
	assert.Equal(t, "test", d3.Old.AssetValue().Text)
	assert.True(t, d3.New.IsNull())
}

func TestArchivePropertyValueDiffs(t *testing.T) {
	t.Parallel()
	path, err := tempArchive("test", false)
	assert.Nil(t, err)
	defer func() { contract.IgnoreError(os.Remove(path)) }()
	a1, err := NewPathArchive(path)
	assert.Nil(t, err)
	d1 := NewArchiveProperty(a1).Diff(NewArchiveProperty(a1))
	assert.Nil(t, d1)
	path2, err := tempArchive("test2", true)
	assert.Nil(t, err)
	defer func() { contract.IgnoreError(os.Remove(path)) }()
	a2, err := NewPathArchive(path2)
	assert.Nil(t, err)
	d2 := NewArchiveProperty(a1).Diff(NewArchiveProperty(a2))
	assert.NotNil(t, d2)
	assert.Nil(t, d2.Array)
	assert.Nil(t, d2.Object)
	assert.True(t, d2.Old.IsArchive())
	assert.Equal(t, path, d2.Old.ArchiveValue().Path)
	assert.True(t, d2.New.IsArchive())
	assert.Equal(t, path2, d2.New.ArchiveValue().Path)
	d3 := NewArchiveProperty(a1).Diff(NewNullProperty())
	assert.NotNil(t, d3)
	assert.Nil(t, d3.Array)
	assert.Nil(t, d3.Object)
	assert.True(t, d3.Old.IsArchive())
	assert.Equal(t, path, d3.Old.ArchiveValue().Path)
	assert.True(t, d3.New.IsNull())
}

func TestMismatchedPropertyValueDiff(t *testing.T) {
	t.Parallel()

	a1 := NewPropertyValue([]string{"a", "b", "c"})
	a2 := NewPropertyValue([]string{"a", "b", "c"})

	s1 := MakeSecret(a1)
	s2 := MakeSecret(a2)

	assert.True(t, s2.DeepEquals(s1))
	assert.True(t, s1.DeepEquals(s2))
}
