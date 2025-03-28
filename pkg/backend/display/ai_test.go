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
	elapsedMs := int64(100)
	buf := new(bytes.Buffer)
	opts := Options{
		Stdout: buf,
		Color:  colors.Never,
	}

	// Render to buffer
	RenderCopilotErrorSummary(&CopilotErrorSummaryMetadata{
		Summary:   summary,
		ElapsedMs: elapsedMs,
	}, nil, opts)

	expectedCopilotSummary := fmt.Sprintf(`AI-generated summary%s: 100ms
  This is a test summary

`, copilotEmojiOr())
	assert.Equal(t, expectedCopilotSummary, buf.String())
}

func TestRenderCopilotErrorSummaryError(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	opts := Options{
		Stdout: buf,
		Color:  colors.Never,
	}

	RenderCopilotErrorSummary(nil, errors.New("test error"), opts)

	expectedCopilotSummaryWithError := fmt.Sprintf(`AI-generated summary%s:
  error summarizing update output: test error

`, copilotEmojiOr())
	assert.Equal(t, expectedCopilotSummaryWithError, buf.String())
}

func TestRenderCopilotErrorSummaryNoSummaryOrError(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	opts := Options{
		Stdout: buf,
		Color:  colors.Never,
	}

	RenderCopilotErrorSummary(nil, nil, opts)

	assert.Equal(t, "", buf.String())
}

// Edge case, just make sure we're handling this gracefully.
func TestRenderCopilotErrorSummaryWithError(t *testing.T) {
	t.Parallel()

	summary := "This is a test summary"
	elapsedMs := int64(100)
	buf := new(bytes.Buffer)
	opts := Options{
		Stdout: buf,
		Color:  colors.Never,
	}

	RenderCopilotErrorSummary(&CopilotErrorSummaryMetadata{
		Summary:   summary,
		ElapsedMs: elapsedMs,
	}, errors.New("test error"), opts)

	expectedCopilotSummaryWithErrorAndSummary := fmt.Sprintf(`AI-generated summary%s: 100ms
  error summarizing update output: test error

`, copilotEmojiOr())
	assert.Equal(t, expectedCopilotSummaryWithErrorAndSummary, buf.String())
}
