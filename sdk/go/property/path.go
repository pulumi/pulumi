// Copyright 2016, Pulumi Corporation.
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
	"encoding"
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Path provides access and alteration methods on [Value]s.
//
// Paths are composed of [PathSegment]s, which can be one of:
//
// - [KeySegment]: For indexing into [Map]s.
// - [IndexSegment]: For indexing into [Array]s.
type Path struct {
	pathRepr
	// ensure that there isn't a public cast from Path to [Glob].
	_isPath struct{}
}

func PathFromSegments(segments ...PathSegment) Path {
	return Path{pathReprFromSegments(segments), struct{}{}}
}

var (
	_ encoding.TextMarshaler   = Path{}
	_ encoding.TextUnmarshaler = &Path{}
)

func (g Path) MarshalText() (text []byte, err error) {
	return g.AsGlob().MarshalText()
}

func (p *Path) UnmarshalText(text []byte) error {
	var g Glob
	if err := g.UnmarshalText(text); err != nil {
		return err
	}
	*p = Path{}
	for v := range g.segments {
		if _, ok := v.(splat); ok {
			return errors.New("splat not allowed in non-glob paths")
		}
	}
	p.pathRepr = g.pathRepr
	return nil
}

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
	for segment := range p.segments {
		var err PathApplyFailure
		v, err = segment.(PathSegment).apply(v)
		if err != nil {
			return Value{}, err
		}
	}
	return v, nil
}

// Set the value described by the path in src to newValue.
//
// Set does not mutate src, instead a copy of src with the change applied is returned. is
// returned that holds the change.
//
// Any returned error will implement [PathApplyFailure].
func (p Path) Set(src, newValue Value) (Value, error) {
	if p.len() == 0 {
		return newValue, nil
	}

	butLast, last := p.last()
	v, err := Path{butLast, struct{}{}}.Get(src)
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
		if i.Index() >= len(slice) {
			return Value{}, pathApplyIndexOutOfBoundsError{found: v.AsArray(), idx: i.Index()}
		}
		slice[i.Index()] = newValue
		return Path{butLast, struct{}{}}.Set(src, New(slice))
	case v.IsMap():
		k, ok := last.(KeySegment)
		if !ok {
			return Value{}, pathErrorf(v, "expected a KeySegment, found %T", last)
		}

		return Path{butLast, struct{}{}}.Set(src, New(v.AsMap().Set(k.string, newValue)))
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
	GlobSegment
	apply(Value) (Value, PathApplyFailure)
}

// NewSegment creates a new [PathSegment] suitable for use in [Path].
func NewSegment[T interface{ string | int }](v T) PathSegment {
	switch v := any(v).(type) {
	case string:
		return KeySegment{v}
	case int:
		contract.Assertf(v >= 0, "index must be non-negative, got %d", v)
		return IndexSegment{uint64(v)} //nolint:gosec // checked above
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

func (k KeySegment) Key() string { return k.string }

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

// IndexSegment represents an index into an [Array].
//
// To create an IndexSegment, use [NewSegment].
type IndexSegment struct {
	// i must be constrained into a uint63, since it must be cleanly castable to an [int] ([int64]).
	i uint64
}

func (k IndexSegment) Index() int { return int(k.i) } //nolint:gosec // will always fit

func (k IndexSegment) apply(v Value) (Value, PathApplyFailure) {
	if v.IsArray() {
		i := int(k.i) //nolint:gosec // will always fit
		a := v.AsArray()
		if i >= a.Len() {
			return Value{}, pathApplyIndexOutOfBoundsError{found: a, idx: i}
		}
		return a.Get(i), nil
	}
	return Value{}, pathApplyIndexExpectedArrayError{found: v}
}

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

func (g Path) Segments(yield func(PathSegment) bool) {
	for v := range g.segments {
		if !yield(v.(PathSegment)) {
			return
		}
	}
}
