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
	Members() ClassMemberMap // this type's members.
	Record() bool            // true if this is a record type.
	Interface() bool         // true if this is an interface type.
}

// Types is a list of type symbols.
type Types []Type

// PrimitiveType is an internal representation of a primitive type symbol (any, bool, number, string).
type PrimitiveType struct {
	Nm tokens.TypeName
}

var _ Symbol = (*PrimitiveType)(nil)
var _ Type = (*PrimitiveType)(nil)

func (node *PrimitiveType) symbol()                 {}
func (node *PrimitiveType) Name() tokens.Name       { return tokens.Name(node.Nm) }
func (node *PrimitiveType) Token() tokens.Token     { return tokens.Token(node.Nm) }
func (node *PrimitiveType) Tree() diag.Diagable     { return nil }
func (node *PrimitiveType) typesym()                {}
func (node *PrimitiveType) Members() ClassMemberMap { return noClassMembers }
func (node *PrimitiveType) Record() bool            { return false }
func (node *PrimitiveType) Interface() bool         { return false }
func (node *PrimitiveType) String() string          { return string(node.Name()) }

func NewPrimitiveType(nm tokens.TypeName) *PrimitiveType {
	return &PrimitiveType{nm}
}

// PointerType represents a pointer to any other type.
type PointerType struct {
	Nm      tokens.TypeName
	Tok     tokens.Type
	Element Type
}

var _ Symbol = (*PointerType)(nil)
var _ Type = (*PointerType)(nil)

func (node *PointerType) symbol()                 {}
func (node *PointerType) Name() tokens.Name       { return tokens.Name(node.Nm) }
func (node *PointerType) Token() tokens.Token     { return tokens.Token(node.Tok) }
func (node *PointerType) Tree() diag.Diagable     { return nil }
func (node *PointerType) typesym()                {}
func (node *PointerType) Members() ClassMemberMap { return noClassMembers }
func (node *PointerType) Record() bool            { return false }
func (node *PointerType) Interface() bool         { return false }
func (node *PointerType) String() string          { return string(node.Name()) }

func NewPointerType(elem Type) *PointerType {
	nm := newPointerTypeName(elem)
	tok := newPointerTypeToken(elem)
	return &PointerType{nm, tok, elem}
}

const (
	TypeDecorsPointer       = TypeDecorsPointerPrefix + "%v"
	TypeDecorsPointerPrefix = "*"
)

// newPointerTypeName creates a new array type name from an element type.
func newPointerTypeName(elem Type) tokens.TypeName {
	return tokens.TypeName(fmt.Sprintf(TypeDecorsPointer, elem.Name()))
}

// newPointerTypeToken creates a new array type token from an element type.
func newPointerTypeToken(elem Type) tokens.Type {
	return tokens.Type(fmt.Sprintf(TypeDecorsPointer, elem.Token()))
}

// ArrayType is an array whose elements are of some other type.
type ArrayType struct {
	Nm      tokens.TypeName
	Tok     tokens.Type
	Element Type
}

var _ Symbol = (*ArrayType)(nil)
var _ Type = (*ArrayType)(nil)

func (node *ArrayType) symbol()                 {}
func (node *ArrayType) Name() tokens.Name       { return tokens.Name(node.Nm) }
func (node *ArrayType) Token() tokens.Token     { return tokens.Token(node.Tok) }
func (node *ArrayType) Tree() diag.Diagable     { return nil }
func (node *ArrayType) typesym()                {}
func (node *ArrayType) Members() ClassMemberMap { return noClassMembers }
func (node *ArrayType) Record() bool            { return false }
func (node *ArrayType) Interface() bool         { return false }
func (node *ArrayType) String() string          { return string(node.Name()) }

func NewArrayType(elem Type) *ArrayType {
	nm := newArrayTypeName(elem)
	tok := newArrayTypeToken(elem)
	return &ArrayType{nm, tok, elem}
}

const (
	TypeDecorsArray       = TypeDecorsArrayPrefix + "%v"
	TypeDecorsArrayPrefix = "[]"
)

// newArrayTypeName creates a new array type name from an element type.
func newArrayTypeName(elem Type) tokens.TypeName {
	return tokens.TypeName(fmt.Sprintf(TypeDecorsArray, elem.Name()))
}

