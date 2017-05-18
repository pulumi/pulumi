// Copyright 2017 Pulumi, Inc. All rights reserved.

package ast

import (
	"github.com/pulumi/lumi/pkg/tokens"
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
	Name        *Identifier  `json:"name"`                  // a required name, unique amongst siblings.
	Description *string      `json:"description,omitempty"` // an optional informative description.
	Attributes  *[]Attribute `json:"attributes,omitempty"`  // an optional list of metadata decorators.
}

func (node *DefinitionNode) definition()             {}
func (node *DefinitionNode) GetName() *Identifier    { return node.Name }
func (node *DefinitionNode) GetDescription() *string { return node.Description }

// Attribute is a simple decorator token that acts as a metadata annotation.
type Attribute struct {
	NodeValue
	Decorator *Token `json:"decorator"`
}

var _ Node = (*Attribute)(nil)

const AttributeKind NodeKind = "Attribute"

/* Modules */

// Module contains members, including variables, functions, and/or classes.
type Module struct {
	DefinitionNode
	Exports *ModuleExports `json:"exports,omitempty"` // the exported symbols available for use by consuming modules.
	Members *ModuleMembers `json:"members,omitempty"` // the inner members of this module, private for its own use.
}

var _ Node = (*Module)(nil)
var _ Definition = (*Module)(nil)

const ModuleKind NodeKind = "Module"

// Modules is a map of qualified module name to Module AST node
type Modules map[tokens.ModuleName]*Module

// Export re-exports a Definition from another Module, possibly with a different name.
type Export struct {
	DefinitionNode
	Referent *Token `json:"referent"`
}

var _ Node = (*Export)(nil)
var _ Definition = (*Export)(nil)

const ExportKind NodeKind = "Export"

// ModuleExports is a map of name to export symbol.
type ModuleExports map[tokens.ModuleMemberName]*Export

// ModuleMember is a definition that belongs to a Module.
type ModuleMember interface {
	Definition
	moduleMember()
}

type ModuleMemberNode struct {
	DefinitionNode
}

func (node *ModuleMemberNode) moduleMember() {}

// ModuleMembers is a map of member name to ModuleMember symbol.
type ModuleMembers map[tokens.ModuleMemberName]ModuleMember

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
	GetAccess() *tokens.Accessibility
	GetStatic() *bool
	GetPrimary() *bool
}

type ClassMemberNode struct {
	DefinitionNode
	Access  *tokens.Accessibility `json:"access,omitempty"`
	Static  *bool                 `json:"static,omitempty"`
	Primary *bool                 `json:"primary,omitempty"`
}

func (node *ClassMemberNode) classMember()                     {}
func (node *ClassMemberNode) GetAccess() *tokens.Accessibility { return node.Access }
func (node *ClassMemberNode) GetStatic() *bool                 { return node.Static }
func (node *ClassMemberNode) GetPrimary() *bool                { return node.Primary }

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
	Type     *TypeToken   `json:"type"`
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

// ClassProperty is like a module property with some extra attributes.  By default, it is a data descriptor that simply
// manipulates the underlying property value, although a custom getter and/or setter may be provided.
type ClassProperty struct {
	VariableNode
	ClassMemberNode
	Getter   *ClassMethod `json:"getter,omitempty"`
	Setter   *ClassMethod `json:"setter,omitempty"`
	Optional *bool        `json:"optional,omitempty"`
}

var _ Node = (*ClassProperty)(nil)
var _ Definition = (*ClassProperty)(nil)
var _ ClassMember = (*ClassProperty)(nil)

const ClassPropertyKind NodeKind = "ClassProperty"

/* Functions */

// Function is an executable bit of code: a class function, class method, or a lambda (see IL module).
type Function interface {
	Node
	GetParameters() *[]*LocalVariable
	GetReturnType() *TypeToken
	GetBody() Statement
}

type FunctionNode struct {
	// note that this node intentionally omits any embedded base, to avoid diamond "inheritance".
	Parameters *[]*LocalVariable `json:"parameters,omitempty"`
	ReturnType *TypeToken        `json:"returnType,omitempty"`
	Body       Statement         `json:"body,omitempty"`
}

func (node *FunctionNode) GetParameters() *[]*LocalVariable { return node.Parameters }
func (node *FunctionNode) GetReturnType() *TypeToken        { return node.ReturnType }
func (node *FunctionNode) GetBody() Statement               { return node.Body }

// ModuleMethod is just a function with an accessibility modifier.
type ModuleMethod struct {
	FunctionNode
	ModuleMemberNode
}

var _ Node = (*ModuleMethod)(nil)
var _ Function = (*ModuleMethod)(nil)
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
var _ Function = (*ClassMethod)(nil)
var _ Definition = (*ClassMethod)(nil)
var _ ClassMember = (*ClassMethod)(nil)

const ClassMethodKind NodeKind = "ClassMethod"
