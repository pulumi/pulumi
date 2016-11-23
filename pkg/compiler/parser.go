// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/encoding"
	"github.com/marapongo/mu/pkg/errors"
	"github.com/marapongo/mu/pkg/util"
)

// Parse transforms a program's input text into a parse tree.
type Parser interface {
	core.Phase

	// ParseStack parses a Mufile from the given document.  If an error occurs, the return value will be nil.  It is
	// expected that errors are conveyed using the diag.Sink interface.
	ParseStack(doc *diag.Document) *ast.Stack
	// ParseWorkspace parses workspace settings from the given document.  If an error occurs, the return value will be
	// nil.  It is expected that errors are conveyed using the diag.Sink interface.
	ParseWorkspace(doc *diag.Document) *ast.Workspace
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

func (p *parser) ParseWorkspace(doc *diag.Document) *ast.Workspace {
	glog.Infof("Parsing workspace settings: %v (len(body)=%v)", doc.File, len(doc.Body))
	if glog.V(2) {
		defer func() {
			glog.V(2).Infof("Parsing workspace settings '%v' completed w/ %v warnings and %v errors",
				doc.File, p.Diag().Warnings(), p.Diag().Errors())
		}()
	}

	// We support many file formats.  Detect the file extension and deserialize the contents.
	// TODO: we need to expand templates as part of the parsing process
	var w ast.Workspace
	marshaler, has := encoding.Marshalers[doc.Ext()]
	util.AssertMF(has, "No marshaler registered for this workspace extension: %v", doc.Ext())
	if err := marshaler.Unmarshal(doc.Body, &w); err != nil {
		p.Diag().Errorf(errors.ErrorIllegalWorkspaceSyntax.WithDocument(doc), err)
		// TODO: it would be great if we issued an error per issue found in the file with line/col numbers.
		return nil
	}

	glog.V(3).Infof("Workspace settings %v parsed: %v clusters; %v deps",
		doc.File, len(w.Clusters), len(w.Dependencies))
	return &w
}
func (p *parser) ParseStack(doc *diag.Document) *ast.Stack {
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
	util.AssertMF(has, "No marshaler registered for this Mufile extension: %v", doc.Ext())
	if err := marshaler.Unmarshal(doc.Body, &stack); err != nil {
		p.Diag().Errorf(errors.ErrorIllegalMufileSyntax.WithDocument(doc), err)
		// TODO: it would be great if we issued an error per issue found in the file with line/col numbers.
		return nil
	}

	glog.V(3).Infof("Mufile %v stack parsed: %v name; %v publics; %v privates",
		doc.File, stack.Name, len(stack.Services.Public), len(stack.Services.Private))
	return &stack
}
