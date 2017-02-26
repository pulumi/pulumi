// Copyright 2016 Pulumi, Inc. All rights reserved.

package resource

import (
	"bytes"
	"encoding/json"
	"reflect"
	"time"

	"github.com/pulumi/coconut/pkg/compiler/core"
	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/contract"
)

// Deployment is a serialized deployment target plus a record of the latest deployment.
type Deployment struct {
	Husk   tokens.QName      `json:"husk"`             // the target environment name.
	Latest *DeploymentRecord `json:"latest,omitempty"` // the latest/current deployment record.
}

// DeploymentRecord is a serializable, flattened CocoGL graph structure, representing a deployment.   It is similar
// to the actual Snapshot interface, except that it flattens and rearranges a few data structures for serializability.
// Over time, we also expect this to gather more information about deployments themselves.
type DeploymentRecord struct {
	Time      time.Time              `json:"time"`                // the time of the deployment.
	Reftag    *string                `json:"reftag,omitempty"`    // the ref alias, if any (`#ref` by default).
	Package   tokens.Package         `json:"package"`             // the nut that this husk belongs to.
	Args      *core.Args             `json:"args,omitempty"`      // the blueprint args for graph creation.
	Resources *ResourceDeploymentMap `json:"resources,omitempty"` // a map of monikers to resource vertices.
}

// DefaultDeploymentReftag is the default ref tag for intra-graph edges.
const DefaultDeploymentReftag = "#ref"

// ResourceDeployment is a serializable vertex within a CocoGL graph, specifically for resource snapshots.
type ResourceDeployment struct {
	ID         *ID                  `json:"id,omitempty"`         // the provider ID for this resource, if any.
	Type       tokens.Type          `json:"type"`                 // this resource's full type token.
	Properties *DeployedPropertyMap `json:"properties,omitempty"` // an untyped bag of properties.
}

// DeployedPropertyMap is a property map from resource key to the underlying property value.
type DeployedPropertyMap map[string]interface{}

// SerializeDeployment turns a snapshot into a CocoGL data structure suitable for serialization.
func SerializeDeployment(husk tokens.QName, snap Snapshot, reftag string) *Deployment {
	// If snap is nil, that's okay, we will just create an empty deployment; otherwise, serialize the whole snapshot.
	var latest *DeploymentRecord
	if snap != nil {
		latest = serializeDeploymentRecord(snap, reftag)
	}
	return &Deployment{
		Husk:   husk,
		Latest: latest,
	}
}

func serializeDeploymentRecord(snap Snapshot, reftag string) *DeploymentRecord {
	// Initialize the reftag if needed, and only serialize if overridden.
	var refp *string
	if reftag == "" {
		reftag = DefaultDeploymentReftag
	} else {
		refp = &reftag
	}

	// Serialize all vertices and only include a vertex section if non-empty.
	var resm *ResourceDeploymentMap
	if snapres := snap.Resources(); len(snapres) > 0 {
		resm = NewResourceDeploymentMap()
		for _, res := range snap.Resources() {
			m := res.Moniker()
			contract.Assertf(string(m) != "", "Unexpected empty resource moniker")
			contract.Assertf(!resm.Has(m), "Unexpected duplicate resource moniker '%v'", m)
			resm.Add(m, serializeResourceDeployment(res, reftag))
		}
	}

	// Initialize the args pointer, but only serialize if the args are non-empty.
	var argsp *core.Args
	if args := snap.Args(); len(args) > 0 {
		argsp = &args
	}

	return &DeploymentRecord{
		Time:      time.Now(),
		Reftag:    refp,
		Package:   snap.Pkg(),
		Args:      argsp,
		Resources: resm,
	}
}

// serializeResourceDeployment turns a resource into a CocoGL data structure suitable for serialization.
func serializeResourceDeployment(res Resource, reftag string) *ResourceDeployment {
	contract.Assert(res != nil)

	// Only serialize the ID if it is non-empty.
	var idp *ID
	if id := res.ID(); id != ID("") {
		idp = &id
	}

	// Serialize all properties recursively, and add them if non-empty.
	var props *DeployedPropertyMap
	if result, use := serializeProperties(res.Properties(), reftag); use {
		props = &result
	}

	return &ResourceDeployment{
		ID:         idp,
		Type:       res.Type(),
		Properties: props,
	}
}

// serializeProperties serializes a resource property bag so that it's suitable for serialization.
func serializeProperties(props PropertyMap, reftag string) (DeployedPropertyMap, bool) {
	dst := make(DeployedPropertyMap)
	for _, k := range StablePropertyKeys(props) {
		if v, use := serializeProperty(props[k], reftag); use {
			dst[string(k)] = v
		}
	}
	if len(dst) > 0 {
		return dst, true
	}
	return nil, false
}

