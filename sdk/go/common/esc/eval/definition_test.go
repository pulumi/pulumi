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

package eval

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func TestDefinitionPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		doc      string
		path     string
		expected string
	}{
		{"empty document", "", "pulumiConfig.foo", "values.pulumiConfig.foo"},
		{"plain value", "config: {}", "foo.bar", "values.foo.bar"},
		{"legacy section", "values: {pulumiConfig: {}}", "pulumiConfig.foo", "values.pulumiConfig.foo"},
		{"top-level section", "pulumiConfig: {}", "pulumiConfig.foo", "pulumiConfig.foo"},
		{"top-level alias", "values: {a: b}\nconfig: {}", "pulumiConfig.foo", "config.foo"},
		{"top-level section, other case", "EnvironmentVariables: {}", "environmentVariables.A", "EnvironmentVariables.A"},
		{"legacy wins", "values: {files: {}}\nfiles: {}", "files.A", "values.files.A"},
		{"undefined section", "values: {a: b}", "files.A", "values.files.A"},
		{"config is not a value alias", "config: {}", "config.foo", "values.config.foo"},
		{"imports", "imports: [a]", "imports[0]", "imports[0]"},
		{"environments", "environments: [a]", "imports[0]", "environments[0]"},
		{"imports wins", "environments: [a]\nimports: [b]", "imports[0]", "imports[0]"},
		{"no imports", "values: {}", "imports[0]", "imports[0]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			var doc yaml.Node
			require.NoError(t, yaml.Unmarshal([]byte(c.doc), &doc))

			path, err := resource.ParsePropertyPath(c.path)
			require.NoError(t, err)

			assert.Equal(t, c.expected, DefinitionPath(&doc, path).String())
		})
	}
}
