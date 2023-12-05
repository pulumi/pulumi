// Copyright 2016-2023, Pulumi Corporation.
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

package plugin

import (
	"github.com/hashicorp/hcl/v2"

	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

func HclDiagnosticToRPCDiagnostic(diag *hcl.Diagnostic) *codegenrpc.Diagnostic {
	hclPosToPos := func(pos hcl.Pos) *codegenrpc.Pos {
		return &codegenrpc.Pos{
			Line:   int64(pos.Line),
			Column: int64(pos.Column),
			Byte:   int64(pos.Byte),
		}
	}

	var subject *codegenrpc.Range
	if diag.Subject != nil {
		subject = &codegenrpc.Range{
			Filename: diag.Subject.Filename,
			Start:    hclPosToPos(diag.Subject.Start),
			End:      hclPosToPos(diag.Subject.End),
		}
	}

	var context *codegenrpc.Range
	if diag.Context != nil {
		context = &codegenrpc.Range{
			Filename: diag.Context.Filename,
			Start:    hclPosToPos(diag.Context.Start),
			End:      hclPosToPos(diag.Context.End),
		}
	}

	return &codegenrpc.Diagnostic{
		Severity: codegenrpc.DiagnosticSeverity(diag.Severity),
		Summary:  diag.Summary,
		Detail:   diag.Detail,
		Subject:  subject,
		Context:  context,
	}
}

func HclDiagnosticsToRPCDiagnostics(diags []*hcl.Diagnostic) []*codegenrpc.Diagnostic {
	rpcDiagnostics := slice.Prealloc[*codegenrpc.Diagnostic](len(diags))
	for _, diag := range diags {
		rpcDiagnostics = append(rpcDiagnostics, HclDiagnosticToRPCDiagnostic(diag))
	}
	return rpcDiagnostics
}

func RPCDiagnosticToHclDiagnostic(diag *codegenrpc.Diagnostic) *hcl.Diagnostic {
	rpcPosToPos := func(pos *codegenrpc.Pos) hcl.Pos {
		return hcl.Pos{
			Line:   int(pos.Line),
			Column: int(pos.Column),
			Byte:   int(pos.Byte),
		}
	}

	var subject *hcl.Range
	if diag.Subject != nil {
		subject = &hcl.Range{
			Filename: diag.Subject.Filename,
			Start:    rpcPosToPos(diag.Subject.Start),
			End:      rpcPosToPos(diag.Subject.End),
		}
	}

	var context *hcl.Range
	if diag.Context != nil {
		context = &hcl.Range{
			Filename: diag.Context.Filename,
			Start:    rpcPosToPos(diag.Context.Start),
			End:      rpcPosToPos(diag.Context.End),
		}
	}

	return &hcl.Diagnostic{
		Severity: hcl.DiagnosticSeverity(diag.Severity),
		Summary:  diag.Summary,
		Detail:   diag.Detail,
		Subject:  subject,
		Context:  context,
	}
}
