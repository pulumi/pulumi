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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestDiffEnvironmentDefinitions(t *testing.T) {
	t.Parallel()

	decode := func(t *testing.T, s string) any {
		var v any
		require.NoError(t, yaml.Unmarshal([]byte(s), &v))
		return v
	}

	current := decode(t, `
imports:
  - payments/secrets
values:
  pulumiConfig:
    aws:region: us-west-2
    payments:instanceCount: 3
    payments:removed: gone
    payments:dbPassword:
      fn::secret:
        ciphertext: AAAB
`)
	proposed := decode(t, `
imports:
  - payments/secrets
  - payments/base
values:
  pulumiConfig:
    aws:region: us-west-2
    payments:instanceCount: 6
    payments:added:
      nested: true
    payments:dbPassword:
      fn::secret: hunter2
`)

	assert.Equal(t, []string{
		"+ imports[1]: payments/base",
		"+ values.pulumiConfig.payments:added: {\"nested\":true}",
		"~ values.pulumiConfig.payments:instanceCount: 3 -> 6",
		"- values.pulumiConfig.payments:removed: gone",
	}, diffEnvironmentDefinitions(current, proposed))

	assert.Equal(t, []string{
		"+ values: {\"pulumiConfig\":{\"a:b\":\"[secret]\"}}",
	}, diffEnvironmentDefinitions(nil, decode(t, "values:\n  pulumiConfig:\n    a:b:\n      fn::secret: hunter2\n")))

	assert.Empty(t, diffEnvironmentDefinitions(current, current))
}

func TestPrintStackEnvironmentPreview(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	printStackEnvironmentPreview(&out, &backend.StackEnvironmentSync{
		Environment:       "payments/prod",
		Revision:          3,
		Changed:           true,
		CurrentDefinition: []byte("values:\n  pulumiConfig:\n    payments:instanceCount: 3\n"),
	}, []byte("values:\n  pulumiConfig:\n    payments:instanceCount: 6\n"))
	assert.Equal(t, "Environment payments/prod will be updated (revision 3):\n"+
		"    ~ values.pulumiConfig.payments:instanceCount: 3 -> 6\n\n", out.String())

	out.Reset()
	printStackEnvironmentPreview(&out, &backend.StackEnvironmentSync{Environment: "payments/prod", Revision: 3}, nil)
	assert.Empty(t, out.String())
}

type stackEnvironmentTestHarness struct {
	stack        *backend.MockStack
	projectStack workspace.ProjectStack
	syncCalls    []backend.StackEnvironmentSyncOptions
	published    []byte
	// dryRunOpens makes the dry run open the submitted definition, as the service does once the
	// environment exists.
	dryRunOpens bool
}

// newStackEnvironmentTestHarness returns a stack whose configuration file holds an inline environment
// definition and whose backend publishes it to "project/stack" at revision 3, serving revision 4 once
// published.
func newStackEnvironmentTestHarness(t *testing.T, supported bool) *stackEnvironmentTestHarness {
	t.Helper()
	h := &stackEnvironmentTestHarness{}

	anonymous := map[string]esc.Value{
		"pulumiConfig": esc.NewValue(map[string]esc.Value{"test:source": esc.NewValue("anonymous")}),
	}
	published := map[string]esc.Value{
		"pulumiConfig": esc.NewValue(map[string]esc.Value{"test:source": esc.NewValue("published")}),
	}

	be := &backend.MockStackEnvironmentsBackend{
		MockEnvironmentsBackend: backend.MockEnvironmentsBackend{
			MockBackend: backend.MockBackend{NameF: func() string { return "test" }},
			OpenYAMLEnvironmentF: func(
				context.Context, string, []byte, time.Duration, map[string]string,
			) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
				return &esc.Environment{Properties: anonymous}, nil, nil
			},
		},
		SyncStackEnvironmentF: func(
			_ context.Context, _ backend.Stack, definition []byte, opts backend.StackEnvironmentSyncOptions,
		) (*backend.StackEnvironmentSync, error) {
			if !supported {
				return nil, backend.ErrStackEnvironmentSyncUnsupported
			}
			h.syncCalls = append(h.syncCalls, opts)
			res := &backend.StackEnvironmentSync{
				Environment:       "project/stack",
				PreviousRevision:  3,
				Revision:          3,
				Changed:           true,
				CurrentDefinition: []byte("values:\n  pulumiConfig:\n    test:source: old\n"),
			}
			if opts.DryRun {
				if h.dryRunOpens {
					res.OpenSessionID = "open-dry"
					res.Opened = &esc.Environment{Properties: published}
				}
				return res, nil
			}
			h.published = definition
			res.Revision = 4
			res.OpenSessionID = "open-1"
			res.Opened = &esc.Environment{Properties: published}
			return res, nil
		},
	}
	h.stack = &backend.MockStack{
		OrgNameF: func() string { return "test-org" },
		BackendF: func() backend.Backend { return be },
	}
	require.NoError(t, yaml.Unmarshal(
		[]byte("environment:\n  values:\n    pulumiConfig:\n      test:source: new\n"), &h.projectStack))
	return h
}

