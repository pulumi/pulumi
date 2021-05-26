package dotnet

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
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
			},
		},
	}
	testDir := filepath.Join("..", "internal", "test", "testdata")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := test.GeneratePackageFilesFromSchema(
				filepath.Join(testDir, tt.schemaDir, "schema.json"), GeneratePackage)
			assert.NoError(t, err)

			expectedFiles, err := test.LoadFiles(filepath.Join(testDir, tt.schemaDir), "dotnet", tt.expectedFiles)
			assert.NoError(t, err)

			test.ValidateFileEquality(t, files, expectedFiles)
		})
	}
}

func TestGenerateOutputFuncs(t *testing.T) {
	testDir := filepath.Join("..", "internal", "test", "testdata", "output-funcs")

	examples := []string{
		"listStorageAccountKeys",
		"funcWithDefaultValue",
		"funcWithAllOptionalInputs",
	}

	gen := func(reader io.Reader, writer io.Writer) error {
		var pkgSpec schema.PackageSpec
		err := json.NewDecoder(reader).Decode(&pkgSpec)
		if err != nil {
			return err
		}
		pkg, err := schema.ImportSpec(pkgSpec, nil)
		if err != nil {
			return err
		}
		fun := pkg.Functions[0]
		mod := &modContext{
			pkg: pkg,
			namespaces: map[string]string{
				"azure-native":   "AzureNative",
				"madeup-package": "MadeupPackage",
			},
		}
		code, err := mod.genFunctionFileCode(fun)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(writer, "%s", code)
		return err
	}

	for _, ex := range examples {
		t.Run(ex, func(t *testing.T) {
			inputFile := filepath.Join(testDir, fmt.Sprintf("%s.json", ex))
			expectedOutputFile := filepath.Join(testDir, "dotnet", fmt.Sprintf("%s.cs", ex))
			test.ValidateFileTransformer(t, inputFile, expectedOutputFile, gen)
		})
	}
}
