// Copyright 2025, Pulumi Corporation.
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

package display

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/stretchr/testify/assert"
)

func TestRenderCopilotErrorSummary(t *testing.T) {
	t.Parallel()

	summary := "This is a test summary"
	buf := new(bytes.Buffer)
	opts := Options{
		Stdout:        buf,
		Color:         colors.Never,
		ShowLinkToNeo: true,
	}

	// Render to buffer
	RenderNeoErrorSummary(&NeoErrorSummaryMetadata{
		Summary: summary,
	}, nil, opts, "http://foo.bar/baz")

	expectedCopilotSummary := fmt.Sprintf(`Neo Diagnostics%s
  This is a test summary

  Would you like additional help with this update?
  http://foo.bar/baz?explainFailure

`, neoDelimiterEmoji())
	assert.Equal(t, expectedCopilotSummary, buf.String())
}

func TestRenderCopilotErrorSummaryError(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	opts := Options{
		Stdout: buf,
		Color:  colors.Never,
	}

	RenderNeoErrorSummary(nil, errors.New("test error"), opts, "http://foo.bar/baz")

	expectedCopilotSummaryWithError := fmt.Sprintf(`Neo Diagnostics%s
  error summarizing update output: test error

`, neoDelimiterEmoji())
	assert.Equal(t, expectedCopilotSummaryWithError, buf.String())
}

func TestRenderCopilotErrorSummaryNoSummaryOrError(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	opts := Options{
		Stdout: buf,
		Color:  colors.Never,
	}

	RenderNeoErrorSummary(nil, nil, opts, "http://foo.bar/baz")

	assert.Equal(t, "", buf.String())
}

// Edge case, just make sure we're handling this gracefully.
func TestRenderCopilotErrorSummaryWithError(t *testing.T) {
	t.Parallel()

	summary := "This is a test summary"
	buf := new(bytes.Buffer)
	opts := Options{
		Stdout: buf,
		Color:  colors.Never,
	}

	RenderNeoErrorSummary(&NeoErrorSummaryMetadata{
		Summary: summary,
	}, errors.New("test error"), opts, "http://foo.bar/baz")

	expectedCopilotSummaryWithErrorAndSummary := fmt.Sprintf(`Neo Diagnostics%s
  error summarizing update output: test error

`, neoDelimiterEmoji())
	assert.Equal(t, expectedCopilotSummaryWithErrorAndSummary, buf.String())
}

func TestRenderBoldMarkdown(t *testing.T) {
	t.Parallel()

	summary := `**This** is a test **summary**
**Resource** has been **created**`

	highlightColor := colors.BrightBlue

	expectedSummary := highlightColor + "This" + colors.Reset + " is a test " + highlightColor + "summary" + colors.Reset +
		"\n" +
		highlightColor + "Resource" + colors.Reset + " has been " + highlightColor + "created" + colors.Reset
	formattedSummary := renderBoldMarkdown(summary, Options{Color: colors.Always})
	assert.Equal(t, expectedSummary, formattedSummary)
}

func TestRenderBoldMarkdownNever(t *testing.T) {
	t.Parallel()

	summary := `This is a test summary
Resource has been created`

	expectedSummary := "This is a test summary\nResource has been created"
	formattedSummary := renderBoldMarkdown(summary, Options{Color: colors.Never})
	assert.Equal(t, expectedSummary, formattedSummary)
}

// mockNeoTaskResult implements NeoTaskResult for testing
type mockNeoTaskResult struct {
	taskID string
}

func (m *mockNeoTaskResult) GetTaskID() string {
	return m.taskID
}

func TestRenderNeoTaskCreated(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	opts := Options{
		Stdout: buf,
		Color:  colors.Never,
	}

	taskResult := &mockNeoTaskResult{taskID: "task_abc123"}
	RenderNeoTaskCreated(taskResult, nil, "https://app.pulumi.com", "test-org", opts)

	expected := fmt.Sprintf(`
Neo Task Created%s
  A Neo task has been started to help debug this error.
  https://app.pulumi.com/test-org/neo/tasks/task_abc123

`, neoDelimiterEmoji())
	assert.Equal(t, expected, buf.String())
}

func TestRenderNeoTaskCreatedError(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	opts := Options{
		Stdout: buf,
		Color:  colors.Never,
	}

	RenderNeoTaskCreated(nil, errors.New("failed to create task"), "https://app.pulumi.com", "test-org", opts)

	expected := fmt.Sprintf(`
Neo Task%s
  error creating Neo task: failed to create task

`, neoDelimiterEmoji())
	assert.Equal(t, expected, buf.String())
}

func TestRenderNeoTaskCreatedNilResult(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	opts := Options{
		Stdout: buf,
		Color:  colors.Never,
	}

	RenderNeoTaskCreated(nil, nil, "https://app.pulumi.com", "test-org", opts)
	assert.Equal(t, "", buf.String())
}

func TestRenderNeoTaskCreatedEmptyTaskID(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	opts := Options{
		Stdout: buf,
		Color:  colors.Never,
	}

	taskResult := &mockNeoTaskResult{taskID: ""}
	RenderNeoTaskCreated(taskResult, nil, "https://app.pulumi.com", "test-org", opts)
	assert.Equal(t, "", buf.String())
}
