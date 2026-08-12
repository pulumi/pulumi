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

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturedCall records the arguments a single ReadResource call received,
// so tests can assert that flags propagate correctly to the cloud client.
type capturedCall struct {
	org               string
	account           string
	resourceTypeAndID string
}

// mockInsightsClient stubs insightsResourceClient. Each instance returns a
// fixed resource (or error) and records the most recent invocation.
type mockInsightsClient struct {
	resource apitype.InsightsResourceWithVersion
	err      error
	captured *capturedCall
}

func (m *mockInsightsClient) GetInsightsResource(
	_ context.Context, org, account, resourceTypeAndID string,
) (apitype.InsightsResourceWithVersion, error) {
	if m.captured != nil {
		*m.captured = capturedCall{
			org:               org,
			account:           account,
			resourceTypeAndID: resourceTypeAndID,
		}
	}
	if m.err != nil {
		return apitype.InsightsResourceWithVersion{}, m.err
	}
	return m.resource, nil
}

// stubFactory returns a clientFactory that always yields client and effectiveOrg.
// If orgOverride is non-empty, the override wins — matching production behaviour
// so per-call --org assertions still work in the cobra-level test.
func stubFactory(client insightsResourceClient, defaultOrg string) clientFactory {
	return func(_ context.Context, orgOverride string) (insightsResourceClient, string, error) {
		org := orgOverride
		if org == "" {
			org = defaultOrg
		}
		return client, org, nil
	}
}

// failingFactory returns a clientFactory that always errors. Useful to cover
// the not-logged-in / missing-org branches.
func failingFactory(err error) clientFactory {
	return func(_ context.Context, _ string) (insightsResourceClient, string, error) {
		return nil, "", err
	}
}

func sampleResource() apitype.InsightsResourceWithVersion {
	return apitype.InsightsResourceWithVersion{
		Account:  "prod-aws",
		Type:     "aws:s3/bucket:Bucket",
		ID:       "my-bucket",
		Version:  7,
		Modified: time.Date(2026, 5, 1, 14, 30, 0, 0, time.UTC),
		State:    json.RawMessage(`{"arn":"arn:aws:s3:::my-bucket","region":"us-east-1"}`),
	}
}

func defaultGetArgs() insightsResourceGetArgs {
	return insightsResourceGetArgs{account: "prod-aws", renderOutput: renderResourceText}
}

func jsonGetArgs() insightsResourceGetArgs {
	a := defaultGetArgs()
	a.renderOutput = renderResourceJSON
	return a
}

func TestInsightsResourceGetCmd_DefaultOutput(t *testing.T) {
	t.Parallel()

	client := &mockInsightsClient{resource: sampleResource()}
	c := &insightsResourceGetCmd{clientFactory: stubFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "aws:s3/bucket:Bucket::my-bucket", defaultGetArgs())
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Account:      prod-aws")
	assert.Contains(t, output, "Type:         aws:s3/bucket:Bucket")
	assert.Contains(t, output, "ID:           my-bucket")
	assert.Contains(t, output, "Version:      7")
	assert.Contains(t, output, "Modified:     2026-05-01T14:30:00Z")
	// Policy state is empty in the sample → must not be rendered, to avoid
	// suggesting "no policy state" is itself a policy outcome.
	assert.NotContains(t, output, "Policy state")
	// JSON body indented under "State:".
	assert.Contains(t, output, "State:")
	assert.Contains(t, output, `"arn": "arn:aws:s3:::my-bucket"`)
}

func TestInsightsResourceGetCmd_DefaultOutput_WithPolicyState(t *testing.T) {
	t.Parallel()

	resource := sampleResource()
	resource.PolicyState = "violation"
	client := &mockInsightsClient{resource: resource}
	c := &insightsResourceGetCmd{clientFactory: stubFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "aws:s3/bucket:Bucket::my-bucket", defaultGetArgs())
	require.NoError(t, err)

	assert.Contains(t, out.String(), "Policy state: violation")
}

func TestInsightsResourceGetCmd_DefaultOutput_NoState(t *testing.T) {
	t.Parallel()

	resource := sampleResource()
	resource.State = nil
	client := &mockInsightsClient{resource: resource}
	c := &insightsResourceGetCmd{clientFactory: stubFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "aws:s3/bucket:Bucket::my-bucket", defaultGetArgs())
	require.NoError(t, err)

	// Header fields are present, but no "State:" section when state is empty.
	output := out.String()
	assert.Contains(t, output, "Account:      prod-aws")
	assert.NotContains(t, output, "State:")
}

// assertResourcesEqual checks two InsightsResourceWithVersion values for
// semantic equality, comparing the `State` JSON field via JSONEq so the test
// is insensitive to how the encoder reformats the nested document.
func assertResourcesEqual(t *testing.T, want, got apitype.InsightsResourceWithVersion) {
	t.Helper()
	wantState, gotState := want.State, got.State
	want.State, got.State = nil, nil
	assert.Equal(t, want, got)
	if wantState == nil && gotState == nil {
		return
	}
	require.NotNil(t, wantState, "want.State was nil but got.State was set")
	require.NotNil(t, gotState, "got.State was nil but want.State was set")
	assert.JSONEq(t, string(wantState), string(gotState))
}

