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

package analyzer

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	accessLogsURN = resource.URN("urn:pulumi:prod::web::aws:s3/bucket:Bucket::access-logs")
	cfLogsURN     = resource.URN("urn:pulumi:prod::web::aws:s3/bucket:Bucket::cf-logs")
	devLogsURN    = resource.URN("urn:pulumi:dev::web::aws:s3/bucket:Bucket::access-logs")
	webStackURN   = resource.URN("urn:pulumi:prod::web::pulumi:pulumi:Stack::web-prod")
)

func mustParseExceptions(t *testing.T, js string) *PolicyExceptions {
	t.Helper()
	set, warnings, err := ParsePolicyExceptionsBlock(json.RawMessage(js), true)
	require.NoError(t, err)
	require.Empty(t, warnings)
	return set
}

// matchID returns the ID of the exception that matches, or "" if none does.
func matchID(set *PolicyExceptions, policy string, urn resource.URN) string {
	id, _, _ := set.Match(policy, urn)
	return id
}

func TestParsePolicyExceptionsEmpty(t *testing.T) {
	t.Parallel()

	for _, js := range []string{``, `null`, `{}`} {
		set, warnings, err := ParsePolicyExceptionsBlock(json.RawMessage(js), true)
		require.NoError(t, err, js)
		assert.Empty(t, warnings, js)
		assert.Equal(t, 0, set.Len(), js)
	}
}

func TestParsePolicyExceptionsInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, js, err string
	}{
		{"not an object", `[]`, `"exceptions" must be an object`},
		{"entry not an object", `{"a": "x"}`, `policy exception "a": must be an object`},
		{
			"invalid ID", `{"a b": {"reason": "r", "match": [{"policy": "p", "urn": "` + string(accessLogsURN) + `"}]}}`,
			`policy exception "a b": the ID must be non-empty`,
		},
		{
			"missing reason", `{"a": {"match": [{"policy": "p", "urn": "` + string(accessLogsURN) + `"}]}}`,
			`policy exception "a": "reason" is required`,
		},
		{
			"blank reason", `{"a": {"reason": " ", "match": [{"policy": "p", "urn": "` + string(accessLogsURN) + `"}]}}`,
			`policy exception "a": "reason" is required`,
		},
		{"missing match", `{"a": {"reason": "r"}}`, `policy exception "a": "match" must list at least one`},
		{"empty match", `{"a": {"reason": "r", "match": []}}`, `policy exception "a": "match" must list at least one`},
		{"match not a list", `{"a": {"reason": "r", "match": {}}}`, `policy exception "a": "match" must be a list`},
		{
			"entry not an object", `{"a": {"reason": "r", "match": ["p"]}}`,
			`policy exception "a": match[0]: must be an object`,
		},
		{
			"missing policy", `{"a": {"reason": "r", "match": [{"urn": "` + string(accessLogsURN) + `"}]}}`,
			`policy exception "a": match[0]: "policy" is required`,
		},
		{
			"empty policy", `{"a": {"reason": "r", "match": [{"policy": "", "urn": "` + string(accessLogsURN) + `"}]}}`,
			`policy exception "a": match[0]: "policy" is required`,
		},
		{
			"all", `{"a": {"reason": "r", "match": [{"policy": "all", "urn": "` + string(accessLogsURN) + `"}]}}`,
			`policy exception "a": match[0]: "policy" must name one policy; "all" isn't allowed`,
		},
		{
			"missing urn", `{"a": {"reason": "r", "match": [{"policy": "p"}]}}`,
			`policy exception "a": match[0]: "urn" is required`,
		},
		{
			"invalid urn", `{"a": {"reason": "r", "match": [{"policy": "p", "urn": "access-logs"}]}}`,
			`policy exception "a": match[0]: "urn" "access-logs" is not a valid URN`,
		},
		{
			"error in a later entry", `{"a": {"reason": "r", "match": [
			{"policy": "p", "urn": "` + string(accessLogsURN) + `"}, {"policy": "p"}]}}`,
			`policy exception "a": match[1]: "urn" is required`,
		},
		{
			"policies in an entry", `{"a": {"reason": "r", "match": [
				{"policies": ["p"], "urn": "` + string(accessLogsURN) + `"}]}}`,
			`policy exception "a": match[0]: "policies" isn't supported yet; list one "policy" and "urn" per entry`,
		},
		{
			"stacks in an entry", `{"a": {"reason": "r", "match": [{"policy": "p", "stacks": ["web/prod"]}]}}`,
			`policy exception "a": match[0]: "stacks" isn't supported yet; list one "policy" and "urn" per entry`,
		},
		{
			"resources in an entry", `{"a": {"reason": "r", "match": [{"policy": "p", "resources": []}]}}`,
			`policy exception "a": match[0]: "resources" isn't supported yet; list one "policy" and "urn" per entry`,
		},
		{
			"unknown field in an entry", `{"a": {"reason": "r", "match": [
			{"policy": "p", "urn": "` + string(accessLogsURN) + `", "type": "aws:s3/bucket:Bucket"}]}}`,
			`policy exception "a": match[0]: unknown field "type"`,
		},
		{
			"unknown field in an exception", `{"a": {"reason": "r", "expires": "2027-01-01", "match": [
			{"policy": "p", "urn": "` + string(accessLogsURN) + `"}]}}`,
			`policy exception "a": unknown field "expires"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// A local file fails.
			_, _, err := ParsePolicyExceptionsBlock(json.RawMessage(tt.js), true)
			assert.ErrorContains(t, err, tt.err)

			// Exceptions from the service are skipped with a warning.
			set, warnings, err := ParsePolicyExceptionsBlock(json.RawMessage(tt.js), false)
			require.NoError(t, err)
			assert.Equal(t, 0, set.Len())
			require.Len(t, warnings, 1)
			assert.Contains(t, warnings[0], tt.err)
		})
	}
}

// Outside strict mode, an invalid exception is skipped and the others still apply.
func TestParsePolicyExceptionsSkipsInvalid(t *testing.T) {
	t.Parallel()

	set, warnings, err := ParsePolicyExceptionsBlock(json.RawMessage(`{
		"bad": {"reason": "r", "match": [{"policy": "p", "stacks": ["web/prod"]}]},
		"good": {"reason": "r", "match": [{"policy": "p", "urn": "`+string(accessLogsURN)+`"}]}
	}`), false)
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], `policy exception "bad"`)
	assert.Contains(t, warnings[0], "ignoring it. Try upgrading the Pulumi CLI")
	assert.Equal(t, 1, set.Len())
	assert.Equal(t, "good", matchID(set, "p", accessLogsURN))
}

func TestPolicyExceptionsMatch(t *testing.T) {
	t.Parallel()

	set := mustParseExceptions(t, `{
		"log-buckets": {
			"reason": "Log target buckets",
			"match": [
				{"policy": "s3-no-public-read", "urn": "`+string(accessLogsURN)+`"},
				{"policy": "s3-no-public-read", "urn": "`+string(accessLogsURN)+`"},
				{"policy": "s3-block-public-acls", "urn": "`+string(accessLogsURN)+`"},
				{"policy": "s3-no-public-read", "urn": "`+string(cfLogsURN)+`"},
				{"policy": "s3-no-public-read", "urn": "`+string(devLogsURN)+`"}
			]
		},
		"flow-logs": {"reason": "Only in prod", "match": [{"policy": "vpc-flow-logs", "urn": "`+string(webStackURN)+`"}]}
	}`)
	require.Equal(t, 2, set.Len())

	tests := []struct {
		name   string
		policy string
		urn    resource.URN
		want   string
	}{
		{"exact match", "s3-no-public-read", accessLogsURN, "log-buckets"},
		{"another policy in the same exception", "s3-block-public-acls", accessLogsURN, "log-buckets"},
		{"another resource in the same exception", "s3-no-public-read", cfLogsURN, "log-buckets"},
		{"entries span stacks", "s3-no-public-read", devLogsURN, "log-buckets"},
		{"the root stack URN", "vpc-flow-logs", webStackURN, "flow-logs"},
		{"policy and URN must match in one entry", "s3-block-public-acls", cfLogsURN, ""},
		{"another policy", "required-tags", accessLogsURN, ""},
		{"another stack", "s3-block-public-acls", devLogsURN, ""},
		{"another name", "s3-no-public-read", "urn:pulumi:prod::web::aws:s3/bucket:Bucket::access-log", ""},
		{"another type", "s3-no-public-read", "urn:pulumi:prod::web::aws:s3/bucketV2:BucketV2::access-logs", ""},
		{"an empty URN", "s3-no-public-read", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, matchID(set, tt.policy, tt.urn))
		})
	}

	id, reason, ok := set.Match("vpc-flow-logs", webStackURN)
	assert.True(t, ok)
	assert.Equal(t, "flow-logs", id)
	assert.Equal(t, "Only in prod", reason)

	var empty *PolicyExceptions
	_, _, ok = empty.Match("p", accessLogsURN)
	assert.False(t, ok)
}

// When several exceptions match, the one with the lowest ID wins, with digits compared as numbers.
func TestPolicyExceptionsMatchOrder(t *testing.T) {
	t.Parallel()

	entry := `"match": [{"policy": "p", "urn": "` + string(accessLogsURN) + `"}]`
	set := mustParseExceptions(t, `{
		"EXC-10": {"reason": "ten", `+entry+`},
		"EXC-9": {"reason": "nine", `+entry+`},
		"EXC-100": {"reason": "hundred", `+entry+`}
	}`)
	id, reason, ok := set.Match("p", accessLogsURN)
	require.True(t, ok)
	assert.Equal(t, "EXC-9", id)
	assert.Equal(t, "nine", reason)

	assert.Equal(t, -1, compareExceptionIDs("EXC-2", "EXC-10"))
	assert.Equal(t, -1, compareExceptionIDs("EXC-2", "EXC-02a"))
	assert.Equal(t, -1, compareExceptionIDs("a", "b"))
	assert.Equal(t, 0, compareExceptionIDs("EXC-7", "EXC-7"))
}

func TestPolicyExceptionsUnknownPolicyWarnings(t *testing.T) {
	t.Parallel()

	set := mustParseExceptions(t, `{"a": {"reason": "r", "match": [
		{"policy": "known", "urn": "`+string(accessLogsURN)+`"},
		{"policy": "gone", "urn": "`+string(accessLogsURN)+`"},
		{"policy": "gone", "urn": "`+string(cfLogsURN)+`"}
	]}}`)
	warnings := set.UnknownPolicyWarnings("pack", []plugin.AnalyzerPolicyInfo{{Name: "known"}})
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], `"gone"`)

	var empty *PolicyExceptions
	assert.Empty(t, empty.UnknownPolicyWarnings("pack", nil))
}

// stubAnalyzer reports the same diagnostics from Analyze and AnalyzeStack. Its other methods aren't called.
type stubAnalyzer struct {
	plugin.Analyzer
	diags []plugin.AnalyzeDiagnostic
}

func (a *stubAnalyzer) Analyze(context.Context, plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
	return plugin.AnalyzeResponse{Diagnostics: slices.Clone(a.diags)}, nil
}

func (a *stubAnalyzer) AnalyzeStack(
	context.Context, []plugin.AnalyzerStackResource,
) (plugin.AnalyzeResponse, error) {
	return plugin.AnalyzeResponse{Diagnostics: slices.Clone(a.diags)}, nil
}

func TestWithPolicyExceptions(t *testing.T) {
	t.Parallel()

	set := mustParseExceptions(t, `{
		"EXC-1": {"reason": "because", "match": [{"policy": "p", "urn": "`+string(accessLogsURN)+`"}]}
	}`)
	excepted := &apitype.PolicyEventException{ID: "EXC-1", Reason: "because"}

	// No exceptions: the analyzer is returned unchanged.
	inner := &stubAnalyzer{}
	assert.Same(t, inner, WithPolicyExceptions(inner, nil))

	t.Run("Analyze matches the analyzed resource", func(t *testing.T) {
		t.Parallel()
		a := WithPolicyExceptions(&stubAnalyzer{diags: []plugin.AnalyzeDiagnostic{
			{PolicyName: "p", EnforcementLevel: apitype.Mandatory},
			{PolicyName: "other", EnforcementLevel: apitype.Mandatory},
		}}, set)

		resp, err := a.Analyze(t.Context(), plugin.AnalyzerResource{URN: accessLogsURN})
		require.NoError(t, err)
		assert.Equal(t, excepted, resp.Diagnostics[0].Exception)
		assert.Nil(t, resp.Diagnostics[1].Exception)

		resp, err = a.Analyze(t.Context(), plugin.AnalyzerResource{URN: cfLogsURN})
		require.NoError(t, err)
		assert.Nil(t, resp.Diagnostics[0].Exception)
	})

	t.Run("AnalyzeStack matches the diagnostic's URN", func(t *testing.T) {
		t.Parallel()
		a := WithPolicyExceptions(&stubAnalyzer{diags: []plugin.AnalyzeDiagnostic{
			{PolicyName: "p", URN: accessLogsURN},
			{PolicyName: "p", URN: cfLogsURN},
		}}, set)

		resp, err := a.AnalyzeStack(t.Context(), []plugin.AnalyzerStackResource{
			{AnalyzerResource: plugin.AnalyzerResource{URN: cfLogsURN}},
			{AnalyzerResource: plugin.AnalyzerResource{URN: accessLogsURN}},
		})
		require.NoError(t, err)
		assert.Equal(t, excepted, resp.Diagnostics[0].Exception)
		assert.Nil(t, resp.Diagnostics[1].Exception)
	})
}
