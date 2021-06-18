package dotnet

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
				"Resource.cs",
				"OtherResource.cs",
				"ArgFunction.cs",
			},
		},
		{
			"Simple schema with enum types",
			"simple-enum-schema",
			[]string{
				"Tree/V1/RubberTree.cs",
				"Tree/V1/Nursery.cs",
				"Tree/V1/Enums.cs",
				"Enums.cs",
				"Inputs/ContainerArgs.cs",
				"Outputs/Container.cs",
			},
		},
		{
			"External resource schema",
			"external-resource-schema",
			[]string{
				"Inputs/PetArgs.cs",
				"ArgFunction.cs",
				"Cat.cs",
				"Component.cs",
				"Workload.cs",
			},
		},
		{
			"Simple schema with plain properties",
			"simple-plain-schema",
			[]string{
				"Inputs/FooArgs.cs",
				"Component.cs",
				"DoFoo.cs",
			},
		},
		{
			"Repro for #6957",
			"plain-schema-gh6957",
			[]string{
				"StaticPage.cs",
				"Inputs/FooArgs.cs",
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
			lang := "dotnet"

			test.RewriteFilesWhenPulumiAccept(t, dir, lang, files)

			expectedFiles, err := test.LoadFiles(filepath.Join(testDir, tt.schemaDir), lang, tt.expectedFiles)
			assert.NoError(t, err)

			test.ValidateFileEquality(t, files, expectedFiles)
		})
	}
}
