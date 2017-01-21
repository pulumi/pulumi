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

	// First bind all imports to concrete symbols.  These will be used to perform initialization later on.
	if node.Imports != nil {
		for _, imptok := range *node.Imports {
			if imp := b.scope.LookupModule(imptok.Tok); imp != nil {
				module.Imports = append(module.Imports, imp)
			}
		}
		// TODO: add the imports to the symbol table.
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
	refsym := b.scope.Lookup(node.Referent.Tok)
	if refsym == nil {
		// TODO: issue a verification error; name not found!  Also sub in a "bad" symbol.
		contract.Failf("Export name not found: %v", node.Referent)
	}
	return symbols.NewExportSym(node, parent, refsym)
}

func (b *binder) bindModuleProperty(node *ast.ModuleProperty, parent *symbols.Module) *symbols.ModuleProperty {
	glog.V(3).Infof("Binding module '%v' property '%v'", parent.Name(), node.Name.Ident)

	// Look up this node's type and inject it into the type table.
	b.registerVariableType(node)
	return symbols.NewModulePropertySym(node, parent)
}

func (b *binder) bindModuleMethod(node *ast.ModuleMethod, parent *symbols.Module) *symbols.ModuleMethod {
	glog.V(3).Infof("Binding module '%v' method '%v'", parent.Name(), node.Name.Ident)

	// Make a function type out of this method and inject it into the type table.
	b.registerFunctionType(node)

	// Note that we don't actually bind the body of this method yet.  Until we have gone ahead and injected *all*
	// top-level symbols into the type table, we would potentially encounter missing intra-module symbols.
	return symbols.NewModuleMethodSym(node, parent)
}

func (b *binder) bindModuleMethodBody(method *symbols.ModuleMethod) {
	glog.V(3).Infof("Binding module method '%v' body", method.Token())
	b.bindFunctionBody(method.Node)
}
