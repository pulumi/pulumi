// Copyright 2016 Marapongo, Inc. All rights reserved.

package resource

import (
	"reflect"

	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

// MuglSnapshot is a serializable, flattened MuGL graph structure, specifically for snapshots.   It is very similar to
// the actual Snapshot interface, except that it flattens and rearranges a few data structures for serializability.
type MuglSnapshot struct {
	Package   tokens.PackageName `json:"package"`             // the package which created this graph.
	Args      *core.Args         `json:"args,omitempty"`      // the blueprint args for graph creation.
	Refs      *string            `json:"refs,omitempty"`      // the ref alias, if any (`#ref` by default).
	Resources *MuglResourceMap   `json:"resources,omitempty"` // a map of monikers to resource vertices.
}

// DefaultSnapshotReftag is the default ref tag for intra-graph edges.
const DefaultSnapshotReftag = "#ref"

// MuglResourceMap is a map of object moniker to the resource vertex for that moniker.
type MuglResourceMap map[Moniker]*MuglResource

// MuglResource is a serializable vertex within a MuGL graph, specifically for resource snapshots.
type MuglResource struct {
	ID         *ID              `json:"id,omitempty"`         // the provider ID for this resource, if any.
	Type       tokens.Type      `json:"type"`                 // this resource's full type token.
	Properties *MuglPropertyMap `json:"properties,omitempty"` // an untyped bag of properties.
}

// MuglPropertyMap is a property map from resource key to the underlying property value.
type MuglPropertyMap map[string]interface{}

// SerializeSnapshot turns a snapshot into a MuGL data structure suitable for serialization.
func SerializeSnapshot(snap Snapshot, reftag string) *MuglSnapshot {
	contract.Assert(snap != nil)

	// Initialize the reftag if needed, and only serialize it if overridden.
	var refp *string
	if reftag == "" {
		reftag = DefaultSnapshotReftag
	} else {
		refp = &reftag
	}

	// Serialize all vertices and only include a vertex section if non-empty.
	var resmp *MuglResourceMap
	resm := make(MuglResourceMap)
	for _, res := range snap.Resources() {
		m := res.Moniker()
		contract.Assertf(string(m) != "", "Unexpected empty resource moniker")
		contract.Assertf(resm[m] == nil, "Unexpected duplicate resource moniker '%v'", m)
		resm[m] = SerializeResource(res, reftag)
	}
	if len(resm) > 0 {
		resmp = &resm
	}

	// Only include the arguments in the output if non-emtpy.
	var argsp *core.Args
	if args := snap.Args(); len(args) > 0 {
		argsp = &args
	}

	return &MuglSnapshot{
		Package:   snap.Pkg(), // TODO: eventually, this should carry version metadata too.
		Args:      argsp,
		Refs:      refp,
		Resources: resmp,
	}
}

// SerializeResource turns a resource into a MuGL data structure suitable for serialization.
func SerializeResource(res Resource, reftag string) *MuglResource {
	contract.Assert(res != nil)

	// Only serialize the ID if it is non-empty.
	var idp *ID
	if id := res.ID(); id != ID("") {
		idp = &id
	}

	// Serialize all properties recursively, and add them if non-empty.
	var props *MuglPropertyMap
	if result, use := SerializeProperties(res.Properties(), reftag); use {
		props = &result
	}

	return &MuglResource{
		ID:         idp,
		Type:       res.Type(),
		Properties: props,
	}
}

// SerializeProperties serializes a resource property bag so that it's suitable for serialization.
func SerializeProperties(props PropertyMap, reftag string) (MuglPropertyMap, bool) {
	dst := make(MuglPropertyMap)
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

// DeserializeSnapshot takes a serialized MuGL snapshot data structure and returns its associated snapshot.
func DeserializeSnapshot(ctx *Context, mugl *MuglSnapshot) Snapshot {
	// Determine the reftag to use.
	var reftag string
	if mugl.Refs == nil {
		reftag = DefaultSnapshotReftag
	} else {
		reftag = *mugl.Refs
	}

	// For every serialized resource vertex, create a MuglResource out of it.
	var resources []Resource
	if mugl.Resources != nil {
		// TODO: we need to enumerate resources in the specific order in which they were emitted.
		for m, res := range *mugl.Resources {
			// Deserialize the resources, if they exist.
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
			resources = append(resources, NewResource(id, m, res.Type, props))
		}
	}

	var args core.Args
	if mugl.Args != nil {
		args = *mugl.Args
	}

	return NewSnapshot(ctx, mugl.Package, args, resources)
}

func DeserializeProperties(props MuglPropertyMap, reftag string) PropertyMap {
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
			obj := DeserializeProperties(MuglPropertyMap(w), reftag)
			return NewPropertyObject(obj)
		default:
			contract.Failf("Unrecognized property type: %v", reflect.ValueOf(v))
		}
	}

	return NewPropertyNull()
}
