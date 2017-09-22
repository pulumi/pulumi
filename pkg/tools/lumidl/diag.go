// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package lumidl

import (
	"go/token"
	"path/filepath"

	"golang.org/x/tools/go/loader"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

type goPos interface {
	Pos() token.Pos
}

// goDiag produces a diagnostics object out of a Go type artifact.
func goDiag(prog *loader.Program, elem goPos, relto string) diag.Diagable {
	pos := prog.Fset.Position(elem.Pos())
	file := pos.Filename
	if relto != "" {
		var err error
		file, err = filepath.Rel(relto, file)
		contract.Assertf(err == nil, "error: %v", err)
	}
	return &goDiagable{
		doc: diag.NewDocument(file),
		loc: &diag.Location{
			Start: diag.Pos{
				Line:   pos.Line,
				Column: pos.Column,
			},
		},
	}
}

type goDiagable struct {
	doc *diag.Document
	loc *diag.Location
}

func (d *goDiagable) Where() (*diag.Document, *diag.Location) {
	return d.doc, d.loc
}
