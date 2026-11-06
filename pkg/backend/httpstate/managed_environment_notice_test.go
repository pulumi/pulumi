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
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	esc_client "github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	sdkDisplay "github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

var managedEnvironmentNotice = "Environment p/env has local changes based on revision 7. " +
	"up, refresh and destroy publish them; preview doesn't.\n" +
	"    " + colors.SpecCreate + `+ values.pulumiConfig["p:a"]: 1` + colors.Reset + "\n"

type publishedEnvironmentClient struct {
	esc_client.Client
	definition []byte
}

func (c publishedEnvironmentClient) GetEnvironment(
	context.Context, string, string, string, string, bool,
) ([]byte, string, int, error) {
	return c.definition, "etag", 7, nil
}

func (c publishedEnvironmentClient) OpenEnvironment(
	context.Context, string, string, string, string, time.Duration,
) (string, []esc_client.EnvironmentDiagnostic, error) {
	return "open-id", nil, nil
}

func (c publishedEnvironmentClient) GetOpenEnvironment(
	context.Context, string, string, string, string,
) (*esc.Environment, error) {
	return &esc.Environment{}, nil
}

func TestEvaluateManagedEnvironmentReturnsPublishedDefinition(t *testing.T) {
	t.Parallel()

	published := []byte("values:\n  a: 1\n")
	local := []byte("values:\n    a: 1\n")
	b := &cloudBackend{escClient: publishedEnvironmentClient{definition: published}}
	evaluation, diags, err := b.EvaluateManagedEnvironment(t.Context(), "org", "p", "env", local)
	require.NoError(t, err)
	assert.Empty(t, diags)
	assert.Equal(t, published, evaluation.PublishedDefinition)
}

func runDestroyPreview(t *testing.T, notice string, jsonDisplay bool) []engine.Event {
	t.Helper()

	provider := (&secrets.MockProvider{}).Add(b64.Type, func(json.RawMessage) (secrets.Manager, error) {
		return b64.NewBase64SecretsManager(), nil
	})
	fx := newRunEngineActionFixture(t, &deploy.Snapshot{SecretsManager: b64.NewBase64SecretsManager()},
		provider, b64.NewBase64SecretsManager())
	fx.op.Scopes = backend.CancellationScopes
	fx.op.Opts.Display.JSONDisplay = jsonDisplay
	fx.op.StackConfiguration.ManagedEnvironmentNotice = notice

	events := make(chan engine.Event)
	collected := make(chan []engine.Event)
	go func() {
		var all []engine.Event
		for e := range events {
			all = append(all, e)
		}
		collected <- all
	}()
	_, _, _, err := fx.backend.runEngineAction(
		t.Context(), apitype.DestroyUpdate, fx.stackRef, fx.op, fx.update,
		"lease-token", "", engine.NewEvent(engine.UpdateStartedEventPayload{UpdateID: "update-id"}),
		events, true /* dryRun */, 0)
	close(events)
	require.NoError(t, err)
	return <-collected
}

func TestRunEngineActionSendsManagedEnvironmentNotice(t *testing.T) {
	t.Parallel()

	events := runDestroyPreview(t, managedEnvironmentNotice, false)
	require.Greater(t, len(events), 2)
	assert.Equal(t, engine.UpdateStartedEvent, events[0].Type)
	assert.Equal(t, engine.StdoutColorEvent, events[1].Type)
	// Credentials stored by other tests in this process stay in the global filter.
	assert.Equal(t,
		engine.StdoutEventPayload{Message: logging.FilterString(managedEnvironmentNotice), Color: colors.Never},
		events[1].Payload())
	for _, e := range events[2:] {
		assert.NotEqual(t, engine.StdoutColorEvent, e.Type)
	}

	for _, e := range runDestroyPreview(t, "", false) {
		assert.NotEqual(t, engine.StdoutColorEvent, e.Type)
	}
}

func TestRunEngineActionFiltersResolvedSecretsFromManagedEnvironmentNotice(t *testing.T) {
	t.Parallel()

	const resolved = "managed-notice-resolved-secret"
	logging.AddGlobalSecretFilter([]string{resolved}, "[secret]")

	events := runDestroyPreview(t, "    + values.dbPassword: \""+resolved+"\"\n", false)
	require.Greater(t, len(events), 1)
	assert.Equal(t,
		engine.StdoutEventPayload{Message: "    + values.dbPassword: \"[secret]\"\n", Color: colors.Never},
		events[1].Payload())
}

//nolint:paralleltest // replaces os.Stdout, where the JSON preview digest is printed
func TestRunEngineActionManagedEnvironmentNoticeJSON(t *testing.T) {
	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	require.NoError(t, err)
	oldStdout := os.Stdout
	os.Stdout = stdout
	t.Cleanup(func() {
		os.Stdout = oldStdout
		require.NoError(t, stdout.Close())
	})

	runDestroyPreview(t, managedEnvironmentNotice, true)

	out, err := os.ReadFile(stdout.Name())
	require.NoError(t, err)
	var digest sdkDisplay.PreviewDigest
	require.NoError(t, json.Unmarshal(out, &digest))
	assert.Equal(t, []sdkDisplay.PreviewDiagnostic{{
		Message:  colors.Never.Colorize(logging.FilterString(managedEnvironmentNotice)),
		Severity: diag.Info,
	}}, digest.Diagnostics)
}
