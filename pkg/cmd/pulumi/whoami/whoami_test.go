// Copyright 2023, Pulumi Corporation.
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

package whoami

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/agentdetect"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWhoAmICmdDoesNotCreateAgentAccount(t *testing.T) {
	t.Setenv("AI_AGENT", "codex")
	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	t.Setenv("PULUMI_CREDENTIALS_PATH", "")
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", t.TempDir())
	t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")
	var requests atomic.Int32
	var signups atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.URL.Path {
		case "/api/user":
			assert.Equal(t, "token existing-agent-token", r.Header.Get("Authorization"))
			if err := json.NewEncoder(w).Encode(map[string]any{
				"githubLogin": "existing-agent", "organizations": []any{},
			}); err != nil {
				t.Errorf("encoding user response: %v", err)
			}
		case "/api/capabilities":
			if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
				t.Errorf("encoding capabilities response: %v", err)
			}
		case "/api/agents/signup":
			signups.Add(1)
			w.WriteHeader(http.StatusBadRequest)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("PULUMI_BACKEND_URL", server.URL)

	for _, cached := range []bool{false, true} {
		if cached {
			require.NoError(t, workspace.StoreAgentAccount(server.URL, workspace.Account{
				AccessToken:     "existing-agent-token",
				Username:        "existing-agent",
				LastValidatedAt: time.Now(),
			}, true))
		}
		var output bytes.Buffer
		cmd := NewWhoAmICmd(pkgWorkspace.Instance, cmdBackend.DefaultLoginManager)
		cmd.SetOut(&output)
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		err := cmd.Execute()
		if cached {
			require.NoError(t, err)
			assert.Equal(t, "existing-agent\n", output.String())
		} else {
			require.ErrorIs(t, err, backenderr.ErrLoginRequired)
			assert.Empty(t, output.String())
			assert.Zero(t, requests.Load())
		}
		assert.Zero(t, signups.Load(), "identity lookup must not create an account")
	}
}

func TestWhoAmICmd_default(t *testing.T) { //nolint:paralleltest // clears agent-detection environment
	clearAgentDetection(t)

	ws := &pkgWorkspace.MockContext{}
	be := &backend.MockBackend{
		CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
			return "user1", []string{"org1", "org2"}, nil, nil
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(
			context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project, bool, bool, colors.Colorization,
		) (backend.Backend, error) {
			return be, nil
		},
	}

	var buff bytes.Buffer
	cmd := NewWhoAmICmd(ws, lm)
	cmd.SetOut(&buff)
	err := cmd.Execute()
	require.NoError(t, err)

	assert.Equal(t, "user1\n", buff.String())
}

func TestWhoAmICmd_verbose(t *testing.T) { //nolint:paralleltest // clears agent-detection environment
	clearAgentDetection(t)

	ws := &pkgWorkspace.MockContext{}
	be := &backend.MockBackend{
		CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
			return "user2", []string{"org1", "org2"}, nil, nil
		},
		URLF: func() string {
			return "https://pulumi.example.com"
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(
			context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project, bool, bool, colors.Colorization,
		) (backend.Backend, error) {
			return be, nil
		},
	}

	var buff bytes.Buffer
	cmd := NewWhoAmICmd(ws, lm)
	cmd.SetArgs([]string{"--verbose"})
	cmd.SetOut(&buff)
	err := cmd.Execute()
	require.NoError(t, err)

	stdout := buff.String()
	assert.Contains(t, stdout, "User: user2")
	assert.Contains(t, stdout, "Organizations: org1, org2")
	assert.Contains(t, stdout, "Backend URL: https://pulumi.example.com")
	assert.Contains(t, stdout, "Token type: personal")
}

