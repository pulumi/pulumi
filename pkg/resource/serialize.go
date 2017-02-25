// Copyright 2016 Pulumi, Inc. All rights reserved.

package resource

import (
	"bytes"
	"encoding/json"
	"reflect"

	"github.com/pulumi/coconut/pkg/compiler/core"
	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/contract"
)

// SerializedSnapshot is a serializable, flattened CocoGL graph structure, specifically for snapshots.   It is similar
// to the actual Snapshot interface, except that it flattens and rearranges a few data structures for serializability.
type SerializedSnapshot struct {
	Husk      tokens.QName           `json:"husk"`                // the target environment name.
	Package   tokens.PackageName     `json:"package"`             // the package which created this graph.
	Args      *core.Args             `json:"args,omitempty"`      // the blueprint args for graph creation.
	Refs      *string                `json:"refs,omitempty"`      // the ref alias, if any (`#ref` by default).
	Resources *SerializedResourceMap `json:"resources,omitempty"` // a map of monikers to resource vertices.
}

// DefaultSnapshotReftag is the default ref tag for intra-graph edges.
const DefaultSnapshotReftag = "#ref"

// SerializedResource is a serializable vertex within a CocoGL graph, specifically for resource snapshots.
type SerializedResource struct {
	ID         *ID                    `json:"id,omitempty"`         // the provider ID for this resource, if any.
	Type       tokens.Type            `json:"type"`                 // this resource's full type token.
	Properties *SerializedPropertyMap `json:"properties,omitempty"` // an untyped bag of properties.
}

// SerializedPropertyMap is a property map from resource key to the underlying property value.
type SerializedPropertyMap map[string]interface{}

// SerializeSnapshot turns a snapshot into a CocoGL data structure suitable for serialization.
func SerializeSnapshot(snap Snapshot, reftag string) *SerializedSnapshot {
	contract.Assert(snap != nil)

	// Initialize the reftag if needed, and only serialize it if overridden.
	var refp *string
	if reftag == "" {
		reftag = DefaultSnapshotReftag
	} else {
		refp = &reftag
	}

	// Serialize all vertices and only include a vertex section if non-empty.
	var resm *SerializedResourceMap
	if snapres := snap.Resources(); len(snapres) > 0 {
		resm = NewSerializedResourceMap()
		for _, res := range snap.Resources() {
			m := res.Moniker()
			contract.Assertf(string(m) != "", "Unexpected empty resource moniker")
			contract.Assertf(!resm.Has(m), "Unexpected duplicate resource moniker '%v'", m)
			resm.Add(m, SerializeResource(res, reftag))
		}
	}

	// Only include the arguments in the output if non-emtpy.
	var argsp *core.Args
	if args := snap.Args(); len(args) > 0 {
		argsp = &args
	}

	return &SerializedSnapshot{
		Husk:      snap.Husk(),
		Package:   snap.Pkg(), // TODO: eventually, this should carry version metadata too.
		Args:      argsp,
		Refs:      refp,
		Resources: resm,
	}
}

// SerializeResource turns a resource into a CocoGL data structure suitable for serialization.
func SerializeResource(res Resource, reftag string) *SerializedResource {
	contract.Assert(res != nil)

	// Only serialize the ID if it is non-empty.
	var idp *ID
	if id := res.ID(); id != ID("") {
		idp = &id
	}

	// Serialize all properties recursively, and add them if non-empty.
	var props *SerializedPropertyMap
	if result, use := SerializeProperties(res.Properties(), reftag); use {
		props = &result
	}

	return &SerializedResource{
		ID:         idp,
		Type:       res.Type(),
		Properties: props,
	}
}

// SerializeProperties serializes a resource property bag so that it's suitable for serialization.
func SerializeProperties(props PropertyMap, reftag string) (SerializedPropertyMap, bool) {
	dst := make(SerializedPropertyMap)
	for _, k := range StablePropertyKeys(props) {
		if v, use := SerializeProperty(props[k], reftag); use {
			dst[string(k)] = v
		}
	}
	if len(dst) > 0 {
		return dst, true
	}
	return nil, false
}

