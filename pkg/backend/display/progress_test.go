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
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func defaultOpts() Options {
	return Options{
		Color:                colors.Raw,
		ShowConfig:           true,
		ShowReplacementSteps: true,
		ShowSameResources:    true,
		ShowReads:            true,
		DeterministicOutput:  true,
		ShowLinkToCopilot:    false,
		RenderOnDirty:        true,
	}
}

func testProgressEvents(
	t testing.TB,
	path string,
	accept bool,
	suffix string,
	testOpts Options,
	width, height int,
	raw bool,
) {
	events, err := loadEvents(path)
	require.NoError(t, err)

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

	opts := testOpts
	opts.Stdout = &stdout
	opts.Stderr = &stderr
	opts.term = terminal.NewMockTerminal(&stdout, width, height, raw)

	go ShowProgressEvents(
		"test", "update", tokens.MustParseStackName("stack"), "project", "link", eventChannel, doneChannel,
		opts, false)

	for _, e := range events {
		eventChannel <- e
	}
	<-doneChannel

	if _, ok := t.(*testing.B); ok {
		// Benchmark mode: don't check the output.
		return
	}

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

//nolint:paralleltest // sets the TERM environment variable
func TestProgressEvents(t *testing.T) {
	t.Setenv("TERM", "vt102")

	accept := cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT"))

	entries, err := os.ReadDir("testdata/not-truncated")
	require.NoError(t, err)

	dimensions := []struct{ width, height int }{
		{width: 80, height: 24},
		{width: 100, height: 80},
		{width: 200, height: 80},
	}

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
						suffix := fmt.Sprintf(".interactive-%vx%v", width, height)
						opts := defaultOpts()
						opts.IsInteractive = true
						testProgressEvents(t, path, accept, suffix, opts, width, height, true)
					})

					t.Run("cooked", func(t *testing.T) {
						suffix := fmt.Sprintf(".interactive-%vx%v-cooked", width, height)
						opts := defaultOpts()
						opts.IsInteractive = true
						testProgressEvents(t, path, accept, suffix, opts, width, height, false)
					})

					t.Run("plain", func(t *testing.T) {
						suffix := fmt.Sprintf(".interactive-%vx%v-plain", width, height)
						opts := defaultOpts()
						opts.ShowResourceChanges = true
						testSimpleRenderer(t, path, accept, suffix, opts, width, height)
					})
				})
			}

			t.Run("no-show-sames", func(t *testing.T) {
				opts := defaultOpts()
				opts.IsInteractive = true
				opts.ShowSameResources = false
				testProgressEvents(t, path, accept, ".interactive-no-show-sames", opts, 80, 24, true)
			})
		})

		t.Run(entry.Name()+"non-interactive", func(t *testing.T) {
			t.Parallel()

			opts := defaultOpts()
			testProgressEvents(t, path, accept, ".non-interactive", opts, 80, 24, false)
		})
	}
}

func sliceToBufferedChan[T any](slice []T) <-chan T {
	ch := make(chan T, len(slice))
	for _, v := range slice {
		ch <- v
	}
	close(ch)
	return ch
}

func TestCaptureProgressEventsCapturesOutput(t *testing.T) {
	t.Parallel()

	// Push some example events
	events := []engine.Event{
		engine.NewEvent(engine.StdoutEventPayload{
			Message: "Hello, world!",
			// Note: System events need their own Color instance
			Color: colors.Never,
		}),
	}
	eventsChannel := sliceToBufferedChan(events)

	captureRenderer := NewCaptureProgressEvents(
		tokens.MustParseStackName("stack"), "project", Options{}, false, apitype.UpdateUpdate)
	captureRenderer.ProcessEvents(eventsChannel, make(chan<- bool))

	assert.False(t, captureRenderer.OutputIncludesFailure())
	assert.Contains(t, captureRenderer.Output(), "Hello, world!")
}

func TestCaptureProgressEventsDetectsAndCapturesFailure(t *testing.T) {
	t.Parallel()

	// If we see a ResourceOperationFailed event, the update is marked as failed.
	resourceOperationFailedEvent := engine.NewEvent(engine.ResourceOperationFailedPayload{
		Metadata: engine.StepEventMetadata{
			URN: "urn:pulumi:dev::eks::pulumi:pulumi:Stack::eks-dev",
			Op:  deploy.OpUpdate,
		},
	})
	// Some diagnostics which is what we're usually interested in.
	diagEvent := engine.NewEvent(engine.DiagEventPayload{
		URN:     "urn:pulumi:dev::eks::pulumi:pulumi:Stack::eks-dev",
		Message: "Failed to update",
	})
	failureEvents := []engine.Event{resourceOperationFailedEvent, diagEvent}
	eventsChannel := sliceToBufferedChan(failureEvents)

	captureRenderer := NewCaptureProgressEvents(
		tokens.MustParseStackName("stack"), "project", Options{}, false, apitype.UpdateUpdate)
	captureRenderer.ProcessEvents(eventsChannel, make(chan<- bool))

	assert.True(t, captureRenderer.OutputIncludesFailure())
	assert.Contains(t, captureRenderer.Output(), "Failed to update")
}

