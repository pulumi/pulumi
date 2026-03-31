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

package ints

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/stretchr/testify/require"

	ui "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
)

type scopedWriter struct {
	buf    *bytes.Buffer
	active bool
}

func (w *scopedWriter) Write(p []byte) (n int, err error) {
	if !w.active {
		return len(p), nil
	}
	return w.buf.Write(p)
}

func (w *scopedWriter) SetActive(active bool) {
	w.active = active
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestUp_JSONSummaryFooter(t *testing.T) {
	// Avoid capturing output from commands other than `pulumi up`.
	var upOut bytes.Buffer
	writer := &scopedWriter{buf: &upOut}

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:                    "single_resource",
		Dependencies:           []string{"@pulumi/pulumi"},
		Quick:                  true,
		Verbose:                true,
		Stdout:                 writer,
		Stderr:                 writer,
		UpdateCommandlineFlags: []string{"--format", "json"},
		PrePulumiCommand: func(verb string) (func(err error) error, error) {
			if verb != "up" {
				return nil, nil
			}

			writer.SetActive(true)
			return func(err error) error {
				writer.SetActive(false)
				return nil
			}, nil
		},
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// Smoke sanity: verify the program actually ran and produced a deployment.
			require.NotNil(t, stackInfo.Deployment)
		},
	})

	// Find the JSON summary object inside the captured `pulumi up --format json` output.
	// The output is JSONL (one JSON object per line) and we emit the summary as a single final line.
	for line := range strings.SplitSeq(upOut.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var summary ui.OperationSummaryJSON
		if err := json.Unmarshal([]byte(line), &summary); err != nil {
			continue
		}
		if summary.Result != "" && len(summary.ChangeSummary) > 0 && summary.Duration != 0 {
			require.Equal(t, ui.OperationResultSucceeded, summary.Result)
			require.NotEmpty(t, summary.ChangeSummary)
			require.NotZero(t, summary.Duration)
			return
		}
	}

	t.Fatal("expected to find operation summary JSON in pulumi up output")
}

// Ensure our scopedWriter implements io.Writer.
var _ io.Writer = (*scopedWriter)(nil)
