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

package logs

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConfirmToken(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		targets []logEntry
		want    string
	}{
		{
			name:    "empty targets falls back to yes",
			targets: nil,
			want:    "yes",
		},
		{
			name: "single stack target uses stack name",
			targets: []logEntry{
				{stack: "dev"},
			},
			want: "dev",
		},
		{
			name: "multiple targets in same stack use stack name",
			targets: []logEntry{
				{stack: "dev"},
				{stack: "dev"},
				{stack: "dev"},
			},
			want: "dev",
		},
		{
			name: "multiple stacks fall back to yes",
			targets: []logEntry{
				{stack: "dev"},
				{stack: "prod"},
			},
			want: "yes",
		},
		{
			name: "cli-level log falls back to yes",
			targets: []logEntry{
				{stack: "", cliLevel: true},
			},
			want: "yes",
		},
		{
			name: "stack mixed with cli-level falls back to yes",
			targets: []logEntry{
				{stack: "dev"},
				{stack: "", cliLevel: true},
			},
			want: "yes",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, confirmToken(tc.targets))
		})
	}
}

func TestFilterLogs(t *testing.T) {
	t.Parallel()

	t0 := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	entries := []logEntry{
		{stack: "dev", timestamp: t0},
		{stack: "dev", timestamp: t0.Add(-24 * time.Hour)},
		{stack: "prod", timestamp: t0.Add(-48 * time.Hour)},
		{stack: "", timestamp: t0.Add(-72 * time.Hour), cliLevel: true},
	}

	t.Run("no filter returns everything", func(t *testing.T) {
		t.Parallel()
		got := filterLogs(entries, "", nil)
		require.Len(t, got, 4)
	})

	t.Run("stack filter narrows by stack", func(t *testing.T) {
		t.Parallel()
		got := filterLogs(entries, "dev", nil)
		require.Len(t, got, 2)
		for _, e := range got {
			require.Equal(t, "dev", e.stack)
		}
	})

	t.Run("stack filter does not match cli-level logs", func(t *testing.T) {
		t.Parallel()
		got := filterLogs(entries, "", nil)
		require.Len(t, got, 4)
		// And with an explicit stack name, cli-level is excluded.
		got = filterLogs(entries, "dev", nil)
		for _, e := range got {
			require.False(t, e.cliLevel)
		}
	})

	t.Run("before filter excludes newer logs", func(t *testing.T) {
		t.Parallel()
		cutoff := t0.Add(-36 * time.Hour)
		got := filterLogs(entries, "", &cutoff)
		require.Len(t, got, 2)
		for _, e := range got {
			require.True(t, e.timestamp.Before(cutoff))
		}
	})

	t.Run("stack and before combine", func(t *testing.T) {
		t.Parallel()
		cutoff := t0.Add(-12 * time.Hour)
		got := filterLogs(entries, "dev", &cutoff)
		require.Len(t, got, 1)
		require.Equal(t, "dev", got[0].stack)
		require.True(t, got[0].timestamp.Before(cutoff))
	})

	t.Run("nothing matches yields empty slice", func(t *testing.T) {
		t.Parallel()
		got := filterLogs(entries, "missing", nil)
		require.Empty(t, got)
	})
}

func TestSelectLogsToRemoveByFilter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	devPath := filepath.Join(dir, "dev-20260401T120000.log")
	prodPath := filepath.Join(dir, "prod-20260401T100000.log")
	require.NoError(t, os.WriteFile(devPath, []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(prodPath, []byte("x"), 0o600))

	t.Run("--all picks everything", func(t *testing.T) {
		t.Parallel()
		targets, err := selectLogsToRemove(dir, "", "", true)
		require.NoError(t, err)
		require.Len(t, targets, 2)
	})

	t.Run("--stack narrows", func(t *testing.T) {
		t.Parallel()
		targets, err := selectLogsToRemove(dir, "dev", "", false)
		require.NoError(t, err)
		require.Len(t, targets, 1)
		require.Equal(t, "dev", targets[0].stack)
	})

	t.Run("--before with bad value errors", func(t *testing.T) {
		t.Parallel()
		_, err := selectLogsToRemove(dir, "", "not-a-time", false)
		require.Error(t, err)
	})

	t.Run("non-matching filter returns empty without erroring", func(t *testing.T) {
		t.Parallel()
		targets, err := selectLogsToRemove(dir, "missing", "", false)
		require.NoError(t, err)
		require.Empty(t, targets)
	})
}

func TestFormatLogRemovalChoicesDisambiguatesDuplicates(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	entries := []logEntry{
		{stack: "dev", timestamp: ts, updateID: "u-1", path: "/a.log"},
		{stack: "dev", timestamp: ts, updateID: "u-1", path: "/b.log"},
	}

	options, optionMap := formatLogRemovalChoices(entries)
	require.Len(t, options, 2)
	require.Len(t, optionMap, 2, "duplicates must be disambiguated to distinct keys")
	require.NotEqual(t, options[0], options[1])

	got := map[string]bool{}
	for _, opt := range options {
		got[optionMap[opt]] = true
	}
	require.True(t, got["/a.log"])
	require.True(t, got["/b.log"])
}
