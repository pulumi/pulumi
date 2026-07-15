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
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/httputil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Resolve returns paths to the requested Node.js version, downloading and
// caching the distribution under ~/.pulumi/node/ on first use. It is safe to
// call concurrently across processes.
func Resolve(ctx context.Context, spec Spec) (Result, error) {
	name, err := archiveName(spec.Version, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return Result{}, err
	}
	base := archiveBase(name)
	root, err := workspace.GetPulumiPath("node", base)
	if err != nil {
		return Result{}, err
	}
	binDir := layoutBinDir(root, base, runtime.GOOS)
	result := Result{
		Node:   filepath.Join(binDir, nodeExe()),
		Npm:    filepath.Join(binDir, npmExe()),
		BinDir: binDir,
	}
	if installed(root, result) {
		return result, nil
	}

	if err := os.MkdirAll(filepath.Dir(root), 0o700); err != nil {
		return Result{}, err
	}
	mutex := fsutil.NewFileMutex(root + ".lock")
	if err := mutex.Lock(); err != nil {
		return Result{}, err
	}
	defer func() { contract.IgnoreError(mutex.Unlock()) }()

	// Another process may have completed the install while we waited.
	if installed(root, result) {
		return result, nil
	}
	// The directory is a failed or corrupted install; replace it.
	if _, err := os.Stat(root); err == nil {
		if err := os.RemoveAll(root); err != nil {
			return Result{}, err
		}
	}

	if spec.Output != nil {
		fmt.Fprintf(spec.Output, "Downloading Node.js v%s...\n", spec.Version)
	}
	if err := install(ctx, spec, name, root); err != nil {
		return Result{}, err
	}
	return result, nil
}

// installed reports whether root holds a complete install: no in-progress
// .partial marker, and both the node and npm binaries present. os.Stat
// follows symlinks, so a dangling npm symlink (e.g. npm-cli.js shipped
// separately from node) is correctly treated as incomplete.
func installed(root string, result Result) bool {
	if _, err := os.Stat(partialPath(root)); err == nil {
		return false
	}
	if _, err := os.Stat(result.Node); err != nil {
		return false
	}
	if _, err := os.Stat(result.Npm); err != nil {
		return false
	}
	return true
}

func partialPath(root string) string {
	return root + ".partial"
}

func archiveBase(name string) string {
	return strings.TrimSuffix(strings.TrimSuffix(name, ".tar.gz"), ".zip")
}

func install(ctx context.Context, spec Spec, name, root string) error {
	url := spec.baseURL() + "/v" + spec.Version + "/" + name
	archiveFile, err := os.CreateTemp(filepath.Dir(root), name+".download")
	if err != nil {
		return err
	}
	defer func() {
		archiveFile.Close()
		os.Remove(archiveFile.Name())
	}()
	size, err := downloadTo(ctx, url, archiveFile)
	if err != nil {
		return err
	}

	// Extract directly into the final directory instead of renaming a temp
	// directory into place: directory renames fail intermittently on Windows
	// when virus scanners hold freshly written files open (the same reason
	// plugin installs work this way, see pkg/pluginstorage). The .partial
	// marker is left behind on failure so the next Resolve replaces the
	// directory.
	if err := os.WriteFile(partialPath(root), nil, 0o600); err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	if strings.HasSuffix(name, ".zip") {
		err = extractZip(archiveFile, size, root)
	} else {
		if _, err = archiveFile.Seek(0, io.SeekStart); err == nil {
			err = archive.ExtractTGZ(archiveFile, root)
		}
	}
	if err != nil {
		return fmt.Errorf("extracting %s: %w", name, err)
	}
	return os.Remove(partialPath(root))
}

var downloadClient = &http.Client{Timeout: 5 * time.Minute}

func downloadTo(ctx context.Context, url string, w io.Writer) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := httputil.DoWithRetry(req, downloadClient)
	if err != nil {
		return 0, fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("downloading %s: HTTP %d", url, resp.StatusCode)
	}
	return io.Copy(w, resp.Body)
}

func extractZip(r io.ReaderAt, size int64, dir string) error {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return err
	}
	cleanDir := filepath.Clean(dir)
	for _, f := range zr.File {
		path := filepath.Join(cleanDir, f.Name) //nolint:gosec // path traversal is checked below
		if path != cleanDir && !strings.HasPrefix(path, cleanDir+string(os.PathSeparator)) {
			return fmt.Errorf("zip entry %q escapes destination directory", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0o700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		src, err := f.Open()
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, f.Mode())
		if err != nil {
			src.Close()
			return err
		}
		_, err = io.Copy(dst, src) //nolint:gosec // official Node.js release archive
		src.Close()
		dst.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
