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

package hints

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

var (
	hintA = Hint{ID: "a", Cooldown: 7 * 24 * time.Hour, Message: "hint a"}
	hintB = Hint{ID: "b", Cooldown: 7 * 24 * time.Hour, Message: "hint b"}
)

type testManager struct {
	*manager
	now      time.Time
	disabled bool
	skip     []string
	local    bool
	path     string
	// resources is what stackResources returns for any stack, and resourcesErr the error it returns.
	resources    int
	resourcesErr error
}

func newTestManager(t *testing.T) *testManager {
	tm := &testManager{
		now:  time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC),
		path: filepath.Join(t.TempDir(), stateFileName),
	}
	tm.manager = &manager{
		now:       func() time.Time { return tm.now },
		statePath: func() (string, error) { return tm.path, nil },
		settings: func() settings {
			s := settings{disabled: tm.disabled, skip: map[string]bool{}}
			for _, id := range tm.skip {
				s.skip[id] = true
			}
			return s
		},
		isCloud: func() bool { return !tm.local },
		stackResources: func(backend.Stack) (int, error) {
			return tm.resources, tm.resourcesErr
		},
	}
	return tm
}

// run offers hs and flushes them, returning what was printed.
func (tm *testManager) run(hs ...Hint) string {
	for _, h := range hs {
		tm.offer(h)
	}
	var buf bytes.Buffer
	tm.flush(&buf)
	return buf.String()
}

func TestMultiLineHint(t *testing.T) {
	t.Parallel()
	tm := newTestManager(t)

	// Later lines line up with the first line, after the "Hint: " label.
	out := tm.run(Hint{ID: "multi", Message: "first line\nLearn more: https://example.com"})
	assert.Contains(t, out, "Hint: first line\n      Learn more: https://example.com\n")
}

func TestOneHintPerCommand(t *testing.T) {
	t.Parallel()
	tm := newTestManager(t)

	out := tm.run(hintA, hintB)
	assert.Contains(t, out, "hint a")
	assert.NotContains(t, out, "hint b")

	// Each hint has its own cooldown, so hint b can be shown right after hint a.
	assert.Contains(t, tm.run(hintB), "hint b")
}

func TestPerHintCooldown(t *testing.T) {
	t.Parallel()
	tm := newTestManager(t)

	assert.Contains(t, tm.run(hintA), "hint a")

	// Within hint a's cooldown, so hint b is picked instead.
	tm.now = tm.now.Add(2 * 24 * time.Hour)
	out := tm.run(hintA, hintB)
	assert.NotContains(t, out, "hint a")
	assert.Contains(t, out, "hint b")

	tm.now = tm.now.Add(2 * 24 * time.Hour)
	assert.Empty(t, tm.run(hintA))

	tm.now = tm.now.Add(3 * 24 * time.Hour)
	assert.Contains(t, tm.run(hintA), "hint a")
}

func TestSkippedHint(t *testing.T) {
	t.Parallel()
	tm := newTestManager(t)
	tm.skip = []string{"a"}

	out := tm.run(hintA, hintB)
	assert.NotContains(t, out, "hint a")
	assert.Contains(t, out, "hint b")

	tm.now = tm.now.Add(30 * 24 * time.Hour)
	assert.Empty(t, tm.run(hintA))
}

func TestFlushResetsSuppress(t *testing.T) {
	t.Parallel()
	tm := newTestManager(t)

	tm.suppress()
	assert.Empty(t, tm.run(hintA))

	// The next command in the same process isn't suppressed.
	assert.Contains(t, tm.run(hintA), "hint a")
}

func TestCloudOnlyHint(t *testing.T) {
	t.Parallel()
	tm := newTestManager(t)
	cloudHint := Hint{ID: "cloud", Cooldown: time.Hour, Message: "cloud hint", CloudOnly: true}

	tm.local = true
	out := tm.run(cloudHint, hintA)
	assert.NotContains(t, out, "cloud hint")
	assert.Contains(t, out, "hint a")

	tm.local = false
	assert.Contains(t, tm.run(cloudHint), "cloud hint")
}

func TestSuppressedAndDisabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(tm *testManager)
	}{
		{"suppressed", func(tm *testManager) { tm.suppress() }},
		{"disabled", func(tm *testManager) { tm.disabled = true }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tm := newTestManager(t)
			tt.setup(tm)
			assert.Empty(t, tm.run(hintA))

			// A hint that wasn't shown doesn't start its cooldown.
			st, _, err := tm.load()
			require.NoError(t, err)
			assert.Empty(t, st.LastShown)
		})
	}
}

func TestFlushClearsQueue(t *testing.T) {
	t.Parallel()
	tm := newTestManager(t)
	tm.offer(hintA)

	var buf bytes.Buffer
	tm.flush(&buf)
	tm.flush(&buf)
	assert.Equal(t, 1, bytes.Count(buf.Bytes(), []byte("hint a")))
}

func TestStateRoundTrip(t *testing.T) {
	t.Parallel()
	tm := newTestManager(t)

	assert.Contains(t, tm.run(hintA), "hint a")

	st, _, err := tm.load()
	require.NoError(t, err)
	assert.True(t, tm.now.Equal(st.LastShown["a"]), "the showing is recorded")
}

func TestRecoversFromCorruptStateFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{"not json", "{not json"},
		{"empty", ""},
		// What a reader sees if another CLI process is partway through writing the file.
		{"truncated", `{"lastShown": {"a": "2026-11-06T12:`},
		{"wrong type", `{"lastShown": 5}`},
		{"null", "null"},
		{"null map", `{"lastShown": null}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tm := newTestManager(t)
			require.NoError(t, os.WriteFile(tm.path, []byte(tt.content), 0o600))

			// A corrupt file is treated as empty state, so the hint is shown...
			assert.Contains(t, tm.run(hintA), "hint a")

			// ...and the file is replaced with valid state, so the cooldown holds from then on rather than
			// resetting on every command.
			raw, err := os.ReadFile(tm.path)
			require.NoError(t, err)
			var st state
			require.NoError(t, json.Unmarshal(raw, &st))
			assert.Contains(t, st.LastShown, "a")
			assert.Empty(t, tm.run(hintA))
		})
	}
}

func TestUnwritableStateShowsNothing(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("file permissions don't stop writes here")
	}
	tm := newTestManager(t)
	require.NoError(t, os.WriteFile(tm.path, []byte("{}"), 0o400))

	// A hint that can't be recorded isn't shown, so it can't be shown on every command.
	assert.Empty(t, tm.run(hintA))
}

func TestUnreadableStateShowsNothing(t *testing.T) {
	t.Parallel()
	tm := newTestManager(t)
	// Make the state path a directory, so it can neither be read nor replaced.
	tm.path = t.TempDir()
	assert.Empty(t, tm.run(hintA))
}

// TestLoadSettings sets environment variables, so it can't run in parallel.
func TestLoadSettings(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(workspace.PulumiCredentialsPathEnvVar, dir)
	t.Setenv("PULUMI_DISABLE_HINTS", "")
	t.Setenv("PULUMI_SKIP_HINTS", " a, ,b ")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"),
		[]byte(`{"hints": {"skip": ["c"]}}`), 0o600))

	s := loadSettings()
	assert.False(t, s.disabled)
	assert.Equal(t, map[string]bool{"a": true, "b": true, "c": true}, s.skip)

	t.Setenv("PULUMI_DISABLE_HINTS", "true")
	assert.True(t, loadSettings().disabled)
}

func TestESCHintOnCredentialsFailure(t *testing.T) {
	t.Parallel()

	const message = "Run `pulumi env setup aws`."
	stack := &backend.MockStack{}
	tests := []struct {
		name         string
		kind         plugin.HintKind
		message      string
		isCloud      bool
		stack        backend.Stack
		resources    int
		resourcesErr error
		want         bool
	}{
		{"established cloud stack", plugin.HintDynamicCredentials, message, true, stack, 2, nil, true},
		{"diy backend", plugin.HintDynamicCredentials, message, false, stack, 2, nil, false},
		{"no message", plugin.HintDynamicCredentials, "", true, stack, 2, nil, false},
		{"unspecified hint", plugin.HintUnspecified, message, true, stack, 2, nil, false},
		{"no stack", plugin.HintDynamicCredentials, message, true, nil, 2, nil, false},
		{"too few resources", plugin.HintDynamicCredentials, message, true, stack, 1, nil, false},
		{"resources unknown", plugin.HintDynamicCredentials, message, true, stack, 2, errors.New("offline"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tm := newTestManager(t)
			tm.local = !tt.isCloud
			tm.resources, tm.resourcesErr = tt.resources, tt.resourcesErr
			(&recorder{m: tm.manager, stack: tt.stack}).RecordHint(plugin.Hint{Kind: tt.kind, Message: tt.message})
			out := tm.run()
			if tt.want {
				assert.Contains(t, out, message)
			} else {
				assert.Empty(t, out)
			}
		})
	}
}

func TestHintWithoutStackGateIgnoresStack(t *testing.T) {
	t.Parallel()
	tm := newTestManager(t)
	tm.resourcesErr = errors.New("must not be called")

	assert.Contains(t, tm.run(hintA), "hint a")
}

func TestCountStackResources(t *testing.T) {
	t.Parallel()

	deployment := `{"resources": [
		{"type": "pulumi:pulumi:Stack"},
		{"type": "pulumi:providers:aws"},
		{"type": "aws:s3/bucket:Bucket"},
		{"type": "aws:s3/bucket:Bucket", "delete": true},
		{"type": "aws:iam/role:Role"}
	]}`
	s := &backend.MockStack{BackendF: func() backend.Backend {
		return &backend.MockBackend{
			ExportDeploymentF: func(context.Context, backend.Stack) (*apitype.UntypedDeployment, error) {
				return &apitype.UntypedDeployment{Version: 3, Deployment: json.RawMessage(deployment)}, nil
			},
		}
	}}
	n, err := countStackResources(s)
	require.NoError(t, err)
	assert.Equal(t, 2, n)
}
