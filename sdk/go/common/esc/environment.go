// Copyright 2023, Pulumi Corporation.  All rights reserved.

package esc

import "github.com/pulumi/esc/schema"

// An Environment contains the result of evaluating an environment definition.
type Environment struct {
	// Exprs contains the AST for each expression in the environment definition.
	Exprs map[string]Expr `json:"exprs,omitempty"`

	// Properties contains the detailed values produced by the environment.
	Properties map[string]Value `json:"properties,omitempty"`

	// Schema contains the schema for Properties.
	Schema *schema.Schema `json:"schema,omitempty"`
}
