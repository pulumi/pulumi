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

package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const exportCSV = "id,policyName\ni1,p\n"

type mockPolicyIssueExportClient struct {
	csv    string
	err    error
	org    string
	req    cloudapitype.AngularGridGetRowsRequest
	closed bool
}

func (m *mockPolicyIssueExportClient) ExportPolicyIssues(
	_ context.Context, org string, req cloudapitype.AngularGridGetRowsRequest, closed bool,
) (io.ReadCloser, error) {
	m.org, m.req, m.closed = org, req, closed
	if m.err != nil {
		return nil, m.err
	}
	return io.NopCloser(strings.NewReader(m.csv)), nil
}

func newPolicyIssueExportServer(t *testing.T, c *mockPolicyIssueExportClient) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		org, ok := strings.CutSuffix(strings.TrimPrefix(r.URL.Path, "/api/orgs/"), "/policyresults/issues/export")
		if !ok {
			http.NotFound(w, r)
			return
		}
		var req cloudapitype.AngularGridGetRowsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		stream, err := c.ExportPolicyIssues(r.Context(), org, req, r.URL.Query().Get("closed") == "true")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(apitype.ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}
		defer stream.Close()
		w.Header().Set("Content-Type", "text/csv")
		_, _ = io.Copy(w, stream)
	}))
	t.Cleanup(server.Close)
	return server
}

func newPolicyIssueExportTestLoginManager(be backend.Backend) cmdBackend.LoginManager {
	return &cmdBackend.MockLoginManager{
		LoginF: func(
			context.Context, pkgWorkspace.Context, diag.Sink,
			string, *workspace.Project, bool, bool, colors.Colorization, ...cmdBackend.LoginOption,
		) (backend.Backend, error) {
			return be, nil
		},
	}
}

func runPolicyIssueExportCmd(
	t *testing.T, c *mockPolicyIssueExportClient, args ...string,
) (string, error) {
	t.Helper()
	server := newPolicyIssueExportServer(t, c)
	be := &httpstate.MockHTTPBackend{
		MockBackend: backend.MockBackend{
			GetDefaultOrgF: func(context.Context) (string, error) { return "acme", nil },
			CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
				return "user", []string{"acme"}, nil, nil
			},
		},
		FClient: func() *client.Client { return client.NewClient(server.URL, "", false, nil) },
	}
	cmd := newPolicyIssueExportCmd(&pkgWorkspace.MockContext{}, newPolicyIssueExportTestLoginManager(be))
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(t.Context())
	return out.String(), err
}

func TestPolicyIssueExport_StdoutAndFile(t *testing.T) {
	t.Parallel()

	c := &mockPolicyIssueExportClient{csv: exportCSV}
	out, err := runPolicyIssueExportCmd(t, c)
	require.NoError(t, err)
	assert.Equal(t, exportCSV, out)
	assert.Equal(t, "acme", c.org)

	path := filepath.Join(t.TempDir(), "issues.csv")
	require.NoError(t, os.WriteFile(path, []byte("stale content that is longer than the export"), 0o600))
	out, err = runPolicyIssueExportCmd(t, c, "--file", path)
	require.NoError(t, err)
	assert.Empty(t, out)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, exportCSV, string(data))
}

func TestPolicyIssueExport_PassesFilters(t *testing.T) {
	t.Parallel()

	c := &mockPolicyIssueExportClient{csv: exportCSV}
	_, err := runPolicyIssueExportCmd(t, c,
		"--closed", "--severity", "high", "--assigned-to", "alice", "--sort", "observedAt:desc")
	require.NoError(t, err)
	assert.True(t, c.closed)
	require.NotNil(t, c.req.FilterModel)
	require.NotEmpty(t, c.req.SortModel)
	assert.Zero(t, c.req.StartRow)
	assert.Zero(t, c.req.EndRow)
}

func TestPolicyIssueExport_InvalidFilterFailsBeforeRequest(t *testing.T) {
	t.Parallel()

	c := &mockPolicyIssueExportClient{csv: exportCSV}
	_, err := runPolicyIssueExportCmd(t, c, "--severity", "urgent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "critical")
	assert.Empty(t, c.org)
}

func TestPolicyIssueExport_InvalidFilterFailsBeforeLogin(t *testing.T) {
	t.Parallel()

	lm := &cmdBackend.MockLoginManager{
		LoginF: func(
			context.Context, pkgWorkspace.Context, diag.Sink,
			string, *workspace.Project, bool, bool, colors.Colorization, ...cmdBackend.LoginOption,
		) (backend.Backend, error) {
			t.Fatal("login must not happen when a filter is invalid")
			return nil, nil
		},
	}
	cmd := newPolicyIssueExportCmd(&pkgWorkspace.MockContext{}, lm)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--severity", "urgent"})
	err := cmd.ExecuteContext(t.Context())
	assert.ErrorContains(t, err, "critical")
}

func TestPolicyIssueExport_ServerError(t *testing.T) {
	t.Parallel()

	c := &mockPolicyIssueExportClient{err: assert.AnError}
	path := filepath.Join(t.TempDir(), "issues.csv")
	_, err := runPolicyIssueExportCmd(t, c, "--file", path)
	require.ErrorContains(t, err, assert.AnError.Error())
	assert.NoFileExists(t, path)
}
