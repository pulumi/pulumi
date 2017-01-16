// Copyright 2016 Marapongo, Inc. All rights reserved.

// This package contains the core MuPackage metadata types.
package pack

import (
	"github.com/marapongo/mu/pkg/pack/ast"
	"github.com/marapongo/mu/pkg/pack/symbols"
)

// Metadata is an informational section describing a package.
type Metadata struct {
	Name        string  `json:"name"`                  // a required fully qualified name.
	Description *string `json:"description,omitempty"` // an optional informational description.
	Author      *string `json:"author,omitempty"`      // an optional author.
	Website     *string `json:"website,omitempty"`     // an optional website for additional info.
	License     *string `json:"license,omitempty"`     // an optional license governing this package's usage.
}

// Package is a top-level package definition.
type Package struct {
	Metadata

	Dependencies *[]symbols.ModuleToken `json:"dependencies,omitempty"` // all of the module dependencies.
	Modules      *ast.Modules           `json:"modules,omitempty"`      // a collection of top-level modules.
}
