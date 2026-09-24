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

package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestA2AUsesCurrentBackendAuthentication(t *testing.T) {
	t.Parallel()
	var origin string
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "token active-cli-token", r.Header.Get("Authorization"))
		assert.Equal(t, "1.0", r.Header.Get("A2A-Version"))
		assert.Equal(t, approvalExtension, r.Header.Get("A2A-Extensions"))
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, err := fmt.Fprintf(w, `{
 "supportedInterfaces":[{"url":%q,"protocolBinding":"JSONRPC","protocolVersion":"1.0"}],
 "capabilities":{"streaming":true}
}`, origin+"/api/preview/agents/acme/a2a/custom-id")
			require.NoError(t, err)
			return
		}
		var rpc struct {
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&rpc))
		assert.Equal(t, "SendMessage", rpc.Method)
		assert.Equal(t, map[string]any{"returnImmediately": false}, rpc.Params["configuration"])
		_, err := io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{
 "task":{"id":"a2a-task","contextId":"conversation","status":{"state":"TASK_STATE_COMPLETED"}}
}}`)
		require.NoError(t, err)
	}))
	defer server.Close()
	origin = server.URL
	connection := &a2aClient{caller: client.NewClient(server.URL, "active-cli-token", false, nil), cloudURL: server.URL}
	cmd := newAgentsCmd(func(context.Context) (*a2aClient, error) { return connection, nil })
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader(`{
 "message":{"messageId":"stable-id","role":"ROLE_USER","parts":[{"text":"Inspect my stack"}]},
 "configuration":{"returnImmediately":false}
}`))
	cmd.SetArgs([]string{"a2a", "send", "custom-id", "--org", "acme", "--request", "-", "--json"})
	require.NoError(t, cmd.Execute())
	assert.JSONEq(t,
		`{"task":{"id":"a2a-task","contextId":"conversation","status":{"state":"TASK_STATE_COMPLETED"}}}`,
		out.String())
	assert.NotContains(t, out.String(), "active-cli-token")
	assert.Equal(t, []string{
		"GET /api/preview/agents/acme/a2a/custom-id/.well-known/agent-card.json",
		"POST /api/preview/agents/acme/a2a/custom-id",
	}, calls)
}

func TestA2ARejectsCredentialForwarding(t *testing.T) {
	t.Parallel()
	for _, target := range []string{
		"https://attacker.example/api/preview/agents/acme/a2a/custom-id",
		"http://api.pulumi.com/api/preview/agents/acme/a2a/custom-id",
		"https://user:password@api.pulumi.com/api/preview/agents/acme/a2a/custom-id",
		"https://api.pulumi.com/api/user",
		"https://api.pulumi.com/api/preview/agents/../../user",
		"https://api.pulumi.com/api/preview/agents/acme/a2a/%2e%2e",
		"https://api.pulumi.com/api/preview/agents/acme/a2a/custom%2fid",
		"https://api.pulumi.com/api/preview/agents/acme/a2a/custom-id?redirect=elsewhere",
	} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			var card agentCard
			raw := fmt.Sprintf(`{
 "supportedInterfaces":[{"url":%q,"protocolBinding":"JSONRPC","protocolVersion":"1.0"}]
}`, target)
			require.NoError(t, json.Unmarshal([]byte(raw), &card))
			_, err := (&a2aClient{cloudURL: "https://api.pulumi.com"}).endpoint(card)
			require.Error(t, err)
		})
	}
}

func TestA2AStreamPreservesResponsesAndErrors(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	err := readSSE(strings.NewReader(`: heartbeat

id: 1
data: {"jsonrpc":"2.0","id":1,
data: "result":{"task":{"id":"task"}}}

data: {"jsonrpc":"2.0","id":1,"error":{"code":-32001,"message":"Task not found",
data: "data":[{"reason":"TASK_NOT_FOUND"}]}}

`), &out)
	var protocolErr *rpcError
	require.ErrorAs(t, err, &protocolErr)
	assert.Equal(t, -32001, protocolErr.Code)
	assert.Contains(t, err.Error(), "TASK_NOT_FOUND")
	assert.Equal(t, "{\"task\":{\"id\":\"task\"}}\n", out.String())
}

func TestA2AValidatesBeforeConnecting(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"a2a", "discover"},
		{"a2a", "get", "task", "--org", "acme"},
		{"a2a", "send", "agent", "--org", "acme"},
		{"a2a", "list", "--agent", "agent", "--org", "acme", "--page-size", "0"},
		{"a2a", "get", "task", "--agent", "agent", "--org", "acme", "--history-length", "-1"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()
			cmd := newAgentsCmd(func(context.Context) (*a2aClient, error) {
				t.Fatal("must validate before using credentials")
				return nil, nil
			})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(args)
			require.Error(t, cmd.Execute())
		})
	}
}

func TestA2AStreamDetectsUnexpectedClosure(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"TASK_STATE_WORKING", "TASK_STATE_COMPLETED", "TASK_STATE_INPUT_REQUIRED"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			payload := fmt.Sprintf(`data: {"jsonrpc":"2.0","id":1,"result":{"statusUpdate":{"status":{"state":%q}}}}

`, state)
			err := readSSE(strings.NewReader(payload), io.Discard)
			if state == "TASK_STATE_WORKING" {
				require.ErrorContains(t, err, "use get or watch")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDelegationSkillDoesNotRequireLogin(t *testing.T) {
	t.Parallel()
	cmd := newAgentsCmd(func(context.Context) (*a2aClient, error) {
		t.Fatal("skill must not access credentials")
		return nil, nil
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"a2a", "skill"})
	require.NoError(t, cmd.Execute())
	assert.Equal(t, delegationSkill, out.String())
	assert.Contains(t, out.String(), "pulumi agents a2a discover")
}

func TestA2ADoesNotFollowRedirects(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirected" {
			t.Error("redirect must not be followed")
		}
		http.Redirect(w, r, "/redirected", http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	connection := &a2aClient{caller: client.NewClient(server.URL, "test-token", false, nil), cloudURL: server.URL}
	_, err := connection.request(t.Context(), http.MethodPost, "/original", []byte(`{}`), false)
	require.ErrorContains(t, err, "HTTP 307")
}

func TestA2ARefreshesTheActiveCredential(t *testing.T) {
	t.Parallel()
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/oauth/token" {
			require.NoError(t, r.ParseForm())
			assert.Equal(t, "active-refresh", r.Form.Get("refresh_token"))
			_, err := io.WriteString(w, `{"access_token":"fresh-access","token_type":"Bearer","expires_in":3600}`)
			require.NoError(t, err)
			return
		}
		calls = append(calls, r.Header.Get("Authorization"))
		if len(calls) == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, err := io.WriteString(w, `{"agents":[]}`)
		require.NoError(t, err)
	}))
	defer server.Close()
	active := client.NewClient(server.URL, "expired-access", false, nil)
	var saved string
	active.WithRefresh("active-refresh", func(token string, _ time.Time, _ string) error { saved = token; return nil })
	connection := &a2aClient{caller: active, cloudURL: server.URL}
	response, err := connection.request(t.Context(), http.MethodGet, catalogPath("acme"), nil, false)
	require.NoError(t, err)
	defer response.Body.Close()
	assert.Equal(t, []string{"token expired-access", "token fresh-access"}, calls)
	assert.Equal(t, "fresh-access", saved)
}
