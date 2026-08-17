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

package pluginstorage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func testPluginSpec(t *testing.T) workspace.PluginDescriptor {
	t.Setenv("PULUMI_HOME", t.TempDir())
	version := semver.MustParse("1.0.0")
	return workspace.PluginDescriptor{
		Name:    "test",
		Kind:    apitype.ResourcePlugin,
		Version: &version,
	}
}

func testPluginContent(t *testing.T, contents string) Content {
	src := t.TempDir()
	// The plugin binary must be executable.
	require.NoError(t, os.WriteFile(filepath.Join(src, "pulumi-resource-test"), []byte(contents), 0o700)) //nolint:gosec
	return DirPlugin(src)
}

func binaryContents(t *testing.T, spec workspace.PluginDescriptor) string {
	dir, err := spec.DirPath()
	require.NoError(t, err)
	contents, err := os.ReadFile(filepath.Join(dir, "pulumi-resource-test"))
	require.NoError(t, err)
	return string(contents)
}

//nolint:paralleltest // t.Setenv is incompatible with t.Parallel
func TestUnpackContentsFreshInstall(t *testing.T) {
	spec := testPluginSpec(t)

	cleanup, err := UnpackContents(t.Context(), spec, testPluginContent(t, "new"), false)
	require.NoError(t, err)
	cleanup(true)

	assert.Equal(t, "new", binaryContents(t, spec))
	assert.True(t, workspace.HasPlugin(spec))
}

//nolint:paralleltest // t.Setenv is incompatible with t.Parallel
func TestUnpackContentsKeepsCompleteInstall(t *testing.T) {
	spec := testPluginSpec(t)

	cleanup, err := UnpackContents(t.Context(), spec, testPluginContent(t, "original"), false)
	require.NoError(t, err)
	cleanup(true)

	cleanup, err = UnpackContents(t.Context(), spec, testPluginContent(t, "new"), false)
	require.NoError(t, err)
	cleanup(true)

	assert.Equal(t, "original", binaryContents(t, spec))
	assert.True(t, workspace.HasPlugin(spec))
}

//nolint:paralleltest // t.Setenv is incompatible with t.Parallel
func TestUnpackContentsReinstallOverwrites(t *testing.T) {
	spec := testPluginSpec(t)

	cleanup, err := UnpackContents(t.Context(), spec, testPluginContent(t, "original"), false)
	require.NoError(t, err)
	cleanup(true)

	cleanup, err = UnpackContents(t.Context(), spec, testPluginContent(t, "new"), true)
	require.NoError(t, err)
	cleanup(true)

	assert.Equal(t, "new", binaryContents(t, spec))
	assert.True(t, workspace.HasPlugin(spec))
}

//nolint:paralleltest // t.Setenv is incompatible with t.Parallel
func TestUnpackContentsRecoversFailedInstall(t *testing.T) {
	spec := testPluginSpec(t)

	cleanup, err := UnpackContents(t.Context(), spec, testPluginContent(t, "broken"), false)
	require.NoError(t, err)
	cleanup(false)

	assert.False(t, workspace.HasPlugin(spec))

	cleanup, err = UnpackContents(t.Context(), spec, testPluginContent(t, "new"), false)
	require.NoError(t, err)
	cleanup(true)

	assert.Equal(t, "new", binaryContents(t, spec))
	assert.True(t, workspace.HasPlugin(spec))
}
