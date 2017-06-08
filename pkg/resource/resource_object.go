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
	"reflect"

	"github.com/golang/glog"

	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/compiler/types"
	"github.com/pulumi/lumi/pkg/compiler/types/predef"
	"github.com/pulumi/lumi/pkg/eval/rt"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

type objectResource struct {
	ctx     *Context    // the resource context this object is associated with.
	id      ID          // the resource's unique ID, assigned by the resource provider (or blank if uncreated).
	urn     URN         // the resource's object urn, a human-friendly, unique name for the resource.
	obj     *rt.Object  // the resource's live object reference.
	outputs PropertyMap // the resource's output properties (as specified by the resource provider).
}

// NewObjectResource creates a new resource object out of the runtime object provided.  The context is used to resolve
// dependencies between resources and must contain all references that could be encountered.
func NewObjectResource(ctx *Context, obj *rt.Object) Resource {
	contract.Assertf(predef.IsResourceType(obj.Type()), "Expected a resource type")
	return &objectResource{
		ctx:     ctx,
		obj:     obj,
		outputs: make(PropertyMap),
	}
}

func (r *objectResource) ID() ID               { return r.id }
func (r *objectResource) URN() URN             { return r.urn }
func (r *objectResource) Type() tokens.Type    { return r.obj.Type().TypeToken() }
func (r *objectResource) Inputs() PropertyMap  { return cloneResource(r.ctx, r.obj) }
func (r *objectResource) Outputs() PropertyMap { return r.outputs }

func (r *objectResource) SetID(id ID) {
	contract.Requiref(!HasID(r), "id", "empty")
	glog.V(9).Infof("Assigning ID=%v to resource w/ URN=%v", id, r.urn)
	r.id = id
}

func (r *objectResource) SetURN(m URN) {
	contract.Requiref(!HasURN(r), "urn", "empty")
	r.urn = m
}

// cloneResource creates a property map out of a resource's runtime object.
func cloneResource(ctx *Context, resobj *rt.Object) PropertyMap {
	return cloneObject(ctx, resobj, resobj)
}

// cloneObject creates a property map out of a runtime object.  The result is fully serializable in the sense that it
// can be stored in a JSON or YAML file, serialized over an RPC interface, etc.  In particular, any references to other
// resources are replaced with their urn equivalents, which the runtime understands.
func cloneObject(ctx *Context, resobj *rt.Object, obj *rt.Object) PropertyMap {
	contract.Assert(obj != nil)
	props := obj.PropertyValues()
	return cloneObjectProperties(ctx, resobj, props)
}

// cloneObjectProperty creates a single property value out of a runtime object.  It returns false if the property could
// not be stored in a property (e.g., it is a function or other unrecognized or unserializable runtime object).
func cloneObjectProperty(ctx *Context, resobj *rt.Object, obj *rt.Object) (PropertyValue, bool) {
	t := obj.Type()

	// Serialize resource references as URNs.
	if predef.IsResourceType(t) {
		// For resources, simply look up the urn from the resource map.
		urn, hasm := ctx.ObjURN[obj]
		contract.Assertf(hasm, "Missing object reference for %v; possible out of order dependency walk", obj)
		return NewResourceProperty(urn), true
	}

	// Serialize simple primitive types with their primitive equivalents.
	switch t {
	case types.Null:
		return NewNullProperty(), true
	case types.Bool:
		return NewBoolProperty(obj.BoolValue()), true
	case types.Number:
		return NewNumberProperty(obj.NumberValue()), true
	case types.String:
		return NewStringProperty(obj.StringValue()), true
	case types.Object, types.Dynamic:
		result := cloneObject(ctx, resobj, obj) // an object literal, clone it
		return NewObjectProperty(result), true
	}

	// Serialize arrays, maps, and object instances in the obvious way.
	switch t.(type) {
	case *symbols.ArrayType:
		// Make a new array, clone each element, and return the result.
		var result []PropertyValue
		for _, e := range *obj.ArrayValue() {
			if v, ok := cloneObjectProperty(ctx, obj, e.Obj()); ok {
				result = append(result, v)
			}
		}
		return NewArrayProperty(result), true
	case *symbols.MapType:
		// Make a new map, clone each property value, and return the result.
		props := obj.PropertyValues()
		result := cloneObjectProperties(ctx, resobj, props)
		return NewObjectProperty(result), true
	case *symbols.Class:
		// Make a new object that contains a deep clone of the source.
		result := cloneObject(ctx, resobj, obj)
		return NewObjectProperty(result), true
	}

	// If a computed value, we can propagate an unknown value, but only for certain cases.
	if t.Computed() {
		// If this is an output property, then this property will turn into an output.  Otherwise, it will be marked
		// completed.  An output property is permitted in more places by virtue of the fact that it is expected not to
		// exist during resource create operations, whereas all computed properties should have been resolved by then.
		comp := obj.ComputedValue()
		outprop := (!comp.Expr && len(comp.Sources) == 1 && comp.Sources[0] == resobj)
		var makeProperty func(PropertyValue) PropertyValue
		if outprop {
			// For output properties, we need not track any URNs.
			makeProperty = func(v PropertyValue) PropertyValue {
				return MakeOutput(v)
			}
		} else {
			// For all other properties, we need to look up and store the URNs for all resource dependencies.
			var urns []URN
			for _, src := range comp.Sources {
				// For all inter-resource references, materialize a URN.  Skip this for intra-resource references!
				if src != resobj {
					urn, hasm := ctx.ObjURN[src]
					contract.Assertf(hasm,
						"Missing computed reference from %v to %v; possible out of order dependency walk", resobj, src)
					urns = append(urns, urn)
				}
			}
			makeProperty = func(v PropertyValue) PropertyValue {
				return MakeComputed(v, urns)
			}
		}

		future := t.(*symbols.ComputedType).Element
		switch future {
		case types.Null:
			return makeProperty(NewNullProperty()), true
		case types.Bool:
			return makeProperty(NewBoolProperty(false)), true
		case types.Number:
			return makeProperty(NewNumberProperty(0)), true
		case types.String:
			return makeProperty(NewStringProperty("")), true
		case types.Object, types.Dynamic:
			return makeProperty(NewObjectProperty(make(PropertyMap))), true
		}
		switch future.(type) {
		case *symbols.ArrayType:
			return makeProperty(NewArrayProperty(nil)), true
		case *symbols.Class:
			return makeProperty(NewObjectProperty(make(PropertyMap))), true
		}
	}

	// We can safely skip serializing functions, however, anything else is unexpected at this point.
	_, isfunc := t.(*symbols.FunctionType)
	contract.Assertf(isfunc, "Unrecognized resource property object type '%v' (%v)", t, reflect.TypeOf(t))
	return PropertyValue{}, false
}

// cloneObjectProperties copies a resource's properties.
func cloneObjectProperties(ctx *Context, resobj *rt.Object, props *rt.PropertyMap) PropertyMap {
	// Walk the object's properties and serialize them in a stable order.
	result := make(PropertyMap)
	for _, k := range props.Stable() {
		if v, ok := cloneObjectProperty(ctx, resobj, props.Get(k)); ok {
			result[PropertyKey(k)] = v
		}
	}
	return result
}
