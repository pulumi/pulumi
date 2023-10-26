// Copyright 2016-2023, Pulumi Corporation.
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

// Note: to regenerate the baselines for these tests, run `go test` with `PULUMI_ACCEPT=true`.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/display/internal/terminal"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testProgressEvents(t *testing.T, path string, accept, interactive bool, width, height int, raw bool) {
	events, err := loadEvents(path)
	require.NoError(t, err)

	suffix := ".non-interactive"
	if interactive {
		suffix = fmt.Sprintf(".interactive-%vx%v", width, height)
		if !raw {
			suffix += "-cooked"
		}
	}

	var expectedStdout []byte
	var expectedStderr []byte
	if !accept {
		expectedStdout, err = os.ReadFile(path + suffix + ".stdout.txt")
		require.NoError(t, err)

		expectedStderr, err = os.ReadFile(path + suffix + ".stderr.txt")
		require.NoError(t, err)
	}

	eventChannel, doneChannel := make(chan engine.Event), make(chan bool)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	go ShowProgressEvents(
		"test", "update", tokens.MustParseStackName("stack"), "project", "link", eventChannel, doneChannel,
		Options{
			IsInteractive:        interactive,
			Color:                colors.Raw,
			ShowConfig:           true,
			ShowReplacementSteps: true,
			ShowSameResources:    true,
			ShowReads:            true,
			Stdout:               &stdout,
			Stderr:               &stderr,
			term:                 terminal.NewMockTerminal(&stdout, width, height, true),
			deterministicOutput:  true,
		}, false)

	for _, e := range events {
		eventChannel <- e
	}
	<-doneChannel

	if !accept {
		assert.Equal(t, string(expectedStdout), stdout.String())
		assert.Equal(t, string(expectedStderr), stderr.String())
	} else {
		err = os.WriteFile(path+suffix+".stdout.txt", stdout.Bytes(), 0o600)
		require.NoError(t, err)

		err = os.WriteFile(path+suffix+".stderr.txt", stderr.Bytes(), 0o600)
		require.NoError(t, err)
	}
}

func TestProgressEvents(t *testing.T) {
	t.Parallel()

	accept := cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT"))

	entries, err := os.ReadDir("testdata/not-truncated")
	require.NoError(t, err)

	dimensions := []struct{ width, height int }{
		{width: 80, height: 24},
		{width: 100, height: 80},
		{width: 200, height: 80},
	}

	//nolint:paralleltest
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join("testdata/not-truncated", entry.Name())

		t.Run(entry.Name()+"interactive", func(t *testing.T) {
			t.Parallel()

			for _, dim := range dimensions {
				width, height := dim.width, dim.height
				t.Run(fmt.Sprintf("%vx%v", width, height), func(t *testing.T) {
					t.Parallel()

					t.Run("raw", func(t *testing.T) {
						testProgressEvents(t, path, accept, true, width, height, true)
					})

					t.Run("cooked", func(t *testing.T) {
						testProgressEvents(t, path, accept, true, width, height, false)
					})
				})
			}
		})

		t.Run(entry.Name()+"non-interactive", func(t *testing.T) {
			t.Parallel()

			testProgressEvents(t, path, accept, false, 80, 24, false)
		})
	}
}

// The following test checks that the status display elements have retain on delete details added.
func TestStatusDisplayFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		stepOp       display.StepOp
		shouldRetain bool
	}{
		// Should display `retain`.
		{"delete", deploy.OpDelete, true},
		{"replace", deploy.OpReplace, true},
		{"create-replacement", deploy.OpCreateReplacement, true},
		{"delete-replaced", deploy.OpDeleteReplaced, true},

		// Should be unaffected.
		{"same", deploy.OpSame, false},
		{"create", deploy.OpCreate, false},
		{"update", deploy.OpUpdate, false},
		{"read", deploy.OpRead, false},
		{"read-replacement", deploy.OpReadReplacement, false},
		{"refresh", deploy.OpRefresh, false},
		{"discard", deploy.OpReadDiscard, false},
		{"discard-replaced", deploy.OpDiscardReplaced, false},
		{"import", deploy.OpImport, false},
		{"import-replacement", deploy.OpImportReplacement, false},

		// "remove-pending-replace" is not a valid step operation.
		// {"remove-pending-replace", deploy.OpRemovePendingReplace, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := &ProgressDisplay{}
			name := resource.NewURN("test", "test", "test", "test", "test")

			step := engine.StepEventMetadata{
				URN: name,
				Op:  tt.stepOp,
				Old: &engine.StepEventStateMetadata{
					State: &resource.State{
						RetainOnDelete: true,
					},
				},
			}

			doneStatus := d.getStepStatus(step,
				true,  // done
				false, // failed
			)
			inProgressStatus := d.getStepStatus(step,
				false, // done
				false, // failed
			)
			if tt.shouldRetain {
				assert.Contains(t, doneStatus, "[retain]", "%s should contain [retain] (done)", step.Op)
				assert.Contains(t, inProgressStatus, "[retain]", "%s should contain [retain] (in-progress)", step.Op)
			} else {
				assert.NotContains(t, doneStatus, "[retain]", "%s should NOT contain [retain] (done)", step.Op)
				assert.NotContains(t, inProgressStatus, "[retain]", "%s should NOT contain [retain] (in-progress)", step.Op)
			}
		})
	}
}

func TestPrintDiagnosticsIsTolerantOfDiagnostics(t *testing.T) {
	t.Parallel()
	makeDisplayWithDiagnostic := func(sev diag.Severity) *ProgressDisplay {
		return &ProgressDisplay{
			eventUrnToResourceRow: map[resource.URN]ResourceRow{
				"urn:pulumi:test::test::pulumi:pulumi:Stack::test": &resourceRowData{
					diagInfo: &DiagInfo{
						StreamIDToDiagPayloads: map[int32][]engine.DiagEventPayload{
							0: {
								{
									Severity: sev,
								},
							},
						},
					},
				},
			},
		}
	}

	tests := []struct {
		name string
		give diag.Severity
		want bool
	}{
		{"info", diag.Info, false},
		{"warning", diag.Warning, false},
		{"error", diag.Error, true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := makeDisplayWithDiagnostic(tt.give)
			got := d.printDiagnostics()
			assert.Equal(t, tt.want, got, "printDiagnostics(%v) = %v, want %v", tt.give, got, tt.want)
		})
	}
}
