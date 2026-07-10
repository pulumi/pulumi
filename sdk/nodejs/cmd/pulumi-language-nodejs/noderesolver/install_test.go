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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeArchive returns a gzipped tarball laid out like a real Node distribution
// for the current platform, and its sha256.
func fakeArchive(t *testing.T, version string) ([]byte, string, string) {
	t.Helper()
	name, err := archiveFile(Spec{Version: version, Checksums: map[string]string{
		fmt.Sprintf("node-v%s-%s-%s.tar.gz", version, runtime.GOOS, mustArch(t)): "",
	}}, runtime.GOOS, runtime.GOARCH)
	require.NoError(t, err)
	base := name[:len(name)-len(".tar.gz")]

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	script := []byte("#!/bin/sh\necho fake-node\n")
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: base + "/bin/node", Mode: 0o755, Size: int64(len(script)), Typeflag: tar.TypeReg,
	}))
	_, err = tw.Write(script)
	require.NoError(t, err)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: base + "/bin/npm", Linkname: "node", Typeflag: tar.TypeSymlink,
	}))
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	sum := sha256.Sum256(buf.Bytes())
	return buf.Bytes(), hex.EncodeToString(sum[:]), name
}

func mustArch(t *testing.T) string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "arm64":
		return "arm64"
	}
	t.Skip("unsupported test arch")
	return ""
}

func testSpec(t *testing.T, home string) (Spec, *atomic.Int64) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("tarball fixture is unix-only; windows zip path covered by TestExtractZip")
	}
	archive, sum, name := fakeArchive(t, "1.0.0")
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		require.Equal(t, "/v1.0.0/"+name, r.URL.Path)
		_, _ = w.Write(archive)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PULUMI_HOME", home)
	return Spec{Version: "1.0.0", BaseURL: srv.URL, Checksums: map[string]string{name: sum}}, &hits
}

func TestResolveDownloadsWhenMissing(t *testing.T) {
	spec, hits := testSpec(t, t.TempDir())
	t.Setenv("PATH", t.TempDir()) // no ambient node

	res, err := ResolveWith(context.Background(), spec)
	require.NoError(t, err)
	assert.True(t, res.Managed)
	assert.FileExists(t, res.Node)
	fi, err := os.Lstat(res.Npm)
	require.NoError(t, err)
	assert.NotZero(t, fi.Mode()&os.ModeSymlink)
	assert.Equal(t, int64(1), hits.Load())

	// Second resolve: cache hit, no network.
	res2, err := ResolveWith(context.Background(), spec)
	require.NoError(t, err)
	assert.Equal(t, res.Node, res2.Node)
	assert.Equal(t, int64(1), hits.Load())
}

func TestResolveAmbientShortCircuits(t *testing.T) {
	spec, hits := testSpec(t, t.TempDir())
	bin := t.TempDir()
	for _, n := range []string{"node", "npm"} {
		require.NoError(t, os.WriteFile(filepath.Join(bin, n), []byte("#!/bin/sh\n"), 0o755))
	}
	t.Setenv("PATH", bin)

	res, err := ResolveWith(context.Background(), spec)
	require.NoError(t, err)
	assert.False(t, res.Managed)
	assert.Equal(t, filepath.Join(bin, "node"), res.Node)
	assert.Equal(t, int64(0), hits.Load())
}

func TestResolveChecksumMismatch(t *testing.T) {
	spec, _ := testSpec(t, t.TempDir())
	t.Setenv("PATH", t.TempDir())
	for k := range spec.Checksums {
		spec.Checksums[k] = "deadbeef"
	}
	_, err := ResolveWith(context.Background(), spec)
	require.ErrorContains(t, err, "checksum")
}

func TestResolveDisabled(t *testing.T) {
	spec, hits := testSpec(t, t.TempDir())
	t.Setenv("PATH", t.TempDir())
	spec.Disabled = true
	_, err := ResolveWith(context.Background(), spec)
	require.ErrorContains(t, err, "PATH")
	assert.Equal(t, int64(0), hits.Load())
}

func TestResolveEmitsDownloadMessageOnlyWhenDownloading(t *testing.T) {
	spec, hits := testSpec(t, t.TempDir())
	t.Setenv("PATH", t.TempDir())
	var buf bytes.Buffer
	spec.Output = &buf

	_, err := ResolveWith(context.Background(), spec)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Downloading Node.js v")
	assert.Equal(t, int64(1), hits.Load())

	buf.Reset()
	_, err = ResolveWith(context.Background(), spec)
	require.NoError(t, err)
	assert.Empty(t, buf.String())
	assert.Equal(t, int64(1), hits.Load())
}

func TestResolveConcurrent(t *testing.T) {
	spec, _ := testSpec(t, t.TempDir())
	t.Setenv("PATH", t.TempDir())
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, err := ResolveWith(context.Background(), spec)
			errs <- err
		}()
	}
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
}

func TestExtractZip(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	dirHdr := &zip.FileHeader{Name: "node/bin/"}
	dirHdr.SetMode(0o755 | os.ModeDir)
	_, err := zw.CreateHeader(dirHdr)
	require.NoError(t, err)

	fileHdr := &zip.FileHeader{Name: "node/bin/node.exe"}
	fileHdr.SetMode(0o755)
	fw, err := zw.CreateHeader(fileHdr)
	require.NoError(t, err)
	content := []byte("fake-node-exe")
	_, err = fw.Write(content)
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	dir := t.TempDir()
	require.NoError(t, extractZip(buf.Bytes(), dir))

	got, err := os.ReadFile(filepath.Join(dir, "node", "bin", "node.exe"))
	require.NoError(t, err)
	assert.Equal(t, content, got)

	fi, err := os.Stat(filepath.Join(dir, "node", "bin", "node.exe"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), fi.Mode().Perm())

	dirFi, err := os.Stat(filepath.Join(dir, "node", "bin"))
	require.NoError(t, err)
	assert.True(t, dirFi.IsDir())
}

func TestExtractZipRejectsPathTraversal(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	fw, err := zw.Create("../evil.txt")
	require.NoError(t, err)
	_, err = fw.Write([]byte("evil"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	dir := t.TempDir()
	err = extractZip(buf.Bytes(), dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes")
}
