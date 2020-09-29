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
			filepath.Join("schema", "simple-enum-schema.json"),
			false,
			func(files map[string][]byte) {
				assert.Contains(t, files, "rubberTree.ts")
				assert.Contains(t, files, "types/input.ts")
				assert.Contains(t, files, "types/enum.ts")

				for fileName, file := range files {
					if fileName == "rubberTree.ts" {
						// Import for enums
						assert.Contains(t, string(file), `import * as enums from "./types/enum";`)
						// Correct references to enum types
						assert.Contains(t, string(file), "readonly type?: pulumi.Input<enums.RubberTreeVariety>;")
						// Correct references to object types
						assert.Contains(t, string(file), "readonly container?: pulumi.Input<inputs.Container>;")
					}
					if fileName == "types/input.ts" {
						// Import for enums
						assert.Contains(t, string(file), `import * as enums from "../types/enum";`)
						// Correct references to enum types
						assert.Contains(t, string(file), "color?: pulumi.Input<enums.ContainerColor | string>;")
						assert.Contains(t, string(file), "size: pulumi.Input<enums.ContainerSize>;")
					}
					if fileName == "types/enum.ts" {
						// Correct string enum definitions
						assert.Contains(t, string(file), `export const redContainerColor: ContainerColor = "red";`)
						assert.Contains(t, string(file), `export type ContainerColor = "red" | "blue" | "yellow";`)
						// Correct integer enum definitions
						assert.Contains(t, string(file), "export const FourInchContainerSize: ContainerSize = 4;")
						assert.Contains(t, string(file), "export type ContainerSize = 4 | 6 | 8;")
						// Correct enum with docstring
						assert.Contains(t, string(file), "/** A burgundy rubber tree. */\nexport const BurgundyRubberTreeVariety: RubberTreeVariety = \"Burgundy\";")
						assert.Contains(t, string(file), `export type RubberTreeVariety = "Burgundy" | "Ruby" | "Tineke";`)
					}
				}
			},
		},
	}
	testDir := filepath.Join("..", "internal", "test", "testdata")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := test.GeneratePackageFilesFromSchema(
				filepath.Join(testDir, tt.schemaDir, "schema.json"), GeneratePackage)
			assert.NoError(t, err)

			expectedFiles, err := test.LoadFiles(filepath.Join(testDir, tt.schemaDir), tt.expectedFiles)
			assert.NoError(t, err)

			test.ValidateFileEquality(t, files, expectedFiles)
		})
	}
}
