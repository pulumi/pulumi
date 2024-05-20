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
	"fmt"
	"io"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"

	"github.com/hashicorp/hcl/v2"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// A LangaugeGenerator generates code for a given Pulumi program to an io.Writer.
type LanguageGenerator func(w io.Writer, p *pcl.Program) error

// A NameTable maps URNs to language-specific variable names.
type NameTable map[resource.URN]string

// A DiagnosticsError captures HCL2 diagnostics.
type DiagnosticsError struct {
	diagnostics         hcl.Diagnostics
	newDiagnosticWriter func(w io.Writer, width uint, color bool) hcl.DiagnosticWriter
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

func removeDuplicatePathedValues(pathedValues []PathedLiteralValue) []PathedLiteralValue {
	uniqueValues := make([]PathedLiteralValue, 0)
	occurrences := make(map[string]int)
	for _, pathedValue := range pathedValues {
		occurrences[pathedValue.Value]++
	}

	for _, pathedValue := range pathedValues {
		if occurrences[pathedValue.Value] > 1 {
			continue
		}
		uniqueValues = append(uniqueValues, pathedValue)
	}
	return uniqueValues
}

func createImportState(states []*resource.State, names NameTable) ImportState {
	pathedLiteralValues := make([]PathedLiteralValue, 0)
	for _, state := range states {
		resourceID := state.ID.String()
		if resourceID == "" {
			continue
		}

		name := state.URN.Name()
		pathedLiteralValues = append(pathedLiteralValues, PathedLiteralValue{
			Root:  name,
			Value: resourceID,
			ExpressionReference: &model.ScopeTraversalExpression{
				RootName: name,
				Traversal: hcl.Traversal{
					hcl.TraverseRoot{Name: name},
					hcl.TraverseAttr{Name: "id"},
				},
			},
		})
	}

	// remove duplicates so that if multiple resources have the same ID, we don't refer to one or the other
	// instead, we just maintain the literal value as is.
	valuesWithoutDuplicates := removeDuplicatePathedValues(pathedLiteralValues)
	return ImportState{
		Names:               names,
		PathedLiteralValues: valuesWithoutDuplicates,
	}
}

// GenerateLanguageDefintions generates a list of resource definitions from the given resource states.
func GenerateLanguageDefinitions(w io.Writer, loader schema.Loader, gen LanguageGenerator, states []*resource.State,
	names NameTable,
) error {
	var hcl2Text bytes.Buffer

	importState := createImportState(states, names)
	for i, state := range states {
		hcl2Def, err := GenerateHCL2Definition(loader, state, importState)
		if err != nil {
			return err
		}

		pre := ""
		if i > 0 {
			pre = "\n"
		}
		_, err = fmt.Fprintf(&hcl2Text, "%s%v", pre, hcl2Def)
		contract.IgnoreError(err)
	}

	parser := syntax.NewParser()
	if err := parser.ParseFile(&hcl2Text, "anonymous.pp"); err != nil {
		return err
	}
	if parser.Diagnostics.HasErrors() {
		// HCL2 text generation should always generate proper code.
		return fmt.Errorf("internal error: %w", &DiagnosticsError{
			diagnostics:         parser.Diagnostics,
			newDiagnosticWriter: parser.NewDiagnosticWriter,
		})
	}

	program, diags, err := pcl.BindProgram(parser.Files, pcl.Loader(loader), pcl.AllowMissingVariables)
	if err != nil {
		return err
	}
	if diags.HasErrors() {
		// It is possible that the provided states do not contain appropriately-shaped inputs, so this may be user
		// error.
		return &DiagnosticsError{
			diagnostics:         diags,
			newDiagnosticWriter: program.NewDiagnosticWriter,
		}
	}

	return gen(w, program)
}
