// Copyright 2016 Marapongo, Inc. All rights reserved.

package eval

import (
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/util/contract"
)

// localScope is a kind of scope that holds local variable values.
type localScope struct {
	Slot   **localScope
	Parent *localScope
	Values ValueMap
}

func initLocalScope(slot **localScope) *localScope {
	return &localScope{
		Slot:   slot,
		Parent: *slot,
		Values: make(ValueMap),
	}
}

func (s *localScope) Push() *localScope {
	return initLocalScope(s.Slot)
}

func (s *localScope) Pop() {
	contract.Assert(*s.Slot == s)
	*s.Slot = s.Parent
}

// ValueMap maps local variables to their current known object value (if any).
type ValueMap map[*symbols.LocalVariable]*Object
