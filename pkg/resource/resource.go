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
	"crypto/rand"
	"encoding/hex"
	"reflect"

	"github.com/golang/glog"

	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/compiler/types"
	"github.com/pulumi/lumi/pkg/compiler/types/predef"
	"github.com/pulumi/lumi/pkg/eval/heapstate"
	"github.com/pulumi/lumi/pkg/eval/rt"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// ID is a unique resource identifier; it is managed by the provider and is mostly opaque to Lumi.
type ID string

func MaybeID(s *string) *ID {
	var ret *ID
	if s != nil {
		id := ID(*s)
		ret = &id
	}
	return ret
}

func (id ID) String() string { return string(id) }
func (id *ID) StringPtr() *string {
	if id == nil {
		return nil
	}
	ids := (*id).String()
	return &ids
}
func IDStrings(ids []ID) []string {
	ss := make([]string, len(ids))
	for i, id := range ids {
		ss[i] = id.String()
	}
	return ss
}

// Resource is an instance of a resource with an ID, type, and bag of state.
type Resource interface {
	ID() ID                          // the resource's unique ID assigned by the provider (or blank if uncreated).
	URN() URN                        // the resource's object urn, a human-friendly, unique name for the resource.
	Type() tokens.Type               // the resource's type.
	Properties() PropertyMap         // the resource's property map.
	Outputs() PropertySet            // the set of properties that were set via outputs from the provider.
	ClearOutputs()                   // clears the outputs set in preparation for an operation that marks them.
	MarkOutput(k PropertyKey)        // marks a property as an output from the provider.
	HasID() bool                     // returns true if the resource has been assigned an ID.
	SetID(id ID)                     // assignes an ID to this resource, for those under creation.
	HasURN() bool                    // returns true if the resource has been assigned URN.
	SetURN(m URN)                    // assignes a URN to this resource, for those under creation.
	ShallowClone() Resource          // make a shallow clone of the resource.
	PropagateOutputs(other Resource) // copy any required output properties onto the target object.
}

// State is returned when an error has occurred during a resource provider operation.  It indicates whether the
// operation could be rolled back cleanly (OK).  If not, it means the resource was left in an indeterminate state.
type State int

const (
	StateOK State = iota
	StateUnknown
)

// IsResourceVertex returns true if the heap graph vertex has an object whose type is the standard resource class.
func IsResourceVertex(v *heapstate.ObjectVertex) bool {
	return predef.IsResourceType(v.Obj().Type())
}

type resource struct {
	id         ID          // the resource's unique ID, assigned by the resource provider (or blank if uncreated).
	urn        URN         // the resource's object urn, a human-friendly, unique name for the resource.
	t          tokens.Type // the resource's type.
	properties PropertyMap // the resource's property map.
	outs       PropertySet // the set of properties that were set via outputs from the provider.
}

func (r *resource) ID() ID                  { return r.id }
func (r *resource) URN() URN                { return r.urn }
func (r *resource) Type() tokens.Type       { return r.t }
func (r *resource) Properties() PropertyMap { return r.properties }
func (r *resource) Outputs() PropertySet    { return r.outs }

func (r *resource) ClearOutputs() {
	r.outs = nil
}
func (r *resource) MarkOutput(k PropertyKey) {
	if r.outs == nil {
		r.outs = make(PropertySet)
	}
	r.outs[k] = true
}

func (r *resource) HasID() bool { return (string(r.id) != "") }
func (r *resource) SetID(id ID) {
	contract.Requiref(!r.HasID(), "id", "empty")
	glog.V(9).Infof("Assigning ID=%v to resource w/ URN=%v", id, r.urn)
	r.id = id
}

func (r *resource) HasURN() bool { return (string(r.urn) != "") }
func (r *resource) SetURN(m URN) {
	contract.Requiref(!r.HasURN(), "urn", "empty")
	r.urn = m
}

// ShallowClone clones a resource object so that any modifications to it are not reflected in the original.  Note that
// the property map is only shallowly cloned so any mutations deep within it may get reflected in the original.
func (r *resource) ShallowClone() Resource {
	return &resource{
		id:         r.id,
		urn:        r.urn,
		t:          r.t,
		properties: r.properties.ShallowClone(),
		outs:       r.outs.ShallowClone(),
	}
}

// PropagateOutputs copies any required output properties onto the target object.
func (r *resource) PropagateOutputs(other Resource) {
	rs := r.Properties()
	others := other.Properties()
	for k := range r.Outputs() {
		if others.NeedsValue(k) {
			others[k] = rs[k]
			other.MarkOutput(k)
		}
	}
}

// NewResource creates a new resource from the information provided.
func NewResource(id ID, urn URN, t tokens.Type, properties PropertyMap) Resource {
	return &resource{
		id:         id,
		urn:        urn,
		t:          t,
		properties: properties,
		outs:       nil, // lazily allocated when provider operations occur.
	}
}

