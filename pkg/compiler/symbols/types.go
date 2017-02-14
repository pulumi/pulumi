// Copyright 2016 Marapongo, Inc. All rights reserved.

package symbols

import (
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/tokens"
)

// Type is a type symbol that can be used for typechecking operations.
type Type interface {
	Symbol
	typesym()
	Base() Type                  // the optional base type.
	TypeName() tokens.TypeName   // this type's name identifier.
	TypeToken() tokens.Type      // this type's (qualified) token.
	TypeMembers() ClassMemberMap // this type's members.
	Record() bool                // true if this is a record type.
	Interface() bool             // true if this is an interface type.
}

// Types is a list of type symbols.
type Types []Type

// PrimitiveType is an internal representation of a primitive type symbol (any, bool, number, string).
type PrimitiveType struct {
	Nm tokens.TypeName
}

var _ Symbol = (*PrimitiveType)(nil)
var _ Type = (*PrimitiveType)(nil)

func (node *PrimitiveType) symbol()                     {}
func (node *PrimitiveType) Name() tokens.Name           { return tokens.Name(node.Nm) }
func (node *PrimitiveType) Token() tokens.Token         { return tokens.Token(node.Nm) }
func (node *PrimitiveType) Tree() diag.Diagable         { return nil }
func (node *PrimitiveType) typesym()                    {}
func (node *PrimitiveType) Base() Type                  { return nil }
func (node *PrimitiveType) TypeName() tokens.TypeName   { return node.Nm }
func (node *PrimitiveType) TypeToken() tokens.Type      { return tokens.Type(node.Nm) }
func (node *PrimitiveType) TypeMembers() ClassMemberMap { return noClassMembers }
func (node *PrimitiveType) Record() bool                { return false }
func (node *PrimitiveType) Interface() bool             { return false }
func (node *PrimitiveType) String() string              { return string(node.Name()) }

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

func (node *PointerType) symbol()                     {}
func (node *PointerType) Name() tokens.Name           { return tokens.Name(node.Nm) }
func (node *PointerType) Token() tokens.Token         { return tokens.Token(node.Tok) }
func (node *PointerType) Tree() diag.Diagable         { return nil }
func (node *PointerType) typesym()                    {}
func (node *PointerType) Base() Type                  { return nil }
func (node *PointerType) TypeName() tokens.TypeName   { return node.Nm }
func (node *PointerType) TypeToken() tokens.Type      { return node.Tok }
func (node *PointerType) TypeMembers() ClassMemberMap { return noClassMembers }
func (node *PointerType) Record() bool                { return false }
func (node *PointerType) Interface() bool             { return false }
func (node *PointerType) String() string              { return string(node.Name()) }

// pointerTypeCache is a cache keyed by token, helping to avoid creating superfluous symbol objects.
var pointerTypeCache = make(map[tokens.Type]*PointerType)

// NewPointerType returns an existing type symbol from the cache, if one exists, or allocates a new one otherwise.
func NewPointerType(elem Type) *PointerType {
	tok := tokens.NewPointerTypeToken(elem.TypeToken())
	if ptr, has := pointerTypeCache[tok]; has {
		return ptr
	}

	nm := tokens.NewPointerTypeName(elem.TypeName())
	ptr := &PointerType{nm, tok, elem}
	pointerTypeCache[tok] = ptr
	return ptr
}

// ArrayType is an array whose elements are of some other type.
type ArrayType struct {
	Nm      tokens.TypeName
	Tok     tokens.Type
	Element Type
}

var _ Symbol = (*ArrayType)(nil)
var _ Type = (*ArrayType)(nil)

func (node *ArrayType) symbol()                     {}
func (node *ArrayType) Name() tokens.Name           { return tokens.Name(node.Nm) }
func (node *ArrayType) Token() tokens.Token         { return tokens.Token(node.Tok) }
func (node *ArrayType) Tree() diag.Diagable         { return nil }
func (node *ArrayType) typesym()                    {}
func (node *ArrayType) Base() Type                  { return nil }
func (node *ArrayType) TypeName() tokens.TypeName   { return node.Nm }
func (node *ArrayType) TypeToken() tokens.Type      { return node.Tok }
func (node *ArrayType) TypeMembers() ClassMemberMap { return noClassMembers }
func (node *ArrayType) Record() bool                { return false }
func (node *ArrayType) Interface() bool             { return false }
func (node *ArrayType) String() string              { return string(node.Name()) }

