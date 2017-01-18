// Copyright 2016 Marapongo, Inc. All rights reserved.

// This package contains the core Mu abstract syntax tree types.
//
// N.B. for the time being, we are leveraging the same set of types for parse trees and abstract syntax trees.  The
// reason is that minimal "extra" information is necessary between front- and back-end parts of the compiler, and so
// reusing the trees leads to less duplication in types and faster runtime performance.  As the compiler matures in
// functionality, we may want to revisit this.  The "back-end-only" parts of the data structures are easily identified
// because their fields do not map to any serializable fields (i.e., `json:"-"`).
//
// Another controversial decision is to mutate nodes in place, rather than taking the performance hit of immutability.
// This can certainly be tricky to deal with, however, it is simpler and we can revisit it down the road if needed.
// Of course, during lowering, sometimes nodes will be transformed to new types entirely, allocating entirely anew.
package ast

import (
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/tokens"
)

// Node is the base of all abstract syntax tree types.
type Node struct {
}

func (node *Node) Where() (*diag.Document, *diag.Location) {
	// TODO[marapongo/mu#14]: implement diag.Diagable on all AST nodes.
	return nil, nil
}

// Stack represents a collection of private and public cloud resources, a method for constructing them, and optional
// dependencies on other Stacks (by name).
type Stack struct {
	Node

	Name        tokens.Name    `json:"name,omitempty"`        // a friendly name for this node.
	Version     tokens.Version `json:"version,omitempty"`     // a specific version number.
	Description string         `json:"description,omitempty"` // an optional friendly description.
	Author      string         `json:"author,omitempty"`      // an optional author.
	Website     string         `json:"website,omitempty"`     // an optional website for additional info.
	License     string         `json:"license,omitempty"`     // an optional license governing use of this package.

	Base                tokens.Ref         `json:"base,omitempty"`      // an optional base Stack type.
	BoundBase           *Stack             `json:"-"`                   // base, optionally bound during analysis.
	Abstract            bool               `json:"abstract,omitempty"`  // true if this stack is "abstract".
	Intrinsic           bool               `json:"intrinsic,omitempty"` // true if this stack is an "intrinsic" type.
	Properties          Properties         `json:"properties,omitempty"`
	PropertyValues      PropertyBag        `json:"-"`               // the properties used to construct this stack.
	BoundPropertyValues LiteralPropertyBag `json:"-"`               // bound properties used to construct this stack.
	Types               Schemas            `json:"types,omitempty"` // an optional types section with custom schemas.
	Services            Services           `json:"services,omitempty"`

	Doc *diag.Document `json:"-"` // the document from which this came.

	// TODO[marapongo/mu#8]: permit Stacks to declare exported APIs.
}

func (stack *Stack) Where() (*diag.Document, *diag.Location) {
	return stack.Doc, nil
}

// UninstStack represents a dependency that hasn't yet been instantiated.  This is like an uninstantiated generic type
// in classical programming languages, except that in our case we use template expansion on the document itself.
// TODO(joe): eventually this ought to also encompass cross-stack schema references.
type UninstStack struct {
	Node
	Ref tokens.Ref     `json:"-"`
	Doc *diag.Document `json:"-"`
}

// DependendyRefs is simply a map of dependency reference to the associated uninstantiated Stack for that dependency.
type DependencyRefs map[tokens.Ref]*UninstStack

// Propertys maps property names to metadata about those propertys.
type Properties map[string]*Property

// Property describes the requirements of arguments used when constructing Stacks, etc.
type Property struct {
	Node

	Type        tokens.Ref  `json:"type,omitempty"`        // the type of the property; required.
	BoundType   *Type       `json:"-"`                     // if the property is a stack type, it will be bound.
	Description string      `json:"description,omitempty"` // an optional friendly description of the property.
	Default     interface{} `json:"default,omitempty"`     // an optional default value if the caller elides one.
	Optional    bool        `json:"optional,omitempty"`    // true if may be omitted (inferred if a default value).
	Readonly    bool        `json:"readonly,omitempty"`    // true if this property is readonly.
	Perturbs    bool        `json:"perturbs,omitempty"`    // true if changing this property is perturbing.

	Name string `json:"-"` // name is decorated post-parsing, since it is contextual.
}

// Schemas is a list of public and private service references, keyed by name.
type Schemas struct {
	Public  SchemaMap `json:"-"`
	Private SchemaMap `json:"-"`
}

// SchemaMap is a map of schema names to metadata about those schemas.
type SchemaMap map[tokens.Name]*Schema

