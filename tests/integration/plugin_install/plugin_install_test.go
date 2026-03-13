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

package ints

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
)

// Test that plugin installation respects language runtime options
func TestPluginInstall(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		plugin string   // the plugin's name
		files  []string // files that should exist after installation
		dirs   []string // directories that should exist after installation
		output string   // expected output from the plugin when run
	}{
		{
			plugin: "python-uv",
			dirs:   []string{".venv"},
			files:  []string{"uv.lock"},
			output: "hello from python-uv",
		},
		{
			plugin: "nodejs-pnpm",
			dirs:   []string{"node_modules"},
			files:  []string{"pnpm-lock.yaml"},
			output: "hello from nodejs-pnpm",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.plugin, func(t *testing.T) {
			t.Parallel()
			e := ptesting.NewEnvironment(t)
			defer e.DeleteIfNotFailed()
			t.Logf("home = %s", e.HomePath)

			pluginPath, err := filepath.Abs(filepath.Join(tc.plugin))
			require.NoError(t, err)
			stdout, stderr := e.RunCommand("pulumi", "plugin", "install", "tool", tc.plugin, "--file", pluginPath, "0.0.1")
			t.Logf("stdout = %s", stdout)
			t.Logf("stderr = %s", stderr)
			pluginDir := fmt.Sprintf("tool-%s-v0.0.1", tc.plugin)
			for _, f := range tc.dirs {
				require.DirExists(t, filepath.Join(e.HomePath, "plugins", pluginDir, f))
			}
			for _, f := range tc.files {
				require.FileExists(t, filepath.Join(e.HomePath, "plugins", pluginDir, f))
			}
			stdout, stderr = e.RunCommand("pulumi", "plugin", "run", tc.plugin, "--kind", "tool")
			require.Contains(t, stdout, tc.output, "stdout = %s, stderr = %s", stdout, stderr)
		})
	}
}
