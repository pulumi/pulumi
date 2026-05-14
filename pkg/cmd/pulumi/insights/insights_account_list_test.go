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

// capturedAccountListCall records the most recent call to ListInsightsAccounts,
// so tests can assert that flags propagate correctly to the cloud client.
type capturedAccountListCall struct {
	org    string
	params apitype.ListInsightsAccountsParams
}

// mockInsightsAccountListClient stubs insightsAccountListClient. Successive
// invocations return the next entry of `pages`, simulating server-side
// pagination via continuationToken/nextToken.
type mockInsightsAccountListClient struct {
	pages []apitype.ListInsightsAccountsResponse
	calls []capturedAccountListCall
	err   error
}

func (m *mockInsightsAccountListClient) ListInsightsAccounts(
	_ context.Context, org string, params apitype.ListInsightsAccountsParams,
) (apitype.ListInsightsAccountsResponse, error) {
	m.calls = append(m.calls, capturedAccountListCall{org: org, params: params})
	if m.err != nil {
		return apitype.ListInsightsAccountsResponse{}, m.err
	}
	if len(m.pages) == 0 {
		// Caller exhausted the canned pages; return an empty terminal page so
		// the loop in collectInsightsAccounts can finish cleanly.
		return apitype.ListInsightsAccountsResponse{}, nil
	}
	resp := m.pages[0]
	m.pages = m.pages[1:]
	return resp, nil
}

// stubAccountListFactory returns an accountListClientFactory that always yields
// client and effectiveOrg. If orgOverride is non-empty, the override wins —
// matching production behaviour so per-call --org assertions still work.
func stubAccountListFactory(
	client insightsAccountListClient, defaultOrg string,
) accountListClientFactory {
	return func(_ context.Context, orgOverride string) (insightsAccountListClient, string, error) {
		org := orgOverride
		if org == "" {
			org = defaultOrg
		}
		return client, org, nil
	}
}

// failingAccountListFactory returns an accountListClientFactory that always
// errors. Useful to cover the not-logged-in / missing-org branches.
func failingAccountListFactory(err error) accountListClientFactory {
	return func(_ context.Context, _ string) (insightsAccountListClient, string, error) {
		return nil, "", err
	}
}

// defaultListArgs returns a fresh insightsAccountListArgs with the OutputFlag
// pre-populated. Tests that drive Run directly need this because they bypass
// the cobra constructor that would otherwise install the renderer table.
func defaultListArgs() insightsAccountListArgs {
	return insightsAccountListArgs{output: defaultAccountListOutputFormat()}
}

// withOutput is a small helper for tests that need to flip the output format
// without rebuilding the whole args struct.
func withOutput(args insightsAccountListArgs, format string) insightsAccountListArgs {
	if err := args.output.Set(format); err != nil {
		panic(err)
	}
	return args
}

func sampleAccount(name string) apitype.InsightsAccount {
	finished := time.Date(2026, 5, 12, 16, 7, 24, 0, time.UTC)
	return apitype.InsightsAccount{
		ID:                   "id-" + name,
		Name:                 name,
		Provider:             "aws",
		ProviderEnvRef:       "team/" + name + "@1",
		ScheduledScanEnabled: true,
		OwnedBy: apitype.InsightsAccountOwner{
			Name:        "Ada Lovelace",
			GitHubLogin: "ada-pulumi-corp",
			AvatarURL:   "https://api.pulumi.com/static/avatars/A.png",
		},
		ScanStatus: &apitype.InsightsAccountScanStatus{
			ID:            "scan-" + name,
			OrgID:         "org-1",
			UserID:        "user-1",
			Status:        "succeeded",
			StartedAt:     time.Date(2026, 5, 12, 16, 6, 1, 0, time.UTC),
			FinishedAt:    &finished,
			LastUpdatedAt: time.Date(2026, 5, 12, 16, 7, 24, 0, time.UTC),
			ResourceCount: 42,
		},
	}
}

