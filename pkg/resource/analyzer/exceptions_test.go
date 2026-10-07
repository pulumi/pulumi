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
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	accessLogsURN = resource.URN("urn:pulumi:prod::web::aws:s3/bucket:Bucket::access-logs")
	cfLogsURN     = resource.URN("urn:pulumi:prod::web::aws:s3/bucket:Bucket::cf-logs")
	auditTableURN = resource.URN("urn:pulumi:prod::web::aws:dynamodb/table:Table::audit-events")
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
func matchID(set *PolicyExceptions, policy string, urn resource.URN, props property.Map) string {
	id, _, _ := set.Match(policy, urn, props)
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
		name    string
		json    string
		wantErr string
	}{
		{"not an object", `[]`, "must be an object"},
		{"entry not an object", `{"a": "b"}`, "must be an object"},
		{"bad id", `{"a b": {"reason": "r"}}`, "the ID"},
		{"no reason", `{"a": {"policies": ["p"]}}`, `"reason"`},
		{"blank reason", `{"a": {"reason": "  "}}`, `"reason"`},
		{"empty policy", `{"a": {"policies": [""], "reason": "r"}}`, `empty name`},
		{"policy pattern", `{"a": {"policies": ["s3-*"], "reason": "r"}}`, "exact name"},
		{"stack pattern", `{"a": {"stacks": ["*/prod"], "reason": "r"}}`, "exact name"},
		{"bad stack", `{"a": {"stacks": ["prod"], "reason": "r"}}`, "<project>/<stack>"},
		{"org stack", `{"a": {"stacks": ["o/p/s"], "reason": "r"}}`, "<project>/<stack>"},
		{"wrong type", `{"a": {"policies": "p", "reason": "r"}}`, `policy exception "a"`},
		{"resources not a list", `{"a": {"resources": {}, "reason": "r"}}`, `"resources" must be a list`},
		{"selector not an object", `{"a": {"resources": ["x"], "reason": "r"}}`, "resources[0]: must be an object"},
		{"bad urn", `{"a": {"resources": [{"urn": "access-logs"}], "reason": "r"}}`, "not a valid URN"},
		{
			"urn contradicts type",
			`{"a": {"resources": [{"urn": "` + string(accessLogsURN) + `", "type": "aws:dynamodb/table:Table"}], ` +
				`"reason": "r"}}`,
			`never matches "type"`,
		},
		{
			"urn contradicts name",
			`{"a": {"resources": [{"urn": "` + string(accessLogsURN) + `", "name": "cf-*"}], "reason": "r"}}`,
			`never matches "name"`,
		},
		{
			"urn outside stacks",
			`{"a": {"stacks": ["web/dev"], "resources": [{"urn": "` + string(accessLogsURN) + `"}], "reason": "r"}}`,
			`"stacks" doesn't include`,
		},
		{
			"bad property path", `{"a": {"resources": [{"properties": {"tags[": "x"}}], "reason": "r"}}`,
			"invalid property path",
		},
		{"object value", `{"a": {"resources": [{"properties": {"tags": {"env": "dev"}}}], "reason": "r"}}`, "longer path"},
		{"empty value list", `{"a": {"resources": [{"properties": {"tags.env": []}}], "reason": "r"}}`, "never matches"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Strict: an error.
			_, _, err := ParsePolicyExceptionsBlock(json.RawMessage(tt.json), true)
			assert.ErrorContains(t, err, tt.wantErr)

			// Not strict: a warning, and the entry is ignored.
			set, warnings, err := ParsePolicyExceptionsBlock(json.RawMessage(tt.json), false)
			require.NoError(t, err)
			assert.Equal(t, 0, set.Len())
			require.Len(t, warnings, 1)
			assert.Contains(t, warnings[0], tt.wantErr)
		})
	}
}

// An entry with a field this version doesn't understand, at the top level or in a selector, is skipped in both
// modes, because the field might narrow the exception.
func TestParsePolicyExceptionsUnknownField(t *testing.T) {
	t.Parallel()

	js := `{
		"a": {"stacks": ["web/prod"], "reason": "r", "expires": "2027-01-01"},
		"b": {"resources": [{"name": "access-logs", "tags": {"env": "dev"}}], "reason": "r"},
		"c": {"stacks": ["web/prod"], "reason": "r"}
	}`
	for _, strict := range []bool{true, false} {
		set, warnings, err := ParsePolicyExceptionsBlock(json.RawMessage(js), strict)
		require.NoError(t, err)
		assert.Equal(t, 1, set.Len())
		assert.Equal(t, "c", matchID(set, "p", accessLogsURN, property.Map{}))
		require.Len(t, warnings, 2)
		assert.Contains(t, warnings[0], `"a" has unrecognized fields (expires)`)
		assert.Contains(t, warnings[1], `"b" has unrecognized fields (resources[0].tags)`)
	}
}

