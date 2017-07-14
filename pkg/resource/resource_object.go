// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"reflect"

	"github.com/golang/glog"

	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/compiler/types"
	"github.com/pulumi/lumi/pkg/compiler/types/predef"
	"github.com/pulumi/lumi/pkg/eval/rt"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// IsResourceObject returns true if the given runtime object is a
func IsResourceObject(obj *rt.Object) bool {
	return obj != nil && predef.IsResourceType(obj.Type())
}

// Object is a live resource object, connected to state that may change due to evaluation.
type Object struct {
	obj *rt.Object // the resource's live object reference.
}

var _ Resource = (*Object)(nil)

// NewObject creates a new resource object out of the runtime object provided.  The context is used to resolve
// dependencies between resources and must contain all references that could be encountered.
func NewObject(obj *rt.Object) *Object {
	contract.Assertf(IsResourceObject(obj), "Expected a resource type")
	return &Object{obj: obj}
}

// NewEmptyObject allocates an empty resource object of a given type.
func NewEmptyObject(t symbols.Type) *Object {
	contract.Assert(predef.IsResourceType(t))
	return &Object{
		obj: rt.NewObject(t, nil, nil, nil),
	}
}

func (r *Object) Obj() *rt.Object   { return r.obj }
func (r *Object) Type() tokens.Type { return r.obj.Type().TypeToken() }

// ID fetches the object's ID.
func (r *Object) ID() ID {
	if idobj := getIDObject(r.Obj()); idobj != nil && idobj.IsString() {
		return ID(idobj.StringValue())
	}
	return ID("")
}

// HasID returns true if the object already has an ID assigned to it.
func (r *Object) HasID() bool {
	return r.ID() != ""
}

// HasComputedID returns true if the object has an ID, but is computed and its value is not known yet.
func (r *Object) HasComputedID() bool {
	idobj := getIDObject(r.Obj())
	return idobj != nil && idobj.IsComputed()
}

// SetID assigns an ID to the target object.  This must only happen once.
func (r *Object) SetID(id ID) {
	prop := r.obj.GetPropertyAddr(IDProperty, true, true)
	contract.Assertf(prop.Obj().IsNull() || prop.Obj().IsComputed(), "Unexpected double set on ID; previous=%v", prop)
	prop.Set(rt.NewStringObject(id.String()))
}

// getIDObject fetches the ID off the target object, dynamically, given its runtime value.
func getIDObject(obj *rt.Object) *rt.Object {
	contract.Assert(IsResourceObject(obj))
	if idprop := obj.GetPropertyAddr(IDProperty, false, false); idprop != nil {
		id := idprop.Obj()
		contract.Assert(id != nil)
		contract.Assert(id.IsString() || id.IsComputed())
		return id
	}
	return nil
}

const (
	// URNProperty is the special Universal Pulumi Name (UPN) property name.
	URNProperty = rt.PropertyKey("upn")
	// URNPropertyKey is the special Universal Pulumi Name (UPN) property name for resource maps.
	URNPropertyKey = PropertyKey("upn")
)

// URN fetches the object's URN.
func (r *Object) URN() URN {
	if urnobj := getURNObject(r.Obj()); urnobj != nil && urnobj.IsString() {
		return URN(urnobj.StringValue())
	}
	return URN("")
}

// HasURN returns true if the object has a URN assigned.
func (r *Object) HasURN() bool {
	return r.URN() != ""
}

// SetURN assignes a URN to the target object.  This must only happen once.
func (r *Object) SetURN(urn URN) {
	prop := r.obj.GetPropertyAddr(URNProperty, true, true)
	contract.Assertf(prop.Obj().IsNull() || prop.Obj().IsComputed(), "Unexpected double set on URN; previous=%v", prop)
	prop.Set(rt.NewStringObject(string(urn)))
}

