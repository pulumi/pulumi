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

package resource

import (
	"bytes"
	"encoding/json"
	"reflect"
	"time"

	"github.com/pulumi/lumi/pkg/compiler/core"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// DeploymentRecord is a serializable, flattened LumiGL graph structure, representing a deployment.   It is similar
// to the actual Snapshot interface, except that it flattens and rearranges a few data structures for serializability.
// Over time, we also expect this to gather more information about deployments themselves.
type DeploymentRecord struct {
	Time      time.Time            `json:"time"`                // the time of the deployment.
	Reftag    *string              `json:"reftag,omitempty"`    // the ref alias, if any (`#ref` by default).
	Package   tokens.Package       `json:"package"`             // the package deployed by this record.
	Args      *core.Args           `json:"args,omitempty"`      // the blueprint args for graph creation.
	Resources *DeploymentResources `json:"resources,omitempty"` // a map of URNs to resource vertices.
}

// DefaultDeploymentReftag is the default ref tag for intra-graph edges.
const DefaultDeploymentReftag = "#ref"

// Deployment is a serializable vertex within a LumiGL graph, specifically for resource snapshots.
type Deployment struct {
	ID      *ID                    `json:"id,omitempty"`      // the provider ID for this resource, if any.
	Type    tokens.Type            `json:"type"`              // this resource's full type token.
	Inputs  map[string]interface{} `json:"inputs,omitempty"`  // the input properties from the program.
	Outputs map[string]interface{} `json:"outputs,omitempty"` // the output properties from the resource provider.
}

// SerializeDeploymentRecord serializes an entire snapshot using the given reftag for cross-resource references.
func SerializeDeploymentRecord(snap Snapshot, reftag string) *DeploymentRecord {
	// Initialize the reftag if needed, and only serialize if overridden.
	var refp *string
	if reftag == "" {
		reftag = DefaultDeploymentReftag
	} else {
		refp = &reftag
	}

	// Serialize all vertices and only include a vertex section if non-empty.
	var resm *DeploymentResources
	if snapres := snap.Resources(); len(snapres) > 0 {
		resm = NewDeploymentResources()
		for _, res := range snap.Resources() {
			m := res.URN()
			contract.Assertf(string(m) != "", "Unexpected empty resource URN")
			contract.Assertf(!resm.Has(m), "Unexpected duplicate resource URN '%v'", m)
			resm.Add(m, SerializeDeployment(res, reftag))
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

// SerializeDeployment turns a resource into a LumiGL data structure suitable for serialization.
func SerializeDeployment(res Resource, reftag string) *Deployment {
	contract.Assert(res != nil)

	// Only serialize the ID if it is non-empty.
	var idp *ID
	if id := res.ID(); id != ID("") {
		idp = &id
	}

	// Serialize all input and output properties recursively, and add them if non-empty.
	var inputs map[string]interface{}
	if inp := res.Inputs(); inp != nil {
		inputs = SerializeDeploymentProperties(inp, reftag)
	}
	var outputs map[string]interface{}
	if outp := res.Outputs(); outp != nil {
		outputs = SerializeDeploymentProperties(outp, reftag)
	}

	return &Deployment{
		ID:      idp,
		Type:    res.Type(),
		Inputs:  inputs,
		Outputs: outputs,
	}
}

// SerializeDeploymentProperties serializes a resource property bag so that it's suitable for serialization.
func SerializeDeploymentProperties(props PropertyMap, reftag string) map[string]interface{} {
	dst := make(map[string]interface{})
	for _, k := range StablePropertyKeys(props) {
		if v := SerializeDeploymentProperty(props[k], reftag); v != nil {
			dst[string(k)] = v
		}
	}
	return dst
}

// SerializeDeploymentProperty serializes a resource property value so that it's suitable for serialization.
func SerializeDeploymentProperty(prop PropertyValue, reftag string) interface{} {
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
			dstarr[i] = SerializeDeploymentProperty(elem, reftag)
		}
		return dstarr
	}

	// Also for objects, recurse and use naked properties.
	if prop.IsObject() {
		return SerializeDeploymentProperties(prop.ObjectValue(), reftag)
	}

	// Morph resources into their equivalent `{ "#ref": "<URN>" }` form.
	if prop.IsResource() {
		return map[string]string{
			reftag: string(prop.ResourceValue()),
		}
	}

	// All others are returned as-is.
	return prop.V
}

// DeserializeDeploymentProperties deserializes an entire map of deployment properties into a resource property map.
func DeserializeDeploymentProperties(props map[string]interface{}, reftag string) PropertyMap {
	result := make(PropertyMap)
	for k, prop := range props {
		result[PropertyKey(k)] = DeserializeDeploymentProperty(prop, reftag)
	}
	return result
}

// DeserializeDeploymentProperty deserializes a single deployment property into a resource property value.
func DeserializeDeploymentProperty(v interface{}, reftag string) PropertyValue {
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
				arr = append(arr, DeserializeDeploymentProperty(elem, reftag))
			}
			return NewArrayProperty(arr)
		case map[string]interface{}:
			// If the map has a single entry and it is the reftag, this is a URN.
			if len(w) == 1 {
				if tag, has := w[reftag]; has {
					if tagstr, isstring := tag.(string); isstring {
						return NewResourceProperty(URN(tagstr))
					}
				}
			}

			// Otherwise, this is an arbitrary object value.
			obj := DeserializeDeploymentProperties(w, reftag)
			return NewObjectProperty(obj)
		default:
			contract.Failf("Unrecognized property type: %v", reflect.ValueOf(v))
		}
	}

	return NewNullProperty()
}

