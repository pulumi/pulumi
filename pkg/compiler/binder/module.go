// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/util/contract"
)

func (b *binder) bindModule(node *ast.Module, parent *symbols.Package) *symbols.Module {
	glog.V(3).Infof("Binding package %v module %v", parent.Name, node.Name)

	// First create a module symbol with empty members, so we can use it as a parent below.
	module := symbols.NewModuleSym(node, parent)

	// First bind all imports to concrete symbols.  These will be used to perform initialization later on.
	if node.Imports != nil {
		for _, imptok := range *node.Imports {
			if imp := b.scope.LookupModule(imptok); imp != nil {
				module.Imports = append(module.Imports, imp)
			}
		}
		// TODO: add the imports to the symbol table.
	}

	// Next, bind each member at the symbolic level; in particular, we do not yet bind bodies of methods.
	if node.Members != nil {
		for memtok, member := range *node.Members {
			module.Members[memtok] = b.bindModuleMember(member, module)
		}
	}
	// TODO: add these to the the symbol table for binding bodies.

	// TODO: bind the bodies.

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
	// To bind an export, simply look up the referent symbol and associate this name with it.
	refsym := b.scope.Lookup(node.Referent)
	if refsym == nil {
		// TODO: issue a verification error; name not found!  Also sub in a "bad" symbol.
		contract.Failf("Export name not found: %v", node.Referent)
	}
	return symbols.NewExportSym(node, parent, refsym)
}

func (b *binder) bindModuleProperty(node *ast.ModuleProperty, parent *symbols.Module) *symbols.ModuleProperty {
	// Look up this node's type and inject it into the type table.
	b.registerVariableType(node)
	return symbols.NewModulePropertySym(node, parent)
}

func (b *binder) bindModuleMethod(node *ast.ModuleMethod, parent *symbols.Module) *symbols.ModuleMethod {
	// Make a function type out of this method and inject it into the type table.
	b.registerFunctionType(node)

	// Note that we don't actually bind the body of this method yet.  Until we have gone ahead and injected *all*
	// top-level symbols into the type table, we would potentially encounter missing intra-module symbols.
	return symbols.NewModuleMethodSym(node, parent)
}
