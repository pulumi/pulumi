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

package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLastUsedSidecarPath(t *testing.T) {
	t.Parallel()

	pluginDir := filepath.Join("tmp", "plugins", "resource-aws-v7.30.0")
	got := LastUsedSidecarPath(pluginDir)
	assert.Equal(t, pluginDir+".lastused", got)
}

func TestRecordPluginUsage_WritesTimestamp(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "resource-aws-v7.30.0")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))

	before := time.Now()
	require.NoError(t, recordPluginUsage(pluginDir))
	after := time.Now()

	sidecar := LastUsedSidecarPath(pluginDir)
	info, err := os.Stat(sidecar)
	require.NoError(t, err)
	assert.False(t, info.ModTime().Before(before.Add(-time.Second)))
	assert.False(t, info.ModTime().After(after.Add(time.Second)))

	body, err := os.ReadFile(sidecar)
	require.NoError(t, err)
	bodyText := string(body)
	require.True(t, strings.HasSuffix(bodyText, "\n"))
	parsed, err := time.Parse(time.RFC3339, strings.TrimSuffix(bodyText, "\n"))
	require.NoError(t, err)
	assert.WithinDuration(t, before, parsed, 2*time.Second)
}

func TestRecordedLastUsedTime_Missing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "resource-aws-v7.30.0")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))

	got, ok := RecordedLastUsedTime(pluginDir)
	assert.False(t, ok)
	assert.True(t, got.IsZero())
}

func TestRecordedLastUsedTime_Present(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "resource-aws-v7.30.0")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	require.NoError(t, recordPluginUsage(pluginDir))

	got, ok := RecordedLastUsedTime(pluginDir)
	assert.True(t, ok)
	assert.WithinDuration(t, time.Now(), got, time.Second)
}

func TestRecordPluginUsage_Concurrent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "resource-aws-v7.30.0")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))

	const writers = 16
	var wg sync.WaitGroup
	wg.Add(writers)
	before := time.Now()
	for range writers {
		go func() {
			defer wg.Done()
			_ = recordPluginUsage(pluginDir)
		}()
	}
	wg.Wait()
	after := time.Now()

	mtime, ok := RecordedLastUsedTime(pluginDir)
	require.True(t, ok, "expected sidecar to exist after concurrent writes")
	assert.False(t, mtime.Before(before.Add(-time.Second)),
		"mtime %v should not predate the start of the test (%v)", mtime, before)
	assert.False(t, mtime.After(after.Add(time.Second)),
		"mtime %v should not be later than the end of the test (%v)", mtime, after)
}
