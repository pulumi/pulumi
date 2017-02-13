// Copyright 2016 Marapongo, Inc. All rights reserved.

package rt

import (
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

type Properties map[PropertyKey]*Pointer // an object's properties.
type PropertyKey string                  // property keys are strings (incl. invalid identifiers for dynamic).

// GetAddr returns a reference to a map's property.  If no entry is found and `init` is true, the property will
// be auto-initialized.  If this happens and class is non-nil, it will be used to seed the default value.
func (props Properties) GetAddr(key PropertyKey, init bool, class *symbols.Class, this *Object) *Pointer {
	ptr, hasprop := props[key]
	if !hasprop && init {
		// The property didn't already exist, but was zero-initialized to null.  Look up the property definition (if
		// any) in the members list, so that we may seed a default initialization value.
		var obj *Object
		var readonly bool
		if class != nil {
			if member, hasmember := class.Members[tokens.ClassMemberName(key)]; hasmember {
				// Only use this member if it's of the expected instance/static variety.
				if member.Static() == (this == nil) {
					switch m := member.(type) {
					case *symbols.ClassProperty:
						if m.Default() != nil {
							obj = NewConstantObject(*m.Default())
						}
						readonly = m.Readonly()
					case *symbols.ClassMethod:
						obj = NewFunctionObject(m, this)
						readonly = true // TODO[marapongo/mu#56]: consider permitting JS-style overwriting of methods.
					default:
						contract.Failf("Unexpected member type: %v", member)
					}
				}
			}
		}

		// If no entry was found, and init is true, initialize the slot to null.
		if obj == nil {
			obj = NewNullObject()
		}

		ptr = NewPointer(obj, readonly)
		props[key] = ptr
	}
	return ptr
}
