// Copyright 2016 Marapongo, Inc. All rights reserved.

// This package contains the core MuIL symbol and token types.
package tokens

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

const ModuleDelimiter string = ":"       // the character following a package (before a module).
const ModuleMemberDelimiter string = "/" // the character following a module (before a module member).
const ClassMemberDelimiter string = "."  // the character following a class name (before a class member).

// Package is a token representing just a package.  It uses a much simpler grammar:
//		Package = <PackageName>
// Note that a package name of "." means "current package", to simplify emission and lookups.
type Package Token

// PackageName is a qualified name referring to an imported package.
type PackageName QName

// Module is a token representing a module.  It uses the following subset of the token grammar:
//		Module = <Package> ":" <ModuleName>
// Note that a module name of "." means "current module", to simplify emission and lookups.
type Module Token

// ModuleName is a qualified name referring to an imported module from a package.
type ModuleName QName

// ModuleMember is a token representing a module's member.  It uses the following grammar.  Note that this is not
// ambiguous because member names cannot contain slashes, and so the "last" slash in a name delimits the member:
//		ModuleMember = <Module> "/" <ModuleMemberName>
type ModuleMember Token

// ModuleMemberName is a simple name representing the module member's identifier.
type ModuleMemberName Name

// ClassMember is a token representing a class's member.  It uses the following grammar.  Unlike ModuleMember, this
// cannot use a slash for delimiting names, because we use often ClassMember and ModuleMember interchangably:
//		ClassMember = <ModuleMember "." <ClassMemberName>
type ClassMember Token

// ClassMemberName is a simple name representing the class member's identifier.
type ClassMemberName Name

// Type is a token representing a type.  It is either a primitive type name, or a reference to a module class:
//		Type = <Name> | <ModuleMember>
type Type Token

// Variable is a token representing a variable (module property, class property, or local variable (including
// parameters)).  It can be a simple name for the local cases, or a true token for others:
//		Variable = <Name> | <ModuleMember> | <ClassMember>
type Variable Token

// Function is a token representing a variable (module method or class method).  Its grammar is as follows:
//		Variable = <ModuleMember> | <ClassMember>
type Function Token

// NewModuleToken produces a new qualified token from a parent package and the module.
func NewModuleToken(parent Package, name ModuleName) Module {
	return Module(string(parent) + ModuleDelimiter + string(name))
}

// NewModuleMemberToken produces a new qualified token from a parent module and the member's identifier.
func NewModuleMemberToken(parent Module, name ModuleMemberName) ModuleMember {
	return ModuleMember(string(parent) + ModuleMemberDelimiter + string(name))
}

// NewClassMemberToken produces a new qualified token from a parent class and the class's identifier.
func NewClassMemberToken(parent ModuleMember, name ClassMemberName) ClassMember {
	return ClassMember(string(parent) + ClassMemberDelimiter + string(name))
}
