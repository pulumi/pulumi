// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/marapongo/mu/pkg/ast"
)

// Symbol is a named entity that can be referenced and bound to.
type Symbol struct {
	Name ast.Name
	Node *ast.Node
}

// SymbolKind indicates the kind of symbol being registered (e.g., Stack, Service, etc).
type SymbolKind int

const (
	Service SymbolKind = iota
)

func NewServiceSymbol(nm ast.Name, svc *ast.Service) *Symbol {
	return &Symbol{nm, &svc.Node}
}
