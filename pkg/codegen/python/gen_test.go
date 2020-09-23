package python

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v2/codegen/schema"
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
		name       string
		schemaFile string
		wantErr    bool
		validator  func(files map[string][]byte)
	}{
		{
			"Simple schema with local resource properties",
			"schema-simple.json",
			false,
			func(files map[string][]byte) {
				assert.Contains(t, files, filepath.Join("pulumi_example", "resource.py"))
				assert.Contains(t, files, filepath.Join("pulumi_example", "other_resource.py"))

				for fileName, file := range files {
					if fileName == filepath.Join("pulumi_example", "other_resource.py") {
						// Correct import for local resource
						assert.Contains(t, string(file), "from . import Resource")
						// Correct type for resource input property
						assert.Contains(t, string(file), "foo: Optional[pulumi.Input['Resource']] = None,")
						// Correct type for resource property
						assert.Contains(t, string(file), "def foo(self) -> pulumi.Output[Optional['Resource']]:")
					}
					if fileName == filepath.Join("pulumi_example", "arg_function.py") {
						// Correct type for function arg
						assert.Contains(t, string(file), "arg1: Optional['Resource'] = None")
						// Correct result type for resource ref
						assert.Contains(t, string(file), "if result and not isinstance(result, Resource):")
						// Correct type for result property
						assert.Contains(t, string(file), "def result(self) -> Optional['Resource']:")
					}
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Read in, decode, and import the schema.
			schemaBytes, err := ioutil.ReadFile(
				filepath.Join("..", "internal", "test", "testdata", tt.schemaFile))
			assert.NoError(t, err)

			var pkgSpec schema.PackageSpec
			err = json.Unmarshal(schemaBytes, &pkgSpec)
			assert.NoError(t, err)

			pkg, err := schema.ImportSpec(pkgSpec, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ImportSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			files, err := GeneratePackage("test", pkg, nil)
			if err != nil {
				panic(err)
			}
			tt.validator(files)
		})
	}
}
