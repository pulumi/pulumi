// Copyright 2016-2017, Pulumi Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package symbols

import (
	"sync"

	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/tokens"
)

// Type is a type symbol that can be used for typechecking operations.
type Type interface {
	Symbol
	Base() Type                  // the optional base type.
	TypeName() tokens.TypeName   // this type's name identifier.
	TypeToken() tokens.Type      // this type's (qualified) token.
	TypeMembers() ClassMemberMap // this type's members.
	Ctor() Function              // this type's constructor (or nil if none).
	Record() bool                // true if this is a record type.
	Interface() bool             // true if this is an interface type.
	Computed() bool              // true if this is a "latent" type (triggering deferred evaluation).
	HasValue() bool              // true if this kind of type carries a concrete value.
}

// Types is a list of type symbols.
type Types []Type

// These are some type caches, keyed by token, to avoid creating superfluous type symbol objects.
var (
	pointerTypeCache   = make(map[tokens.Type]*PointerType)
	computedTypeCache  = make(map[tokens.Type]*ComputedType)
	arrayTypeCache     = make(map[tokens.Type]*ArrayType)
	mapTypeCache       = make(map[tokens.Type]*MapType)
	moduleTypeCache    = make(map[*Module]*ModuleType)
	functionTypeCache  = make(map[tokens.Type]*FunctionType)
	prototypeTypeCache = make(map[Type]*PrototypeType)
)

// typeCacheLock synchronizes all read and update access to the type caches.
// IDEA: eventually, we should make these associated with the compiler context, rather than being global.
var typeCacheLock sync.Locker = &sync.Mutex{}

// PrimitiveType is an internal representation of a primitive type symbol (any, bool, number, string).
type PrimitiveType struct {
	Nm   tokens.TypeName
	Null bool
}

var _ Symbol = (*PrimitiveType)(nil)
var _ Type = (*PrimitiveType)(nil)

func (node *PrimitiveType) Name() tokens.Name           { return tokens.Name(node.Nm) }
func (node *PrimitiveType) Token() tokens.Token         { return tokens.Token(node.Nm) }
func (node *PrimitiveType) Special() bool               { return false }
func (node *PrimitiveType) Tree() diag.Diagable         { return nil }
func (node *PrimitiveType) Base() Type                  { return nil }
func (node *PrimitiveType) TypeName() tokens.TypeName   { return node.Nm }
func (node *PrimitiveType) TypeToken() tokens.Type      { return tokens.Type(node.Nm) }
func (node *PrimitiveType) TypeMembers() ClassMemberMap { return noClassMembers }
func (node *PrimitiveType) Ctor() Function              { return nil }
func (node *PrimitiveType) Record() bool                { return false }
func (node *PrimitiveType) Interface() bool             { return false }
func (node *PrimitiveType) Computed() bool              { return false }
func (node *PrimitiveType) HasValue() bool              { return !node.Null }
func (node *PrimitiveType) String() string              { return string(node.Token()) }

func NewPrimitiveType(nm tokens.TypeName, null bool) *PrimitiveType {
	return &PrimitiveType{
		Nm:   nm,
		Null: null,
	}
}

// PointerType represents a pointer to any other type.
type PointerType struct {
	Nm      tokens.TypeName
	Tok     tokens.Type
	Element Type
}

var _ Symbol = (*PointerType)(nil)
var _ Type = (*PointerType)(nil)

func (node *PointerType) Name() tokens.Name           { return tokens.Name(node.Nm) }
func (node *PointerType) Token() tokens.Token         { return tokens.Token(node.Tok) }
func (node *PointerType) Special() bool               { return false }
func (node *PointerType) Tree() diag.Diagable         { return nil }
func (node *PointerType) Base() Type                  { return nil }
func (node *PointerType) TypeName() tokens.TypeName   { return node.Nm }
func (node *PointerType) TypeToken() tokens.Type      { return node.Tok }
func (node *PointerType) TypeMembers() ClassMemberMap { return noClassMembers }
func (node *PointerType) Ctor() Function              { return nil }
func (node *PointerType) Record() bool                { return false }
func (node *PointerType) Interface() bool             { return false }
func (node *PointerType) Computed() bool              { return false }
func (node *PointerType) HasValue() bool              { return true }
func (node *PointerType) String() string              { return string(node.Token()) }

// NewPointerType returns an existing type symbol from the cache, if one exists, or allocates a new one otherwise.
func NewPointerType(elem Type) *PointerType {
	tok := tokens.NewPointerTypeToken(elem.TypeToken())

	typeCacheLock.Lock()
	defer typeCacheLock.Unlock()
	if ptr, has := pointerTypeCache[tok]; has {
		return ptr
	}

	nm := tokens.NewPointerTypeName(elem.TypeName())
	ptr := &PointerType{nm, tok, elem}
	pointerTypeCache[tok] = ptr
	return ptr
}

