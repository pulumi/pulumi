// Copyright 2016 Marapongo, Inc. All rights reserved.

// Package pack contains the core MuPackage metadata types.
package pack

import (
	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/tokens"
)

// Package is a top-level package definition.
type Package struct {
	Name tokens.PackageName `json:"name"` // a required fully qualified name.

	Description *string `json:"description,omitempty"` // an optional informational description.
	Author      *string `json:"author,omitempty"`      // an optional author.
	Website     *string `json:"website,omitempty"`     // an optional website for additional info.
	License     *string `json:"license,omitempty"`     // an optional license governing this package's usage.

	Dependencies *Dependencies  `json:"dependencies,omitempty"` // all of the package dependencies.
	Modules      *ast.Modules   `json:"modules,omitempty"`      // a collection of top-level modules.
	Aliases      *ModuleAliases `json:"aliases,omitempty"`      // an optional mapping of aliased module names.

	Doc *diag.Document `json:"-"` // the document from which this package came.
}

var _ diag.Diagable = (*Package)(nil)

func (s *Package) Where() (*diag.Document, *diag.Location) {
	return s.Doc, nil
}

// Dependencies maps dependency names to the full URL, including version, of the package.
type Dependencies map[tokens.PackageName]PackageURLString

// ModuleAliases can be used to map module names to other module names during binding.  This is useful for representing
// "default" modules in various forms; e.g., "index" as ".default"; "lib/index" as "lib"; and so on.
type ModuleAliases map[tokens.ModuleName]tokens.ModuleName