// Schema represents a complex schema type that extends Mu's type system and can be used by name.
// TODO[marapongo/mu#9]: support the full set of JSON schema operators (like allOf, anyOf, etc.); to see the full list,
//     refer to the spec: http://json-schema.org/latest/json-schema-validation.html.
// TODO[marapongo/mu#9]: we deviate from the spec in a few areas; e.g., we default to required and support optional.  We
//     should do an audit of all such places and decide whether it's worth deviating.  If yes, we must clearly document.
type Schema struct {
	Node

	Base       tokens.Ref `json:"base,omitempty"`       // the base type from which this derives.
	BoundBase  *Type      `json:"-"`                    // base, optionally bound during analysis.
	Properties Properties `json:"properties,omitempty"` // all of the custom properties for object types.

	// constraints for string types:
	Pattern   *string  `json:"pattern,omitempty"`   // an optional regex pattern for string types.
	MaxLength *float64 `json:"maxLength,omitempty"` // an optional max string length (in characters).
	MinLength *float64 `json:"minLength,omitempty"` // an optional min string length (in characters).

	// constraints for numeric types:
	Maximum *float64 `json:"maximum,omitempty"` // an optional max value for numeric types.
	Minimum *float64 `json:"minimum,omitempty"` // an optional min value for numeric types.

	// constraints for strings *and* number types:
	Enum []interface{} `json:"enum,omitempty"` // an optional enum of legal values.

	Name   tokens.Name `json:"-"` // a friendly name; decorated post-parsing, since it is contextual.
	Public bool        `json:"-"` // true if this schema type is publicly exposed; also decorated post-parsing.
}

// Services is a list of public and private service references, keyed by name.
type Services struct {
	// These fields are expanded after parsing:
	Public  ServiceMap `json:"-"`
	Private ServiceMap `json:"-"`

	// These fields are "untyped" due to limitations in the JSON parser.  Namely, Go's parser will ignore
	// properties in the payload that it doesn't recognize as mapping to a field.  That's not what we want, especially
	// for services since they are highly extensible and the contents will differ per-type.  Therefore, we will first
	// map the services into a weakly typed map, and later on during compilation, expand them to the below fields.
	// TODO[marapongo/mu#4]: support for `json:",inline"` or the equivalent so we can eliminate these fields.
	PublicUntyped  UntypedServiceMap `json:"public,omitempty"`
	PrivateUntyped UntypedServiceMap `json:"private,omitempty"`
}

// ServiceMap is a map of service names to metadata about those services.
type ServiceMap map[tokens.Name]*Service

// UntypedServiceMap is a map of service names to untyped, bags of parsed properties for those services.
type UntypedServiceMap map[tokens.Name]PropertyBag

// Service is a directive for instantiating another Stack, including its name, arguments, etc.
type Service struct {
	Node

	Type            tokens.Ref         `json:"type,omitempty"` // an explicit type; if missing, the name is used.
	BoundType       *Stack             `json:"-"`              // services are bound to stacks during semantic analysis.
	Properties      PropertyBag        `json:"-"`              // all of the custom properties (minus what's above).
	BoundProperties LiteralPropertyBag `json:"-"`              // the bound properties, expanded and typed correctly.

	Name   tokens.Name `json:"-"` // a friendly name; decorated post-parsing, since it is contextual.
	Public bool        `json:"-"` // true if this service is publicly exposed; also decorated post-parsing.
}

// PropertyBag is simply a map of string property names to untyped data values.
type PropertyBag map[string]interface{}

// LiteralPropertyBag is simply a map of string property names to literal typed AST nodes.
type LiteralPropertyBag map[string]Literal

// ServiceRef is an intra- or inter-stack reference to a specific service.
type ServiceRef struct {
	Name     tokens.Name // the name used to resolve the capability.
	Selector tokens.Name // the "selector" used if the target service exports multiple endpoints.
	Service  *Service    // the service that this capability reference names.
	Selected *Service    // the selected service resolved during binding.
}

// Literal represents a strongly typed AST value.
type Literal interface {
	Node() *Node
	Type() *Type
}

// AnyLiteral is an AST node containing a literal value of "any" type (`interface{}`).
type AnyLiteral interface {
	Literal
	Any() interface{}
}

// BoolLiteral is an AST node containing a literal boolean (`bool`).
type BoolLiteral interface {
	Literal
	Bool() bool
}

// NumberLiteral is an AST node containing a literal number (`float64`).
type NumberLiteral interface {
	Literal
	Number() float64
}

// StringLiteral is an AST node containing a literal string (`string`).
type StringLiteral interface {
	Literal
	String() string
}

// ServiceLiteral is an AST node containing a literal capability reference.
type ServiceLiteral interface {
	Literal
	Service() *ServiceRef
}

// ArrayLiteral is an AST node containing an array of other literals (`[]x`).
type ArrayLiteral interface {
	Literal
	ElemType() *Type
	Array() []Literal
}

// MapLiteral is an AST node containing a map from literals of a certain type to other literals.
type MapLiteral interface {
	Literal
	KeyType() *Type
	ValueType() *Type
	Keys() []Literal
	Values() []Literal
}

// SchemaLiteral is an AST node containing a literal value that is strongly typed, but too complex to represent
// structually in Go's type system.  For these types, we resort to dynamic manipulation of the contents.
type SchemaLiteral interface {
	Literal
	Properties() LiteralPropertyBag
}

// TODO[marapongo/mu#9]: extensible schema support.
// TODO[marapongo/mu#17]: identity (users, roles, groups).
// TODO[marapongo/mu#16]: configuration and secret support.
