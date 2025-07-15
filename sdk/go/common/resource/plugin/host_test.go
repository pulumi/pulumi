// Copyright 2016-2024, Pulumi Corporation.
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
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClosePanic(t *testing.T) {
	t.Parallel()

	sink := diagtest.LogSink(t)
	ctx, err := NewContext(context.Background(), sink, sink, nil, nil, "", nil, false, nil)
	require.NoError(t, err)
	host, ok := ctx.Host.(*defaultHost)
	require.True(t, ok)

	// Spin up a load of loadPlugin calls and then Close the context. This should not panic.
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// We expect some of these to error that the host is shutting down, that's fine this test is just
			// checking nothing panics.
			_, _ = host.loadPlugin(host.loadRequests, func() (interface{}, error) {
				return nil, nil
			})
		}()
	}
	err = host.Close()
	require.NoError(t, err)

	wg.Wait()
}

func TestIsLocalPluginPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "explicit relative path with ./",
			path:     "./my-plugin",
			expected: true,
		},
		{
			name:     "explicit relative path with ../",
			path:     "../my-plugin",
			expected: true,
		},
		{
			name:     "absolute path",
			path:     "/path/to/my-plugin",
			expected: true,
		},
		{
			name:     "windows absolute path",
			path:     "C:\\path\\to\\my-plugin",
			expected: true, // This will be true because it doesn't match plugin name regexp
		},
		{
			name:     "standard plugin name",
			path:     "aws",
			expected: false, // Standard plugin names match the regexp
		},
		{
			name:     "standard plugin name with version",
			path:     "aws@v4.0.0",
			expected: false,
		},
		{
			name:     "git URL",
			path:     "git://github.com/pulumi/pulumi-aws",
			expected: false,
		},
		{
			name:     "github URL",
			path:     "github.com/pulumi/pulumi-aws",
			expected: false,
		},
		{
			name:     "github HTTPS URL",
			path:     "https://github.com/pulumi/pulumi-aws",
			expected: false,
		},
		{
			name:     "plugin name",
			path:     "my-provider",
			expected: false,
		},
		{
			name:     "local path that looks like a plugin name",
			path:     "_my_local_path", // Doesn't match plugin name regexp
			expected: true,
		},
		{
			name:     "empty string",
			path:     "", // Can't be a valid plugin name
			expected: true,
		},
		{
			name:     "private github URL",
			path:     "github.com/pulumi/home",
			expected: false,
		},
		{
			name:     "non-existent repo URL",
			path:     "example.com/no-repo-exists/here",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := IsLocalPluginPath(ctx, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewDefaultHost_PackagesResolution(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for our test
	tempDir := t.TempDir()

	// Create a subdirectory to use as a local plugin path
	localPluginDir := filepath.Join(tempDir, "local-plugin")
	err := os.Mkdir(localPluginDir, 0o755)
	require.NoError(t, err)

	// Create another subdirectory to use as a relative plugin path
	relativePluginDir := filepath.Join(tempDir, "relative-path")
	err = os.Mkdir(relativePluginDir, 0o755)
	require.NoError(t, err)

	// Create a context for testing
	ctx := &Context{
		Root: tempDir,
		Diag: diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{
			Color: colors.Never,
		}),
	}

	// Create packages map with various types of sources
	packages := map[string]workspace.PackageSpec{
		"local-plugin":    {Source: localPluginDir},
		"relative-plugin": {Source: "./relative-path"},
		"aws":             {Source: "aws"},                                // This should be skipped as it's not a local path
		"azure":           {Source: "azure@v4.0.0"},                       // This should be skipped as it's not a local path
		"git-plugin":      {Source: "git://github.com/pulumi/pulumi-aws"}, // This should be skipped
	}

	// Create the host with our packages
	host, err := NewDefaultHost(ctx, nil, false, nil, packages, nil, nil, "")
	require.NoError(t, err)
	defer host.Close()

	// Get the project plugins
	projectPlugins := host.GetProjectPlugins()

	// We should have 2 plugins (local-plugin and relative-plugin)
	assert.Equal(t, 2, len(projectPlugins))

	// Create a map of plugin names to paths for easier verification
	pluginMap := make(map[string]string)
	for _, plugin := range projectPlugins {
		pluginMap[plugin.Name] = plugin.Path
	}

	// Verify the expected plugins are present with correct paths
	assert.Contains(t, pluginMap, "local-plugin")
	assert.Contains(t, pluginMap, "relative-plugin")
	assert.Equal(t, localPluginDir, pluginMap["local-plugin"])
	assert.Equal(t, filepath.Join(tempDir, "relative-path"), pluginMap["relative-plugin"])

	// Verify the unexpected plugins are not present
	assert.NotContains(t, pluginMap, "aws")
	assert.NotContains(t, pluginMap, "azure")
	assert.NotContains(t, pluginMap, "git-plugin")
}

// TestNewDefaultHost_BothPluginsAndPackages tests the combined resolution of plugins and packages
func TestNewDefaultHost_BothPluginsAndPackages(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for our test
	tempDir := t.TempDir()

	// Create a subdirectory to use as a local plugin path
	localPluginDir := filepath.Join(tempDir, "local-plugin")
	err := os.Mkdir(localPluginDir, 0o755)
	require.NoError(t, err)

	awsProviderDir := filepath.Join(tempDir, "aws-provider")
	err = os.Mkdir(awsProviderDir, 0o755)
	require.NoError(t, err)

	// Create a context for testing
	ctx := &Context{
		Root: tempDir,
		Diag: diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{
			Color: colors.Never,
		}),
	}

	// Create test plugins
	plugins := &workspace.Plugins{
		Providers: []workspace.PluginOptions{
			{Name: "aws", Path: "./aws-provider"},
		},
	}

	// Create test packages
	packages := map[string]workspace.PackageSpec{
		"local-plugin": {Source: localPluginDir},
		"azure":        {Source: "azure"}, // This should be skipped as it's not a local path
	}

	host, err := NewDefaultHost(ctx, nil, false, plugins, packages, nil, nil, "")
	require.NoError(t, err)
	defer host.Close()

	projectPlugins := host.GetProjectPlugins()

	// We should have 2 plugins (1 from plugins, 1 from packages)
	assert.Equal(t, 2, len(projectPlugins))

	// Check that all expected plugins are present
	pluginNames := map[string]bool{}
	for _, plugin := range projectPlugins {
		pluginNames[plugin.Name] = true
	}

	assert.True(t, pluginNames["aws"])
	assert.True(t, pluginNames["local-plugin"])
	assert.False(t, pluginNames["azure"])
}
