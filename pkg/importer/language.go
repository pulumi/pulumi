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
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/zclconf/go-cty/cty"

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
			// a value that has occurred multiple times is not unique
			continue
		}

		uniqueValues = append(uniqueValues, pathedValue)
	}
	return uniqueValues
}

func nextPropertyPath(path hcl.Traversal, key hcl.Traverser) hcl.Traversal {
	return append(path, key)
}

func createPathedValue(
	root string,
	property resource.PropertyValue,
	currentPath hcl.Traversal,
) *PathedLiteralValue {
	if property.IsNull() {
		return nil
	}

	if property.IsString() {
		return &PathedLiteralValue{
			Root:  root,
			Value: property.StringValue(),
			ExpressionReference: &model.ScopeTraversalExpression{
				RootName:  root,
				Traversal: currentPath,
			},
		}
	}

	if property.IsSecret() {
		// unwrap the secret
		secret := property.SecretValue()
		return createPathedValue(root, secret.Element, currentPath)
	}

	return nil
}

func sanitizeName(name string) string {
	return strings.ReplaceAll(name, ".", "_")
}

func createImportState(states []*resource.State, snapshot []*resource.State, names NameTable) ImportState {
	pathedLiteralValues := make([]PathedLiteralValue, 0)
	for _, state := range states {
		resourceID := state.ID.String()
		if resourceID == "" {
			continue
		}

		name := sanitizeName(state.URN.Name())
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

		initialPath := hcl.Traversal{hcl.TraverseRoot{Name: name}}

		for key, value := range state.Outputs {
			if string(key) == "name" || string(key) == "arn" {
				nextPath := nextPropertyPath(initialPath, hcl.TraverseAttr{Name: string(key)})
				if output := createPathedValue(name, value, nextPath); output != nil {
					pathedLiteralValues = append(pathedLiteralValues, *output)
				}
			}
		}
	}

	return ImportState{
		Names:               names,
		PathedLiteralValues: pathedLiteralValues,
		Snapshot:            snapshot,
	}
}

// GenerateLanguageDefintions generates a list of resource definitions for the given resource states. The current stack
// snapshot is also provided in order to allow the importer to resolve package providers.
func GenerateLanguageDefinitions(
	w io.Writer,
	loader schema.Loader,
	gen LanguageGenerator,
	states []*resource.State,
	snapshot []*resource.State,
	names NameTable,
) error {
	generateProgramText := func(importState ImportState) (*pcl.Program, hcl.Diagnostics, error) {
		var hcl2Text bytes.Buffer

		// Keep track of packages we've seen, we assume package names are unique.
		seenPkgs := mapset.NewSet[string]()

		for i, state := range states {
			hcl2Def, pkgDesc, err := GenerateHCL2Definition(loader, state, importState)
			if err != nil {
				return nil, nil, err
			}
			pre := ""
			if i > 0 {
				pre = "\n"
			}

			pkgName := pkgDesc.Name
			if pkgDesc.Replacement != nil {
				pkgName = pkgDesc.Replacement.Name
			}
			if !seenPkgs.Contains(pkgName) {
				seenPkgs.Add(pkgName)

				items := make([]model.BodyItem, 0)
				items = append(items, &model.Attribute{
					Name: "baseProviderName",
					Value: &model.LiteralValueExpression{
						Value: cty.StringVal("\"" + pkgDesc.Name + "\""),
					},
				})
				if pkgDesc.Version != nil {
					items = append(items, &model.Attribute{
						Name: "baseProviderVersion",
						Value: &model.LiteralValueExpression{
							Value: cty.StringVal("\"" + pkgDesc.Version.String() + "\""),
						},
					})
				}
				if pkgDesc.DownloadURL != "" {
					items = append(items, &model.Attribute{
						Name: "baseProviderDownloadUrl",
						Value: &model.LiteralValueExpression{
							Value: cty.StringVal("\"" + pkgDesc.DownloadURL + "\""),
						},
					})
				}
				if pkgDesc.Replacement != nil {
					base64Value := base64.StdEncoding.EncodeToString(pkgDesc.Replacement.Value)

					items = append(items, &model.Block{
						Tokens: syntax.NewBlockTokens("parameterization"),
						Type:   "parameterization",
						Body: &model.Body{
							Items: []model.BodyItem{
								&model.Attribute{
									Name: "name",
									Value: &model.LiteralValueExpression{
										Value: cty.StringVal("\"" + pkgDesc.Replacement.Name + "\""),
									},
								},
								&model.Attribute{
									Name: "version",
									Value: &model.LiteralValueExpression{
										Value: cty.StringVal("\"" + pkgDesc.Replacement.Version.String() + "\""),
									},
								},
								&model.Attribute{
									Name: "value",
									Value: &model.LiteralValueExpression{
										Value: cty.StringVal("\"" + base64Value + "\""),
									},
								},
							},
						},
					})
				}

				pkgBlock := &model.Block{
					Tokens: syntax.NewBlockTokens("package", pkgName),
					Type:   "package",
					Labels: []string{pkgName},
					Body: &model.Body{
						Items: items,
					},
				}
				_, err = fmt.Fprintf(&hcl2Text, "%s%v", pre, pkgBlock)
				contract.IgnoreError(err)
				pre = "\n"
			}

			_, err = fmt.Fprintf(&hcl2Text, "%s%v", pre, hcl2Def)
			contract.IgnoreError(err)
		}

		parser := syntax.NewParser()
		if err := parser.ParseFile(&hcl2Text, "anonymous.pp"); err != nil {
			return nil, nil, err
		}
		if parser.Diagnostics.HasErrors() {
			// HCL2 text generation should always generate proper code.
			return nil, nil, fmt.Errorf("internal error: %w", &DiagnosticsError{
				diagnostics:         parser.Diagnostics,
				newDiagnosticWriter: parser.NewDiagnosticWriter,
			})
		}

		return pcl.BindProgram(parser.Files, pcl.Loader(loader), pcl.AllowMissingVariables)
	}

	importState := createImportState(states, snapshot, names)
	program, diags, err := generateProgramText(importState)
	if err != nil {
		if strings.Contains(err.Error(), "circular reference") {
			// hitting an edge case when guessing references between resources
			// this happens when an input of a _parent_ resource is equal to the ID of a _child_ resource
			// for example importing the following program:
			//    const bucket = new aws.s3.Bucket("my-bucket", {
			//        website: {
			//            indexDocument: "index.html",
			//        },
			//    });
			//
			//    const bucketObject = new aws.s3.BucketObject("index.html", {
			//        bucket: bucket.id
			//    });
			// fallback to the old code path where we don't guess references
			// and instead just generate the code with the outputs as literals
			program, diags, err = generateProgramText(ImportState{Names: names, Snapshot: snapshot})
			if err != nil {
				return nil
			}
		} else {
			return err
		}
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