// serializeProperty serializes a resource property value so that it's suitable for serialization.
func serializeProperty(prop PropertyValue, reftag string) (interface{}, bool) {
	// Skip nulls.
	if prop.IsNull() {
		return nil, false
	}

	// For arrays, make sure to recurse.
	if prop.IsArray() {
		var arr []interface{}
		for _, elem := range prop.ArrayValue() {
			if v, use := serializeProperty(elem, reftag); use {
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
		return serializeProperties(prop.ObjectValue(), reftag)
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

// DeserializeDeploymentRecord takes a serialized deployment record and returns its associated snapshot.
func DeserializeDeployment(ctx *Context, ser *Deployment) Snapshot {
	latest := ser.Latest
	if latest == nil {
		return nil
	}

	// Determine the reftag to use.
	var reftag string
	if latest.Reftag == nil {
		reftag = DefaultDeploymentReftag
	} else {
		reftag = *latest.Reftag
	}

	// For every serialized resource vertex, create a ResourceDeployment out of it.
	var resources []Resource
	if latest.Resources != nil {
		// TODO: we need to enumerate resources in the specific order in which they were emitted.
		for _, kvp := range latest.Resources.Iter() {
			// Deserialize the resources, if they exist.
			res := kvp.Value
			var props PropertyMap
			if res.Properties == nil {
				props = make(PropertyMap)
			} else {
				props = deserializeProperties(*res.Properties, reftag)
			}

			// And now just produce a resource object using the information available.
			var id ID
			if res.ID != nil {
				id = *res.ID
			}

			resources = append(resources, NewResource(id, kvp.Key, res.Type, props))
		}
	}

	return NewSnapshot(ctx, ser.Husk, latest.Package, *latest.Args, resources)
}

func deserializeProperties(props DeployedPropertyMap, reftag string) PropertyMap {
	result := make(PropertyMap)
	for k, prop := range props {
		result[PropertyKey(k)] = deserializeProperty(prop, reftag)
	}
	return result
}

func deserializeProperty(v interface{}, reftag string) PropertyValue {
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
				arr = append(arr, deserializeProperty(elem, reftag))
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
			obj := deserializeProperties(DeployedPropertyMap(w), reftag)
			return NewPropertyObject(obj)
		default:
			contract.Failf("Unrecognized property type: %v", reflect.ValueOf(v))
		}
	}

	return NewPropertyNull()
}

// ResourceDeploymentMap is a map of moniker to resource, that also preserves a stable order of its keys.  This ensures
// enumerations are ordered deterministically, versus Go's built-in map type whose enumeration is randomized.
// Additionally, because of this stable ordering, marshaling to and from JSON also preserves the order of keys.
type ResourceDeploymentMap struct {
	m    map[Moniker]*ResourceDeployment
	keys []Moniker
}

func NewResourceDeploymentMap() *ResourceDeploymentMap {
	return &ResourceDeploymentMap{m: make(map[Moniker]*ResourceDeployment)}
}

func (m *ResourceDeploymentMap) Keys() []Moniker { return m.keys }
func (m *ResourceDeploymentMap) Len() int        { return len(m.keys) }

func (m *ResourceDeploymentMap) Add(k Moniker, v *ResourceDeployment) {
	_, has := m.m[k]
	contract.Assertf(!has, "Unexpected duplicate key '%v' added to map")
	m.m[k] = v
	m.keys = append(m.keys, k)
}

func (m *ResourceDeploymentMap) Delete(k Moniker) {
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

func (m *ResourceDeploymentMap) Get(k Moniker) (*ResourceDeployment, bool) {
	v, has := m.m[k]
	return v, has
}

func (m *ResourceDeploymentMap) Has(k Moniker) bool {
	_, has := m.m[k]
	return has
}

func (m *ResourceDeploymentMap) Must(k Moniker) *ResourceDeployment {
	v, has := m.m[k]
	contract.Assertf(has, "Expected key '%v' to exist in this map", k)
	return v
}

func (m *ResourceDeploymentMap) Set(k Moniker, v *ResourceDeployment) {
	_, has := m.m[k]
	contract.Assertf(has, "Expected key '%v' to exist in this map for setting an element", k)
	m.m[k] = v
}

func (m *ResourceDeploymentMap) SetOrAdd(k Moniker, v *ResourceDeployment) {
	if _, has := m.m[k]; has {
		m.Set(k, v)
	} else {
		m.Add(k, v)
	}
}

type ResourceDeploymentKeyValue struct {
	Key   Moniker
	Value *ResourceDeployment
}

// Iter can be used to conveniently range over a map's contents stably.
func (m *ResourceDeploymentMap) Iter() []ResourceDeploymentKeyValue {
	var kvps []ResourceDeploymentKeyValue
	for _, k := range m.Keys() {
		kvps = append(kvps, ResourceDeploymentKeyValue{k, m.Must(k)})
	}
	return kvps
}

func (m *ResourceDeploymentMap) MarshalJSON() ([]byte, error) {
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

func (m *ResourceDeploymentMap) UnmarshalJSON(b []byte) error {
	contract.Assert(m.m == nil)
	m.m = make(map[Moniker]*ResourceDeployment)

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

	// Parse out every resource key (Moniker) and element (*ResourceDeployment):
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
		var v *ResourceDeployment
		if err := dec.Decode(&v); err != nil {
			return err
		}
		contract.Assert(!m.Has(k))
		m.Add(k, v)
	}

	return nil
}