func TestInsightsAccountListCmd_DefaultOutput(t *testing.T) {
	t.Parallel()

	client := &mockInsightsAccountListClient{
		pages: []apitype.ListInsightsAccountsResponse{{
			Accounts: []apitype.InsightsAccount{sampleAccount("prod-aws"), sampleAccount("dev-aws")},
		}},
	}
	c := &insightsAccountListCmd{clientFactory: stubAccountListFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, defaultListArgs())
	require.NoError(t, err)

	output := out.String()
	// Headers and rows from the table renderer.
	assert.Contains(t, output, "Name")
	assert.Contains(t, output, "Provider")
	assert.Contains(t, output, "Owner")
	assert.Contains(t, output, "Scheduled Scan")
	assert.Contains(t, output, "Last Scan")
	assert.Contains(t, output, "Resources")
	assert.Contains(t, output, "prod-aws")
	assert.Contains(t, output, "dev-aws")
	assert.Contains(t, output, "ada-pulumi-corp")
	assert.Contains(t, output, "yes")                    // scheduled scan enabled
	assert.Contains(t, output, "succeeded (2026-05-12)") // status + finish date
	assert.Contains(t, output, "42")                     // resource count
}

func TestInsightsAccountListCmd_DefaultOutput_NoResults(t *testing.T) {
	t.Parallel()

	client := &mockInsightsAccountListClient{
		pages: []apitype.ListInsightsAccountsResponse{{Accounts: nil}},
	}
	c := &insightsAccountListCmd{clientFactory: stubAccountListFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, defaultListArgs())
	require.NoError(t, err)
	assert.Equal(t, "No accounts found.\n", out.String())
}

// TestInsightsAccountListCmd_DefaultOutput_NoScanStatus covers the row layout
// when an account has never been scanned — the scan-derived columns must render
// as `-` so the table stays aligned and the absence is obvious.
func TestInsightsAccountListCmd_DefaultOutput_NoScanStatus(t *testing.T) {
	t.Parallel()

	account := sampleAccount("fresh-aws")
	account.ScanStatus = nil
	account.ScheduledScanEnabled = false
	client := &mockInsightsAccountListClient{
		pages: []apitype.ListInsightsAccountsResponse{{Accounts: []apitype.InsightsAccount{account}}},
	}
	c := &insightsAccountListCmd{clientFactory: stubAccountListFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, defaultListArgs())
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "fresh-aws")
	assert.Contains(t, output, "no") // scheduled scan disabled
	// `-` filler appears for both "Last Scan" and "Resources" columns. We
	// don't assert the exact column position to avoid coupling to table layout.
	assert.Contains(t, output, " - ")
}

func TestInsightsAccountListCmd_JSONOutput(t *testing.T) {
	t.Parallel()

	want := []apitype.InsightsAccount{sampleAccount("prod-aws"), sampleAccount("dev-aws")}
	client := &mockInsightsAccountListClient{
		pages: []apitype.ListInsightsAccountsResponse{{Accounts: want}},
	}
	c := &insightsAccountListCmd{clientFactory: stubAccountListFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, withOutput(defaultListArgs(), "json"))
	require.NoError(t, err)

	var got struct {
		Accounts []apitype.InsightsAccount `json:"accounts"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Equal(t, want, got.Accounts)
}

// TestInsightsAccountListCmd_JSONOutput_EmptyList ensures an empty result set
// serialises to `[]` rather than `null`, so jq scripting can iterate without a
// nil-check.
func TestInsightsAccountListCmd_JSONOutput_EmptyList(t *testing.T) {
	t.Parallel()

	client := &mockInsightsAccountListClient{
		pages: []apitype.ListInsightsAccountsResponse{{Accounts: nil}},
	}
	c := &insightsAccountListCmd{clientFactory: stubAccountListFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, withOutput(defaultListArgs(), "json"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"accounts":[]}`, out.String())
}

