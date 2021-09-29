package docs

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func TestGeneratePackageTree(t *testing.T) {
	initTestPackageSpec(t)

	schemaPkg, err := schema.ImportSpec(testPackageSpec, nil)
	assert.NoError(t, err, "importing spec")

	Initialize(unitTestTool, schemaPkg)

	pkgTree, err := GeneratePackageTree()
	if err != nil {
		t.Errorf("Error generating the package tree for package %s: %v", schemaPkg.Name, err)
	}

	assert.NotEmpty(t, pkgTree, "Package tree was empty")

	t.Run("ValidatePackageTreeTopLevelItems", func(t *testing.T) {
		assert.Equal(t, entryTypeModule, pkgTree[0].Type)
		assert.Equal(t, entryTypeResource, pkgTree[1].Type)
		assert.Equal(t, entryTypeResource, pkgTree[2].Type)
		assert.Equal(t, entryTypeFunction, pkgTree[3].Type)
	})

	t.Run("ValidatePackageTreeModuleChildren", func(t *testing.T) {
		assert.Equal(t, 2, len(pkgTree[0].Children))
		children := pkgTree[0].Children
		assert.Equal(t, entryTypeResource, children[0].Type)
		assert.Equal(t, entryTypeFunction, children[1].Type)
	})
}
