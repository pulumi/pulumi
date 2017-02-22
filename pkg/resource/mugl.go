// Copyright 2016 Marapongo, Inc. All rights reserved.

package resource

import (
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

// MuglSnapshot is a serializable, flattened MuGL graph structure, specifically for snapshots.   It is very similar to
// the actual Snapshot interface, except that it flattens and rearranges a few data structures for serializability.
type MuglSnapshot struct {
	Package  tokens.PackageName `json:"package"`            // the package which created this graph.
	Args     *core.Args         `json:"args,omitempty"`     // the blueprint args for graph creation.
	Refs     *string            `json:"refs,omitempty"`     // the ref alias, if any (`#ref` by default).
	Vertices *MuglResourceMap   `json:"vertices,omitempty"` // a map of monikers to resource vertices.
}

// SnapshotRefTag is the default ref tag for intra-graph edges.
const SnapshotRefTag = "#ref"

// MuglResourceMap is a map of object moniker to the resource vertex for that moniker.
type MuglResourceMap map[Moniker]*MuglResource

// MuglResource is a serializable vertex within a MuGL graph, specifically for resource snapshots.
type MuglResource struct {
	ID         *ID              `json:"id,omitempty"`         // the provider ID for this resource, if any.
	Type       tokens.Type      `json:"type"`                 // this resource's full type token.
	Properties *MuglPropertyMap `json:"properties,omitempty"` // an untyped bag of properties.
}

// MuglPropertyMap is a property map from resource key to the underlying property value.
type MuglPropertyMap map[PropertyKey]interface{}

// SerializeSnapshot turns a snapshot into a MuGL data structure suitable for serialization.
func SerializeSnapshot(snap Snapshot, reftag string) *MuglSnapshot {
	contract.Assert(snap != nil)

	// Set the ref to the default `#ref` if empty.  Only include it in the serialized output if non-default.
	var refp *string
	if reftag == "" {
		reftag = SnapshotRefTag
	} else {
		refp = &reftag
	}

	// Serialize all vertices and only include a vertex section if non-empty.
	var vertsp *MuglResourceMap
	verts := make(MuglResourceMap)
	for _, res := range snap.Topsort() {
		m := res.Moniker()
		contract.Assertf(string(m) != "", "Unexpected empty resource moniker")
		contract.Assertf(verts[m] == nil, "Unexpected duplicate resource moniker '%v'", m)
		verts[m] = SerializeResource(res, reftag)
	}
	if len(verts) > 0 {
		vertsp = &verts
	}

	// Only include the arguments in the output if non-emtpy.
	var argsp *core.Args
	if args := snap.Args(); len(args) > 0 {
		argsp = &args
	}

	return &MuglSnapshot{
		Package:  snap.Pkg(), // TODO: eventually, this should carry version metadata too.
		Args:     argsp,
		Refs:     refp,
		Vertices: vertsp,
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
	srcprops := res.Properties()
	dstprops := make(MuglPropertyMap)
	for _, key := range StablePropertyKeys(srcprops) {
		if v, use := SerializeProperty(srcprops[key], reftag); use {
			dstprops[key] = v
		}
	}
	if len(dstprops) > 0 {
		props = &dstprops
	}

	return &MuglResource{
		ID:         idp,
		Type:       res.Type(),
		Properties: props,
	}
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
		src := prop.ObjectValue()
		dst := make(map[PropertyKey]interface{})
		for _, k := range StablePropertyKeys(src) {
			if v, use := SerializeProperty(src[k], reftag); use {
				dst[k] = v
			}
		}
		if len(dst) > 0 {
			return dst, true
		}
		return nil, false
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
func DeserializeSnapshotMugl(mugl *MuglSnapshot) Snapshot {
	contract.Failf("MuGL deserialization not yet implemented")
	return nil
}