// getURNObject fetches the URN off the target object, dynamically, given its runtime value.
func getURNObject(obj *rt.Object) *rt.Object {
	contract.Assert(IsResourceObject(obj))
	if urnprop := obj.GetPropertyAddr(URNProperty, false, false); urnprop != nil {
		urn := urnprop.Obj()
		contract.Assert(urn != nil)
		contract.Assert(urn.IsString() || urn.IsComputed())
		return urn
	}
	return nil
}

// Update updates the target object URN, ID, and resource property map.  This mutates the live object connected to this
// resource and also archives the resource object's present state in the form of a state snapshot.
func (r *Object) Update(urn URN, id ID, outputs PropertyMap) *State {
	// First take a snapshot of the properties.
	inputs := r.CopyProperties()

	// Now assign the URN, ID, and copy everything in the property map, overwriting what exists.
	r.SetURN(urn)
	r.SetID(id)
	r.SetProperties(outputs)

	// Finally, return a state snapshot of the underlying object state.
	return NewState(r.Type(), r.URN(), id, inputs, outputs)
}

// CopyProperties creates a property map out of a resource's runtime object.  This is a snapshot and is completely
// disconnected from the object itself, such that any subsequent object updates will not be observed.
func (r *Object) CopyProperties() PropertyMap {
	resobj := r.Obj()
	return copyObject(resobj, resobj)
}

// SetProperties copies from a resource property map to the runtime object, overwriting properties as it goes.
func (r *Object) SetProperties(props PropertyMap) {
	if props != nil {
		setRuntimeProperties(r.Obj(), props)
	}
}

func copyObject(resobj *rt.Object, obj *rt.Object) PropertyMap {
	contract.Assert(obj != nil)
	props := obj.PropertyValues()
	return copyObjectProperties(resobj, props)
}

// CopyObject flattens a single object into a serializable "JSON-like" property value.
func CopyObject(obj *rt.Object) PropertyValue {
	return copyObjectProperty(nil, obj)
}

// copyObjectProperty creates a single property value out of a runtime object.  It returns false if the property could
// not be stored in a property (e.g., it is a function or other unrecognized or unserializable runtime object).
func copyObjectProperty(resobj *rt.Object, obj *rt.Object) PropertyValue {
	t := obj.Type()

	if predef.IsResourceType(t) {
		// Resource references expand to that resource's ID.
		idobj := getIDObject(obj)
		if idobj != nil && idobj.IsString() {
			return NewStringProperty(idobj.StringValue())
		}

		// If an ID hasn't yet been assigned, we must be planning, and so this is a computed property.
		return MakeComputed(NewStringProperty(""))
	}

	// Serialize simple primitive types with their primitive equivalents.
	switch t {
	case types.Null:
		return NewNullProperty()
	case types.Bool:
		return NewBoolProperty(obj.BoolValue())
	case types.Number:
		return NewNumberProperty(obj.NumberValue())
	case types.String:
		return NewStringProperty(obj.StringValue())
	case types.Object, types.Dynamic:
		result := copyObject(nil, obj) // an object literal, clone it
		return NewObjectProperty(result)
	}

	// Serialize arrays, maps, and object instances in the obvious way.
	switch t.(type) {
	case *symbols.ArrayType:
		// Make a new array, clone each element, and return the result.
		var result []PropertyValue
		for _, e := range *obj.ArrayValue() {
			result = append(result, copyObjectProperty(nil, e.Obj()))
		}
		return NewArrayProperty(result)
	case *symbols.MapType:
		// Make a new map, clone each property value, and return the result.
		props := obj.PropertyValues()
		result := copyObjectProperties(nil, props)
		return NewObjectProperty(result)
	case *symbols.Class:
		// Make a new object that contains a deep clone of the source.
		result := copyObject(nil, obj)
		return NewObjectProperty(result)
	}

	// If a computed value, we can propagate an unknown value, but only for certain cases.
	if t.Computed() {
		// See if this is an output property.  An output property is a property that is set directly on the resource
		// object that is computed from precisely a single dependency and no expression.  Otherwise, it is computed.
		var makeProperty func(PropertyValue) PropertyValue
		if isOutputObject(resobj, obj) {
			makeProperty = MakeOutput
		} else {
			makeProperty = MakeComputed
		}

		// Now just wrap the underlying object appropriately.
		elem := t.(*symbols.ComputedType).Element
		switch elem {
		case types.Null:
			return makeProperty(NewNullProperty())
		case types.Bool:
			return makeProperty(NewBoolProperty(false))
		case types.Number:
			return makeProperty(NewNumberProperty(0))
		case types.String:
			return makeProperty(NewStringProperty(""))
		case types.Object, types.Dynamic:
			return makeProperty(NewObjectProperty(make(PropertyMap)))
		}
		switch elem.(type) {
		case *symbols.ArrayType:
			return makeProperty(NewArrayProperty(nil))
		case *symbols.Class:
			return makeProperty(NewObjectProperty(make(PropertyMap)))
		}
	}

	// We can safely skip serializing functions, however, anything else is unexpected at this point.
	_, isfunc := t.(*symbols.FunctionType)
	contract.Assertf(isfunc, "Unrecognized resource property object type '%v' (%v)", t, reflect.TypeOf(t))
	return PropertyValue{}
}

