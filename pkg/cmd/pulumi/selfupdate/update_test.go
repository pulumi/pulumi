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

package selfupdate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// release serves the subset of the release endpoints the updater reads: the latest version, a checksums file, and
// the archive itself. These tests must not run in parallel, since they repoint the package level URLs.
type release struct {
	version  semver.Version
	artifact string
	archive  []byte
	// corruptChecksum publishes a digest that does not match the archive, standing in for a tampered download.
	corruptChecksum bool
	// omitFromGitHub forces the download to fall back to the secondary host.
	omitFromGitHub bool
	// ignoreRange answers the whole archive even when a range was asked for, as some mirrors do.
	ignoreRange bool
	// ranges collects the Range headers the server was sent.
	ranges *[]string
}

func serveRelease(t *testing.T, r release) {
	t.Helper()

	sum := sha256.Sum256(r.archive)
	digest := hex.EncodeToString(sum[:])
	if r.corruptChecksum {
		digest = strings.Repeat("0", len(digest))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/latest-version", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, r.version.String())
	})
	mux.HandleFunc(fmt.Sprintf("/releases/v%s/pulumi-%s-checksums.txt", r.version, r.version),
		func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprintf(w, "%s  %s\n", digest, r.artifact)
		})
	// http.ServeContent answers Range requests, which is what makes an interrupted download resumable.
	serve := func(w http.ResponseWriter, req *http.Request) {
		if got := req.Header.Get("Range"); got != "" && r.ranges != nil {
			*r.ranges = append(*r.ranges, got)
		}
		http.ServeContent(w, req, r.artifact, time.Time{}, bytes.NewReader(r.archive))
	}

	mux.HandleFunc(fmt.Sprintf("/releases/v%s/%s", r.version, r.artifact),
		func(w http.ResponseWriter, req *http.Request) {
			if r.omitFromGitHub {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if r.ignoreRange {
				// Some mirrors answer the whole body regardless of what was asked for.
				_, _ = w.Write(r.archive)
				return
			}
			serve(w, req)
		})
	mux.HandleFunc("/cdn/"+r.artifact, serve)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	latestVersionURL, releaseBaseURL, fallbackBaseURL = srv.URL+"/latest-version", srv.URL+"/releases", srv.URL+"/cdn"
	t.Cleanup(func() {
		latestVersionURL = "https://www.pulumi.com/latest-version"
		releaseBaseURL = "https://github.com/pulumi/pulumi/releases/download"
		fallbackBaseURL = "https://get.pulumi.com/releases/sdk"
	})
}

// newRelease builds a release archive laid out the way the real ones are, holding the given binaries.
func newRelease(t *testing.T, v semver.Version, contents string) release {
	t.Helper()

	artifact, err := artifactName(v)
	require.NoError(t, err)

	dir := t.TempDir()
	path := filepath.Join(dir, artifact)
	files := map[string]string{}
	for _, name := range []string{"pulumi", "pulumi-language-nodejs", "pulumi-language-python"} {
		files["pulumi/"+name] = contents + " " + name
	}
	if strings.HasSuffix(artifact, ".zip") {
		writeZip(t, path, files)
	} else {
		writeTarball(t, path, files)
	}

	archive, err := os.ReadFile(path)
	require.NoError(t, err)
	return release{version: v, artifact: artifact, archive: archive}
}

//nolint:paralleltest // serveRelease repoints package level URLs, so these cannot run alongside each other
func TestUpdateInstallsTheLatestRelease(t *testing.T) {
	target := semver.MustParse("3.257.0")
	r := newRelease(t, target, "new")
	serveRelease(t, r)

	install := bundle(t, "old")
	var out bytes.Buffer

	err := update(t.Context(), &out, install, semver.MustParse("3.200.0"), "", false)

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Pulumi is now at v3.257.0.")
	for _, name := range []string{"pulumi", "pulumi-language-nodejs", "pulumi-language-python"} {
		contents, err := os.ReadFile(filepath.Join(install, name))
		require.NoError(t, err)
		assert.Equal(t, "new "+name, string(contents))
	}
}