// TestInsightsAccountListCmd_DefaultStopsAfterFirstPage: without --count or
// --all the command must return exactly what the first server-side page
// contains, even when the server signals more pages are available. This is the
// "Default is the size of the first page" rule from the epic.
func TestInsightsAccountListCmd_DefaultStopsAfterFirstPage(t *testing.T) {
	t.Parallel()

	client := &mockInsightsAccountListClient{
		pages: []apitype.ListInsightsAccountsResponse{
			{
				Accounts:  []apitype.InsightsAccount{sampleAccount("a1"), sampleAccount("a2")},
				NextToken: "cursor-1",
			},
			// A second page is canned but must not be consumed when --count
			// and --all are both unset.
			{
				Accounts:  []apitype.InsightsAccount{sampleAccount("a3")},
				NextToken: "",
			},
		},
	}
	c := &insightsAccountListCmd{clientFactory: stubAccountListFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, withOutput(defaultListArgs(), "json"))
	require.NoError(t, err)

	var got struct {
		Accounts []apitype.InsightsAccount `json:"accounts"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	require.Len(t, got.Accounts, 2)
	assert.Equal(t, "a1", got.Accounts[0].Name)
	assert.Equal(t, "a2", got.Accounts[1].Name)
	// Only the first page was requested.
	require.Len(t, client.calls, 1)
}

// TestInsightsAccountListCmd_AllFollowsPagination: --all keeps following the
// continuationToken cursor until the server reports an empty nextToken,
// collecting every account from every page.
func TestInsightsAccountListCmd_AllFollowsPagination(t *testing.T) {
	t.Parallel()

	client := &mockInsightsAccountListClient{
		pages: []apitype.ListInsightsAccountsResponse{
			{
				Accounts:  []apitype.InsightsAccount{sampleAccount("a1"), sampleAccount("a2")},
				NextToken: "cursor-1",
			},
			{
				Accounts:  []apitype.InsightsAccount{sampleAccount("a3")},
				NextToken: "cursor-2",
			},
			{
				Accounts:  []apitype.InsightsAccount{sampleAccount("a4")},
				NextToken: "",
			},
		},
	}
	c := &insightsAccountListCmd{clientFactory: stubAccountListFactory(client, "acme")}

	args := withOutput(defaultListArgs(), "json")
	args.all = true

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, args)
	require.NoError(t, err)

	var got struct {
		Accounts []apitype.InsightsAccount `json:"accounts"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	require.Len(t, got.Accounts, 4)
	assert.Equal(t, "a1", got.Accounts[0].Name)
	assert.Equal(t, "a4", got.Accounts[3].Name)

	// The continuationToken on each request must come from the previous
	// response's nextToken.
	require.Len(t, client.calls, 3)
	assert.Empty(t, client.calls[0].params.ContinuationToken)
	assert.Equal(t, "cursor-1", client.calls[1].params.ContinuationToken)
	assert.Equal(t, "cursor-2", client.calls[2].params.ContinuationToken)
}

// TestInsightsAccountListCmd_CountPaginatesUntilSatisfied: --count N pages
// across as many server pages as needed to gather N rows, and truncates the
// last page so the user sees exactly N.
func TestInsightsAccountListCmd_CountPaginatesUntilSatisfied(t *testing.T) {
	t.Parallel()

	client := &mockInsightsAccountListClient{
		pages: []apitype.ListInsightsAccountsResponse{
			{
				Accounts:  []apitype.InsightsAccount{sampleAccount("a1"), sampleAccount("a2")},
				NextToken: "cursor-1",
			},
			{
				Accounts:  []apitype.InsightsAccount{sampleAccount("a3"), sampleAccount("a4")},
				NextToken: "cursor-2",
			},
			// A third page is canned but should not be requested because
			// --count=3 is already satisfied at the end of page 2.
			{
				Accounts:  []apitype.InsightsAccount{sampleAccount("a5")},
				NextToken: "",
			},
		},
	}
	c := &insightsAccountListCmd{clientFactory: stubAccountListFactory(client, "acme")}

	args := withOutput(defaultListArgs(), "json")
	args.count = 3
	args.countSet = true

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, args)
	require.NoError(t, err)

	var got struct {
		Accounts []apitype.InsightsAccount `json:"accounts"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	require.Len(t, got.Accounts, 3, "result should be truncated to --count")
	assert.Equal(t, []string{"a1", "a2", "a3"},
		[]string{got.Accounts[0].Name, got.Accounts[1].Name, got.Accounts[2].Name})
	require.Len(t, client.calls, 2, "should have stopped paginating after --count satisfied")
}

// TestInsightsAccountListCmd_CountZeroReturnsEmpty: --count 0 is an explicit
// "I want nothing" — return an empty list immediately without contacting the
// server. This is the difference between "flag unset" (one page) and "flag set
// to zero" (no rows).
func TestInsightsAccountListCmd_CountZeroReturnsEmpty(t *testing.T) {
	t.Parallel()

	client := &mockInsightsAccountListClient{
		pages: []apitype.ListInsightsAccountsResponse{
			// Canned page that must not be requested.
			{Accounts: []apitype.InsightsAccount{sampleAccount("a1")}},
		},
	}
	c := &insightsAccountListCmd{clientFactory: stubAccountListFactory(client, "acme")}

	args := withOutput(defaultListArgs(), "json")
	args.count = 0
	args.countSet = true

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, args)
	require.NoError(t, err)
	assert.JSONEq(t, `{"accounts":[]}`, out.String())
	assert.Empty(t, client.calls, "--count 0 must not contact the server")
}

// TestInsightsAccountListCmd_CountLargerThanAvailable: when --count asks for
// more rows than the server has, the command returns every available row
// without erroring or padding.
func TestInsightsAccountListCmd_CountLargerThanAvailable(t *testing.T) {
	t.Parallel()

	client := &mockInsightsAccountListClient{
		pages: []apitype.ListInsightsAccountsResponse{
			{
				Accounts:  []apitype.InsightsAccount{sampleAccount("a1"), sampleAccount("a2")},
				NextToken: "",
			},
		},
	}
	c := &insightsAccountListCmd{clientFactory: stubAccountListFactory(client, "acme")}

	args := withOutput(defaultListArgs(), "json")
	args.count = 100
	args.countSet = true

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, args)
	require.NoError(t, err)

	var got struct {
		Accounts []apitype.InsightsAccount `json:"accounts"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	require.Len(t, got.Accounts, 2)
}

func TestInsightsAccountListCmd_FilterPropagation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		mutate     func(*insightsAccountListArgs)
		defaultOrg string
		wantOrg    string
		wantParent string
		wantRoleID string
	}{
		{
			name:       "default org used when --org omitted",
			defaultOrg: "acme",
			wantOrg:    "acme",
		},
		{
			name:       "--org overrides default",
			mutate:     func(a *insightsAccountListArgs) { a.org = "other-co" },
			defaultOrg: "acme",
			wantOrg:    "other-co",
		},
		{
			name: "--parent and --role-id propagate",
			mutate: func(a *insightsAccountListArgs) {
				a.parent = "aws-mgmt"
				a.roleID = "role-42"
			},
			defaultOrg: "acme",
			wantOrg:    "acme",
			wantParent: "aws-mgmt",
			wantRoleID: "role-42",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockInsightsAccountListClient{
				pages: []apitype.ListInsightsAccountsResponse{{Accounts: nil}},
			}
			c := &insightsAccountListCmd{clientFactory: stubAccountListFactory(client, tt.defaultOrg)}

			args := defaultListArgs()
			if tt.mutate != nil {
				tt.mutate(&args)
			}

			var out bytes.Buffer
			err := c.Run(t.Context(), &out, args)
			require.NoError(t, err)
			require.Len(t, client.calls, 1)
			assert.Equal(t, tt.wantOrg, client.calls[0].org)
			assert.Equal(t, tt.wantParent, client.calls[0].params.Parent)
			assert.Equal(t, tt.wantRoleID, client.calls[0].params.RoleID)
		})
	}
}

