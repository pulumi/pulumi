// Copyright 2016 Marapongo, Inc. All rights reserved.

package metadata

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/compiler/errors"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/encoding"
	"github.com/marapongo/mu/pkg/pack"
	"github.com/marapongo/mu/pkg/util/contract"
	"github.com/marapongo/mu/pkg/workspace"
)

// Reader reads a document by decoding/parsing it into its AST form.
type Reader interface {
	core.Phase

	// ReadPackage parses a MuPackage from the given document.  If an error occurs, the return value will be nil.  It
	// is expected that errors are conveyed using the diag.Sink interface.
	ReadPackage(doc *diag.Document) *pack.Package
	// ReadWorkspace parses workspace settings from the given document.  If an error occurs, the return value will be
	// nil.  It is expected that errors are conveyed using the diag.Sink interface.
	ReadWorkspace(doc *diag.Document) *workspace.Workspace
}

func NewReader(ctx *core.Context) Reader {
	return &reader{ctx}
}

type reader struct {
	ctx *core.Context
}

func (r *reader) Diag() diag.Sink {
	return r.ctx.Diag
}

func (r *reader) ReadPackage(doc *diag.Document) *pack.Package {
	glog.Infof("Reading MuPackage: %v (len(body)=%v)", doc.File, len(doc.Body))
	if glog.V(2) {
		defer glog.V(2).Infof("Reading MuPackage '%v' completed w/ %v warnings and %v errors",
			doc.File, r.Diag().Warnings(), r.Diag().Errors())
	}

	// We support many file formats.  Detect the file extension and deserialize the contents.
	m, has := encoding.Marshalers[doc.Ext()]
	contract.Assertf(has, "No marshaler registered for this Mufile extension: %v", doc.Ext())
	pkg, err := encoding.Decode(m, doc.Body)
	if err != nil {
		r.Diag().Errorf(errors.ErrorIllegalMufileSyntax.At(doc), err)
		// TODO[marapongo/mu#14]: issue an error per issue found in the file with line/col numbers.
		return nil
	}

	// Remember that this package came from this document.
	pkg.Doc = doc

	glog.V(3).Infof("MuPackage %v parsed: name=%v", doc.File, pkg.Name)
	return pkg
}

func (r *reader) ReadWorkspace(doc *diag.Document) *workspace.Workspace {
	glog.Infof("Reading Muspace settings: %v (len(body)=%v)", doc.File, len(doc.Body))
	if glog.V(2) {
		defer glog.V(2).Infof("Reading Muspace settings '%v' completed w/ %v warnings and %v errors",
			doc.File, r.Diag().Warnings(), r.Diag().Errors())
	}

	// We support many file formats.  Detect the file extension and deserialize the contents.
	var w workspace.Workspace
	marshaler, has := encoding.Marshalers[doc.Ext()]
	contract.Assertf(has, "No marshaler registered for this workspace extension: %v", doc.Ext())
	if err := marshaler.Unmarshal(doc.Body, &w); err != nil {
		r.Diag().Errorf(errors.ErrorIllegalWorkspaceSyntax.At(doc), err)
		// TODO[marapongo/mu#14]: issue an error per issue found in the file with line/col numbers.
		return nil
	}
	glog.V(3).Infof("Muspace settings %v parsed: %v clusters; %v deps",
		doc.File, len(w.Clusters), len(w.Dependencies))

	// Remember that this workspace came from this document.
	w.Doc = doc

	return &w
}
