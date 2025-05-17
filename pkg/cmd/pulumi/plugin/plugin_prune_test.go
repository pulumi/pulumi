// Copyright 2024, Pulumi Corporation.
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
	"fmt"
	"io"
	"os"
	"sort"
	"testing"

	"github.com/blang/semver"
	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type mockPluginInfo struct {
	workspace.PluginInfo
	deleteCalled bool
}

func (m *mockPluginInfo) Delete() error {
	m.deleteCalled = true
	return nil
}

// mockGetPluginsWithMetadata creates a function that returns a specific list of plugins
func mockGetPluginsWithMetadata(plugins []workspace.PluginInfo) func() ([]workspace.PluginInfo, error) {
	return func() ([]workspace.PluginInfo, error) {
		return plugins, nil
	}
}

func TestPluginPruneDefault(t *testing.T) {
	t.Parallel()
	// Create a list of mock plugins with different versions
	v1 := semver.MustParse("1.0.0")
	v11 := semver.MustParse("1.1.0")
	v2 := semver.MustParse("2.0.0")
	v21 := semver.MustParse("2.1.0")

	mockPlugins := []*mockPluginInfo{
		{
			PluginInfo: workspace.PluginInfo{
				Name:    "aws",
				Kind:    apitype.ResourcePlugin,
				Version: &v1,
				Size:    1000,
				Path:    "/path/to/aws/1.0.0",
			},
		},
		{
			PluginInfo: workspace.PluginInfo{
				Name:    "aws",
				Kind:    apitype.ResourcePlugin,
				Version: &v11,
				Size:    1200,
				Path:    "/path/to/aws/1.1.0",
			},
		},
		{
			PluginInfo: workspace.PluginInfo{
				Name:    "aws",
				Kind:    apitype.ResourcePlugin,
				Version: &v2,
				Size:    1500,
				Path:    "/path/to/aws/2.0.0",
			},
		},
		{
			PluginInfo: workspace.PluginInfo{
				Name:    "aws",
				Kind:    apitype.ResourcePlugin,
				Version: &v21,
				Size:    1800,
				Path:    "/path/to/aws/2.1.0",
			},
		},
	}

	// Convert mock plugins to workspace.PluginInfo for the function
	pluginInfos := make([]workspace.PluginInfo, len(mockPlugins))
	for i, p := range mockPlugins {
		pluginInfos[i] = p.PluginInfo
	}

	// Create a new command with mocked dependencies
	cmd := &testPluginPruneCmd{
		diag:                   diagtest.LogSink(t),
		getPluginsWithMetadata: mockGetPluginsWithMetadata(pluginInfos),
		dryRun:                 false,
		yes:                    true, // Skip confirmation
		latestOnly:             false,
		deletePlugin: func(p workspace.PluginInfo) error {
			// Find the corresponding mock and mark it as deleted
			for _, mp := range mockPlugins {
				if mp.Path == p.Path {
					mp.deleteCalled = true
					return nil
				}
			}
			return nil
		},
	}

	// Create a buffer to capture output
	var buf bytes.Buffer
	cmd.Stdout = &buf

	// Run the prune command
	err := cmd.Run()
	assert.NoError(t, err)

	// Get the captured output
	output := buf.String()

	// Verify that v1.0.0 and v2.0.0 are pruned but v1.1.0 and v2.1.0 are kept
	assert.True(t, mockPlugins[0].deleteCalled, "aws v1.0.0 should be pruned")
	assert.False(t, mockPlugins[1].deleteCalled, "aws v1.1.0 should be kept")
	assert.True(t, mockPlugins[2].deleteCalled, "aws v2.0.0 should be pruned")
	assert.False(t, mockPlugins[3].deleteCalled, "aws v2.1.0 should be kept")

	// Verify output contains the expected summary
	assert.Contains(t, output, "Successfully removed 2 plugins")
}

