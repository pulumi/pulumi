// Copyright 2016 Marapongo, Inc. All rights reserved.

package ast

import (
	"github.com/marapongo/mu/pkg/tokens"
)

/* Definitions */

// Definition is a type system element that is possibly exported from a module for external usage.
type Definition interface {
	Node
	definition()
	GetName() *Identifier    // a required name, unique amongst definitions with a common parent.
	GetDescription() *string // an optional informative description.
}

type definitionNode struct {
	node
	Name        *Identifier `json:"name"`
	Description *string     `json:"description,omitempty"`
}

func (node *definitionNode) definition()             {}
func (node *definitionNode) GetName() *Identifier    { return node.Name }
func (node *definitionNode) GetDescription() *string { return node.Description }

/* Modules */

// Module contains members, including variables, functions, and/or classes.
type Module struct {
	definitionNode
	Imports *[]tokens.Module `json:"imports,omitempty"`
	Members *ModuleMembers   `json:"members,omitempty"`
}

var _ Node = (*Module)(nil)
var _ Definition = (*Module)(nil)

const ModuleKind NodeKind = "Module"

// Modules is a map of Name to Module AST node
type Modules map[tokens.Name]*Module

// ModuleMember is a definition that belongs to a Module.
type ModuleMember interface {
	Definition
	moduleMember()
	GetAccess() *tokens.Accessibility
}

type moduleMemberNode struct {
	definitionNode
	Access *tokens.Accessibility `json:"access,omitempty"`
}

func (node *moduleMemberNode) moduleMember()                    {}
func (node *moduleMemberNode) GetAccess() *tokens.Accessibility { return node.Access }

// ModuleMembers is a map of Token to ModuleMember.
type ModuleMembers map[tokens.Name]ModuleMember

// Export re-exports a Definition from another Module, possibly with a different name.
type Export struct {
	moduleMemberNode
	Referent tokens.Token `json:"referent"`
}

var _ Node = (*Export)(nil)
var _ Definition = (*Export)(nil)
var _ ModuleMember = (*Export)(nil)

const ExportKind NodeKind = "Export"

/* Classes */

// Class can be constructed to create an object, and exports properties, methods, and has a number of attributes.
type Class struct {
	moduleMemberNode
	Extends    *tokens.Type   `json:"extends,omitempty"`
	Implements *[]tokens.Type `json:"implements,omitempty"`
	Sealed     *bool          `json:"sealed,omitempty"`
	Abstract   *bool          `json:"abstract,omitempty"`
	Record     *bool          `json:"record,omitempty"`
	Interface  *bool          `json:"interface,omitempty"`
	Members    *ClassMembers  `json:"members,omitempty"`
}

var _ Node = (*Class)(nil)
var _ Definition = (*Class)(nil)
var _ ModuleMember = (*Class)(nil)

const ClassKind NodeKind = "Class"

// ClassMember is a Definition that belongs to a Class.
type ClassMember interface {
	Definition
	GetAccess() *tokens.ClassMemberAccessibility
	GetStatic() *bool
}

type classMemberNode struct {
	definitionNode
	Access *tokens.ClassMemberAccessibility `json:"access,omitempty"`
	Static *bool                            `json:"static,omitempty"`
}

func (node *classMemberNode) classMember()                                {}
func (node *classMemberNode) GetAccess() *tokens.ClassMemberAccessibility { return node.Access }
func (node *classMemberNode) GetStatic() *bool                            { return node.Static }

// ClassMembers is a map of Name to ClassMember.
type ClassMembers map[tokens.Name]ClassMember

/* Variables */

// Variable is a storage location with an optional type.
type Variable interface {
	Definition
	GetType() *tokens.Type
	GetDefault() *interface{} // a trivially serializable default value.
	GetReadonly() *bool
}

type variableNode struct {
	// note that this node intentionally omits any embedded base, to avoid diamond "inheritance".
	Type     *tokens.Type `json:"type,omitempty"`
	Default  *interface{} `json:"default,omitempty"`
	Readonly *bool        `json:"readonly,omitempty"`
}

func (node *variableNode) GetType() *tokens.Type    { return node.Type }
func (node *variableNode) GetDefault() *interface{} { return node.Default }
func (node *variableNode) GetReadonly() *bool       { return node.Readonly }

// LocalVariable is a variable that is lexically scoped within a function (either a parameter or local).
type LocalVariable struct {
	variableNode
	definitionNode
}

var _ Node = (*LocalVariable)(nil)
var _ Definition = (*LocalVariable)(nil)

const LocalVariableKind NodeKind = "LocalVariable"

// ModuleProperty is like a variable but belongs to a module.
type ModuleProperty struct {
	variableNode
	moduleMemberNode
}

var _ Node = (*ModuleProperty)(nil)
var _ Definition = (*ModuleProperty)(nil)
var _ ModuleMember = (*ModuleProperty)(nil)

const ModulePropertyKind NodeKind = "ModuleProperty"

// ClassProperty is like a module property with some extra attributes.
type ClassProperty struct {
	variableNode
	classMemberNode
	Primary *bool `json:"primary,omitempty"`
}

var _ Node = (*ClassProperty)(nil)
var _ Definition = (*ClassProperty)(nil)
var _ ClassMember = (*ClassProperty)(nil)

const ClassPropertyKind NodeKind = "ClassProperty"

/* Functions */

// Function is an executable bit of code: a class function, class method, or a lambda (see IL module).
type Function interface {
	Definition
	GetParameters() *[]*LocalVariable
	GetReturnType() *tokens.Type
	GetBody() *Block
}

type functionNode struct {
	// note that this node intentionally omits any embedded base, to avoid diamond "inheritance".
	Parameters *[]*LocalVariable `json:"parameters,omitempty"`
	ReturnType *tokens.Type      `json:"returnType,omitempty"`
	Body       *Block            `json:"body,omitempty"`
}

func (node *functionNode) GetParameters() *[]*LocalVariable { return node.Parameters }
func (node *functionNode) GetReturnType() *tokens.Type      { return node.ReturnType }
func (node *functionNode) GetBody() *Block                  { return node.Body }

// ModuleMethod is just a function with an accessibility modifier.
type ModuleMethod struct {
	functionNode
	moduleMemberNode
}

var _ Node = (*ModuleMethod)(nil)
var _ Definition = (*ModuleMethod)(nil)
var _ ModuleMember = (*ModuleMethod)(nil)

const ModuleMethodKind NodeKind = "ModuleMethod"

// ClassMethod is just like a module method with some extra attributes.
type ClassMethod struct {
	functionNode
	classMemberNode
	Sealed   *bool `json:"sealed,omitempty"`
	Abstract *bool `json:"abstract,omitempty"`
}

var _ Node = (*ClassMethod)(nil)
var _ Definition = (*ClassMethod)(nil)
var _ ClassMember = (*ClassMethod)(nil)

const ClassMethodKind NodeKind = "ClassMethod"
