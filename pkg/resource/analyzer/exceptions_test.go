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
	accessLogsURN = "urn:pulumi:prod::web::aws:s3/bucket:Bucket::access-logs"
	cfLogsURN     = "urn:pulumi:prod::web::aws:s3/bucket:Bucket::cf-logs"
	webStackURN   = "urn:pulumi:prod::web::pulumi:pulumi:Stack::web-prod"
)

func mustParseExceptions(t *testing.T, js string) *PolicyExceptions {
	t.Helper()
	set, warnings, err := ParsePolicyExceptionsBlock(json.RawMessage(js), true)
	require.NoError(t, err)
	require.Empty(t, warnings)
	return set
}

func TestParsePolicyExceptionsValid(t *testing.T) {
	t.Parallel()

	set := mustParseExceptions(t, `{
		"log-buckets": {
			"policies": ["s3-no-public-read", "s3-block-public-acls"],
			"urns": ["`+accessLogsURN+`", "`+cfLogsURN+`"],
			"reason": "Log target buckets"
		},
		"EXC-42": {"policies": ["required-tags"], "stacks": ["legacy/prod"], "reason": "Migrating"}
	}`)
	assert.Equal(t, 2, set.Len())
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
		{"bad id", `{"a b": {"policies": ["p"], "stacks": ["p/s"], "reason": "r"}}`, "the ID"},
		{"no policies", `{"a": {"stacks": ["p/s"], "reason": "r"}}`, `"policies"`},
		{"empty policy", `{"a": {"policies": [""], "stacks": ["p/s"], "reason": "r"}}`, `empty name`},
		{"all", `{"a": {"policies": ["all"], "stacks": ["p/s"], "reason": "r"}}`, `"all"`},
		{"no reason", `{"a": {"policies": ["p"], "stacks": ["p/s"]}}`, `"reason"`},
		{"blank reason", `{"a": {"policies": ["p"], "stacks": ["p/s"], "reason": "  "}}`, `"reason"`},
		{"no targets", `{"a": {"policies": ["p"], "reason": "r"}}`, "at least one target"},
		{"bad stack", `{"a": {"policies": ["p"], "stacks": ["prod"], "reason": "r"}}`, "<project>/<stack>"},
		{"org stack", `{"a": {"policies": ["p"], "stacks": ["o/p/s"], "reason": "r"}}`, "<project>/<stack>"},
		{"bad urn", `{"a": {"policies": ["p"], "urns": ["access-logs"], "reason": "r"}}`, "not a valid URN"},
		{"wrong type", `{"a": {"policies": "p", "stacks": ["p/s"], "reason": "r"}}`, `policy exception "a"`},
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

// An entry with a field this version doesn't understand is skipped in both modes, because the field might
// narrow the exception.
func TestParsePolicyExceptionsUnknownField(t *testing.T) {
	t.Parallel()

	js := `{
		"a": {"policies": ["p"], "stacks": ["p/s"], "reason": "r", "tags": {"env": "dev"}},
		"b": {"policies": ["p"], "stacks": ["p/s"], "reason": "r"}
	}`
	for _, strict := range []bool{true, false} {
		set, warnings, err := ParsePolicyExceptionsBlock(json.RawMessage(js), strict)
		require.NoError(t, err)
		assert.Equal(t, 1, set.Len())
		id, _, ok := set.Match("p", resource.URN("urn:pulumi:s::p::pulumi:pulumi:Stack::p-s"))
		assert.True(t, ok)
		assert.Equal(t, "b", id)
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], `"a" has unrecognized fields (tags)`)
	}
}

func TestPolicyExceptionsMatch(t *testing.T) {
	t.Parallel()

	set := mustParseExceptions(t, `{
		"log-buckets": {
			"policies": ["s3-no-public-read", "s3-block-public-acls"],
			"urns": ["`+accessLogsURN+`", "`+cfLogsURN+`"],
			"reason": "Log target buckets"
		},
		"a-web-prod": {"policies": ["s3-no-public-read", "stack-policy"], "stacks": ["web/prod"], "reason": "Whole stack"},
		"b-web-prod": {"policies": ["s3-no-public-read"], "stacks": ["web/prod"], "reason": "Also whole stack"},
		"billing": {
			"policies": ["required-tags"],
			"stacks": ["legacy/staging"],
			"urns": ["urn:pulumi:prod::web::aws:ec2/instance:Instance::billing-bridge"],
			"reason": "Tags applied outside Pulumi"
		}
	}`)

	tests := []struct {
		name   string
		policy string
		urn    string
		wantID string
	}{
		{"urn beats stack", "s3-no-public-read", accessLogsURN, "log-buckets"},
		{
			"stack match, lowest id wins", "s3-no-public-read",
			"urn:pulumi:prod::web::aws:s3/bucket:Bucket::other", "a-web-prod",
		},
		{"policy narrows", "s3-versioning", accessLogsURN, ""},
		{"second policy of entry", "s3-block-public-acls", cfLogsURN, "log-buckets"},
		{"other stack", "s3-no-public-read", "urn:pulumi:dev::web::aws:s3/bucket:Bucket::access-logs", ""},
		{"stack-level violation on root stack", "stack-policy", webStackURN, "a-web-prod"},
		{
			"targets add up: urn from another stack", "required-tags",
			"urn:pulumi:prod::web::aws:ec2/instance:Instance::billing-bridge", "billing",
		},
		{"targets add up: stack", "required-tags", "urn:pulumi:staging::legacy::aws:ec2/instance:Instance::x", "billing"},
		{"empty urn", "stack-policy", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			id, e, ok := set.Match(tt.policy, resource.URN(tt.urn))
			assert.Equal(t, tt.wantID != "", ok)
			assert.Equal(t, tt.wantID, id)
			if ok {
				assert.NotEmpty(t, e.Reason)
			}
		})
	}
}

func TestPolicyExceptionsUnknownPolicyWarnings(t *testing.T) {
	t.Parallel()

	set := mustParseExceptions(t, `{"a": {"policies": ["known", "gone"], "stacks": ["p/s"], "reason": "r"}}`)
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

	set := mustParseExceptions(t, `{"EXC-1": {"policies": ["p"], "urns": ["`+accessLogsURN+`"], "reason": "because"}}`)
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

		resp, err := a.AnalyzeStack(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, excepted, resp.Diagnostics[0].Exception)
		assert.Nil(t, resp.Diagnostics[1].Exception)
	})
}
