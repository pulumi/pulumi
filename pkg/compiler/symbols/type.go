// Copyright 2016 Marapongo, Inc. All rights reserved.

package symbols

import (
	"fmt"

	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/tokens"
)

// Type is a type symbol that can be used for typechecking operations.
type Type interface {
	Symbol
	typesym()
}

// primitive is an internal representation of a primitive type symbol (any, bool, number, string).
type primitive struct {
	Name tokens.Token
}

var _ Symbol = (*primitive)(nil)
var _ Type = (*primitive)(nil)

func (node *primitive) symbol()                {}
func (node *primitive) typesym()               {}
func (node *primitive) GetName() tokens.Token  { return node.Name }
func (node *primitive) GetTree() diag.Diagable { return nil }

// All of the primitive types.
var (
	AnyType    = &primitive{"any"}
	BoolType   = &primitive{"bool"}
	NumberType = &primitive{"number"}
	StringType = &primitive{"string"}
)

// Primitives contains a map of all primitive types, keyed by their token/name.
var Primitives = map[tokens.Token]Type{
	AnyType.Name:    AnyType,
	BoolType.Name:   BoolType,
	NumberType.Name: NumberType,
	StringType.Name: StringType,
}

// Declare some type decorator strings used for parsing and producing array/map types.
const (
	TypeDecorsArray        = "%v" + TypeDecorsArraySuffix
	TypeDecorsArraySuffix  = "[]"
	TypeDecorsMap          = TypeDecorsMapPrefix + "%v" + TypeDecorsMapSeparator + "%v"
	TypeDecorsMapPrefix    = "map["
	TypeDecorsMapSeparator = "]"
)

// ArrayName creates a new array name from an element type.
func ArrayName(elem Type) tokens.Token {
	// TODO: consider caching this to avoid creating needless strings.
	return tokens.Token(fmt.Sprintf(TypeDecorsArray, elem.GetName()))
}

// MapName creates a new array name from an element type.
func MapName(key Type, elem Type) tokens.Token {
	// TODO: consider caching this to avoid creating needless strings.
	return tokens.Token(fmt.Sprintf(TypeDecorsMap, key.GetName(), elem.GetName()))
}

// ArrayType is an array whose elements are of some other type.
type ArrayType struct {
	Element Type
}

var _ Symbol = (*ArrayType)(nil)
var _ Type = (*ArrayType)(nil)

func (node *ArrayType) symbol()                {}
func (node *ArrayType) typesym()               {}
func (node *ArrayType) GetName() tokens.Token  { return ArrayName(node.Element) }
func (node *ArrayType) GetTree() diag.Diagable { return nil }

// KeyType is an array whose keys and elements are of some other types.
type MapType struct {
	Key     Type
	Element Type
}

var _ Symbol = (*MapType)(nil)
var _ Type = (*MapType)(nil)

func (node *MapType) symbol()                {}
func (node *MapType) typesym()               {}
func (node *MapType) GetName() tokens.Token  { return MapName(node.Key, node.Element) }
func (node *MapType) GetTree() diag.Diagable { return nil }