// newArrayTypeToken creates a new array type token from an element type.
func newArrayTypeToken(elem Type) tokens.Type {
	return tokens.Type(fmt.Sprintf(TypeDecorsArray, elem.Token()))
}

// KeyType is an array whose keys and elements are of some other types.
type MapType struct {
	Nm      tokens.TypeName
	Tok     tokens.Type
	Key     Type
	Element Type
}

var _ Symbol = (*MapType)(nil)
var _ Type = (*MapType)(nil)

func (node *MapType) symbol()                 {}
func (node *MapType) Name() tokens.Name       { return tokens.Name(node.Nm) }
func (node *MapType) Token() tokens.Token     { return tokens.Token(node.Tok) }
func (node *MapType) Tree() diag.Diagable     { return nil }
func (node *MapType) typesym()                {}
func (node *MapType) Members() ClassMemberMap { return noClassMembers }
func (node *MapType) Record() bool            { return false }
func (node *MapType) Interface() bool         { return false }
func (node *MapType) String() string          { return string(node.Name()) }

func NewMapType(key Type, elem Type) *MapType {
	nm := newMapTypeName(key, elem)
	tok := newMapTypeToken(key, elem)
	return &MapType{nm, tok, key, elem}
}

const (
	TypeDecorsMap          = TypeDecorsMapPrefix + "%v" + TypeDecorsMapSeparator + "%v"
	TypeDecorsMapPrefix    = "map["
	TypeDecorsMapSeparator = "]"
)

// newMapTypeName creates a new map type name from an element type.
func newMapTypeName(key Type, elem Type) tokens.TypeName {
	return tokens.TypeName(fmt.Sprintf(TypeDecorsMap, key.Name(), elem.Name()))
}

// newMapTypeToken creates a new map type token from an element type.
func newMapTypeToken(key Type, elem Type) tokens.Type {
	return tokens.Type(fmt.Sprintf(TypeDecorsMap, key.Token(), elem.Token()))
}

// FunctionType is an invocable type, representing a signature with optional parameters and a return type.
type FunctionType struct {
	Nm         tokens.TypeName
	Tok        tokens.Type
	Parameters []Type // an array of optional parameter types.
	Return     Type   // a return type, or nil if "void".
}

var _ Symbol = (*FunctionType)(nil)
var _ Type = (*FunctionType)(nil)

func (node *FunctionType) symbol()                 {}
func (node *FunctionType) Name() tokens.Name       { return tokens.Name(node.Nm) }
func (node *FunctionType) Token() tokens.Token     { return tokens.Token(node.Tok) }
func (node *FunctionType) Tree() diag.Diagable     { return nil }
func (node *FunctionType) typesym()                {}
func (node *FunctionType) Members() ClassMemberMap { return noClassMembers }
func (node *FunctionType) Record() bool            { return false }
func (node *FunctionType) Interface() bool         { return false }
func (node *FunctionType) String() string          { return string(node.Name()) }

func NewFunctionType(params []Type, ret Type) *FunctionType {
	nm := newFunctionTypeName(params, ret)
	tok := newFunctionTypeToken(params, ret)
	return &FunctionType{nm, tok, params, ret}
}

const (
	TypeDecorsFunction          = TypeDecorsFunctionPrefix + "%v" + TypeDecorsFunctionSeparator + "%v"
	TypeDecorsFunctionPrefix    = "("
	TypeDecorsFunctionParamSep  = ","
	TypeDecorsFunctionSeparator = ")"
)

// newFunctionTypeName creates a new function type token from parameter and return types.
func newFunctionTypeName(params []Type, ret Type) tokens.TypeName {
	// Stringify the parameters (if any).
	sparams := ""
	for i, param := range params {
		if i > 0 {
			sparams += TypeDecorsFunctionParamSep
		}
		sparams += string(param.Name())
	}

	// Stringify the return type (if any).
	sret := ""
	if ret != nil {
		sret = string(ret.Name())
	}

	return tokens.TypeName(fmt.Sprintf(TypeDecorsFunction, sparams, sret))
}

// newFunctionTypeToken creates a new function type token from parameter and return types.
func newFunctionTypeToken(params []Type, ret Type) tokens.Type {
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
