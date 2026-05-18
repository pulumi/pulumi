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

package deployment

// AI Generated - needs human review

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturedEditPatch records what was sent to PatchStackDeploymentSettings so
// tests can assert the call shape.
type capturedEditPatch struct {
	stack client.StackIdentifier
	patch json.RawMessage
}

// mockDeploymentSettingsEditClient stubs the patch + get pair. patchErr fires
// from PatchStackDeploymentSettings; getResp / getErr come from the follow-up
// GetStackDeploymentSettings.
type mockDeploymentSettingsEditClient struct {
	patchErr error
	getResp  *apitype.DeploymentSettings
	getErr   error
	captured *capturedEditPatch
}

func (m *mockDeploymentSettingsEditClient) PatchStackDeploymentSettings(
	_ context.Context, stack client.StackIdentifier, patch json.RawMessage,
) error {
	if m.captured != nil {
		m.captured.stack = stack
		m.captured.patch = patch
	}
	return m.patchErr
}

func (m *mockDeploymentSettingsEditClient) GetStackDeploymentSettings(
	_ context.Context, _ client.StackIdentifier,
) (*apitype.DeploymentSettings, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.getResp, nil
}

func stubSettingsEditFactory(c deploymentSettingsEditClient) deploymentSettingsEditClientFactory {
	return func(_ context.Context, _ string) (deploymentSettingsEditClient, client.StackIdentifier, error) {
		return c, testStackID, nil
	}
}

func failingSettingsEditFactory(err error) deploymentSettingsEditClientFactory {
	return func(_ context.Context, _ string) (deploymentSettingsEditClient, client.StackIdentifier, error) {
		return nil, client.StackIdentifier{}, err
	}
}

// writePatchFile writes content to a temp file and returns its absolute path.
func writePatchFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "patch.json")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

func TestDeploymentSettingsEdit_DefaultOutput(t *testing.T) {
	t.Parallel()

	patchJSON := `{"sourceContext":{"git":{"branch":"feature"}}}`
	patchFile := writePatchFile(t, patchJSON)

	captured := &capturedEditPatch{}
	c := &mockDeploymentSettingsEditClient{
		getResp:  sampleDeploymentSettings(),
		captured: captured,
	}

	var buf bytes.Buffer
	err := runDeploymentSettingsEdit(t.Context(), &buf, nil,
		stubSettingsEditFactory(c),
		deploymentSettingsEditArgs{file: patchFile, outputFormat: defaultDeploymentSettingsGetOutputFormat()})
	require.NoError(t, err)

	assert.Equal(t, testStackID, captured.stack)
	require.NotNil(t, captured.patch)
	assert.JSONEq(t, patchJSON, string(captured.patch))

	assert.Equal(t, `Source: GitHub
  Repository:               acme/infra
  Branch:                   main
  Commit:                   abc123
  Pulumi.yaml folder:       stacks/prod
  Run previews for PRs:     yes
  Run updates on push:      yes
  PR stack template:        no
  Path filters:             stacks/prod/**

Deployment runner
  Runner pool:              pool-1
  Executor image:           pulumi/pulumi:latest
  Working directory:        /work

Pre-run commands
  echo hi

Environment variables
  BAZ
  FOO
`, buf.String())
}

