// Copyright 2016-2021, Pulumi Corporation.
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

package pulumi

import (
	"sort"
)

type urnSet map[URN]struct{}

func (s urnSet) add(v URN) {
	s[v] = struct{}{}
}

func (s urnSet) has(v URN) bool {
	_, ok := s[v]
	return ok
}

func (s urnSet) contains(other urnSet) bool {
	for v := range other {
		if !s.has(v) {
			return false
		}
	}
	return true
}

func (s urnSet) union(other urnSet) {
	for v := range other {
		s.add(v)
	}
}

func (s urnSet) values() []URN {
	values := make([]URN, 0, len(s))
	for v := range s {
		values = append(values, v)
	}
	return values
}

func (s urnSet) sortedValues() []URN {
	v := s.values()
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
	return v
}