// NewObjectResource creates a new resource object out of the runtime object provided.  The context is used to resolve
// dependencies between resources and must contain all references that could be encountered.
func NewObjectResource(ctx *Context, obj *rt.Object) Resource {
	t := obj.Type()
	contract.Assert(predef.IsResourceType(t))

	// Extract the urn.  This must already exist.
	urn, hasm := ctx.ObjURN[obj]
	contract.Assertf(!hasm, "Object already assigned a urn '%v'; double allocation detected", urn)

	// Do a deep copy of the resource properties.  This ensures property serializability.
	props := cloneObject(ctx, obj)

	// Finally allocate and return the resource object; note that ID is left blank until the provider assignes one.
	return &resource{
		t:          t.TypeToken(),
		properties: props,
	}
}

const resourceOutPropertyToken = tokens.Token("lumi:resource:out")

// cloneObject creates a property map out of a runtime object.  The result is fully serializable in the sense that it
// can be stored in a JSON or YAML file, serialized over an RPC interface, etc.  In particular, any references to other
// resources are replaced with their urn equivalents, which the runtime understands.
func cloneObject(ctx *Context, obj *rt.Object) PropertyMap {
	contract.Assert(obj != nil)

	// First accumulate a list of properties that are known to be outputs.
	var outs map[PropertyKey]bool
	t := obj.Type()
	for t != nil {
		for name, member := range t.TypeMembers() {
			if prop, isprop := member.(*symbols.ClassProperty); isprop {
				if attrs := prop.Node.Attributes; attrs != nil {
					for _, attr := range *attrs {
						if attr.Decorator.Tok == resourceOutPropertyToken {
							if outs == nil {
								outs = make(map[PropertyKey]bool)
							}
							outs[PropertyKey(name)] = true
							break
						}
					}
				}
			}
		}
		t = t.Base()
	}

	// Next walk the object's properties and serialize them in a stable order.
	src := obj.PropertyValues()
	dest := make(PropertyMap)
	for _, k := range src.Stable() {
		// TODO: detect cycles.
		obj := src.Get(k)
		pky := PropertyKey(k)
		var out bool
		if outs != nil {
			out = outs[pky]
		}
		if v, ok := cloneObjectProperty(ctx, obj, out); ok {
			dest[pky] = v
		}
	}
	return dest
}

// cloneObjectProperty creates a single property value out of a runtime object.  It returns false if the property could
// not be stored in a property (e.g., it is a function or other unrecognized or unserializable runtime object).
func cloneObjectProperty(ctx *Context, obj *rt.Object, out bool) (PropertyValue, bool) {
	t := obj.Type()

	// Serialize resource references as URNs.
	if predef.IsResourceType(t) {
		// For resources, simply look up the urn from the resource map.
		urn, hasm := ctx.ObjURN[obj]
		contract.Assertf(hasm, "Missing object reference; possible out of order dependency walk")
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
		obj := cloneObject(ctx, obj) // an object literal, clone it
		return NewObjectProperty(obj), true
	}

	// Serialize arrays, maps, and object instances in the obvious way.
	// TODO: handle symbols.MapType.
	switch t.(type) {
	case *symbols.ArrayType:
		var result []PropertyValue
		for _, e := range *obj.ArrayValue() {
			if v, ok := cloneObjectProperty(ctx, e.Obj(), false); ok {
				result = append(result, v)
			}
		}
		return NewArrayProperty(result), true
	case *symbols.Class:
		obj := cloneObject(ctx, obj) // a class, just deep clone it
		return NewObjectProperty(obj), true
	}

	// If a latent value, we can propagate an unknown value, but only for certain cases.
	if t.Latent() {
		// If this is an output property, then this property will turn into an output.  Otherwise, it will be marked
		// completed.  An output property is permitted in more places by virtue of the fact that it is expected not to
		// exist during resource create operations, whereas all computed properties should have been resolved by then.
		var makeProperty func(PropertyValue) PropertyValue
		if out {
			makeProperty = MakeOutput
		} else {
			makeProperty = MakeComputed
		}

		future := t.(*symbols.LatentType).Element
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

// NewUniqueHex generates a new "random" hex string for use by resource providers.  It has the given optional prefix and
// the total length is capped to the maxlen.  Note that capping to maxlen necessarily increases the risk of collisions.
func NewUniqueHex(prefix string, randlen, maxlen int) string {
	bs := make([]byte, randlen)
	n, err := rand.Read(bs)
	contract.Assert(err == nil)
	contract.Assert(n == len(bs))

	str := prefix + hex.EncodeToString(bs)
	if len(str) > maxlen {
		str = str[:maxlen]
	}
	return str
}

// NewUniqueHexID generates a new "random" hex ID for use by resource providers.  It has the given optional prefix and
// the total length is capped to the maxlen.  Note that capping to maxlen necessarily increases the risk of collisions.
func NewUniqueHexID(prefix string, randlen, maxlen int) ID {
	return ID(NewUniqueHex(prefix, randlen, maxlen))
}
