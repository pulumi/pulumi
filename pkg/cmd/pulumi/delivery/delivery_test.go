// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package delivery

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
)

func TestDeliveryCommandSurface(t *testing.T) {
	t.Parallel()
	command := NewDeliveryCmd()
	names := make([]string, 0, len(command.Commands()))
	for _, child := range command.Commands() {
		names = append(names, child.Name())
	}
	assert.ElementsMatch(t, []string{"status", "releases", "history", "logs", "pause", "resume", "retry",
		"redeploy", "promote", "rollback", "signal", "report", "report-source", "approve", "preview-candidate"}, names)
}

func TestReadCandidatePreviewManifest(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "manifest.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
		"candidateId":"candidate","shapeVersion":2,"releaseId":"release",
		"changeRequestId":"cr","revisionNumber":3,"workflowRunId":"run","workspace":"/workspace",
		"members":[{"urn":"urn:pulumi:test::p::delivery:index:Stack::app","name":"app","stage":"test",
		"targetStack":"acme/app/dev","directory":"app","dependencies":[]}]
	}`), 0o600))
	manifest, err := readCandidatePreviewManifest(path)
	require.NoError(t, err)
	assert.Equal(t, "candidate", manifest.CandidateID)
	assert.Equal(t, "acme/app/dev", manifest.Members[0].TargetStack)
}

func TestReadCandidatePreviewManifestUsesWorkflowEnvironment(t *testing.T) {
	t.Setenv("PULUMI_WORKFLOW_RUN_ID", "allocated-run")
	path := filepath.Join(t.TempDir(), "manifest.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
		"candidateId":"candidate","shapeVersion":2,"releaseId":"release",
		"changeRequestId":"cr","revisionNumber":3,"workspace":"/workspace",
		"members":[{"urn":"urn","targetStack":"acme/app/dev","directory":"app"}]
	}`), 0o600))
	manifest, err := readCandidatePreviewManifest(path)
	require.NoError(t, err)
	assert.Equal(t, "allocated-run", manifest.WorkflowRunID)
}

func TestCandidatePreviewEnvironmentHelpers(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"HOME=/tmp/home", "PATH=/bin", "TOKEN=secret"},
		mergedEnvironment([]string{"HOME=/old", "PATH=/bin"}, map[string]string{"TOKEN": "secret", "HOME": "/tmp/home"}))
	assert.Equal(t, map[string]string{
		"acme/base": "acme/base@42",
		"acme/live": "acme/live",
	}, environmentRevisions([]string{"acme/base@42", "acme/live"}))
	assert.JSONEq(t, `{"diagnostic":"token=[secret]","other":"safe"}`,
		string(redactJSONSecrets(json.RawMessage(`{"diagnostic":"token=s3cr\"et","other":"safe"}`),
			[]string{`s3cr"et`})))
}

func TestRunCandidatePreviewRejectsSymlinkOutsideWorkspace(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(workspace, "member")))
	_, _, _, err := runCandidatePreview(t.Context(), candidatePreviewManifest{
		Workspace: workspace,
		Members:   []candidatePreviewMember{{Name: "member", TargetStack: "acme/app/dev", Directory: "member"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the release workspace")
}

func TestStageActionRequiresReadVersion(t *testing.T) {
	t.Parallel()
	command := NewDeliveryCmd()
	command.SetArgs([]string{"pause", "production"})
	err := command.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--version is required")
}

func TestPromoteRequiresRelease(t *testing.T) {
	t.Parallel()
	command := NewDeliveryCmd()
	command.SetArgs([]string{"promote", "production", "--version", "7"})
	err := command.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--release is required")
}

func TestDeliveryPipelineEnvironmentSetsStack(t *testing.T) {
	t.Setenv("PULUMI_DELIVERY_PIPELINE", "acme/shop/pipeline")
	command := NewDeliveryCmd()
	stack, err := command.PersistentFlags().GetString("stack")
	require.NoError(t, err)
	assert.Equal(t, "acme/shop/pipeline", stack)
}

func TestHumanStatusShowsPausedAndMemberReason(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	err := (&command{}).render(&output, client.DeliveryPipelineSnapshot{
		ID: "pipeline-id", Name: "web", ShapeVersion: 4,
		Stages: []client.DeliveryStageSnapshot{{
			Name: "production", State: "failed", Paused: true, PauseReason: "incident", Version: 7,
			Members: []map[string]any{{
				"name": "deploy", "state": "failed", "transition": map[string]any{"message": "health check failed"},
			}},
		}},
	})
	require.NoError(t, err)
	assert.Contains(t, output.String(), "pipeline-id")
	assert.Contains(t, output.String(), "paused: incident")
	assert.Contains(t, output.String(), "deploy")
	assert.Contains(t, output.String(), "health check failed")
}

func TestHumanLogsUsesServiceFields(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	err := (&command{}).render(&output, client.DeliveryJobLogsResponse{
		Status: "running",
		Lines:  []map[string]any{{"timestamp": "2026-09-10T10:00:00Z", "header": "stdout: ", "line": "deployed"}},
	})
	require.NoError(t, err)
	assert.Contains(t, output.String(), "running")
	assert.Contains(t, output.String(), "stdout: deployed")
}