// TestInsightsAccountListCmd_NegativeCount guards Run against a programming
// mistake — cobra accepts a negative int but the command should refuse rather
// than send `count=-1` to the server.
func TestInsightsAccountListCmd_NegativeCount(t *testing.T) {
	t.Parallel()

	client := &mockInsightsAccountListClient{}
	c := &insightsAccountListCmd{clientFactory: stubAccountListFactory(client, "acme")}

	args := defaultListArgs()
	args.count = -1
	args.countSet = true

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--count must be non-negative")
	assert.Empty(t, client.calls, "no API call should be made on invalid --count")
}

func TestInsightsAccountListCmd_ClientError(t *testing.T) {
	t.Parallel()

	client := &mockInsightsAccountListClient{err: errors.New("403 forbidden")}
	c := &insightsAccountListCmd{clientFactory: stubAccountListFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, defaultListArgs())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing insights accounts")
	assert.Contains(t, err.Error(), "403 forbidden")
}

func TestInsightsAccountListCmd_FactoryError(t *testing.T) {
	t.Parallel()

	c := &insightsAccountListCmd{
		clientFactory: failingAccountListFactory(errors.New("not logged in")),
	}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, defaultListArgs())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestInsightsAccountListCmd_PaginationLimit(t *testing.T) {
	t.Parallel()

	// A server that always reports a next page must not spin us forever.
	// Generate listPageLimit+1 pages of nonempty cursors so the safety guard
	// trips deterministically.
	pages := make([]apitype.ListInsightsAccountsResponse, 0, listPageLimit+1)
	for i := 0; i <= listPageLimit; i++ {
		pages = append(pages, apitype.ListInsightsAccountsResponse{
			Accounts:  []apitype.InsightsAccount{sampleAccount("a")},
			NextToken: "always-more",
		})
	}
	client := &mockInsightsAccountListClient{pages: pages}
	c := &insightsAccountListCmd{clientFactory: stubAccountListFactory(client, "acme")}

	args := defaultListArgs()
	args.all = true

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pagination exceeded")
}