// SerializeProperty serializes a resource property value so that it's suitable for serialization.
func SerializeProperty(prop PropertyValue, reftag string) (interface{}, bool) {
	// Skip nulls.
	if prop.IsNull() {
		return nil, false
	}

	// For arrays, make sure to recurse.
	if prop.IsArray() {
		var arr []interface{}
		for _, elem := range prop.ArrayValue() {
			if v, use := SerializeProperty(elem, reftag); use {
				arr = append(arr, v)
			}
		}
		if len(arr) > 0 {
			return arr, true
		}
		return nil, false
	}

	// Also for objects, recurse and use naked properties.
	if prop.IsObject() {
		return SerializeProperties(prop.ObjectValue(), reftag)
	}

	// Morph resources into their equivalent `{ "#ref": "<moniker>" }` form.
	if prop.IsResource() {
		return map[string]string{
			reftag: string(prop.ResourceValue()),
		}, true
	}

	// All others are returned as-is.
	return prop.V, true
}

// DeserializeSnapshot takes a serialized CocoGL snapshot data structure and returns its associated snapshot.
func DeserializeSnapshot(ctx *Context, ser *SerializedSnapshot) Snapshot {
	// Determine the reftag to use.
	var reftag string
	if ser.Refs == nil {
		reftag = DefaultSnapshotReftag
	} else {
		reftag = *ser.Refs
	}

	// For every serialized resource vertex, create a SerializedResource out of it.
	var resources []Resource
	if ser.Resources != nil {
		// TODO: we need to enumerate resources in the specific order in which they were emitted.
		for _, kvp := range ser.Resources.Iter() {
			// Deserialize the resources, if they exist.
			res := kvp.Value
			var props PropertyMap
			if res.Properties == nil {
				props = make(PropertyMap)
			} else {
				props = DeserializeProperties(*res.Properties, reftag)
			}

			// And now just produce a resource object using the information available.
			var id ID
			if res.ID != nil {
				id = *res.ID
			}

			resources = append(resources, NewResource(id, kvp.Key, res.Type, props))
		}
	}

	var args core.Args
	if ser.Args != nil {
		args = *ser.Args
	}

	return NewSnapshot(ctx, ser.Husk, ser.Package, args, resources)
}

func DeserializeProperties(props SerializedPropertyMap, reftag string) PropertyMap {
	result := make(PropertyMap)
	for k, prop := range props {
		result[PropertyKey(k)] = DeserializeProperty(prop, reftag)
	}
	return result
}

func DeserializeProperty(v interface{}, reftag string) PropertyValue {
	if v != nil {
		switch w := v.(type) {
		case bool:
			return NewPropertyBool(w)
		case float64:
			return NewPropertyNumber(w)
		case string:
			return NewPropertyString(w)
		case []interface{}:
			var arr []PropertyValue
			for _, elem := range w {
				arr = append(arr, DeserializeProperty(elem, reftag))
			}
			return NewPropertyArray(arr)
		case map[string]interface{}:
			// If the map has a single entry and it is the reftag, this is a moniker.
			if len(w) == 1 {
				if tag, has := w[reftag]; has {
					if tagstr, isstring := tag.(string); isstring {
						return NewPropertyResource(Moniker(tagstr))
					}
				}
			}

			// Otherwise, this is an arbitrary object value.
			obj := DeserializeProperties(SerializedPropertyMap(w), reftag)
			return NewPropertyObject(obj)
		default:
			contract.Failf("Unrecognized property type: %v", reflect.ValueOf(v))
		}
	}

	return NewPropertyNull()
}

// SerializedResourceMap is a map of moniker to resource, that also preserves a stable order of its keys.  This ensures
// enumerations are ordered deterministically, versus Go's built-in map type whose enumeration is randomized.
// Additionally, because of this stable ordering, marshaling to and from JSON also preserves the order of keys.
type SerializedResourceMap struct {
	m    map[Moniker]*SerializedResource
	keys []Moniker
}

