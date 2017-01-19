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
	Name tokens.Token
}

var _ Symbol = (*primitive)(nil)
var _ Type = (*primitive)(nil)

func (node *primitive) symbol()             {}
func (node *primitive) typesym()            {}
func (node *primitive) Token() tokens.Token { return node.Name }
func (node *primitive) Tree() diag.Diagable { return nil }

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

// ArrayName creates a new array name from an element type.
func ArrayName(elem Type) tokens.Token {
	// TODO: consider caching this to avoid creating needless strings.
	return tokens.Token(fmt.Sprintf(TypeDecorsArray, elem.Token()))
}

// MapName creates a new array name from an element type.
func MapName(key Type, elem Type) tokens.Token {
	// TODO: consider caching this to avoid creating needless strings.
	return tokens.Token(fmt.Sprintf(TypeDecorsMap, key.Token(), elem.Token()))
}

// FunctionName creates a new array name from an element type.
func FunctionName(params *[]Type, ret *Type) tokens.Token {
	// TODO: consider caching this to avoid creating needless strings.

	// Stringify the parameters (if any).
	sparams := ""
	if params != nil {
		for i, param := range *params {
			if i > 0 {
				sparams += TypeDecorsFunctionParamSep
			}
			sparams += string(param.Token())
		}
	}

	// Stringify the return type (if any).
	sret := ""
	if ret != nil {
		sret = string((*ret).Token())
	}

	return tokens.Token(fmt.Sprintf(TypeDecorsFunction, sparams, sret))
}

// ArrayType is an array whose elements are of some other type.
type ArrayType struct {
	Element Type
}

var _ Symbol = (*ArrayType)(nil)
var _ Type = (*ArrayType)(nil)

func (node *ArrayType) symbol()             {}
func (node *ArrayType) typesym()            {}
func (node *ArrayType) Token() tokens.Token { return ArrayName(node.Element) }
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
func (node *MapType) Token() tokens.Token { return MapName(node.Key, node.Element) }
func (node *MapType) Tree() diag.Diagable { return nil }

func NewMapType(key Type, elem Type) *MapType {
	return &MapType{key, elem}
}

// FunctionType is an invocable type, representing a signature with optional parameters and a return type.
type FunctionType struct {
	Parameters *[]Type
	Return     *Type
}

var _ Symbol = (*FunctionType)(nil)
var _ Type = (*FunctionType)(nil)

func (node *FunctionType) symbol()             {}
func (node *FunctionType) typesym()            {}
func (node *FunctionType) Token() tokens.Token { return FunctionName(node.Parameters, node.Return) }
func (node *FunctionType) Tree() diag.Diagable { return nil }

func NewFunctionType(params *[]Type, ret *Type) *FunctionType {
	return &FunctionType{params, ret}
}
