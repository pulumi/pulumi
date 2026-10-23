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
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func stubPolicyIssueExportFactory(c policyIssueExportClient, org string) policyIssueExportClientFactory {
	return func(_ context.Context, _ string) (policyIssueExportClient, string, error) {
		return c, org, nil
	}
}

func runPolicyIssueExportCmd(
	t *testing.T, c policyIssueExportClient, args ...string,
) (string, error) {
	t.Helper()
	cmd := newPolicyIssueExportCmdWith(stubPolicyIssueExportFactory(c, "acme"))
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

func TestPolicyIssueExport_InvalidFilterFailsBeforeFactory(t *testing.T) {
	t.Parallel()

	cmd := newPolicyIssueExportCmdWith(func(context.Context, string) (policyIssueExportClient, string, error) {
		t.Fatal("factory must not be called when a filter is invalid")
		return nil, "", nil
	})
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
	require.ErrorIs(t, err, assert.AnError)
	assert.Equal(t, assert.AnError.Error(), err.Error())
	assert.NoFileExists(t, path)
}