func TestInsightsResourceGetCmd_JSONOutput(t *testing.T) {
	t.Parallel()

	client := &mockInsightsClient{resource: sampleResource()}
	c := &insightsResourceGetCmd{clientFactory: stubFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "aws:s3/bucket:Bucket::my-bucket", jsonGetArgs())
	require.NoError(t, err)

	var got apitype.InsightsResourceWithVersion
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assertResourcesEqual(t, sampleResource(), got)
}

func TestInsightsResourceGetCmd_OrgOverride(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        insightsResourceGetArgs
		defaultOrg  string
		wantOrg     string
		wantAccount string
		wantTypeID  string
	}{
		{
			name:        "default org used when --org omitted",
			args:        insightsResourceGetArgs{account: "prod-aws"},
			defaultOrg:  "acme",
			wantOrg:     "acme",
			wantAccount: "prod-aws",
			wantTypeID:  "aws:s3/bucket:Bucket::my-bucket",
		},
		{
			name:        "--org overrides default",
			args:        insightsResourceGetArgs{account: "prod-aws", org: "other-co"},
			defaultOrg:  "acme",
			wantOrg:     "other-co",
			wantAccount: "prod-aws",
			wantTypeID:  "aws:s3/bucket:Bucket::my-bucket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var captured capturedCall
			client := &mockInsightsClient{
				resource: sampleResource(),
				captured: &captured,
			}
			c := &insightsResourceGetCmd{clientFactory: stubFactory(client, tt.defaultOrg)}

			args := tt.args
			args.renderOutput = renderResourceText
			var out bytes.Buffer
			err := c.Run(t.Context(), &out, tt.wantTypeID, args)
			require.NoError(t, err)
			assert.Equal(t, capturedCall{
				org:               tt.wantOrg,
				account:           tt.wantAccount,
				resourceTypeAndID: tt.wantTypeID,
			}, captured)
		})
	}
}

func TestInsightsResourceGetCmd_MissingAccount(t *testing.T) {
	t.Parallel()

	// Tests bypass cobra's MarkFlagRequired, so Run must defend itself.
	client := &mockInsightsClient{resource: sampleResource()}
	c := &insightsResourceGetCmd{clientFactory: stubFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "aws:s3/bucket:Bucket::my-bucket", insightsResourceGetArgs{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--account is required")
}

func TestInsightsResourceGetCmd_ClientError(t *testing.T) {
	t.Parallel()

	client := &mockInsightsClient{err: errors.New("404 not found")}
	c := &insightsResourceGetCmd{clientFactory: stubFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "aws:s3/bucket:Bucket::missing", defaultGetArgs())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading insights resource")
	assert.Contains(t, err.Error(), "404 not found")
}

func TestInsightsResourceGetCmd_FactoryError(t *testing.T) {
	t.Parallel()

	c := &insightsResourceGetCmd{clientFactory: failingFactory(errors.New("not logged in"))}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "aws:s3/bucket:Bucket::my-bucket", defaultGetArgs())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestNewInsightsResourceGetCmd_FlagBinding(t *testing.T) {
	t.Parallel()

	var captured capturedCall
	client := &mockInsightsClient{
		resource: sampleResource(),
		captured: &captured,
	}
	cmd := newInsightsResourceGetCmd(stubFactory(client, "acme"))
	assert.Equal(t, "get", cmd.Name())

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--org", "other-co",
		"--account", "prod-aws",
		"--output", "json",
		"aws:s3/bucket:Bucket::my-bucket",
	})
	require.NoError(t, cmd.ExecuteContext(t.Context()))

	assert.Equal(t, capturedCall{
		org:               "other-co",
		account:           "prod-aws",
		resourceTypeAndID: "aws:s3/bucket:Bucket::my-bucket",
	}, captured)

	var got apitype.InsightsResourceWithVersion
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assertResourcesEqual(t, sampleResource(), got)
}

func TestNewInsightsResourceGetCmd_RequiredAccount(t *testing.T) {
	t.Parallel()

	cmd := newInsightsResourceGetCmd(stubFactory(&mockInsightsClient{}, "acme"))
	cmd.SetArgs([]string{"aws:s3/bucket:Bucket::my-bucket"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	// Cobra surfaces "required flag(s) ... not set" before RunE fires.
	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account")
}

func TestNewInsightsResourceGetCmd_NilFactoryUsesDefault(t *testing.T) {
	t.Parallel()

	// Passing nil installs the production factory; we only check the command is
	// well-formed without invoking it, since the default factory would hit the
	// real cloud context.
	cmd := newInsightsResourceGetCmd(nil)
	require.NotNil(t, cmd)
	assert.Equal(t, "get", cmd.Name())
}
