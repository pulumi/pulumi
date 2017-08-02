// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package eval

import (
	"github.com/pulumi/pulumi-fabric/pkg/compiler/symbols"
	"github.com/pulumi/pulumi-fabric/pkg/diag"
	"github.com/pulumi/pulumi-fabric/pkg/eval/rt"
)

// Hooks is a set of callbacks that can be used to hook into interesting interpreter events.
type Hooks interface {
	// OnStart is invoked just before interpretation begins.
	OnStart() *rt.Unwind
	// OnEnterPackage is invoked whenever we enter a package.
	OnEnterPackage(pkg *symbols.Package) (*rt.Unwind, func())
	// OnEnterModule is invoked whenever we enter a module.
	OnEnterModule(sym *symbols.Module) (*rt.Unwind, func())
	// OnEnterFunction is invoked whenever we enter a function.
	OnEnterFunction(fnc symbols.Function, args []*rt.Object) (*rt.Unwind, func())
	// OnObjectInit is invoked after an object has been allocated and initialized.  This means that its constructor, if
	// any, has been run to completion.  The diagnostics tree is the AST node responsible for the allocation.
	OnObjectInit(tree diag.Diagable, o *rt.Object) *rt.Unwind
	// OnDone is invoked after interpretation has completed.  It is given access to the final unwind information.
	OnDone(uw *rt.Unwind) *rt.Unwind
}
