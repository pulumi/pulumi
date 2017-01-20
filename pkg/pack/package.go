// Copyright 2016 Marapongo, Inc. All rights reserved.

// This package contains the core MuPackage metadata types.
package pack

import (
	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/tokens"
)

// Package is a top-level package definition.
type Package struct {
	Name string `json:"name"` // a required fully qualified name.

	Description *string `json:"description,omitempty"` // an optional informational description.
	Author      *string `json:"author,omitempty"`      // an optional author.
	Website     *string `json:"website,omitempty"`     // an optional website for additional info.
	License     *string `json:"license,omitempty"`     // an optional license governing this package's usage.

	Dependencies *Dependencies `json:"dependencies,omitempty"` // all of the package dependencies.
	Modules      *ast.Modules  `json:"modules,omitempty"`      // a collection of top-level modules.

	Doc *diag.Document `json:"-"` // the document from which this package came.
}

var _ diag.Diagable = (*Package)(nil)

func (s *Package) Where() (*diag.Document, *diag.Location) {
	return s.Doc, nil
}

// Dependencies maps dependency names to the full URL, including version, of the package.
type Dependencies map[tokens.Package]PackageURL

// PackageURL represents a fully qualified "URL-like" reference to an entity, usually another package.  This string
// starts with an optional "protocol" (like https://, git://, etc), followed by an optional "base" part (like
// hub.mu.com/, github.com/, etc), followed by the "name" part (which is just a Name), followed by an optional "#" and
// version number (where version may be "latest", a semantic version range, or a Git SHA hash).
type PackageURL string
