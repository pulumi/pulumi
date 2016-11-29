// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/compiler/backends"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/workspace"
)

// buildDocumentBE runs the back-end phases of the compiler.
func (c *compiler) buildDocumentBE(w workspace.W, stack *ast.Stack) {
	if c.opts.SkipCodegen {
		glog.V(2).Infof("Skipping code-generation (opts.SkipCodegen=true)")
	} else {
		glog.V(2).Infof("Stack %v targets cluster=%v arch=%v", stack.Name, c.ctx.Cluster.Name, c.ctx.Arch)

		// Now get the backend cloud provider to process the stack from here on out.
		be := backends.New(c.ctx.Arch, c.Diag())
		be.CodeGen(core.Compiland{c.ctx.Cluster, stack})
	}
}
