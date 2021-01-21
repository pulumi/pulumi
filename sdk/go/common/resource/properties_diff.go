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
	"sort"
)

// ObjectDiff holds the results of diffing two object property maps.
type ObjectDiff struct {
	Adds    PropertyMap               // properties in this map are created in the new.
	Deletes PropertyMap               // properties in this map are deleted from the new.
	Sames   PropertyMap               // properties in this map are the same.
	Updates map[PropertyKey]ValueDiff // properties in this map are changed in the new.
}

// Added returns true if the property 'k' has been added in the new property set.
func (diff *ObjectDiff) Added(k PropertyKey) bool {
	_, has := diff.Adds[k]
	return has
}

// Deleted returns true if the property 'k' has been deleted from the new property set.
func (diff *ObjectDiff) Deleted(k PropertyKey) bool {
	_, has := diff.Deletes[k]
	return has
}

// Updated returns true if the property 'k' has been changed between new and old property sets.
func (diff *ObjectDiff) Updated(k PropertyKey) bool {
	_, has := diff.Updates[k]
	return has
}

// Changed returns true if the property 'k' is known to be different between old and new.
func (diff *ObjectDiff) Changed(k PropertyKey) bool {
	return diff.Added(k) || diff.Deleted(k) || diff.Updated(k)
}

// Same returns true if the property 'k' is *not* known to be different; note that this isn't the same as looking up in
// the Sames map, because it is possible the key is simply missing altogether (as is the case for nulls).
func (diff *ObjectDiff) Same(k PropertyKey) bool {
	return !diff.Changed(k)
}

// Keys returns a stable snapshot of all keys known to this object, across adds, deletes, sames, and updates.
func (diff *ObjectDiff) Keys() []PropertyKey {
	var ks []PropertyKey
	for k := range diff.Adds {
		ks = append(ks, k)
	}
	for k := range diff.Deletes {
		ks = append(ks, k)
	}
	for k := range diff.Sames {
		ks = append(ks, k)
	}
	for k := range diff.Updates {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool { return ks[i] < ks[j] })
	return ks
}

// ValueDiff holds the results of diffing two property values.
type ValueDiff struct {
	Old    PropertyValue // the old value.
	New    PropertyValue // the new value.
	Array  *ArrayDiff    // the array's detailed diffs (only for arrays).
	Object *ObjectDiff   // the object's detailed diffs (only for objects).
}

// ArrayDiff holds the results of diffing two arrays of property values.
type ArrayDiff struct {
	Adds    map[int]PropertyValue // elements added in the new.
	Deletes map[int]PropertyValue // elements deleted in the new.
	Sames   map[int]PropertyValue // elements the same in both.
	Updates map[int]ValueDiff     // elements that have changed in the new.
}

// Len computes the length of this array, taking into account adds, deletes, sames, and updates.
func (diff *ArrayDiff) Len() int {
	len := 0
	for i := range diff.Adds {
		if i+1 > len {
			len = i + 1
		}
	}
	for i := range diff.Deletes {
		if i+1 > len {
			len = i + 1
		}
	}
	for i := range diff.Sames {
		if i+1 > len {
			len = i + 1
		}
	}
	for i := range diff.Updates {
		if i+1 > len {
			len = i + 1
		}
	}
	return len
}

// IgnoreKeyFunc is the callback type for Diff's ignore option.
type IgnoreKeyFunc func(key PropertyKey) bool

// Diff returns a diffset by comparing the property map to another; it returns nil if there are no diffs.
func (props PropertyMap) Diff(other PropertyMap, ignoreKeys ...IgnoreKeyFunc) *ObjectDiff {
	adds := make(PropertyMap)
	deletes := make(PropertyMap)
	sames := make(PropertyMap)
	updates := make(map[PropertyKey]ValueDiff)

	ignore := func(key PropertyKey) bool {
		for _, ikf := range ignoreKeys {
			if ikf(key) {
				return true
			}
		}
		return false
	}

	// First find any updates or deletes.
	for k, old := range props {
		if ignore(k) {
			continue
		}

		if new, has := other[k]; has {
			// If a new exists, use it; for output properties, however, ignore differences.
			if new.IsOutput() {
				sames[k] = old
			} else if diff := old.Diff(new, ignoreKeys...); diff != nil {
				if !old.HasValue() {
					adds[k] = new
				} else if !new.HasValue() {
					deletes[k] = old
				} else {
					updates[k] = *diff
				}
			} else {
				sames[k] = old
			}
		} else if old.HasValue() {
			// If there was no new property, it has been deleted.
			deletes[k] = old
		}
	}

	// Next find any additions not in the old map.
	for k, new := range other {
		if ignore(k) {
			continue
		}

		if _, has := props[k]; !has && new.HasValue() {
			adds[k] = new
		}
	}

	// If no diffs were found, return nil; else return a diff structure.
	if len(adds) == 0 && len(deletes) == 0 && len(updates) == 0 {
		return nil
	}
	return &ObjectDiff{
		Adds:    adds,
		Deletes: deletes,
		Sames:   sames,
		Updates: updates,
	}
}

