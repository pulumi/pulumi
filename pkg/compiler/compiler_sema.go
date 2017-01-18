// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
	"github.com/marapongo/mu/pkg/symbols"
	"github.com/marapongo/mu/pkg/util/contract"
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
}

// bindStack performs the two phases of binding plus dependency resolution for the given Stack.
func (c *compiler) bindStack(b Binder, w workspace.W, stack *ast.Stack) {
	contract.Assert(stack != nil)

	// First prepare the AST for binding.
	refs := b.PrepareStack(stack)
	if !c.Diag().Success() {
		return
	}

	// Next, resolve all dependencies discovered during this first pass.
	deprefs := make(ast.DependencyRefs)
	for _, ref := range refs {
		// Only resolve dependencies that are currently unknown.  This will exlude built-in types that have already
		// been bound to a stack during the first phase of binding.  Note that we don't actually parse and perform
		// template substitution here; instead, we remember the document and let the binder do this, since it has
		// all of the information necessary to create a unique Stack per-PropertyBag used to instantiate it.
		if doc := c.resolveDependency(w, stack, ref); doc != nil {
			deprefs[ref] = &ast.UninstStack{Ref: ref, Doc: doc}
		}
	}
	if !c.Diag().Success() {
		return
	}

	// Complete the binding process.
	deps := b.BindStack(stack, deprefs)
	if !c.Diag().Success() {
		return
	}

	// Now ensure we bind all dependency stacks too.
	for _, dep := range deps {
		c.bindStack(b, w, dep)
	}
	if !c.Diag().Success() {
		return
	}

	// Finally, now that we know the entire tree, and its transitive closure, is bound, perform final validation.
	b.ValidateStack(stack)
}

// resolveDependency loads up the target dependency from the current workspace using the stack resolution rules.
func (c *compiler) resolveDependency(w workspace.W, stack *ast.Stack, ref symbols.Ref) *diag.Document {
	glog.V(3).Infof("Loading Stack %v dependency %v", stack.Name, ref)

	// First, see if we've already loaded this dependency (anywhere in any Stacks).  If yes, reuse it.
	// TODO: check for version mismatches.
	if doc, exists := c.deps[ref]; exists {
		return doc
	}

	// There are many places a dependency could come from.  Consult the workspace for a list of those paths.  It will
	// return a number of them, in preferred order, and we simply probe each one until we find something.
	dep := ref.MustParse()
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

			// Memoize this in the compiler's cache and return it.
			c.deps[ref] = doc
			return doc
		}
	}

	// If we got to this spot, we could not find the dependency.  Issue an error and bail out.
	c.Diag().Errorf(errors.ErrorStackTypeNotFound.At(stack), ref)
	return nil
}
