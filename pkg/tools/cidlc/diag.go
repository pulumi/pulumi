// Copyright 2017 Pulumi, Inc. All rights reserved.

package cidlc

import (
	"go/token"
	"path/filepath"

	"golang.org/x/tools/go/loader"

	"github.com/pulumi/coconut/pkg/diag"
)

type goPos interface {
	Pos() token.Pos
}

// goDiag produces a diagnostics object out of a Go type artifact.
func goDiag(prog *loader.Program, elem goPos, relto string) diag.Diagable {
	pos := prog.Fset.Position(elem.Pos())
	file := pos.Filename
	if relto != "" {
		file, _ = filepath.Rel(relto, file)
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
