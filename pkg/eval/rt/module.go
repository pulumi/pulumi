// Copyright 2016 Marapongo, Inc. All rights reserved.

package rt

import (
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/tokens"
)

// ModuleGlobals is a holder for static variables associated with a given class.
type ModuleGlobals struct {
	module     *symbols.Module
	properties Properties
}

func NewModuleGlobals(module *symbols.Module) *ModuleGlobals {
	return &ModuleGlobals{
		module:     module,
		properties: make(Properties),
	}
}

// GetPropertyAddr returns the reference to a module's property, lazily initializing if 'init' is true, or
// returning nil otherwise.  The `ctx` context is used to determine whether to ignore readonliness.
func (m *ModuleGlobals) GetPropertyAddr(key PropertyKey, init bool, ctx symbols.Function) *Pointer {
	// The fast path is just a quick check for the slot.
	if ptr := m.properties.GetAddr(key); ptr != nil || !init {
		return ptr
	}

	// Otherwise, the property didn't exist.  See if the module has a definition we can use as a default value.
	var obj *Object
	var readonly bool
	if member, hasmember := m.module.Members[tokens.ModuleMemberName(key)]; hasmember {
		switch t := member.(type) {
		case *symbols.ModuleMethod:
			obj = NewFunctionObject(t, nil)
			readonly = true // TODO[marapongo/mu#56]: consider permitting JS-style overwriting of methods.
		case *symbols.ModuleProperty:
			// If there is a default value, use it.
			if t.Default() != nil {
				obj = NewConstantObject(*t.Default())
			}
			// If the property is readonly, enforce it (unless we are initializing).
			if t.Readonly() {
				if meth, ismeth := ctx.(*symbols.ModuleMethod); ismeth &&
					meth.Parent == m.module && meth.MemberName() == tokens.ModuleInitFunction {
					readonly = false
				} else {
					readonly = true
				}
			}
		}
	}

	// Now use those values, if any, to initialize the slot and return it.
	return m.properties.InitAddr(key, obj, readonly)
}
