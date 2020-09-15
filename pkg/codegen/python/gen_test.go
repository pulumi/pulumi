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
	// Read in, decode, and import the schema.
	schemaBytes, err := ioutil.ReadFile(filepath.Join("..", "internal", "test", "testdata", "schema-simple.json"))
	if err != nil {
		panic(err)
	}

	var pkgSpec schema.PackageSpec
	if err = json.Unmarshal(schemaBytes, &pkgSpec); err != nil {
		panic(err)
	}

	pkg, err := schema.ImportSpec(pkgSpec, nil)
	if err != nil {
		t.Errorf("ImportSpec() error = %v", err)
	}

	files, err := GeneratePackage("test", pkg, nil)
	if err != nil {
		panic(err)
	}

	assert.Contains(t, files, filepath.Join("pulumi_example", "resource.py"))
	assert.Contains(t, files, filepath.Join("pulumi_example", "other_resource.py"))
}
