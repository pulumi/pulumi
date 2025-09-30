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
	"iter"
	"slices"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// An immutable path into a [PropertyValue] that supports [Glob]s.
type GlobPath struct {
	// GlobPath is implemented as a singly linked list to allow O(1) appends without
	// mutation.
	tail *pathSegment[GlobPathSegment]
}

func (g GlobPath) AppendSegments(segments ...GlobPathSegment) GlobPath {
	for _, v := range segments {
		g.tail = &pathSegment[GlobPathSegment]{
			value:    v,
			previous: g.tail,
		}
	}
	return g
}

// Create a new empty [GlobPath].
func NewGlobPath(segments ...GlobPathSegment) GlobPath {
	return GlobPath{}.AppendSegments(segments...)
}

func (g GlobPath) Get(v Value) ([]Value, error) { return getForPath(v, g.asPath()) }

type globKey struct{}

// Glob is the special marker value for a * in a [GlobPath].
//
// Glob will match any [PathSegment].
var Glob globKey

type GlobPathSegment interface {
	applyGlob(Value) ([]Value, PathApplyFailure)
}

func (g GlobPath) asPath() path {
	var p []GlobPathSegment
	for g.tail != nil {
		p = append(p, g.tail.value)
		g.tail = g.tail.previous
	}
	slices.Reverse(p)
	return slices.Values(p)
}

// Expand a glob path onto a value.
//
// A glob can only be applied to an [Array] or a [Map]. Other values will expand to the
// empty set.
//
// For example, given a path p := "*" applied to v := ["a", "b", "c"], p.Expand(v) would
// yield paths 0, 1 and 2. Given a path p := "array[*]" and value { array: ["a", "b", "c"]
// }, p.Expand(v) would yield paths "array[0]", "array[1]" and "array[2]".
func (g GlobPath) Expand(v Value) []Path {
	paths := []Path{{} /* Start with a single path pointing at v */}

	dropPath := func(i int) {
		if i == len(paths)-1 {
			paths = paths[:i]
		} else {
			// Swap in the last path, then trim it.
			paths[i] = paths[len(paths)-1]
			paths = paths[:len(paths)-1]
		}
	}

	for segment := range g.asPath() {
		switch segment := segment.(type) {
		case globKey:
			// For each value in paths, expand out the globs
			for jdx := len(paths) - 1; jdx >= 0; jdx-- {
				atPath, err := paths[jdx].Get(v) // Get the current value the path is on, without segment applied
				// A non-nil error means that the path failed to
				// apply at the glob, so we need to remove
				// paths[jdx] from path.
				if err != nil {
					dropPath(jdx)
					continue
				}

				root := paths[jdx]
				// We are expanding paths[jdx], so we won't have that path
				// in paths anymore, only it's children
				dropPath(jdx)

				// The path has applied, so expand the glob
				switch {
				case atPath.IsArray():
					for i := range atPath.AsArray().Len() {
						next := make(Path, len(root)+1)
						copy(next, root)
						next[len(next)-1] = NewSegment(i)
						paths = append(paths, next)
					}
				case atPath.IsMap():
					for k := range atPath.AsMap().AllStable {
						next := make(Path, len(root)+1)
						copy(next, root)
						next[len(next)-1] = NewSegment(k)
						paths = append(paths, next)
					}
				default: // We can't apply a glob here
				}
			}
		case IndexSegment:
			for i, p := range paths {
				paths[i] = append(p, segment)
			}
		case KeySegment:
			for i, p := range paths {
				paths[i] = append(p, segment)
			}
		default:
			contract.Failf("Unknown path element %T", segment)
		}
	}
	if len(paths) == 0 {
		return nil
	}
	return paths
}

// A mutable inner representation of a [GlobPath].
type path = iter.Seq[GlobPathSegment]

func getForPath(v Value, p path) ([]Value, error) {
	queue := []Value{v}
	for segment := range p { // For each segment in the path
		for idx := len(queue) - 1; idx >= 0; idx-- {

			// Values to the right of idx are processed by this segment, while
			// values to the left are unprocessed by segment.

			values, err := segment.applyGlob(queue[idx])
			if err != nil {
				return nil, err
			}
			if len(values) == 0 {
				// The last item in the queue just converted into no
				// items, so the queue is empty.
				if len(queue) == 1 {
					return nil, err
				}
				// If queue[idx] is the rightmost value in the queue and
				// it expanded to no items, we can just shorten the queue.
				if idx == len(queue)-1 {
					queue = queue[:idx]
				}

				// Otherwise there is a right-most value in the queue, so
				// we can swap that in and then shorten the queue.
				queue[idx] = queue[len(queue)-1]
				queue = queue[:len(queue)-1]
			} else {
				// queue[idx] has produced at least one value, so swap the
				// first value in at queue[idx] and append the rest to the
				// queue.
				queue[idx] = values[0]
				queue = append(queue, values[1:]...)
			}
		}
	}
	return queue, nil
}

func (globKey) applyGlob(v Value) ([]Value, PathApplyFailure) {
	switch {
	case v.IsArray():
		return v.AsArray().AsSlice(), nil
	case v.IsMap():
		m := v.AsMap()
		values := make([]Value, 0, m.Len())
		for _, v := range m.AllStable {
			values = append(values, v)
		}
		return values, nil
	default:
		return nil, pathErrorf(v, "glob expected map or array, found %T", typeString(v))
	}
}

type pathSegment[P any] struct {
	value    P
	previous *pathSegment[P]
}

// Path provides access and alteration methods on [Value]s.
//
// Paths are composed of [PathSegment]s, which can be one of:
//
// - [KeySegment]: For indexing into [Map]s.
// - [IndexSegment]: For indexing into [Array]s.
type Path []PathSegment

