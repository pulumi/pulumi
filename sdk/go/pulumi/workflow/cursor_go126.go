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

//go:build !go1.27

package workflow

// Get returns the value at key. On Go 1.27 and later this method is generic; see [Get].
func (c *Cursor) Get(key string) (any, bool) { return Get[any](c, key) }

// Require returns the value at key, panicking if it is missing. On Go 1.27 and later this method is
// generic; see [Require].
func (c *Cursor) Require(key string) any { return Require[any](c, key) }
