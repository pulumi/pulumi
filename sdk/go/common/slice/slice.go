// Copyright 2016-2023, Pulumi Corporation.
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

package slice

// Preallocates a slice of type T of a given length. If the input length is 0 then the returned slice will be
// nil. We prefer this over `make([]T, 0, length)` because nil is often treated semantically different from an
// empty slice.
func Prealloc[T any](capacity int) []T {
	if capacity == 0 {
		return nil
	}
	return make([]T, 0, capacity)
}

// Map applies the given function to each element of the given slice and returns a new slice with the results.
func Map[T, U any](s []T, f func(T) U) []U {
	r := make([]U, len(s))
	for i, v := range s {
		r[i] = f(v)
	}
	return r
}

// MapError applies the given function to each element of the given slice and returns a new slice with the
// results. If any element returns an error that error is returned, as well as the slice of results so far.
func MapError[T, U any](s []T, f func(T) (U, error)) ([]U, error) {
	r := make([]U, 0, len(s))
	for _, v := range s {
		var err error
		u, err := f(v)
		if err != nil {
			return r, err
		}
		r = append(r, u)
	}
	return r, nil
}
