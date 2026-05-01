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

package cloud

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// TestResolveBindings_FieldFillsPathParam covers the core bug fix:
// a non-context path template variable (e.g. {poolId}) resolves from a
// matching -F field, and that field is consumed — it must not appear
// in the remaining slice (i.e. it won't be sent as a query or body param).
func TestResolveBindings_FieldFillsPathParam(t *testing.T) {
	t.Parallel()
	mr := &MatchResult{
		Op: &Operation{Method: "GET", Path: "/api/orgs/{orgName}/agentPools/{poolId}"},
		Bindings: map[string]Binding{
			"orgName": {Literal: "acme"},
			"poolId":  {Placeholder: "poolId"},
		},
	}
	fields := []ParsedField{
		{Key: "poolId", Value: int64(123)},
	}
	resolved, remaining, err := resolveBindings(mr, fields, nil)
	require.NoError(t, err)
	assert.Equal(t, "acme", resolved["orgName"])
	assert.Equal(t, "123", resolved["poolId"])
	assert.Empty(t, remaining, "poolId field should have been consumed")
}

// TestResolveBindings_NonMatchingFieldPassesThrough verifies that a -F
// field whose key does not match any path template variable is left
// untouched in the remaining slice so encodeFields can route it to
// the query or body.
func TestResolveBindings_NonMatchingFieldPassesThrough(t *testing.T) {
	t.Parallel()
	mr := &MatchResult{
		Op: &Operation{Method: "GET", Path: "/api/orgs/{orgName}/agentPools/{poolId}"},
		Bindings: map[string]Binding{
			"orgName": {Literal: "acme"},
			"poolId":  {Placeholder: "poolId"},
		},
	}
	fields := []ParsedField{
		{Key: "poolId", Value: int64(123)},
		{Key: "extra", Value: "foo"},
	}
	_, remaining, err := resolveBindings(mr, fields, nil)
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	assert.Equal(t, "extra", remaining[0].Key)
	assert.Equal(t, "foo", remaining[0].Value)
}

// TestResolveBindings_AliasFieldFillsPath verifies that when the user
// writes their own alias for a path template variable (e.g. `{foobar}`
// for a spec param named `orgName`), a -F field whose key matches the
// alias fills the path — not the spec param name. Without this, an
// aliased path silently resolves from context instead.
func TestResolveBindings_AliasFieldFillsPath(t *testing.T) {
	t.Parallel()
	mr := &MatchResult{
		Op: &Operation{Method: "POST", Path: "/api/orgs/{orgName}/hooks"},
		Bindings: map[string]Binding{
			"orgName": {Placeholder: "foo"},
		},
	}
	fields := []ParsedField{
		{Key: "foo", Value: "bar"},
	}
	resolved, remaining, err := resolveBindings(mr, fields, nil)
	require.NoError(t, err)
	assert.Equal(t, "bar", resolved["orgName"])
	assert.Empty(t, remaining, "aliased -F should be consumed by the path")
}

// TestResolveBindings_FieldOverridesContextAutoResolve verifies that a
// -F field takes precedence over context auto-resolution for a
// context-kind param (orgName/projectName/stackName).
func TestResolveBindings_FieldOverridesContextAutoResolve(t *testing.T) {
	t.Parallel()
	mr := &MatchResult{
		Op: &Operation{Method: "GET", Path: "/api/orgs/{orgName}"},
		Bindings: map[string]Binding{
			"orgName": {Placeholder: "orgName"},
		},
	}
	fields := []ParsedField{
		{Key: "orgName", Value: "from-field"},
	}
	resolved, remaining, err := resolveBindings(mr, fields, nil)
	require.NoError(t, err)
	assert.Equal(t, "from-field", resolved["orgName"])
	assert.Empty(t, remaining)
}

// TestResolveBindings_FieldWithContextVarButNoPathVar: a -F orgName field
// on an endpoint whose path does NOT contain {orgName} must pass through
// as a regular body/query field. We don't hijack fields based on name alone.
func TestResolveBindings_FieldWithContextVarButNoPathVar(t *testing.T) {
	t.Parallel()
	mr := &MatchResult{
		Op:       &Operation{Method: "POST", Path: "/api/user"},
		Bindings: map[string]Binding{},
	}
	fields := []ParsedField{
		{Key: "orgName", Value: "foo"},
	}
	_, remaining, err := resolveBindings(mr, fields, nil)
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	assert.Equal(t, "orgName", remaining[0].Key)
}

