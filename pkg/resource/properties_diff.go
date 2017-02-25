// Copyright 2016 Marapongo, Inc. All rights reserved.

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

// Keys returns a stable snapshot of all keys known to this object, across adds, deletes, sames, and updates.
func (diff *ObjectDiff) Keys() []PropertyKey {
	var ks propertyKeys
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
	sort.Sort(ks)
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

// Diff returns a diffset by comparing the property map to another; it returns nil if there are no diffs.
func (props PropertyMap) Diff(other PropertyMap) *ObjectDiff {
	adds := make(PropertyMap)
	deletes := make(PropertyMap)
	sames := make(PropertyMap)
	updates := make(map[PropertyKey]ValueDiff)

	// First find any updates or deletes.
	for k, p := range props {
		if new, has := other[k]; has {
			if diff := p.Diff(new); diff != nil {
				if p.IsNull() {
					adds[k] = new
				} else if new.IsNull() {
					deletes[k] = p
				} else {
					updates[k] = *diff
				}
			} else {
				sames[k] = p
			}
		} else if !new.IsNull() {
			deletes[k] = p
		}
	}

	// Next find any additions not in the old map.
	for k, p := range other {
		if _, has := props[k]; !has && !p.IsNull() {
			adds[k] = p
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
func (v PropertyValue) Diff(other PropertyValue) *ValueDiff {
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
		if diff := old.Diff(new); diff != nil {
			return &ValueDiff{
				Old:    v,
				New:    other,
				Object: diff,
			}
		}
		return nil
	}

	// If we got here, either the values are primitives, or they weren't the same type; do a simple diff.
	if v.V == other.V {
		return nil
	}
	return &ValueDiff{Old: v, New: other}
}
