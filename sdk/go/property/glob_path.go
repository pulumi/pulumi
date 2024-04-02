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
	"errors"
)

type GlobPath []any

func NewGlobPath() Path { return Path{} }

func ParseGlobPath(path string) (GlobPath, error) {
	el, _, err := pathParse([]rune(path))
	return GlobPath(el), err
}

func (p GlobPath) Index(i int) GlobPath    { return append(p, i) }
func (p GlobPath) Field(s string) GlobPath { return append(p, s) }
func (p GlobPath) Glob() GlobPath          { return append(p, glob) }

type Glob string

const glob Glob = "*"

// Expands any globs "*" in path according to value.
//
// If a glob doesn't match any value, the empty set is returned. Non-glob path elements do
// not need to match values to be returned. As an example, consider this PropertyPath:
//
//	foo["*"].bar
//
// Given the value `{ "foo": [ {}, { "v1": 0 } ] }`, `expandGlob(path, value)` would return
// `[foo[0].bar, foo[1].bar]`.
//
// Given the value `{ "missing": true }`, `expandGlob(path, value)` would return the empty
// list: `[]`.
//
// The order or returned paths is non-deterministic due to dictionary iteration.
func (p GlobPath) Expand(v Value) []Path {
	for i, seg := range p {
		// If seg isn't a glob, then we don't need to expand here.
		_, ok := seg.(Glob)
		if !ok {
			continue
		}
		// Get the item that encloses seg.
		//
		// The cast Path(..) cast is safe because we have verified that all
		// elements before i are not globs.
		v, err := Path(p[:i]).Get(v)
		if err != nil {
			// We have failed to expand, which means that the glob
			// returns empty.
			return nil
		}

		switch {
		case v.IsArray():
			results := make([]Path, 0, len(v.AsArray()))
			for el, val := range v.AsArray() {
				results = append(results,
					addPrefix(append(Path(p[:i]), el),
						p[i+1:].Expand(val))...)
			}
			return results
		case v.IsMap():
			results := make([]Path, 0, len(v.AsMap()))
			for el, val := range v.AsMap() {
				results = append(results,
					addPrefix(append(Path(p[:i]), string(el)),
						p[i+1:].Expand(val))...)
			}
			return results
		// v is not glob-able, so we expand to nothing.
		default:
			return nil
		}
	}
	return []Path{Path(p)}
}

func addPrefix(prefix Path, ends []Path) []Path {
	results := make([]Path, 0, len(ends))
	for _, end := range ends {
		cp := make(Path, len(prefix)+len(end))
		copy(cp, prefix)
		copy(cp[len(prefix):], end)
		results = append(results, cp)
	}
	return results
}

func (p GlobPath) Get(v Value) ([]Value, error) {
	var result []Value
	var errs []error
	for _, p := range p.Expand(v) {
		r, err := p.Get(v)
		if err != nil {
			errs = append(errs, err)
		} else {
			result = append(result, r)
		}
	}
	return result, errors.Join(errs...)
}

func (p GlobPath) Set(dst, v Value) error {
	var errs []error
	for _, p := range p.Expand(dst) {
		if err := p.Set(dst, v); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (p GlobPath) Delete(v Value) error {
	if len(p) == 0 {
		return ZeroLengthPathError{"Delete"}
	}

	var errs []error
	for _, p := range p.Expand(v) {
		if err := p.Delete(v); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (p GlobPath) Map(v Value, f func(Value) Value) error {
	var errs []error
	for _, p := range p.Expand(v) {
		e, err := p.Get(v)
		if err != nil {
			return err
		}
		return p.Set(v, f(e))
	}
	return errors.Join(errs...)

}

func (p GlobPath) String() string   { return pathString(p) }
func (p GlobPath) GoString() string { return pathGoString("property.NewGlobPath", p) }
