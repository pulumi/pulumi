// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/encoding"
	"github.com/marapongo/mu/pkg/errors"
)

// Parse transforms a program's input text into a parse tree.
type Parser interface {
	core.Phase

	// Parse detects and parses input from the given path.  If an error occurs, the return value will be nil.  It is
	// expected that errors are conveyed using the diag.Sink interface.
	Parse(doc *diag.Document) *ast.Stack
}

func NewParser(c Compiler) Parser {
	return &parser{c}
}

type parser struct {
	c Compiler
}

func (p *parser) Diag() diag.Sink {
	return p.c.Diag()
}

func (p *parser) Parse(doc *diag.Document) *ast.Stack {
	glog.Infof("Parsing Mufile: %v (len(body)=%v)", doc.File, len(doc.Body))
	if glog.V(2) {
		defer func() {
			glog.V(2).Infof("Parsing Mufile '%v' completed w/ %v warnings and %v errors",
				doc.File, p.Diag().Warnings(), p.Diag().Errors())
		}()
	}

	// We support many file formats.  Detect the file extension and deserialize the contents.
	// TODO: we need to expand templates as part of the parsing process
	var stack ast.Stack
	marshaler, has := encoding.Marshalers[doc.Ext()]
	if !has {
		glog.Fatalf("No marshaler registered for this Mufile extension: %v", doc.Ext())
		return nil
	}

	if err := marshaler.Unmarshal(doc.Body, &stack); err != nil {
		p.Diag().Errorf(errors.IllegalMufileSyntax.WithDocument(doc), err)
		// TODO: it would be great if we issued an error per issue found in the file with line/col numbers.
		return nil
	}

	if glog.V(3) {
		glog.V(3).Infof("Mufile %v stack parsed: %v name; %v deps; %v publics; %v privates",
			doc.File, stack.Name, len(stack.Dependencies), len(stack.Services.Public), len(stack.Services.Private))
	}
	return &stack
}