// arrayTypeCache is a cache keyed by token, helping to avoid creating superfluous symbol objects.
var arrayTypeCache = make(map[tokens.Type]*ArrayType)

// NewArrayType returns an existing type symbol from the cache, if one exists, or allocates a new one otherwise.
func NewArrayType(elem Type) *ArrayType {
	tok := tokens.NewArrayTypeToken(elem.TypeToken())
	if arr, has := arrayTypeCache[tok]; has {
		return arr
	}

	nm := tokens.NewArrayTypeName(elem.TypeName())
	arr := &ArrayType{nm, tok, elem}
	arrayTypeCache[tok] = arr
	return arr
}

// MapType is an array whose keys and elements are of some other types.
type MapType struct {
	Nm      tokens.TypeName
	Tok     tokens.Type
	Key     Type
	Element Type
}

var _ Symbol = (*MapType)(nil)
var _ Type = (*MapType)(nil)

func (node *MapType) symbol()                     {}
func (node *MapType) Name() tokens.Name           { return tokens.Name(node.Nm) }
func (node *MapType) Token() tokens.Token         { return tokens.Token(node.Tok) }
func (node *MapType) Tree() diag.Diagable         { return nil }
func (node *MapType) typesym()                    {}
func (node *MapType) Base() Type                  { return nil }
func (node *MapType) TypeName() tokens.TypeName   { return node.Nm }
func (node *MapType) TypeToken() tokens.Type      { return node.Tok }
func (node *MapType) TypeMembers() ClassMemberMap { return noClassMembers }
func (node *MapType) Record() bool                { return false }
func (node *MapType) Interface() bool             { return false }
func (node *MapType) String() string              { return string(node.Name()) }

// mapTypeCache is a cache keyed by token, helping to avoid creating superfluous symbol objects.
var mapTypeCache = make(map[tokens.Type]*MapType)

// NewMapType returns an existing type symbol from the cache, if one exists, or allocates a new one otherwise.
func NewMapType(key Type, elem Type) *MapType {
	tok := tokens.NewMapTypeToken(key.TypeToken(), elem.TypeToken())
	if mam, has := mapTypeCache[tok]; has {
		return mam
	}

	nm := tokens.NewMapTypeName(key.TypeName(), elem.TypeName())
	mam := &MapType{nm, tok, key, elem}
	mapTypeCache[tok] = mam
	return mam
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

func (node *FunctionType) symbol()                     {}
func (node *FunctionType) Name() tokens.Name           { return tokens.Name(node.Nm) }
func (node *FunctionType) Token() tokens.Token         { return tokens.Token(node.Tok) }
func (node *FunctionType) Tree() diag.Diagable         { return nil }
func (node *FunctionType) typesym()                    {}
func (node *FunctionType) Base() Type                  { return nil }
func (node *FunctionType) TypeName() tokens.TypeName   { return node.Nm }
func (node *FunctionType) TypeToken() tokens.Type      { return node.Tok }
func (node *FunctionType) TypeMembers() ClassMemberMap { return noClassMembers }
func (node *FunctionType) Record() bool                { return false }
func (node *FunctionType) Interface() bool             { return false }
func (node *FunctionType) String() string              { return string(node.Name()) }

// functionTypeCache is a cache keyed by token, helping to avoid creating superfluous symbol objects.
var functionTypeCache = make(map[tokens.Type]*FunctionType)

// NewFunctionType returns an existing type symbol from the cache, if one exists, or allocates a new one otherwise.
func NewFunctionType(params []Type, ret Type) *FunctionType {
	var paramsn []tokens.TypeName
	var paramst []tokens.Type
	for _, param := range params {
		paramsn = append(paramsn, param.TypeName())
		paramst = append(paramst, param.TypeToken())
	}
	var retn *tokens.TypeName
	var rett *tokens.Type
	if ret != nil {
		nm := ret.TypeName()
		retn = &nm
		tok := ret.TypeToken()
		rett = &tok
	}

	tok := tokens.NewFunctionTypeToken(paramst, rett)
	if fnc, has := functionTypeCache[tok]; has {
		return fnc
	}

	nm := tokens.NewFunctionTypeName(paramsn, retn)
	fnc := &FunctionType{nm, tok, params, ret}
	functionTypeCache[tok] = fnc
	return fnc
}