// ComputedType is a wrapper over an ordinary type that indicates a particular expression's value is not yet known and
// that it will remain unknown until some future condition is met.  In many cases, the interpreter can speculate beyond
// a computed value, producing even more derived computed values.  Eventually, of course, the real value must be known
// in order to proceed (e.g., for conditionals), however even in these cases, the interpreter may choose to proceed.
type ComputedType struct {
	Element Type // the real underlying type.
}

var _ Symbol = (*ComputedType)(nil)
var _ Type = (*ComputedType)(nil)

func (node *ComputedType) Name() tokens.Name {
	return tokens.Name(string(node.Element.Name())) + ".computed"
}
func (node *ComputedType) Token() tokens.Token {
	return tokens.Token(string(node.Element.Token())) + ".computed"
}
func (node *ComputedType) Special() bool               { return false }
func (node *ComputedType) Tree() diag.Diagable         { return nil }
func (node *ComputedType) Base() Type                  { return nil }
func (node *ComputedType) TypeName() tokens.TypeName   { return tokens.TypeName(node.Name()) }
func (node *ComputedType) TypeToken() tokens.Type      { return tokens.Type(node.Token()) }
func (node *ComputedType) TypeMembers() ClassMemberMap { return noClassMembers }
func (node *ComputedType) Ctor() Function              { return nil }
func (node *ComputedType) Record() bool                { return false }
func (node *ComputedType) Interface() bool             { return false }
func (node *ComputedType) Computed() bool              { return true }
func (node *ComputedType) HasValue() bool              { return false }
func (node *ComputedType) String() string              { return string(node.Token()) }

// NewComputedType returns an existing type symbol from the cache, if one exists, or allocates a new one otherwise.
func NewComputedType(elem Type) *ComputedType {
	tok := elem.TypeToken()

	typeCacheLock.Lock()
	defer typeCacheLock.Unlock()
	if ev, has := computedTypeCache[tok]; has {
		return ev
	}

	ev := &ComputedType{Element: elem}
	computedTypeCache[tok] = ev
	return ev
}

// ArrayType is an array whose elements are of some other type.
type ArrayType struct {
	Nm      tokens.TypeName
	Tok     tokens.Type
	Element Type
}

var _ Symbol = (*ArrayType)(nil)
var _ Type = (*ArrayType)(nil)

func (node *ArrayType) Name() tokens.Name           { return tokens.Name(node.Nm) }
func (node *ArrayType) Token() tokens.Token         { return tokens.Token(node.Tok) }
func (node *ArrayType) Special() bool               { return false }
func (node *ArrayType) Tree() diag.Diagable         { return nil }
func (node *ArrayType) Base() Type                  { return nil }
func (node *ArrayType) TypeName() tokens.TypeName   { return node.Nm }
func (node *ArrayType) TypeToken() tokens.Type      { return node.Tok }
func (node *ArrayType) TypeMembers() ClassMemberMap { return noClassMembers }
func (node *ArrayType) Ctor() Function              { return nil }
func (node *ArrayType) Record() bool                { return false }
func (node *ArrayType) Interface() bool             { return false }
func (node *ArrayType) Computed() bool              { return false }
func (node *ArrayType) HasValue() bool              { return true }
func (node *ArrayType) String() string              { return string(node.Token()) }

