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

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturedSearchCall records the arguments a single SearchInsightsResources
// call received, so tests can assert that flags propagate correctly.
type capturedSearchCall struct {
	org    string
	params apitype.InsightsResourceSearchParams
}

// mockSearchClient stubs insightsResourceSearchClient.
type mockSearchClient struct {
	response apitype.InsightsResourceSearchResponse
	err      error
	captured *capturedSearchCall
}

func (m *mockSearchClient) SearchInsightsResources(
	_ context.Context, org string, params apitype.InsightsResourceSearchParams,
) (apitype.InsightsResourceSearchResponse, error) {
	if m.captured != nil {
		*m.captured = capturedSearchCall{org: org, params: params}
	}
	if m.err != nil {
		return apitype.InsightsResourceSearchResponse{}, m.err
	}
	return m.response, nil
}

// stubSearchFactory returns a searchClientFactory that always yields client and
// effectiveOrg. orgOverride wins so per-call --org assertions still work.
func stubSearchFactory(client insightsResourceSearchClient, defaultOrg string) searchClientFactory {
	return func(_ context.Context, orgOverride string) (insightsResourceSearchClient, string, error) {
		org := orgOverride
		if org == "" {
			org = defaultOrg
		}
		return client, org, nil
	}
}

func failingSearchFactory(err error) searchClientFactory {
	return func(_ context.Context, _ string) (insightsResourceSearchClient, string, error) {
		return nil, "", err
	}
}

func sampleSearchResponse() apitype.InsightsResourceSearchResponse {
	truthy := true
	return apitype.InsightsResourceSearchResponse{
		Total: 2,
		Resources: []apitype.InsightsResourceSearchResult{
			{
				Account:  "prod-aws",
				Type:     "aws:s3/bucket:Bucket",
				ID:       "my-bucket",
				URN:      "urn:pulumi:prod::api::aws:s3/bucket:Bucket::my-bucket",
				Stack:    "prod",
				Project:  "api",
				Modified: "2026-05-01T14:30:00Z",
				Custom:   &truthy,
			},
			{
				Account: "prod-aws",
				Type:    "aws:s3/bucket:Bucket",
				ID:      "other-bucket",
			},
		},
		Pagination: &apitype.InsightsResourceSearchPagination{
			Cursor: "bookmark",
			Next:   "/api/orgs/acme/search/resourcesv2?query=foo&cursor=next-token&size=2",
		},
	}
}

func TestInsightsResourceSearchCmd_DefaultOutput(t *testing.T) {
	t.Parallel()

	client := &mockSearchClient{response: sampleSearchResponse()}
	c := &insightsResourceSearchCmd{clientFactory: stubSearchFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, insightsResourceSearchArgs{query: "type:aws:s3"})
	require.NoError(t, err)

	output := out.String()
	// Headers are present.
	assert.Contains(t, output, "URN")
	assert.Contains(t, output, "TYPE")
	assert.Contains(t, output, "MODIFIED")
	// The full first-row URN can wrap across lines on narrow terminals, so
	// only spot-check distinctive substrings either side of a likely break.
	assert.Contains(t, output, "urn:pulumi:prod")
	assert.Contains(t, output, "my-bucket")
	// Second row lacks a URN — should fall back to <type>::<id>.
	assert.Contains(t, output, "aws:s3/bucket:Bucket::other-bucket")
	// Summary line.
	assert.Contains(t, output, "Showing 2 of 2 resources.")
	// Pagination hint surfaces the cursor.
	assert.Contains(t, output, `--cursor "next-token"`)
}

func TestInsightsResourceSearchCmd_DefaultOutput_Empty(t *testing.T) {
	t.Parallel()

	client := &mockSearchClient{response: apitype.InsightsResourceSearchResponse{}}
	c := &insightsResourceSearchCmd{clientFactory: stubSearchFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, insightsResourceSearchArgs{})
	require.NoError(t, err)
	assert.Equal(t, "No resources found.\n", out.String())
}