func NewSerializedResourceMap() *SerializedResourceMap {
	return &SerializedResourceMap{m: make(map[Moniker]*SerializedResource)}
}

func (m *SerializedResourceMap) Keys() []Moniker { return m.keys }
func (m *SerializedResourceMap) Len() int        { return len(m.keys) }

func (m *SerializedResourceMap) Add(k Moniker, v *SerializedResource) {
	_, has := m.m[k]
	contract.Assertf(!has, "Unexpected duplicate key '%v' added to map")
	m.m[k] = v
	m.keys = append(m.keys, k)
}

func (m *SerializedResourceMap) Delete(k Moniker) {
	_, has := m.m[k]
	contract.Assertf(has, "Unexpected delete of non-existent key key '%v'")
	delete(m.m, k)
	for i, ek := range m.keys {
		if ek == k {
			newk := m.keys[:i]
			m.keys = append(newk, m.keys[i+1:]...)
			break
		}
		contract.Assertf(i != len(m.keys)-1, "Expected to find deleted key '%v' in map's keys")
	}
}

func (m *SerializedResourceMap) Get(k Moniker) (*SerializedResource, bool) {
	v, has := m.m[k]
	return v, has
}

func (m *SerializedResourceMap) Has(k Moniker) bool {
	_, has := m.m[k]
	return has
}

func (m *SerializedResourceMap) Must(k Moniker) *SerializedResource {
	v, has := m.m[k]
	contract.Assertf(has, "Expected key '%v' to exist in this map", k)
	return v
}

func (m *SerializedResourceMap) Set(k Moniker, v *SerializedResource) {
	_, has := m.m[k]
	contract.Assertf(has, "Expected key '%v' to exist in this map for setting an element", k)
	m.m[k] = v
}

func (m *SerializedResourceMap) SetOrAdd(k Moniker, v *SerializedResource) {
	if _, has := m.m[k]; has {
		m.Set(k, v)
	} else {
		m.Add(k, v)
	}
}

type SerializedResourceKeyValue struct {
	Key   Moniker
	Value *SerializedResource
}

// Iter can be used to conveniently range over a map's contents stably.
func (m *SerializedResourceMap) Iter() []SerializedResourceKeyValue {
	var kvps []SerializedResourceKeyValue
	for _, k := range m.Keys() {
		kvps = append(kvps, SerializedResourceKeyValue{k, m.Must(k)})
	}
	return kvps
}

func (m *SerializedResourceMap) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteString("{")
	for i, k := range m.Keys() {
		if i != 0 {
			b.WriteString(",")
		}

		kb, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		b.Write(kb)

		b.WriteString(":")

		vb, err := json.Marshal(m.Must(k))
		if err != nil {
			return nil, err
		}
		b.Write(vb)
	}
	b.WriteString("}")
	return b.Bytes(), nil
}

func (m *SerializedResourceMap) UnmarshalJSON(b []byte) error {
	contract.Assert(m.m == nil)
	m.m = make(map[Moniker]*SerializedResource)

	// Do a pass and read keys and values in the right order.
	rdr := bytes.NewReader(b)
	dec := json.NewDecoder(rdr)

	// First, eat the open object curly '{':
	contract.Assert(dec.More())
	opencurly, err := dec.Token()
	if err != nil {
		return err
	}
	contract.Assert(opencurly.(json.Delim) == '{')

	// Parse out every resource key (Moniker) and element (*SerializedResource):
	for dec.More() {
		// See if we've reached the closing '}'; if yes, chew on it and break.
		token, err := dec.Token()
		if err != nil {
			return err
		}
		if closecurly, isclose := token.(json.Delim); isclose {
			contract.Assert(closecurly == '}')
			break
		}

		k := Moniker(token.(string))
		contract.Assert(dec.More())
		var v *SerializedResource
		if err := dec.Decode(&v); err != nil {
			return err
		}
		contract.Assert(!m.Has(k))
		m.Add(k, v)
	}

	return nil
}
