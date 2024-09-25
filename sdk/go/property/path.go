// Copyright 2016-2024, Pulumi Corporation.
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
	"fmt"
)

// Path provides access and alteration methods on [Value]s.
//
// Paths are composed of [PathSegment]s, which can be one of:
//
// - [KeySegment]: For indexing into [Map]s.
// - [IndexSegment]: For indexing into [Array]s.
type Path []PathSegment

func (p Path) Get(v Value) (Value, PathApplyFailure) {
	for _, segment := range p {
		var err PathApplyFailure
		v, err = segment.apply(v)
		if err != nil {
			return Value{}, err
		}
	}
	return v, nil
}

// Set a value.
//
// Set does not src in place, rather producing a copy.
func (p Path) Set(src, to Value) (Value, PathApplyFailure) {
	if len(p) == 0 {
		return to, nil
	}
	butLast, last := p[:len(p)-1], p[len(p)-1]
	v, err := butLast.Get(src)
	if err != nil {
		return Value{}, err
	}
	switch {
	case v.IsArray():
		i, ok := last.(IndexSegment)
		if !ok {
			return Value{}, pathErrorf(v, "expected an index step, found %T", last)
		}
		if i.int < 0 || i.int > len(v.AsArray()) {
			return Value{}, PathApplyIndexOutOfBounds{found: v.AsArray(), idx: i.int}
		}
		// We now want to set set v.AsArray()[i]=to, but we don't want to mutate v. We make
		// a shallow copy of v and then alter that.
		cp := make(Array, len(v.AsArray()))
		copy(cp, v.AsArray())
		cp[i.int] = to
		return butLast.Set(src, New(cp)) // Finally, propagate the shallow change back up.
	case v.IsMap():
		k, ok := last.(KeySegment)
		if !ok {
			return Value{}, pathErrorf(v, "expected a key step, found %T", last)
		}
		// We now want to set set v.AsMap()[k]=to, but we don't want to mutate v. We make
		// a shallow copy of v and then alter that.
		cp := make(Map, len(v.AsMap()))
		for k, e := range v.AsMap() {
			cp[k] = e
		}
		cp[k.string] = to                // Assign to the new map
		return butLast.Set(src, New(cp)) // Finally, propagate the shallow change back up.
	default:
		return Value{}, pathErrorf(v, "expected a map or array, found %s", typeString(v))
	}
}

// Alter changes the value at p by applying f.
//
// To preserve metadata, use [WithGoValue] in conjunction with Alter:
//
//	p.Alter(v, func(v Value) Value) {
//		return property.WithGoValue(v, "new-value")
//	})
//
// This will preserve any secrets or dependencies encoded in `v`.
func (p Path) Alter(v Value, f func(v Value) Value) (Value, PathApplyFailure) {
	oldValue, err := p.Get(v)
	if err != nil {
		return Value{}, err
	}
	return p.Set(v, f(oldValue))
}

type PathSegment interface {
	apply(Value) (Value, PathApplyFailure)
}

// NewSegment creates a new [PathSegment] suitable for use in [Path].
func NewSegment[T interface{ string | int }](v T) PathSegment {
	switch v := any(v).(type) {
	case string:
		return KeySegment{v}
	case int:
		return IndexSegment{v}
	default:
		panic("impossible")
	}
}

type PathApplyFailure interface {
	error

	Found() Value
}

// KeySegment represents a traversal into a map by a key.
//
// KeySegment does not support glob ("*") expansion. Values are treated as is.
type KeySegment struct{ string }

func (k KeySegment) apply(v Value) (Value, PathApplyFailure) {
	if v.IsMap() {
		m := v.AsMap()
		r, ok := m[k.string]
		if ok {
			return r, nil
		}
		return Value{}, PathApplyKeyMissing{found: m, needle: k.string}
	}
	return Value{}, PathApplyKeyExpectedMap{found: v}
}

type IndexSegment struct{ int }

func (k IndexSegment) apply(v Value) (Value, PathApplyFailure) {
	if v.IsArray() {
		a := v.AsArray()
		if k.int < 0 || k.int >= len(a) {
			return Value{}, PathApplyIndexOutOfBounds{found: a, idx: k.int}
		}
		return a[k.int], nil
	}
	return Value{}, PathApplyIndexExpectedArray{found: v}
}

type PathApplyKeyExpectedMap struct {
	found Value
}

func (err PathApplyKeyExpectedMap) Error() string {
	return "expected a map, found a " + typeString(err.found)
}

func (err PathApplyKeyExpectedMap) Found() Value {
	return err.found
}

type PathApplyKeyMissing struct {
	found  Map
	needle string
}

func (err PathApplyKeyMissing) Error() string {
	return fmt.Sprintf("missing key %q in map", err.needle)
}

func (err PathApplyKeyMissing) Found() Value {
	return New(err.found)
}

type PathApplyIndexExpectedArray struct {
	found Value
}

func (err PathApplyIndexExpectedArray) Error() string {
	return "expected an array, found a " + typeString(err.found)
}

func (err PathApplyIndexExpectedArray) Found() Value {
	return err.found
}

type PathApplyIndexOutOfBounds struct {
	found Array
	idx   int
}

func (err PathApplyIndexOutOfBounds) Found() Value {
	return New(err.found)
}

func (err PathApplyIndexOutOfBounds) Error() string {
	return fmt.Sprintf("index %d out of bounds of an array of length %d",
		err.idx, len(err.found))
}

func pathErrorf(v Value, msg string, a ...any) PathApplyFailure {
	return pathApplyFailure{found: v, msg: fmt.Sprintf(msg, a...)}
}

type pathApplyFailure struct {
	found Value
	msg   string
}

func (err pathApplyFailure) Error() string { return err.msg }
func (err pathApplyFailure) Found() Value  { return err.found }
