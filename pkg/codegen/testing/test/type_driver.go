// Copyright 2021-2024, Pulumi Corporation.
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

package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type typeTestCase struct {
	Expected map[string]interface{} `json:"expected"`
}

type typeTestImporter int

func (typeTestImporter) ImportDefaultSpec(bytes json.RawMessage) (interface{}, error) {
	return bytes, nil
}

func (typeTestImporter) ImportPropertySpec(bytes json.RawMessage) (interface{}, error) {
	var test typeTestCase
	if err := json.Unmarshal([]byte(bytes), &test); err != nil {
		return nil, err
	}
	return &test, nil
}

func (typeTestImporter) ImportObjectTypeSpec(bytes json.RawMessage) (interface{}, error) {
	return bytes, nil
}

func (typeTestImporter) ImportResourceSpec(bytes json.RawMessage) (interface{}, error) {
	return bytes, nil
}

func (typeTestImporter) ImportFunctionSpec(bytes json.RawMessage) (interface{}, error) {
	return bytes, nil
}

func (typeTestImporter) ImportPackageSpec(bytes json.RawMessage) (interface{}, error) {
	return bytes, nil
}

type NewTypeNameGeneratorFunc func(pkg *schema.Package) TypeNameGeneratorFunc

type TypeNameGeneratorFunc func(t schema.Type) string

func TestTypeNameCodegen(t *testing.T, language string, newTypeNameGenerator NewTypeNameGeneratorFunc) { //nolint:revive
	// Read in, decode, and import the schema.
	schemaBytes, err := os.ReadFile(filepath.FromSlash("../testing/test/testdata/types.json"))
	require.NoError(t, err)

	var pkgSpec schema.PackageSpec
	err = json.Unmarshal(schemaBytes, &pkgSpec)
	require.NoError(t, err)

	pkg, err := schema.ImportSpec(
		pkgSpec,
		map[string]schema.Language{"test": typeTestImporter(0)},
		schema.ValidationOptions{},
	)

	require.NoError(t, err)

	typeName := newTypeNameGenerator(pkg)

	if !cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT")) {
		runTests := func(where string, props []*schema.Property, inputShape bool) {
			for _, p := range props {
				p := p
				if testCase, ok := p.Language["test"].(*typeTestCase); ok {
					if expected, ok := testCase.Expected[language]; ok {
						typ := p.Type
						t.Run(where+"/"+p.Name, func(t *testing.T) {
							t.Parallel()

							var expectedName string
							switch expected := expected.(type) {
							case string:
								expectedName = expected
							case map[string]interface{}:
								if inputShape {
									expectedName = expected["input"].(string)
								} else {
									expectedName = expected["plain"].(string)
								}
							}

							assert.Equalf(t, expectedName, typeName(typ),
								"Property '%s' with type %s (inputShape=%t)",
								p.Name, p.Type, inputShape)
						})
					}
				}
			}
		}

		runTests("#/config", pkg.Config, false)

		runTests("#/provider/properties", pkg.Provider.Properties, false)
		runTests("#/provider/inputProperties", pkg.Provider.InputProperties, false)
		if pkg.Provider.StateInputs != nil {
			runTests("#/provider/stateInputs/properties", pkg.Provider.StateInputs.Properties, false)
		}

		for _, typ := range pkg.Types {
			if o, ok := typ.(*schema.ObjectType); ok {
				if o.IsInputShape() {
					continue
				}

				runTests("#/types/"+o.Token+"/properties", o.Properties, false)
				runTests("#/types/"+o.InputShape.Token+"/properties", o.InputShape.Properties, true)
			}
		}
		for _, r := range pkg.Resources {
			runTests("#/resources/"+r.Token+"/properties", r.Properties, false)
			runTests("#/resources/"+r.Token+"/inputProperties", r.InputProperties, false)
			if r.StateInputs != nil {
				runTests("#/resources/"+r.Token+"/properties", r.StateInputs.Properties, false)
			}
		}
		for _, f := range pkg.Functions {
			if f.Inputs != nil {
				runTests("/functions/"+f.Token+"/inputs/properties", f.Inputs.Properties, false)
			}
			if f.ReturnType != nil {
				if objectType, ok := f.ReturnType.(*schema.ObjectType); ok && objectType != nil {
					runTests("/functions/"+f.Token+"/outputs/properties", objectType.Properties, false)
				}
			}
		}
		return
	}

	updateTests := func(props []*schema.Property) {
		for _, p := range props {
			testCase, _ := p.Language["test"].(*typeTestCase)
			if testCase == nil {
				testCase = &typeTestCase{}
				p.Language["test"] = testCase
			}
			if testCase.Expected == nil {
				testCase.Expected = map[string]interface{}{}
			}
			testCase.Expected[language] = typeName(p.Type)
		}
	}

	updateTests(pkg.Config)

	updateTests(pkg.Provider.Properties)
	updateTests(pkg.Provider.InputProperties)
	if pkg.Provider.StateInputs != nil {
		updateTests(pkg.Provider.StateInputs.Properties)
	}

	for _, typ := range pkg.Types {
		if o, ok := typ.(*schema.ObjectType); ok {
			if o.IsInputShape() {
				continue
			}

			updateTests(o.Properties)
			updateTests(o.InputShape.Properties)

			for i, p := range o.Properties {
				testCase := p.Language["test"].(*typeTestCase)
				plain := testCase.Expected[language].(string)
				input := o.InputShape.Properties[i].Language["test"].(*typeTestCase).Expected[language].(string)
				testCase.Expected[language] = map[string]interface{}{
					"plain": plain,
					"input": input,
				}
				o.InputShape.Properties[i].Language["test"] = testCase
			}
		}
	}
	for _, r := range pkg.Resources {
		updateTests(r.Properties)
		updateTests(r.InputProperties)
		if r.StateInputs != nil {
			updateTests(r.StateInputs.Properties)
		}
	}
	for _, f := range pkg.Functions {
		if f.Inputs != nil {
			updateTests(f.Inputs.Properties)
		}
		if f.ReturnType != nil {
			if objectType, ok := f.ReturnType.(*schema.ObjectType); ok && objectType != nil {
				updateTests(objectType.Properties)
			}
		}
	}

	f, err := os.Create(filepath.FromSlash("../testing/test/testdata/types.json"))
	require.NoError(t, err)
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	err = encoder.Encode(pkg)
	require.NoError(t, err)
}
