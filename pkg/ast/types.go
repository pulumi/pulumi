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
)

// Name is an identifier.  Names may be optionally fully qualified, using the delimiter `/`, or simple.  Each element
// conforms to the regex [A-Za-z_][A-Za-z0-9_]*.  For example, `marapongo/mu/stack`.
type Name string

// Ref is a dependency reference.  It is "name-like", in that it contains a Name embedded inside of it, but also carries
// a URL-like structure.  A Ref starts with an optional "protocol" (like https://, git://, etc), followed by an optional
// "base" part (like hub.mu.com/, github.com/, etc), followed by the "name" part (which is just a Name), followed by
// an optional "@" and version number (where version may be "latest", a semantic version range, or a Git SHA hash).
type Ref string

// Version represents a precise version number.  It may be either a Git SHA hash or a semantic version (not a range).
type Version string

// VersionSpec represents a specification of a version that is bound to a precise number through a separate process.
// It may take the form of a Version (see above), a semantic version range, or the string "latest", to indicate that the
// latest available sources are to be used at compile-time.
type VersionSpec string

// Node is the base of all abstract syntax tree types.
type Node struct {
	// TODO[marapongo/mu#14]: implement diag.Diagable on all AST nodes.
}

// Workspace defines settings shared amongst many related Stacks.
type Workspace struct {
	Node

	Clusters     Clusters     `json:"clusters,omitempty"` // an optional set of predefined target clusters.
	Dependencies Dependencies `json:"dependencies,omitempty"`

	Doc *diag.Document `json:"-"` // the document from which this came.
}

func (w *Workspace) Where() (*diag.Document, *diag.Location) {
	return w.Doc, nil
}

// Clusters is a map of target names to metadata about those targets.
type Clusters map[string]Cluster

// Cluster describes a predefined cloud runtime target, including its OS and Scheduler combination.
type Cluster struct {
	Node

	Default     bool        `json:"default,omitempty"`     // a single target can carry default settings.
	Description string      `json:"description,omitempty"` // a human-friendly description of this target.
	Cloud       string      `json:"cloud,omitempty"`       // the cloud target.
	Scheduler   string      `json:"scheduler,omitempty"`   // the cloud scheduler target.
	Settings    PropertyBag `json:"settings,omitempty"`    // any options passed to the cloud provider.

	Name string `json:"-"` // name is decorated post-parsing, since it is contextual.
}

// Dependencies maps dependency refs to the semantic version the consumer depends on.
type Dependencies map[Ref]Dependency

// Dependency is metadata describing a dependency target (for now, just its target version).
type Dependency VersionSpec

// DependendyDocuments is simply a map of dependency reference to the associated document containing that dependency.
type DependencyDocuments map[Ref]*diag.Document

// Stack represents a collection of private and public cloud resources, a method for constructing them, and optional
// dependencies on other Stacks (by name).
type Stack struct {
	Node

	Name        Name    `json:"name,omitempty"`        // a friendly name for this node.
	Version     Version `json:"version,omitempty"`     // a specific version number.
	Description string  `json:"description,omitempty"` // an optional friendly description.
	Author      string  `json:"author,omitempty"`      // an optional author.
	Website     string  `json:"website,omitempty"`     // an optional website for additional info.
	License     string  `json:"license,omitempty"`     // an optional license governing legal uses of this package.

	Base       Ref        `json:"base,omitempty"`     // an optional base Stack type.
	BoundBase  *Stack     `json:"-"`                  // base, if available, is bound during semantic analysis.
	Abstract   bool       `json:"abstract,omitempty"` // true if this stack is "abstract" (uninstantiable).
	Properties Properties `json:"properties,omitempty"`
	Services   Services   `json:"services,omitempty"`

	Predef bool           `json:"-"` // true if this is a predefined type (treated specially).
	Doc    *diag.Document `json:"-"` // the document from which this came.

	// TODO[marapongo/mu#8]: permit Stacks to declare exported APIs.
}

