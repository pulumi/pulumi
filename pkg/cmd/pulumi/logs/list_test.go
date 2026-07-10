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
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/engine/encryptedlog"
)

func TestParseLogName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		filename string
		stack    string
		updateID string
		command  string
		cliLevel bool
	}{
		{
			name:     "cli-level with pid",
			filename: "pulumi-20260401T120000-12345.log",
			stack:    "",
			updateID: "12345",
			cliLevel: true,
		},
		{
			name:     "cli-level with pid and command",
			filename: "pulumi-20260401T120000-12345-about.log",
			stack:    "",
			updateID: "12345",
			command:  "about",
			cliLevel: true,
		},
		{
			name:     "cli-level with multi-word command",
			filename: "pulumi-20260401T120000-12345-stack_ls.log",
			stack:    "",
			updateID: "12345",
			command:  "stack ls",
			cliLevel: true,
		},
		{
			name:     "cli-level with hyphenated command",
			filename: "pulumi-20260401T120000-12345-gen-completion.log",
			stack:    "",
			updateID: "12345",
			command:  "gen-completion",
			cliLevel: true,
		},
		{
			name:     "stack only",
			filename: "dev-20260401T120000.log",
			stack:    "dev",
			updateID: "",
			cliLevel: false,
		},
		{
			name:     "stack with update id",
			filename: "dev-20260401T120000-abc123.log",
			stack:    "dev",
			updateID: "abc123",
			cliLevel: false,
		},
		{
			name:     "org/project/stack encoded with plus",
			filename: "acme+myproj+prod-20260401T120000-u-1.log",
			stack:    "acme/myproj/prod",
			updateID: "u-1",
			cliLevel: false,
		},
		{
			name:     "no timestamp returns empty",
			filename: "garbage.log",
			stack:    "",
			updateID: "",
			cliLevel: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stack, updateID, command, cliLevel := parseLogName(tc.filename)
			require.Equal(t, tc.stack, stack)
			require.Equal(t, tc.updateID, updateID)
			require.Equal(t, tc.command, command)
			require.Equal(t, tc.cliLevel, cliLevel)
		})
	}
}

func TestListLogs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cliPath := filepath.Join(dir, "pulumi-20260401T100000-9999.log")
	require.NoError(t, os.WriteFile(cliPath, []byte("plain"), 0o600))

	cliCmdPath := filepath.Join(dir, "pulumi-20260401T090000-8888-stack_ls.log")
	require.NoError(t, os.WriteFile(cliCmdPath, []byte("plain"), 0o600))

	devPath := filepath.Join(dir, "acme+proj+dev-20260401T120000-u-1.log")
	require.NoError(t, os.WriteFile(devPath, []byte(encryptedlog.Magic), 0o600))

	prodPath := filepath.Join(dir, "prod-20260401T110000.log")
	gzFile, err := os.Create(prodPath)
	require.NoError(t, err)
	gz := gzip.NewWriter(gzFile)
	_, err = gz.Write([]byte("payload"))
	require.NoError(t, err)
	require.NoError(t, gz.Close())
	require.NoError(t, gzFile.Close())

	// Files that should be ignored.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "stray.log"), []byte("x"), 0o600))

	entries, err := listLogs(dir)
	require.NoError(t, err)
	require.Len(t, entries, 4)

	require.Equal(t, "acme+proj+dev-20260401T120000-u-1.log", entries[0].name)
	require.Equal(t, devPath, entries[0].path)
	require.Equal(t, "acme/proj/dev", entries[0].stack)
	require.Equal(t, "u-1", entries[0].updateID)
	require.False(t, entries[0].cliLevel)

	require.Equal(t, "prod-20260401T110000.log", entries[1].name)
	require.Equal(t, prodPath, entries[1].path)
	require.Equal(t, "prod", entries[1].stack)
	require.Equal(t, "", entries[1].updateID)

	require.Equal(t, "pulumi-20260401T100000-9999.log", entries[2].name)
	require.Equal(t, cliPath, entries[2].path)
	require.Equal(t, "", entries[2].stack)
	require.True(t, entries[2].cliLevel)
	require.Equal(t, "9999", entries[2].updateID)
	require.Equal(t, "", entries[2].command)

	require.Equal(t, "pulumi-20260401T090000-8888-stack_ls.log", entries[3].name)
	require.Equal(t, cliCmdPath, entries[3].path)
	require.Equal(t, "", entries[3].stack)
	require.True(t, entries[3].cliLevel)
	require.Equal(t, "8888", entries[3].updateID)
	require.Equal(t, "stack ls", entries[3].command)
}

