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

package credentialhelper

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitialResponse(t *testing.T) {
	t.Parallel()

	response, err := decodeResponse([]byte(`{
		"version": 1,
		"backendUrl": "https://api.example.com",
		"accessToken": "pul-token",
		"headers": {"Proxy-Authorization": ["Bearer proxy-token"], "X-Session": ["first", "second"]},
		"env": {"AWS_PROFILE": "corp", "EMPTY": ""},
		"expiresAt": "2026-10-02T12:00:00Z"
	}`), Request{Reason: Initial})
	require.NoError(t, err)
	expiry := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, &Response{
		BackendURL: "https://api.example.com", AccessToken: "pul-token",
		Headers: http.Header{
			"Proxy-Authorization": {"Bearer proxy-token"}, "X-Session": {"first", "second"},
		},
		Env: map[string]string{"AWS_PROFILE": "corp", "EMPTY": ""}, ExpiresAt: &expiry,
	}, response)
	assert.True(t, response.HasHTTPCredentials())
}

func TestInitialResponseDecline(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`{}`, `{"version":1}`, " \n{}\t\n"} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			response, err := decodeResponse([]byte(raw), Request{Reason: Initial})
			require.NoError(t, err)
			assert.Nil(t, response)
			assert.False(t, response.HasHTTPCredentials())
		})
	}
}

func TestInvalidResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{"empty", ""},
		{"null", `null`},
		{"array", `[]`},
		{"string", `"secret-value"`},
		{"malformed", `{"version":1,"accessToken":"secret-value",`},
		{"multiple objects", `{"version":1,"accessToken":"secret-value"} {}`},
		{"trailing output", `{"version":1} secret-value`},
		{"missing version", `{"accessToken":"secret-value"}`},
		{"unsupported version", `{"version":2}`},
		{"zero version", `{"version":0}`},
		{"zero version with credentials", `{"version":0,"accessToken":"secret-value"}`},
		{"string version", `{"version":"1"}`},
		{"fractional version", `{"version":1.0}`},
		{"unknown field", `{"version":1,"secret-value":"secret-value"}`},
		{"number token", `{"version":1,"accessToken":42}`},
		{"token newline", `{"version":1,"accessToken":"secret-value\r\nInjected: true"}`},
		{"header object type", `{"version":1,"headers":[]}`},
		{"header array type", `{"version":1,"headers":{"X-Session":"secret-value"}}`},
		{"header element type", `{"version":1,"headers":{"X-Session":[1]}}`},
		{"invalid header name", `{"version":1,"headers":{"secret-value:extra":["value"]}}`},
		{"header newline", `{"version":1,"headers":{"X-Session":["secret-value\r\nInjected: true"]}}`},
		{"env object type", `{"version":1,"env":[]}`},
		{"env value type", `{"version":1,"env":{"AWS_PROFILE":42}}`},
		{"empty env name", `{"version":1,"env":{"":"secret-value"}}`},
		{"invalid env name", `{"version":1,"env":{"secret-value=extra":"value"}}`},
		{"env nul", `{"version":1,"env":{"AWS_PROFILE":"secret-value\u0000"}}`},
		{"timestamp type", `{"version":1,"expiresAt":123}`},
		{"invalid timestamp", `{"version":1,"expiresAt":"secret-value"}`},
		{"backend type", `{"version":1,"backendUrl":42}`},
		{"empty backend", `{"version":1,"backendUrl":""}`},
		{"empty timestamp", `{"version":1,"accessToken":"secret-value","expiresAt":""}`},
		{"null version", `{"version":null}`},
		{"null token", `{"accessToken":null}`},
		{"null backend", `{"version":1,"backendUrl":null}`},
		{"null headers", `{"version":1,"headers":null}`},
		{"null header values", `{"version":1,"headers":{"X-Session":null}}`},
		{"null header element", `{"version":1,"headers":{"X-Session":[null]}}`},
		{"null env", `{"version":1,"env":null}`},
		{"null env value", `{"version":1,"env":{"AWS_PROFILE":null}}`},
		{"null timestamp", `{"version":1,"expiresAt":null}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			response, err := decodeResponse([]byte(tt.raw), Request{Reason: Initial})
			require.Error(t, err)
			assert.Nil(t, response)
			assert.NotContains(t, err.Error(), "secret-value")
		})
	}
}

func TestForbiddenHeaders(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"Authorization", "Connection", "Content-Length", "Host", "Keep-Alive", "Proxy-Connection",
		"TE", "Trailer", "Transfer-Encoding", "Upgrade",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for _, variant := range []string{name, strings.ToLower(name), strings.ToUpper(name)} {
				raw, err := json.Marshal(map[string]any{"version": 1, "headers": map[string][]string{variant: {"secret-value"}}})
				require.NoError(t, err)
				_, err = decodeResponse(raw, Request{Reason: Initial})
				require.ErrorContains(t, err, "forbidden HTTP header")
				assert.Contains(t, err.Error(), variant)
				assert.NotContains(t, err.Error(), "secret-value")
			}
		})
	}
}

func TestForbiddenEnvironment(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"PULUMI_ACCESS_TOKEN", "PULUMI_BACKEND_URL", "PULUMI_API", "PULUMI_CREDENTIAL_HELPER",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for _, variant := range []string{name, strings.ToLower(name)} {
				raw, err := json.Marshal(map[string]any{"version": 1, "env": map[string]string{variant: "secret-value"}})
				require.NoError(t, err)
				_, err = decodeResponse(raw, Request{Reason: Initial})
				require.ErrorContains(t, err, "forbidden environment variable")
				assert.NotContains(t, err.Error(), "secret-value")
			}
		})
	}
}

func TestSelectedBackendCannotBeReplaced(t *testing.T) {
	t.Parallel()
	for _, value := range []string{`"https://api.example.com"`, `"https://other.example.com"`} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			request := Request{Reason: Initial, SelectedBackendURL: "https://api.example.com"}
			_, err := decodeResponse([]byte(`{"version":1,"backendUrl":`+value+`}`), request)
			require.ErrorContains(t, err, "without a selected backend")
		})
	}
}

func TestRefreshResponse(t *testing.T) {
	t.Parallel()
	for _, reason := range []Reason{Expired, Rejected} {
		t.Run(string(reason), func(t *testing.T) {
			t.Parallel()
			request := Request{Reason: reason, SelectedBackendURL: "https://api.example.com"}
			response, err := decodeResponse([]byte(`{"version":1,"accessToken":"new-token"}`), request)
			require.NoError(t, err)
			assert.Equal(t, &Response{AccessToken: "new-token"}, response)
		})
	}
}

func TestHeaderOnlyRefresh(t *testing.T) {
	t.Parallel()
	request := Request{Reason: Rejected, SelectedBackendURL: "https://api.example.com"}
	response, err := decodeResponse([]byte(`{"version":1,"headers":{"X-New-Session":["new"]}}`), request)
	require.NoError(t, err)
	assert.Equal(t, &Response{Headers: http.Header{"X-New-Session": {"new"}}}, response)
	assert.True(t, response.HasHTTPCredentials())
}

func TestInvalidRefreshResponse(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		`{}`, `{"version":1}`, `{"version":1,"accessToken":""}`, `{"version":1,"headers":{"X-Session":[]}}`,
		`{"version":1,"accessToken":"new-token","backendUrl":"https://api.example.com"}`,
		`{"version":1,"accessToken":"new-token","backendUrl":""}`,
		`{"version":1,"accessToken":"new-token","env":{}}`,
	} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			request := Request{Reason: Expired, SelectedBackendURL: "https://api.example.com"}
			response, err := decodeResponse([]byte(raw), request)
			require.Error(t, err)
			assert.Nil(t, response)
		})
	}
}
