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

package insights

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturedAccountCall records the arguments a single CreateInsightsAccount
// call received, so tests can assert that flags propagate correctly to the
// cloud client.
type capturedAccountCall struct {
	org     string
	account string
	req     apitype.CreateInsightsAccountRequest
}

// mockInsightsAccountNewClient stubs insightsAccountNewClient. Each instance
// records the most recent Create invocation and replays a fixed GetAccount
// response (or an error on either method) so tests can drive each branch.
type mockInsightsAccountNewClient struct {
	getAccount apitype.InsightsAccount
	createErr  error
	getErr     error
	captured   *capturedAccountCall
}

func (m *mockInsightsAccountNewClient) CreateInsightsAccount(
	_ context.Context, org, account string, req apitype.CreateInsightsAccountRequest,
) error {
	if m.captured != nil {
		*m.captured = capturedAccountCall{org: org, account: account, req: req}
	}
	return m.createErr
}

func (m *mockInsightsAccountNewClient) GetInsightsAccount(
	_ context.Context, _, _ string,
) (apitype.InsightsAccount, error) {
	if m.getErr != nil {
		return apitype.InsightsAccount{}, m.getErr
	}
	return m.getAccount, nil
}

// stubAccountNewFactory returns an accountNewClientFactory that always yields
// client and effectiveOrg. If orgOverride is non-empty, the override wins —
// matching production behaviour so per-call --org assertions still work in
// the cobra-level test.
func stubAccountNewFactory(client insightsAccountNewClient, defaultOrg string) accountNewClientFactory {
	return func(_ context.Context, orgOverride string) (insightsAccountNewClient, string, error) {
		org := orgOverride
		if org == "" {
			org = defaultOrg
		}
		return client, org, nil
	}
}

// failingAccountNewFactory returns a factory that always errors. Useful to
// cover the not-logged-in / missing-org branches.
func failingAccountNewFactory(err error) accountNewClientFactory {
	return func(_ context.Context, _ string) (insightsAccountNewClient, string, error) {
		return nil, "", err
	}
}

// sampleNewAccount is a representative successful read-back. Tests that don't
// care about the renderer reuse it to keep test setup compact.
func sampleNewAccount() apitype.InsightsAccount {
	finished := time.Date(2026, 5, 13, 10, 0, 0, 0, time.UTC)
	return apitype.InsightsAccount{
		ID:                   "acc-123",
		Name:                 "prod-aws",
		Provider:             "aws",
		ProviderVersion:      "6.0.0",
		ProviderEnvRef:       "infra/prod-aws-creds",
		ScheduledScanEnabled: true,
		AgentPoolID:          "pool-1",
		ProviderConfig:       json.RawMessage(`{"regions":["us-east-1"]}`),
		OwnedBy: apitype.InsightsAccountOwner{
			Name:        "Jane Doe",
			GitHubLogin: "jdoe",
			AvatarURL:   "https://example.com/avatar.png",
		},
		ScanStatus: &apitype.InsightsAccountScanStatus{
			ID:            "scan-1",
			OrgID:         "org-1",
			UserID:        "user-1",
			Status:        "succeeded",
			ResourceCount: 42,
			FinishedAt:    &finished,
		},
	}
}

func TestRunInsightsAccountNew_DefaultOutput(t *testing.T) {
	t.Parallel()

	client := &mockInsightsAccountNewClient{getAccount: sampleNewAccount()}
	var out bytes.Buffer
	err := runInsightsAccountNew(
		t.Context(), &out, stubAccountNewFactory(client, "acme"), "prod-aws",
		accountNewArgs{provider: "aws", environment: "infra/prod-aws-creds"},
		renderInsightsAccountText,
	)
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "ID:")
	assert.Contains(t, output, "acc-123")
	assert.Contains(t, output, "Owner:")
	assert.Contains(t, output, "Jane Doe (jdoe)")
	assert.Contains(t, output, "Provider:")
	assert.Contains(t, output, "aws (v6.0.0)")
	assert.Contains(t, output, "Environment:")
	assert.Contains(t, output, "infra/prod-aws-creds")
	assert.Contains(t, output, "Scheduled scan:")
	assert.Contains(t, output, "Scan status:")
	assert.Contains(t, output, "succeeded")
	assert.Contains(t, output, "Resources discovered:")
	assert.Contains(t, output, "42")
}

func TestRunInsightsAccountNew_JSONOutput(t *testing.T) {
	t.Parallel()

	client := &mockInsightsAccountNewClient{getAccount: sampleNewAccount()}
	var out bytes.Buffer
	err := runInsightsAccountNew(
		t.Context(), &out, stubAccountNewFactory(client, "acme"), "prod-aws",
		accountNewArgs{provider: "aws", environment: "infra/prod-aws-creds"},
		renderInsightsAccountJSON,
	)
	require.NoError(t, err)

	var got apitype.InsightsAccount
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	// ProviderConfig is `json.RawMessage`, so the encoder may re-emit the
	// bytes with normalized whitespace. Compare it semantically and the
	// rest of the struct field-by-field.
	want := sampleNewAccount()
	assert.JSONEq(t, string(want.ProviderConfig), string(got.ProviderConfig))
	want.ProviderConfig, got.ProviderConfig = nil, nil
	assert.Equal(t, want, got)
}

