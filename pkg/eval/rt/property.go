// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package rt

import (
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/util/contract"
)

type PropertyKey string // property keys are strings (incl. invalid identifiers for dynamic).

type PropertyMap struct {
	m      map[PropertyKey]*Pointer // an object's properties.
	chrono []PropertyKey            // the ascending chronological order of property creation.
}

// NewPropertyMap returns a fresh property map ready for use.
func NewPropertyMap() *PropertyMap {
	return &PropertyMap{
		m: make(map[PropertyKey]*Pointer),
	}
}

// Len returns the length of the property map.
func (props *PropertyMap) Len() int {
	return len(props.m)
}

// Stable returns the keys for the target map in a stable order.
func (props *PropertyMap) Stable() []PropertyKey {
	return props.chrono // chronological order is already stable, so just return that.
}

// Has checks whether a property exists in the current map.
func (props *PropertyMap) Has(key PropertyKey) bool {
	_, has := props.m[key]
	return has
}

// GetAddr returns a reference to a map's property.  If no entry is found, the return value is nil.
func (props *PropertyMap) GetAddr(key PropertyKey) *Pointer {
	return props.m[key]
}

// Get returns a map's property valye.  If no entry is found, the return value is nil.
func (props *PropertyMap) Get(key PropertyKey) *Object {
	if ptr := props.GetAddr(key); ptr != nil {
		return ptr.Obj()
	}
	return nil
}

// TryGetAddr returns a reference to a map's property.  If no entry is found, the return value is nil, and the second
// boolean return value will be set to false.  Otherwise, the boolean will be true.
func (props *PropertyMap) TryGetAddr(key PropertyKey) (*Pointer, bool) {
	ptr, has := props.m[key]
	return ptr, has
}

// TryGet returns a map's property value.  If no entry is found, the return value is nil, and the second
// boolean return value will be set to false.  Otherwise, the boolean will be true.
func (props *PropertyMap) TryGet(key PropertyKey) (*Object, bool) {
	if ptr, has := props.m[key]; has {
		return ptr.Obj(), true
	}
	return nil, false
}

// GetInitAddr returns a reference to a map's property.  If no entry is found, the location will be
// auto-initialized to an empty value.  Otherwise, nil is simply returned.
func (props *PropertyMap) GetInitAddr(key PropertyKey) *Pointer {
	ptr, has := props.m[key]
	if !has {
		ptr = props.InitAddr(key, nil, false, nil, nil)
	}
	return ptr
}

// InitAddr initializes a map's property slot with the given default value, substituting null if that's empty.
func (props *PropertyMap) InitAddr(key PropertyKey, obj *Object,
	readonly bool, get symbols.Function, set symbols.Function) *Pointer {
	contract.Assertf(props.m[key] == nil, "Cannot initialize an existing slot: %v", key)

	// If no object was provided, initialize the slot to null.
	if obj == nil {
		obj = Null
	}

	ptr := NewPointer(obj, readonly, get, set)
	props.m[key] = ptr
	props.chrono = append(props.chrono, key)
	return ptr
}

// Set sets a map's property value, initializing the slot if required.  If a value already exists, it is overwritten.
func (props *PropertyMap) Set(key PropertyKey, value *Object) {
	ptr := props.GetInitAddr(key)
	ptr.Set(value)
}

// SetFrom sets a map's property value by copying it from another property, initializing the slot if required.  If a
// value already exists, it is overwritten.  Note that this does *not* create an alias; the value is simply copied.
func (props *PropertyMap) SetFrom(key PropertyKey, other *Pointer) {
	ptr := props.GetInitAddr(key)
	value := other.Obj()
	ptr.Set(value)
}
