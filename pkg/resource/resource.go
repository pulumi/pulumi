// Copyright 2016 Marapongo, Inc. All rights reserved.

package resource

import (
	"reflect"

	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/compiler/types"
	"github.com/marapongo/mu/pkg/compiler/types/predef"
	"github.com/marapongo/mu/pkg/eval/heapstate"
	"github.com/marapongo/mu/pkg/eval/rt"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

// ID is a unique resource identifier; it is managed by the provider and is mostly opaque to Mu.
type ID string

// Resource is an instance of a resource with an ID, type, and bag of state.
type Resource interface {
	ID() ID                  // the resource's unique ID, assigned by the resource provider (or blank if uncreated).
	Moniker() Moniker        // the resource's object moniker, a human-friendly, unique name for the resource.
	Type() tokens.Type       // the resource's type.
	Properties() PropertyMap // the resource's property map.
	HasID() bool             // returns true if the resource has been assigned an ID.
	SetID(id ID)             // assignes an ID to this resource, for those under creation.
	HasMoniker() bool        // returns true if the resource has been assigned  moniker.
	SetMoniker(m Moniker)    // assignes a moniker to this resource, for those under creation.
}

// ResourceState is returned when an error has occurred during a resource provider operation.  It indicates whether the
// operation could be rolled back cleanly (OK).  If not, it means the resource was left in an indeterminate state.
type ResourceState int

const (
	StateOK ResourceState = iota
	StateUnknown
)

func IsResourceType(t symbols.Type) bool              { return types.HasBaseName(t, predef.MuResourceClass) }
func IsResourceVertex(v *heapstate.ObjectVertex) bool { return IsResourceType(v.Obj().Type()) }

type resource struct {
	id         ID          // the resource's unique ID, assigned by the resource provider (or blank if uncreated).
	moniker    Moniker     // the resource's object moniker, a human-friendly, unique name for the resource.
	t          tokens.Type // the resource's type.
	properties PropertyMap // the resource's property map.
}

func (r *resource) ID() ID                  { return r.id }
func (r *resource) Moniker() Moniker        { return r.moniker }
func (r *resource) Type() tokens.Type       { return r.t }
func (r *resource) Properties() PropertyMap { return r.properties }

func (r *resource) HasID() bool { return (string(r.id) != "") }
func (r *resource) SetID(id ID) {
	contract.Requiref(!r.HasID(), "id", "empty")
	r.id = id
}

func (r *resource) HasMoniker() bool { return (string(r.moniker) != "") }
func (r *resource) SetMoniker(m Moniker) {
	contract.Requiref(!r.HasMoniker(), "moniker", "empty")
	r.moniker = m
}

// NewResource creates a new resource from the information provided.
func NewResource(id ID, moniker Moniker, t tokens.Type, properties PropertyMap) Resource {
	return &resource{
		id:         id,
		moniker:    moniker,
		t:          t,
		properties: properties,
	}
}

// NewObjectResource creates a new resource object out of the runtime object provided.  The context is used to resolve
// dependencies between resources and must contain all references that could be encountered.
func NewObjectResource(ctx *Context, obj *rt.Object) Resource {
	t := obj.Type()
	contract.Assert(IsResourceType(t))

	// Extract the moniker.  This must already exist.
	m, hasm := ctx.ObjMks[obj]
	contract.Assertf(!hasm, "Object already assigned a moniker '%v'; double allocation detected", m)

	// Do a deep copy of the resource properties.  This ensures property serializability.
	props := cloneObject(ctx, obj)

	// Finally allocate and return the resource object; note that ID is left blank until the provider assignes one.
	return &resource{
		t:          t.TypeToken(),
		properties: props,
	}
}

// cloneObject creates a property map out of a runtime object.  The result is fully serializable in the sense that it
// can be stored in a JSON or YAML file, serialized over an RPC interface, etc.  In particular, any references to other
// resources are replaced with their moniker equivalents, which the runtime understands.
func cloneObject(ctx *Context, obj *rt.Object) PropertyMap {
	contract.Assert(obj != nil)
	src := obj.PropertyValues()
	dest := make(PropertyMap)
	for _, k := range rt.StablePropertyKeys(src) {
		// TODO: detect cycles.
		if v, ok := cloneObjectValue(ctx, src[k].Obj()); ok {
			dest[PropertyKey(k)] = v
		}
	}
	return dest
}

// cloneObjectValue creates a single property value out of a runtime object.  It returns false if the property could not
// be stored in a property (e.g., it is a function or other unrecognized or unserializable runtime object).
func cloneObjectValue(ctx *Context, obj *rt.Object) (PropertyValue, bool) {
	t := obj.Type()
	if IsResourceType(t) {
		// For resources, simply look up the moniker from the resource map.
		m, hasm := ctx.ObjMks[obj]
		contract.Assertf(hasm, "Missing object reference; possible out of order dependency walk")
		return NewPropertyResource(m), true
	}

	switch t {
	case types.Null:
		return NewPropertyNull(), true
	case types.Bool:
		return NewPropertyBool(obj.BoolValue()), true
	case types.Number:
		return NewPropertyNumber(obj.NumberValue()), true
	case types.String:
		return NewPropertyString(obj.StringValue()), true
	case types.Object, types.Dynamic:
		obj := cloneObject(ctx, obj) // an object literal, clone it
		return NewPropertyObject(obj), true
	}

	switch t.(type) {
	case *symbols.ArrayType:
		var result []PropertyValue
		for _, e := range *obj.ArrayValue() {
			if v, ok := cloneObjectValue(ctx, e.Obj()); ok {
				result = append(result, v)
			}
		}
		return NewPropertyArray(result), true
	case *symbols.Class:
		obj := cloneObject(ctx, obj) // a class, just deep clone it
		return NewPropertyObject(obj), true
	}

	// TODO: handle symbols.MapType.
	// TODO: it's unclear if we should do something more drastic here.  There will always be unrecognized property
	//     kinds because objects contain things like constructors, methods, etc.  But we may want to ratchet this a bit.
	glog.V(5).Infof("Ignoring object value of type '%v': unrecognized kind %v", t, reflect.TypeOf(t))
	return PropertyValue{}, false
}