// Diff returns a diff by comparing a single property value to another; it returns nil if there are no diffs.
func (v PropertyValue) Diff(other PropertyValue, ignoreKeys ...IgnoreKeyFunc) *ValueDiff {
	if v.IsArray() && other.IsArray() {
		old := v.ArrayValue()
		new := other.ArrayValue()
		// If any elements exist in the new array but not the old, track them as adds.
		adds := make(map[int]PropertyValue)
		for i := len(old); i < len(new); i++ {
			adds[i] = new[i]
		}
		// If any elements exist in the old array but not the new, track them as adds.
		deletes := make(map[int]PropertyValue)
		for i := len(new); i < len(old); i++ {
			deletes[i] = old[i]
		}
		// Now if elements exist in both, track them as sames or updates.
		sames := make(map[int]PropertyValue)
		updates := make(map[int]ValueDiff)
		for i := 0; i < len(old) && i < len(new); i++ {
			if diff := old[i].Diff(new[i]); diff != nil {
				updates[i] = *diff
			} else {
				sames[i] = old[i]
			}
		}

		if len(adds) == 0 && len(deletes) == 0 && len(updates) == 0 {
			return nil
		}
		return &ValueDiff{
			Old: v,
			New: other,
			Array: &ArrayDiff{
				Adds:    adds,
				Deletes: deletes,
				Sames:   sames,
				Updates: updates,
			},
		}
	}
	if v.IsObject() && other.IsObject() {
		old := v.ObjectValue()
		new := other.ObjectValue()
		if diff := old.Diff(new, ignoreKeys...); diff != nil {
			return &ValueDiff{
				Old:    v,
				New:    other,
				Object: diff,
			}
		}
		return nil
	}

	// If we got here, either the values are primitives, or they weren't the same type; do a simple diff.
	if v.DeepEquals(other) {
		return nil
	}
	return &ValueDiff{Old: v, New: other}
}

// DeepEquals returns true if this property map is deeply equal to the other property map; and false otherwise.
func (props PropertyMap) DeepEquals(other PropertyMap) bool {
	// If any in props either doesn't exist, or is of a different value, return false.
	for _, k := range props.StableKeys() {
		v := props[k]
		if p, has := other[k]; has {
			if !v.DeepEquals(p) {
				return false
			}
		} else if v.HasValue() {
			return false
		}
	}

	// If the other map has properties that this map doesn't have, return false.
	for _, k := range other.StableKeys() {
		if _, has := props[k]; !has && other[k].HasValue() {
			return false
		}
	}

	return true
}

// DeepEquals returns true if this property map is deeply equal to the other property map; and false otherwise.
func (v PropertyValue) DeepEquals(other PropertyValue) bool {
	// Arrays are equal if they are both of the same size and elements are deeply equal.
	if v.IsArray() {
		if !other.IsArray() {
			return false
		}
		va := v.ArrayValue()
		oa := other.ArrayValue()
		if len(va) != len(oa) {
			return false
		}
		for i, elem := range va {
			if !elem.DeepEquals(oa[i]) {
				return false
			}
		}
		return true
	}

	// Assets and archives enjoy value equality.
	if v.IsAsset() {
		if !other.IsAsset() {
			return false
		}
		return v.AssetValue().Equals(other.AssetValue())
	} else if v.IsArchive() {
		if !other.IsArchive() {
			return false
		}
		return v.ArchiveValue().Equals(other.ArchiveValue())
	}

	// Object values are equal if their contents are deeply equal.
	if v.IsObject() {
		if !other.IsObject() {
			return false
		}
		vo := v.ObjectValue()
		oa := other.ObjectValue()
		return vo.DeepEquals(oa)
	}

	// Secret are equal if the value they wrap are equal.
	if v.IsSecret() {
		if !other.IsSecret() {
			return false
		}
		vs := v.SecretValue()
		os := other.SecretValue()

		return vs.Element.DeepEquals(os.Element)
	}

	// Resource references are equal if they refer to the same resource. The package version is ignored.
	if v.IsResourceReference() {
		if !other.IsResourceReference() {
			return false
		}
		vr := v.ResourceReferenceValue()
		or := other.ResourceReferenceValue()

		return vr.URN == or.URN && vr.ID == or.ID
	}

	// For all other cases, primitives are equal if their values are equal.
	return v.V == other.V
}
