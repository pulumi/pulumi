// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package environment

import (
	"bytes"
	"encoding/json"
	"reflect"
	"time"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// Deployment is a serializable, flattened LumiGL graph structure, representing a deploy.   It is similar
// to the actual Snapshot structure, except that it flattens and rearranges a few data structures for serializability.
// Over time, we also expect this to gather more information about deploys themselves.
type Deployment struct {
	Time      time.Time   `json:"time"`                // the time of the deploy.
	Info      interface{} `json:"info,omitempty"`      // optional information about the source.
	Resources *Resources  `json:"resources,omitempty"` // a map of resource.URNs to resource vertices.
}

// Resource is a serializable vertex within a LumiGL graph, specifically for resource snapshots.
type Resource struct {
	ID       resource.ID            `json:"id"`                 // the provider ID for this resource, if any.
	Type     tokens.Type            `json:"type"`               // this resource's full type token.
	Inputs   map[string]interface{} `json:"inputs,omitempty"`   // the input properties from the program.
	Defaults map[string]interface{} `json:"defaults,omitempty"` // the default property values from the provider.
	Outputs  map[string]interface{} `json:"outputs,omitempty"`  // the output properties from the resource provider.
}

// SerializeDeployment serializes an entire snapshot as a deploy record.
func SerializeDeployment(snap *deploy.Snapshot) *Deployment {
	// Serialize all vertices and only include a vertex section if non-empty.
	var resm *Resources
	if snapres := snap.Resources; len(snapres) > 0 {
		resm = NewResources()
		for _, res := range snapres {
			urn := res.URN
			contract.Assertf(string(urn) != "", "Unexpected empty resource resource.URN")
			contract.Assertf(!resm.Has(urn), "Unexpected duplicate resource resource.URN '%v'", urn)
			resm.Add(urn, SerializeResource(res))
		}
	}

	return &Deployment{
		Time:      time.Now(),
		Info:      snap.Info,
		Resources: resm,
	}
}

// SerializeResource turns a resource into a LumiGL data structure suitable for serialization.
func SerializeResource(res *resource.State) *Resource {
	contract.Assert(res != nil)

	// Serialize all input and output properties recursively, and add them if non-empty.
	var inputs map[string]interface{}
	if inp := res.Inputs; inp != nil {
		inputs = SerializeProperties(inp)
	}
	var defaults map[string]interface{}
	if defp := res.Defaults; defp != nil {
		defaults = SerializeProperties(defp)
	}
	var outputs map[string]interface{}
	if outp := res.Outputs; outp != nil {
		outputs = SerializeProperties(outp)
	}

	return &Resource{
		ID:       res.ID,
		Type:     res.Type,
		Inputs:   inputs,
		Defaults: defaults,
		Outputs:  outputs,
	}
}

// SerializeProperties serializes a resource property bag so that it's suitable for serialization.
func SerializeProperties(props resource.PropertyMap) map[string]interface{} {
	dst := make(map[string]interface{})
	for _, k := range props.StableKeys() {
		if v := SerializePropertyValue(props[k]); v != nil {
			dst[string(k)] = v
		}
	}
	return dst
}

// SerializePropertyValue serializes a resource property value so that it's suitable for serialization.
func SerializePropertyValue(prop resource.PropertyValue) interface{} {
	contract.Assert(!prop.IsComputed())

	// Skip nulls and "outputs"; the former needn't be serialized, and the latter happens if there is an output
	// that hasn't materialized (either because we're serializing inputs or the provider didn't give us the value).
	if !prop.HasValue() {
		return nil
	}

	// For arrays, make sure to recurse.
	if prop.IsArray() {
		srcarr := prop.ArrayValue()
		dstarr := make([]interface{}, len(srcarr))
		for i, elem := range prop.ArrayValue() {
			dstarr[i] = SerializePropertyValue(elem)
		}
		return dstarr
	}

	// Also for objects, recurse and use naked properties.
	if prop.IsObject() {
		return SerializeProperties(prop.ObjectValue())
	}

	// For assets, we need to serialize them a little carefully, so we can recover them afterwards.
	if prop.IsAsset() {
		return prop.AssetValue().Serialize()
	} else if prop.IsArchive() {
		return prop.ArchiveValue().Serialize()
	}

	// All others are returned as-is.
	return prop.V
}

// DeserializeProperties deserializes an entire map of deploy properties into a resource property map.
func DeserializeProperties(props map[string]interface{}) resource.PropertyMap {
	result := make(resource.PropertyMap)
	for k, prop := range props {
		result[resource.PropertyKey(k)] = DeserializePropertyValue(prop)
	}
	return result
}

// DeserializePropertyValue deserializes a single deploy property into a resource property value.
func DeserializePropertyValue(v interface{}) resource.PropertyValue {
	if v != nil {
		switch w := v.(type) {
		case bool:
			return resource.NewBoolProperty(w)
		case float64:
			return resource.NewNumberProperty(w)
		case string:
			return resource.NewStringProperty(w)
		case []interface{}:
			var arr []resource.PropertyValue
			for _, elem := range w {
				arr = append(arr, DeserializePropertyValue(elem))
			}
			return resource.NewArrayProperty(arr)
		case map[string]interface{}:
			obj := DeserializeProperties(w)
			// This could be an asset or archive; if so, recover its type.
			objmap := obj.Mappable()
			if asset, isasset := resource.DeserializeAsset(objmap); isasset {
				return resource.NewAssetProperty(asset)
			} else if archive, isarchive := resource.DeserializeArchive(objmap); isarchive {
				return resource.NewArchiveProperty(archive)
			}
			// Otherwise, it's just a weakly typed object map.
			return resource.NewObjectProperty(obj)
		default:
			contract.Failf("Unrecognized property type: %v", reflect.ValueOf(v))
		}
	}

	return resource.NewNullProperty()
}

// Resources is a map of URN to resource, that also preserves a stable order of its keys.  This ensures
// enumerations are ordered deterministically, versus Go's built-in map type whose enumeration is randomized.
// Additionally, because of this stable ordering, marshaling to and from JSON also preserves the order of keys.
type Resources struct {
	m    map[resource.URN]*Resource
	keys []resource.URN
}

func NewResources() *Resources {
	return &Resources{m: make(map[resource.URN]*Resource)}
}

func (m *Resources) Keys() []resource.URN { return m.keys }
func (m *Resources) Len() int             { return len(m.keys) }

func (m *Resources) Add(k resource.URN, v *Resource) {
	_, has := m.m[k]
	contract.Assertf(!has, "Unexpected duplicate key '%v' added to map")
	m.m[k] = v
	m.keys = append(m.keys, k)
}

func (m *Resources) Delete(k resource.URN) {
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

func (m *Resources) Get(k resource.URN) (*Resource, bool) {
	v, has := m.m[k]
	return v, has
}

func (m *Resources) Has(k resource.URN) bool {
	_, has := m.m[k]
	return has
}

func (m *Resources) Must(k resource.URN) *Resource {
	v, has := m.m[k]
	contract.Assertf(has, "Expected key '%v' to exist in this map", k)
	return v
}

func (m *Resources) Set(k resource.URN, v *Resource) {
	_, has := m.m[k]
	contract.Assertf(has, "Expected key '%v' to exist in this map for setting an element", k)
	m.m[k] = v
}

func (m *Resources) SetOrAdd(k resource.URN, v *Resource) {
	if _, has := m.m[k]; has {
		m.Set(k, v)
	} else {
		m.Add(k, v)
	}
}

type ResourceKV struct {
	Key   resource.URN
	Value *Resource
}

// Iter can be used to conveniently range over a map's contents stably.
func (m *Resources) Iter() []ResourceKV {
	var kvps []ResourceKV
	for _, k := range m.Keys() {
		kvps = append(kvps, ResourceKV{k, m.Must(k)})
	}
	return kvps
}

func (m *Resources) MarshalJSON() ([]byte, error) {
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

func (m *Resources) UnmarshalJSON(b []byte) error {
	contract.Assert(m.m == nil)
	m.m = make(map[resource.URN]*Resource)

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

	// Parse out every resource key (resource.URN) and element (*Deployment):
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

		k := resource.URN(token.(string))
		contract.Assert(dec.More())
		var v *Resource
		if err := dec.Decode(&v); err != nil {
			return err
		}
		contract.Assert(!m.Has(k))
		m.Add(k, v)
	}

	return nil
}