func TestRunInsightsAccountNew_RequestPropagation(t *testing.T) {
	t.Parallel()

	var captured capturedAccountCall
	client := &mockInsightsAccountNewClient{
		getAccount: sampleNewAccount(),
		captured:   &captured,
	}
	err := runInsightsAccountNew(
		t.Context(), &bytes.Buffer{}, stubAccountNewFactory(client, "acme"), "prod-aws",
		accountNewArgs{
			provider:       "gcp",
			environment:    "infra/gcp-creds@2",
			scanSchedule:   "12h",
			agentPoolID:    "pool-xyz",
			providerConfig: `{"project":"my-proj"}`,
		},
		renderInsightsAccountText,
	)
	require.NoError(t, err)

	assert.Equal(t, capturedAccountCall{
		org:     "acme",
		account: "prod-aws",
		req: apitype.CreateInsightsAccountRequest{
			Provider:       "gcp",
			Environment:    "infra/gcp-creds@2",
			ScanSchedule:   "12h",
			AgentPoolID:    "pool-xyz",
			ProviderConfig: json.RawMessage(`{"project":"my-proj"}`),
		},
	}, captured)
}

func TestRunInsightsAccountNew_OrgOverride(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       accountNewArgs
		defaultOrg string
		wantOrg    string
	}{
		{
			name:       "default org used when --org omitted",
			args:       accountNewArgs{provider: "aws", environment: "infra/creds"},
			defaultOrg: "acme",
			wantOrg:    "acme",
		},
		{
			name: "--org overrides default",
			args: accountNewArgs{
				org: "other-co", provider: "aws", environment: "infra/creds",
			},
			defaultOrg: "acme",
			wantOrg:    "other-co",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var captured capturedAccountCall
			client := &mockInsightsAccountNewClient{
				getAccount: sampleNewAccount(),
				captured:   &captured,
			}
			err := runInsightsAccountNew(
				t.Context(), &bytes.Buffer{}, stubAccountNewFactory(client, tt.defaultOrg),
				"prod-aws", tt.args, renderInsightsAccountText,
			)
			require.NoError(t, err)
			assert.Equal(t, tt.wantOrg, captured.org)
			assert.Equal(t, "prod-aws", captured.account)
		})
	}
}

