// Copyright 2024, Pulumi Corporation.
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

	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
)

type depSet map[URN]Resource

func (s depSet) add(v URN, r Resource) {
	s[v] = r
}

func (s depSet) has(v URN) bool {
	_, ok := s[v]
	return ok
}

func (s depSet) contains(other urnSet) bool {
	for v := range other {
		if !s.has(v) {
			return false
		}
	}
	return true
}

func (s depSet) union(other depSet) {
	for v, r := range other {
		s.add(v, r)
	}
}

func (s depSet) urns() []URN {
	values := slice.Prealloc[URN](len(s))
	for v := range s {
		values = append(values, v)
	}
	return values
}

func (s depSet) sortedURNs() []URN {
	v := s.urns()
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
	return v
}

func (d depSet) toURNSet() urnSet {
	s := make(urnSet)
	for v := range d {
		s.add(v)
	}
	return s
}
