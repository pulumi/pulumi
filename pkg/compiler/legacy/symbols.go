// Copyright 2016 Marapongo, Inc. All rights reserved.

package legacy

import (
	"github.com/marapongo/mu/pkg/compiler/legacy/ast"
	"github.com/marapongo/mu/pkg/tokens"
)

// Symbol is a named entity that can be referenced and bound to.
type Symbol struct {
	Kind SymbolKind  // the kind of symbol.
	Name tokens.Name // the symbol's unique name.
	Node *ast.Node   // the Node part of the payload data structure.
	Real interface{} // the real part of the payload (i.e., the whole structure).
}

// SymbolKind indicates the kind of symbol being registered (e.g., Stack, Service, etc).
type SymbolKind int

const (
	SymKindService SymbolKind = iota
	SymKindStack
	SymKindUninstStack
	SymKindSchema
)

func NewServiceSymbol(nm tokens.Name, svc *ast.Service) *Symbol {
	return &Symbol{SymKindService, nm, &svc.Node, svc}
}

func NewStackSymbol(nm tokens.Name, stack *ast.Stack) *Symbol {
	return &Symbol{SymKindStack, nm, &stack.Node, stack}
}

func NewUninstStackSymbol(nm tokens.Name, ref *ast.UninstStack) *Symbol {
	return &Symbol{SymKindUninstStack, nm, &ref.Node, ref}
}

func NewSchemaSymbol(nm tokens.Name, schema *ast.Schema) *Symbol {
	return &Symbol{SymKindSchema, nm, &schema.Node, schema}
}
