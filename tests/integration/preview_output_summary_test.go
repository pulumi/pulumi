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
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestPreview_OutputJSONSummary(t *testing.T) {
	// Capture only `pulumi preview`'s stdout. Like the symmetric up test, we
	// expect it to consist solely of the summary JSON object.
	//
	// `pulumi preview` is only invoked explicitly when `SkipPreview` is false
	// (i.e. not `Quick: true`). The internal preview that `pulumi up` runs is
	// part of the `up` invocation, not a separate `preview` verb. We therefore
	// run a minimal lifecycle that includes the explicit preview step.
	var previewStdout bytes.Buffer
	writer := &scopedWriter{buf: &previewStdout}

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:                     "single_resource",
		Dependencies:            []string{"@pulumi/pulumi"},
		SkipExportImport:        true,
		SkipEmptyPreviewUpdate:  true,
		SkipRefresh:             true,
		Verbose:                 true,
		Stdout:                  writer,
		Stderr:                  io.Discard, // human-readable progress goes here; we don't assert on it
		PreviewCommandlineFlags: []string{"--output", "json"},
		PrePulumiCommand: func(verb string) (func(err error) error, error) {
			if verb != "preview" {
				return nil, nil
			}

			writer.SetActive(true)
			return func(err error) error {
				writer.SetActive(false)
				return nil
			}, nil
		},
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// Smoke check: the program actually ran and produced a deployment.
			require.NotNil(t, stackInfo.Deployment)
		},
	})

	// The whole of stdout should be a single JSON object. If parsing fails or
	// fields are missing, the report includes the captured stdout to make
	// debugging easy.
	raw := bytes.TrimSpace(previewStdout.Bytes())

	var summary display.SummaryJSON
	require.NoErrorf(t, json.Unmarshal(raw, &summary),
		"expected stdout to be exactly one JSON summary object, got:\n%s", raw)

	require.Equal(t, apitype.OperationResultSucceeded, summary.Result)
	require.NotEmpty(t, summary.Summary, "summary should record the resource changes")
	require.NotEmpty(t, summary.Resources, "summary should list the affected resources")
	for i, r := range summary.Resources {
		require.NotEmptyf(t, r.URN, "resource %d should have a URN", i)
		require.NotEmptyf(t, r.Type, "resource %d should have a type", i)
		require.NotEmptyf(t, r.Name, "resource %d should have a name", i)
		require.NotEmptyf(t, r.Op, "resource %d should have an op", i)
		// `same` resources are filtered out by default, so every entry should be
		// an actual planned change. Preview against an empty stack is all creates.
		require.NotEqualf(t, apitype.OpSame, r.Op, "resource %d should not be a same", i)
	}
}
