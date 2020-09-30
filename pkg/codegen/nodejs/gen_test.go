package nodejs

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v2/codegen/schema"
	"github.com/stretchr/testify/assert"
)

func TestGeneratePackage(t *testing.T) {
	tests := []struct {
		name          string
		schemaDir     string
		expectedFiles []string
		wantErr       bool
		validator     func(files, expectedFiles map[string][]byte)
	}{
		{
			"Simple schema with local resource properties",
			"simple-resource-schema",
			[]string{
				"resource.ts",
				"otherResource.ts",
				"argFunction.ts",
			},
			false,
			func(files, expectedFiles map[string][]byte) {
				for name, file := range expectedFiles {
					assert.Contains(t, files, name)
					assert.Equal(t, file, files[name])
				}
			},
		},
	}
	testDir := filepath.Join("..", "internal", "test", "testdata")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Read in, decode, and import the schema.
			schemaBytes, err := ioutil.ReadFile(
				filepath.Join(testDir, tt.schemaDir, "schema.json"))
			assert.NoError(t, err)

			expectedFiles := map[string][]byte{}
			for _, file := range tt.expectedFiles {
				fileBytes, err := ioutil.ReadFile(filepath.Join(testDir, tt.schemaDir, file))
				assert.NoError(t, err)

				expectedFiles[file] = fileBytes
			}

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
			tt.validator(files, expectedFiles)
		})
	}
}