func TestCaptureProgressEventsDetectsAndCapturesFailurePreview(t *testing.T) {
	t.Parallel()

	diagEventWithErrors := engine.NewEvent(engine.DiagEventPayload{
		URN:      "urn:pulumi:dev::eks::pulumi:pulumi:Stack::eks-dev",
		Message:  "Failed to update",
		Severity: diag.Error,
	})
	failureEvents := []engine.Event{diagEventWithErrors}
	eventsChannel := sliceToBufferedChan(failureEvents)

	captureRenderer := NewCaptureProgressEvents(
		tokens.MustParseStackName("stack"), "project", Options{}, true, apitype.PreviewUpdate)
	captureRenderer.ProcessEvents(eventsChannel, make(chan<- bool))

	assert.True(t, captureRenderer.OutputIncludesFailure())
	assert.Contains(t, captureRenderer.Output(), "Failed to update")
}

func BenchmarkProgressEvents(t *testing.B) {
	t.Setenv("TERM", "vt102")
	t.ReportAllocs()

	entries, err := os.ReadDir("testdata/not-truncated")
	require.NoError(t, err)

	dimensions := []struct{ width, height int }{
		{width: 80, height: 24},
		{width: 100, height: 80},
		{width: 200, height: 80},
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join("testdata/not-truncated", entry.Name())

		t.Run(entry.Name()+"interactive", func(t *testing.B) {
			for _, dim := range dimensions {
				width, height := dim.width, dim.height
				t.Run(fmt.Sprintf("%vx%v", width, height), func(t *testing.B) {
					t.Run("raw", func(t *testing.B) {
						for i := 0; i < t.N; i++ {
							suffix := fmt.Sprintf(".interactive-%vx%v", width, height)
							opts := defaultOpts()
							opts.IsInteractive = true
							testProgressEvents(t, path, false, suffix, opts, width, height, true)
						}
					})

					t.Run("cooked", func(t *testing.B) {
						for i := 0; i < t.N; i++ {
							suffix := fmt.Sprintf(".interactive-%vx%v-cooked", width, height)
							opts := defaultOpts()
							opts.IsInteractive = true
							testProgressEvents(t, path, false, suffix, opts, width, height, false)
						}
					})

					t.Run("plain", func(t *testing.B) {
						for i := 0; i < t.N; i++ {
							suffix := fmt.Sprintf(".interactive-%vx%v-plain", width, height)
							opts := defaultOpts()
							opts.ShowResourceChanges = true
							testSimpleRenderer(t, path, false, suffix, opts, width, height)
						}
					})
				})
			}
		})

		t.Run(entry.Name()+"non-interactive", func(t *testing.B) {
			for i := 0; i < t.N; i++ {
				opts := defaultOpts()
				testProgressEvents(t, path, false, ".non-interactive", opts, 80, 24, false)
			}
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
					RetainOnDelete: true,
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

func TestProgressPolicyPacks(t *testing.T) {
	t.Parallel()
	eventChannel, doneChannel := make(chan engine.Event), make(chan bool)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	go ShowProgressEvents(
		"test", "update", tokens.MustParseStackName("stack"), "project", "link", eventChannel, doneChannel,
		Options{
			IsInteractive:        true,
			Color:                colors.Raw,
			ShowConfig:           true,
			ShowReplacementSteps: true,
			ShowSameResources:    true,
			ShowReads:            true,
			Stdout:               &stdout,
			Stderr:               &stderr,
			term:                 terminal.NewMockTerminal(&stdout, 80, 24, true),
			DeterministicOutput:  true,
		}, false)

	// Send policy pack event to the channel
	eventChannel <- engine.NewEvent(engine.PolicyLoadEventPayload{})
	close(eventChannel)
	<-doneChannel

	assert.Contains(t, stdout.String(), "Loading policy packs...")
}

func testSimpleRenderer(
	t testing.TB,
	path string,
	accept bool,
	suffix string,
	testOpts Options,
	width, height int,
) {
	events, err := loadEvents(path)
	require.NoError(t, err)

	fileName := path + suffix + ".txt"
	var expectedStdout []byte
	if !accept {
		expectedStdout, err = os.ReadFile(fileName)
		require.NoError(t, err)
	}

	eventChannel, doneChannel := make(chan engine.Event), make(chan bool)

	var stdout bytes.Buffer

	opts := testOpts
	opts.Stdout = &stdout

	go RenderProgressEvents(
		"test",
		"update",
		tokens.MustParseStackName("stack"),
		"project",
		"link",
		eventChannel,
		doneChannel,
		opts,
		false,
		width,
		height,
	)

	for _, e := range events {
		eventChannel <- e
	}
	<-doneChannel

	if _, ok := t.(*testing.B); ok {
		// Benchmark mode: don't check the output.
		return
	}

	if !accept {
		assert.Equal(t, string(expectedStdout), stdout.String())
	} else {
		err = os.WriteFile(fileName, stdout.Bytes(), 0o600)
		require.NoError(t, err)
	}
}