// Only the reason is required: an exception without filters excepts every violation of the pack.
func TestPolicyExceptionsNoFilters(t *testing.T) {
	t.Parallel()

	set := mustParseExceptions(t, `{"everything": {"reason": "Audit mode"}}`)
	assert.Equal(t, "everything", matchID(set, "any-policy", accessLogsURN, property.Map{}))
	assert.Equal(t, "everything", matchID(set, "any-policy", "", property.Map{}))
}

func TestPolicyExceptionsMatch(t *testing.T) {
	t.Parallel()

	set := mustParseExceptions(t, `{
		"log-buckets": {
			"policies": ["s3-no-public-read", "s3-versioning"],
			"resources": [{"urn": "`+string(accessLogsURN)+`"}, {"urn": "`+string(cfLogsURN)+`"}],
			"reason": "Log target buckets"
		},
		"prod-web": {"policies": ["stack-policy"], "stacks": ["web/prod"], "reason": "Whole stack"},
		"archive": {
			"policies": ["kms-encryption"],
			"stacks": ["web/prod"],
			"resources": [
				{"type": "aws:s3/bucket:Bucket", "name": "access-logs"},
				{"type": "aws:dynamodb/table:Table", "name": "audit-*"}
			],
			"reason": "Compliance archive"
		}
	}`)

	tests := []struct {
		name   string
		policy string
		urn    resource.URN
		wantID string
	}{
		{"policy and urn", "s3-no-public-read", accessLogsURN, "log-buckets"},
		{"urn is exact", "s3-no-public-read", accessLogsURN + "-2", ""},
		{"second selector", "s3-versioning", cfLogsURN, "log-buckets"},
		{"policy narrows", "required-tags", accessLogsURN, ""},
		{"stack only", "stack-policy", webStackURN, "prod-web"},
		{"stack only, other stack", "stack-policy", "urn:pulumi:dev::web::pulumi:pulumi:Stack::web-dev", ""},
		{"selector type and name", "kms-encryption", accessLogsURN, "archive"},
		{"other selector", "kms-encryption", auditTableURN, "archive"},
		{"no cross product", "kms-encryption", "urn:pulumi:prod::web::aws:s3/bucket:Bucket::audit-x", ""},
		{"stacks and resources both apply", "kms-encryption", "urn:pulumi:dev::web::aws:dynamodb/table:Table::audit-x", ""},
		{"empty urn", "stack-policy", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.wantID, matchID(set, tt.policy, tt.urn, property.Map{}))
		})
	}
}

func TestPolicyExceptionsMatchProperties(t *testing.T) {
	t.Parallel()

	set := mustParseExceptions(t, `{
		"physical-name": {"resources": [{"properties": {"bucket": "prod-logs-*"}}], "reason": "r"},
		"nested": {
			"resources": [{"properties": {"tags.env": ["dev", "staging"], "tags[\"team.name\"]": "data"}}],
			"reason": "r"
		},
		"scalars": {
			"resources": [{"properties": {"mapPublicIpOnLaunch": true, "port": 22, "kmsKeyId": null}}],
			"reason": "r"
		},
		"wildcard-path": {"resources": [{"properties": {"ingress[*].cidrBlocks[*]": "10.*"}}], "reason": "r"}
	}`)
	str := property.New[string]
	tags := func(kv ...string) property.Value {
		m := map[string]property.Value{}
		for i := 0; i < len(kv); i += 2 {
			m[kv[i]] = str(kv[i+1])
		}
		return property.New(property.NewMap(m))
	}

	tests := []struct {
		name   string
		props  map[string]property.Value
		wantID string
	}{
		{"glob on string", map[string]property.Value{"bucket": str("prod-logs-eu")}, "physical-name"},
		{"glob mismatch", map[string]property.Value{"bucket": str("dev-logs")}, ""},
		{"nested paths AND", map[string]property.Value{"tags": tags("env", "dev", "team.name", "data")}, "nested"},
		{"nested value list", map[string]property.Value{"tags": tags("env", "staging", "team.name", "data")}, "nested"},
		{"nested one fails", map[string]property.Value{"tags": tags("env", "prod", "team.name", "data")}, ""},
		{"nested missing", map[string]property.Value{"tags": tags("env", "dev")}, ""},
		{
			"scalars, missing is null",
			map[string]property.Value{"mapPublicIpOnLaunch": property.New(true), "port": property.New(22.0)},
			"scalars",
		},
		{
			"scalars, explicit null",
			map[string]property.Value{
				"mapPublicIpOnLaunch": property.New(true), "port": property.New(22.0), "kmsKeyId": property.New(property.Null),
			},
			"scalars",
		},
		{
			"scalar type matters",
			map[string]property.Value{"mapPublicIpOnLaunch": str("true"), "port": property.New(22.0)},
			"",
		},
		{
			"wildcard path, any element",
			map[string]property.Value{"ingress": property.New(property.NewArray([]property.Value{
				property.New(property.NewMap(map[string]property.Value{
					"cidrBlocks": property.New(property.NewArray([]property.Value{str("0.0.0.0/0"), str("10.0.0.0/8")})),
				})),
			}))},
			"wildcard-path",
		},
		{"unknown never matches", map[string]property.Value{"bucket": property.New(property.Computed)}, ""},
		{"secret never matches", map[string]property.Value{"bucket": str("prod-logs-eu").WithSecret(true)}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.wantID, matchID(set, "p", accessLogsURN, property.NewMap(tt.props)))
		})
	}
}

