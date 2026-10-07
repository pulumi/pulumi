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

package newcmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestMoveConfigIntoEnvironment(t *testing.T) {
	t.Parallel()

	ps := &workspace.ProjectStack{
		Config: config.Map{
			config.MustMakeKey("aws", "region"):         config.NewValue("us-west-2"),
			config.MustMakeKey("project", "replicas"):   config.NewObjectValue(`{"min":1,"max":3}`),
			config.MustMakeKey("project", "dbPassword"): config.NewSecureValue("ciphertext"),
		},
	}

	require.NoError(t, moveConfigIntoEnvironment(ps))

	// The secret stays encrypted in the config block; everything else moved into the environment.
	assert.Equal(t, config.Map{
		config.MustMakeKey("project", "dbPassword"): config.NewSecureValue("ciphertext"),
	}, ps.Config)
	require.True(t, ps.Environment.IsDefinition())

	var definition map[string]any
	require.NoError(t, yaml.Unmarshal(ps.EnvironmentBytes(), &definition))
	assert.Equal(t, map[string]any{
		"values": map[string]any{
			"pulumiConfig": map[string]any{
				"aws:region":       "us-west-2",
				"project:replicas": map[string]any{"min": 1, "max": 3},
			},
		},
	}, definition)

	imports := &workspace.ProjectStack{Environment: workspace.NewEnvironment([]string{"shared"})}
	assert.ErrorContains(t, moveConfigIntoEnvironment(imports), "imports environments")
}
