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

package config

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
)

func TestConfigEnvChecksPrivileged(t *testing.T) {
	t.Parallel()

	projectYAML := `name: test
runtime: yaml`

	newParent := func(projectStackYAML string, gotPrivileged *[]bool) *configEnvCmd {
		var newStackYAML string
		return newConfigEnvCmdForTestWithCheckYAMLEnvironment(
			nil,
			&bytes.Buffer{},
			projectYAML,
			projectStackYAML,
			nil,
			func(
				ctx context.Context,
				org string,
				yaml []byte,
				privileged bool,
			) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
				*gotPrivileged = append(*gotPrivileged, privileged)
				return &esc.Environment{}, nil, nil
			},
			&newStackYAML,
		)
	}

	t.Run("add", func(t *testing.T) {
		t.Parallel()

		var gotPrivileged []bool
		add := &configEnvAddCmd{parent: newParent("", &gotPrivileged), yes: true}
		require.NoError(t, add.run(t.Context(), []string{"env"}))
		assert.Equal(t, []bool{true}, gotPrivileged)
	})

	t.Run("rm", func(t *testing.T) {
		t.Parallel()

		var gotPrivileged []bool
		rm := &configEnvRmCmd{parent: newParent("environment:\n  - env\n  - other\n", &gotPrivileged), yes: true}
		require.NoError(t, rm.run(t.Context(), []string{"env"}))
		assert.Equal(t, []bool{true}, gotPrivileged)
	})

	t.Run("config", func(t *testing.T) {
		t.Parallel()

		var gotPrivileged []bool
		parent := newParent("environment:\n  - env\n", &gotPrivileged)
		projectStack, project, stack, err := parent.loadEnvPreamble(t.Context())
		require.NoError(t, err)

		err = listConfig(t.Context(), parent.ssml, &bytes.Buffer{}, project, *stack, projectStack,
			false /*showSecrets*/, false /*jsonOut*/, false /*openEnvironment*/, false /*checkPrivileged*/, "")
		require.NoError(t, err)
		assert.Equal(t, []bool{false}, gotPrivileged)
	})
}
