// Copyright 2016 Marapongo, Inc. All rights reserved.

package eval

import (
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/compiler/types"
	"github.com/marapongo/mu/pkg/util/contract"
)

// localScope is a kind of scope that holds local variable values.
type localScope struct {
	slot   **localScope
	parent *localScope
	values valueMap
}

// valueMap maps local variables to their current known object value (if any).
type valueMap map[*symbols.LocalVariable]*Object

func initLocalScope(slot **localScope) *localScope {
	return &localScope{
		slot:   slot,
		parent: *slot,
		values: make(valueMap),
	}
}

func (s *localScope) Push() *localScope {
	return initLocalScope(s.slot)
}

func (s *localScope) Pop() {
	contract.Assert(*s.slot == s)
	*s.slot = s.parent
}

func (s *localScope) GetValue(sym *symbols.LocalVariable) *Object {
	return s.values[sym]
}

func (s *localScope) SetValue(sym *symbols.LocalVariable, obj *Object) {
	contract.Assert(obj == nil || types.CanConvert(obj.Type, sym.Type()))
	s.values[sym] = obj
}
