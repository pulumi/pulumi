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
func TestRefresh_OutputJSONSummary(t *testing.T) {
	// Capture only `pulumi refresh`'s stdout. Like the symmetric up/destroy
	// tests, we expect it to consist solely of the summary JSON object.
	var refreshStdout bytes.Buffer
	writer := &scopedWriter{buf: &refreshStdout}

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:                     "single_resource",
		Dependencies:            []string{"@pulumi/pulumi"},
		Quick:                   true,
		Verbose:                 true,
		Stdout:                  writer,
		Stderr:                  io.Discard, // human-readable progress goes here; we don't assert on it
		RefreshCommandlineFlags: []string{"--output", "json"},
		PrePulumiCommand: func(verb string) (func(err error) error, error) {
			if verb != "refresh" {
				return nil, nil
			}

			writer.SetActive(true)
			return func(err error) error {
				writer.SetActive(false)
				return nil
			}, nil
		},
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// Smoke check: the program actually ran and produced a deployment to refresh.
			require.NotNil(t, stackInfo.Deployment)
		},
	})

	// The whole of stdout should be a single JSON object. If parsing fails or
	// fields are missing, the report includes the captured stdout to make
	// debugging easy.
	raw := bytes.TrimSpace(refreshStdout.Bytes())

	var summary display.SummaryJSON
	require.NoErrorf(t, json.Unmarshal(raw, &summary),
		"expected stdout to be exactly one JSON summary object, got:\n%s", raw)

	require.Equal(t, apitype.OperationResultSucceeded, summary.Result)
	// Refresh emits a ResourcePreEvent with Op=refresh for every resource it
	// inspects (the per-resource ResultOp — same/update — is a separate
	// post-step computation that doesn't surface here). So Resources is
	// populated even when nothing actually changes.
	require.NotEmpty(t, summary.Resources, "summary should list the refreshed resources")
	for i, r := range summary.Resources {
		require.NotEmptyf(t, r.URN, "resource %d should have a URN", i)
		require.NotEmptyf(t, r.Type, "resource %d should have a type", i)
		require.NotEmptyf(t, r.Name, "resource %d should have a name", i)
		require.Equalf(t, apitype.OpRefresh, r.Op, "resource %d should be a refresh", i)
	}
}
