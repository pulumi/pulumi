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

// Types is a list of type symbols.
type Types []Type

// primitive is an internal representation of a primitive type symbol (any, bool, number, string).
type primitive struct {
	Nm tokens.Type
}

var _ Symbol = (*primitive)(nil)
var _ Type = (*primitive)(nil)

func (node *primitive) symbol()             {}
func (node *primitive) typesym()            {}
func (node *primitive) Name() tokens.Name   { return tokens.Name(node.Nm) }
func (node *primitive) Token() tokens.Token { return tokens.Token(node.Nm) }
func (node *primitive) Tree() diag.Diagable { return nil }

// All of the primitive types.
var (
	AnyType    = &primitive{"any"}
	BoolType   = &primitive{"bool"}
	NumberType = &primitive{"number"}
	StringType = &primitive{"string"}
)

// Primitives contains a map of all primitive types, keyed by their token/name.
var Primitives = map[tokens.Type]Type{
	AnyType.Nm:    AnyType,
	BoolType.Nm:   BoolType,
	NumberType.Nm: NumberType,
	StringType.Nm: StringType,
}

// Declare some type decorator strings used for parsing and producing array/map types.
const (
	TypeDecorsArray             = "%v" + TypeDecorsArraySuffix
	TypeDecorsArraySuffix       = "[]"
	TypeDecorsMap               = TypeDecorsMapPrefix + "%v" + TypeDecorsMapSeparator + "%v"
	TypeDecorsMapPrefix         = "map["
	TypeDecorsMapSeparator      = "]"
	TypeDecorsFunction          = TypeDecorsFunctionPrefix + "%v" + TypeDecorsFunctionSeparator + "%v"
	TypeDecorsFunctionPrefix    = "("
	TypeDecorsFunctionParamSep  = ","
	TypeDecorsFunctionSeparator = ")"
)

// ArrayName creates a new array type token from an element type.
func ArrayTypeToken(elem Type) tokens.Type {
	// TODO: consider caching this to avoid creating needless strings.
	return tokens.Type(fmt.Sprintf(TypeDecorsArray, elem.Token()))
}

// MapTypeToken creates a new map type token from an element type.
func MapTypeToken(key Type, elem Type) tokens.Type {
	// TODO: consider caching this to avoid creating needless strings.
	return tokens.Type(fmt.Sprintf(TypeDecorsMap, key.Token(), elem.Token()))
}

// FunctionTypeToken creates a new function type token from parameter and return types.
func FunctionTypeToken(params []Type, ret Type) tokens.Type {
	// TODO: consider caching this to avoid creating needless strings.

	// Stringify the parameters (if any).
	sparams := ""
	for i, param := range params {
		if i > 0 {
			sparams += TypeDecorsFunctionParamSep
		}
		sparams += string(param.Token())
	}

	// Stringify the return type (if any).
	sret := ""
	if ret != nil {
		sret = string(ret.Token())
	}

	return tokens.Type(fmt.Sprintf(TypeDecorsFunction, sparams, sret))
}

// ArrayType is an array whose elements are of some other type.
type ArrayType struct {
	Element Type
}

var _ Symbol = (*ArrayType)(nil)
var _ Type = (*ArrayType)(nil)

func (node *ArrayType) symbol()             {}
func (node *ArrayType) typesym()            {}
func (node *ArrayType) Name() tokens.Name   { return tokens.Name(node.Token()) }
func (node *ArrayType) Token() tokens.Token { return tokens.Token(ArrayTypeToken(node.Element)) }
func (node *ArrayType) Tree() diag.Diagable { return nil }

func NewArrayType(elem Type) *ArrayType {
	return &ArrayType{elem}
}

// KeyType is an array whose keys and elements are of some other types.
type MapType struct {
	Key     Type
	Element Type
}

var _ Symbol = (*MapType)(nil)
var _ Type = (*MapType)(nil)

func (node *MapType) symbol()             {}
func (node *MapType) typesym()            {}
func (node *MapType) Name() tokens.Name   { return tokens.Name(node.Token()) }
func (node *MapType) Token() tokens.Token { return tokens.Token(MapTypeToken(node.Key, node.Element)) }
func (node *MapType) Tree() diag.Diagable { return nil }

func NewMapType(key Type, elem Type) *MapType {
	return &MapType{key, elem}
}

// FunctionType is an invocable type, representing a signature with optional parameters and a return type.
type FunctionType struct {
	Parameters []Type // an array of optional parameter types.
	Return     Type   // a return type, or nil if "void".
}

var _ Symbol = (*FunctionType)(nil)
var _ Type = (*FunctionType)(nil)

func (node *FunctionType) symbol()           {}
func (node *FunctionType) typesym()          {}
func (node *FunctionType) Name() tokens.Name { return tokens.Name(node.Token()) }
func (node *FunctionType) Token() tokens.Token {
	return tokens.Token(FunctionTypeToken(node.Parameters, node.Return))
}
func (node *FunctionType) Tree() diag.Diagable { return nil }

func NewFunctionType(params []Type, ret Type) *FunctionType {
	return &FunctionType{params, ret}
}
