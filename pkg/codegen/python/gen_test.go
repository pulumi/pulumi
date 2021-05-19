package python

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

var pathTests = []struct {
	input    string
	expected string
}{
	{".", "."},
	{"", "."},
	{"../", ".."},
	{"../..", "..."},
	{"../../..", "...."},
	{"something", ".something"},
	{"../parent", "..parent"},
	{"../../module", "...module"},
}

func TestRelPathToRelImport(t *testing.T) {
	for _, tt := range pathTests {
		t.Run(tt.input, func(t *testing.T) {
			result := relPathToRelImport(tt.input)
			if result != tt.expected {
				t.Errorf("expected \"%s\"; got \"%s\"", tt.expected, result)
			}
		})
	}
}

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
				filepath.Join("pulumi_example", "resource.py"),
				filepath.Join("pulumi_example", "other_resource.py"),
				filepath.Join("pulumi_example", "arg_function.py"),
			},
		},
		{
			"Simple schema with local resource properties and custom Python package name",
			"simple-resource-schema-custom-pypackage-name",
			[]string{
				filepath.Join("custom_py_package", "resource.py"),
				filepath.Join("custom_py_package", "other_resource.py"),
				filepath.Join("custom_py_package", "arg_function.py"),
			},
		},
		{
			"External resource schema",
			"external-resource-schema",
			[]string{
				filepath.Join("pulumi_example", "_inputs.py"),
				filepath.Join("pulumi_example", "arg_function.py"),
				filepath.Join("pulumi_example", "cat.py"),
				filepath.Join("pulumi_example", "component.py"),
				filepath.Join("pulumi_example", "workload.py"),
			},
		},
		{
			"Simple schema with enum types",
			"simple-enum-schema",
			[]string{
				filepath.Join("pulumi_plant", "_enums.py"),
				filepath.Join("pulumi_plant", "_inputs.py"),
				filepath.Join("pulumi_plant", "outputs.py"),
				filepath.Join("pulumi_plant", "__init__.py"),
				filepath.Join("pulumi_plant", "tree", "__init__.py"),
				filepath.Join("pulumi_plant", "tree", "v1", "_enums.py"),
				filepath.Join("pulumi_plant", "tree", "v1", "__init__.py"),
				filepath.Join("pulumi_plant", "tree", "v1", "rubber_tree.py"),
				filepath.Join("pulumi_plant", "tree", "v1", "nursery.py"),
			},
		},
		{
			"Simple schema with plain properties",
			"simple-plain-schema",
			[]string{
				filepath.Join("pulumi_example", "_inputs.py"),
				filepath.Join("pulumi_example", "component.py"),
				filepath.Join("pulumi_example", "outputs.py"),
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
			lang := "python"

			test.RewriteFilesWhenPulumiAccept(t, dir, lang, files)

			expectedFiles, err := test.LoadFiles(dir, lang, tt.expectedFiles)
			assert.NoError(t, err)

			test.ValidateFileEquality(t, files, expectedFiles)
		})
	}
}

func TestGenerateOutputFuncs(t *testing.T) {
	testDir := filepath.Join("..", "internal", "test", "testdata", "output-funcs")

	examples := []string{
		"listStorageAccountKeys",
		"getClientConfig",
		"getIntegrationRuntimeObjectMetadatum",
		"funcWithConstInput",
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

		mod := &modContext{}
		funcCode, err := mod.genFunction(fun)
		if err != nil {
			return err
		}

		writer.Write([]byte(funcCode))
		return nil
	}

	for _, ex := range examples {
		t.Run(ex, func(t *testing.T) {
			inputFile := filepath.Join(testDir, fmt.Sprintf("%s.json", ex))
			expectedOutputFile := filepath.Join(testDir, fmt.Sprintf("%s.py", ex))
			test.ValidateFileTransformer(t, inputFile, expectedOutputFile, gen)
		})
	}
}