func TestDeploymentSettingsEdit_JSONOutput(t *testing.T) {
	t.Parallel()

	patchFile := writePatchFile(t, `{"sourceContext":{"git":{"branch":"feature"}}}`)

	c := &mockDeploymentSettingsEditClient{getResp: sampleDeploymentSettings()}

	args := deploymentSettingsEditArgs{file: patchFile, outputFormat: defaultDeploymentSettingsGetOutputFormat()}
	require.NoError(t, args.outputFormat.Set("json"))
	var buf bytes.Buffer
	err := runDeploymentSettingsEdit(t.Context(), &buf, nil,
		stubSettingsEditFactory(c), args)
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"source": {
			"kind": "github",
			"repository": "acme/infra",
			"branch": "main",
			"commit": "abc123",
			"folder": "stacks/prod",
			"previewPullRequests": true,
			"runUpdatesOnPush": true,
			"pullRequestTemplate": false,
			"pathFilters": ["stacks/prod/**"]
		},
		"runner": {
			"pool": "pool-1",
			"executorImage": "pulumi/pulumi:latest",
			"workingDirectory": "/work"
		},
		"preRunCommands": ["echo hi"],
		"environmentVariables": ["BAZ", "FOO"]
	}`, buf.String())
}

func TestDeploymentSettingsEdit_EmptyFileFlag(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentSettingsEditClient{getResp: &apitype.DeploymentSettings{}}

	var buf bytes.Buffer
	err := runDeploymentSettingsEdit(t.Context(), &buf, nil,
		stubSettingsEditFactory(c),
		deploymentSettingsEditArgs{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--file is required")
}

func TestDeploymentSettingsEdit_FileDoesNotExist(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentSettingsEditClient{getResp: &apitype.DeploymentSettings{}}

	var buf bytes.Buffer
	err := runDeploymentSettingsEdit(t.Context(), &buf, nil,
		stubSettingsEditFactory(c),
		deploymentSettingsEditArgs{file: filepath.Join(t.TempDir(), "missing.json")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading deployment settings patch")
}

func TestDeploymentSettingsEdit_EmptyFileContent(t *testing.T) {
	t.Parallel()

	patchFile := writePatchFile(t, "   \n\t")

	c := &mockDeploymentSettingsEditClient{getResp: &apitype.DeploymentSettings{}}

	var buf bytes.Buffer
	err := runDeploymentSettingsEdit(t.Context(), &buf, nil,
		stubSettingsEditFactory(c),
		deploymentSettingsEditArgs{file: patchFile, outputFormat: defaultDeploymentSettingsGetOutputFormat()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "patch file is empty")
}

func TestDeploymentSettingsEdit_MalformedJSON(t *testing.T) {
	t.Parallel()

	patchFile := writePatchFile(t, `{"sourceContext": `) // unterminated

	c := &mockDeploymentSettingsEditClient{getResp: &apitype.DeploymentSettings{}}

	var buf bytes.Buffer
	err := runDeploymentSettingsEdit(t.Context(), &buf, nil,
		stubSettingsEditFactory(c),
		deploymentSettingsEditArgs{file: patchFile, outputFormat: defaultDeploymentSettingsGetOutputFormat()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading deployment settings patch")
}

func TestDeploymentSettingsEdit_PatchError(t *testing.T) {
	t.Parallel()

	patchFile := writePatchFile(t, `{}`)

	c := &mockDeploymentSettingsEditClient{patchErr: errors.New("boom")}

	var buf bytes.Buffer
	err := runDeploymentSettingsEdit(t.Context(), &buf, nil,
		stubSettingsEditFactory(c),
		deploymentSettingsEditArgs{file: patchFile, outputFormat: defaultDeploymentSettingsGetOutputFormat()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "editing deployment settings")
	assert.Contains(t, err.Error(), "boom")
}

func TestDeploymentSettingsEdit_GetAfterPatchError(t *testing.T) {
	t.Parallel()

	patchFile := writePatchFile(t, `{}`)

	c := &mockDeploymentSettingsEditClient{getErr: errors.New("get boom")}

	var buf bytes.Buffer
	err := runDeploymentSettingsEdit(t.Context(), &buf, nil,
		stubSettingsEditFactory(c),
		deploymentSettingsEditArgs{file: patchFile, outputFormat: defaultDeploymentSettingsGetOutputFormat()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting deployment settings")
	assert.Contains(t, err.Error(), "get boom")
}

func TestDeploymentSettingsEdit_FactoryError(t *testing.T) {
	t.Parallel()

	patchFile := writePatchFile(t, `{}`)

	var buf bytes.Buffer
	err := runDeploymentSettingsEdit(t.Context(), &buf, nil,
		failingSettingsEditFactory(errors.New("not logged in")),
		deploymentSettingsEditArgs{file: patchFile, outputFormat: defaultDeploymentSettingsGetOutputFormat()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestDeploymentSettingsEdit_StdinPatch(t *testing.T) {
	t.Parallel()

	captured := &capturedEditPatch{}
	c := &mockDeploymentSettingsEditClient{
		getResp:  &apitype.DeploymentSettings{},
		captured: captured,
	}

	patchJSON := `{"sourceContext":{"git":{"branch":"from-stdin"}}}`
	stdin := strings.NewReader(patchJSON)

	var buf bytes.Buffer
	err := runDeploymentSettingsEdit(t.Context(), &buf, stdin,
		stubSettingsEditFactory(c),
		deploymentSettingsEditArgs{file: "-", outputFormat: defaultDeploymentSettingsGetOutputFormat()})
	require.NoError(t, err)

	require.NotNil(t, captured.patch)
	assert.JSONEq(t, patchJSON, string(captured.patch))
}

// TestDeploymentSettingsEdit_ForwardsExplicitNull pins the documented
// "null property means delete" semantic: if the user writes a literal null,
// the bytes must reach the server unchanged so the server-side merge clears
// the field.
func TestDeploymentSettingsEdit_ForwardsExplicitNull(t *testing.T) {
	t.Parallel()

	patchJSON := `{"operationContext":{"environmentVariables":null}}`
	patchFile := writePatchFile(t, patchJSON)

	captured := &capturedEditPatch{}
	c := &mockDeploymentSettingsEditClient{
		getResp:  &apitype.DeploymentSettings{},
		captured: captured,
	}

	var buf bytes.Buffer
	err := runDeploymentSettingsEdit(t.Context(), &buf, nil,
		stubSettingsEditFactory(c),
		deploymentSettingsEditArgs{file: patchFile, outputFormat: defaultDeploymentSettingsGetOutputFormat()})
	require.NoError(t, err)

	require.NotNil(t, captured.patch)
	assert.JSONEq(t, patchJSON, string(captured.patch),
		"explicit null must reach the server so the merge deletes the property")
	assert.Contains(t, string(captured.patch), "null",
		"the literal null must survive — JSONEq alone wouldn't catch a re-encoding that dropped it")
}

func TestDeploymentSettingsEdit_RejectsUnknownFields(t *testing.T) {
	t.Parallel()

	// Typo: enviornmentVariables instead of environmentVariables.
	patchFile := writePatchFile(t, `{"operationContext":{"enviornmentVariables":{"X":"y"}}}`)

	c := &mockDeploymentSettingsEditClient{getResp: &apitype.DeploymentSettings{}}

	var buf bytes.Buffer
	err := runDeploymentSettingsEdit(t.Context(), &buf, nil,
		stubSettingsEditFactory(c),
		deploymentSettingsEditArgs{file: patchFile, outputFormat: defaultDeploymentSettingsGetOutputFormat()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enviornmentVariables")
}
