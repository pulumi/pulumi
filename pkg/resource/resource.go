// Copyright 2017 Pulumi, Inc. All rights reserved.

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
	HasID() bool                     // returns true if the resource has been assigned an ID.
	SetID(id ID)                     // assignes an ID to this resource, for those under creation.
	SetPropertiesFrom(m PropertyMap) // copies properties from the given map to the resource.
	HasURN() bool                    // returns true if the resource has been assigned URN.
	SetURN(m URN)                    // assignes a URN to this resource, for those under creation.
}

// State is returned when an error has occurred during a resource provider operation.  It indicates whether the
// operation could be rolled back cleanly (OK).  If not, it means the resource was left in an indeterminate state.
type State int

const (
	StateOK State = iota
	StateUnknown
)

func IsResourceType(t symbols.Type) bool              { return types.HasBaseName(t, predef.LumiStdlibResourceClass) }
func IsResourceVertex(v *heapstate.ObjectVertex) bool { return IsResourceType(v.Obj().Type()) }

type resource struct {
	id         ID          // the resource's unique ID, assigned by the resource provider (or blank if uncreated).
	urn        URN         // the resource's object urn, a human-friendly, unique name for the resource.
	t          tokens.Type // the resource's type.
	properties PropertyMap // the resource's property map.
}

func (r *resource) ID() ID                  { return r.id }
func (r *resource) URN() URN                { return r.urn }
func (r *resource) Type() tokens.Type       { return r.t }
func (r *resource) Properties() PropertyMap { return r.properties }

func (r *resource) HasID() bool { return (string(r.id) != "") }
func (r *resource) SetID(id ID) {
	contract.Requiref(!r.HasID(), "id", "empty")
	r.id = id
}

func (r *resource) SetPropertiesFrom(m PropertyMap) {
	for k, v := range m {
		r.properties[k] = v
	}
}

func (r *resource) HasURN() bool { return (string(r.urn) != "") }
func (r *resource) SetURN(m URN) {
	contract.Requiref(!r.HasURN(), "urn", "empty")
	r.urn = m
}

// NewResource creates a new resource from the information provided.
func NewResource(id ID, urn URN, t tokens.Type, properties PropertyMap) Resource {
	return &resource{
		id:         id,
		urn:        urn,
		t:          t,
		properties: properties,
	}
}

// NewObjectResource creates a new resource object out of the runtime object provided.  The context is used to resolve
// dependencies between resources and must contain all references that could be encountered.
func NewObjectResource(ctx *Context, obj *rt.Object) Resource {
	t := obj.Type()
	contract.Assert(IsResourceType(t))

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

// cloneObject creates a property map out of a runtime object.  The result is fully serializable in the sense that it
// can be stored in a JSON or YAML file, serialized over an RPC interface, etc.  In particular, any references to other
// resources are replaced with their urn equivalents, which the runtime understands.
func cloneObject(ctx *Context, obj *rt.Object) PropertyMap {
	contract.Assert(obj != nil)
	src := obj.PropertyValues()
	dest := make(PropertyMap)
	for _, k := range src.Stable() {
		// TODO: detect cycles.
		obj := src.Get(k)
		if v, ok := cloneObjectValue(ctx, obj); ok {
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
		// For resources, simply look up the urn from the resource map.
		urn, hasm := ctx.ObjURN[obj]
		contract.Assertf(hasm, "Missing object reference; possible out of order dependency walk")
		return NewPropertyResource(urn), true
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
