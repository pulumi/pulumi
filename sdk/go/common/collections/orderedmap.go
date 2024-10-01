package collections

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"iter"
	"slices"

	"gopkg.in/yaml.v3"
)

// TODO test yaml and json marshalling and unmarshalling

// OrderedMap is a generic map of K to V that preserves order of the keys
// (including for marshalling to JSON or YAML).
//
// For now it has O(1) Set and Get but O(n) Delete, this could be mitigated
// with list instead of a slice for keys.
type OrderedMap[K comparable, V any] struct {
	orderedKeys []K
	innerMap    map[K]V
}

func NewOrderedMap[K comparable, V any]() OrderedMap[K, V] {
	return OrderedMap[K, V]{
		orderedKeys: make([]K, 0),
		innerMap:    make(map[K]V),
	}
}

func NewOrderedMapWithCapacity[K comparable, V any](capacity int) OrderedMap[K, V] {
	return OrderedMap[K, V]{
		orderedKeys: make([]K, capacity),
		innerMap:    make(map[K]V, capacity),
	}
}

func NewOrderedMapWithValues[K comparable, V any](pairs ...Pair[K, V]) OrderedMap[K, V] {
	m := NewOrderedMapWithCapacity[K, V](len(pairs))
	m.InsertAll(pairs...)
	return m
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

type Pair[K comparable, V any] struct {
	Key   K
	Value V
}

// InsertAll uses variadic arguments to insert multiple key-value pairs into the map.
func (m *OrderedMap[K, V]) InsertAll(pairs ...Pair[K, V]) {
	for _, pair := range pairs {
		m.Set(pair.Key, pair.Value)
	}
}

func (m *OrderedMap[K, V]) Delete(key K) error {
	index := slices.Index(m.orderedKeys, key)
	if index == -1 {
		return fmt.Errorf("key %v not found", key)
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

func (m *OrderedMap[K, V]) MarshalJSON() ([]byte, error) {
	bytes := bytes.Buffer{}

	buf := bufio.NewWriterSize(&bytes, 512)
	_, err := buf.WriteRune('{')
	if err != nil {
		return nil, err
	}

	for i, key := range m.orderedKeys {
		marshalledKey, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}

		_, err = buf.Write(marshalledKey)
		if err != nil {
			return nil, err
		}

		_, err = buf.WriteRune(':')
		if err != nil {
			return nil, err
		}

		v, ok := m.innerMap[key]
		if !ok {
			return nil, err
		}
		marshalledValue, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}

		_, err = buf.Write(marshalledValue)
		if err != nil {
			return nil, err
		}

		if i < len(m.orderedKeys)-1 {
			_, err = buf.WriteRune(',')
			if err != nil {
				return nil, err
			}

		}
	}

	_, err = buf.WriteRune('}')
	if err != nil {
		return nil, err
	}

	return bytes.Bytes(), nil
}

func (m OrderedMap[K, V]) MarshalYAML() (interface{}, error) {
	node := &yaml.Node{
		Kind: yaml.MappingNode,
	}

	for _, key := range m.orderedKeys {
		keyNode := &yaml.Node{}
		valNode := &yaml.Node{}

		// Encode the key into a YAML node
		if err := keyNode.Encode(key); err != nil {
			return nil, err
		}

		// Retrieve the value from the map
		val, ok := m.innerMap[key]
		if !ok {
			return nil, fmt.Errorf("key %v not found in innerMap", key)
		}

		// Encode the value into a YAML node
		if err := valNode.Encode(val); err != nil {
			return nil, err
		}

		// Append the key and value nodes to the mapping node
		node.Content = append(node.Content, keyNode, valNode)
	}

	return node, nil
}
