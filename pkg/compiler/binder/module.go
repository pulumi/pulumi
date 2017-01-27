// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/util/contract"
)

func (b *binder) bindModule(node *ast.Module, parent *symbols.Package) *symbols.Module {
	glog.V(3).Infof("Binding package '%v' module '%v'", parent.Name(), node.Name.Ident)

	// First create a module symbol with empty members, so we can use it as a parent below.
	module := symbols.NewModuleSym(node, parent)
	b.ctx.RegisterSymbol(node, module)

	// Set the current module in the context so we can e.g. enforce accessibility.
	priormodule := b.ctx.Currmodule
	b.ctx.Currmodule = module
	defer func() { b.ctx.Currmodule = priormodule }()

	// First bind all imports to concrete symbols.  These will be used to perform initialization later on.
	if node.Imports != nil {
		for _, imptok := range *node.Imports {
			if imp := b.bindModuleToken(imptok); imp != nil {
				module.Imports = append(module.Imports, imp)
			}
		}
	}

	// Next, bind each member and add it to the module's map.
	if node.Members != nil {
		// First bind members and add them to the symbol table.  Note that this does not bind bodies just yet.  The
		// reason is that module members may freely reference one another, so we must do this in a second pass.
		for nm, member := range *node.Members {
			module.Members[nm] = b.bindModuleMember(member, module)
		}
	}

	return module
}

func (b *binder) bindModuleMember(node ast.ModuleMember, parent *symbols.Module) symbols.ModuleMember {
	switch n := node.(type) {
	case *ast.Class:
		return b.bindClass(n, parent)
	case *ast.Export:
		return b.bindExport(n, parent)
	case *ast.ModuleProperty:
		return b.bindModuleProperty(n, parent)
	case *ast.ModuleMethod:
		return b.bindModuleMethod(n, parent)
	default:
		contract.Failf("Unrecognized module member kind: %v", node.GetKind())
		return nil
	}
}

func (b *binder) bindExport(node *ast.Export, parent *symbols.Module) *symbols.Export {
	glog.V(3).Infof("Binding module '%v' export '%v'", parent.Name(), node.Name.Ident)

	// To bind an export, simply look up the referent symbol and associate this name with it.
	if refsym := b.lookupSymbolToken(node.Referent, node.Referent.Tok, true); refsym != nil {
		sym := symbols.NewExportSym(node, parent, refsym)
		b.ctx.RegisterSymbol(node, sym)
		return sym
	}
	return nil
}

func (b *binder) bindModuleProperty(node *ast.ModuleProperty, parent *symbols.Module) *symbols.ModuleProperty {
	glog.V(3).Infof("Binding module '%v' property '%v'", parent.Name(), node.Name.Ident)

	// Look up this node's type and inject it into the type table.
	ty := b.bindType(node.Type)
	sym := symbols.NewModulePropertySym(node, parent, ty)
	b.ctx.RegisterSymbol(node, sym)
	return sym
}

func (b *binder) bindModuleMethod(node *ast.ModuleMethod, parent *symbols.Module) *symbols.ModuleMethod {
	glog.V(3).Infof("Binding module '%v' method '%v'", parent.Name(), node.Name.Ident)

	// Make a function type out of this method and inject it into the type table.
	ty := b.bindFunctionType(node)
	sym := symbols.NewModuleMethodSym(node, parent, ty)
	b.ctx.RegisterSymbol(node, sym)

	// Note that we don't actually bind the body of this method yet.  Until we have gone ahead and injected *all*
	// top-level symbols into the type table, we would potentially encounter missing intra-module symbols.
	return sym
}

func (b *binder) bindModuleMethodBodies(module *symbols.Module) {
	// Set the current module in the context so we can e.g. enforce accessibility.  We need to do this again while
	// binding the module bodies so that the correct context is reestablished for lookups, etc.
	priormodule := b.ctx.Currmodule
	b.ctx.Currmodule = module
	defer func() { b.ctx.Currmodule = priormodule }()

	// Just dig through, find all ModuleMethod and ClassMethod symbols, and bind them.
	for _, member := range module.Members {
		switch m := member.(type) {
		case *symbols.ModuleMethod:
			b.bindModuleMethodBody(m)
		case *symbols.Class:
			b.bindClassMethodBodies(m)
		}
	}
}

func (b *binder) bindModuleMethodBody(method *symbols.ModuleMethod) {
	glog.V(3).Infof("Binding module method '%v' body", method.Token())
	// Push a new activation frame and bind the body.
	scope := b.ctx.Scope.Push(true)
	defer scope.Pop()
	b.bindFunctionBody(method.Node)
}
