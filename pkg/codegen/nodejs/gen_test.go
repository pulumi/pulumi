// nolint: lll
package nodejs

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v2/codegen/internal/test"
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
				"tree/v1/rubberTree.ts",
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
			"Only nested enums",
			"enum-nested-only-schema",
			[]string{
				"types/enums/index.ts",
				"types/enums/tree/index.ts",
				"types/enums/tree/v1/index.ts",
			},
		},
	}
	testDir := filepath.Join("..", "internal", "test", "testdata")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := test.GeneratePackageFilesFromSchema(
				filepath.Join(testDir, tt.schemaDir, "schema.json"), GeneratePackage)
			assert.NoError(t, err)

			expectedFiles, err := test.LoadFiles(filepath.Join(testDir, tt.schemaDir), "nodejs", tt.expectedFiles)
			assert.NoError(t, err)

			test.ValidateFileEquality(t, files, expectedFiles)
		})
	}
}

func TestMakeSafeEnumName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"red", "Red"},
		{"snake_cased_name", "Snake_cased_name"},
		{"8", "_8"},
		{"Microsoft-Windows-Shell-Startup", "Microsoft_Windows_Shell_Startup"},
		{"Microsoft.Batch", "Microsoft_Batch"},
		{"readonly", "Readonly"},
		{"SystemAssigned, UserAssigned", "SystemAssigned__UserAssigned"},
		{"Dev(NoSLA)_Standard_D11_v2", "Dev_NoSLA__Standard_D11_v2"},
		{"Standard_E8as_v4+1TB_PS", "Standard_E8as_v4_1TB_PS"},
	}
	for _, tt := range tests {
		result := makeSafeEnumName(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}
