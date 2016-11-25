// Copyright 2016 Marapongo, Inc. All rights reserved.

package core

import (
	"github.com/marapongo/mu/pkg/ast"
)

// Context holds all state available to any templates or code evaluated at compile-time.
type Context struct {
	Properties ast.PropertyBag // properties supplied at stack construction time.
}

// WithProps returns a new, clone of this Context with the given properties attached to it.
func (c *Context) WithProps(props ast.PropertyBag) *Context {
	return &Context{
		Properties: props,
	}
}
