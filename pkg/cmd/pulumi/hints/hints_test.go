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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

var (
	hintA = Hint{ID: "a", Cooldown: 7 * 24 * time.Hour, Message: "hint a"}
	hintB = Hint{ID: "b", Cooldown: 7 * 24 * time.Hour, Message: "hint b"}
)

type testManager struct {
	*manager
	now      time.Time
	disabled bool
	ci       bool
	path     string
}

func newTestManager(t *testing.T) *testManager {
	tm := &testManager{
		now:  time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC),
		path: filepath.Join(t.TempDir(), stateFileName),
	}
	tm.manager = &manager{
		now:       func() time.Time { return tm.now },
		statePath: func() (string, error) { return tm.path, nil },
		disabled:  func() bool { return tm.disabled },
		isCI:      func() bool { return tm.ci },
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

func TestNoHintBeforeSuccessfulUpdate(t *testing.T) {
	t.Parallel()
	tm := newTestManager(t)

	assert.Empty(t, tm.run(hintA))
	_, err := os.Stat(tm.path)
	assert.True(t, os.IsNotExist(err), "no state should be written for a hint that wasn't shown")

	tm.recordSuccessfulUpdate()
	assert.Contains(t, tm.run(hintA), "hint a")
}

func TestOneHintPerCommandAndGlobalBudget(t *testing.T) {
	t.Parallel()
	tm := newTestManager(t)
	tm.recordSuccessfulUpdate()

	out := tm.run(hintA, hintB)
	assert.Contains(t, out, "hint a")
	assert.NotContains(t, out, "hint b")

	// hint b has never been shown, but the global budget allows only one hint a day.
	tm.now = tm.now.Add(23 * time.Hour)
	assert.Empty(t, tm.run(hintB))

	tm.now = tm.now.Add(time.Hour)
	assert.Contains(t, tm.run(hintB), "hint b")
}

func TestPerHintCooldown(t *testing.T) {
	t.Parallel()
	tm := newTestManager(t)
	tm.recordSuccessfulUpdate()

	assert.Contains(t, tm.run(hintA), "hint a")

	// Past the global budget but within hint a's cooldown, so hint b is picked instead.
	tm.now = tm.now.Add(2 * 24 * time.Hour)
	out := tm.run(hintA, hintB)
	assert.NotContains(t, out, "hint a")
	assert.Contains(t, out, "hint b")

	tm.now = tm.now.Add(2 * 24 * time.Hour)
	assert.Empty(t, tm.run(hintA))

	tm.now = tm.now.Add(3 * 24 * time.Hour)
	assert.Contains(t, tm.run(hintA), "hint a")
}

func TestSuppressedDisabledAndCI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(tm *testManager)
	}{
		{"suppressed", func(tm *testManager) { tm.suppress() }},
		{"disabled", func(tm *testManager) { tm.disabled = true }},
		{"ci", func(tm *testManager) { tm.ci = true }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tm := newTestManager(t)
			tm.recordSuccessfulUpdate()
			tt.setup(tm)
			assert.Empty(t, tm.run(hintA))

			// A hint that wasn't shown doesn't use up the budget.
			st, _, err := tm.load()
			require.NoError(t, err)
			assert.Empty(t, st.LastShown)
		})
	}
}

func TestFlushClearsQueue(t *testing.T) {
	t.Parallel()
	tm := newTestManager(t)
	tm.recordSuccessfulUpdate()
	tm.offer(hintA)

	var buf bytes.Buffer
	tm.flush(&buf)
	tm.flush(&buf)
	assert.Equal(t, 1, bytes.Count(buf.Bytes(), []byte("hint a")))
}

func TestCorruptStateFile(t *testing.T) {
	t.Parallel()
	tm := newTestManager(t)
	require.NoError(t, os.WriteFile(tm.path, []byte("{not json"), 0o600))

	// A corrupt file is treated as empty state and replaced.
	tm.recordSuccessfulUpdate()
	assert.Contains(t, tm.run(hintA), "hint a")
}

func TestUnreadableStateShowsNothing(t *testing.T) {
	t.Parallel()
	tm := newTestManager(t)
	tm.recordSuccessfulUpdate()
	// Make the state path a directory, so it can neither be read nor replaced.
	tm.path = t.TempDir()
	assert.Empty(t, tm.run(hintA))
}

func TestESCHintOnCredentialsFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		record  map[tokens.Package]plugin.ErrorKind
		isCloud bool
		want    string
	}{
		{
			"aws on cloud",
			map[tokens.Package]plugin.ErrorKind{"aws": plugin.ErrorKindCredentials},
			true,
			"`pulumi env setup aws`",
		},
		{
			"azure-native on cloud",
			map[tokens.Package]plugin.ErrorKind{"azure-native": plugin.ErrorKindCredentials},
			true,
			"`pulumi env setup azure`",
		},
		{"aws on diy", map[tokens.Package]plugin.ErrorKind{"aws": plugin.ErrorKindCredentials}, false, ""},
		{"unsupported package", map[tokens.Package]plugin.ErrorKind{"random": plugin.ErrorKindCredentials}, true, ""},
		{"unspecified kind", map[tokens.Package]plugin.ErrorKind{"aws": plugin.ErrorKindUnspecified}, true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tm := newTestManager(t)
			tm.recordSuccessfulUpdate()
			for pkg, kind := range tt.record {
				tm.RecordErrorKind(pkg, kind)
			}
			tm.offerFailureHints(func() bool { return tt.isCloud })
			out := tm.run()
			if tt.want == "" {
				assert.Empty(t, out)
			} else {
				assert.Contains(t, out, tt.want)
			}
		})
	}
}
