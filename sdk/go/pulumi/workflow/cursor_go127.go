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

//go:build go1.27

package workflow

// Get returns the value at key as T. See [Get].
func (c *Cursor) Get[T any](key string) (T, bool) { return Get[T](c, key) }

// Require returns the value at key as T, panicking if it is missing or not a T. See [Require].
func (c *Cursor) Require[T any](key string) T { return Require[T](c, key) }
