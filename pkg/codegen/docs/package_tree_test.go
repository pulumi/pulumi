// Copyright 2016-2021, Pulumi Corporation.
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

package docs

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func TestGeneratePackageTree(t *testing.T) {
	t.Parallel()

	dctx := newDocGenContext()
	initTestPackageSpec(t)

	schemaPkg, err := schema.ImportSpec(testPackageSpec, nil)
	assert.NoError(t, err, "importing spec")

	dctx.initialize(unitTestTool, schemaPkg)
	pkgTree, err := dctx.generatePackageTree()
	if err != nil {
		t.Errorf("Error generating the package tree for package %s: %v", schemaPkg.Name, err)
	}

	assert.NotEmpty(t, pkgTree, "Package tree was empty")

	t.Run("ValidatePackageTreeTopLevelItems", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, entryTypeModule, pkgTree[0].Type)
		assert.Equal(t, entryTypeModule, pkgTree[1].Type)
		assert.Equal(t, entryTypeResource, pkgTree[2].Type)
		assert.Equal(t, entryTypeResource, pkgTree[3].Type)
		assert.Equal(t, entryTypeFunction, pkgTree[4].Type)
	})

	t.Run("ValidateSortOrder", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "module", pkgTree[0].Name)
		assert.Equal(t, "module2", pkgTree[1].Name)
		assert.Equal(t, "PackageLevelResource", pkgTree[2].Name)
		assert.Equal(t, "Provider", pkgTree[3].Name)
		assert.Equal(t, "getPackageResource", pkgTree[4].Name)
	})

	t.Run("ValidatePackageTreeModuleChildren", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, 2, len(pkgTree[0].Children))
		children := pkgTree[0].Children
		assert.Equal(t, entryTypeResource, children[0].Type)
		assert.Equal(t, entryTypeFunction, children[1].Type)
	})
}