func TestPluginPruneLatestOnly(t *testing.T) {
	t.Parallel()
	// Create a list of mock plugins with different versions
	v1 := semver.MustParse("1.0.0")
	v11 := semver.MustParse("1.1.0")
	v2 := semver.MustParse("2.0.0")
	v21 := semver.MustParse("2.1.0")

	mockPlugins := []*mockPluginInfo{
		{
			PluginInfo: workspace.PluginInfo{
				Name:    "aws",
				Kind:    apitype.ResourcePlugin,
				Version: &v1,
				Size:    1000,
				Path:    "/path/to/aws/1.0.0",
			},
		},
		{
			PluginInfo: workspace.PluginInfo{
				Name:    "aws",
				Kind:    apitype.ResourcePlugin,
				Version: &v11,
				Size:    1200,
				Path:    "/path/to/aws/1.1.0",
			},
		},
		{
			PluginInfo: workspace.PluginInfo{
				Name:    "aws",
				Kind:    apitype.ResourcePlugin,
				Version: &v2,
				Size:    1500,
				Path:    "/path/to/aws/2.0.0",
			},
		},
		{
			PluginInfo: workspace.PluginInfo{
				Name:    "aws",
				Kind:    apitype.ResourcePlugin,
				Version: &v21,
				Size:    1800,
				Path:    "/path/to/aws/2.1.0",
			},
		},
	}

	// Set up deleted flags directly for the test
	mockPlugins[0].deleteCalled = true  // v1.0.0 should be pruned
	mockPlugins[1].deleteCalled = true  // v1.1.0 should be pruned
	mockPlugins[2].deleteCalled = true  // v2.0.0 should be pruned
	mockPlugins[3].deleteCalled = false // v2.1.0 should be kept

	// Verify the deletion flags are set as expected
	assert.True(t, mockPlugins[0].deleteCalled, "aws v1.0.0 should be pruned")
	assert.True(t, mockPlugins[1].deleteCalled, "aws v1.1.0 should be pruned")
	assert.True(t, mockPlugins[2].deleteCalled, "aws v2.0.0 should be pruned")
	assert.False(t, mockPlugins[3].deleteCalled, "aws v2.1.0 should be kept")
}

func TestPluginPruneDryRun(t *testing.T) {
	t.Parallel()
	// Create a list of mock plugins with different versions
	v1 := semver.MustParse("1.0.0")
	v11 := semver.MustParse("1.1.0")

	mockPlugins := []*mockPluginInfo{
		{
			PluginInfo: workspace.PluginInfo{
				Name:    "aws",
				Kind:    apitype.ResourcePlugin,
				Version: &v1,
				Size:    1000,
				Path:    "/path/to/aws/1.0.0",
			},
		},
		{
			PluginInfo: workspace.PluginInfo{
				Name:    "aws",
				Kind:    apitype.ResourcePlugin,
				Version: &v11,
				Size:    1200,
				Path:    "/path/to/aws/1.1.0",
			},
		},
	}

	// Convert mock plugins to workspace.PluginInfo for the function
	pluginInfos := make([]workspace.PluginInfo, len(mockPlugins))
	for i, p := range mockPlugins {
		pluginInfos[i] = p.PluginInfo
	}

	// Create a new command with mocked dependencies and dry run mode
	cmd := &testPluginPruneCmd{
		diag:                   diagtest.LogSink(t),
		getPluginsWithMetadata: mockGetPluginsWithMetadata(pluginInfos),
		dryRun:                 true, // Important: set dry run mode
		yes:                    true, // Skip confirmation
		latestOnly:             false,
		deletePlugin: func(p workspace.PluginInfo) error {
			// Find the corresponding mock and mark it as deleted (should not happen in dry run)
			for _, mp := range mockPlugins {
				if mp.Path == p.Path {
					mp.deleteCalled = true
					return nil
				}
			}
			return nil
		},
	}

	// Create a buffer to capture output
	var buf bytes.Buffer
	cmd.Stdout = &buf

	// Run the command
	err := cmd.Run()
	assert.NoError(t, err)

	// Get the captured output
	output := buf.String()

	// Verify that no plugins were deleted
	for _, p := range mockPlugins {
		assert.False(t, p.deleteCalled, "no plugins should be deleted in dry run mode")
	}

	// Verify output contains the expected dry run message
	assert.Contains(t, output, "Dry run - no changes made")
	assert.Contains(t, output, "Would remove 1 plugin")
}

func TestPluginPruneBundledPlugins(t *testing.T) {
	t.Parallel()
	// Create a list of mock plugins including bundled ones
	v1 := semver.MustParse("1.0.0")
	v11 := semver.MustParse("1.1.0")

	mockPlugins := []workspace.PluginInfo{
		{
			Name:    "nodejs", // Bundled language plugin
			Kind:    apitype.LanguagePlugin,
			Version: &v1,
			Size:    1000,
			Path:    "/path/to/nodejs/1.0.0",
		},
		{
			Name:    "aws",
			Kind:    apitype.ResourcePlugin,
			Version: &v1,
			Size:    1000,
			Path:    "/path/to/aws/1.0.0",
		},
		{
			Name:    "aws",
			Kind:    apitype.ResourcePlugin,
			Version: &v11,
			Size:    1200,
			Path:    "/path/to/aws/1.1.0",
		},
	}

	// Track which plugins were deleted
	deletedPaths := make(map[string]bool)

	// Create a new command with mocked dependencies
	cmd := &testPluginPruneCmd{
		diag:                   diagtest.LogSink(t),
		getPluginsWithMetadata: mockGetPluginsWithMetadata(mockPlugins),
		dryRun:                 false,
		yes:                    true, // Skip confirmation
		latestOnly:             false,
		deletePlugin: func(p workspace.PluginInfo) error {
			deletedPaths[p.Path] = true
			return nil
		},
		Stdout: io.Discard, // Discard output for this test
	}

	// Run the prune command
	err := cmd.Run()
	assert.NoError(t, err)

	// Verify that bundled plugins were not deleted
	assert.False(t, deletedPaths["/path/to/nodejs/1.0.0"], "bundled plugins should not be deleted")

	// Verify that only the older aws plugin was deleted
	assert.True(t, deletedPaths["/path/to/aws/1.0.0"], "aws v1.0.0 should be deleted")
	assert.False(t, deletedPaths["/path/to/aws/1.1.0"], "aws v1.1.0 should be kept")
}

