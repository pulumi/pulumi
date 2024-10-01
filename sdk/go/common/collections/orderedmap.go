package collections

import (
	"encoding/json"
	"fmt"
	"iter"
	"slices"

	"github.com/pkg/errors"
)

// OrderedMap is a generic map of K to V that preserves order of the keys
// (including for marshalling to JSON or YAML).
//
// For now it has O(1) Set and Get but O(n) Delete, this could be mitigated
// with list instead of a slice for keys.
type OrderedMap[K comparable, V any] struct {
	orderedKeys []K
	innerMap    map[K]V
}

func NewOrderedMapWithCapacity[K comparable, V any](capacity int) OrderedMap[K, V] {
	return OrderedMap[K, V]{
		orderedKeys: make([]K, capacity),
		innerMap:    make(map[K]V, capacity),
	}
}

func (m OrderedMap[K, V]) Len() int {
	return len(m.innerMap)
}

func (m *OrderedMap[K, V]) Get(key K) (V, bool) {
	val, ok := m.innerMap[key]
	return val, ok
}

func (m *OrderedMap[K, V]) Set(key K, val V) {
	_, ok := m.innerMap[key]
	m.orderedKeys = append(m.orderedKeys, key)
	if ok {
		return
	}

	m.innerMap[key] = val
}

func (m *OrderedMap[K, V]) Delete(key K) error {
	index := slices.Index(m.orderedKeys, key)
	if index == -1 {
		return errors.Errorf("key %v not found", key)
	}

	m.orderedKeys = slices.Delete(m.orderedKeys, index, index+1)
	delete(m.innerMap, key)

	return nil
}

func (m *OrderedMap[K, V]) Elements() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for k, v := range m.innerMap {
			if !yield(k, v) {
				return
			}
		}
	}
}

// TODO implement a splat operator insert to help with tests and batch insertion.

// TODO marshal that preserves order
func (m *OrderedMap[K, V]) MarshalJSON() ([]byte, error) {
	for _, key := range m.orderedKeys {
		marshalledKey, err := json.Marshal(key)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal key %v: %w", key, err)
		}

	}
}