// Get the [Value] from v by applying the [Path].
//
//	value := property.New(map[string]property.Value{
//		"cities": property.New([]property.Value{
//			property.New("Seattle"),
//			property.New("London"),
//		}),
//	})
//
//	firstCity := property.Path{
//		property.NewSegment("cities"),
//		property.NewSegment(0),
//	}
//
//	city, _ := firstCity.Get(value) // Seattle
//
// If the [Path] does not describe a value in v, then an error will be returned. The
// returned error can be safely cast to [PathApplyFailure].
func (p Path) Get(v Value) (Value, error) {
	values, err := getForPath(v, func(yield func(GlobPathSegment) bool) {
		for _, v := range p {
			if !yield(v) {
				break
			}
		}
	})
	if err != nil {
		return Value{}, err
	}
	contract.Assertf(len(values) == 1, "If p.Get(v) applied cleanly, it should return exactly 1 value")
	return values[0], nil
}

// Set the value described by the path in src to newValue.
//
// Set does not mutate src, instead a copy of src with the change applied is returned. is
// returned that holds the change.
//
// Any returned error will implement [PathApplyFailure].
func (p Path) Set(src, newValue Value) (Value, error) {
	if len(p) == 0 {
		return newValue, nil
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
			return Value{}, pathErrorf(v, "expected an IndexSegment, found %T", last)
		}
		slice := v.AsArray().AsSlice()
		if i.int < 0 || i.int >= len(slice) {
			return Value{}, pathApplyIndexOutOfBoundsError{found: v.AsArray(), idx: i.int}
		}
		slice[i.int] = newValue
		return butLast.Set(src, New(slice))
	case v.IsMap():
		k, ok := last.(KeySegment)
		if !ok {
			return Value{}, pathErrorf(v, "expected a KeySegment, found %T", last)
		}

		return butLast.Set(src, New(v.AsMap().Set(k.string, newValue)))
	default:
		return Value{}, pathApplyKeyExpectedMapError{found: v}
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
//
// Any returned error will implement [PathApplyFailure].
func (p Path) Alter(v Value, f func(v Value) Value) (Value, error) {
	oldValue, err := p.Get(v)
	if err != nil {
		return Value{}, err
	}
	return p.Set(v, f(oldValue))
}

type PathSegment interface {
	GlobPathSegment
	isPathSegment()
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

	// The last value in a path traversal successfully reached.
	Found() Value
}

// KeySegment represents a traversal into a [Map] by a key.
//
// KeySegment does not support glob ("*") expansion. Values are treated as is.
//
// To create an KeySegment, use [NewSegment].
type KeySegment struct{ string }

func (k KeySegment) apply(v Value) (Value, PathApplyFailure) {
	if v.IsMap() {
		m := v.AsMap()
		r, ok := m.GetOk(k.string)
		if ok {
			return r, nil
		}
		return Value{}, pathApplyKeyMissingError{found: m, needle: k.string}
	}
	return Value{}, pathApplyKeyExpectedMapError{found: v}
}

func (k KeySegment) applyGlob(v Value) ([]Value, PathApplyFailure) {
	result, err := k.apply(v)
	if err != nil {
		return nil, err
	}
	return []Value{result}, nil
}

func (KeySegment) isPathSegment() {}

// IndexSegment represents an index into an [Array].
//
// To create an IndexSegment, use [NewSegment].
type IndexSegment struct{ int }

func (k IndexSegment) apply(v Value) (Value, PathApplyFailure) {
	if v.IsArray() {
		a := v.AsArray()
		if k.int < 0 || k.int >= a.Len() {
			return Value{}, pathApplyIndexOutOfBoundsError{found: a, idx: k.int}
		}
		return a.Get(k.int), nil
	}
	return Value{}, pathApplyIndexExpectedArrayError{found: v}
}

func (k IndexSegment) applyGlob(v Value) ([]Value, PathApplyFailure) {
	result, err := k.apply(v)
	if err != nil {
		return nil, err
	}
	return []Value{result}, nil
}

func (IndexSegment) isPathSegment() {}

type pathApplyKeyExpectedMapError struct {
	found Value
}

func (err pathApplyKeyExpectedMapError) Error() string {
	return "expected a map, found a " + typeString(err.found)
}

func (err pathApplyKeyExpectedMapError) Found() Value {
	return err.found
}

type pathApplyKeyMissingError struct {
	found  Map
	needle string
}

func (err pathApplyKeyMissingError) Error() string {
	return fmt.Sprintf("missing key %q in map", err.needle)
}

func (err pathApplyKeyMissingError) Found() Value {
	return New(err.found)
}

type pathApplyIndexExpectedArrayError struct {
	found Value
}

func (err pathApplyIndexExpectedArrayError) Error() string {
	return "expected an array, found a " + typeString(err.found)
}

func (err pathApplyIndexExpectedArrayError) Found() Value {
	return err.found
}

type pathApplyIndexOutOfBoundsError struct {
	found Array
	idx   int
}

func (err pathApplyIndexOutOfBoundsError) Found() Value {
	return New(err.found)
}

func (err pathApplyIndexOutOfBoundsError) Error() string {
	return fmt.Sprintf("index %d out of bounds of an array of length %d",
		err.idx, err.found.Len())
}

func pathErrorf(v Value, msg string, a ...any) PathApplyFailure {
	return pathApplyError{found: v, msg: fmt.Sprintf(msg, a...)}
}

type pathApplyError struct {
	found Value
	msg   string
}

func (err pathApplyError) Error() string { return err.msg }
func (err pathApplyError) Found() Value  { return err.found }