// TestResolveBindings_MissingParamErrorSuggestsFlag: an unresolved
// non-context path var produces an error that suggests the -F form.
func TestResolveBindings_MissingParamErrorSuggestsFlag(t *testing.T) {
	t.Parallel()
	mr := &MatchResult{
		Op: &Operation{Method: "GET", Path: "/api/orgs/acme/agentPools/{poolId}"},
		Bindings: map[string]Binding{
			"poolId": {Placeholder: "poolId"},
		},
	}
	_, _, err := resolveBindings(mr, nil, nil)
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, ErrMissingContext, apiErr.Envelope.Error.Code)
	joined := strings.Join(apiErr.Envelope.Error.Suggestions, "|")
	assert.Contains(t, joined, "-F poolId=")
}

// TestResolveBindings_NullFieldRejected: -F poolId=null must not produce
// a literal "null" in the path; it must error.
func TestResolveBindings_NullFieldRejected(t *testing.T) {
	t.Parallel()
	mr := &MatchResult{
		Op: &Operation{Method: "GET", Path: "/api/orgs/acme/agentPools/{poolId}"},
		Bindings: map[string]Binding{
			"poolId": {Placeholder: "poolId"},
		},
	}
	fields := []ParsedField{
		{Key: "poolId", Value: nil},
	}
	_, _, err := resolveBindings(mr, fields, nil)
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, ErrInvalidFlags, apiErr.Envelope.Error.Code)
}

// TestNegotiateAccept_Default uses the op's declared primary response content type.
func TestNegotiateAccept_Default(t *testing.T) {
	t.Parallel()
	op := &Operation{
		ResponseContentType: "application/json",
		SuccessContentTypes: []string{"application/json", "text/markdown"},
	}
	got, err := negotiateAccept(op, "")
	require.NoError(t, err)
	assert.Equal(t, "application/json", got)
}

// TestNegotiateAccept_JSONWithPlainJSON and the +json variant both pass.
func TestNegotiateAccept_JSONMatchesSubtypes(t *testing.T) {
	t.Parallel()
	op := &Operation{SuccessContentTypes: []string{"application/vnd.pulumi+json"}}
	got, err := negotiateAccept(op, "json")
	require.NoError(t, err)
	assert.Equal(t, "application/vnd.pulumi+json", got)
}

// TestNegotiateAccept_MarkdownSupported picks text/markdown when declared.
func TestNegotiateAccept_MarkdownSupported(t *testing.T) {
	t.Parallel()
	op := &Operation{SuccessContentTypes: []string{"application/json", "text/markdown"}}
	got, err := negotiateAccept(op, "markdown")
	require.NoError(t, err)
	assert.Equal(t, "text/markdown", got)
}

// TestNegotiateAccept_MarkdownNotDeclared errors with a descriptive
// suggestion listing the op's actual declared types.
func TestNegotiateAccept_MarkdownNotDeclared(t *testing.T) {
	t.Parallel()
	op := &Operation{
		OperationID:         "GetX",
		SuccessContentTypes: []string{"application/json"},
	}
	_, err := negotiateAccept(op, "markdown")
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, ErrInvalidFlags, apiErr.Envelope.Error.Code)
	joined := strings.Join(apiErr.Envelope.Error.Suggestions, "|")
	assert.Contains(t, joined, "application/json")
}

// TestNegotiateAccept_InvalidValue rejects unknown --format values.
func TestNegotiateAccept_InvalidValue(t *testing.T) {
	t.Parallel()
	_, err := negotiateAccept(&Operation{}, "yaml")
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, ErrInvalidFlags, apiErr.Envelope.Error.Code)
}

// TestResolveBindings_FieldValueStringification covers the non-string
// ParsedField.Value types (int64, float64, bool) being converted to
// plain decimal / bool strings for path substitution.
func TestResolveBindings_FieldValueStringification(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		value any
		want  string
	}{
		{"int64", int64(42), "42"},
		{"float64 whole", 3.0, "3"},
		{"float64 frac", 3.14, "3.14"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"string", "literal", "literal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mr := &MatchResult{
				Op: &Operation{Method: "GET", Path: "/api/thing/{id}"},
				Bindings: map[string]Binding{
					"id": {Placeholder: "id"},
				},
			}
			fields := []ParsedField{{Key: "id", Value: tc.value}}
			resolved, _, err := resolveBindings(mr, fields, nil)
			require.NoError(t, err)
			assert.Equal(t, tc.want, resolved["id"])
		})
	}
}

