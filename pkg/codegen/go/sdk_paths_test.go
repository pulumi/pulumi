// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gen

import (
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/modfile"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
)

// The import path we tell users to write must address the SDK we just generated: the module in its
// go.mod plus the directory the package was written to.
func TestSDKPathsMatchGeneratedLayout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		language string
	}{
		{"no go info", `{}`},
		{"import base path", `{"go":{"importBasePath":"github.com/example/File/sdk/go/File"}}`},
		{"module path", `{"go":{"modulePath":"github.com/example/File/sdk/go"}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var spec schema.PackageSpec
			require.NoError(t, json.Unmarshal([]byte(fmt.Sprintf(`{
				"name": "file",
				"namespace": "example",
				"version": "1.0.0",
				"meta": {"supportPack": true},
				"language": %s,
				"resources": {
					"file:index:Thing": {
						"properties": {"x": {"type": "string"}},
						"inputProperties": {"x": {"type": "string"}}
					}
				}
			}`, tt.language)), &spec))

			loader := schema.NewPluginLoader(utils.NewContext(testdataPath))
			pkg, diags, err := schema.BindSpec(spec, loader, schema.ValidationOptions{})
			require.NoError(t, err)
			require.False(t, diags.HasErrors(), diags.Error())

			files, err := GeneratePackage("test", pkg, nil)
			require.NoError(t, err)

			gomod, err := modfile.Parse("go.mod", files["go.mod"], nil)
			require.NoError(t, err)
			assert.Equal(t, gomod.Module.Mod.Path, SDKModulePath(pkg.Reference()))

			var dirs []string
			for file := range files {
				if dir, _, found := strings.Cut(file, "/"); found && !slices.Contains(dirs, dir) {
					dirs = append(dirs, dir)
				}
			}
			require.Len(t, dirs, 1, "expected a single package directory")
			assert.Equal(t, path.Join(gomod.Module.Mod.Path, dirs[0]), SDKImportPath(pkg.Reference()))
		})
	}
}
