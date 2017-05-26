// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsctx

type Hash string

type Hashable interface {
	HashKey() Hash
	HashValue() Hash
}

type HashSet struct {
	items map[Hash]Hashable
}

func (set *HashSet) Add(item Hashable) *Hash {
	key := item.HashKey()
	_, existed := set.items[key]
	set.items[key] = item
	if existed {
		return &key
	}
	return nil
}
func (set *HashSet) Length() int                       { return len(set.items) }
func (old *HashSet) Changes(new *HashSet) *HashSetDiff { return newHashSetDiff(old, new) }

func NewHashSet() *HashSet { return &HashSet{map[Hash]Hashable{}} }

type HashSetDiff struct {
	adds    []Hashable
	updates []Hashable
	deletes []Hashable
}

// Adds returns the items added b
func (diff *HashSetDiff) Adds() []Hashable    { return diff.adds }
func (diff *HashSetDiff) Updates() []Hashable { return diff.updates }
func (diff *HashSetDiff) Deletes() []Hashable { return diff.deletes }
func (diff *HashSetDiff) AddOrUpdates() []Hashable {
	newArr := []Hashable{}
	for _, update := range diff.updates {
		newArr = append(newArr, update)
	}
	for _, add := range diff.adds {
		newArr = append(newArr, add)
	}
	return newArr
}

func newHashSetDiff(old *HashSet, new *HashSet) *HashSetDiff {
	hashSetDiff := HashSetDiff{
		adds:    []Hashable{},
		updates: []Hashable{},
		deletes: []Hashable{},
	}
	for key, val := range new.items {
		oldVal, exists := old.items[key]
		if exists {
			if oldVal.HashValue() != val.HashValue() {
				// If it exists in both, but with different values, it's an update
				hashSetDiff.updates = append(hashSetDiff.updates, val)
			}
		} else {
			// If it exists in new but not in old, its an add
			hashSetDiff.adds = append(hashSetDiff.adds, val)
		}
	}
	for key, val := range old.items {
		_, exists := new.items[key]
		if !exists {
			// If it exists in old but not in new, its a delete
			hashSetDiff.deletes = append(hashSetDiff.deletes, val)
		}
	}
	return &hashSetDiff
}
