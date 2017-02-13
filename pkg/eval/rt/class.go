// Copyright 2016 Marapongo, Inc. All rights reserved.

package rt

import (
	"github.com/marapongo/mu/pkg/compiler/symbols"
)

// ClassStatics is a holder for static variables associated with a given class.
type ClassStatics struct {
	class   *symbols.Class
	statics Properties
}

func NewClassStatics(class *symbols.Class) *ClassStatics {
	return &ClassStatics{
		class:   class,
		statics: make(Properties),
	}
}

// GetPropertyAddr returns the reference to a class's static property, lazily initializing if 'init' is true, or
// returning nil otherwise.
func (c *ClassStatics) GetPropertyAddr(key PropertyKey, init bool, ctx symbols.Function) *Pointer {
	// The fast path is a quick lookup.
	if ptr := c.statics.GetAddr(key); ptr != nil || !init {
		return ptr
	}

	// Nothing was found; go ahead and try again, initializing the slot with a default value.
	obj, readonly := DefaultClassProperty(key, c.class, nil, ctx)
	return c.statics.InitAddr(key, obj, readonly)
}
