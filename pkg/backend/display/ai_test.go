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
		Stdout:            buf,
		Color:             colors.Never,
		ShowLinkToCopilot: true,
	}

	// Render to buffer
	RenderCopilotErrorSummary(&CopilotErrorSummaryMetadata{
		Summary: summary,
	}, nil, opts, "http://foo.bar/baz")

	expectedCopilotSummary := fmt.Sprintf(`Copilot Diagnostics%s
  This is a test summary

  Would you like additional help with this update?
  http://foo.bar/baz?explainFailure

`, copilotDelimiterEmoji())
	assert.Equal(t, expectedCopilotSummary, buf.String())
}

func TestRenderCopilotErrorSummaryNoLink(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	opts := Options{
		Stdout:            buf,
		Color:             colors.Never,
		ShowLinkToCopilot: false,
	}

	RenderCopilotErrorSummary(&CopilotErrorSummaryMetadata{
		Summary: "This is a test summary",
	}, nil, opts, "http://foo.bar/baz")

	expectedCopilotSummary := fmt.Sprintf(`Copilot Diagnostics%s
  This is a test summary

`, copilotDelimiterEmoji())
	assert.Equal(t, expectedCopilotSummary, buf.String())
}

func TestRenderCopilotErrorSummaryError(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	opts := Options{
		Stdout: buf,
		Color:  colors.Never,
	}

	RenderCopilotErrorSummary(nil, errors.New("test error"), opts, "http://foo.bar/baz")

	expectedCopilotSummaryWithError := fmt.Sprintf(`Copilot Diagnostics%s
  error summarizing update output: test error

`, copilotDelimiterEmoji())
	assert.Equal(t, expectedCopilotSummaryWithError, buf.String())
}

func TestRenderCopilotErrorSummaryNoSummaryOrError(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	opts := Options{
		Stdout: buf,
		Color:  colors.Never,
	}

	RenderCopilotErrorSummary(nil, nil, opts, "http://foo.bar/baz")

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

	RenderCopilotErrorSummary(&CopilotErrorSummaryMetadata{
		Summary: summary,
	}, errors.New("test error"), opts, "http://foo.bar/baz")

	expectedCopilotSummaryWithErrorAndSummary := fmt.Sprintf(`Copilot Diagnostics%s
  error summarizing update output: test error

`, copilotDelimiterEmoji())
	assert.Equal(t, expectedCopilotSummaryWithErrorAndSummary, buf.String())
}