// NewArrayType returns an existing type symbol from the cache, if one exists, or allocates a new one otherwise.
func NewArrayType(elem Type) *ArrayType {
	tok := tokens.NewArrayTypeToken(elem.TypeToken())

	typeCacheLock.Lock()
	defer typeCacheLock.Unlock()
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

func (node *MapType) Name() tokens.Name           { return tokens.Name(node.Nm) }
func (node *MapType) Token() tokens.Token         { return tokens.Token(node.Tok) }
func (node *MapType) Special() bool               { return false }
func (node *MapType) Tree() diag.Diagable         { return nil }
func (node *MapType) Base() Type                  { return nil }
func (node *MapType) TypeName() tokens.TypeName   { return node.Nm }
func (node *MapType) TypeToken() tokens.Type      { return node.Tok }
func (node *MapType) TypeMembers() ClassMemberMap { return noClassMembers }
func (node *MapType) Ctor() Function              { return nil }
func (node *MapType) Record() bool                { return false }
func (node *MapType) Interface() bool             { return false }
func (node *MapType) Computed() bool              { return false }
func (node *MapType) HasValue() bool              { return true }
func (node *MapType) String() string              { return string(node.Token()) }

// NewMapType returns an existing type symbol from the cache, if one exists, or allocates a new one otherwise.
func NewMapType(key Type, elem Type) *MapType {
	tok := tokens.NewMapTypeToken(key.TypeToken(), elem.TypeToken())

	typeCacheLock.Lock()
	defer typeCacheLock.Unlock()
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

func (node *FunctionType) Name() tokens.Name           { return tokens.Name(node.Nm) }
func (node *FunctionType) Token() tokens.Token         { return tokens.Token(node.Tok) }
func (node *FunctionType) Special() bool               { return false }
func (node *FunctionType) Tree() diag.Diagable         { return nil }
func (node *FunctionType) Base() Type                  { return nil }
func (node *FunctionType) TypeName() tokens.TypeName   { return node.Nm }
func (node *FunctionType) TypeToken() tokens.Type      { return node.Tok }
func (node *FunctionType) TypeMembers() ClassMemberMap { return noClassMembers }
func (node *FunctionType) Ctor() Function              { return nil }
func (node *FunctionType) Record() bool                { return false }
func (node *FunctionType) Interface() bool             { return false }
func (node *FunctionType) Computed() bool              { return false }
func (node *FunctionType) HasValue() bool              { return true }
func (node *FunctionType) String() string              { return string(node.Token()) }

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

	typeCacheLock.Lock()
	defer typeCacheLock.Unlock()
	if fnc, has := functionTypeCache[tok]; has {
		return fnc
	}

	nm := tokens.NewFunctionTypeName(paramsn, retn)
	fnc := &FunctionType{nm, tok, params, ret}
	functionTypeCache[tok] = fnc
	return fnc
}

// ModuleType is a runtime representation of a module as an object.
type ModuleType struct {
	Module *Module // the module this object covers.
}

var _ Symbol = (*ModuleType)(nil)
var _ Type = (*ModuleType)(nil)

func (node *ModuleType) Name() tokens.Name   { return tokens.Name(node.TypeName()) }
func (node *ModuleType) Token() tokens.Token { return tokens.Token(node.TypeToken()) }
func (node *ModuleType) Special() bool       { return false }
func (node *ModuleType) Tree() diag.Diagable { return nil }
func (node *ModuleType) Base() Type          { return nil }
func (node *ModuleType) TypeName() tokens.TypeName {
	return tokens.TypeName(string(node.Module.Name())) + ".modtype"
}
func (node *ModuleType) TypeToken() tokens.Type {
	return tokens.Type(string(node.Module.Token()) + ".modtype")
}
func (node *ModuleType) TypeMembers() ClassMemberMap { return noClassMembers }
func (node *ModuleType) Ctor() Function              { return nil }
func (node *ModuleType) Record() bool                { return false }
func (node *ModuleType) Interface() bool             { return false }
func (node *ModuleType) Computed() bool              { return false }
func (node *ModuleType) HasValue() bool              { return true }
func (node *ModuleType) String() string              { return string(node.Token()) }

func NewModuleType(m *Module) *ModuleType {
	typeCacheLock.Lock()
	defer typeCacheLock.Unlock()
	if mtype, has := moduleTypeCache[m]; has {
		return mtype
	}
	mtype := &ModuleType{m}
	moduleTypeCache[m] = mtype
	return mtype
}

// PrototypeType is the type for "prototypes" (blueprints for object construction).
type PrototypeType struct {
	Type Type // the type this prototype constructs.
}

var _ Symbol = (*PrototypeType)(nil)
var _ Type = (*PrototypeType)(nil)

func (node *PrototypeType) Name() tokens.Name   { return tokens.Name(node.TypeName()) }
func (node *PrototypeType) Token() tokens.Token { return tokens.Token(node.TypeToken()) }
func (node *PrototypeType) Special() bool       { return false }
func (node *PrototypeType) Tree() diag.Diagable { return nil }
func (node *PrototypeType) Base() Type          { return nil }
func (node *PrototypeType) TypeName() tokens.TypeName {
	return tokens.TypeName(string(node.Type.Name())) + ".prototype"
}
func (node *PrototypeType) TypeToken() tokens.Type {
	return tokens.Type(string(node.Type.Token()) + ".prototype")
}
func (node *PrototypeType) TypeMembers() ClassMemberMap { return noClassMembers }
func (node *PrototypeType) Ctor() Function              { return nil }
func (node *PrototypeType) Record() bool                { return false }
func (node *PrototypeType) Interface() bool             { return false }
func (node *PrototypeType) Computed() bool              { return false }
func (node *PrototypeType) HasValue() bool              { return true }
func (node *PrototypeType) String() string              { return string(node.Token()) }

func NewPrototypeType(t Type) *PrototypeType {
	typeCacheLock.Lock()
	defer typeCacheLock.Unlock()
	if proto, has := prototypeTypeCache[t]; has {
		return proto
	}
	proto := &PrototypeType{t}
	prototypeTypeCache[t] = proto
	return proto
}