func TestAttachStackEnvironment(t *testing.T) {
	t.Parallel()

	project := workspace.Project{Name: "project"}
	sm := b64.NewBase64SecretsManager()

	t.Run("open only leaves the environment alone", func(t *testing.T) {
		t.Parallel()
		h := newStackEnvironmentTestHarness(t, true)

		cfg, err := getStackConfigurationFromProjectStack(
			t.Context(), h.stack, &project, sm, &h.projectStack, nil, StackConfigurationOptions{})
		require.NoError(t, err)
		assert.Nil(t, cfg.SyncEnvironment)
		// The open itself goes through a dry run so secrets encrypted for the environment resolve;
		// nothing is published and no diff is printed.
		assert.Equal(t, []backend.StackEnvironmentSyncOptions{{DryRun: true, Duration: stackEnvironmentOpenDuration}},
			h.syncCalls)
		assert.Nil(t, h.published)
	})

	t.Run("preview prints the pending change and does not publish", func(t *testing.T) {
		t.Parallel()
		h := newStackEnvironmentTestHarness(t, true)
		var out bytes.Buffer

		cfg, err := getStackConfigurationFromProjectStack(
			t.Context(), h.stack, &project, sm, &h.projectStack, nil,
			StackConfigurationOptions{EnvironmentMode: StackEnvironmentPreview, Stdout: &out})
		require.NoError(t, err)
		assert.Nil(t, cfg.SyncEnvironment)
		assert.Equal(t, []backend.StackEnvironmentSyncOptions{{DryRun: true, Duration: stackEnvironmentOpenDuration}},
			h.syncCalls)
		assert.Contains(t, out.String(), "Environment project/stack will be updated (revision 3):")
		assert.Contains(t, out.String(), "~ values.pulumiConfig.test:source: old -> new")
		assert.Equal(t, "anonymous", cfg.Environment.Value.(map[string]esc.Value)["test:source"].Value)
	})

	t.Run("sync publishes the definition once invoked and re-reads configuration", func(t *testing.T) {
		t.Parallel()
		h := newStackEnvironmentTestHarness(t, true)
		var out bytes.Buffer

		cfg, err := getStackConfigurationFromProjectStack(
			t.Context(), h.stack, &project, sm, &h.projectStack, nil,
			StackConfigurationOptions{EnvironmentMode: StackEnvironmentSync, Stdout: &out})
		require.NoError(t, err)
		require.NotNil(t, cfg.SyncEnvironment)
		assert.Nil(t, h.published, "nothing is published until the operation is confirmed")

		synced, err := cfg.SyncEnvironment(t.Context())
		require.NoError(t, err)
		require.Len(t, h.syncCalls, 2)
		require.NotNil(t, h.syncCalls[1].ExpectedRevision)
		assert.Equal(t, 3, *h.syncCalls[1].ExpectedRevision)
		assert.Equal(t, h.projectStack.EnvironmentBytes(), h.published)
		assert.Equal(t, &backend.StackEnvironmentRef{Name: "project/stack", Revision: 4, OpenSessionID: "open-1"},
			synced.StackEnvironment)
		assert.Equal(t, "published", synced.Environment.Value.(map[string]esc.Value)["test:source"].Value)
		assert.Contains(t, out.String(), "Published the stack's definition to environment project/stack (revision 3 -> 4)")
	})

	t.Run("an existing environment is opened through the dry run", func(t *testing.T) {
		t.Parallel()
		h := newStackEnvironmentTestHarness(t, true)
		h.dryRunOpens = true
		var out bytes.Buffer

		cfg, err := getStackConfigurationFromProjectStack(
			t.Context(), h.stack, &project, sm, &h.projectStack, nil,
			StackConfigurationOptions{EnvironmentMode: StackEnvironmentPreview, Stdout: &out})
		require.NoError(t, err)
		assert.Equal(t, []backend.StackEnvironmentSyncOptions{{DryRun: true, Duration: stackEnvironmentOpenDuration}},
			h.syncCalls, "one dry run serves both the open and the diff")
		assert.Equal(t, "published", cfg.Environment.Value.(map[string]esc.Value)["test:source"].Value)
		assert.Contains(t, out.String(), "~ values.pulumiConfig.test:source: old -> new")

		// Commands that only read configuration take the same path.
		env, diags, err := openStackEnv(t.Context(), h.stack, &h.projectStack, nil)
		require.NoError(t, err)
		assert.Empty(t, diags)
		assert.Equal(t, "published", env.Properties["pulumiConfig"].Value.(map[string]esc.Value)["test:source"].Value)
	})

	t.Run("overrides keep the anonymous open and skip publishing", func(t *testing.T) {
		t.Parallel()
		h := newStackEnvironmentTestHarness(t, true)
		var out bytes.Buffer

		cfg, err := getStackConfigurationFromProjectStack(
			t.Context(), h.stack, &project, sm, &h.projectStack, []string{"a=b"},
			StackConfigurationOptions{EnvironmentMode: StackEnvironmentSync, Stdout: &out})
		require.NoError(t, err)
		assert.Nil(t, cfg.SyncEnvironment)
		assert.Contains(t, out.String(), "--override-env is set, so environment project/stack is not synchronized")
	})

	t.Run("an older service falls back to the anonymous open", func(t *testing.T) {
		t.Parallel()
		h := newStackEnvironmentTestHarness(t, false)

		cfg, err := getStackConfigurationFromProjectStack(
			t.Context(), h.stack, &project, sm, &h.projectStack, nil,
			StackConfigurationOptions{EnvironmentMode: StackEnvironmentSync})
		require.NoError(t, err)
		assert.Nil(t, cfg.SyncEnvironment)
	})

	t.Run("an import list is never synchronized", func(t *testing.T) {
		t.Parallel()
		h := newStackEnvironmentTestHarness(t, true)
		var imports workspace.ProjectStack
		require.NoError(t, yaml.Unmarshal([]byte("environment:\n  - shared\n"), &imports))

		cfg, err := getStackConfigurationFromProjectStack(
			t.Context(), h.stack, &project, sm, &imports, nil,
			StackConfigurationOptions{EnvironmentMode: StackEnvironmentSync})
		require.NoError(t, err)
		assert.Nil(t, cfg.SyncEnvironment)
		assert.Empty(t, h.syncCalls)
	})
}
