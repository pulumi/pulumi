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

// HashSet is a set of items with set identity determined by a key hash, and with the ability to compute the
// set-based add/delete/update diff based on a value hash.
//
// For many AWS resources, array-valued properties are actually logically treated as sets key'd by some subset of the
// element object properties.  See TODO[pulumi/lumi#178] for additional work to support this pattern in Lumi.
type HashSet struct {
	items map[Hash]Hashable
}

// Hash represents a hash value for use in a HashSet
type Hash string

// Hashable is the element type for HashSets
type Hashable interface {
	// Compute a key hash used to track identity in a HashSet
	HashKey() Hash
	// Compute a value hash used to track updates of items with the same key hash across HashSets
	HashValue() Hash
}

// Add an item to a HashSet.  If an item with the same key hash was already present, it is overwritten and the
// conflicting Hash is returned.  If there was no item with the same key has already present, nil is returned.
func (set *HashSet) Add(item Hashable) *Hash {
	key := item.HashKey()
	_, existed := set.items[key]
	set.items[key] = item
	if existed {
		return &key
	}
	return nil
}

// Length returns the size of the set
func (set *HashSet) Length() int { return len(set.items) }

// Diff computes the add/update/delete changes from an old to a new HashSet
func (set *HashSet) Diff(new *HashSet) *HashSetDiff { return newHashSetDiff(set, new) }

// NewHashSet creates a new empty HashSet
func NewHashSet() *HashSet { return &HashSet{map[Hash]Hashable{}} }

// HashSetDiff represents the add/update/delete diff between two HashSets
type HashSetDiff struct {
	adds    []Hashable
	updates []Hashable
	deletes []Hashable
}

// Adds returns the items from the new HashSet whose key hash was not in the old HashSet
func (diff *HashSetDiff) Adds() []Hashable { return diff.adds }

// Updates returns the items with the same key hash, but different value hashs in the old and new HashSet
func (diff *HashSetDiff) Updates() []Hashable { return diff.updates }

// Deletes returns the items from the old HashSet whose key hash was not in the new HashSet
func (diff *HashSetDiff) Deletes() []Hashable { return diff.deletes }

// AddOrUpdates returns all items tht were added or updates in the new HashSet
func (diff *HashSetDiff) AddOrUpdates() []Hashable {
	result := make([]Hashable, 0, len(diff.updates)+len(diff.adds))
	result = append(result, diff.updates...)
	result = append(result, diff.adds...)
	return result
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
