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

import "reflect"

// Transform applies f to each value in v, children before parents, and returns the
// transformed value. Parents receive their transformed children.
//
// Only arrays and maps are traversed; all other values, including resource references,
// are leaves. Array elements are visited in index order; map order is unspecified.
// Values returned by f are not traversed again.
func (v Value) Transform(f func(Value) Value) Value {
	switch inner := v.v.(type) {
	case Array:
		var cp []Value
		for i, child := range inner.arr {
			transformed := child.Transform(f)
			if !child.transformIdentical(transformed) {
				if cp == nil {
					cp = copyArray(inner.arr)
				}
				cp[i] = transformed
			}
		}
		if cp != nil {
			v.v = Array{cp}
		}
	case Map:
		var cp map[string]Value
		for k, child := range inner.m {
			transformed := child.Transform(f)
			if !child.transformIdentical(transformed) {
				if cp == nil {
					cp = copyMapNonNil(inner.m)
				}
				cp[k] = transformed
			}
		}
		if cp != nil {
			v.v = Map{cp}
		}
	}
	return f(v)
}

// transformIdentical conservatively checks identity, not semantic equality. Comparing
// reflect.Values compares the underlying representation without recursively comparing
// containers or panicking on uncomparable payloads. Equal but separately boxed payloads
// may compare different, which is safe: it only causes an extra container copy.
func (v Value) transformIdentical(other Value) bool {
	return v.secret == other.secret &&
		// Values copy dependencies when they accept them, so the same location implies the same slice
		len(v.dependencies) == len(other.dependencies) &&
		(len(v.dependencies) == 0 || &v.dependencies[0] == &other.dependencies[0]) &&
		reflect.ValueOf(v.v) == reflect.ValueOf(other.v) //nolint:govet // Intentionally compare representation, not contents.
}
