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
	set, err := parsePolicyExceptionsBlock(json.RawMessage(js))
	require.NoError(t, err)
	return set
}

// entry returns a match entry for one policy and one target.
func entry(policy string, target resource.URN) string {
	return `{"policies": ["` + policy + `"], "targets": ["` + string(target) + `"]}`
}

// matchIDs returns the IDs of the exceptions that match, in order.
func matchIDs(set *PolicyExceptions, policy string, urn resource.URN) []string {
	matches := set.match(policy, urn)
	if len(matches) == 0 {
		return nil
	}
	ids := make([]string, 0, len(matches))
	for _, e := range matches {
		ids = append(ids, e.ID)
	}
	return ids
}

func TestParsePolicyExceptionsEmpty(t *testing.T) {
	t.Parallel()

	for _, js := range []string{``, `null`, `{}`} {
		set, err := parsePolicyExceptionsBlock(json.RawMessage(js))
		require.NoError(t, err, js)
		assert.Equal(t, 0, set.len(), js)
	}
}

func TestParsePolicyExceptionsInvalid(t *testing.T) {
	t.Parallel()

	valid := entry("p", accessLogsURN)
	tests := []struct {
		name, js, err string
	}{
		{"not an object", `[]`, `"exceptions" must be an object`},
		{"entry not an object", `{"a": "x"}`, `policy exception "a": must be an object`},
		{
			"invalid ID", `{"a b": {"reason": "r", "match": [` + valid + `]}}`,
			`policy exception "a b": the ID must be non-empty`,
		},
		{"missing reason", `{"a": {"match": [` + valid + `]}}`, `policy exception "a": "reason" is required`},
		{
			"blank reason", `{"a": {"reason": " ", "match": [` + valid + `]}}`,
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
			"missing policies", `{"a": {"reason": "r", "match": [{"targets": ["` + string(accessLogsURN) + `"]}]}}`,
			`policy exception "a": match[0]: "policies" must list at least one policy`,
		},
		{
			"empty policy", `{"a": {"reason": "r", "match": [{"policies": [""], "targets": ["` +
				string(accessLogsURN) + `"]}]}}`,
			`policy exception "a": match[0]: "policies" can't contain an empty name`,
		},
		{
			"all", `{"a": {"reason": "r", "match": [` + entry("all", accessLogsURN) + `]}}`,
			`policy exception "a": match[0]: "policies" must name policies; "all" isn't allowed`,
		},
		{
			"missing targets", `{"a": {"reason": "r", "match": [{"policies": ["p"]}]}}`,
			`policy exception "a": match[0]: "targets" must list at least one URN`,
		},
		{
			"invalid urn", `{"a": {"reason": "r", "match": [` + entry("p", "access-logs") + `]}}`,
			`policy exception "a": match[0]: target "access-logs" is not a valid URN`,
		},
		{
			"error in a later entry", `{"a": {"reason": "r", "match": [` + valid + `, {"policies": ["p"]}]}}`,
			`policy exception "a": match[1]: "targets" must list at least one`,
		},
		{
			"unknown field in an entry", `{"a": {"reason": "r", "match": [{"policies": ["p"], "resources": []}]}}`,
			`policy exception "a": match[0]: unknown field "resources"`,
		},
		{
			"unknown field in an entry", `{"a": {"reason": "r", "match": [
			{"policies": ["p"], "targets": ["` + string(accessLogsURN) + `"], "type": "aws:s3/bucket:Bucket"}]}}`,
			`policy exception "a": match[0]: unknown field "type"`,
		},
		{
			"unknown field in an exception", `{"a": {"reason": "r", "expires": "2027-01-01", "match": [` + valid + `]}}`,
			`policy exception "a": unknown field "expires"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parsePolicyExceptionsBlock(json.RawMessage(tt.js))
			assert.ErrorContains(t, err, tt.err)
		})
	}
}

func TestPolicyExceptionsMatch(t *testing.T) {
	t.Parallel()

	set := mustParseExceptions(t, `{
		"log-buckets": {
			"reason": "Log target buckets",
			"match": [
				{
					"policies": ["s3-no-public-read", "s3-block-public-acls"],
					"targets": ["`+string(accessLogsURN)+`"]
				},
				{
					"policies": ["s3-no-public-read"],
					"targets": ["`+string(cfLogsURN)+`", "`+string(devLogsURN)+`"]
				}
			]
		},
		"flow-logs": {"reason": "Only in prod", "match": [`+entry("vpc-flow-logs", webStackURN)+`]}
	}`)
	require.Equal(t, 2, set.len())

	tests := []struct {
		name   string
		policy string
		urn    resource.URN
		want   []string
	}{
		{"exact match", "s3-no-public-read", accessLogsURN, []string{"log-buckets"}},
		{"another policy in the same entry", "s3-block-public-acls", accessLogsURN, []string{"log-buckets"}},
		{"another target in the same entry", "s3-no-public-read", cfLogsURN, []string{"log-buckets"}},
		{"targets span stacks", "s3-no-public-read", devLogsURN, []string{"log-buckets"}},
		{"the root stack URN", "vpc-flow-logs", webStackURN, []string{"flow-logs"}},
		{"policy and target must match in one entry", "s3-block-public-acls", cfLogsURN, nil},
		{"another policy", "s3-encryption", accessLogsURN, nil},
		{"another name", "s3-no-public-read", "urn:pulumi:prod::web::aws:s3/bucket:Bucket::access-log", nil},
		{"another type", "s3-no-public-read", "urn:pulumi:prod::web::aws:s3/bucketV2:BucketV2::access-logs", nil},
		{"an empty URN", "s3-no-public-read", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, matchIDs(set, tt.policy, tt.urn))
		})
	}

	assert.Equal(t, []apitype.PolicyEventException{{ID: "flow-logs", Reason: "Only in prod"}},
		set.match("vpc-flow-logs", webStackURN))

	var empty *PolicyExceptions
	assert.Empty(t, empty.match("p", accessLogsURN))
}

// Every matching exception is returned, lowest ID first, with digits compared as numbers.
func TestPolicyExceptionsMatchOrder(t *testing.T) {
	t.Parallel()

	match := `"match": [` + entry("p", accessLogsURN) + `]`
	set := mustParseExceptions(t, `{
		"EXC-10": {"reason": "ten", `+match+`},
		"EXC-9": {"reason": "nine", `+match+`},
		"EXC-100": {"reason": "hundred", `+match+`}
	}`)
	assert.Equal(t, []apitype.PolicyEventException{
		{ID: "EXC-9", Reason: "nine"},
		{ID: "EXC-10", Reason: "ten"},
		{ID: "EXC-100", Reason: "hundred"},
	}, set.match("p", accessLogsURN))

	assert.Equal(t, -1, compareExceptionIDs("EXC-2", "EXC-10"))
	assert.Equal(t, -1, compareExceptionIDs("EXC-2", "EXC-02a"))
	assert.Equal(t, -1, compareExceptionIDs("a", "b"))
	assert.Equal(t, 0, compareExceptionIDs("EXC-7", "EXC-7"))
	assert.Equal(t, -1, compareExceptionIDs("EXC-007", "EXC-7"), "leading zeros are ordered byte by byte")
}

func TestPolicyExceptionsUnknownPolicyWarnings(t *testing.T) {
	t.Parallel()

	set := mustParseExceptions(t, `{"a": {"reason": "r", "match": [
		{"policies": ["known", "gone"], "targets": ["`+string(accessLogsURN)+`"]},
		`+entry("gone", cfLogsURN)+`
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

	set := mustParseExceptions(t, `{"EXC-1": {"reason": "because", "match": [`+entry("p", accessLogsURN)+`]}}`)
	excepted := []apitype.PolicyEventException{{ID: "EXC-1", Reason: "because"}}

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
		assert.Equal(t, excepted, resp.Diagnostics[0].Exceptions)
		assert.Empty(t, resp.Diagnostics[1].Exceptions)

		resp, err = a.Analyze(t.Context(), plugin.AnalyzerResource{URN: cfLogsURN})
		require.NoError(t, err)
		assert.Empty(t, resp.Diagnostics[0].Exceptions)
	})

	t.Run("stack violations are matched at the URN the engine attributes them to", func(t *testing.T) {
		t.Parallel()
		a := WithPolicyExceptions(&stubAnalyzer{diags: []plugin.AnalyzeDiagnostic{{PolicyName: "p"}}}, set)

		resp, err := a.AnalyzeStack(t.Context(), nil)
		require.NoError(t, err)
		d := resp.Diagnostics[0]
		assert.Empty(t, d.Exceptions, "AnalyzeStack leaves matching to the engine")

		ApplyStackPolicyExceptions(a, &d, cfLogsURN)
		assert.Empty(t, d.Exceptions)
		ApplyStackPolicyExceptions(a, &d, accessLogsURN)
		assert.Equal(t, excepted, d.Exceptions)

		d.Exceptions = nil
		ApplyStackPolicyExceptions(inner, &d, accessLogsURN)
		assert.Empty(t, d.Exceptions, "an analyzer without exceptions matches nothing")
	})
}
