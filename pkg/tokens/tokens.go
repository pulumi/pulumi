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
//		Token				= <Name> |
//							= <PackageName> [ ":" <ModuleName> [ "/" <ModuleMemberName> [ "." <ClassMemberName> ] ] ]
//		PackageName			= <QName>
//		ModuleName			= <QName>
//		ModuleMemberName	= <Name>
//		ClassMemberName		= <Name>
//
// A token may be a simple name in the case that it refers to a built-in symbol, like a primitive type, or when it
// refers to an identifier that is in scope, rather than a symbol that is to be bound through package/module resolution.
//
// Notice that both package and module names may be qualified names (meaning they can have "/" delimiters; see QName's
// comments), and that module and class members must use unqualified, simple names (meaning they have no delimiters).
// The specialized token kinds differ only in what elements they require as part of the token string.
type Token string

func (tok Token) String() string { return string(tok) }

const ModuleDelimiter string = ":"       // the character following a package (before a module).
const ModuleMemberDelimiter string = "/" // the character following a module (before a module member).
const ClassMemberDelimiter string = "."  // the character following a class name (before a class member).

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

func (nm ClassMemberName) String() string { return string(nm) }

// Type is a token representing a type.  It is either a primitive type name, or a reference to a module class:
//		Type = <Name> | <ModuleMember>
type Type Token

func NewTypeToken(mod Module, nm TypeName) Type {
	contract.Assert(IsName(string(nm)))
	return Type(string(mod) + ModuleMemberDelimiter + string(nm))
}

func (tok Type) Package() Package {
	if tok.Primitive() {
		return Package("")
	} else {
		return ModuleMember(tok).Package()
	}
}

func (tok Type) Module() Module {
	if tok.Primitive() {
		return Module("")
	} else {
		return ModuleMember(tok).Module()
	}
}

func (tok Type) Name() TypeName {
	if tok.Primitive() {
		contract.Assert(IsName(string(tok)))
		return TypeName(tok)
	} else {
		return TypeName(ModuleMember(tok).Name())
	}
}

func (tok Type) Member() ModuleMember {
	return ModuleMember(tok)
}

// Primitive indicates whether this type is a primitive type name (i.e., not qualified with a module, etc).
func (tok Type) Primitive() bool {
	return strings.LastIndex(string(tok), ModuleMemberDelimiter) == -1
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
