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

type DefinitionNode struct {
	NodeValue
	Name        *Identifier `json:"name"`
	Description *string     `json:"description,omitempty"`
}

func (node *DefinitionNode) definition()             {}
func (node *DefinitionNode) GetName() *Identifier    { return node.Name }
func (node *DefinitionNode) GetDescription() *string { return node.Description }

/* Modules */

// Module contains members, including variables, functions, and/or classes.
type Module struct {
	DefinitionNode
	Default bool            `json:"default,omitempty"`
	Imports *[]*ModuleToken `json:"imports,omitempty"`
	Members *ModuleMembers  `json:"members,omitempty"`
}

var _ Node = (*Module)(nil)
var _ Definition = (*Module)(nil)

const ModuleKind NodeKind = "Module"

// Modules is a map of qualified module name to Module AST node
type Modules map[tokens.ModuleName]*Module

// ModuleMember is a definition that belongs to a Module.
type ModuleMember interface {
	Definition
	moduleMember()
	GetAccess() *tokens.Accessibility
}

type ModuleMemberNode struct {
	DefinitionNode
	Access *tokens.Accessibility `json:"access,omitempty"`
}

func (node *ModuleMemberNode) moduleMember()                    {}
func (node *ModuleMemberNode) GetAccess() *tokens.Accessibility { return node.Access }

// ModuleMembers is a map of member name to ModuleMember symbol.
type ModuleMembers map[tokens.ModuleMemberName]ModuleMember

// Export re-exports a Definition from another Module, possibly with a different name.
type Export struct {
	ModuleMemberNode
	Referent *Token `json:"referent"`
}

var _ Node = (*Export)(nil)
var _ Definition = (*Export)(nil)
var _ ModuleMember = (*Export)(nil)

const ExportKind NodeKind = "Export"

/* Classes */

// Class can be constructed to create an object, and exports properties, methods, and has a number of attributes.
type Class struct {
	ModuleMemberNode
	Extends    *TypeToken    `json:"extends,omitempty"`
	Implements *[]*TypeToken `json:"implements,omitempty"`
	Sealed     *bool         `json:"sealed,omitempty"`
	Abstract   *bool         `json:"abstract,omitempty"`
	Record     *bool         `json:"record,omitempty"`
	Interface  *bool         `json:"interface,omitempty"`
	Members    *ClassMembers `json:"members,omitempty"`
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

type ClassMemberNode struct {
	DefinitionNode
	Access *tokens.ClassMemberAccessibility `json:"access,omitempty"`
	Static *bool                            `json:"static,omitempty"`
}

func (node *ClassMemberNode) classMember()                                {}
func (node *ClassMemberNode) GetAccess() *tokens.ClassMemberAccessibility { return node.Access }
func (node *ClassMemberNode) GetStatic() *bool                            { return node.Static }

// ClassMembers is a map of class member name to ClassMember symbol.
type ClassMembers map[tokens.ClassMemberName]ClassMember

/* Variables */

// Variable is a storage location with an optional type.
type Variable interface {
	Definition
	GetType() *TypeToken
	GetDefault() *interface{} // a trivially serializable default value.
	GetReadonly() *bool
}

type VariableNode struct {
	// note that this node intentionally omits any embedded base, to avoid diamond "inheritance".
	Type     *TypeToken   `json:"type,omitempty"`
	Default  *interface{} `json:"default,omitempty"`
	Readonly *bool        `json:"readonly,omitempty"`
}

func (node *VariableNode) GetType() *TypeToken      { return node.Type }
func (node *VariableNode) GetDefault() *interface{} { return node.Default }
func (node *VariableNode) GetReadonly() *bool       { return node.Readonly }

// LocalVariable is a variable that is lexically scoped within a function (either a parameter or local).
type LocalVariable struct {
	VariableNode
	DefinitionNode
}

var _ Node = (*LocalVariable)(nil)
var _ Definition = (*LocalVariable)(nil)

const LocalVariableKind NodeKind = "LocalVariable"

// ModuleProperty is like a variable but belongs to a module.
type ModuleProperty struct {
	VariableNode
	ModuleMemberNode
}

var _ Node = (*ModuleProperty)(nil)
var _ Definition = (*ModuleProperty)(nil)
var _ ModuleMember = (*ModuleProperty)(nil)

const ModulePropertyKind NodeKind = "ModuleProperty"

// ClassProperty is like a module property with some extra attributes.
type ClassProperty struct {
	VariableNode
	ClassMemberNode
	Primary  *bool `json:"primary,omitempty"`
	Optional *bool `json:"optional,omitempty"`
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
	GetReturnType() *TypeToken
	GetBody() *Block
}

type FunctionNode struct {
	// note that this node intentionally omits any embedded base, to avoid diamond "inheritance".
	Parameters *[]*LocalVariable `json:"parameters,omitempty"`
	ReturnType *TypeToken        `json:"returnType,omitempty"`
	Body       *Block            `json:"body,omitempty"`
}

func (node *FunctionNode) GetParameters() *[]*LocalVariable { return node.Parameters }
func (node *FunctionNode) GetReturnType() *TypeToken        { return node.ReturnType }
func (node *FunctionNode) GetBody() *Block                  { return node.Body }

// ModuleMethod is just a function with an accessibility modifier.
type ModuleMethod struct {
	FunctionNode
	ModuleMemberNode
}

var _ Node = (*ModuleMethod)(nil)
var _ Definition = (*ModuleMethod)(nil)
var _ ModuleMember = (*ModuleMethod)(nil)

const ModuleMethodKind NodeKind = "ModuleMethod"

// ClassMethod is just like a module method with some extra attributes.
type ClassMethod struct {
	FunctionNode
	ClassMemberNode
	Sealed   *bool `json:"sealed,omitempty"`
	Abstract *bool `json:"abstract,omitempty"`
}

var _ Node = (*ClassMethod)(nil)
var _ Definition = (*ClassMethod)(nil)
var _ ClassMember = (*ClassMethod)(nil)

const ClassMethodKind NodeKind = "ClassMethod"
