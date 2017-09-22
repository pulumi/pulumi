// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Package pack contains the core LumiPack metadata types.
package pack

import (
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/tokens"
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

func (pkg *Package) Where() (*diag.Document, *diag.Location) {
	return pkg.Doc, nil
}

func (pkg *Package) Validate() error {
	if pkg.Name == "" {
		return errors.New("package is missing a 'name' attribute")
	}
	if pkg.Runtime == "" {
		return errors.New("package is missing a 'runtime' attribute")
	}
	return nil
}

// Analyzers is a list of analyzers to run on this project.
type Analyzers []tokens.QName
