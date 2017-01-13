// Copyright 2016 Marapongo, Inc. All rights reserved.

package ast

import (
	"github.com/marapongo/mu/pkg/pack/symbols"
)

/* Definitions */

// Definition is a type system element that is possibly exported from a module for external usage.
type Definition interface {
	Node
	definition()
	GetName() *Identifier    // a required name, unique amongst definitions with a common parent.
	GetDescription() *string // an optional informative description.
}

type definition struct {
	node
	Name        *Identifier `json:"name"`
	Description *string     `json:"description,omitempty"`
}

func (node *definition) nd()                     {}
func (node *definition) definition()             {}
func (node *definition) GetName() *Identifier    { return node.Name }
func (node *definition) GetDescription() *string { return node.Description }

/* Modules */

// Module contains members, including variables, functions, and/or classes.
type Module struct {
	definition
}

// Modules is a map of ModuleToken to Module.
type Modules map[symbols.ModuleToken]*Module

// ModuleMember is a definition that belongs to a Module.
type ModuleMember interface {
	Definition
	moduleMember()
	GetAccess() *symbols.Accessibility
}

type moduleMember struct {
	definition
	Access *symbols.Accessibility `json:"access,omitempty"`
}

func (node *moduleMember) moduleMember()                     {}
func (node *moduleMember) GetAccess() *symbols.Accessibility { return node.Access }

// ModuleMembers is a map of Token to ModuleMember.
type ModuleMembers map[symbols.Token]ModuleMember

// Export re-exports a Definition from another Module, possibly with a different name.
type Export struct {
	moduleMember
	Token symbols.Token `json:"token"`
}

/* Classes */

// Class can be constructed to create an object, and exports properties, methods, and has a number of attributes.
type Class struct {
	moduleMember
	Extends    *symbols.TypeToken `json:"extends,omitempty"`
	Implements *symbols.TypeToken `json:"implements,omitempty"`
	Sealed     *bool              `json:"sealed,omitempty"`
	Abstract   *bool              `json:"abstract,omitempty"`
	Record     *bool              `json:"record,omitempty"`
	Interface  *bool              `json:"interface,omitempty"`
	Members    []ClassMember      `json:"members,omitempty"`
}

// ClassMember is a Definition that belongs to a Class.
type ClassMember interface {
	Definition
	GetAccess() *symbols.ClassMemberAccessibility
	GetStatic() *bool
}

type classMember struct {
	definition
	Access *symbols.ClassMemberAccessibility `json:"access,omitempty"`
	Static *bool                             `json:"static,omitempty"`
}

// ClassMembers is a map of Token to ClassMember.
type ClassMembers map[symbols.Token]ClassMember

/* Variables */

// Variable is a storage location with an optional type.
type Variable interface {
	Definition
	GetType() *symbols.TypeToken
	GetDefault() *interface{} // a trivially serializable default value.
	GetReadonly() *bool
}

type variable struct {
	definition
	Type     *symbols.TypeToken `json:"type,omitempty"`
	Default  *interface{}       `json:"default,omitempty"`
	Readonly *bool              `json:"readonly,omitempty"`
}

func (node *variable) GetType() *symbols.TypeToken { return node.Type }
func (node *variable) GetDefault() *interface{}    { return node.Default }
func (node *variable) GetReadonly() *bool          { return node.Readonly }

// LocalVariable is a variable that is lexically scoped within a function (either a parameter or local).
type LocalVariable struct {
	variable
}

// ModuleProperty is like a variable but belongs to a module.
type ModuleProperty struct {
	variable
	moduleMember
}

// ClassProperty is like a module property with some extra attributes.
type ClassProperty struct {
	variable
	classMember
	Primary *bool `json:"primary,omitempty"`
}

/* Functions */

// Function is an executable bit of code: a class function, class method, or a lambda (see IL module).
type Function interface {
	Definition
	GetParameters() *[]*LocalVariable
	GetReturnType() *symbols.TypeToken
	GetBody() *Block
}

type function struct {
	definition
	Parameters *[]*LocalVariable  `json:"parameters,omitempty"`
	ReturnType *symbols.TypeToken `json:"returnType,omitempty"`
	Body       *Block             `json:"body,omitempty"`
}

func (node *function) GetParameters() *[]*LocalVariable  { return node.Parameters }
func (node *function) GetReturnType() *symbols.TypeToken { return node.ReturnType }
func (node *function) GetBody() *Block                   { return node.Body }

// ModuleMethod is just a function with an accessibility modifier.
type ModuleMethod struct {
	function
	moduleMember
}

// ClassMethod is just like a module method with some extra attributes.
type ClassMethod struct {
	function
	classMember
	Sealed   *bool `json:"sealed,omitempty"`
	Abstract *bool `json:"abstract,omitempty"`
}
