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
	"slices"

	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
)

// ObjectDiff holds the results of diffing two object property maps.
type ObjectDiff struct {
	Adds    map[string]Value     // properties in this map are created in the new.
	Deletes map[string]Value     // properties in this map are deleted from the new.
	Sames   map[string]Value     // properties in this map are the same.
	Updates map[string]ValueDiff // properties in this map are changed in the new.
}

// Added returns true if the property 'k' has been added in the new property set.
func (diff *ObjectDiff) Added(k string) bool {
	_, has := diff.Adds[k]
	return has
}

// Deleted returns true if the property 'k' has been deleted from the new property set.
func (diff *ObjectDiff) Deleted(k string) bool {
	_, has := diff.Deletes[k]
	return has
}

// Updated returns true if the property 'k' has been changed between new and old property sets.
func (diff *ObjectDiff) Updated(k string) bool {
	_, has := diff.Updates[k]
	return has
}

// Changed returns true if the property 'k' is known to be different between old and new.
func (diff *ObjectDiff) Changed(k string) bool {
	return diff.Added(k) || diff.Deleted(k) || diff.Updated(k)
}

// Same returns true if the property 'k' is *not* known to be different; note that this isn't the same as looking up in
// the Sames map, because it is possible the key is simply missing altogether (as is the case for nulls).
func (diff *ObjectDiff) Same(k string) bool {
	return !diff.Changed(k)
}

// AnyChanges returns true if there are any changes (adds, deletes, updates) in the diff. Otherwise returns false.
func (diff *ObjectDiff) AnyChanges() bool {
	return len(diff.Adds)+len(diff.Deletes)+len(diff.Updates) > 0
}

// Keys returns a stable snapshot of all keys known to this object, across adds, deletes, sames, and updates.
func (diff *ObjectDiff) Keys() []string {
	bufferSize := len(diff.Adds) + len(diff.Deletes) + len(diff.Sames) + len(diff.Updates)
	ks := slice.Prealloc[string](bufferSize)
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
	slices.Sort(ks)
	return ks
}

// All keys where Changed(k) = true.
func (diff *ObjectDiff) ChangedKeys() []string {
	var ks []string
	if diff != nil {
		for _, k := range diff.Keys() {
			if diff.Changed(k) {
				ks = append(ks, k)
			}
		}
	}
	return ks
}

// ValueDiff holds the results of diffing two property values.
type ValueDiff struct {
	Old    Value       // the old value.
	New    Value       // the new value.
	Array  *ArrayDiff  // the array's detailed diffs (only for arrays).
	Object *ObjectDiff // the object's detailed diffs (only for objects).
}

// ArrayDiff holds the results of diffing two arrays of property values.
type ArrayDiff struct {
	Adds    map[int]Value     // elements added in the new.
	Deletes map[int]Value     // elements deleted in the new.
	Sames   map[int]Value     // elements the same in both.
	Updates map[int]ValueDiff // elements that have changed in the new.
}

// Len computes the length of this array, taking into account adds, deletes, sames, and updates.
func (diff *ArrayDiff) Len() int {
	length := 0
	for i := range diff.Adds {
		if i+1 > length {
			length = i + 1
		}
	}
	for i := range diff.Deletes {
		if i+1 > length {
			length = i + 1
		}
	}
	for i := range diff.Sames {
		if i+1 > length {
			length = i + 1
		}
	}
	for i := range diff.Updates {
		if i+1 > length {
			length = i + 1
		}
	}
	return length
}

type DiffOption interface {
	apply(*diffOptions)
}

type diffOptions struct {
	ignoreKeyFuncs []func(key string) bool
	ignorePathFunc []func(key Path) bool
	initialPath    Path
}

// IgnoreKeyFunc is the callback type for Diff's ignore option.
type IgnoreKeyFunc func(key string) bool

// IgnorePathFunc is the callback type for Diff's ignore path option.
//
// If provided functions want path to outlive the callback, they should make their own
// copy.
type IgnorePathFunc func(path Path) bool

// Set the initial property path for DiffWithOptions.
//
// The passed in property path will be mutated via append.
type InitialPropertyPath Path

func (opt IgnoreKeyFunc) apply(o *diffOptions)       { o.ignoreKeyFuncs = append(o.ignoreKeyFuncs, opt) }
func (opt IgnorePathFunc) apply(o *diffOptions)      { o.ignorePathFunc = append(o.ignorePathFunc, opt) }
func (opt InitialPropertyPath) apply(o *diffOptions) { o.initialPath = Path(opt) }

