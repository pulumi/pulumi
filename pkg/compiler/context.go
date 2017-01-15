// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/compiler/backends"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Context holds all state available to any templates or code evaluated at compile-time.
type Context struct {
	Options    *Options        // compiler options supplied.
	Cluster    *ast.Cluster    // the cluster that we will deploy to.
	Arch       backends.Arch   // the target cloud architecture.
	Properties ast.PropertyBag // properties supplied at stack construction time.
}

// NewContext returns a new, empty context.
func NewContext(opts *Options) *Context {
	return &Context{
		Options:    opts,
		Properties: make(ast.PropertyBag),
	}
}

// WithClusterArch returns a clone of this Context with the given cluster and architecture attached to it.
func (c *Context) WithClusterArch(cl *ast.Cluster, a backends.Arch) *Context {
	contract.Assert(cl != nil)
	return &Context{
		Cluster:    cl,
		Arch:       a,
		Options:    c.Options,
		Properties: c.Properties,
	}
}

// WithProps returns a clone of this Context with the given properties attached to it.
func (c *Context) WithProps(props ast.PropertyBag) *Context {
	if props == nil {
		props = make(ast.PropertyBag)
	}
	return &Context{
		Cluster:    c.Cluster,
		Arch:       c.Arch,
		Options:    c.Options,
		Properties: props,
	}
}
