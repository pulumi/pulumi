// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Package pack contains the core LumiPack metadata types.
package pack

import (
	"io/ioutil"
	"path/filepath"

	"github.com/pulumi/pulumi/pkg/util/contract"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/encoding"
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

// Load reads a package definition from a file
func Load(path string) (*Package, error) {
	contract.Require(path != "", "pkg")

	m, err := marshallerForPath(path)
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var pkg Package
	err = m.Unmarshal(b, &pkg)
	if err != nil {
		return nil, err
	}

	err = pkg.Validate()
	if err != nil {
		return nil, err
	}

	return &pkg, err
}

// Save writes a package defitiniton to a file
func Save(path string, pkg *Package) error {
	contract.Require(path != "", "pkg")
	contract.Require(pkg != nil, "pkg")
	contract.Requiref(pkg.Validate() == nil, "pkg", "Validate()")

	m, err := marshallerForPath(path)
	if err != nil {
		return err
	}

	b, err := m.Marshal(pkg)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, b, 0644)
}

func marshallerForPath(path string) (encoding.Marshaler, error) {
	ext := filepath.Ext(path)
	m, has := encoding.Marshalers[ext]
	if !has {
		return nil, errors.Errorf("no marshaler found for file format '%v'", ext)
	}

	return m, nil
}