// Diff returns a diffset by comparing the property map to another; it returns nil if there are no diffs.
func (props Map) Diff(other Map, options ...DiffOption) *ObjectDiff {
	var opts diffOptions
	for _, v := range options {
		v.apply(&opts)
	}
	return props.diff(other, opts, opts.initialPath)
}

// Diff returns a diffset by comparing the property map to another; it returns nil if there are no diffs.
func (props Map) diff(other Map, opts diffOptions, path Path) *ObjectDiff {
	adds := make(map[string]Value)
	deletes := make(map[string]Value)
	sames := make(map[string]Value)
	updates := make(map[string]ValueDiff)

	ignore := func(key string) bool {
		for _, ikf := range opts.ignoreKeyFuncs {
			if ikf(key) {
				return true
			}
		}
		for _, ikp := range opts.ignorePathFunc {
			newpath := path.appendKey(key)
			if ikp(Path{newpath, struct{}{}}) {
				return true
			}
		}
		return false
	}

	// First find any updates or deletes.
	for k, old := range props.All {
		if ignore(k) {
			continue
		}

		if new, has := other.GetOk(k); has {
			// If a new exists, use it; for output properties, however, ignore differences.
			newpath := path.appendKey(k)
			if diff := old.diff(new, opts, Path{newpath, struct{}{}}); diff != nil {
				if !old.hasValue() {
					adds[k] = new
				} else if !new.hasValue() {
					deletes[k] = old
				} else {
					updates[k] = *diff
				}
			} else {
				sames[k] = old
			}
		} else if old.hasValue() {
			// If there was no new property, it has been deleted.
			deletes[k] = old
		}
	}

	// Next find any additions not in the old map.
	for k, new := range other.All {
		if ignore(k) {
			continue
		}

		if _, has := props.GetOk(k); !has && new.hasValue() {
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

func (props Value) Diff(other Value, options ...DiffOption) *ValueDiff {
	var opts diffOptions
	for _, v := range options {
		v.apply(&opts)
	}
	return props.diff(other, opts, opts.initialPath)
}

// Diff returns a diff by comparing a single property value to another; it returns nil if there are no diffs.
func (v Value) diff(other Value, opts diffOptions, path Path) *ValueDiff {
	// If secretness differs, then the values are different.
	if v.Secret() != other.Secret() {
		return &ValueDiff{Old: v, New: other}
	}
	// If the dependencies differ, then the values are different.
	if !slices.Equal(v.Dependencies(), other.Dependencies()) {
		return &ValueDiff{Old: v, New: other}
	}

	opaque := v.Secret() || len(v.Dependencies()) > 0

	if v.IsArray() && other.IsArray() {
		old := v.AsArray()
		new := other.AsArray()
		// If any elements exist in the new array but not the old, track them as adds.
		adds := make(map[int]Value)
		for i := old.Len(); i < new.Len(); i++ {
			adds[i] = new.Get(i)
		}
		// If any elements exist in the old array but not the new, track them as adds.
		deletes := make(map[int]Value)
		for i := new.Len(); i < old.Len(); i++ {
			deletes[i] = old.Get(i)
		}
		// Now if elements exist in both, track them as sames or updates.
		sames := make(map[int]Value)
		updates := make(map[int]ValueDiff)
		for i := 0; i < old.Len() && i < new.Len(); i++ {
			newpath := path.appendIndex(uint64(i))
			if diff := old.Get(i).diff(new.Get(i), opts, Path{newpath, struct{}{}}); diff != nil {
				updates[i] = *diff
			} else {
				sames[i] = old.Get(i)
			}
		}

		if len(adds) == 0 && len(deletes) == 0 && len(updates) == 0 {
			return nil
		}
		if opaque {
			return &ValueDiff{Old: v, New: other}
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
	if v.IsMap() && other.IsMap() {
		old := v.AsMap()
		new := other.AsMap()
		if diff := old.diff(new, opts, path); diff != nil {
			if opaque {
				return &ValueDiff{Old: v, New: other}
			}
			return &ValueDiff{
				Old:    v,
				New:    other,
				Object: diff,
			}
		}
		return nil
	}

	// If we got here, either the values are primitives, or they weren't the same type; do a simple diff.
	if v.Equals(other) {
		return nil
	}
	return &ValueDiff{Old: v, New: other}
}
