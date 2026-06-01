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

package plugin

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/pluginstorage"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func runPluginRm(
	t *testing.T,
	plugins []workspace.PluginInfo,
	args ...string,
) (string, error) {
	t.Helper()

	cmd := newPluginRmCmd(pluginstorage.MockContext{
		GetPluginsF: func(context.Context) ([]workspace.PluginInfo, error) {
			return plugins, nil
		},
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(t.Context())
	return out.String(), err
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()

	_, err := os.Stat(path)
	require.NoError(t, err)
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()

	_, err := os.Stat(path)
	require.True(t, os.IsNotExist(err), "expected %q to be removed, got %v", path, err)
}

func TestPluginRmOlderThanDeletesOnlyOldSidecar(t *testing.T) {
	t.Parallel()

	now := time.Now()
	recent := now.Add(-10 * 24 * time.Hour)
	old := now.Add(-90 * 24 * time.Hour)
	recentPlugin := mkPlugin(t, "aws", "7.30.0", &recent)
	oldPlugin := mkPlugin(t, "aws", "7.29.0", &old)
	unknownPlugin := mkPlugin(t, "aws", "7.28.0", nil)

	out, err := runPluginRm(t, []workspace.PluginInfo{recentPlugin, oldPlugin, unknownPlugin},
		"resource", "aws", "--older-than", "30d", "--yes")

	require.NoError(t, err)
	assert.Contains(t, out, "removed: resource aws-7.29.0")
	assertPathExists(t, recentPlugin.Path)
	assertPathMissing(t, oldPlugin.Path)
	assertPathExists(t, unknownPlugin.Path)
}

func TestPluginRmOlderThanKeepLatestProtectsNewest(t *testing.T) {
	t.Parallel()

	now := time.Now()
	old := now.Add(-90 * 24 * time.Hour)
	latest := mkPlugin(t, "aws", "7.30.0", &old)
	secondLatest := mkPlugin(t, "aws", "7.29.0", &old)
	thirdLatest := mkPlugin(t, "aws", "7.28.0", &old)

	_, err := runPluginRm(t, []workspace.PluginInfo{latest, secondLatest, thirdLatest},
		"resource", "aws", "--older-than", "30d", "--keep-latest", "2", "--yes")

	require.NoError(t, err)
	assertPathExists(t, latest.Path)
	assertPathExists(t, secondLatest.Path)
	assertPathMissing(t, thirdLatest.Path)
}

func TestPluginRmOlderThanAllowedWithoutAll(t *testing.T) {
	t.Parallel()

	now := time.Now()
	old := now.Add(-90 * 24 * time.Hour)
	oldPlugin := mkPlugin(t, "aws", "7.29.0", &old)

	_, err := runPluginRm(t, []workspace.PluginInfo{oldPlugin}, "--older-than", "30d", "--yes")

	require.NoError(t, err)
	assertPathMissing(t, oldPlugin.Path)
}

func TestPluginRmAllWithFilterErrors(t *testing.T) {
	t.Parallel()

	_, err := runPluginRm(t, nil, "--all", "--older-than", "30d", "--yes")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--all cannot be combined with filters")
}

func TestPluginRmKeepLatestNegativeErrors(t *testing.T) {
	t.Parallel()

	_, err := runPluginRm(t, nil, "resource", "aws", "--keep-latest", "-1", "--yes")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--keep-latest must be non-negative")
}

func TestPluginRmKeepLatestZeroRequiresAll(t *testing.T) {
	t.Parallel()

	now := time.Now()
	old := now.Add(-90 * 24 * time.Hour)
	oldPlugin := mkPlugin(t, "aws", "7.29.0", &old)

	_, err := runPluginRm(t, []workspace.PluginInfo{oldPlugin}, "--keep-latest", "0", "--yes")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--keep-latest 0 is equivalent to --all")
	assertPathExists(t, oldPlugin.Path)
}

func TestFormatPluginDeleteLineIncludesReasons(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 22, 17, 1, 23, 0, time.UTC)
	v := mkPlugin(t, "aws", "7.29.0", &now)
	got := formatPluginDeleteLine(pluginDeleteSelection{
		Plugin: v,
		Reasons: []string{
			"outside latest 2 versions",
			"last used 2026-05-22T17:01:23Z",
		},
	})

	assert.Equal(t,
		"    resource aws-7.29.0 (outside latest 2 versions; last used 2026-05-22T17:01:23Z)",
		got)
}
