// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
	"github.com/marapongo/mu/pkg/util"
	"github.com/marapongo/mu/pkg/workspace"
)

// buildDocumentSema runs the middle semantic analysis phases of the compiler.
func (c *compiler) buildDocumentSema(w workspace.W, stack *ast.Stack) {
	// Perform semantic analysis on all stacks passes to validate, transform, and/or update the AST.
	b := NewBinder(c)
	c.bindStack(b, w, stack)
	if !c.Diag().Success() {
		return
	}

	// Now ensure that we analyze all dependency ASTs also, which will populate their bound nodes.
	for _, ref := range ast.StableBoundDependencies(stack.BoundDependencies) {
		c.bindStack(b, w, stack.BoundDependencies[ref].Stack)
	}
}

// bindStack performs the two phases of binding plus dependency resolution for the given Stack.
func (c *compiler) bindStack(b Binder, w workspace.W, stack *ast.Stack) {
	// First prepare the AST for binding.
	util.Assert(stack.BoundDependencies == nil)
	b.PrepareStack(stack)
	if !c.Diag().Success() {
		return
	}

	// Next, resolve all dependencies discovered during this first pass.
	util.Assert(stack.BoundDependencies != nil)
	for _, ref := range ast.StableBoundDependencies(stack.BoundDependencies) {
		dep := stack.BoundDependencies[ref]
		if dep.Stack == nil {
			// Only resolve dependencies that are currently unknown.  This will exlude built-in types that have already
			// been bound to a stack during the first phase of binding.
			dep.Stack = c.resolveDependency(w, stack, ref, dep.Ref)
			stack.BoundDependencies[ref] = dep
		}
	}
	if !c.Diag().Success() {
		return
	}

	// Finally, actually complete the binding process.
	b.BindStack(stack)
}

// resolveDependency loads up the target dependency from the current workspace using the stack resolution rules.
func (c *compiler) resolveDependency(w workspace.W, stack *ast.Stack, ref ast.Ref, dep ast.RefParts) *ast.Stack {
	glog.V(3).Infof("Loading Stack %v dependency %v", stack.Name, dep)

	// First, see if we've already loaded this dependency (anywhere in any Stacks).  If yes, reuse it.
	// TODO: check for version mismatches.
	if stack, exists := c.deps[ref]; exists {
		return stack
	}

	// There are many places a dependency could come from.  Consult the workspace for a list of those paths.  It will
	// return a number of them, in preferred order, and we simply probe each one until we find something.
	for _, loc := range w.DepCandidates(dep) {
		// Try to read this location as a document.
		isMufile := workspace.IsMufile(loc, c.Diag())
		glog.V(5).Infof("Probing for dependency %v at %v: %v", dep, loc, isMufile)

		if isMufile {
			doc, err := diag.ReadDocument(loc)
			if err != nil {
				c.Diag().Errorf(errors.ErrorCouldNotReadMufile.AtFile(loc), err)
				return nil
			}

			// If we got this far, we've loaded up the dependency's Mufile; parse it and return the result.  Note that
			// this will be processed later on in semantic analysis, to ensure semantic problems are caught.
			p := NewParser(c)
			stack := p.ParseStack(doc)
			if !c.Diag().Success() {
				return nil
			}

			// Memoize this in the compiler's cache and return it.
			c.deps[ref] = stack
			return stack
		}
	}

	// If we got to this spot, we could not find the dependency.  Issue an error and bail out.
	c.Diag().Errorf(errors.ErrorStackTypeNotFound.At(stack.Doc), dep)
	return nil
}
