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

package do

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyRead(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		read plugin.ReadResponse
		want readVerdict
	}{
		"blank id": {
			read: plugin.ReadResponse{},
			want: readBlankID,
		},
		"blank id wins over present state": {
			read: plugin.ReadResponse{ReadResult: plugin.ReadResult{
				Outputs: resource.PropertyMap{"name": resource.NewProperty("x")},
			}},
			want: readBlankID,
		},
		"nil outputs": {
			read: plugin.ReadResponse{ReadResult: plugin.ReadResult{ID: "res-1"}},
			want: readNilState,
		},
		"echoed id over empty bags": {
			read: plugin.ReadResponse{ReadResult: plugin.ReadResult{
				ID:      "res-1",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
			}},
			want: readEmptyState,
		},
		"echoed id with only an id property": {
			read: plugin.ReadResponse{ReadResult: plugin.ReadResult{
				ID:      "res-1",
				Inputs:  resource.PropertyMap{"id": resource.NewProperty("res-1")},
				Outputs: resource.PropertyMap{"id": resource.NewProperty("res-1")},
			}},
			want: readEmptyState,
		},
		"state in outputs": {
			read: plugin.ReadResponse{ReadResult: plugin.ReadResult{
				ID:      "res-1",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{"name": resource.NewProperty("out")},
			}},
			want: readPresent,
		},
		"state in inputs only": {
			read: plugin.ReadResponse{ReadResult: plugin.ReadResult{
				ID:      "res-1",
				Inputs:  resource.PropertyMap{"name": resource.NewProperty("in")},
				Outputs: resource.PropertyMap{},
			}},
			want: readPresent,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := classifyRead(tt.read)
			assert.Equal(t, tt.want, got)
			// readNotFound must stay a pure restatement of this, or the logs would describe a
			// decision the CLI did not actually make.
			assert.Equal(t, got.missing(), readNotFound(tt.read))
		})
	}
}

// captureHandler collects records so a test can assert on the structured attributes rather than on
// rendered text.
type captureHandler struct {
	mu      sync.Mutex
	records []map[string]any
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	attrs := map[string]any{"msg": r.Message}
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, attrs)
	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(string) slog.Handler      { return h }

// matching returns records whose event and resourceId both match, so concurrent tests sharing the
// default logger cannot contaminate the result.
func (h *captureHandler) matching(event, resourceID string) []map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []map[string]any
	for _, rec := range h.records {
		if rec["event"] == event && rec["resourceId"] == resourceID {
			out = append(out, rec)
		}
	}
	return out
}

// captureLogs installs a capturing default logger for the duration of the test. Not parallel: it
// swaps process-wide state.
func captureLogs(t *testing.T) *captureHandler {
	t.Helper()
	h := &captureHandler{}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return h
}

