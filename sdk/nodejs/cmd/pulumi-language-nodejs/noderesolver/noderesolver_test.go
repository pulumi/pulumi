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

package noderesolver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchiveFile(t *testing.T) {
	t.Parallel()
	spec := Spec{
		Version: "24.4.1",
		Checksums: map[string]string{
			"node-v24.4.1-linux-x64.tar.gz":    "dummy",
			"node-v24.4.1-linux-arm64.tar.gz":  "dummy",
			"node-v24.4.1-darwin-x64.tar.gz":   "dummy",
			"node-v24.4.1-darwin-arm64.tar.gz": "dummy",
			"node-v24.4.1-win-x64.zip":         "dummy",
		},
	}
	for _, tt := range []struct {
		goos, goarch, want string
	}{
		{"linux", "amd64", "node-v24.4.1-linux-x64.tar.gz"},
		{"linux", "arm64", "node-v24.4.1-linux-arm64.tar.gz"},
		{"darwin", "amd64", "node-v24.4.1-darwin-x64.tar.gz"},
		{"darwin", "arm64", "node-v24.4.1-darwin-arm64.tar.gz"},
		{"windows", "amd64", "node-v24.4.1-win-x64.zip"},
	} {
		got, err := archiveFile(spec, tt.goos, tt.goarch)
		require.NoError(t, err)
		assert.Equal(t, tt.want, got)
	}
	_, err := archiveFile(spec, "windows", "arm64")
	assert.Error(t, err)
}

func TestDefaultSpec(t *testing.T) {
	spec := Default()
	assert.Equal(t, PinnedVersion, spec.Version)
	assert.Equal(t, "https://nodejs.org/dist", spec.BaseURL)
	assert.NotEmpty(t, spec.Checksums)
	assert.False(t, spec.Disabled)

	t.Setenv("PULUMI_NODE_DOWNLOAD_URL", "https://mirror.example.com/node")
	assert.Equal(t, "https://mirror.example.com/node", Default().BaseURL)
}

func TestDefaultSpecDisabledFromEnv(t *testing.T) {
	t.Setenv("PULUMI_DISABLE_MANAGED_NODE", "true")
	assert.True(t, Default().Disabled)

	t.Setenv("PULUMI_DISABLE_MANAGED_NODE", "1")
	assert.True(t, Default().Disabled)

	t.Setenv("PULUMI_DISABLE_MANAGED_NODE", "false")
	assert.False(t, Default().Disabled)
}

func TestLayoutBinDir(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "/root/node-v24.4.1-linux-x64/bin",
		layoutBinDir("/root", "node-v24.4.1-linux-x64", "linux"))
	assert.Equal(t, "/root/node-v24.4.1-win-x64",
		layoutBinDir("/root", "node-v24.4.1-win-x64", "windows"))
}
