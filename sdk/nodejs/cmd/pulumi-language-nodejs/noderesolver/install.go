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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// ResolveWith finds node and npm: ambient PATH first, then the managed
// install, downloading it on first use.
func ResolveWith(ctx context.Context, spec Spec) (Result, error) {
	if node, err := exec.LookPath(nodeExe()); err == nil {
		npm, _ := exec.LookPath(npmExe())
		return Result{Node: node, Npm: npm, BinDir: filepath.Dir(node)}, nil
	}
	if spec.Disabled {
		return Result{}, errors.New(
			"node not found on the PATH and the managed Node.js fallback is disabled " +
				"(PULUMI_DISABLE_MANAGED_NODE); install Node.js to continue")
	}
	binDir, err := ensureInstalled(ctx, spec)
	if err != nil {
		return Result{}, err
	}
	return Result{
		Node:    filepath.Join(binDir, nodeExe()),
		Npm:     filepath.Join(binDir, npmExe()),
		BinDir:  binDir,
		Managed: true,
	}, nil
}

func nodeExe() string {
	if runtime.GOOS == "windows" {
		return "node.exe"
	}
	return "node"
}

func npmExe() string {
	if runtime.GOOS == "windows" {
		return "npm.cmd"
	}
	return "npm"
}

func ensureInstalled(ctx context.Context, spec Spec) (string, error) {
	name, err := archiveFile(spec, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	base := strings.TrimSuffix(strings.TrimSuffix(name, ".tar.gz"), ".zip")
	root, err := workspace.GetPulumiPath("node", base)
	if err != nil {
		return "", err
	}
	binDir := layoutBinDir(root, base, runtime.GOOS)
	if _, err := os.Stat(filepath.Join(binDir, nodeExe())); err == nil {
		return binDir, nil
	}

	if spec.Output != nil {
		fmt.Fprintf(spec.Output, "Downloading Node.js v%s (node was not found on your PATH)...\n", spec.Version)
	}
	binDir, err = installArchive(ctx, spec, name, base, root, binDir)
	if err != nil {
		return "", fmt.Errorf("%w; install Node.js or set PULUMI_NODE_DOWNLOAD_URL to a reachable mirror", err)
	}
	return binDir, nil
}

func installArchive(ctx context.Context, spec Spec, name, base, root, binDir string) (string, error) {
	data, err := download(ctx, spec.BaseURL+"/v"+spec.Version+"/"+name)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != spec.Checksums[name] {
		return "", fmt.Errorf("checksum mismatch for %s: got %s, want %s", name, got, spec.Checksums[name])
	}

	if err := os.MkdirAll(filepath.Dir(root), 0o700); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(root), filepath.Base(root)+".tmp")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	if strings.HasSuffix(name, ".zip") {
		err = extractZip(data, tmp)
	} else {
		err = archive.ExtractTGZ(bytes.NewReader(data), tmp)
	}
	if err != nil {
		return "", fmt.Errorf("extracting %s: %w", name, err)
	}

	// A concurrent process may win the rename; either install is identical.
	if err := os.Rename(tmp, root); err != nil {
		if _, statErr := os.Stat(filepath.Join(binDir, nodeExe())); statErr == nil {
			return binDir, nil
		}
		return "", fmt.Errorf("installing Node.js: %w", err)
	}
	return binDir, nil
}

var downloadClient = &http.Client{Timeout: 5 * time.Minute}

func download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := downloadClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func extractZip(data []byte, dir string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	cleanDir := filepath.Clean(dir)
	for _, f := range zr.File {
		path := filepath.Join(cleanDir, f.Name)
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
		_, err = io.Copy(dst, src) //nolint:gosec // trusted, checksum-verified archive
		src.Close()
		dst.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