// TestDoCmdResourceStructuredLogging pins the records an operator keys off in production. The
// attribute names and the event identifiers are the contract here — not the messages.
func TestDoCmdResourceStructuredLogging(t *testing.T) {
	//nolint:paralleltest // installs a process-wide default logger
	t.Run("emptied pre-flight read reports the echoed-id shape", func(t *testing.T) {
		logs := captureLogs(t)
		cmd, _, _ := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(false),
			MockProvider: plugin.MockProvider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					return plugin.ReadResponse{ReadResult: plugin.ReadResult{
						ID:      req.ID,
						Inputs:  resource.PropertyMap{},
						Outputs: resource.PropertyMap{},
					}}, nil
				},
			},
		})
		cmd.SetArgs([]string{"--stateless", "azure:index:myResource", "delete", "log-gone", "--yes"})
		require.Error(t, cmd.Execute())

		verdicts := logs.matching(eventReadVerdict, "log-gone")
		require.Len(t, verdicts, 1)
		assert.Equal(t, "delete", verdicts[0]["operation"])
		assert.Equal(t, "azure:index:myResource", verdicts[0]["resourceType"])
		assert.Equal(t, string(readEmptyState), verdicts[0]["verdict"])
		assert.Equal(t, true, verdicts[0]["missing"])

		// No mutation was attempted, so the re-read path must be silent. Its absence is what makes
		// the event usable as a signal that the race actually happened.
		assert.Empty(t, logs.matching(eventRecheck, "log-gone"))
	})

	//nolint:paralleltest // installs a process-wide default logger
	t.Run("delete losing the race emits a recheck with outcome gone", func(t *testing.T) {
		logs := captureLogs(t)
		var reads int
		cmd, _, _ := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(false),
			MockProvider: plugin.MockProvider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					reads++
					if reads == 1 {
						return plugin.ReadResponse{ReadResult: plugin.ReadResult{
							ID:      req.ID,
							Inputs:  resource.PropertyMap{"name": resource.NewProperty("in")},
							Outputs: resource.PropertyMap{"name": resource.NewProperty("out")},
						}}, nil
					}
					return plugin.ReadResponse{ReadResult: plugin.ReadResult{
						ID:      req.ID,
						Inputs:  resource.PropertyMap{},
						Outputs: resource.PropertyMap{},
					}}, nil
				},
				DeleteF: func(_ context.Context, _ plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					return plugin.DeleteResponse{}, errors.New("api error InvalidPermission.NotFound")
				},
			},
		})
		cmd.SetArgs([]string{"--stateless", "azure:index:myResource", "delete", "log-race", "--yes"})
		require.Error(t, cmd.Execute())

		verdicts := logs.matching(eventReadVerdict, "log-race")
		require.Len(t, verdicts, 1)
		assert.Equal(t, string(readPresent), verdicts[0]["verdict"])

		rechecks := logs.matching(eventRecheck, "log-race")
		require.Len(t, rechecks, 1, "the re-read path should record exactly once")
		assert.Equal(t, "delete", rechecks[0]["operation"])
		assert.Equal(t, "gone", rechecks[0]["outcome"])
		assert.Equal(t, true, rechecks[0]["reclassified"])
		assert.Equal(t, string(readEmptyState), rechecks[0]["verdict"])
	})

	//nolint:paralleltest // installs a process-wide default logger
	t.Run("genuine failure records a recheck that did not reclassify", func(t *testing.T) {
		logs := captureLogs(t)
		cmd, _, _ := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(false),
			MockProvider: plugin.MockProvider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					return plugin.ReadResponse{ReadResult: plugin.ReadResult{
						ID:      req.ID,
						Inputs:  resource.PropertyMap{"name": resource.NewProperty("in")},
						Outputs: resource.PropertyMap{"name": resource.NewProperty("out")},
					}}, nil
				},
				DeleteF: func(_ context.Context, _ plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					return plugin.DeleteResponse{}, errors.New("api error DependencyViolation")
				},
			},
		})
		cmd.SetArgs([]string{"--stateless", "azure:index:myResource", "delete", "log-fail", "--yes"})
		require.Error(t, cmd.Execute())

		rechecks := logs.matching(eventRecheck, "log-fail")
		require.Len(t, rechecks, 1)
		assert.Equal(t, "present", rechecks[0]["outcome"])
		assert.Equal(t, false, rechecks[0]["reclassified"])
	})

	//nolint:paralleltest // installs a process-wide default logger and sets an env var
	t.Run("PULUMI_DO_SKIP_RECHECK sheds the extra call but still records", func(t *testing.T) {
		logs := captureLogs(t)
		t.Setenv("PULUMI_DO_SKIP_RECHECK", "true")
		var reads int
		cmd, _, _ := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(false),
			MockProvider: plugin.MockProvider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					reads++
					return plugin.ReadResponse{ReadResult: plugin.ReadResult{
						ID:      req.ID,
						Inputs:  resource.PropertyMap{"name": resource.NewProperty("in")},
						Outputs: resource.PropertyMap{"name": resource.NewProperty("out")},
					}}, nil
				},
				DeleteF: func(_ context.Context, _ plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					return plugin.DeleteResponse{}, errors.New("api error ThrottlingException")
				},
			},
		})
		cmd.SetArgs([]string{"--stateless", "azure:index:myResource", "delete", "log-skip", "--yes"})
		require.Error(t, cmd.Execute())

		// The whole point of the switch: no second Read.
		assert.Equal(t, 1, reads, "the re-read must not be issued when the switch is set")

		rechecks := logs.matching(eventRecheck, "log-skip")
		require.Len(t, rechecks, 1, "the decision is still recorded, so dashboards do not go quiet")
		assert.Equal(t, "skipped", rechecks[0]["outcome"])
		assert.Equal(t, false, rechecks[0]["reclassified"])
	})

	//nolint:paralleltest // installs a process-wide default logger
	t.Run("a re-read that fails is recorded as such", func(t *testing.T) {
		logs := captureLogs(t)
		var reads int
		cmd, _, _ := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(false),
			MockProvider: plugin.MockProvider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					reads++
					if reads == 1 {
						return plugin.ReadResponse{ReadResult: plugin.ReadResult{
							ID:      req.ID,
							Inputs:  resource.PropertyMap{"name": resource.NewProperty("in")},
							Outputs: resource.PropertyMap{"name": resource.NewProperty("out")},
						}}, nil
					}
					return plugin.ReadResponse{}, errors.New("api error Throttling: rate exceeded")
				},
				DeleteF: func(_ context.Context, _ plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					return plugin.DeleteResponse{}, errors.New("api error DependencyViolation")
				},
			},
		})
		cmd.SetArgs([]string{"--stateless", "azure:index:myResource", "delete", "log-throttle", "--yes"})
		require.Error(t, cmd.Execute())

		rechecks := logs.matching(eventRecheck, "log-throttle")
		require.Len(t, rechecks, 1)
		assert.Equal(t, "read-failed", rechecks[0]["outcome"])
		assert.Equal(t, false, rechecks[0]["reclassified"])
		assert.Contains(t, rechecks[0]["err"], "Throttling")
	})
}
