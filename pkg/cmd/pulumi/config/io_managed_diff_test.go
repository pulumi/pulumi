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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestManagedEnvironmentNotice(t *testing.T) {
	t.Parallel()

	const header = "Environment p/old: local changes based on revision 7; an update publishes them, a preview does not\n"
	cases := []struct {
		name            string
		published       string
		changeRequestID string
		expected        string
	}{
		{
			name:            "changed",
			published:       "values:\n  pulumiConfig:\n    otherproj:key: old\n    otherproj:gone: 1\n",
			changeRequestID: "cr-1",
			expected: header +
				`    ` + colors.SpecDelete + `- values.pulumiConfig["otherproj:gone"]: 1` + colors.Reset + "\n" +
				`    ` + colors.SpecUpdate + `~ values.pulumiConfig["otherproj:key"]: "old" -> "env"` + colors.Reset + "\n",
		},
		{
			name:            "no value changes",
			published:       "values: {pulumiConfig: {otherproj:key: env}}\n",
			changeRequestID: "cr-1",
			expected: "Environment p/old: local changes based on revision 7 (no value changes); " +
				"an update publishes them, a preview does not\n",
		},
		{
			name:            "diff error",
			published:       "- not a mapping\n",
			changeRequestID: "cr-1",
			expected: "using local changes to environment p/old (based on revision 7); " +
				"an update publishes them, a preview does not\n",
		},
		{
			name:      "unchanged",
			published: "values:\n  pulumiConfig:\n    otherproj:key: env\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			stack, _ := newManagedStack(t, func(
				string, string, string, []byte,
			) (*backend.ManagedEnvironmentEvaluation, apitype.EnvironmentDiagnostics, error) {
				return &backend.ManagedEnvironmentEvaluation{
					Environment: &esc.Environment{Properties: map[string]esc.Value{
						"pulumiConfig": esc.NewValue(map[string]esc.Value{"otherproj:key": esc.NewValue("env")}),
					}},
					BaseRevision:        7,
					PublishedDefinition: []byte(c.published),
					ChangeRequestID:     c.changeRequestID,
				}, nil, nil
			})

			cfg, err := getStackConfigurationFromProjectStack(t.Context(), stack, &workspace.Project{Name: "otherproj"},
				nil, loadTestProjectStack(t, managedStackYAML), nil)
			require.NoError(t, err)
			assert.Equal(t, c.expected, cfg.ManagedEnvironmentNotice)
			assert.Equal(t, c.changeRequestID, cfg.ManagedEnvironment.ChangeRequestID)
		})
	}
}
