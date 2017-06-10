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

package environment

import (
	"bytes"
	"encoding/json"
	"reflect"
	"time"

	"github.com/pulumi/lumi/pkg/compiler/core"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/resource/deployment"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// Deployment is a serializable, flattened LumiGL graph structure, representing a deployment.   It is similar
// to the actual Snapshot structure, except that it flattens and rearranges a few data structures for serializability.
// Over time, we also expect this to gather more information about deployments themselves.
type Deployment struct {
	Time           time.Time       `json:"time"`                // the time of the deployment.
	Package        tokens.Package  `json:"package"`             // the package deployed by this record.
	Args           *core.Args      `json:"args,omitempty"`      // the blueprint args for graph creation.
	ResourceStates *ResourceStates `json:"resources,omitempty"` // a map of resource.URNs to resource vertices.
}

// ResourceState is a serializable vertex within a LumiGL graph, specifically for resource snapshots.
type ResourceState struct {
	ID      resource.ID            `json:"id"`                // the provider ID for this resource, if any.
	Type    tokens.Type            `json:"type"`              // this resource's full type token.
	Inputs  map[string]interface{} `json:"inputs,omitempty"`  // the input properties from the program.
	Outputs map[string]interface{} `json:"outputs,omitempty"` // the output properties from the resource provider.
}

// SerializeDeployment serializes an entire snapshot as a deployment record.
func SerializeDeployment(snap *deployment.Snapshot) *Deployment {
	// Serialize all vertices and only include a vertex section if non-empty.
	var resm *ResourceStates
	if snapres := snap.ResourceStates(); len(snapres) > 0 {
		resm = NewResourceStates()
		for _, res := range snap.ResourceStates() {
			m := res.resource.URN()
			contract.Assertf(string(m) != "", "Unexpected empty resource resource.URN")
			contract.Assertf(!resm.Has(m), "Unexpected duplicate resource resource.URN '%v'", m)
			resm.Add(m, SerializeResourceState(res))
		}
	}

	// Initialize the args pointer, but only serialize if the args are non-empty.
	var argsp *core.Args
	if args := snap.Args(); len(args) > 0 {
		argsp = &args
	}

	return &Deployment{
		Time:           time.Now(),
		Reftag:         refp,
		Package:        snap.Pkg(),
		Args:           argsp,
		ResourceStates: resm,
	}
}

// SerializeResourceState turns a resource into a LumiGL data structure suitable for serialization.
func SerializeResourceState(res resource.State) *ResourceState {
	contract.Assert(res != nil)

	// Serialize all input and output properties recursively, and add them if non-empty.
	var inputs map[string]interface{}
	if inp := res.Inputs(); inp != nil {
		inputs = SerializeProperties(inp)
	}
	var outputs map[string]interface{}
	if outp := res.Outputs(); outp != nil {
		outputs = SerializeProperties(outp)
	}

	return &ResourceState{
		ID:      res.ID(),
		Type:    res.Type(),
		Inputs:  inputs,
		Outputs: outputs,
	}
}

// SerializeProperties serializes a resource property bag so that it's suitable for serialization.
func SerializeProperties(props resource.PropertyMap) map[string]interface{} {
	dst := make(map[string]interface{})
	for _, k := range StablePropertyKeys(props) {
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

	// All others are returned as-is.
	return prop.V
}

// DeserializeProperties deserializes an entire map of deployment properties into a resource property map.
func DeserializeProperties(props map[string]interface{}) resource.PropertyMap {
	result := make(PropertyMap)
	for k, prop := range props {
		result[PropertyKey(k)] = DeserializePropertyValue(prop)
	}
	return result
}

// DeserializePropertyValue deserializes a single deployment property into a resource property value.
func DeserializePropertyValue(v interface{}) resource.PropertyValue {
	if v != nil {
		switch w := v.(type) {
		case bool:
			return NewBoolProperty(w)
		case float64:
			return NewNumberProperty(w)
		case string:
			return NewStringProperty(w)
		case []interface{}:
			var arr []PropertyValue
			for _, elem := range w {
				arr = append(arr, DeserializePropertyValue(elem))
			}
			return NewArrayProperty(arr)
		case map[string]interface{}:
			obj := DeserializeProperties(w)
			return NewObjectProperty(obj)
		default:
			contract.Failf("Unrecognized property type: %v", reflect.ValueOf(v))
		}
	}

	return NewNullProperty()
}

// ResourceStates is a map of URN to resource, that also preserves a stable order of its keys.  This ensures
// enumerations are ordered deterministically, versus Go's built-in map type whose enumeration is randomized.
// Additionally, because of this stable ordering, marshaling to and from JSON also preserves the order of keys.
type ResourceStates struct {
	m    map[resource.URN]*ResourceState
	keys []resource.URN
}

func NewResourceStates() *ResourceStates {
	return &ResourceStates{m: make(map[resource.URN]*ResourceState)}
}

func (m *ResourceStates) Keys() []resource.URN { return m.keys }
func (m *ResourceStates) Len() int             { return len(m.keys) }

func (m *ResourceStates) Add(k resource.URN, v *ResourceState) {
	_, has := m.m[k]
	contract.Assertf(!has, "Unexpected duplicate key '%v' added to map")
	m.m[k] = v
	m.keys = append(m.keys, k)
}

func (m *ResourceStates) Delete(k resource.URN) {
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

func (m *ResourceStates) Get(k resource.URN) (*ResourceState, bool) {
	v, has := m.m[k]
	return v, has
}

func (m *ResourceStates) Has(k resource.URN) bool {
	_, has := m.m[k]
	return has
}

func (m *ResourceStates) Must(k resource.URN) *ResourceState {
	v, has := m.m[k]
	contract.Assertf(has, "Expected key '%v' to exist in this map", k)
	return v
}

func (m *ResourceStates) Set(k resource.URN, v *ResourceState) {
	_, has := m.m[k]
	contract.Assertf(has, "Expected key '%v' to exist in this map for setting an element", k)
	m.m[k] = v
}

func (m *ResourceStates) SetOrAdd(k resource.URN, v *ResourceState) {
	if _, has := m.m[k]; has {
		m.Set(k, v)
	} else {
		m.Add(k, v)
	}
}

type ResourceStateKV struct {
	Key   resource.URN
	Value *ResourceState
}

// Iter can be used to conveniently range over a map's contents stably.
func (m *ResourceStates) Iter() []ResourceStateKV {
	var kvps []ResourceStateKV
	for _, k := range m.Keys() {
		kvps = append(kvps, ResourceStateKV{k, m.Must(k)})
	}
	return kvps
}

func (m *ResourceStates) MarshalJSON() ([]byte, error) {
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

func (m *ResourceStates) UnmarshalJSON(b []byte) error {
	contract.Assert(m.m == nil)
	m.m = make(map[resource.URN]*ResourceState)

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
		var v *ResourceState
		if err := dec.Decode(&v); err != nil {
			return err
		}
		contract.Assert(!m.Has(k))
		m.Add(k, v)
	}

	return nil
}