func TestNewInsightsAccountListCmd_FlagBinding(t *testing.T) {
	t.Parallel()

	client := &mockInsightsAccountListClient{
		pages: []apitype.ListInsightsAccountsResponse{
			{Accounts: []apitype.InsightsAccount{sampleAccount("prod-aws")}},
		},
	}
	cmd := newInsightsAccountListCmd(stubAccountListFactory(client, "acme"))
	assert.Equal(t, "list", cmd.Name())
	assert.Contains(t, cmd.Aliases, "ls")

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--org", "other-co",
		"--parent", "aws-mgmt",
		"--role-id", "role-42",
		"--output", "json",
	})
	require.NoError(t, cmd.ExecuteContext(t.Context()))

	require.Len(t, client.calls, 1)
	assert.Equal(t, "other-co", client.calls[0].org)
	assert.Equal(t, "aws-mgmt", client.calls[0].params.Parent)
	assert.Equal(t, "role-42", client.calls[0].params.RoleID)

	var got struct {
		Accounts []apitype.InsightsAccount `json:"accounts"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	require.Len(t, got.Accounts, 1)
	assert.Equal(t, "prod-aws", got.Accounts[0].Name)
}

// TestNewInsightsAccountListCmd_CountAllMutuallyExclusive ensures cobra
// surfaces the conflict before RunE fires so the user can't accidentally
// combine the two pagination knobs.
func TestNewInsightsAccountListCmd_CountAllMutuallyExclusive(t *testing.T) {
	t.Parallel()

	cmd := newInsightsAccountListCmd(stubAccountListFactory(&mockInsightsAccountListClient{}, "acme"))
	cmd.SetArgs([]string{"--count", "10", "--all"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	// Cobra's wording: "if any flags in the group [count all] are set none of
	// the others can be; [all count] were all set".
	assert.Contains(t, err.Error(), "[count all]")
}

func TestNewInsightsAccountListCmd_InvalidOutput(t *testing.T) {
	t.Parallel()

	// outputflag.OutputFlag rejects unsupported values at flag-parse time, so
	// the error fires before any API call. This protects users from typos.
	client := &mockInsightsAccountListClient{}
	cmd := newInsightsAccountListCmd(stubAccountListFactory(client, "acme"))
	cmd.SetArgs([]string{"--output", "yaml"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `output "yaml" not supported`)
	assert.Empty(t, client.calls, "no API call should be made on invalid --output")
}

func TestNewInsightsAccountListCmd_NilFactoryUsesDefault(t *testing.T) {
	t.Parallel()

	// Passing nil installs the production factory; we only check the command is
	// well-formed without invoking it, since the default factory would hit the
	// real cloud context.
	cmd := newInsightsAccountListCmd(nil)
	require.NotNil(t, cmd)
	assert.Equal(t, "list", cmd.Name())
}
