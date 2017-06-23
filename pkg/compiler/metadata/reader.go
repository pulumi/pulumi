// Copyright 2016-2017, Pulumi Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metadata

import (
	"github.com/golang/glog"

	"github.com/pulumi/lumi/pkg/compiler/core"
	"github.com/pulumi/lumi/pkg/compiler/errors"
	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/encoding"
	"github.com/pulumi/lumi/pkg/pack"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// Reader reads a document by decoding/parsing it into its AST form.
type Reader interface {
	core.Phase

	// ReadPackage parses a LumiPack from the given document.  If an error occurs, the return value will be nil.  It
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
	glog.Infof("Reading LumiPack: %v (len(body)=%v)", doc.File, len(doc.Body))
	contract.Assert(len(doc.Body) != 0)
	if glog.V(2) {
		defer glog.V(2).Infof("Reading LumiPack '%v' completed w/ %v warnings and %v errors",
			doc.File, r.Diag().Warnings(), r.Diag().Errors())
	}

	// We support many file formats.  Detect the file extension and deserialize the contents.
	m, has := encoding.Marshalers[doc.Ext()]
	contract.Assertf(has, "No marshaler registered for this Lumifile extension: %v", doc.Ext())
	pkg, err := encoding.Decode(m, doc.Body)
	if err != nil {
		r.Diag().Errorf(errors.ErrorIllegalProjectSyntax.At(doc), err)
		// TODO[pulumi/lumi#14]: issue an error per issue found in the file with line/col numbers.
		return nil
	}

	// Remember that this package came from this document.
	pkg.Doc = doc

	glog.V(3).Infof("LumiPack %v parsed: name=%v", doc.File, pkg.Name)
	return pkg
}
