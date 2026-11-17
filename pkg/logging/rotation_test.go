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

package logging

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRotateByAge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create files with different ages.
	old := filepath.Join(dir, "old-20260101T000000.log")
	recent := filepath.Join(dir, "recent-20260401T000000.log")
	notALog := filepath.Join(dir, "keep.txt")

	require.NoError(t, os.WriteFile(old, []byte("old"), 0o600))
	require.NoError(t, os.WriteFile(recent, []byte("recent"), 0o600))
	require.NoError(t, os.WriteFile(notALog, []byte("keep"), 0o600))

	now := time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC)
	rotateLogs(dir, now)

	// Old .log file should be deleted, recent .log and .txt should remain.
	_, err := os.Stat(old)
	assert.True(t, os.IsNotExist(err), "old log should be deleted")

	_, err = os.Stat(recent)
	require.NoError(t, err, "recent log should remain")

	_, err = os.Stat(notALog)
	require.NoError(t, err, "non-log file should remain")
}

func TestRotateBySize(t *testing.T) {
	dir := t.TempDir()

	// Create files that together exceed defaultMaxTotalMB.
	// Use a low max by setting the env var.
	t.Setenv("PULUMI_LOG_ROTATION_MAX_TOTAL_MB", "1")

	// Create 3 files, ~400KB each = 1.2MB total, exceeding 1MB limit.
	data := make([]byte, 400*1024)
	f1 := filepath.Join(dir, "c-20260401T010000.log")
	f2 := filepath.Join(dir, "b-20260401T020000.log")
	f3 := filepath.Join(dir, "a-20260401T030000.log")

	require.NoError(t, os.WriteFile(f1, data, 0o600))
	require.NoError(t, os.WriteFile(f2, data, 0o600))
	require.NoError(t, os.WriteFile(f3, data, 0o600))

	now := time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC)
	rotateLogs(dir, now)

	// Total is 1.2MB > 1MB limit. Delete oldest (f1) → 800KB, under limit.
	// f1 should be deleted; f2 and f3 should remain.
	_, err := os.Stat(f1)
	assert.True(t, os.IsNotExist(err), "oldest file should be deleted")

	_, err = os.Stat(f2)
	require.NoError(t, err, "second file should remain")

	_, err = os.Stat(f3)
	require.NoError(t, err, "newest file should remain")
}
