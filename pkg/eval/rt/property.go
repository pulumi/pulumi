// Copyright 2017 Pulumi, Inc. All rights reserved.

package rt

import (
	"github.com/pulumi/coconut/pkg/util/contract"
)

type PropertyMap map[PropertyKey]*Pointer // an object's properties.
type PropertyKey string                   // property keys are strings (incl. invalid identifiers for dynamic).
type Initer func(string) (*Object, bool)  // an initialization function returning an object and readonly indicator.

// GetAddr returns a reference to a map's property.  If no entry is found, and `init` is true, the location will be
// auto-initialized to an empty value.  Otherwise, nil is simply returned.
func (props PropertyMap) GetAddr(key PropertyKey, init bool) *Pointer {
	ptr, has := props[key]
	if !has && init {
		ptr = props.InitAddr(key, nil, false)
	}
	return ptr
}

// InitAddr initializes a map's property slot with the given default value, substituting null if that's empty.
func (props PropertyMap) InitAddr(key PropertyKey, obj *Object, readonly bool) *Pointer {
	contract.Assertf(props[key] == nil, "Cannot initialize an existing slot: %v", key)

	// If no object was provided, initialize the slot to null.
	if obj == nil {
		obj = NewNullObject()
	}

	ptr := NewPointer(obj, readonly)
	props[key] = ptr
	return ptr
}