// TestRawCall_ForwardsUserHeaders pins the contract that -H/--header values
// make it onto the outgoing HTTP request through buildAPIHeaders →
// Client.RawCall. Authorization is reserved (a user override is dropped by
// buildAPIHeaders and pinned again by the transport); everything else is
// forwarded and wins over encoder defaults (Accept, Content-Type).
func TestRawCall_ForwardsUserHeaders(t *testing.T) {
	t.Parallel()
	var received http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = r.Header.Clone()
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	apiClient := client.NewClient(srv.URL, "my-token", false, nil)
	hdrs := []ParsedHeader{
		{Name: "X-Trace-Id", Value: "abc-123"},
		{Name: "Idempotency-Key", Value: "key-42"},
		// User-supplied Content-Type must beat the encoder default.
		{Name: "Content-Type", Value: "application/x-yaml"},
		// Authorization override must be ignored so a typo can't leak the token.
		{Name: "Authorization", Value: "token attacker"},
	}
	resp, err := apiClient.RawCall(t.Context(), http.MethodGet, "/echo", nil, nil,
		buildAPIHeaders("application/json", "application/json", hdrs), false)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, "abc-123", received.Get("X-Trace-Id"))
	assert.Equal(t, "key-42", received.Get("Idempotency-Key"))
	assert.Equal(t, "application/x-yaml", received.Get("Content-Type"),
		"user-supplied Content-Type must win over encoder default")
	assert.Equal(t, "token my-token", received.Get("Authorization"),
		"Authorization must stay pinned to the resolved token")
}

// TestValidateFlagCombos_BodyAndInputMutuallyExclusive pins the user-visible
// error when both --body and --input are set.
func TestValidateFlagCombos_BodyAndInputMutuallyExclusive(t *testing.T) {
	t.Parallel()
	err := validateFlagCombos(&apiCommand{
		envelopeVersion: SchemaVersion,
		body:            `{"a":1}`,
		input:           "payload.json",
	})
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, ErrInvalidFlags, apiErr.Envelope.Error.Code)
	assert.Equal(t, cmdutil.ExitConfigurationError, apiErr.ExitCode)
}

// TestValidateFlagCombos_SilentAndVerboseMutuallyExclusive pins the
// user-visible error when both --silent and --verbose are set: silent
// suppresses the response body, verbose dumps everything — combining them
// is contradictory.
func TestValidateFlagCombos_SilentAndVerboseMutuallyExclusive(t *testing.T) {
	t.Parallel()
	err := validateFlagCombos(&apiCommand{
		envelopeVersion: SchemaVersion,
		silent:          true,
		verbose:         true,
	})
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, ErrInvalidFlags, apiErr.Envelope.Error.Code)
	assert.Equal(t, cmdutil.ExitConfigurationError, apiErr.ExitCode)
}

// TestEncodeFields_BodyFlagPassesThroughVerbatim verifies --body is sent as-is
// with application/json, and that any -F alongside it is routed to the query
// string (mirroring --input behavior).
func TestEncodeFields_BodyFlagPassesThroughVerbatim(t *testing.T) {
	t.Parallel()
	fields := []ParsedField{{Key: "dry", Value: true}}
	body, extras, ct, err := encodeFields(
		"POST", fields, &apiCommand{body: `{"name":"acme"}`}, &Operation{},
	)
	require.NoError(t, err)
	assert.Equal(t, `{"name":"acme"}`, string(body))
	assert.Equal(t, "application/json", ct)
	assert.Equal(t, "true", extras.Get("dry"))
}

// TestEncodeFields_NestedJSONFieldMarshalsAsObject verifies that a JSON-literal
// -F value round-trips into the body as a nested object (not a quoted string).
func TestEncodeFields_NestedJSONFieldMarshalsAsObject(t *testing.T) {
	t.Parallel()
	tags, err := parseField(`tags={"env":"prod"}`, true, nil)
	require.NoError(t, err)
	body, _, ct, err := encodeFields(
		"POST",
		[]ParsedField{{Key: "name", Value: "web"}, tags},
		&apiCommand{},
		&Operation{},
	)
	require.NoError(t, err)
	assert.Equal(t, "application/json", ct)
	assert.JSONEq(t, `{"name":"web","tags":{"env":"prod"}}`, string(body))
}