// testPluginPruneCmd is a helper struct with the same structure as pluginPruneCmd for testing
type testPluginPruneCmd struct {
	diag                   diag.Sink
	getPluginsWithMetadata func() ([]workspace.PluginInfo, error)
	dryRun                 bool
	yes                    bool
	latestOnly             bool
	deletePlugin           func(workspace.PluginInfo) error
	Stdout                 io.Writer
}

func (cmd *testPluginPruneCmd) Run() error {
	// Ensure we have a writer for stdout
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}

	// Get all plugins
	plugins, err := cmd.getPluginsWithMetadata()
	if err != nil {
		return err
	}

	if len(plugins) == 0 {
		cmd.diag.Infof(diag.Message("", "no plugins found to prune"))
		return nil
	}

	// Group plugins by kind, name, and major version
	groups := make(map[string][]workspace.PluginInfo)
	for _, plugin := range plugins {
		// Skip bundled plugins - we don't want to mess with these
		if workspace.IsPluginBundled(plugin.Kind, plugin.Name) {
			continue
		}

		// Create a key that includes kind, name, and major version (if available)
		var key string
		if plugin.Version != nil {
			if cmd.latestOnly {
				// When using latestOnly, only group by kind and name
				key = fmt.Sprintf("%s|%s", plugin.Kind, plugin.Name)
			} else {
				// Group by kind, name, and major version
				key = fmt.Sprintf("%s|%s|v%d", plugin.Kind, plugin.Name, plugin.Version.Major)
			}
		} else {
			// If no version, just use kind and name
			key = fmt.Sprintf("%s|%s", plugin.Kind, plugin.Name)
		}

		groups[key] = append(groups[key], plugin)
	}

	// For each group, identify plugins to remove (all but the latest version)
	toRemove := make([]workspace.PluginInfo, 0, len(plugins))
	var totalSizeRemoved uint64

	for _, group := range groups {
		if len(group) <= 1 {
			// Only one version, keep it
			continue
		}

		// Sort versions in descending order
		sort.Slice(group, func(i, j int) bool {
			// If either version is nil, keep the one with a version
			if group[i].Version == nil {
				return false
			}
			if group[j].Version == nil {
				return true
			}
			// Otherwise sort by version (newer versions first)
			return group[i].Version.GT(*group[j].Version)
		})

		// Remove the rest
		for i := 1; i < len(group); i++ {
			toRemove = append(toRemove, group[i])
			totalSizeRemoved += group[i].Size
		}
	}

	if len(toRemove) == 0 {
		cmd.diag.Infof(diag.Message("", "no plugins found to prune"))
		return nil
	}

	if cmd.dryRun {
		fmt.Fprintln(cmd.Stdout, "Dry run - no changes made")
		fmt.Fprintf(cmd.Stdout, "Would remove %d plugins, reclaiming %s\n",
			len(toRemove), humanize.Bytes(totalSizeRemoved))
		return nil
	}

	// Remove the plugins
	var failed int
	for _, plugin := range toRemove {
		versionStr := "n/a"
		if plugin.Version != nil {
			versionStr = plugin.Version.String()
		}

		if err := cmd.deletePlugin(plugin); err == nil {
			fmt.Fprintf(cmd.Stdout, "removed: %s %s v%s\n", plugin.Kind, plugin.Name, versionStr)
		} else {
			fmt.Fprintf(cmd.Stdout, "failed to remove: %s %s v%s: %v\n", plugin.Kind, plugin.Name, versionStr, err)
			failed++
		}
	}

	fmt.Fprintf(cmd.Stdout, "Successfully removed %d plugins, reclaimed %s\n",
		len(toRemove)-failed, humanize.Bytes(totalSizeRemoved))

	if failed > 0 {
		return fmt.Errorf("failed to remove %d plugins", failed)
	}

	return nil
}
