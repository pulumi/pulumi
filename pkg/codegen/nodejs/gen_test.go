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
		name       string
		schemaFile string
		wantErr    bool
		validator  func(files map[string][]byte)
	}{
		{
			"Simple schema with local resource properties",
			"simple-resource-schema.json",
			false,
			func(files map[string][]byte) {
				assert.Contains(t, files, "resource.ts")
				assert.Contains(t, files, "otherResource.ts")

				for fileName, file := range files {
					if fileName == "resource.ts" {
						// Correct parent class
						assert.Contains(t, string(file), "export class Resource extends pulumi.CustomResource {")
						// Remote option not set
						assert.NotContains(t, string(file), "opts.remote = true;")
					}
					if fileName == "otherResource.ts" {
						// Correct parent class
						assert.Contains(t, string(file), "export class OtherResource extends pulumi.ComponentResource {")
						// Remote resource option is set
						assert.Contains(t, string(file), "opts.remote = true;")
						// Correct import for local resource
						assert.Contains(t, string(file), `import {Resource} from "./index";`)
						// Correct type for resource input property
						assert.Contains(t, string(file), "readonly foo?: pulumi.Input<Resource>;")
						// Correct type for resource property
						assert.Contains(t, string(file), "public readonly foo!: pulumi.Output<Resource | undefined>;")
					}
					if fileName == "argFunction.ts" {
						// Correct import for local resource
						assert.Contains(t, string(file), `import {Resource} from "./index";`)
						// Correct type for function arg
						assert.Contains(t, string(file), "readonly arg1?: Resource;")
						// Correct type for result
						assert.Contains(t, string(file), "readonly result?: Resource;")
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