// TestFieldToQueryString_ComplexValueJSONEncoded verifies that map/slice
// values from JSON-literal -F round-trip to the query string as JSON.
func TestFieldToQueryString_ComplexValueJSONEncoded(t *testing.T) {
	t.Parallel()
	assert.Equal(t,
		`{"env":"prod"}`,
		fieldToQueryString(ParsedField{Key: "tags", Value: map[string]any{"env": "prod"}}),
	)
	assert.Equal(t,
		`["a","b"]`,
		fieldToQueryString(ParsedField{Key: "tags", Value: []any{"a", "b"}}),
	)
}

// TestChooseInputContentType covers the precedence: file extension, then
// op.BodyContentType, then the JSON default.
func TestChooseInputContentType(t *testing.T) {
	t.Parallel()
	yamlOp := &Operation{BodyContentType: "application/x-yaml"}
	jsonOp := &Operation{BodyContentType: "application/json"}
	cases := []struct {
		name  string
		input string
		op    *Operation
		want  string
	}{
		{"yaml ext beats json op", "env.yaml", jsonOp, "application/x-yaml"},
		{"yml ext beats json op", "env.yml", jsonOp, "application/x-yaml"},
		{"json ext beats yaml op", "body.json", yamlOp, "application/json"},
		{"at-prefixed yaml", "@env.yaml", nil, "application/x-yaml"},
		{"no ext, yaml op", "payload", yamlOp, "application/x-yaml"},
		{"no ext, no op → json default", "-", nil, "application/json"},
		{"unknown ext falls through to op", "env.txt", yamlOp, "application/x-yaml"},
		{"unknown ext, no op → json", "env.txt", nil, "application/json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, chooseInputContentType(tc.input, tc.op))
		})
	}
}

// TestBuildConcretePath covers template substitution with URL-safe encoding,
// double-encoding for the reserved param names, and multi-segment paths.
func TestBuildConcretePath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		path     string
		resolved map[string]string
		want     string
	}{
		{
			name:     "single_segment",
			path:     "/api/orgs/{orgName}/members",
			resolved: map[string]string{"orgName": "acme"},
			want:     "/api/orgs/acme/members",
		},
		{
			name:     "multiple_segments",
			path:     "/api/stacks/{orgName}/{projectName}/{stackName}",
			resolved: map[string]string{"orgName": "acme", "projectName": "proj", "stackName": "dev"},
			want:     "/api/stacks/acme/proj/dev",
		},
		{
			name:     "slash_in_value_is_escaped",
			path:     "/api/stacks/{orgName}/{projectName}",
			resolved: map[string]string{"orgName": "acme", "projectName": "p/q"},
			want:     "/api/stacks/acme/p%2Fq",
		},
		{
			name: "double_encoded_name_is_double_escaped",
			path: "/api/preview/insights/{orgName}/accounts/{accountName}",
			resolved: map[string]string{
				"orgName":     "acme",
				"accountName": "team/a",
			},
			want: "/api/preview/insights/acme/accounts/team%252Fa",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			op := &Operation{Path: tc.path}
			got := buildConcretePath(op, tc.resolved)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestEmitDryRun_Envelope pins the JSON envelope shape: schemaVersion, dryRun
// plan with method/URL/headers/body, Authorization redacted, body inlined as
// JSON when it's valid JSON and as a quoted string otherwise.
func TestEmitDryRun_Envelope(t *testing.T) {
	t.Parallel()
	t.Run("valid_json_body_inlined", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		err := emitDryRun(&buf, "POST", "https://example.test/api/x?y=1",
			[]ParsedHeader{{Name: "X-Trace", Value: "abc"}},
			"application/json", "application/json",
			[]byte(`{"name":"acme"}`))
		require.NoError(t, err)
		var env struct {
			SchemaVersion int `json:"schemaVersion"`
			DryRun        struct {
				Plan struct {
					Method  string            `json:"method"`
					URL     string            `json:"url"`
					Headers map[string]string `json:"headers"`
					Body    json.RawMessage   `json:"body"`
				} `json:"plan"`
			} `json:"dryRun"`
		}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
		assert.Equal(t, SchemaVersion, env.SchemaVersion)
		assert.Equal(t, "POST", env.DryRun.Plan.Method)
		assert.Equal(t, "https://example.test/api/x?y=1", env.DryRun.Plan.URL)
		assert.Equal(t, "token ***", env.DryRun.Plan.Headers["Authorization"])
		assert.Equal(t, "application/json", env.DryRun.Plan.Headers["Content-Type"])
		assert.Equal(t, "abc", env.DryRun.Plan.Headers["X-Trace"])
		assert.JSONEq(t, `{"name":"acme"}`, string(env.DryRun.Plan.Body))
	})
	t.Run("non_json_body_quoted", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		err := emitDryRun(&buf, "POST", "https://example.test/x", nil,
			"text/plain", "application/json", []byte("hello world"))
		require.NoError(t, err)
		var env struct {
			DryRun struct {
				Plan struct {
					Body json.RawMessage `json:"body"`
				} `json:"plan"`
			} `json:"dryRun"`
		}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
		var s string
		require.NoError(t, json.Unmarshal(env.DryRun.Plan.Body, &s))
		assert.Equal(t, "hello world", s)
	})
}

