// nolint: lll
package nodejs

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
	"github.com/stretchr/testify/assert"
)

func TestGeneratePackage(t *testing.T) {
	tests := []struct {
		name          string
		schemaDir     string
		expectedFiles []string
	}{
		{
			"Simple schema with local resource properties",
			"simple-resource-schema",
			[]string{
				"resource.ts",
				"otherResource.ts",
				"argFunction.ts",
			},
		},
		{
			"Simple schema with enum types",
			"simple-enum-schema",
			[]string{
				"index.ts",
				"tree/v1/rubberTree.ts",
				"tree/v1/nursery.ts",
				"tree/v1/index.ts",
				"tree/index.ts",
				"types/input.ts",
				"types/output.ts",
				"types/index.ts",
				"types/enums/index.ts",
				"types/enums/tree/index.ts",
				"types/enums/tree/v1/index.ts",
			},
		},
		{
			"External resource schema",
			"external-resource-schema",
			[]string{
				"index.ts",
				"argFunction.ts",
				"cat.ts",
				"component.ts",
				"workload.ts",
				"types/index.ts",
				"types/input.ts",
				"types/output.ts",
			},
		},
		{
			"Simple schema with plain properties",
			"simple-plain-schema",
			[]string{
				"component.ts",
				"doFoo.ts",
				"types/input.ts",
				"types/output.ts",
				"types/index.ts",
			},
		},
		{
			"Repro for #6957",
			"plain-schema-gh6957",
			[]string{
				"staticPage.ts",
				"types/input.ts",
				"types/output.ts",
				"types/index.ts",
			},
		},
	}
	testDir := filepath.Join("..", "internal", "test", "testdata")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := test.GeneratePackageFilesFromSchema(
				filepath.Join(testDir, tt.schemaDir, "schema.json"), GeneratePackage)
			assert.NoError(t, err)

			dir := filepath.Join(testDir, tt.schemaDir)
			lang := "nodejs"

			test.RewriteFilesWhenPulumiAccept(t, dir, lang, files)

			expectedFiles, err := test.LoadFiles(filepath.Join(testDir, tt.schemaDir), lang, tt.expectedFiles)
			assert.NoError(t, err)

			test.ValidateFileEquality(t, files, expectedFiles)
		})
	}
}