// copyObjectProperties copies a resource's properties.
func copyObjectProperties(resobj *rt.Object, props *rt.PropertyMap) PropertyMap {
	// Walk the object's properties and serialize them in a stable order.
	result := make(PropertyMap)
	for _, k := range props.Stable() {
		result[PropertyKey(k)] = copyObjectProperty(resobj, props.Get(k))
	}
	return result
}

// isOutputObject returns true if the object obj is a computed output property for resource object resobj.
func isOutputObject(resobj *rt.Object, obj *rt.Object) bool {
	if obj.IsComputed() {
		v := obj.ComputedValue()
		return !v.Expr && len(v.Sources) == 1 && v.Sources[0] == resobj
	}
	return false
}

// setRuntimeProperties translates from a resource property map into the equivalent runtime objects, and stores them on
// the given runtime object.
func setRuntimeProperties(obj *rt.Object, props PropertyMap) {
	for k, v := range props {
		prop := obj.GetPropertyAddr(rt.PropertyKey(k), true, true)
		// TODO[pulumi/lumi#260]: we are only setting if IsNull or IsComputed, to avoid certain shortcomings in our
		//     serialization format today.  For example, if a resource ID appears, we must map it back to the runtime
		//     object.  This means some resource outputs won't get reflected accurately.  We will need to fix this.
		pobj := prop.Obj()
		if pobj.IsNull() || isOutputObject(obj, pobj) {
			glog.V(9).Infof("Setting resource object property: %v=%v", k, v)
			val := createRuntimeProperty(v)
			prop.Set(val)
		} else {
			glog.V(9).Infof("Skipping resource object property: %v=%v; existing=%v", k, v, pobj)
		}
	}
}

// createRuntimeProperty translates a property value into a runtime object.
func createRuntimeProperty(v PropertyValue) *rt.Object {
	if v.IsNull() {
		return rt.Null
	} else if v.IsBool() {
		return rt.Bools[v.BoolValue()]
	} else if v.IsNumber() {
		return rt.NewNumberObject(v.NumberValue())
	} else if v.IsString() {
		return rt.NewStringObject(v.StringValue())
	} else if v.IsArray() {
		src := v.ArrayValue()
		arr := make([]*rt.Pointer, len(src))
		for i, elem := range src {
			ve := createRuntimeProperty(elem)
			arr[i] = rt.NewPointer(ve, false, nil, nil)
		}
		return rt.NewArrayObject(types.Dynamic, &arr)
	}

	contract.Assertf(v.IsObject(), "Expected an object, not a computed/output value")
	obj := rt.NewObject(types.Dynamic, nil, nil, nil)
	setRuntimeProperties(obj, v.ObjectValue())
	return obj
}
