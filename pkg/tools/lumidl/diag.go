// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lumidl

import (
	"go/token"
	"path/filepath"

	"golang.org/x/tools/go/loader"

	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/util/contract"
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
		contract.Assert(err != nil)
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
