// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/util/contract"
)

func (b *binder) bindClass(node *ast.Class, parent *symbols.Module) *symbols.Class {
	glog.V(3).Infof("Binding module '%v' class '%v'", parent.Name(), node.Name.Ident)

	// Bind base type tokens to actual symbols.
	var extends *symbols.Type
	if node.Extends != nil {
		*extends = b.scope.LookupType(*node.Extends)
	}
	var implements *symbols.Types
	if node.Implements != nil {
		*implements = make(symbols.Types, 0, len(*node.Implements))
		for _, impltok := range *node.Implements {
			if impl := b.scope.LookupType(impltok); impl != nil {
				*implements = append(*implements, impl)
			}
		}
	}

	// Now create a class symbol.  This is required as a parent for the members.
	class := symbols.NewClassSym(node, parent, extends, implements)

	// Next, bind each member at the symbolic level; in particular, we do not yet bind bodies of methods.
	if node.Members != nil {
		for memtok, member := range *node.Members {
			class.Members[memtok] = b.bindClassMember(member, class)
		}
	}
	// TODO: add these to the the symbol table for binding bodies.

	// TODO: bind the bodies.

	return class
}

func (b *binder) bindClassMember(node ast.ClassMember, parent *symbols.Class) symbols.ClassMember {
	switch n := node.(type) {
	case *ast.ClassProperty:
		return b.bindClassProperty(n, parent)
	case *ast.ClassMethod:
		return b.bindClassMethod(n, parent)
	default:
		contract.Failf("Unrecognized class member kind: %v", node.GetKind())
		return nil
	}
}

func (b *binder) bindClassProperty(node *ast.ClassProperty, parent *symbols.Class) *symbols.ClassProperty {
	glog.V(3).Infof("Binding class '%v' property '%v'", parent.Name(), node.Name.Ident)

	// Look up this node's type and inject it into the type table.
	b.registerVariableType(node)
	return symbols.NewClassPropertySym(node, parent)
}

func (b *binder) bindClassMethod(node *ast.ClassMethod, parent *symbols.Class) *symbols.ClassMethod {
	glog.V(3).Infof("Binding class '%v' method '%v'", parent.Name(), node.Name.Ident)

	// Make a function type out of this method and inject it into the type table.
	b.registerFunctionType(node)

	// Note that we don't actually bind the body of this method yet.  Until we have gone ahead and injected *all*
	// top-level symbols into the type table, we would potentially encounter missing intra-module symbols.
	return symbols.NewClassMethodSym(node, parent)
}

func (b *binder) bindClassMethodBody(method *symbols.ClassMethod) {
	glog.V(3).Infof("Binding class method '%v' body", method.Token())
}