// TestDumpRequestVerbose_DropsUserAuthorization pins that a user-supplied
// Authorization override does NOT appear in the verbose dump — the dump must
// reflect the real wire, and APIClient.Do drops such overrides.
func TestDumpRequestVerbose_DropsUserAuthorization(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	dumpRequestVerbose(&buf, "GET", "https://example.test/api/x",
		[]ParsedHeader{
			{Name: "Authorization", Value: "token attacker"},
			{Name: "X-Trace", Value: "abc"},
		},
		"", "application/json", nil)
	stderr := buf.String()
	assert.NotContains(t, stderr, "token attacker",
		"user-supplied Authorization must be filtered out of the verbose dump")
	assert.Contains(t, stderr, "Authorization: token ***",
		"the redacted Authorization line must still appear")
	assert.Contains(t, stderr, "X-Trace: abc")
}

// fakeBody wraps a reader with a no-op Close so we can hand-build *http.Response
// values without pulling in httptest for handleResponse tests.
func fakeBody(s string) io.ReadCloser {
	return io.NopCloser(bytes.NewReader([]byte(s)))
}

// TestHandleResponse covers the main post-processing branches: body
// rendering, --silent suppression, --include status+headers, and 4xx
// returning an APIError.
func TestHandleResponse(t *testing.T) {
	t.Parallel()
	newResp := func(status int, ct, body string) *http.Response {
		return &http.Response{
			StatusCode: status,
			Status:     http.StatusText(status),
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header:     http.Header{"Content-Type": []string{ct}},
			Body:       fakeBody(body),
		}
	}

	t.Run("200_renders_body", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		require.NoError(t, handleResponse(&buf, io.Discard, newResp(200, "application/json", `{"a":1}`), &apiCommand{}))
		assert.Contains(t, buf.String(), `"a"`)
	})
	t.Run("204_empty", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		require.NoError(t, handleResponse(&buf, io.Discard, newResp(204, "application/json", ""), &apiCommand{}))
		assert.Empty(t, strings.TrimSpace(buf.String()))
	})
	t.Run("silent_suppresses_body", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		require.NoError(t, handleResponse(&buf, io.Discard, newResp(200, "application/json", `{"a":1}`),
			&apiCommand{silent: true}))
		assert.Empty(t, strings.TrimSpace(buf.String()))
	})
	t.Run("include_prints_status_line_and_headers", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		require.NoError(t, handleResponse(&buf, io.Discard, newResp(200, "application/json", `{}`),
			&apiCommand{include: true}))
		assert.Contains(t, buf.String(), "HTTP/1.1")
		assert.Contains(t, buf.String(), "Content-Type: application/json")
	})
	t.Run("silent_does_not_suppress_error", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		err := handleResponse(&buf, io.Discard, newResp(500, "application/json", `{"code":500}`),
			&apiCommand{silent: true})
		require.Error(t, err)
		var apiErr *APIError
		require.True(t, errors.As(err, &apiErr))
		assert.Equal(t, cmdutil.ExitCodeError, apiErr.ExitCode)
	})
	t.Run("4xx_returns_apierror", func(t *testing.T) {
		t.Parallel()
		err := handleResponse(io.Discard, io.Discard, newResp(404, "application/json", `{"code":404}`), &apiCommand{})
		require.Error(t, err)
		var apiErr *APIError
		require.True(t, errors.As(err, &apiErr))
		assert.Equal(t, cmdutil.ExitCodeError, apiErr.ExitCode)
		assert.Equal(t, ErrHTTP4xx, apiErr.Envelope.Error.Code)
	})
	t.Run("401_returns_auth_apierror", func(t *testing.T) {
		t.Parallel()
		err := handleResponse(io.Discard, io.Discard, newResp(401, "application/json", `{"code":401}`), &apiCommand{})
		require.Error(t, err)
		var apiErr *APIError
		require.True(t, errors.As(err, &apiErr))
		assert.Equal(t, cmdutil.ExitAuthenticationError, apiErr.ExitCode)
	})
}
