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
	var extends symbols.Type
	if node.Extends != nil {
		extends = b.ctx.Scope.LookupType(node.Extends.Tok)
	}
	var implements symbols.Types
	if node.Implements != nil {
		for _, impltok := range *node.Implements {
			if impl := b.ctx.Scope.LookupType(impltok.Tok); impl != nil {
				implements = append(implements, impl)
			}
		}
	}

	// Now create a class symbol.  This is required as a parent for the members.
	class := symbols.NewClassSym(node, parent, extends, implements)
	b.ctx.RegisterSymbol(node, class)

	// Set the current class in the context so we can e.g. enforce accessibility.
	priorclass := b.ctx.Currclass
	b.ctx.Currclass = class
	defer func() { b.ctx.Currclass = priorclass }()

	// Next, bind each member at the symbolic level; in particular, we do not yet bind bodies of methods.
	if node.Members != nil {
		for memtok, member := range *node.Members {
			class.Members[memtok] = b.bindClassMember(member, class)
		}
	}

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
	typ := b.bindType(node.Type)
	sym := symbols.NewClassPropertySym(node, parent, typ)
	b.ctx.RegisterSymbol(node, sym)
	return sym
}

func (b *binder) bindClassMethod(node *ast.ClassMethod, parent *symbols.Class) *symbols.ClassMethod {
	glog.V(3).Infof("Binding class '%v' method '%v'", parent.Name(), node.Name.Ident)

	// Make a function type out of this method and inject it into the type table.
	typ := b.bindFunctionType(node)
	sym := symbols.NewClassMethodSym(node, parent, typ)
	b.ctx.RegisterSymbol(node, sym)

	// Note that we don't actually bind the body of this method yet.  Until we have gone ahead and injected *all*
	// top-level symbols into the type table, we would potentially encounter missing intra-module symbols.
	return sym
}

func (b *binder) bindClassMethodBody(method *symbols.ClassMethod) {
	glog.V(3).Infof("Binding class method '%v' body", method.Token())

	// Set the current class in the context so we can e.g. enforce accessibility.  Note that we have to do it here, in
	// addition to the above during ordinary class binding, due to the two pass function body binding model.
	priorclass := b.ctx.Currclass
	b.ctx.Currclass = method.Parent
	defer func() { b.ctx.Currclass = priorclass }()

	b.bindFunctionBody(method.Node)
}
