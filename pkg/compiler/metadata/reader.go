// Copyright 2017 Pulumi, Inc. All rights reserved.

package metadata

import (
	"github.com/golang/glog"

	"github.com/pulumi/coconut/pkg/compiler/core"
	"github.com/pulumi/coconut/pkg/compiler/errors"
	"github.com/pulumi/coconut/pkg/diag"
	"github.com/pulumi/coconut/pkg/encoding"
	"github.com/pulumi/coconut/pkg/pack"
	"github.com/pulumi/coconut/pkg/util/contract"
)

// Reader reads a document by decoding/parsing it into its AST form.
type Reader interface {
	core.Phase

	// ReadPackage parses a CocoPack from the given document.  If an error occurs, the return value will be nil.  It
	// is expected that errors are conveyed using the diag.Sink interface.
	ReadPackage(doc *diag.Document) *pack.Package
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
	glog.Infof("Reading CocoPack: %v (len(body)=%v)", doc.File, len(doc.Body))
	contract.Assert(len(doc.Body) != 0)
	if glog.V(2) {
		defer glog.V(2).Infof("Reading CocoPack '%v' completed w/ %v warnings and %v errors",
			doc.File, r.Diag().Warnings(), r.Diag().Errors())
	}

	// We support many file formats.  Detect the file extension and deserialize the contents.
	m, has := encoding.Marshalers[doc.Ext()]
	contract.Assertf(has, "No marshaler registered for this Cocofile extension: %v", doc.Ext())
	pkg, err := encoding.Decode(m, doc.Body)
	if err != nil {
		r.Diag().Errorf(errors.ErrorIllegalProjectSyntax.At(doc), err)
		// TODO[pulumi/coconut#14]: issue an error per issue found in the file with line/col numbers.
		return nil
	}

	// Remember that this package came from this document.
	pkg.Doc = doc

	glog.V(3).Infof("CocoPack %v parsed: name=%v", doc.File, pkg.Name)
	return pkg
}