// DeploymentResources is a map of URN to resource, that also preserves a stable order of its keys.  This ensures
// enumerations are ordered deterministically, versus Go's built-in map type whose enumeration is randomized.
// Additionally, because of this stable ordering, marshaling to and from JSON also preserves the order of keys.
type DeploymentResources struct {
	m    map[URN]*Deployment
	keys []URN
}

func NewDeploymentResources() *DeploymentResources {
	return &DeploymentResources{m: make(map[URN]*Deployment)}
}

func (m *DeploymentResources) Keys() []URN { return m.keys }
func (m *DeploymentResources) Len() int    { return len(m.keys) }

func (m *DeploymentResources) Add(k URN, v *Deployment) {
	_, has := m.m[k]
	contract.Assertf(!has, "Unexpected duplicate key '%v' added to map")
	m.m[k] = v
	m.keys = append(m.keys, k)
}

func (m *DeploymentResources) Delete(k URN) {
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

func (m *DeploymentResources) Get(k URN) (*Deployment, bool) {
	v, has := m.m[k]
	return v, has
}

func (m *DeploymentResources) Has(k URN) bool {
	_, has := m.m[k]
	return has
}

func (m *DeploymentResources) Must(k URN) *Deployment {
	v, has := m.m[k]
	contract.Assertf(has, "Expected key '%v' to exist in this map", k)
	return v
}

func (m *DeploymentResources) Set(k URN, v *Deployment) {
	_, has := m.m[k]
	contract.Assertf(has, "Expected key '%v' to exist in this map for setting an element", k)
	m.m[k] = v
}

func (m *DeploymentResources) SetOrAdd(k URN, v *Deployment) {
	if _, has := m.m[k]; has {
		m.Set(k, v)
	} else {
		m.Add(k, v)
	}
}

type DeploymentKeyValue struct {
	Key   URN
	Value *Deployment
}

// Iter can be used to conveniently range over a map's contents stably.
func (m *DeploymentResources) Iter() []DeploymentKeyValue {
	var kvps []DeploymentKeyValue
	for _, k := range m.Keys() {
		kvps = append(kvps, DeploymentKeyValue{k, m.Must(k)})
	}
	return kvps
}

func (m *DeploymentResources) MarshalJSON() ([]byte, error) {
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

func (m *DeploymentResources) UnmarshalJSON(b []byte) error {
	contract.Assert(m.m == nil)
	m.m = make(map[URN]*Deployment)

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

	// Parse out every resource key (URN) and element (*Deployment):
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

		k := URN(token.(string))
		contract.Assert(dec.More())
		var v *Deployment
		if err := dec.Decode(&v); err != nil {
			return err
		}
		contract.Assert(!m.Has(k))
		m.Add(k, v)
	}

	return nil
}