func TestInsightsResourceSearchCmd_DefaultOutput_PageBasedHint(t *testing.T) {
	t.Parallel()

	// When the server paginates by page number rather than cursor, the hint
	// must surface --page (+ --page-size when the URL carries `size`), not a
	// raw URL the user can't usefully copy.
	resp := sampleSearchResponse()
	resp.Pagination = &apitype.InsightsResourceSearchPagination{
		Next: "https://api.pulumi.com/api/orgs/pulumi/search/resourcesv2?page=2&size=25",
	}
	client := &mockSearchClient{response: resp}
	c := &insightsResourceSearchCmd{clientFactory: stubSearchFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, insightsResourceSearchArgs{})
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "More results available. Re-run with --page 2 --page-size 25 to continue.")
	assert.NotContains(t, output, "Next page: http")
}

func TestInsightsResourceSearchCmd_DefaultOutput_LastPage(t *testing.T) {
	t.Parallel()

	resp := sampleSearchResponse()
	// No `next` link → final page, no pagination hint expected.
	resp.Pagination = &apitype.InsightsResourceSearchPagination{Cursor: "bookmark"}
	client := &mockSearchClient{response: resp}
	c := &insightsResourceSearchCmd{clientFactory: stubSearchFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, insightsResourceSearchArgs{})
	require.NoError(t, err)
	assert.NotContains(t, out.String(), "More results available")
}

func TestInsightsResourceSearchCmd_JSONOutput(t *testing.T) {
	t.Parallel()

	client := &mockSearchClient{response: sampleSearchResponse()}
	c := &insightsResourceSearchCmd{clientFactory: stubSearchFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, insightsResourceSearchArgs{output: "json"})
	require.NoError(t, err)

	var got apitype.InsightsResourceSearchResponse
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Equal(t, sampleSearchResponse(), got)
}

func TestInsightsResourceSearchCmd_OrgAndParamsPropagate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       insightsResourceSearchArgs
		defaultOrg string
		wantOrg    string
		wantParams apitype.InsightsResourceSearchParams
	}{
		{
			name:       "default org used when --org omitted",
			args:       insightsResourceSearchArgs{query: "aws"},
			defaultOrg: "acme",
			wantOrg:    "acme",
			wantParams: apitype.InsightsResourceSearchParams{Query: "aws"},
		},
		{
			name: "--org overrides default and every param propagates",
			args: insightsResourceSearchArgs{
				org:        "other-co",
				query:      "type:aws:s3",
				sort:       []string{"modified", "name"},
				asc:        true,
				page:       2,
				size:       50,
				cursor:     "abc",
				properties: true,
				collapse:   true,
			},
			defaultOrg: "acme",
			wantOrg:    "other-co",
			wantParams: apitype.InsightsResourceSearchParams{
				Query:      "type:aws:s3",
				Sort:       []string{"modified", "name"},
				Ascending:  true,
				Page:       2,
				Size:       50,
				Cursor:     "abc",
				Properties: true,
				Collapse:   true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var captured capturedSearchCall
			client := &mockSearchClient{
				response: sampleSearchResponse(),
				captured: &captured,
			}
			c := &insightsResourceSearchCmd{clientFactory: stubSearchFactory(client, tt.defaultOrg)}

			var out bytes.Buffer
			err := c.Run(t.Context(), &out, tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.wantOrg, captured.org)
			assert.Equal(t, tt.wantParams, captured.params)
		})
	}
}

