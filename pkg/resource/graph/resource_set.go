// Copyright 2016-2018, Pulumi Corporation.
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

import "github.com/pulumi/pulumi/pkg/resource"

// ResourceSet represents a set of Resources.
type ResourceSet struct {
	m map[*resource.State]bool
}

// NewResourceSet creates a new empty set of Resources.
func NewResourceSet() ResourceSet {
	return ResourceSet{
		m: make(map[*resource.State]bool),
	}
}

// Empty returns whether or not this set is empty.
func (s ResourceSet) Empty() bool {
	return len(s.m) == 0
}

// Test returns whether or not the given resource is an element of this set.
func (s ResourceSet) Test(state *resource.State) bool {
	if state == nil {
		return false
	}

	return s.m[state]
}

// Add inserts a resource into this set.
func (s ResourceSet) Add(state *resource.State) {
	if state != nil {
		s.m[state] = true
	}
}

// Remove removes a resource from this set.
func (s ResourceSet) Remove(state *resource.State) {
	if state != nil {
		delete(s.m, state)
	}
}

// Elements returns an array containing all resources contained in this set.
func (s ResourceSet) Elements() []*resource.State {
	var keys []*resource.State
	for key := range s.m {
		keys = append(keys, key)
	}

	return keys
}

// Intersect returns a new set that is the intersection of the two given resource sets.
func (s ResourceSet) Intersect(other ResourceSet) ResourceSet {
	newSet := NewResourceSet()
	for _, elem := range s.Elements() {
		if other.Test(elem) {
			newSet.Add(elem)
		}
	}

	return newSet
}
