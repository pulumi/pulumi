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

// An immutable Map of [Value]s.
type Map struct{ m map[string]Value }

func (m Map) AsMap() map[string]Value { return copyMap(m.m) }

func (m Map) All(yield func(string, Value) bool) {
	for k, v := range m.m {
		if !yield(k, v) {
			continue
		}
	}
}

func (m Map) Get(key string) Value {
	return m.m[key]
}

func (m Map) GetOk(key string) (Value, bool) {
	v, ok := m.m[key]
	return v, ok
}

func (m Map) Len() int {
	return len(m.m)
}

func (m Map) Set(key string, value Value) Map {
	cp := copyMap(m.m)
	cp[key] = value
	return Map{cp}
}

func NewMap(m map[string]Value) Map { return Map{copyMap(m)} }

func copyMap(m map[string]Value) map[string]Value {
	cp := make(map[string]Value, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
