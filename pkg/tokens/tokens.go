// Copyright 2016 Marapongo, Inc. All rights reserved.

// This package contains the core MuIL symbol and token types.
package tokens

import (
	"strings"

	"github.com/marapongo/mu/pkg/util/contract"
)

// Token is a qualified name that is capable of resolving to a symbol entirely on its own.  Most uses of tokens are
// typed based on the context, so that a subset of the token syntax is permissable (see the various typedefs below).
// However, in its full generality, a token can have a package part, a module part, a module-member part, and a
// class-member part.  Obviously tokens that are meant to address just a module won't have the module-member part, and
// tokens addressing module members won't have the class-member part, etc.
//
// Token's grammar is as follows:
//
//		Token				= <Identifier> |
//							  <QualifiedToken> |
//							  <DecoratedType>
//		Identifier			= <Name>
//		QualifiedToken		= <PackageName> [ ":" <ModuleName> [ "/" <ModuleMemberName> [ "." <ClassMemberName> ] ] ]
//		PackageName			= <QName>
//		ModuleName			= <QName>
//		ModuleMemberName	= <Name>
//		ClassMemberName		= <Name>
//
// A token may be a simple identifier in the case that it refers to a built-in symbol, like a primitive type, or a
// variable in scope, rather than a qualified token that is to be bound to a symbol through package/module resolution.
//
// Notice that both package and module names may be qualified names (meaning they can have "/" delimiters; see QName's
// comments), and that module and class members must use unqualified, simple names (meaning they have no delimiters).
// The specialized token kinds differ only in what elements they require as part of the token string.
//
// Finally, a token may also be a decorated type.  This is for built-in array, map, pointer, and function types:
//
//		DecoratedType		= "*" <Token> |
//							  "[]" <Token> |
//							  "map[" <Token> "]" <Token> |
//							  "(" [ <Token> [ "," <Token> ]* ] ")" <Token>?
//
// Notice that a recursive parsing process is required to extract elements from a <DecoratedType> token.
type Token string

func (tok Token) String() string { return string(tok) }

const ModuleDelimiter string = ":"       // the character following a package (before a module).
const ModuleMemberDelimiter string = "/" // the character following a module (before a module member).
const ClassMemberDelimiter string = "."  // the character following a class name (before a class member).

func (tok Token) Simple() bool    { return !tok.HasModule() }
func (tok Token) HasModule() bool { return strings.Index(string(tok), ModuleDelimiter) != -1 }
func (tok Token) HasModuleMember() bool {
	return strings.Index(string(tok), ModuleMemberDelimiter) != -1
}
func (tok Token) HasClassMember() bool { return strings.Index(string(tok), ClassMemberDelimiter) != -1 }

// Name returns the Token as a Name (and assumes it is a legal one).
func (tok Token) Name() Name {
	contract.Requiref(tok.Simple(), "tok", "Simple")
	contract.Requiref(IsName(tok.String()), "tok", "IsName")
	return Name(tok.String())
}

// Parts returns as many parts as the Token has, each in a return slot, or "" for those that don't exist.
func (tok Token) Parts() (PackageName, ModuleName, ModuleMemberName, ClassMemberName) {
	// Switch on the token kind to populate the set of names.
	var pkg PackageName
	var mod ModuleName
	var mem ModuleMemberName
	var clm ClassMemberName
	if tok.HasClassMember() {
		clmtok := ClassMember(tok)
		pkg = clmtok.Package().Name()
		mod = clmtok.Module().Name()
		mem = ModuleMemberName(clmtok.Class().Name())
		clm = clmtok.Name()
	} else if tok.HasModuleMember() {
		memtok := ModuleMember(tok)
		pkg = memtok.Package().Name()
		mod = memtok.Module().Name()
		mem = memtok.Name()
	} else if tok.HasModule() {
		modtok := Module(tok)
		pkg = modtok.Package().Name()
		mod = modtok.Name()
	} else {
		pkg = Package(tok).Name()
	}
	contract.Assert(pkg != "")
	return pkg, mod, mem, clm
}

// Package is a token representing just a package.  It uses a much simpler grammar:
//		Package = <PackageName>
// Note that a package name of "." means "current package", to simplify emission and lookups.
type Package Token

func NewPackageToken(nm PackageName) Package {
	contract.Assert(IsQName(string(nm)))
	return Package(nm)
}

func (tok Package) Name() PackageName {
	return PackageName(tok)
}

func (tok Package) String() string { return string(tok) }

// PackageName is a qualified name referring to an imported package.
type PackageName QName

func (nm PackageName) String() string { return string(nm) }

// Module is a token representing a module.  It uses the following subset of the token grammar:
//		Module = <Package> ":" <ModuleName>
// Note that a module name of "." means "current module", to simplify emission and lookups.
type Module Token

func NewModuleToken(pkg Package, nm ModuleName) Module {
	contract.Assert(IsQName(string(nm)))
	return Module(string(pkg) + ModuleDelimiter + string(nm))
}

func (tok Module) Package() Package {
	ix := strings.LastIndex(string(tok), ModuleDelimiter)
	contract.Assert(ix != -1)
	return Package(string(tok)[:ix])
}

