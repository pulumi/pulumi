// Copyright 2016-2020, Pulumi Corporation.
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

package importer

import (
	"bytes"
	"io"

	"github.com/hashicorp/hcl/v2"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// A NameTable maps URNs to language-specific variable names.
type NameTable map[resource.URN]string

// A DiagnosticsError captures HCL2 diagnostics.
type DiagnosticsError struct {
	diagnostics         hcl.Diagnostics
	newDiagnosticWriter func(w io.Writer, width uint, color bool) hcl.DiagnosticWriter
}

func NewDiagnosticsError(
	diagnostics hcl.Diagnostics,
	newDiagnosticWriter func(w io.Writer, width uint, color bool) hcl.DiagnosticWriter) *DiagnosticsError {
	return &DiagnosticsError{
		diagnostics:         diagnostics,
		newDiagnosticWriter: newDiagnosticWriter,
	}
}

func NewDiagnosticsErrorFromParser(parser *syntax.Parser) *DiagnosticsError {
	return NewDiagnosticsError(parser.Diagnostics, parser.NewDiagnosticWriter)
}

func (e *DiagnosticsError) Diagnostics() hcl.Diagnostics {
	return e.diagnostics
}

// NewDiagnosticWriter returns an hcl.DiagnosticWriter that can be used to render the error's diagnostics.
func (e *DiagnosticsError) NewDiagnosticWriter(w io.Writer, width uint, color bool) hcl.DiagnosticWriter {
	return e.newDiagnosticWriter(w, width, color)
}

func (e *DiagnosticsError) Error() string {
	var text bytes.Buffer
	err := e.NewDiagnosticWriter(&text, 0, false).WriteDiagnostics(e.diagnostics)
	contract.IgnoreError(err)
	return text.String()
}

func (e *DiagnosticsError) String() string {
	return e.Error()
}
