// Copyright 2016 Marapongo, Inc. All rights reserved.

package resource

import (
	"reflect"

	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/compiler/types"
	"github.com/marapongo/mu/pkg/compiler/types/predef"
	"github.com/marapongo/mu/pkg/eval/rt"
	"github.com/marapongo/mu/pkg/graph"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

// ID is a unique resource identifier; it is managed by the provider and is mostly opaque to Mu.
type ID string

// Type is a resource type identifier.
type Type tokens.Type

// Resource is an instance of a resource with an ID, type, and bag of state.
type Resource interface {
	ID() ID                  // the resource's unique ID, assigned by the resource provider (or blank if uncreated).
	Moniker() Moniker        // the resource's object moniker, a human-friendly, unique name for the resource.
	Type() Type              // the resource's type.
	Properties() PropertyMap // the resource's property map.
}

type PropertyMap map[PropertyKey]PropertyValue

type PropertyKey tokens.Name // the name of a property.

// PropertyValue is the value of a property, limited to a select few types (see below).
type PropertyValue struct {
	V interface{}
}

func NewPropertyBool(v bool) PropertyValue             { return PropertyValue{v} }
func NewPropertyNumber(v float64) PropertyValue        { return PropertyValue{v} }
func NewPropertyString(v string) PropertyValue         { return PropertyValue{v} }
func NewPropertyArray(v []PropertyValue) PropertyValue { return PropertyValue{v} }
func NewPropertyObject(v PropertyMap) PropertyValue    { return PropertyValue{v} }
func NewPropertyResource(v Moniker) PropertyValue      { return PropertyValue{v} }

func (v PropertyValue) BoolValue() bool             { return v.V.(bool) }
func (v PropertyValue) NumberValue() float64        { return v.V.(float64) }
func (v PropertyValue) StringValue() string         { return v.V.(string) }
func (v PropertyValue) ArrayValue() []PropertyValue { return v.V.([]PropertyValue) }
func (v PropertyValue) ObjectValue() PropertyMap    { return v.V.(PropertyMap) }
func (v PropertyValue) ResourceValue() Moniker      { return v.V.(Moniker) }

func (b PropertyValue) IsBool() bool {
	_, is := b.V.(bool)
	return is
}
func (b PropertyValue) IsNumber() bool {
	_, is := b.V.(float64)
	return is
}
func (b PropertyValue) IsString() bool {
	_, is := b.V.(string)
	return is
}
func (b PropertyValue) IsArray() bool {
	_, is := b.V.([]PropertyValue)
	return is
}
func (b PropertyValue) IsObject() bool {
	_, is := b.V.(PropertyMap)
	return is
}
func (b PropertyValue) IsResource() bool {
	_, is := b.V.(Moniker)
	return is
}

func IsResourceType(t symbols.Type) bool   { return types.HasBaseName(t, predef.MuResourceClass) }
func IsResourceVertex(v graph.Vertex) bool { return IsResourceType(v.Obj().Type()) }

type resource struct {
	id         ID          // the resource's unique ID, assigned by the resource provider (or blank if uncreated).
	moniker    Moniker     // the resource's object moniker, a human-friendly, unique name for the resource.
	t          Type        // the resource's type.
	properties PropertyMap // the resource's property map.
}

func (r *resource) ID() ID                  { return r.id }
func (r *resource) Moniker() Moniker        { return r.moniker }
func (r *resource) Type() Type              { return r.t }
func (r *resource) Properties() PropertyMap { return r.properties }

// NewResource creates a new resource object out of the runtime object provided.  The refs map is used to resolve
// dependencies between resources and must contain all references that could be encountered.
func NewResource(obj *rt.Object, mks objectMonikerMap) Resource {
	t := obj.Type()
	contract.Assert(IsResourceType(t))

	// Extract the moniker.  This must already exist.
	m, hasm := mks[obj]
	contract.Assert(hasm)

	// Do a deep copy of the resource properties.  This ensures property serializability.
	props := cloneObject(obj, mks)

	// Finally allocate and return the resource object; note that ID is left blank until the provider assignes one.
	return &resource{
		moniker:    m,
		t:          Type(t.Token()),
		properties: props,
	}
}

// cloneObject creates a property map out of a runtime object.  The result is fully serializable in the sense that it
// can be stored in a JSON or YAML file, serialized over an RPC interface, etc.  In particular, any references to other
// resources are replaced with their moniker equivalents, which the runtime understands.
func cloneObject(obj *rt.Object, mks objectMonikerMap) PropertyMap {
	contract.Assert(obj != nil)
	src := obj.PropertyValues()
	dest := make(PropertyMap)
	for _, k := range rt.StablePropertyKeys(src) {
		// TODO: detect cycles.
		if v, ok := cloneObjectValue(src[k].Obj(), mks); ok {
			dest[PropertyKey(k)] = v
		}
	}
	return dest
}

// cloneObjectValue creates a single property value out of a runtime object.  It returns false if the property could not
// be stored in a property (e.g., it is a function or other unrecognized or unserializable runtime object).
func cloneObjectValue(obj *rt.Object, res objectMonikerMap) (PropertyValue, bool) {
	t := obj.Type()
	if IsResourceType(t) {
		// For resources, simply look up the moniker from the resource map.
		m, hasm := res[obj]
		contract.Assert(hasm)
		return NewPropertyResource(m), true
	}

	switch t {
	case types.Bool:
		return NewPropertyBool(obj.BoolValue()), true
	case types.Number:
		return NewPropertyNumber(obj.NumberValue()), true
	case types.String:
		return NewPropertyString(obj.StringValue()), true
	case types.Object, types.Dynamic:
		obj := cloneObject(obj, res) // an object literal, clone it
		return NewPropertyObject(obj), true
	}

	switch t.(type) {
	case *symbols.ArrayType:
		var result []PropertyValue
		for _, e := range *obj.ArrayValue() {
			if v, ok := cloneObjectValue(e.Obj(), res); ok {
				result = append(result, v)
			}
		}
		return NewPropertyArray(result), true
	case *symbols.Class:
		obj := cloneObject(obj, res) // a class, just deep clone it
		return NewPropertyObject(obj), true
	}

	// TODO: handle symbols.MapType.
	// TODO: it's unclear if we should do something more drastic here.  There will always be unrecognized property
	//     kinds because objects contain things like constructors, methods, etc.  But we may want to ratchet this a bit.
	glog.V(5).Infof("Ignoring object value of type '%v': unrecognized kind %v", t, reflect.TypeOf(t))
	return PropertyValue{}, false
}
