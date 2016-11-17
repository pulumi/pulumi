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

// Name is an identifier.  Names may be optionally fully qualified, using the delimiter `/`, or simple.  Each element
// conforms to the regex [A-Za-z_][A-Za-z0-9_]*.  For example, `marapongo/mu/stack`.
type Name string

// SemVer represents a version using "semantic versioning" style.  This may include up to three distinct numbers
// delimited by `.`s: a major version number, a minor version number, and a revision number.  For example, `1.0.10`.
type SemVer string

// Node is the base of all abstract syntax tree types.
type Node struct {
}

// Metadata contains human-readable metadata common to Mu's packaging formats (like Stacks and Clusters).
type Metadata struct {
	Node

	Kind string `json:"-"` // kind is decorated post-parsing, since it is contextual.

	Name        Name    `json:"name,omitempty"`        // a friendly name for this node.
	Version     SemVer  `json:"version,omitempty"`     // a specific semantic version.
	Description string  `json:"description,omitempty"` // an optional friendly description.
	Author      string  `json:"author,omitempty"`      // an optional author.
	Website     string  `json:"website,omitempty"`     // an optional website for additional info.
	License     string  `json:"license,omitempty"`     // an optional license governing legal uses of this package.
	Targets     Targets `json:"targets,omitempty"`     // an optional set of predefined target cloud environments.
}

// Targets is a map of target names to metadata about those targets.
type Targets map[string]Target

// Target describes a predefined cloud runtime target, including its OS and Scheduler combination.
type Target struct {
	Node

	Name string `json:"-"` // name is decorated post-parsing, since it is contextual.

	Default     bool                   `json:"default,omitempty"`     // a single target can carry default settings.
	Description string                 `json:"description,omitempty"` // a human-friendly description of this target.
	Cloud       string                 `json:"cloud,omitempty"`       // the cloud target.
	Scheduler   string                 `json:"scheduler,omitempty"`   // the cloud scheduler target.
	Options     map[string]interface{} `json:"options,omitempty"`     // any options passed to the cloud provider.
}

// Cluster describes a cluster of many Stacks, in addition to other metadata, like predefined Targets.
type Cluster struct {
	Metadata
}

// Stack represents a collection of private and public cloud resources, a method for constructing them, and optional
// dependencies on other Stacks (by name).
type Stack struct {
	Metadata
	Parameters   Parameters   `json:"parameters,omitempty"`
	Dependencies Dependencies `json:"dependencies,omitempty"`
	Services     Services     `json:"services,omitempty"`
}

// Parameters maps parameter names to metadata about those parameters.
type Parameters map[string]Parameter

// Parameter describes the requirements of arguments used when constructing Stacks, etc.
type Parameter struct {
	Node

	Name string `json:"-"` // name is decorated post-parsing, since it is contextual.

	Type        Name        `json:"type,omitempty"`        // the type of the parameter; required.
	Description string      `json:"description,omitempty"` // an optional friendly description of the parameter.
	Default     interface{} `json:"default,omitempty"`     // an optional default value if the caller doesn't pass one.
	Optional    bool        `json:"optional,omitempty"`    // true if may be omitted (inferred if a default value).
}

// Dependencies maps dependency names to the semantic version the consumer depends on.
type Dependencies map[Name]Dependency

// Dependency is metadata describing a dependency target (for now, just its semantic version).
type Dependency SemVer

// Services is a list of public and private service references, keyed by name.
type Services struct {
	Public  ServiceMap `json:"public,omitempty"`
	Private ServiceMap `json:"private,omitempty"`
}

// ServiceMap is a map of service names to metadata about those services.
type ServiceMap map[Name]Service

// Service is a directive for instantiating another Stack, including its name, arguments, etc.
type Service struct {
	Node

	Name   Name `json:"-"` // a friendly name; decorated post-parsing, since it is contextual.
	Public bool `json:"-"` // true if this service is publicly exposed; also decorated post-parsing.

	Type Name `json:"type,omitempty"` // an explicit type; if missing, the name is used.
	// TODO: Service metadata is highly type-dependent.  It's not yet clear how best to represent this in the schema.
}

// TODO: several more core types still need to be mapped:
//     - Schema
//     - Identity: User, Role, Group
//     - Configuration
//     - Secret