func (stack *Stack) Where() (*diag.Document, *diag.Location) {
	return stack.Doc, nil
}

// Propertys maps property names to metadata about those propertys.
type Properties map[string]Property

// Property describes the requirements of arguments used when constructing Stacks, etc.
type Property struct {
	Node

	Type        PropertyType `json:"type,omitempty"`        // the type of the property; required.
	Description string       `json:"description,omitempty"` // an optional friendly description of the property.
	Default     interface{}  `json:"default,omitempty"`     // an optional default value if the caller elides one.
	Optional    bool         `json:"optional,omitempty"`    // true if may be omitted (inferred if a default value).
	Readonly    bool         `json:"readonly,omitempty"`    // true if this property is readonly.
	Perturbs    bool         `json:"perturbs,omitempty"`    // true if changing this property is perturbing.

	Name string `json:"-"` // name is decorated post-parsing, since it is contextual.
}

// PropertyType stores the name of a property's type.
type PropertyType Name

// A set of known property types.  Note that this is extensible, so names outside of this list are legal.
// TODO[marapongo/mu#9]: support complex types (like arrays, custom JSON shapes, and so on).
const (
	PropertyTypeAny     PropertyType = "any"     // any structure.
	PropertyTypeString               = "string"  // a JSON-like string.
	PropertyTypeNumber               = "number"  // a JSON-like number (integer or floating point).
	PropertyTypeBool                 = "bool"    // a JSON-like boolean (`true` or `false`).
	PropertyTypeService              = "service" // an untyped service reference; the runtime manifestation is a URL.
)

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
type ServiceMap map[Name]Service

// Service is a directive for instantiating another Stack, including its name, arguments, etc.
type Service struct {
	Node

	Type       Ref                `json:"type,omitempty"` // an explicit type; if missing, the name is used.
	BoundType  *Stack             `json:"-"`              // services are bound to stacks during semantic analysis.
	Props      PropertyBag        `json:"-"`              // all of the "extra" properties, other than what is above.
	BoundProps LiteralPropertyBag `json:"-"`              // the bound properties, expanded and typed correctly.

	Name   Name `json:"-"` // a friendly name; decorated post-parsing, since it is contextual.
	Public bool `json:"-"` // true if this service is publicly exposed; also decorated post-parsing.
}

// UntypedServiceMap is a map of service names to untyped, bags of parsed properties for those services.
type UntypedServiceMap map[Name]PropertyBag

// PropertyBag is simply a map of string property names to untyped data values.
type PropertyBag map[string]interface{}

// LiteralPropertyBag is simply a map of string property names to literal typed AST nodes.
type LiteralPropertyBag map[string]interface{} /*Literal*/

// Literal represents a strongly typed AST value.
type Literal struct {
	Node
}

// LiteralAny is an AST node containing a literal value of "any" type (`interface{}`).
type LiteralAny struct {
	Literal
	Any interface{}
}

// LiteralString is an AST node containing a literal string (`string`).
type LiteralString struct {
	Literal
	String string
}

// LiteralNumber is an AST node containing a literal number (`float64`).
type LiteralNumber struct {
	Literal
	Number float64
}

// LiteralBool is an AST node containing a literal boolean (`bool`).
type LiteralBool struct {
	Literal
	Bool bool
}

// LiteralCapRef is an AST node containing a literal capability reference (`string`).
type LiteralCapRef struct {
	Literal
	Name     Name     // the name used to resolve the capability.
	Selector Name     // the "selector" used if the target service exports multiple endpoints.
	Stack    *Stack   // the stack in which the capability resides.
	Service  *Service // the service that this capability reference names.
	Selected *Service // the selected service resolved during binding.
}

// TODO[marapongo/mu#9]: extensible schema support.
// TODO[marapongo/mu#17]: identity (users, roles, groups).
// TODO[marapongo/mu#16]: configuration and secret support.
