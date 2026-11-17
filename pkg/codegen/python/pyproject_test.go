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

package python

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func TestGenPyprojectTOMLLicense(t *testing.T) {
	t.Parallel()

	gen := func(t *testing.T, license string) map[string]any {
		pkg := &schema.Package{
			Name:     "example",
			License:  license,
			Language: map[string]any{"python": PackageInfo{}},
		}
		out, err := genPyprojectTOML("test", pkg, "pulumi_example", map[string]string{})
		require.NoError(t, err)
		var parsed map[string]any
		_, err = toml.Decode(out, &parsed)
		require.NoError(t, err)
		return parsed
	}

	t.Run("license is an SPDX string", func(t *testing.T) {
		t.Parallel()
		parsed := gen(t, "Apache-2.0")
		project := parsed["project"].(map[string]any)
		assert.Equal(t, "Apache-2.0", project["license"])

		buildSystem := parsed["build-system"].(map[string]any)
		assert.Contains(t, buildSystem["requires"], "setuptools>=77.0")
	})

	t.Run("no license", func(t *testing.T) {
		t.Parallel()
		parsed := gen(t, "")
		project := parsed["project"].(map[string]any)
		assert.NotContains(t, project, "license")
	})
}