func TestRunInsightsAccountNew_MissingRequired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args accountNewArgs
		want string
	}{
		{
			name: "missing provider",
			args: accountNewArgs{environment: "infra/creds"},
			want: "--provider is required",
		},
		{
			name: "missing environment",
			args: accountNewArgs{provider: "aws"},
			want: "--environment is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := &mockInsightsAccountNewClient{getAccount: sampleNewAccount()}
			err := runInsightsAccountNew(
				t.Context(), &bytes.Buffer{}, stubAccountNewFactory(client, "acme"),
				"prod-aws", tt.args, renderInsightsAccountText,
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestRunInsightsAccountNew_InvalidProviderConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{name: "garbage", raw: "not json"},
		// JSON scalars and arrays are not objects — the server contract is
		// "object optional," so we reject early with a clear message.
		{name: "scalar", raw: `"a string"`},
		{name: "array", raw: `["a","b"]`},
		{name: "number", raw: `42`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := &mockInsightsAccountNewClient{getAccount: sampleNewAccount()}
			err := runInsightsAccountNew(
				t.Context(), &bytes.Buffer{}, stubAccountNewFactory(client, "acme"),
				"prod-aws",
				accountNewArgs{provider: "aws", environment: "infra/creds", providerConfig: tt.raw},
				renderInsightsAccountText,
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid --provider-config")
		})
	}
}

func TestRunInsightsAccountNew_CreateError(t *testing.T) {
	t.Parallel()

	client := &mockInsightsAccountNewClient{createErr: errors.New("404 environment not found")}
	err := runInsightsAccountNew(
		t.Context(), &bytes.Buffer{}, stubAccountNewFactory(client, "acme"), "prod-aws",
		accountNewArgs{provider: "aws", environment: "infra/missing"},
		renderInsightsAccountText,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating insights account")
	assert.Contains(t, err.Error(), "404 environment not found")
}

func TestRunInsightsAccountNew_ReadBackError(t *testing.T) {
	t.Parallel()

	// Create succeeds but the follow-up GET fails. The error message must
	// make clear that the account was created so the user knows to use
	// `list` to verify.
	client := &mockInsightsAccountNewClient{getErr: errors.New("503 service unavailable")}
	err := runInsightsAccountNew(
		t.Context(), &bytes.Buffer{}, stubAccountNewFactory(client, "acme"), "prod-aws",
		accountNewArgs{provider: "aws", environment: "infra/creds"},
		renderInsightsAccountText,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `account "prod-aws" was created`)
	assert.Contains(t, err.Error(), `organization "acme"`)
	assert.Contains(t, err.Error(), "503 service unavailable")
	assert.Contains(t, err.Error(), "pulumi insights account list")
}

func TestRunInsightsAccountNew_FactoryError(t *testing.T) {
	t.Parallel()

	err := runInsightsAccountNew(
		t.Context(), &bytes.Buffer{},
		failingAccountNewFactory(errors.New("not logged in")), "prod-aws",
		accountNewArgs{provider: "aws", environment: "infra/creds"},
		renderInsightsAccountText,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestRenderInsightsAccountText_OmitsAbsentFields(t *testing.T) {
	t.Parallel()

	account := apitype.InsightsAccount{
		ID:       "acc-123",
		Name:     "prod-aws",
		Provider: "aws",
		// No ProviderVersion, ProviderEnvRef, ProviderConfig, ScanStatus.
		OwnedBy: apitype.InsightsAccountOwner{Name: "Jane Doe"},
	}
	var out bytes.Buffer
	require.NoError(t, renderInsightsAccountText(&out, account))

	output := out.String()
	// Required fields are always present.
	assert.Contains(t, output, "Provider:")
	assert.Contains(t, output, "aws")
	// Optional sections are suppressed; matching the bare label avoids
	// accidentally tripping on `Provider:` (the required field) when
	// looking for `Provider config:`.
	assert.NotContains(t, output, "Environment:")
	assert.NotContains(t, output, "Provider config:")
	assert.NotContains(t, output, "Scan status:")
	// Provider without a version doesn't get the "(v...)" suffix.
	assert.NotContains(t, output, "(v")
	// Agent pool falls back to "(default)" when unset rather than blank.
	assert.Contains(t, output, "(default)")
}

func TestResolveAccountNewArgs_SkipPromptsRequiresFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args accountNewArgs
		want string
	}{
		{
			name: "missing provider",
			args: accountNewArgs{environment: "infra/creds"},
			want: "--provider is required",
		},
		{
			name: "missing environment",
			args: accountNewArgs{provider: "aws"},
			want: "--environment is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := resolveAccountNewArgs(true, tt.args, display.Options{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestResolveAccountNewArgs_PassesThroughCompleteArgs(t *testing.T) {
	t.Parallel()

	// When both required flags are already set, resolve is a no-op
	// regardless of skipPrompts. This covers the happy path the cobra
	// RunE follows when the user passed everything on the command line.
	args := accountNewArgs{provider: "aws", environment: "infra/creds"}
	got, err := resolveAccountNewArgs(true, args, display.Options{})
	require.NoError(t, err)
	assert.Equal(t, args, got)
}

func TestNewInsightsAccountNewCmd_FlagBinding(t *testing.T) {
	t.Parallel()

	var captured capturedAccountCall
	client := &mockInsightsAccountNewClient{
		getAccount: sampleNewAccount(),
		captured:   &captured,
	}
	cmd := newInsightsAccountNewCmdWith(stubAccountNewFactory(client, "acme"))
	assert.Equal(t, "new", cmd.Name())

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--org", "other-co",
		"--provider", "aws",
		"--environment", "infra/prod-aws-creds@3",
		"--scan-schedule", "daily",
		"--agent-pool-id", "pool-1",
		"--provider-config", `{"regions":["us-east-1"]}`,
		"--output", "json",
		// --yes ensures the test passes even when the harness happens to
		// have an interactive stdin (otherwise resolveAccountNewArgs would
		// try to prompt for confirmation of values already on the command
		// line, which would block).
		"--yes",
		"prod-aws",
	})
	require.NoError(t, cmd.ExecuteContext(t.Context()))

	assert.Equal(t, capturedAccountCall{
		org:     "other-co",
		account: "prod-aws",
		req: apitype.CreateInsightsAccountRequest{
			Provider:       "aws",
			Environment:    "infra/prod-aws-creds@3",
			ScanSchedule:   "daily",
			AgentPoolID:    "pool-1",
			ProviderConfig: json.RawMessage(`{"regions":["us-east-1"]}`),
		},
	}, captured)

	// Output is JSON, so the buffer parses cleanly back to InsightsAccount.
	var got apitype.InsightsAccount
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Equal(t, "prod-aws", got.Name)
}

func TestNewInsightsAccountNewCmd_YesSkipsPrompts(t *testing.T) {
	t.Parallel()

	// With --yes and a required flag missing, the command fails fast with
	// the matching "required" error rather than blocking on a prompt.
	cmd := newInsightsAccountNewCmdWith(
		stubAccountNewFactory(&mockInsightsAccountNewClient{}, "acme"),
	)
	cmd.SetArgs([]string{"--yes", "--provider", "aws", "prod-aws"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--environment is required")
}

func TestNewInsightsAccountNewCmd_NilFactoryUsesDefault(t *testing.T) {
	t.Parallel()

	// Passing nil installs the production factory; we only check the
	// command is well-formed without invoking it, since the default
	// factory would hit the real cloud context.
	cmd := newInsightsAccountNewCmd()
	require.NotNil(t, cmd)
	assert.Equal(t, "new", cmd.Name())
}