func (tok Module) Name() ModuleName {
	ix := strings.LastIndex(string(tok), ModuleDelimiter)
	contract.Assert(ix != -1)
	return ModuleName(string(tok)[ix+1:])
}

func (tok Module) String() string { return string(tok) }

// ModuleName is a qualified name referring to an imported module from a package.
type ModuleName QName

func (nm ModuleName) String() string { return string(nm) }

// ModuleMember is a token representing a module's member.  It uses the following grammar.  Note that this is not
// ambiguous because member names cannot contain slashes, and so the "last" slash in a name delimits the member:
//		ModuleMember = <Module> "/" <ModuleMemberName>
type ModuleMember Token

func NewModuleMemberToken(mod Module, nm ModuleMemberName) ModuleMember {
	contract.Assert(IsName(string(nm)))
	return ModuleMember(string(mod) + ModuleMemberDelimiter + string(nm))
}

func (tok ModuleMember) Package() Package {
	return tok.Module().Package()
}

func (tok ModuleMember) Module() Module {
	ix := strings.LastIndex(string(tok), ModuleMemberDelimiter)
	contract.Assert(ix != -1)
	return Module(string(tok)[:ix])
}

func (tok ModuleMember) Name() ModuleMemberName {
	ix := strings.LastIndex(string(tok), ModuleMemberDelimiter)
	contract.Assert(ix != -1)
	return ModuleMemberName(string(tok)[ix+1:])
}

func (tok ModuleMember) String() string { return string(tok) }

// ModuleMemberName is a simple name representing the module member's identifier.
type ModuleMemberName Name

func (nm ModuleMemberName) String() string { return string(nm) }

// ClassMember is a token representing a class's member.  It uses the following grammar.  Unlike ModuleMember, this
// cannot use a slash for delimiting names, because we use often ClassMember and ModuleMember interchangably:
//		ClassMember = <ModuleMember> "." <ClassMemberName>
type ClassMember Token

func NewClassMemberToken(class Type, nm ClassMemberName) ClassMember {
	contract.Assert(IsName(string(nm)))
	return ClassMember(string(class) + ClassMemberDelimiter + string(nm))
}

func (tok ClassMember) Package() Package {
	return tok.Module().Package()
}

func (tok ClassMember) Module() Module {
	return tok.Class().Module()
}

func (tok ClassMember) Class() Type {
	ix := strings.LastIndex(string(tok), ClassMemberDelimiter)
	contract.Assert(ix != -1)
	return Type(string(tok)[:ix])
}

func (tok ClassMember) Name() ClassMemberName {
	ix := strings.LastIndex(string(tok), ClassMemberDelimiter)
	contract.Assert(ix != -1)
	return ClassMemberName(string(tok)[ix+1:])
}

func (tok ClassMember) String() string { return string(tok) }

// ClassMemberName is a simple name representing the class member's identifier.
type ClassMemberName Name

func (nm ClassMemberName) Name() Name     { return Name(nm) }
func (nm ClassMemberName) String() string { return string(nm) }

// Type is a token representing a type.  It is either a primitive type name, reference to a module class, or decorated:
//		Type = <Name> | <ModuleMember> | <DecoratedType>
type Type Token

func NewTypeToken(mod Module, nm TypeName) Type {
	contract.Assert(IsName(string(nm)))
	return Type(string(mod) + ModuleMemberDelimiter + string(nm))
}

func (tok Type) Package() Package {
	if tok.Primitive() || tok.Decorated() {
		return Package("")
	} else {
		return ModuleMember(tok).Package()
	}
}

func (tok Type) Module() Module {
	if tok.Primitive() || tok.Decorated() {
		return Module("")
	} else {
		return ModuleMember(tok).Module()
	}
}

func (tok Type) Name() TypeName {
	if tok.Primitive() || tok.Decorated() {
		return TypeName(tok)
	} else {
		return TypeName(ModuleMember(tok).Name())
	}
}

func (tok Type) Member() ModuleMember {
	return ModuleMember(tok)
}

// Decorated indicates whether this token represents a decorated type.
func (tok Type) Decorated() bool {
	return tok.Pointer() || tok.Array() || tok.Map() || tok.Function()
}

func (tok Type) Pointer() bool  { return IsPointerType(tok) }
func (tok Type) Array() bool    { return IsArrayType(tok) }
func (tok Type) Map() bool      { return IsMapType(tok) }
func (tok Type) Function() bool { return IsFunctionType(tok) }

// Primitive indicates whether this type is a primitive type name (i.e., not qualified with a module, etc).
func (tok Type) Primitive() bool {
	return !tok.Decorated() && !Token(tok).HasModule()
}

func (tok Type) String() string { return string(tok) }

// TypeName is a simple name representing the type's name, without any package/module qualifiers.
type TypeName Name

func (nm TypeName) String() string { return string(nm) }

// Variable is a token representing a variable (module property, class property, or local variable (including
// parameters)).  It can be a simple name for the local cases, or a true token for others:
//		Variable = <Name> | <ModuleMember> | <ClassMember>
type Variable Token

func (tok Variable) String() string { return string(tok) }

// Function is a token representing a variable (module method or class method).  Its grammar is as follows:
//		Variable = <ModuleMember> | <ClassMember>
type Function Token

func (tok Function) String() string { return string(tok) }
