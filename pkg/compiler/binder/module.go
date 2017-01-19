// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/util/contract"
)

func (b *binder) bindModule(node *ast.Module) *symbols.Module {
	// First bind all imports to concrete symbols.  These will be used to perform initialization later on.
	var imports symbols.Modules
	if node.Imports != nil {
		for _, imptok := range *node.Imports {
			if imp := b.scope.LookupModule(imptok); imp != nil {
				imports = append(imports, imp)
			}
		}
	}
	// TODO: add the imports to the symbol table.

	// Next, bind each member at the symbolic level; in particular, we do not yet bind bodies of methods.
	members := make(symbols.ModuleMemberMap)
	if node.Members != nil {
		for memtok, member := range *node.Members {
			members[memtok] = b.bindModuleMember(member)
		}
	}
	// TODO: add these to the the symbol table for binding bodies.

	// TODO: bind the bodies.

	return &symbols.Module{
		Node:    node,
		Imports: imports,
		Members: members,
	}
}

func (b *binder) bindModuleMember(node ast.ModuleMember) symbols.ModuleMember {
	switch n := node.(type) {
	case *ast.Class:
		return b.bindClass(n)
	case *ast.Export:
		return b.bindExport(n)
	case *ast.ModuleProperty:
		return b.bindModuleProperty(n)
	case *ast.ModuleMethod:
		return b.bindModuleMethod(n)
	default:
		contract.Failf("Unrecognized module member kind: %v", node.GetKind())
		return nil
	}
}

func (b *binder) bindExport(node *ast.Export) *symbols.Export {
	// To bind an export, simply look up the referent symbol and associate this name with it.
	refsym := b.scope.Lookup(node.Referent)
	if refsym == nil {
		// TODO: issue a verification error; name not found!  Also sub in a "bad" symbol.
		contract.Failf("Export name not found: %v", node.Referent)
	}
	return &symbols.Export{
		Node:     node,
		Referent: refsym,
	}
}

func (b *binder) bindModuleProperty(node *ast.ModuleProperty) *symbols.ModuleProperty {
	// Look up this node's type and inject it into the type table.
	b.registerVariableType(node)
	return &symbols.ModuleProperty{Node: node}
}

func (b *binder) bindModuleMethod(node *ast.ModuleMethod) *symbols.ModuleMethod {
	// Make a function type out of this method and inject it into the type table.
	b.registerFunctionType(node)

	// Note that we don't actually bind the body of this method yet.  Until we have gone ahead and injected *all*
	// top-level symbols into the type table, we would potentially encounter missing intra-module symbols.
	return &symbols.ModuleMethod{Node: node}
}