func TestListLogsMissingDir(t *testing.T) {
	t.Parallel()

	entries, err := listLogs(filepath.Join(t.TempDir(), "does-not-exist"))
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestPrintLogsJSON(t *testing.T) {
	t.Parallel()

	ts1, err := time.Parse("20060102T150405", "20260401T120000")
	require.NoError(t, err)
	ts2, err := time.Parse("20060102T150405", "20260401T100000")
	require.NoError(t, err)

	dir := t.TempDir()
	devPath := filepath.Join(dir, "acme+proj+dev-20260401T120000-u-1.log")
	cliPath := filepath.Join(dir, "pulumi-20260401T100000-9999.log")

	entries := []logEntry{
		{
			path:      devPath,
			name:      "acme+proj+dev-20260401T120000-u-1.log",
			stack:     "acme/proj/dev",
			timestamp: ts1,
			updateID:  "u-1",
			cliLevel:  false,
			size:      42,
		},
		{
			path:      cliPath,
			name:      "pulumi-20260401T100000-9999.log",
			stack:     "",
			timestamp: ts2,
			updateID:  "9999",
			command:   "stack ls",
			cliLevel:  true,
			size:      7,
		},
	}

	var buf bytes.Buffer
	require.NoError(t, printLogsJSON(&buf, entries))

	var got []logEntryListJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Len(t, got, 2)

	require.Equal(t, devPath, got[0].Path)
	require.Equal(t, "acme/proj/dev", got[0].Stack)
	require.Equal(t, "u-1", got[0].UpdateID)
	require.Empty(t, got[0].PID)
	require.Equal(t, int64(42), got[0].Size)

	require.Equal(t, cliPath, got[1].Path)
	require.Empty(t, got[1].Stack)
	require.Empty(t, got[1].UpdateID)
	require.Equal(t, "9999", got[1].PID)
	require.Equal(t, "stack ls", got[1].Command)
	require.Empty(t, got[0].Command)
}

func TestPrintLogsTableEmpty(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "logs")

	var buf bytes.Buffer
	require.NoError(t, printLogsTable(&buf, dir, nil))
	require.Contains(t, buf.String(), "No log files found in "+dir)
}

func TestParseLogTimestampLocalZone(t *testing.T) {
	t.Parallel()

	// Filenames are written using time.Now().Format(...) in local time,
	// so parsing must round-trip in the same zone — otherwise displays
	// like "1 hour from now" appear when the offset is non-zero.
	want := time.Date(2026, 4, 1, 12, 0, 0, 0, time.Local)
	got, ok := parseLogTimestamp("dev-20260401T120000.log")
	require.True(t, ok)
	require.True(t, want.Equal(got), "expected %v, got %v", want, got)
}

func TestPrintLogsTableRenders(t *testing.T) {
	t.Parallel()

	ts, err := time.Parse("20060102T150405", "20260401T120000")
	require.NoError(t, err)

	dir := t.TempDir()
	devPath := filepath.Join(dir, "acme+proj+dev-20260401T120000-u-1.log")
	cliPath := filepath.Join(dir, "pulumi-20260401T100000-9999.log")
	cliCmdPath := filepath.Join(dir, "pulumi-20260401T090000-8888-about.log")

	entries := []logEntry{
		{
			path:      devPath,
			name:      "acme+proj+dev-20260401T120000-u-1.log",
			stack:     "acme/proj/dev",
			timestamp: ts,
			updateID:  "u-1",
			size:      4096,
		},
		{
			path:      cliPath,
			name:      "pulumi-20260401T100000-9999.log",
			stack:     "",
			timestamp: ts.Add(-2 * time.Hour),
			updateID:  "9999",
			cliLevel:  true,
			size:      512,
		},
		{
			path:      cliCmdPath,
			name:      "pulumi-20260401T090000-8888-about.log",
			stack:     "",
			timestamp: ts.Add(-3 * time.Hour),
			updateID:  "8888",
			command:   "about",
			cliLevel:  true,
			size:      256,
		},
	}

	var buf bytes.Buffer
	require.NoError(t, printLogsTable(&buf, dir, entries))

	out := buf.String()
	for _, want := range []string{
		"STACK", "CREATED", "UPDATE ID", "SIZE", "FILE",
		"acme/proj/dev", "u-1",
		"(cli)", "9999",
		"(cli: about)", "8888",
		devPath,
		cliPath,
		cliCmdPath,
	} {
		require.Contains(t, out, want, "missing %q in output:\n%s", want, out)
	}
}
