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
	ParseStack(doc *diag.Document, props ast.PropertyBag) *ast.Stack
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
	var w ast.Workspace
	marshaler, has := encoding.Marshalers[doc.Ext()]
	util.AssertMF(has, "No marshaler registered for this workspace extension: %v", doc.Ext())
	if err := marshaler.Unmarshal(doc.Body, &w); err != nil {
		p.Diag().Errorf(errors.ErrorIllegalWorkspaceSyntax.At(doc), err)
		// TODO[marapongo/mu#14]: issue an error per issue found in the file with line/col numbers.
		return nil
	}

	// Remember that this workspace came from this document.
	w.Doc = doc

	glog.V(3).Infof("Workspace settings %v parsed: %v clusters; %v deps",
		doc.File, len(w.Clusters), len(w.Dependencies))
	return &w
}

func (p *parser) ParseStack(doc *diag.Document, props ast.PropertyBag) *ast.Stack {
	glog.Infof("Parsing Mufile: %v (len(body)=%v len(props)=%v)", doc.File, len(doc.Body), len(props))
	if glog.V(2) {
		defer func() {
			glog.V(2).Infof("Parsing Mufile '%v' completed w/ %v warnings and %v errors",
				doc.File, p.Diag().Warnings(), p.Diag().Errors())
		}()
	}

	// Expand templates in the document first and foremost.
	// TODO[marapongo/mu#7]: the order of template expansion is not clear.  The way we've done it right now (i.e.,
	//     performing it right here), we haven't yet type-checked the properties supplied to the stack.  As a result,
	//     there is less compile-time safety.  And furthermore, the properties are in a map rather than being stored
	//     in structured types.  In other words, this is really just a fancy pre-processor, rather than being well-
	//     integrated into the type system.  To do that, however, we'd need to delay processing of templates, which
	//     itself will mess with our ability to parse the document.  This is an area of future thinking.
	// TODO[marapongo/mu#7]: related to this, certain information (like cluster target) isn't even available yet!
	// TODO[marapongo/mu#14]: when we produce precise line/column errors, we'll need to somehow trace back to pre-
	//     template expansion, otherwise the numbers may not make sense to the user.
	var err error
	doc, err = RenderTemplates(doc, p.c.Context().WithProps(props))
	if err != nil {
		p.Diag().Errorf(errors.ErrorBadTemplate.At(doc), err)
		return nil
	}

	// We support many file formats.  Detect the file extension and deserialize the contents.
	var stack ast.Stack
	marshaler, has := encoding.Marshalers[doc.Ext()]
	util.AssertMF(has, "No marshaler registered for this Mufile extension: %v", doc.Ext())
	if err := marshaler.Unmarshal(doc.Body, &stack); err != nil {
		p.Diag().Errorf(errors.ErrorIllegalMufileSyntax.At(doc), err)
		// TODO[marapongo/mu#14]: issue an error per issue found in the file with line/col numbers.
		return nil
	}

	// Remember that this stack came from this document.
	stack.Doc = doc

	glog.V(3).Infof("Mufile %v stack parsed: %v name; %v publics; %v privates",
		doc.File, stack.Name, len(stack.Services.Public), len(stack.Services.Private))
	return &stack
}
