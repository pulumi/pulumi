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
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/httputil"
)

// These are variables rather than constants so that tests can point them at a local server.
var (
	// latestVersionURL reports the newest released CLI version. The install scripts read the same endpoint, and
	// third parties such as the GitHub Actions runner images depend on it, so it is effectively a stable API.
	latestVersionURL = "https://www.pulumi.com/latest-version"

	// releaseBaseURL and fallbackBaseURL mirror the order the install scripts try: GitHub first, then the CDN.
	releaseBaseURL  = "https://github.com/pulumi/pulumi/releases/download"
	fallbackBaseURL = "https://get.pulumi.com/releases/sdk"
)

// latestVersion fetches the newest released version of the CLI.
func latestVersion(ctx context.Context) (semver.Version, error) {
	body, err := get(ctx, latestVersionURL)
	if err != nil {
		return semver.Version{}, fmt.Errorf("could not determine the latest Pulumi version: %w", err)
	}
	defer contract.IgnoreClose(body)

	raw, err := io.ReadAll(body)
	if err != nil {
		return semver.Version{}, err
	}

	v, err := semver.ParseTolerant(strings.TrimSpace(string(raw)))
	if err != nil {
		return semver.Version{}, fmt.Errorf("could not parse the latest Pulumi version %q: %w", raw, err)
	}
	return v, nil
}

// artifactName returns the name of the release archive for the running platform.
func artifactName(v semver.Version) (string, error) {
	return artifactNameFor(v, runtime.GOOS, runtime.GOARCH)
}

// artifactNameFor builds a release archive name for an explicit platform. Releases label x86-64 as x64 rather than
// the amd64 Go reports, and ship a zip rather than a tarball for Windows.
func artifactNameFor(v semver.Version, goos, goarch string) (string, error) {
	var arch string
	switch goarch {
	case "amd64":
		arch = "x64"
	case "arm64":
		arch = "arm64"
	default:
		return "", fmt.Errorf("unsupported architecture %q", goarch)
	}

	switch goos {
	case "linux", "darwin":
		return fmt.Sprintf("pulumi-v%s-%s-%s.tar.gz", v, goos, arch), nil
	case "windows":
		return fmt.Sprintf("pulumi-v%s-windows-%s.zip", v, arch), nil
	default:
		return "", fmt.Errorf("unsupported operating system %q", goos)
	}
}

// fetchChecksum returns the published SHA256 for artifact, taken from the checksums file attached to the release.
func fetchChecksum(ctx context.Context, v semver.Version, artifact string) (string, error) {
	url := fmt.Sprintf("%s/v%s/pulumi-%s-checksums.txt", releaseBaseURL, v, v)
	body, err := get(ctx, url)
	if err != nil {
		return "", fmt.Errorf("could not fetch checksums for v%s: %w", v, err)
	}
	defer contract.IgnoreClose(body)

	raw, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}

	sum, err := findChecksum(string(raw), artifact)
	if err != nil {
		return "", fmt.Errorf("the checksums for v%s %w", v, err)
	}
	return sum, nil
}

// findChecksum picks the digest for artifact out of a file in sha256sum format: the digest, whitespace, then the
// file name.
func findChecksum(contents, artifact string) (string, error) {
	for line := range strings.SplitSeq(contents, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == artifact {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("do not list %s", artifact)
}

// downloadVerified downloads artifact for the given version into dir and checks it against the published checksum.
func downloadVerified(ctx context.Context, v semver.Version, artifact, dir string) (string, error) {
	want, err := fetchChecksum(ctx, v, artifact)
	if err != nil {
		return "", err
	}

	dest := filepath.Join(dir, artifact)
	urls := []string{
		fmt.Sprintf("%s/v%s/%s", releaseBaseURL, v, artifact),
		fmt.Sprintf("%s/%s", fallbackBaseURL, artifact),
	}

	var lastErr error
	for _, url := range urls {
		lastErr = downloadTo(ctx, url, dest)
		if lastErr == nil {
			break
		}
	}
	if lastErr != nil {
		return "", fmt.Errorf("could not download %s: %w", artifact, lastErr)
	}

	got, err := sha256File(dest)
	if err != nil {
		return "", err
	}
	if got != want {
		// Do not leave an archive we could not vouch for lying around.
		_ = os.Remove(dest)
		return "", fmt.Errorf("checksum mismatch for %s: expected %s, got %s", artifact, want, got)
	}
	return dest, nil
}

func downloadTo(ctx context.Context, url, dest string) error {
	body, err := get(ctx, url)
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(body)

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(f)

	_, err = io.Copy(f, body)
	return err
}

func get(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := httputil.DoWithRetry(req, http.DefaultClient)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		contract.IgnoreClose(resp.Body)
		return nil, fmt.Errorf("GET %s returned %s", url, resp.Status)
	}
	return resp.Body, nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer contract.IgnoreClose(f)

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// extract unpacks a downloaded release archive into dir and returns the directory holding the binaries. Archives
// used to carry a top level bin folder, so that older layout is still recognised.
func extract(archivePath, dir string) (string, error) {
	var err error
	if strings.HasSuffix(archivePath, ".zip") {
		err = extractZip(archivePath, dir)
	} else {
		var f *os.File
		if f, err = os.Open(archivePath); err == nil {
			defer contract.IgnoreClose(f)
			err = archive.ExtractTGZ(f, dir)
		}
	}
	if err != nil {
		return "", fmt.Errorf("could not extract %s: %w", filepath.Base(archivePath), err)
	}

	root := filepath.Join(dir, "pulumi")
	if info, err := os.Stat(filepath.Join(root, "bin")); err == nil && info.IsDir() {
		return filepath.Join(root, "bin"), nil
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return "", fmt.Errorf("%s did not contain the expected pulumi directory", filepath.Base(archivePath))
	}
	return root, nil
}

func extractZip(archivePath, dir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(r)

	for _, f := range r.File {
		// Reject entries that would escape the destination directory.
		dest := filepath.Join(dir, filepath.FromSlash(f.Name)) //nolint:gosec // checked immediately below
		if !strings.HasPrefix(dest, filepath.Clean(dir)+string(os.PathSeparator)) {
			return fmt.Errorf("archive entry %q would write outside the destination directory", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0o700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
			return err
		}
		if err := writeZipEntry(f, dest); err != nil {
			return err
		}
	}
	return nil
}

func writeZipEntry(f *zip.File, dest string) error {
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(src)

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.Mode().Perm())
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(out)

	//nolint:gosec // the archive is checked against its published checksum before it is extracted
	_, err = io.Copy(out, src)
	return err
}
