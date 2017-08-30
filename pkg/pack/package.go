// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Package pack contains the core LumiPack metadata types.
package pack

import (
	"github.com/pulumi/pulumi-fabric/pkg/diag"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
)

// Package is a top-level package definition.
type Package struct {
	Name    tokens.PackageName `json:"name"`    // a required fully qualified name.
	Runtime string             `json:"runtime"` // a required runtime that executes code.

	Description *string `json:"description,omitempty"` // an optional informational description.
	Author      *string `json:"author,omitempty"`      // an optional author.
	Website     *string `json:"website,omitempty"`     // an optional website for additional info.
	License     *string `json:"license,omitempty"`     // an optional license governing this package's usage.

	Analyzers *Analyzers `json:"analyzers,omitempty"` // any analyzers enabled for this project.

	Doc *diag.Document `json:"-"` // the document from which this package came.
}

var _ diag.Diagable = (*Package)(nil)

func (s *Package) Where() (*diag.Document, *diag.Location) {
	return s.Doc, nil
}

// Analyzers is a list of analyzers to run on this project.
type Analyzers []tokens.QName
