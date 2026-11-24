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
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

var (
	hintA = Hint{ID: "a", Cooldown: 7 * 24 * time.Hour, Message: "hint a"}
	hintB = Hint{ID: "b", Cooldown: 7 * 24 * time.Hour, Message: "hint b"}
)

type testManager struct {
	*Manager
	now      time.Time
	disabled bool
	// noTerminal is true when the command doesn't run interactively, as in CI.
	noTerminal bool
	skip       []string
	local      bool
	path       string
}

func newTestManager(t *testing.T) *testManager {
	tm := &testManager{
		now:  time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC),
		path: filepath.Join(t.TempDir(), stateFileName),
	}
	tm.Manager = tm.newManager()
	return tm
}

// newManager returns a Manager for a new command that uses tm's clock, state file and settings.
func (tm *testManager) newManager() *Manager {
	return &Manager{
		now:         func() time.Time { return tm.now },
		statePath:   func() (string, error) { return tm.path, nil },
		interactive: func(io.Writer) bool { return !tm.noTerminal },
		settings: func() settings {
			s := settings{disabled: tm.disabled, skip: map[string]bool{}}
			for _, id := range tm.skip {
				s.skip[id] = true
			}
			return s
		},
		isCloud: func() bool { return !tm.local },
	}
}

// run offers hs and flushes them, returning what was printed.
func (tm *testManager) run(hs ...Hint) string {
	for _, h := range hs {
		tm.Offer(h)
	}
	var buf bytes.Buffer
	tm.Flush(&buf)
	// Like the CLI, each command gets its own Manager.
	tm.Manager = tm.newManager()
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
		{"suppressed", func(tm *testManager) { tm.Suppress() }},
		{"disabled", func(tm *testManager) { tm.disabled = true }},
		{"not interactive", func(tm *testManager) { tm.noTerminal = true }},
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

func TestOfferIfChecksOnlyWhenNeeded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(tm *testManager)
		// first is offered before the conditional hint.
		first     []Hint
		wantCalls int
	}{
		{"eligible", func(*testManager) {}, nil, 1},
		{"suppressed", func(tm *testManager) { tm.Suppress() }, nil, 0},
		{"disabled", func(tm *testManager) { tm.disabled = true }, nil, 0},
		{"not interactive", func(tm *testManager) { tm.noTerminal = true }, nil, 0},
		{"skipped", func(tm *testManager) { tm.skip = []string{"a"} }, nil, 0},
		{"on cooldown", func(tm *testManager) { tm.run(hintA) }, nil, 0},
		{"another hint shown first", func(*testManager) {}, []Hint{hintB}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tm := newTestManager(t)
			tt.setup(tm)
			calls := 0
			for _, h := range tt.first {
				tm.Offer(h)
			}
			tm.OfferIf(hintA, func() bool { calls++; return true })
			tm.Flush(&bytes.Buffer{})
			assert.Equal(t, tt.wantCalls, calls)
		})
	}
}

func TestOfferIfNotApplying(t *testing.T) {
	t.Parallel()
	tm := newTestManager(t)

	tm.OfferIf(hintA, func() bool { return false })
	out := tm.run(hintB)
	assert.NotContains(t, out, "hint a")
	assert.Contains(t, out, "hint b")

	// A hint that doesn't apply doesn't start its cooldown.
	st, _, err := tm.load()
	require.NoError(t, err)
	assert.NotContains(t, st.LastShown, "a")
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

func TestIsTerminal(t *testing.T) {
	t.Parallel()

	// Hints redirected to a buffer or a file, as with `2>errors.log`, have no one to read them.
	assert.False(t, isTerminal(&bytes.Buffer{}))
	f, err := os.Create(filepath.Join(t.TempDir(), "errors.log"))
	require.NoError(t, err)
	defer f.Close()
	assert.False(t, isTerminal(f))
}

func TestFirstFlushRecordsFirstSeen(t *testing.T) {
	t.Parallel()
	tm := newTestManager(t)
	first := tm.now

	// A flush with nothing offered still records when hints were first flushed.
	assert.Empty(t, tm.run())
	st, _, err := tm.load()
	require.NoError(t, err)
	assert.True(t, first.Equal(st.FirstSeen))

	// Later flushes, including ones that show a hint, keep it.
	tm.now = tm.now.Add(time.Hour)
	assert.Contains(t, tm.run(hintA), "hint a")
	st, _, err = tm.load()
	require.NoError(t, err)
	assert.True(t, first.Equal(st.FirstSeen))
}

func TestSuppressedFlushDoesNotRecordFirstSeen(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(tm *testManager)
	}{
		{"suppressed", func(tm *testManager) { tm.Suppress() }},
		{"disabled", func(tm *testManager) { tm.disabled = true }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tm := newTestManager(t)
			tt.setup(tm)
			assert.Empty(t, tm.run())
			assert.NoFileExists(t, tm.path)
		})
	}
}

func TestHideFromNewUsers(t *testing.T) {
	t.Parallel()
	tm := newTestManager(t)
	newUserHint := Hint{ID: "later", Cooldown: time.Hour, Message: "later hint", HideFromNewUsers: true}

	// On the first command, only hints that don't wait for new users are shown.
	out := tm.run(newUserHint, hintA)
	assert.NotContains(t, out, "later hint")
	assert.Contains(t, out, "hint a")

	tm.now = tm.now.Add(newUserPeriod - time.Minute)
	assert.Empty(t, tm.run(newUserHint))

	tm.now = tm.now.Add(time.Minute)
	assert.Contains(t, tm.run(newUserHint), "later hint")
}

func TestESCHintOnCredentialsFailure(t *testing.T) {
	t.Parallel()

	const message = "Run `pulumi env setup aws`."
	tests := []struct {
		name      string
		kind      plugin.HintKind
		message   string
		isCloud   bool
		firstSeen time.Duration
		want      bool
	}{
		{"established cloud user", plugin.HintDynamicCredentials, message, true, 2 * newUserPeriod, true},
		{"new user", plugin.HintDynamicCredentials, message, true, newUserPeriod / 2, false},
		{"diy backend", plugin.HintDynamicCredentials, message, false, 2 * newUserPeriod, false},
		{"no message", plugin.HintDynamicCredentials, "", true, 2 * newUserPeriod, false},
		{"unspecified hint", plugin.HintUnspecified, message, true, 2 * newUserPeriod, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tm := newTestManager(t)
			tm.local = !tt.isCloud
			require.NoError(t, save(tm.path, state{FirstSeen: tm.now.Add(-tt.firstSeen)}))
			tm.RecordProviderHint(plugin.Hint{Kind: tt.kind, Message: tt.message})
			out := tm.run()
			if tt.want {
				assert.Contains(t, out, message)
			} else {
				assert.Empty(t, out)
			}
		})
	}
}
