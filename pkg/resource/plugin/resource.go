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

package plugin

import (
	"reflect"

	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/compiler/types"
	"github.com/pulumi/lumi/pkg/compiler/types/predef"
	"github.com/pulumi/lumi/pkg/eval/rt"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// ResourceProperties creates a property map out of a resource's runtime object.
func ResourceProperties(ctx *Context, obj resource.Object) resource.PropertyMap {
	resobj := obj.Obj()
	return cloneObject(ctx, resobj, resobj)
}

// cloneObject creates a property map out of a runtime object.  The result is fully serializable in the sense that it
// can be stored in a JSON or YAML file, serialized over an RPC interface, etc.  In particular, any references to other
// resources are replaced with their urn equivalents, which the runtime understands.
func cloneObject(ctx *Context, resobj *rt.Object, obj *rt.Object) resource.PropertyMap {
	contract.Assert(obj != nil)
	props := obj.PropertyValues()
	return cloneObjectProperties(ctx, resobj, props)
}

// cloneObjectProperty creates a single property value out of a runtime object.  It returns false if the property could
// not be stored in a property (e.g., it is a function or other unrecognized or unserializable runtime object).
func cloneObjectProperty(ctx *Context, resobj *rt.Object, obj *rt.Object) (resource.PropertyValue, bool) {
	t := obj.Type()

	// Serialize resource references as references to the object's special ID property.
	if predef.IsResourceType(t) {
		_, hasm := ctx.ObjURN[obj]
		contract.Assertf(hasm, "Missing object reference for %v; possible out of order dependency walk", obj)
		idprop := obj.GetPropertyAddr(rt.PropertyKey("id"), false, false)
		contract.Assert(idprop != nil)
		idvalue := idprop.Obj()
		contract.Assert(idvalue != nil)
		return cloneObjectProperty(ctx, resobj, idvalue)
	}

	// Serialize simple primitive types with their primitive equivalents.
	switch t {
	case types.Null:
		return resource.NewNullProperty(), true
	case types.Bool:
		return resource.NewBoolProperty(obj.BoolValue()), true
	case types.Number:
		return resource.NewNumberProperty(obj.NumberValue()), true
	case types.String:
		return resource.NewStringProperty(obj.StringValue()), true
	case types.Object, types.Dynamic:
		result := cloneObject(ctx, resobj, obj) // an object literal, clone it
		return resource.NewObjectProperty(result), true
	}

	// Serialize arrays, maps, and object instances in the obvious way.
	switch t.(type) {
	case *symbols.ArrayType:
		// Make a new array, clone each element, and return the result.
		var result []resource.PropertyValue
		for _, e := range *obj.ArrayValue() {
			if v, ok := cloneObjectProperty(ctx, obj, e.Obj()); ok {
				result = append(result, v)
			}
		}
		return resource.NewArrayProperty(result), true
	case *symbols.MapType:
		// Make a new map, clone each property value, and return the result.
		props := obj.PropertyValues()
		result := cloneObjectProperties(ctx, resobj, props)
		return resource.NewObjectProperty(result), true
	case *symbols.Class:
		// Make a new object that contains a deep clone of the source.
		result := cloneObject(ctx, resobj, obj)
		return resource.NewObjectProperty(result), true
	}

	// If a computed value, we can propagate an unknown value, but only for certain cases.
	if t.Computed() {
		// If this is an output property, then this property will turn into an output.  Otherwise, it will be marked
		// completed.  An output property is permitted in more places by virtue of the fact that it is expected not to
		// exist during resource create operations, whereas all computed properties should have been resolved by then.
		comp := obj.ComputedValue()
		outprop := (!comp.Expr && len(comp.Sources) == 1 && comp.Sources[0] == resobj)
		var makeProperty func(resource.PropertyValue) resource.PropertyValue
		if outprop {
			// For output properties, we need not track any URNs.
			makeProperty = func(v resource.PropertyValue) resource.PropertyValue {
				return resource.MakeOutput(v)
			}
		} else {
			// For all other properties, we need to look up and store the URNs for all resource dependencies.
			var urns []resource.URN
			for _, src := range comp.Sources {
				// For all inter-resource references, materialize a URN.  Skip this for intra-resource references!
				if src != resobj {
					urn, hasm := ctx.ObjURN[src]
					contract.Assertf(hasm,
						"Missing computed reference from %v to %v; possible out of order dependency walk", resobj, src)
					urns = append(urns, urn)
				}
			}
			makeProperty = func(v resource.PropertyValue) resource.PropertyValue {
				return resource.MakeComputed(v, urns)
			}
		}

		future := t.(*symbols.ComputedType).Element
		switch future {
		case types.Null:
			return makeProperty(resource.NewNullProperty()), true
		case types.Bool:
			return makeProperty(resource.NewBoolProperty(false)), true
		case types.Number:
			return makeProperty(resource.NewNumberProperty(0)), true
		case types.String:
			return makeProperty(resource.NewStringProperty("")), true
		case types.Object, types.Dynamic:
			return makeProperty(resource.NewObjectProperty(make(resource.PropertyMap))), true
		}
		switch future.(type) {
		case *symbols.ArrayType:
			return makeProperty(resource.NewArrayProperty(nil)), true
		case *symbols.Class:
			return makeProperty(resource.NewObjectProperty(make(resource.PropertyMap))), true
		}
	}

	// We can safely skip serializing functions, however, anything else is unexpected at this point.
	_, isfunc := t.(*symbols.FunctionType)
	contract.Assertf(isfunc, "Unrecognized resource property object type '%v' (%v)", t, reflect.TypeOf(t))
	return resource.PropertyValue{}, false
}

// cloneObjectProperties copies a resource's properties.
func cloneObjectProperties(ctx *Context, resobj *rt.Object, props *rt.PropertyMap) resource.PropertyMap {
	// Walk the object's properties and serialize them in a stable order.
	result := make(resource.PropertyMap)
	for _, k := range props.Stable() {
		if v, ok := cloneObjectProperty(ctx, resobj, props.Get(k)); ok {
			result[resource.PropertyKey(k)] = v
		}
	}
	return result
}
