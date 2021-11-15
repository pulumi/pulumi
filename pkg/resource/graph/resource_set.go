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

package graph

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"sort"
)

// ResourceSet represents a set of Resources.
type ResourceSet map[*resource.State]bool

// Intersect returns a new set that is the intersection of the two given resource sets.
func (s ResourceSet) Intersect(other ResourceSet) ResourceSet {
	newSet := make(ResourceSet)
	for key := range s {
		if other[key] {
			newSet[key] = true
		}
	}

	return newSet
}

// Returns the contents of the set as an array of resources. To ensure
// determinism, they are sorted by urn.
func (s ResourceSet) ToArray() []*resource.State {
	arr := make([]*resource.State, len(s))
	i := 0
	for r := range s {
		arr[i] = r
		i++
	}
	sort.Slice(arr, func(i, j int) bool {
		return arr[i].URN < arr[j].URN
	})
	return arr
}

// Produces a set from an array.
func NewResourceSetFromArray(arr []*resource.State) ResourceSet {
	s := ResourceSet{}
	for _, r := range arr {
		s[r] = true
	}
	return s
}

// Produces a shallow copy of `s`.
func CopyResourceSet(s ResourceSet) ResourceSet {
	result := ResourceSet{}
	for k, v := range s {
		result[k] = v
	}
	return result
}

// Computes s - other. Input sets are unchanged. If `other[k] = false`, then `k`
// will not be removed from `s`.
func (s ResourceSet) SetMinus(other ResourceSet) ResourceSet {
	result := CopyResourceSet(s)
	for k, v := range other {
		if v {
			delete(result, k)
		}
	}
	return result
}

// Produces a new set with elements from both sets. The original sets are unchanged.
func (s ResourceSet) Union(other ResourceSet) ResourceSet {
	result := CopyResourceSet(s)
	return result.UnionWith(other)
}

// Alters `s` to include elements of `other`.
func (s ResourceSet) UnionWith(other ResourceSet) ResourceSet {
	for k, v := range other {
		s[k] = v || s[k]
	}
	return s
}