func TestWhoAmICmd_json(t *testing.T) { //nolint:paralleltest // clears agent-detection environment
	clearAgentDetection(t)

	ws := &pkgWorkspace.MockContext{}
	be := &backend.MockBackend{
		CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
			return "user3", []string{"org1", "org2"}, nil, nil
		},
		URLF: func() string {
			return "https://pulumi.example.com"
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(
			context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project, bool, bool, colors.Colorization,
		) (backend.Backend, error) {
			return be, nil
		},
	}

	var buff bytes.Buffer
	cmd := NewWhoAmICmd(ws, lm)
	cmd.SetArgs([]string{"--json"})
	cmd.SetOut(&buff)
	err := cmd.Execute()
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"user": "user3",
		"organizations": ["org1", "org2"],
		"url": "https://pulumi.example.com"
	}`, buff.String())
}

func TestWhoAmICmd_verbose_teamToken(t *testing.T) { //nolint:paralleltest // clears agent-detection environment
	clearAgentDetection(t)

	ws := &pkgWorkspace.MockContext{}
	be := &backend.MockBackend{
		CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
			return "user2", []string{"org1", "org2"}, &workspace.TokenInformation{
				Name: "team-token",
				Team: "myTeam",
			}, nil
		},
		URLF: func() string {
			return "https://pulumi.example.com"
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(
			context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project, bool, bool, colors.Colorization,
		) (backend.Backend, error) {
			return be, nil
		},
	}

	var buff bytes.Buffer
	cmd := NewWhoAmICmd(ws, lm)
	cmd.SetArgs([]string{"--verbose"})
	cmd.SetOut(&buff)
	err := cmd.Execute()
	require.NoError(t, err)

	stdout := buff.String()
	assert.Contains(t, stdout, "User: user2")
	assert.Contains(t, stdout, "Organizations: org1, org2")
	assert.Contains(t, stdout, "Backend URL: https://pulumi.example.com")
	assert.Contains(t, stdout, "Token type: team: myTeam")
	assert.Contains(t, stdout, "Token name: team-token")
}

func TestWhoAmICmd_json_teamToken(t *testing.T) { //nolint:paralleltest // clears agent-detection environment
	clearAgentDetection(t)

	ws := &pkgWorkspace.MockContext{}
	be := &backend.MockBackend{
		CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
			return "user3", []string{"org1", "org2"}, &workspace.TokenInformation{
				Name: "team-token",
				Team: "myTeam",
			}, nil
		},
		URLF: func() string {
			return "https://pulumi.example.com"
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(
			context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project, bool, bool, colors.Colorization,
		) (backend.Backend, error) {
			return be, nil
		},
	}

	var buff bytes.Buffer
	cmd := NewWhoAmICmd(ws, lm)
	cmd.SetArgs([]string{"--json"})
	cmd.SetOut(&buff)
	err := cmd.Execute()
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"user": "user3",
		"organizations": ["org1", "org2"],
		"tokenInformation": {"name": "team-token", "team": "myTeam"},
		"url": "https://pulumi.example.com"
	}`, buff.String())
}

func TestWhoAmICmd_verbose_unknownToken(t *testing.T) { //nolint:paralleltest // clears agent-detection environment
	clearAgentDetection(t)

	ws := &pkgWorkspace.MockContext{}
	be := &backend.MockBackend{
		CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
			return "user2", []string{"org1", "org2"}, &workspace.TokenInformation{
				Name: "bad-token",
			}, nil
		},
		URLF: func() string {
			return "https://pulumi.example.com"
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(
			context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project, bool, bool, colors.Colorization,
		) (backend.Backend, error) {
			return be, nil
		},
	}

	var buff bytes.Buffer
	cmd := NewWhoAmICmd(ws, lm)
	cmd.SetArgs([]string{"--verbose"})
	cmd.SetOut(&buff)
	err := cmd.Execute()
	require.NoError(t, err)

	stdout := buff.String()
	assert.Contains(t, stdout, "User: user2")
	assert.Contains(t, stdout, "Organizations: org1, org2")
	assert.Contains(t, stdout, "Backend URL: https://pulumi.example.com")
	assert.Contains(t, stdout, "Token type: unknown")
	assert.Contains(t, stdout, "Token name: bad-token")
}

func clearAgentDetection(t *testing.T) {
	t.Helper()
	for _, name := range agentdetect.DetectionEnvVars() {
		t.Setenv(name, "")
	}
}
