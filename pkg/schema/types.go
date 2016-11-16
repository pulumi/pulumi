// Copyright 2016 Marapongo, Inc. All rights reserved.

// The schema package contains the serialized form of the core Mu types.
// TODO: several core types still need to be mapped:
//     - Schema
//     - Identity: User, Role, Group
//     - Configuration
//     - Secret
package schema

// Name is an identifier.  Names may be optionally fully qualified, using the delimiter `/`, or simple.  Each element
// conforms to the regex [A-Za-z_][A-Za-z0-9_]*.  For example, `marapongo/mu/stack`.
type Name string

// SemVer represents a version using "semantic versioning" style.  This may include up to three distinct numbers
// delimited by `.`s: a major version number, a minor version number, and a revision number.  For example, `1.0.10`.
type SemVer string

// Metadata contains human-readable metadata common to Mu's packaging formats (like Stacks and Clusters).
type Metadata struct {
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
	Description string      `json:"description,omitempty"`
	Type        Name        `json:"type,omitempty"`
	Default     interface{} `json:"default,omitempty"`
	Optional    bool        `json:"optional,omitempty"`
}

// Services maps service names to metadata about those services.
type Services map[string]*Service

// Service is a directive for instantiating another Stack, including its name, arguments, etc.
// TODO: Service metadata is highly extensible.  It's not yet clear how best to represent this.
type Service struct {
	Type Name `json:"type,omitempty"`
}

// Dependencies maps dependency names to the semantic version the consumer depends on.
type Dependencies map[string]SemVer

// Cluster describes a cluster of many Stacks, in addition to other metadata, like predefined Targets.
type Cluster struct {
	Metadata
	Targets map[string]*Target `json:"targets,omitempty"`
}

// Target describes a predefined cloud runtime target, including its OS and Scheduler combination.
type Target struct {
	Description string `json:"description,omitempty"`
	Cloud       string `json:"cloud,omitempty"`
	Scheduler   string `json:"scheduler,omitempty"`
	// TODO(joe): configuration.
}
