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
	Name        Name   `json:"name,omitempty"`
	Version     SemVer `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
	Author      string `json:"author,omitempty"`
	Website     string `json:"website,omitempty"`
	License     string `json:"license,omitempty"`
}

// Stack represents a collection of private and public cloud resources, a method for constructing them, and optional
// dependencies on other Stacks (by name).
type Stack struct {
	Metadata
	Parameters   Parameters   `json:"parameters,omitempty"`
	Public       Services     `json:"public,omitempty"`
	Private      Services     `json:"private,omitempty"`
	Dependencies Dependencies `json:"dependencies,omitempty"`
}

// Parameters maps parameter names to metadata about those parameters.
type Parameters map[string]*Parameter

// Parameter describes the requirements of arguments used when constructing Stacks, etc.
type Parameter struct {
	Node
	Description string      `json:"description,omitempty"`
	Type        Name        `json:"type,omitempty"`
	Default     interface{} `json:"default,omitempty"`
	Optional    bool        `json:"optional,omitempty"`
}

// Services maps service names to metadata about those services.
type Services map[string]*Service

// Service is a directive for instantiating another Stack, including its name, arguments, etc.
type Service struct {
	Node
	Type Name `json:"type,omitempty"`
	// TODO: Service metadata is highly extensible.  It's not yet clear how best to represent this.
}

// Dependencies maps dependency names to the semantic version the consumer depends on.
type Dependencies map[string]Dependency

// Dependency is metadata describing a dependency target (for now, just its semantic version).
type Dependency string

// Cluster describes a cluster of many Stacks, in addition to other metadata, like predefined Targets.
type Cluster struct {
	Metadata
	Targets map[string]*Target `json:"targets,omitempty"`
}

// Target describes a predefined cloud runtime target, including its OS and Scheduler combination.
type Target struct {
	Node
	Description string `json:"description,omitempty"`
	Cloud       string `json:"cloud,omitempty"`
	Scheduler   string `json:"scheduler,omitempty"`
	// TODO(joe): configuration.
}

// TODO: several more core types still need to be mapped:
//     - Schema
//     - Identity: User, Role, Group
//     - Configuration
//     - Secret
