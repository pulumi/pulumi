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

package lazy

import "sync"

// Lazy is a type that represents a value that is computed only once on first access
// and then cached for subsequent calls. This is thread-safe.
type Lazy[T any] interface {
	Value() T
}

type lazy[T any] struct {
	once   sync.Once
	result T
	f      func() T
}

// New creates a new Lazy[T] with the given function to compute the value.
func New[T any](f func() T) Lazy[T] {
	return &lazy[T]{
		f:    f,
		once: sync.Once{},
	}
}

// Value returns the computed value, computing it on the first access.
func (l *lazy[T]) Value() T {
	l.once.Do(func() {
		l.result = l.f()
	})
	return l.result
}

// Lazy2 is a type that represents two values that are computed only once on first access
// and then cached for subsequent calls. This is thread-safe.
type Lazy2[T any, U any] interface {
	Value() (T, U)
}

type lazy2[T any, U any] struct {
	once    sync.Once
	result1 T
	result2 U
	f       func() (T, U)
}

// New2 creates a new Lazy2[T, U] with the given function to compute the values.
func New2[T any, U any](f func() (T, U)) Lazy2[T, U] {
	return &lazy2[T, U]{
		f:    f,
		once: sync.Once{},
	}
}

// Value returns the computed values, computing them on the first access.
func (l *lazy2[T, U]) Value() (T, U) {
	l.once.Do(func() {
		l.result1, l.result2 = l.f()
	})
	return l.result1, l.result2
}
