// Copyright 2016-2020, Pulumi Corporation.
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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTGZ creates an in-memory tarball.
func createTGZ(files map[string][]byte) ([]byte, error) {
	buffer := &bytes.Buffer{}
	gw := gzip.NewWriter(buffer)
	writer := tar.NewWriter(gw)

	for name, content := range files {
		if err := writer.WriteHeader(&tar.Header{
			Name: name,
			Size: int64(len(content)),
			Mode: 0o600,
		}); err != nil {
			return nil, err
		}
		if _, err := writer.Write(content); err != nil {
			return nil, err
		}
	}

	// Close the tar and gzip writers to flush and write footers.
	if err := writer.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func prepareTestPluginTGZ(t *testing.T, files map[string][]byte) io.ReadCloser {
	if files == nil {
		files = map[string][]byte{}
	}

	// Add plugin binary to included files.
	if runtime.GOOS == "windows" {
		files["pulumi-resource-test.exe"] = nil
	} else {
		files["pulumi-resource-test"] = nil
	}

	tgz, err := createTGZ(files)
	require.NoError(t, err)
	return io.NopCloser(bytes.NewReader(tgz))
}

func prepareTestDir(t *testing.T, files map[string][]byte) (string, io.ReadCloser, PluginSpec) {
	tarball := prepareTestPluginTGZ(t, files)

	dir := t.TempDir()

	v1 := semver.MustParse("0.1.0")
	plugin := PluginSpec{
		Name:      "test",
		Kind:      apitype.ResourcePlugin,
		Version:   &v1,
		PluginDir: dir,
	}

	return dir, tarball, plugin
}

func assertPluginInstalled(t *testing.T, dir string, plugin PluginSpec) PluginInfo {
	info, err := os.Stat(filepath.Join(dir, plugin.Dir()))
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	file := filepath.Join(dir, plugin.Dir(), plugin.File())
	if runtime.GOOS == "windows" {
		file += ".exe"
	}
	info, err = os.Stat(file)
	require.NoError(t, err)
	assert.False(t, info.IsDir())

	_, err = os.Stat(filepath.Join(dir, plugin.Dir()+".partial"))
	assert.Truef(t, os.IsNotExist(err), "err was not IsNotExists, but was %s", err)

	assert.True(t, HasPlugin(plugin))

	has, err := HasPluginGTE(plugin)
	require.NoError(t, err)
	assert.True(t, has)

	skipMetadata := true
	plugins, err := getPlugins(dir, skipMetadata)
	require.NoError(t, err)
	require.Equal(t, 1, len(plugins))
	assert.Equal(t, plugin.Name, plugins[0].Name)
	assert.Equal(t, plugin.Kind, plugins[0].Kind)
	assert.Equal(t, *plugin.Version, *plugins[0].Version)
	return plugins[0]
}

func testDeletePlugin(t *testing.T, plugin PluginInfo) {
	paths := []string{
		plugin.Path,
		plugin.Path + ".partial",
		plugin.Path + ".lock",
	}
	anyPresent := false
	for _, path := range paths {
		_, err := os.Stat(path)
		if !os.IsNotExist(err) {
			anyPresent = true
		}
	}
	assert.True(t, anyPresent, "None of the expected plugin files were present before Delete")

	err := plugin.Delete()
	require.NoError(t, err)

	for _, path := range paths {
		_, err := os.Stat(path)
		assert.Truef(t, os.IsNotExist(err), "err was not IsNotExists, but was %s", err)
	}
}

func testPluginInstall(t *testing.T, expectedDir string, files map[string][]byte) {
	// Skip during short test runs since this test involves downloading dependencies.
	if testing.Short() {
		t.Skip("Skipped in short test run")
	}

	dir, tarball, plugin := prepareTestDir(t, files)

	err := plugin.Install(tarball, false)
	require.NoError(t, err)

	pluginInfo := assertPluginInstalled(t, dir, plugin)

	info, err := os.Stat(filepath.Join(dir, plugin.Dir(), expectedDir))
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	testDeletePlugin(t, pluginInfo)
}

func TestInstallNoDeps(t *testing.T) {
	t.Parallel()

	name := "foo.txt"
	content := []byte("hello\n")

	dir, tarball, plugin := prepareTestDir(t, map[string][]byte{name: content})

	err := plugin.Install(tarball, false)
	require.NoError(t, err)

	pluginInfo := assertPluginInstalled(t, dir, plugin)

	b, err := os.ReadFile(filepath.Join(dir, plugin.Dir(), name))
	require.NoError(t, err)
	assert.Equal(t, content, b)

	testDeletePlugin(t, pluginInfo)
}

func TestReinstall(t *testing.T) {
	t.Parallel()

	name := "foo.txt"
	content := []byte("hello\n")

	dir, tarball, plugin := prepareTestDir(t, map[string][]byte{name: content})

	err := plugin.Install(tarball, false)
	require.NoError(t, err)

	assertPluginInstalled(t, dir, plugin)

	b, err := os.ReadFile(filepath.Join(dir, plugin.Dir(), name))
	require.NoError(t, err)
	assert.Equal(t, content, b)

	content = []byte("world\n")
	tarball = prepareTestPluginTGZ(t, map[string][]byte{name: content})

	err = plugin.Install(tarball, true)
	require.NoError(t, err)

	pluginInfo := assertPluginInstalled(t, dir, plugin)

	b, err = os.ReadFile(filepath.Join(dir, plugin.Dir(), name))
	require.NoError(t, err)
	assert.Equal(t, content, b)

	testDeletePlugin(t, pluginInfo)
}

func TestConcurrentInstalls(t *testing.T) {
	t.Parallel()

	name := "foo.txt"
	content := []byte("hello\n")

	dir, tarball, plugin := prepareTestDir(t, map[string][]byte{name: content})

	assertSuccess := func() PluginInfo {
		pluginInfo := assertPluginInstalled(t, dir, plugin)

		b, err := os.ReadFile(filepath.Join(dir, plugin.Dir(), name))
		require.NoError(t, err)
		assert.Equal(t, content, b)

		return pluginInfo
	}

	// Run several installs concurrently.
	const iterations = 12
	var wg sync.WaitGroup
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			err := plugin.Install(tarball, false)
			require.NoError(t, err)

			assertSuccess()
		}()
	}
	wg.Wait()

	pluginInfo := assertSuccess()

	testDeletePlugin(t, pluginInfo)
}

