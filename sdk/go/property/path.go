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

import "fmt"

// Path provides access and alteration methods on [Value]s.
//
// Path does not support glob ("*") expansion. Values are treated as is.
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

func (p Path) Set(src, dst Value) PathApplyFailure {
	panic("Unimplemented")
}

func (p Path) Alter(v Value, f func(v Value) Value) {
	panic("Unimplemented")
}

type PathSegment interface {
	apply(Value) (Value, PathApplyFailure)
}

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
	return fmt.Sprintf("expected a map, found a %s", typeString(err.found))
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
	return fmt.Sprintf("expected an array, found a %s", typeString(err.found))
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
