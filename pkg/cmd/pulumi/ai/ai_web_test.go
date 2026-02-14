// Copyright 2016-2026, Pulumi Corporation.
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

package ai

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAIWebCommand_OpensNeoWithoutPrompt(t *testing.T) {
	t.Parallel()
	var stdout bytes.Buffer
	var capturedURL string
	var taskCreated bool

	cmd := &aiWebCmd{
		Stdout: &stdout,
		ws:     pkgWorkspace.Instance,
		currentBackend: func(
			context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
		) (backend.Backend, error) {
			return &httpstate.MockHTTPBackend{
				MockBackend: backend.MockBackend{
					GetDefaultOrgF: func(ctx context.Context) (string, error) {
						return "test-org", nil
					},
				},
				FCloudConsoleURL: func(paths ...string) string {
					return "https://app.pulumi.com/" + strings.Join(paths, "/")
				},
				FCreateNeoTask: func(ctx context.Context, stackRef backend.StackReference, prompt string) (string, error) {
					taskCreated = true
					return "https://app.pulumi.com/test-org/neo/tasks/task-123", nil
				},
			}, nil
		},
		openBrowser: func(url string) error {
			capturedURL = url
			return nil
		},
	}

	// Should not error - just opens the website
	err := cmd.Run(context.Background(), []string{})
	require.NoError(t, err)
	assert.Equal(t, "https://app.pulumi.com/test-org/neo/tasks", capturedURL)
	assert.False(t, taskCreated)
}

func TestAIWebCommand_NoAutoSubmitWithPrompt(t *testing.T) {
	t.Parallel()
	var stdout bytes.Buffer
	var capturedURL string
	var taskCreated bool

	cmd := &aiWebCmd{
		Stdout:            &stdout,
		ws:                pkgWorkspace.Instance,
		disableAutoSubmit: true,
		currentBackend: func(
			context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
		) (backend.Backend, error) {
			return &httpstate.MockHTTPBackend{
				MockBackend: backend.MockBackend{
					GetDefaultOrgF: func(ctx context.Context) (string, error) {
						return "test-org", nil
					},
				},
				FCloudConsoleURL: func(paths ...string) string {
					return "https://app.pulumi.com/" + strings.Join(paths, "/")
				},
				FCreateNeoTask: func(ctx context.Context, stackRef backend.StackReference, prompt string) (string, error) {
					taskCreated = true
					return "https://app.pulumi.com/test-org/neo/tasks/task-123", nil
				},
			}, nil
		},
		openBrowser: func(url string) error {
			capturedURL = url
			return nil
		},
	}

	// Should open with prompt in query string
	err := cmd.Run(context.Background(), []string{"Help me build"})
	require.NoError(t, err)
	assert.Contains(t, capturedURL, "prompt=Help+me+build")
	assert.False(t, taskCreated)
}

func TestAIWebCommand_RequiresCloudBackend(t *testing.T) {
	t.Parallel()
	var stdout bytes.Buffer
	cmd := &aiWebCmd{
		Stdout: &stdout,
		ws:     pkgWorkspace.Instance,
		currentBackend: func(
			context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
		) (backend.Backend, error) {
			// Return a non-cloud backend
			return &mockNonCloudBackend{}, nil
		},
	}

	err := cmd.Run(context.Background(), []string{"Create an S3 bucket"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Neo tasks are only available with the Pulumi Cloud backend")
}

func TestAIWebCommand_CreatesNeoTask(t *testing.T) {
	t.Parallel()
	var stdout bytes.Buffer
	expectedURL := "https://app.pulumi.com/test-org/neo/tasks/task-123"
	var capturedPrompt string
	var taskCreated bool

	var capturedURL string
	cmd := &aiWebCmd{
		Stdout: &stdout,
		ws:     pkgWorkspace.Instance,
		currentBackend: func(
			context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
		) (backend.Backend, error) {
			return &httpstate.MockHTTPBackend{
				MockBackend: backend.MockBackend{
					GetDefaultOrgF: func(ctx context.Context) (string, error) {
						return "test-org", nil
					},
				},
				FCreateNeoTask: func(ctx context.Context, stackRef backend.StackReference, prompt string) (string, error) {
					capturedPrompt = prompt
					taskCreated = true
					return expectedURL, nil
				},
			}, nil
		},
		openBrowser: func(url string) error {
			capturedURL = url
			return nil
		},
	}

	err := cmd.Run(context.Background(), []string{"Create an S3 bucket"})
	require.NoError(t, err)

	assert.Equal(t, "Create an S3 bucket", capturedPrompt)
	assert.Contains(t, stdout.String(), "Pulumi Neo task created successfully!")
	assert.Contains(t, stdout.String(), expectedURL)
	assert.Equal(t, expectedURL, capturedURL)
	assert.True(t, taskCreated)
}

func TestAIWebCommand_AppendsLanguageToPrompt(t *testing.T) {
	t.Parallel()
	var stdout bytes.Buffer
	var capturedPrompt string
	var taskCreated bool
	cmd := &aiWebCmd{
		Stdout:   &stdout,
		ws:       pkgWorkspace.Instance,
		language: Python,
		currentBackend: func(
			context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
		) (backend.Backend, error) {
			return &httpstate.MockHTTPBackend{
				MockBackend: backend.MockBackend{
					GetDefaultOrgF: func(ctx context.Context) (string, error) {
						return "test-org", nil
					},
				},
				FCreateNeoTask: func(ctx context.Context, stackRef backend.StackReference, prompt string) (string, error) {
					capturedPrompt = prompt
					taskCreated = true
					return "https://app.pulumi.com/test-org/neo/tasks/task-123", nil
				},
			}, nil
		},
		openBrowser: func(url string) error {
			return nil
		},
	}

	err := cmd.Run(context.Background(), []string{"Create an S3 bucket"})
	require.NoError(t, err)

	assert.Equal(t, capturedPrompt, "Create an S3 bucket\n\nPlease use Python.")
	assert.True(t, taskCreated)
}

// Mock types for testing
type mockNonCloudBackend struct {
	backend.Backend
}