func TestInstallCleansOldFiles(t *testing.T) {
	t.Parallel()

	dir, tarball, plugin := prepareTestDir(t, nil)

	// Leftover temp dirs.
	//nolint:usetesting // Need to use a specific location for the tmp dir
	tempDir1, err := os.MkdirTemp(dir, plugin.Dir()+".tmp")
	require.NoError(t, err)
	//nolint:usetesting // Need to use a specific location for the tmp dir
	tempDir2, err := os.MkdirTemp(dir, plugin.Dir()+".tmp")
	require.NoError(t, err)
	//nolint:usetesting // Need to use a specific location for the tmp dir
	tempDir3, err := os.MkdirTemp(dir, plugin.Dir()+".tmp")
	require.NoError(t, err)

	// Leftover partial file.
	partialPath := filepath.Join(dir, plugin.Dir()+".partial")
	err = os.WriteFile(partialPath, nil, 0o600)
	require.NoError(t, err)

	err = plugin.Install(tarball, false)
	require.NoError(t, err)

	pluginInfo := assertPluginInstalled(t, dir, plugin)

	// Verify leftover files were removed.
	for _, path := range []string{tempDir1, tempDir2, tempDir3, partialPath} {
		_, err := os.Stat(path)
		assert.Truef(t, os.IsNotExist(err), "err was not IsNotExists, but was %s", err)
	}

	testDeletePlugin(t, pluginInfo)
}

func TestGetPluginsSkipsPartial(t *testing.T) {
	t.Parallel()

	dir, tarball, plugin := prepareTestDir(t, nil)

	err := plugin.Install(tarball, false)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(dir, plugin.Dir()+".partial"), nil, 0o600)
	require.NoError(t, err)

	assert.False(t, HasPlugin(plugin))

	has, err := HasPluginGTE(plugin)
	require.NoError(t, err)
	assert.False(t, has)

	skipMetadata := true
	plugins, err := getPlugins(dir, skipMetadata)
	require.NoError(t, err)
	assert.Equal(t, 0, len(plugins))
}
