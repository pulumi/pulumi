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

// Map returns a new slice containing the results of applying the given function to each element of the given slice.
func Map[T, U any](s []T, f func(T) U) []U {
	result := make([]U, len(s))
	for i, v := range s {
		result[i] = f(v)
	}
	return result
}

// MapError returns a new slice containing the results of applying the given function to each element of the given slice.
func MapError[T, U any](s []T, f func(T) (U, error)) ([]U, error) {
	result := make([]U, len(s))
	for i, v := range s {
		var err error
		result[i], err = f(v)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
