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

// Pulling out some of the repeated strings tokens into constants would harm readability, so we just ignore the
// goconst linter's warning.
//
// nolint: lll, goconst
package docs

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/codegen/python"
	"github.com/pulumi/pulumi/pkg/codegen/schema"
	"github.com/stretchr/testify/assert"
)

// TestResourceNestedPropertyPythonCasing tests that the properties
// of a nested object have the expected casing.
func TestResourceNestedPropertyPythonCasing(t *testing.T) {
	schemaPkg, err := schema.ImportSpec(testPackageSpec)
	assert.NoError(t, err, "importing spec")

	modules := generateModulesFromSchemaPackage(unitTestTool, schemaPkg)
	mod := modules["module"]
	for _, r := range mod.resources {
		nestedTypes := mod.genNestedTypes(r, true)
		if len(nestedTypes) == 0 {
			t.Error("did not find any nested types")
			return
		}

		n := nestedTypes[0]
		assert.Equal(t, "SomeResourceOptions", n.Name, "got %v instead of SomeResourceOptions", n.Name)

		pyProps := n.Properties["python"]
		nestedObject, ok := testPackageSpec.Types["prov:module/SomeResourceOptions:SomeResourceOptions"]
		if !ok {
			t.Error("sample schema package spec does not contain known object type")
			return
		}

		for name := range nestedObject.Properties {
			found := false
			for _, prop := range pyProps {
				if prop.Name == python.PyName(name) {
					found = true
					break
				}
			}

			assert.True(t, found, "expected to find %q", name)
		}
	}
}