func TestInsightsResourceSearchCmd_InvalidSort(t *testing.T) {
	t.Parallel()

	client := &mockSearchClient{response: sampleSearchResponse()}
	c := &insightsResourceSearchCmd{clientFactory: stubSearchFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, insightsResourceSearchArgs{sort: []string{"bogus"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid --sort value "bogus"`)
}

func TestInsightsResourceSearchCmd_InvalidOutput(t *testing.T) {
	t.Parallel()

	client := &mockSearchClient{response: sampleSearchResponse()}
	c := &insightsResourceSearchCmd{clientFactory: stubSearchFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, insightsResourceSearchArgs{output: "yaml"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid --output value "yaml"`)
}

func TestInsightsResourceSearchCmd_TableAliasMatchesDefault(t *testing.T) {
	t.Parallel()

	// `--output table` and `--output default` must produce byte-identical
	// output — they're aliases. Verifying once here keeps the rest of the
	// table-shape assertions in one place (TestInsightsResourceSearchCmd_DefaultOutput).
	client := &mockSearchClient{response: sampleSearchResponse()}
	c := &insightsResourceSearchCmd{clientFactory: stubSearchFactory(client, "acme")}

	var defaultBuf, tableBuf bytes.Buffer
	require.NoError(t, c.Run(t.Context(), &defaultBuf, insightsResourceSearchArgs{}))
	require.NoError(t, c.Run(t.Context(), &tableBuf, insightsResourceSearchArgs{output: "table"}))
	assert.Equal(t, defaultBuf.String(), tableBuf.String())
}

func TestInsightsResourceSearchCmd_ClientError(t *testing.T) {
	t.Parallel()

	client := &mockSearchClient{err: errors.New("402 payment required")}
	c := &insightsResourceSearchCmd{clientFactory: stubSearchFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, insightsResourceSearchArgs{properties: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "searching insights resources")
	assert.Contains(t, err.Error(), "402 payment required")
}

func TestInsightsResourceSearchCmd_FactoryError(t *testing.T) {
	t.Parallel()

	c := &insightsResourceSearchCmd{clientFactory: failingSearchFactory(errors.New("not logged in"))}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, insightsResourceSearchArgs{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestNewInsightsResourceSearchCmd_FlagBinding(t *testing.T) {
	t.Parallel()

	var captured capturedSearchCall
	client := &mockSearchClient{
		response: sampleSearchResponse(),
		captured: &captured,
	}
	cmd := newInsightsResourceSearchCmd(stubSearchFactory(client, "acme"))
	assert.Equal(t, "search", cmd.Name())

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--org", "other-co",
		"--query", "type:aws:s3",
		"--sort", "modified,name",
		"--asc",
		"--page", "2",
		"--page-size", "50",
		"--cursor", "abc",
		"--properties",
		"--collapse",
		"--output", "json",
	})
	require.NoError(t, cmd.ExecuteContext(t.Context()))

	assert.Equal(t, "other-co", captured.org)
	assert.Equal(t, apitype.InsightsResourceSearchParams{
		Query:      "type:aws:s3",
		Sort:       []string{"modified", "name"},
		Ascending:  true,
		Page:       2,
		Size:       50,
		Cursor:     "abc",
		Properties: true,
		Collapse:   true,
	}, captured.params)

	var got apitype.InsightsResourceSearchResponse
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Equal(t, sampleSearchResponse(), got)
}

func TestNewInsightsResourceSearchCmd_NilFactoryUsesDefault(t *testing.T) {
	t.Parallel()

	// Passing nil installs the production factory; we only check the command is
	// well-formed without invoking it, since the default factory would hit the
	// real cloud context.
	cmd := newInsightsResourceSearchCmd(nil)
	require.NotNil(t, cmd)
	assert.Equal(t, "search", cmd.Name())
}

func TestPaginationHint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		link string
		want string
	}{
		{
			name: "cursor with relative link",
			link: "/api/orgs/acme/search/resourcesv2?query=foo&cursor=abc&size=2",
			want: `--cursor "abc"`,
		},
		{
			name: "cursor with absolute link",
			link: "https://api.pulumi.com/api/orgs/acme/search/resourcesv2?cursor=xyz",
			want: `--cursor "xyz"`,
		},
		{
			name: "cursor wins over page when both present",
			link: "/api/orgs/acme/search/resourcesv2?cursor=abc&page=2&size=25",
			want: `--cursor "abc"`,
		},
		{
			name: "page with size",
			link: "https://api.pulumi.com/api/orgs/pulumi/search/resourcesv2?page=2&size=25",
			want: "--page 2 --page-size 25",
		},
		{
			name: "page without size",
			link: "/api/orgs/acme/search/resourcesv2?page=3",
			want: "--page 3",
		},
		{
			name: "size only (no page)",
			link: "/api/orgs/acme/search/resourcesv2?size=25",
			want: "",
		},
		{
			name: "no recognised parameter",
			link: "/api/orgs/acme/search/resourcesv2?query=foo",
			want: "",
		},
		{
			name: "empty link",
			link: "",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, paginationHint(tt.link))
		})
	}
}
