// Copyright 2016 Marapongo, Inc. All rights reserved.

package rt

import (
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

type Properties map[PropertyKey]*Pointer // an object's properties.
type PropertyKey string                  // property keys are strings (incl. invalid identifiers for dynamic).
type Initer func(string) (*Object, bool) // an initialization function returning an object and readonly indicator.

// GetAddr returns a reference to a map's property.  If no entry is found and `init` is true, the location will be auto-
// initializer (using `initer` if it is non-nil).  Otherwise, nil is simply returned.
func (props Properties) GetAddr(key PropertyKey) *Pointer {
	return props[key]
}

// InitAddr initializes a map's property slot with the given default value, substituting null if that's empty.
func (props Properties) InitAddr(key PropertyKey, obj *Object, readonly bool) *Pointer {
	// If no object was provided, initialize the slot to null.
	if obj == nil {
		obj = NewNullObject()
	}

	ptr := NewPointer(obj, readonly)
	contract.Assert(props[key] == nil)
	props[key] = ptr
	return ptr
}

// DefaultClassProperty figures out the right default values for a class property, either static and instance.
func DefaultClassProperty(key PropertyKey,
	class *symbols.Class, this *Object, ctx symbols.Function) (*Object, bool) {
	// The property didn't already exist.  See if the class has a definition we can use as a default value.
	var obj *Object
	var readonly bool
	if class != nil {
		if member, hasmember := class.Members[tokens.ClassMemberName(key)]; hasmember {
			if member.Static() == (this == nil) {
				switch m := member.(type) {
				case *symbols.ClassMethod:
					obj = NewFunctionObject(m, this)
					readonly = true // TODO[marapongo/mu#56]: consider permitting JS-style overwriting of methods.
				case *symbols.ClassProperty:
					// If there is a default value, use it.
					if m.Default() != nil {
						obj = NewConstantObject(*m.Default())
					}
					// If the property is readonly, enforce it (unless we are initializing).
					if m.Readonly() {
						if meth, ismeth := ctx.(*symbols.ClassMethod); ismeth &&
							(meth.Parent == class &&
								(this == nil && meth.MemberName() == tokens.ClassInitFunction) ||
								(this != nil && meth.MemberName() == tokens.ClassConstructorFunction)) {
							readonly = false
						} else {
							readonly = true
						}
					}
				}
			}
		}
	}
	return obj, readonly
}
