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

package httpstate

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	esc_client "github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
)

// revisionClient records the environment versions it opens.
type revisionClient struct {
	esc_client.Client
	diags  []esc_client.EnvironmentDiagnostic
	opened []string
	read   []string
}

func (c *revisionClient) OpenEnvironment(
	_ context.Context, org string, project string, name string, version string, duration time.Duration,
) (string, []esc_client.EnvironmentDiagnostic, error) {
	c.opened = append(c.opened, fmt.Sprintf("%s/%s/%s@%s for %v", org, project, name, version, duration))
	if len(c.diags) != 0 {
		return "", c.diags, nil
	}
	return "open-id", nil, nil
}

func (c *revisionClient) GetOpenEnvironment(
	_ context.Context, org string, project string, name string, id string,
) (*esc.Environment, error) {
	c.read = append(c.read, fmt.Sprintf("%s/%s/%s %s", org, project, name, id))
	return &esc.Environment{Properties: map[string]esc.Value{"pulumiConfig": esc.NewValue("x")}}, nil
}

func TestOpenEnvironmentRevision(t *testing.T) {
	t.Parallel()

	t.Run("opens the revision and reads it", func(t *testing.T) {
		t.Parallel()

		client := &revisionClient{}
		b := &cloudBackend{escClient: client}
		env, diags, err := b.OpenEnvironmentRevision(t.Context(), "acme", "pulumi-delivery", "program-id", 4, time.Hour)
		require.NoError(t, err)
		assert.Empty(t, diags)
		assert.Equal(t, esc.NewValue("x"), env.Properties["pulumiConfig"])
		assert.Equal(t, []string{"acme/pulumi-delivery/program-id@4 for 1h0m0s"}, client.opened)
		assert.Equal(t, []string{"acme/pulumi-delivery/program-id open-id"}, client.read)
	})

	t.Run("diagnostics are returned without reading", func(t *testing.T) {
		t.Parallel()

		client := &revisionClient{diags: []esc_client.EnvironmentDiagnostic{{Summary: "bad"}}}
		b := &cloudBackend{escClient: client}
		env, diags, err := b.OpenEnvironmentRevision(t.Context(), "acme", "pulumi-delivery", "program-id", 4, time.Hour)
		require.NoError(t, err)
		assert.Nil(t, env)
		assert.Equal(t, apitype.EnvironmentDiagnostics{{Summary: "bad"}}, diags)
		assert.Empty(t, client.read)
	})
}
