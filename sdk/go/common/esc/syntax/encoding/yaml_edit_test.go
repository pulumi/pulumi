// Copyright 2023, Pulumi Corporation.
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

package encoding

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v3"
)

func TestYAMLEdit(t *testing.T) {
	unmarshalYAML := func(path string, into any) {
		bytes, err := os.ReadFile(path)
		require.NoError(t, err)

		err = yaml.Unmarshal(bytes, into)
		require.NoError(t, err)
	}

	path := filepath.Join("testdata", "yaml-edit")
	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	for _, e := range entries {
		t.Run(e.Name(), func(t *testing.T) {
			basepath := filepath.Join(path, e.Name())
			yamlpath := filepath.Join(basepath, "doc.yaml")
			editsPath := filepath.Join(basepath, "edits.yaml")
			expectedPath := filepath.Join(basepath, "expected.yaml")

			var doc yaml.Node
			unmarshalYAML(yamlpath, &doc)
			if doc.Kind != yaml.DocumentNode {
				doc = yaml.Node{
					Kind:    yaml.DocumentNode,
					Content: []*yaml.Node{{}},
				}
			}

			var edits struct {
				Edits []map[string]yaml.Node `yaml:"edits"`
			}
			unmarshalYAML(editsPath, &edits)

			docNode := YAMLSyntax{Node: &doc}
			for _, edit := range edits.Edits {
				pathStr := maps.Keys(edit)[0]

				path, err := resource.ParsePropertyPath(pathStr)
				require.NoError(t, err)

				value := edit[pathStr]
				if value.Tag == "!!null" {
					err = docNode.Delete(nil, path)
				} else {
					_, err = docNode.Set(nil, path, edit[pathStr])
				}
				assert.NoError(t, err)
			}

			bytes, err := yaml.Marshal(docNode.Node)
			require.NoError(t, err)

			if accept() {
				err = os.WriteFile(expectedPath, bytes, 0o600)
				require.NoError(t, err)

				return
			}

			expectedBytes, err := os.ReadFile(expectedPath)
			require.NoError(t, err)

			assert.Equal(t, string(expectedBytes), string(bytes))
		})
	}
}
