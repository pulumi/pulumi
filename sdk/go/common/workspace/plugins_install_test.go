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

// +build nodejs python all

package workspace

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
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
			Mode: 0600,
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

func prepareTestDir(t *testing.T, files map[string][]byte) (string, io.ReadCloser, PluginInfo) {
	if files == nil {
		files = map[string][]byte{}
	}

	// Add plugin binary to included files.
	files["pulumi-resource-test"] = nil

	tgz, err := createTGZ(files)
	assert.NoError(t, err)
	tarball := ioutil.NopCloser(bytes.NewReader(tgz))

	dir, err := ioutil.TempDir("", "plugins-test-dir")
	assert.NoError(t, err)

	v1 := semver.MustParse("0.1.0")
	plugin := PluginInfo{
		Name:      "test",
		Kind:      ResourcePlugin,
		Version:   &v1,
		PluginDir: dir,
	}

	return dir, tarball, plugin
}

func assertPluginInstalled(t *testing.T, dir string, plugin PluginInfo) {
	info, err := os.Stat(filepath.Join(dir, plugin.Dir()))
	assert.NoError(t, err)
	assert.True(t, info.IsDir())

	info, err = os.Stat(filepath.Join(dir, plugin.Dir(), plugin.File()))
	assert.NoError(t, err)
	assert.False(t, info.IsDir())

	info, err = os.Stat(filepath.Join(dir, plugin.Dir()+".partial"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	assert.True(t, HasPlugin(plugin))

	has, err := HasPluginGTE(plugin)
	assert.NoError(t, err)
	assert.True(t, has)

	skipMetadata := true
	plugins, err := getPlugins(dir, skipMetadata)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(plugins))
	assert.Equal(t, plugin.Name, plugins[0].Name)
	assert.Equal(t, plugin.Kind, plugins[0].Kind)
	assert.Equal(t, *plugin.Version, *plugins[0].Version)
}

func testDeletePlugin(t *testing.T, dir string, plugin PluginInfo) {
	err := plugin.Delete()
	assert.NoError(t, err)

	paths := []string{
		filepath.Join(dir, plugin.Dir()),
		filepath.Join(dir, plugin.Dir()+".partial"),
		filepath.Join(dir, plugin.Dir()+".lock"),
	}
	for _, path := range paths {
		_, err := os.Stat(path)
		assert.Error(t, err)
		assert.True(t, os.IsNotExist(err))
	}
}

func testPluginInstall(t *testing.T, expectedDir string, files map[string][]byte) {
	// Skip during short test runs since this test involves downloading dependencies.
	if testing.Short() {
		t.Skip("Skipped in short test run")
	}

	dir, tarball, plugin := prepareTestDir(t, files)
	defer os.RemoveAll(dir)

	err := plugin.Install(tarball)
	assert.NoError(t, err)

	assertPluginInstalled(t, dir, plugin)

	info, err := os.Stat(filepath.Join(dir, plugin.Dir(), expectedDir))
	assert.NoError(t, err)
	assert.True(t, info.IsDir())

	testDeletePlugin(t, dir, plugin)
}

func TestInstallNoDeps(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipped on Windows: issues with TEMP dir")
	}

	name := "foo.txt"
	content := []byte("hello\n")

	dir, tarball, plugin := prepareTestDir(t, map[string][]byte{name: content})
	defer os.RemoveAll(dir)

	err := plugin.Install(tarball)
	assert.NoError(t, err)

	assertPluginInstalled(t, dir, plugin)

	b, err := ioutil.ReadFile(filepath.Join(dir, plugin.Dir(), name))
	assert.NoError(t, err)
	assert.Equal(t, content, b)

	testDeletePlugin(t, dir, plugin)
}

func TestConcurrentInstalls(t *testing.T) {
	name := "foo.txt"
	content := []byte("hello\n")

	dir, tarball, plugin := prepareTestDir(t, map[string][]byte{name: content})
	defer os.RemoveAll(dir)

	assertSuccess := func() {
		assertPluginInstalled(t, dir, plugin)

		b, err := ioutil.ReadFile(filepath.Join(dir, plugin.Dir(), name))
		assert.NoError(t, err)
		assert.Equal(t, content, b)
	}

	// Run several installs concurrently.
	const iterations = 12
	var wg sync.WaitGroup
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			err := plugin.Install(tarball)
			assert.NoError(t, err)

			assertSuccess()
		}()
	}
	wg.Wait()

	assertSuccess()

	testDeletePlugin(t, dir, plugin)
}

func TestInstallCleansOldFiles(t *testing.T) {
	dir, tarball, plugin := prepareTestDir(t, nil)
	defer os.RemoveAll(dir)

	// Leftover temp dirs.
	tempDir1, err := ioutil.TempDir(dir, fmt.Sprintf("%s.tmp", plugin.Dir()))
	assert.NoError(t, err)
	tempDir2, err := ioutil.TempDir(dir, fmt.Sprintf("%s.tmp", plugin.Dir()))
	assert.NoError(t, err)
	tempDir3, err := ioutil.TempDir(dir, fmt.Sprintf("%s.tmp", plugin.Dir()))
	assert.NoError(t, err)

	// Leftover partial file.
	partialPath := filepath.Join(dir, plugin.Dir()+".partial")
	err = ioutil.WriteFile(partialPath, nil, 0600)
	assert.NoError(t, err)

	err = plugin.Install(tarball)
	assert.NoError(t, err)

	assertPluginInstalled(t, dir, plugin)

	// Verify leftover files were removed.
	for _, path := range []string{tempDir1, tempDir2, tempDir3, partialPath} {
		_, err := os.Stat(path)
		assert.Error(t, err)
		assert.True(t, os.IsNotExist(err))
	}

	testDeletePlugin(t, dir, plugin)
}

func TestGetPluginsSkipsPartial(t *testing.T) {
	dir, tarball, plugin := prepareTestDir(t, nil)
	defer os.RemoveAll(dir)

	err := plugin.Install(tarball)
	assert.NoError(t, err)

	err = ioutil.WriteFile(filepath.Join(dir, plugin.Dir()+".partial"), nil, 0600)
	assert.NoError(t, err)

	assert.False(t, HasPlugin(plugin))

	has, err := HasPluginGTE(plugin)
	assert.Error(t, err)
	assert.False(t, has)

	skipMetadata := true
	plugins, err := getPlugins(dir, skipMetadata)
	assert.Equal(t, 0, len(plugins))
}
