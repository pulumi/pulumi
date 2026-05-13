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

package env

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

type capturedReferrerCall struct {
	org, project, env string
	opts              client.ListEnvironmentReferrersOptions
}

type mockReferrerClient struct {
	resp     apitype.ListEnvironmentReferrersResponse
	err      error
	captured *capturedReferrerCall
}

func (m *mockReferrerClient) ListEnvironmentReferrers(
	_ context.Context, org, project, env string, opts client.ListEnvironmentReferrersOptions,
) (apitype.ListEnvironmentReferrersResponse, error) {
	if m.captured != nil {
		*m.captured = capturedReferrerCall{org: org, project: project, env: env, opts: opts}
	}
	return m.resp, m.err
}

func stubReferrerFactory(c envReferrerListClient, defaultOrg string) envReferrerListFactory {
	return func(_ context.Context, orgOverride string) (envReferrerListClient, string, error) {
		org := orgOverride
		if org == "" {
			org = defaultOrg
		}
		return c, org, nil
	}
}

func sampleReferrers() apitype.ListEnvironmentReferrersResponse {
	return apitype.ListEnvironmentReferrersResponse{
		Referrers: map[string][]apitype.EnvironmentReferrer{
			"latest": {
				{Environment: &apitype.EnvironmentImportReferrer{
					Project: "p2", Name: "shared", Revision: 2,
				}},
				{Stack: &apitype.EnvironmentStackReferrer{
					Project: "p", Stack: "dev", Version: 4,
				}},
				{InsightsAccount: &apitype.EnvironmentInsightsAccountReferrer{
					AccountName: "scanner",
				}},
			},
			"3": {
				{Stack: &apitype.EnvironmentStackReferrer{
					Project: "p", Stack: "dev", Version: 2,
				}},
			},
		},
		ContinuationToken: "token-2",
	}
}

func TestEnvReferrerList_DefaultOutput(t *testing.T) {
	t.Parallel()
	c := &mockReferrerClient{resp: sampleReferrers()}

	var out bytes.Buffer
	require.NoError(t, runEnvReferrerList(t.Context(), &out, stubReferrerFactory(c, "acme"),
		"my-project", "my-env", envReferrerListArgs{}))

	text := out.String()
	assert.Contains(t, text, "Revision: latest")
	assert.Contains(t, text, "Revision: 3")
	assert.Contains(t, text, "environment  p2/shared  rev=2")
	assert.Contains(t, text, "stack        p/dev  ver=4")
	assert.Contains(t, text, "insights     scanner")
	assert.Contains(t, text, "Next page: --continuation-token=token-2")
	// "latest" should be ordered above "3".
	assert.Less(t, bytes.Index(out.Bytes(), []byte("Revision: latest")),
		bytes.Index(out.Bytes(), []byte("Revision: 3")))
}

func TestEnvReferrerList_JSON(t *testing.T) {
	t.Parallel()
	c := &mockReferrerClient{resp: sampleReferrers()}

	var out bytes.Buffer
	require.NoError(t, runEnvReferrerList(t.Context(), &out, stubReferrerFactory(c, "acme"),
		"my-project", "my-env", envReferrerListArgs{output: "json"}))

	var got apitype.ListEnvironmentReferrersResponse
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Equal(t, sampleReferrers(), got)
}

func TestEnvReferrerList_Empty(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		resp apitype.ListEnvironmentReferrersResponse
	}{
		{"nil map", apitype.ListEnvironmentReferrersResponse{}},
		{"empty groups", apitype.ListEnvironmentReferrersResponse{
			Referrers: map[string][]apitype.EnvironmentReferrer{"latest": {}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := &mockReferrerClient{resp: tc.resp}
			var out bytes.Buffer
			require.NoError(t, runEnvReferrerList(t.Context(), &out,
				stubReferrerFactory(c, "acme"), "my-project", "my-env", envReferrerListArgs{}))
			assert.Contains(t, out.String(), "No referrers found")
		})
	}
}

func TestEnvReferrerList_InvalidOutput(t *testing.T) {
	t.Parallel()
	c := &mockReferrerClient{resp: sampleReferrers()}

	var out bytes.Buffer
	err := runEnvReferrerList(t.Context(), &out, stubReferrerFactory(c, "acme"),
		"p", "e", envReferrerListArgs{output: "yaml"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid --output value "yaml"`)
}

func TestEnvReferrerList_ClientError(t *testing.T) {
	t.Parallel()
	c := &mockReferrerClient{err: errors.New("403 forbidden")}

	var out bytes.Buffer
	err := runEnvReferrerList(t.Context(), &out, stubReferrerFactory(c, "acme"),
		"p", "e", envReferrerListArgs{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing referrers for acme/p/e")
}

func TestNewEnvReferrerListCmd_FlagBinding(t *testing.T) {
	t.Parallel()

	var captured capturedReferrerCall
	c := &mockReferrerClient{resp: sampleReferrers(), captured: &captured}
	cmd := newEnvReferrerListCmdWith(stubReferrerFactory(c, "acme"))

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--org", "other-co", "--count", "100", "--all-revisions",
		"--latest-stack-version-only", "--continuation-token", "tok",
		"--output", "json", "my-project", "my-env",
	})
	require.NoError(t, cmd.ExecuteContext(t.Context()))

	assert.Equal(t, capturedReferrerCall{
		org: "other-co", project: "my-project", env: "my-env",
		opts: client.ListEnvironmentReferrersOptions{
			Count: 100, AllRevisions: true,
			LatestStackVersionOnly: true, ContinuationToken: "tok",
		},
	}, captured)
}

func TestNewEnvReferrerListCmd_DefaultOrg(t *testing.T) {
	t.Parallel()

	var captured capturedReferrerCall
	c := &mockReferrerClient{resp: sampleReferrers(), captured: &captured}
	cmd := newEnvReferrerListCmdWith(stubReferrerFactory(c, "default-org"))

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"proj", "envname"})
	require.NoError(t, cmd.ExecuteContext(t.Context()))

	assert.Equal(t, "default-org", captured.org)
	assert.Equal(t, "proj", captured.project)
	assert.Equal(t, "envname", captured.env)
}
