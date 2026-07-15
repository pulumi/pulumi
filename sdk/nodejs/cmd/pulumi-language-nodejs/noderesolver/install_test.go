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
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeArchive builds a minimal Node distribution archive for the current
// platform: <base>/bin/node on unix, <base>/node.exe on Windows.
func makeArchive(t *testing.T, base string) []byte {
	t.Helper()
	var buf bytes.Buffer
	if runtime.GOOS == "windows" {
		zw := zip.NewWriter(&buf)
		for _, name := range []string{base + "/node.exe", base + "/npm.cmd"} {
			w, err := zw.Create(name)
			require.NoError(t, err)
			_, err = w.Write([]byte("fake"))
			require.NoError(t, err)
		}
		require.NoError(t, zw.Close())
		return buf.Bytes()
	}
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, name := range []string{base + "/bin/node", base + "/bin/npm"} {
		content := []byte("#!/bin/sh\nexit 0\n")
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name: name, Mode: 0o755, Size: int64(len(content)),
		}))
		_, err := tw.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

// serveDist serves /v<version>/<archive>, counting archive downloads.
func serveDist(t *testing.T, version string, archive []byte, name string, downloads *atomic.Int32) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v"+version+"/"+name, func(w http.ResponseWriter, r *http.Request) {
		downloads.Add(1)
		_, _ = w.Write(archive)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func testSpec(t *testing.T, version string) (Spec, string, *atomic.Int32) {
	t.Helper()
	t.Setenv("PULUMI_HOME", t.TempDir())
	name, err := archiveName(version, runtime.GOOS, runtime.GOARCH)
	require.NoError(t, err)
	archive := makeArchive(t, archiveBase(name))
	var downloads atomic.Int32
	srv := serveDist(t, version, archive, name, &downloads)
	return Spec{Version: version, BaseURL: srv.URL}, name, &downloads
}

//nolint:paralleltest // testSpec uses t.Setenv
func TestResolveDownloadsAndCaches(t *testing.T) {
	spec, _, downloads := testSpec(t, "9.9.9")
	var out bytes.Buffer
	spec.Output = &out

	res, err := Resolve(t.Context(), spec)
	require.NoError(t, err)
	assert.FileExists(t, res.Node)
	assert.Contains(t, out.String(), "9.9.9")
	assert.Equal(t, int32(1), downloads.Load())

	out.Reset()
	res2, err := Resolve(t.Context(), spec)
	require.NoError(t, err)
	assert.Equal(t, res, res2)
	assert.Empty(t, out.String(), "cache hit must be silent")
	assert.Equal(t, int32(1), downloads.Load(), "cache hit must not re-download")
}

func TestResolveDownloadNotFound(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	_, err := Resolve(t.Context(), Spec{Version: "9.9.9", BaseURL: srv.URL})
	assert.ErrorContains(t, err, "HTTP 404")
}

//nolint:paralleltest // testSpec uses t.Setenv
func TestResolveConcurrent(t *testing.T) {
	spec, _, downloads := testSpec(t, "9.9.9")

	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = Resolve(t.Context(), spec)
		}()
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
	assert.Equal(t, int32(1), downloads.Load(), "concurrent resolves must download once")
}

//nolint:paralleltest // testSpec uses t.Setenv
func TestResolveRepairsCorruptedCache(t *testing.T) {
	spec, name, downloads := testSpec(t, "9.9.9")

	res, err := Resolve(t.Context(), spec)
	require.NoError(t, err)
	require.NoError(t, os.Remove(res.Node))

	_ = name
	res2, err := Resolve(t.Context(), spec)
	require.NoError(t, err)
	assert.FileExists(t, res2.Node)
	assert.Equal(t, int32(2), downloads.Load())
}

// TestResolveRepairsPartialInstall ensures a directory whose .partial marker
// was left behind by an install that died mid-extract is replaced, even when
// the binaries it did manage to extract look healthy.
//
//nolint:paralleltest // testSpec uses t.Setenv
func TestResolveRepairsPartialInstall(t *testing.T) {
	spec, name, downloads := testSpec(t, "9.9.9")

	_, err := Resolve(t.Context(), spec)
	require.NoError(t, err)
	root, err := workspace.GetPulumiPath("node", archiveBase(name))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(root+".partial", nil, 0o600))

	res, err := Resolve(t.Context(), spec)
	require.NoError(t, err)
	assert.FileExists(t, res.Node)
	assert.NoFileExists(t, root+".partial")
	assert.Equal(t, int32(2), downloads.Load())
}

// TestResolveRepairsDanglingNpm ensures a cache entry with a healthy node
// binary but a missing npm binary is treated as corrupted and re-downloaded,
// rather than being cached forever with a permanently broken npm.
//
//nolint:paralleltest // testSpec uses t.Setenv
func TestResolveRepairsDanglingNpm(t *testing.T) {
	spec, name, downloads := testSpec(t, "9.9.9")

	res, err := Resolve(t.Context(), spec)
	require.NoError(t, err)
	require.NoError(t, os.Remove(res.Npm))

	_ = name
	res2, err := Resolve(t.Context(), spec)
	require.NoError(t, err)
	assert.FileExists(t, res2.Node)
	assert.FileExists(t, res2.Npm)
	assert.Equal(t, int32(2), downloads.Load())
}