//nolint:paralleltest // serveRelease repoints package level URLs, so these cannot run alongside each other
func TestUpdateRejectsAnArchiveThatFailsItsChecksum(t *testing.T) {
	target := semver.MustParse("3.257.0")
	r := newRelease(t, target, "tampered")
	r.corruptChecksum = true
	serveRelease(t, r)

	install := bundle(t, "old")
	var out bytes.Buffer

	err := update(t.Context(), &out, install, semver.MustParse("3.200.0"), "", false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")

	// Nothing may be replaced when the download cannot be vouched for.
	contents, readErr := os.ReadFile(filepath.Join(install, "pulumi"))
	require.NoError(t, readErr)
	assert.Equal(t, "old pulumi", string(contents))
}

//nolint:paralleltest // serveRelease repoints package level URLs, so these cannot run alongside each other
func TestUpdateFallsBackToTheSecondaryHost(t *testing.T) {
	target := semver.MustParse("3.257.0")
	r := newRelease(t, target, "new")
	r.omitFromGitHub = true
	serveRelease(t, r)

	install := bundle(t, "old")
	var out bytes.Buffer

	err := update(t.Context(), &out, install, semver.MustParse("3.200.0"), "", false)

	require.NoError(t, err)
	contents, err := os.ReadFile(filepath.Join(install, "pulumi"))
	require.NoError(t, err)
	assert.Equal(t, "new pulumi", string(contents))
}

//nolint:paralleltest // serveRelease repoints package level URLs, so these cannot run alongside each other
func TestUpdateDoesNothingWhenAlreadyCurrent(t *testing.T) {
	target := semver.MustParse("3.257.0")
	serveRelease(t, newRelease(t, target, "new"))

	install := bundle(t, "old")
	var out bytes.Buffer

	err := update(t.Context(), &out, install, target, "", false)

	require.NoError(t, err)
	assert.Contains(t, out.String(), "already up to date")
	contents, err := os.ReadFile(filepath.Join(install, "pulumi"))
	require.NoError(t, err)
	assert.Equal(t, "old pulumi", string(contents))
}

//nolint:paralleltest // serveRelease repoints package level URLs, so these cannot run alongside each other
func TestUpdateDryRunChangesNothing(t *testing.T) {
	target := semver.MustParse("3.257.0")
	serveRelease(t, newRelease(t, target, "new"))

	install := bundle(t, "old")
	var out bytes.Buffer

	err := update(t.Context(), &out, install, semver.MustParse("3.200.0"), "", true)

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Would update Pulumi v3.200.0 to v3.257.0")
	contents, err := os.ReadFile(filepath.Join(install, "pulumi"))
	require.NoError(t, err)
	assert.Equal(t, "old pulumi", string(contents))
}

//nolint:paralleltest // serveRelease repoints package level URLs, so these cannot run alongside each other
func TestUpdateClearsLeftoversFromAnEarlierRun(t *testing.T) {
	target := semver.MustParse("3.257.0")
	serveRelease(t, newRelease(t, target, "new"))

	install := bundle(t, "old")
	stale := filepath.Join(install, "pulumi"+staleSuffix+".999")
	require.NoError(t, os.WriteFile(stale, []byte("older"), 0o600))
	orphan := filepath.Join(filepath.Dir(install), stagingPrefix+"999")
	require.NoError(t, os.MkdirAll(orphan, 0o700))

	// Even a run with nothing to install should tidy up after the previous one.
	err := update(t.Context(), new(bytes.Buffer), install, target, "", false)

	require.NoError(t, err)
	assert.NoFileExists(t, stale)
	assert.NoDirExists(t, orphan)
}

//nolint:paralleltest // serveRelease repoints package level URLs, so these cannot run alongside each other
func TestUpdateInstallsAnExplicitlyRequestedVersion(t *testing.T) {
	target := semver.MustParse("3.250.0")
	serveRelease(t, newRelease(t, target, "pinned"))

	install := bundle(t, "old")

	err := update(t.Context(), new(bytes.Buffer), install, semver.MustParse("3.200.0"), "3.250.0", false)

	require.NoError(t, err)
	contents, err := os.ReadFile(filepath.Join(install, "pulumi"))
	require.NoError(t, err)
	assert.Equal(t, "pinned pulumi", string(contents))
}

//nolint:paralleltest // serveRelease repoints package level URLs, so these cannot run alongside each other
func TestUpdateReportsAnUnparseableRequestedVersion(t *testing.T) {
	serveRelease(t, newRelease(t, semver.MustParse("3.257.0"), "new"))

	err := update(t.Context(), new(bytes.Buffer), bundle(t, "old"), semver.MustParse("3.200.0"), "banana", false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `could not parse the requested version "banana"`)
}

//nolint:paralleltest // serveRelease repoints package level URLs, so these cannot run alongside each other
func TestUpdateReportsAVersionThatWasNeverPublished(t *testing.T) {
	serveRelease(t, newRelease(t, semver.MustParse("3.257.0"), "new"))

	err := update(t.Context(), new(bytes.Buffer), bundle(t, "old"), semver.MustParse("3.200.0"), "9.99.9", false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not fetch checksums for v9.99.9")
}

// partialDownload writes the first n bytes of the release archive where the next run will look for it, standing in
// for a download that was interrupted.
func partialDownload(t *testing.T, installDir string, r release, n int) string {
	t.Helper()

	dir := filepath.Join(filepath.Dir(installDir), downloadDirName)
	require.NoError(t, os.MkdirAll(dir, 0o700))
	path := filepath.Join(dir, r.artifact)
	require.NoError(t, os.WriteFile(path, r.archive[:n], 0o600))
	return path
}

//nolint:paralleltest // serveRelease repoints package level URLs, so these cannot run alongside each other
func TestUpdateResumesAnInterruptedDownload(t *testing.T) {
	target := semver.MustParse("3.257.0")
	r := newRelease(t, target, "new")
	var ranges []string
	r.ranges = &ranges
	serveRelease(t, r)

	install := bundle(t, "old")
	have := len(r.archive) / 2
	partialDownload(t, install, r, have)

	err := update(t.Context(), new(bytes.Buffer), install, semver.MustParse("3.200.0"), "", false)

	require.NoError(t, err)
	// Only the part that was missing should have been requested.
	require.Len(t, ranges, 1)
	assert.Equal(t, fmt.Sprintf("bytes=%d-", have), ranges[0])

	contents, err := os.ReadFile(filepath.Join(install, "pulumi"))
	require.NoError(t, err)
	assert.Equal(t, "new pulumi", string(contents))
}

//nolint:paralleltest // serveRelease repoints package level URLs, so these cannot run alongside each other
func TestUpdateStartsOverWhenTheServerIgnoresTheRange(t *testing.T) {
	target := semver.MustParse("3.257.0")
	r := newRelease(t, target, "new")
	r.ignoreRange = true
	serveRelease(t, r)

	install := bundle(t, "old")
	partialDownload(t, install, r, len(r.archive)/2)

	// Appending a whole archive onto a partial one would corrupt it, so the file has to be truncated first.
	err := update(t.Context(), new(bytes.Buffer), install, semver.MustParse("3.200.0"), "", false)

	require.NoError(t, err)
	contents, err := os.ReadFile(filepath.Join(install, "pulumi"))
	require.NoError(t, err)
	assert.Equal(t, "new pulumi", string(contents))
}

//nolint:paralleltest // serveRelease repoints package level URLs, so these cannot run alongside each other
func TestUpdateDiscardsAPartialDownloadThatDoesNotVerify(t *testing.T) {
	target := semver.MustParse("3.257.0")
	r := newRelease(t, target, "new")
	serveRelease(t, r)

	install := bundle(t, "old")
	// A partial file holding the wrong bytes resumes into an archive that cannot match the published checksum.
	path := partialDownload(t, install, r, len(r.archive)/2)
	require.NoError(t, os.WriteFile(path, bytes.Repeat([]byte("x"), len(r.archive)/2), 0o600))

	err := update(t.Context(), new(bytes.Buffer), install, semver.MustParse("3.200.0"), "", false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")
	// The bad archive must not be left behind for the next run to resume from.
	assert.NoFileExists(t, path)
}

//nolint:paralleltest // serveRelease repoints package level URLs, so these cannot run alongside each other
func TestUpdateDropsADownloadForAnotherVersion(t *testing.T) {
	target := semver.MustParse("3.257.0")
	r := newRelease(t, target, "new")
	serveRelease(t, r)

	install := bundle(t, "old")
	downloads := filepath.Join(filepath.Dir(install), downloadDirName)
	require.NoError(t, os.MkdirAll(downloads, 0o700))
	stale := filepath.Join(downloads, "pulumi-v3.100.0-linux-x64.tar.gz")
	require.NoError(t, os.WriteFile(stale, []byte("abandoned"), 0o600))

	err := update(t.Context(), new(bytes.Buffer), install, semver.MustParse("3.200.0"), "", false)

	require.NoError(t, err)
	assert.NoFileExists(t, stale)
}

//nolint:paralleltest // serveRelease repoints package level URLs, so these cannot run alongside each other
func TestUpdateRemovesTheArchiveOnceInstalled(t *testing.T) {
	target := semver.MustParse("3.257.0")
	r := newRelease(t, target, "new")
	serveRelease(t, r)

	install := bundle(t, "old")

	err := update(t.Context(), new(bytes.Buffer), install, semver.MustParse("3.200.0"), "", false)

	require.NoError(t, err)
	// Nothing is left to resume, so the archive and its directory should be gone.
	assert.NoDirExists(t, filepath.Join(filepath.Dir(install), downloadDirName))
}

func TestNewSelfUpdateCmdAcceptsItsFlags(t *testing.T) {
	t.Parallel()

	cmd := NewSelfUpdateCmd()

	assert.Equal(t, "self-update", cmd.Use)
	require.NoError(t, cmd.PersistentFlags().Parse([]string{"--version", "3.250.0", "--dry-run"}))
	assert.Equal(t, "3.250.0", cmd.PersistentFlags().Lookup("version").Value.String())
	assert.Equal(t, "true", cmd.PersistentFlags().Lookup("dry-run").Value.String())
	// The command takes no positional arguments.
	require.Error(t, cmd.Args(cmd, []string{"unexpected"}))
}

//nolint:paralleltest // serveRelease repoints package level URLs, so these cannot run alongside each other
func TestLatestVersionReadsThePublishedVersion(t *testing.T) {
	serveRelease(t, newRelease(t, semver.MustParse("3.257.0"), "new"))

	v, err := latestVersion(t.Context())

	require.NoError(t, err)
	assert.Equal(t, "3.257.0", v.String())
}
