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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

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

func testPluginInstall(t *testing.T, expectedDir string, files map[string][]byte) {
	// Skip during short test runs since this test involves downloading dependencies.
	if testing.Short() {
		t.Skip("Skipped in short test run")
	}

	tgz, err := createTGZ(files)
	assert.NoError(t, err)

	finalDirRoot, err := ioutil.TempDir("", "final-dir")
	assert.NoError(t, err)
	defer os.RemoveAll(finalDirRoot)
	finalDir := filepath.Join(finalDirRoot, "final")

	err = installPlugin(finalDir, ioutil.NopCloser(bytes.NewReader(tgz)))
	assert.NoError(t, err)

	info, err := os.Stat(filepath.Join(finalDir, expectedDir))
	assert.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestInstallNoDeps(t *testing.T) {
	name := "foo.txt"
	content := []byte("hello\n")

	tgz, err := createTGZ(map[string][]byte{name: content})
	assert.NoError(t, err)

	finalDirRoot, err := ioutil.TempDir("", "final-dir")
	assert.NoError(t, err)
	defer os.RemoveAll(finalDirRoot)
	finalDir := filepath.Join(finalDirRoot, "final")

	err = installPlugin(finalDir, ioutil.NopCloser(bytes.NewReader(tgz)))
	assert.NoError(t, err)

	info, err := os.Stat(filepath.Join(finalDir, name))
	assert.NoError(t, err)
	assert.False(t, info.IsDir())

	b, err := ioutil.ReadFile(filepath.Join(finalDir, name))
	assert.NoError(t, err)
	assert.Equal(t, content, b)
}