// When several exceptions match, the lowest ID wins, with digits compared as numbers.
func TestPolicyExceptionsMatchOrder(t *testing.T) {
	t.Parallel()

	set := mustParseExceptions(t, `{
		"EXC-10": {"reason": "ten"},
		"EXC-9": {"reason": "nine"},
		"EXC-100": {"reason": "hundred"}
	}`)
	id, reason, ok := set.Match("p", accessLogsURN, property.Map{})
	require.True(t, ok)
	assert.Equal(t, "EXC-9", id)
	assert.Equal(t, "nine", reason)

	assert.Equal(t, -1, compareExceptionIDs("EXC-2", "EXC-10"))
	assert.Equal(t, -1, compareExceptionIDs("EXC-2", "EXC-02a"))
	assert.Equal(t, -1, compareExceptionIDs("a", "b"))
	assert.Equal(t, 0, compareExceptionIDs("EXC-7", "EXC-7"))
}

func TestGlob(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pattern, value string
		want           bool
	}{
		{"admin", "admin", true},
		{"admin", "superadmin", false}, // anchored
		{"my.bucket", "myXbucket", false},
		{"prod-*", "prod-logs", true},
		{"prod-*", "prod-", true},
		{"*-logs", "cf-logs", true},
		{"a*b*c", "aXXbYYc", true},
		{"a*b*c", "acb", false},
		{"ab*ba", "aba", false}, // the prefix and suffix can't overlap
		{"**", "", true},
		{"*", "anything", true},
		{"dev[b*", "dev[box", true},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, compileGlob(tt.pattern).match(tt.value), "%q ~ %q", tt.pattern, tt.value)
	}
}

func TestPolicyExceptionsUnknownPolicyWarnings(t *testing.T) {
	t.Parallel()

	set := mustParseExceptions(t, `{"a": {"policies": ["known", "gone"], "reason": "r"}}`)
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
		"EXC-1": {"policies": ["p"], "resources": [{"urn": "`+string(accessLogsURN)+`"}], "reason": "because"},
		"EXC-2": {"policies": ["by-prop"], "resources": [{"properties": {"bucket": "prod-*"}}], "reason": "prop"}
	}`)
	excepted := &apitype.PolicyEventException{ID: "EXC-1", Reason: "because"}
	bucketProps := property.NewMap(map[string]property.Value{"bucket": property.New("prod-logs")})

	// No exceptions: the analyzer is returned unchanged.
	inner := &stubAnalyzer{}
	assert.Same(t, inner, WithPolicyExceptions(inner, nil))

	t.Run("Analyze matches the analyzed resource and its inputs", func(t *testing.T) {
		t.Parallel()
		a := WithPolicyExceptions(&stubAnalyzer{diags: []plugin.AnalyzeDiagnostic{
			{PolicyName: "p", EnforcementLevel: apitype.Mandatory},
			{PolicyName: "other", EnforcementLevel: apitype.Mandatory},
			{PolicyName: "by-prop", EnforcementLevel: apitype.Mandatory},
		}}, set)

		resp, err := a.Analyze(t.Context(), plugin.AnalyzerResource{URN: accessLogsURN, Properties: bucketProps})
		require.NoError(t, err)
		assert.Equal(t, excepted, resp.Diagnostics[0].Exception)
		assert.Nil(t, resp.Diagnostics[1].Exception)
		assert.Equal(t, "EXC-2", resp.Diagnostics[2].Exception.ID)

		resp, err = a.Analyze(t.Context(), plugin.AnalyzerResource{URN: cfLogsURN})
		require.NoError(t, err)
		assert.Nil(t, resp.Diagnostics[0].Exception)
		assert.Nil(t, resp.Diagnostics[2].Exception)
	})

	t.Run("AnalyzeStack matches the diagnostic's resource and its outputs", func(t *testing.T) {
		t.Parallel()
		a := WithPolicyExceptions(&stubAnalyzer{diags: []plugin.AnalyzeDiagnostic{
			{PolicyName: "p", URN: accessLogsURN},
			{PolicyName: "p", URN: cfLogsURN},
			{PolicyName: "by-prop", URN: cfLogsURN},
			{PolicyName: "by-prop", URN: accessLogsURN},
		}}, set)

		resp, err := a.AnalyzeStack(t.Context(), []plugin.AnalyzerStackResource{
			{AnalyzerResource: plugin.AnalyzerResource{URN: cfLogsURN, Properties: bucketProps}},
			{AnalyzerResource: plugin.AnalyzerResource{URN: accessLogsURN}},
		})
		require.NoError(t, err)
		assert.Equal(t, excepted, resp.Diagnostics[0].Exception)
		assert.Nil(t, resp.Diagnostics[1].Exception)
		assert.Equal(t, "EXC-2", resp.Diagnostics[2].Exception.ID)
		assert.Nil(t, resp.Diagnostics[3].Exception)
	})
}
