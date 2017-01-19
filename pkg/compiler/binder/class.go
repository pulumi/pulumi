// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/util/contract"
)

func (b *binder) bindClass(node *ast.Class) *symbols.Class {
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

	// Next, bind each member at the symbolic level; in particular, we do not yet bind bodies of methods.
	members := make(symbols.ClassMemberMap)
	if node.Members != nil {
		for memtok, member := range *node.Members {
			members[memtok] = b.bindClassMember(member)
		}
	}
	// TODO: add these to the the symbol table for binding bodies.

	// TODO: bind the bodies.

	return &symbols.Class{
		Node:       node,
		Extends:    extends,
		Implements: implements,
		Members:    members,
	}
}

func (b *binder) bindClassMember(node ast.ClassMember) symbols.ClassMember {
	switch n := node.(type) {
	case *ast.ClassProperty:
		return b.bindClassProperty(n)
	case *ast.ClassMethod:
		return b.bindClassMethod(n)
	default:
		contract.Failf("Unrecognized class member kind: %v", node.GetKind())
		return nil
	}
}

func (b *binder) bindClassProperty(node *ast.ClassProperty) *symbols.ClassProperty {
	// Look up this node's type and inject it into the type table.
	b.registerVariableType(node)
	return &symbols.ClassProperty{Node: node}
}

func (b *binder) bindClassMethod(node *ast.ClassMethod) *symbols.ClassMethod {
	// Make a function type out of this method and inject it into the type table.
	b.registerFunctionType(node)

	// Note that we don't actually bind the body of this method yet.  Until we have gone ahead and injected *all*
	// top-level symbols into the type table, we would potentially encounter missing intra-module symbols.
	return &symbols.ClassMethod{Node: node}
}
