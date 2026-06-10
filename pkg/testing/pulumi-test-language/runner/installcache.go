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

package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Most conformance test projects share a small number of distinct dependency
// sets, but each project runs its own dependency install into its own
// directory: for Node.js that is a full npm install of the core SDK's
// dependency tree per test, which dominates test wall time. The install cache
// keeps the node_modules tree of the first project with a given dependency
// manifest and symlinks it into every later project with the same manifest,
// the same sharing model pnpm uses. Tests never modify their installed
// dependencies, so sharing the tree read-only is safe, and the install path
// itself is still exercised by every cache-miss install.

// installCacheLockFiles are the package manager outputs, besides node_modules
// itself, that capture the result of an install.
var installCacheLockFiles = []string{
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
	"bun.lock",
	"bun.lockb",
}

// installCacheKey returns a key identifying the project's dependency set, or
// the empty string if installs for this project can't be cached.
func installCacheKey(languageName, projectDir string) string {
	// Only runtimes whose installed dependencies live in a relocatable
	// node_modules directory. Notably a Python venv is not relocatable.
	if languageName != "nodejs" && languageName != "bun" {
		return ""
	}
	manifestBytes, err := os.ReadFile(filepath.Join(projectDir, "package.json"))
	if err != nil {
		return ""
	}
	var manifest map[string]json.RawMessage
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return ""
	}
	// Projects with the same dependencies differ only in name.
	delete(manifest, "name")
	normalized, err := json.Marshal(manifest)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(normalized)
	return languageName + "-" + hex.EncodeToString(hash[:])
}

// restoreInstallCache links cacheDir's node_modules into projectDir and copies
// the recorded lock files.
func restoreInstallCache(cacheDir, projectDir string) error {
	if err := os.Symlink(filepath.Join(cacheDir, "node_modules"), filepath.Join(projectDir, "node_modules")); err != nil {
		return err
	}
	for _, name := range installCacheLockFiles {
		src := filepath.Join(cacheDir, name)
		if _, err := os.Lstat(src); err != nil {
			continue
		}
		if err := copyFile(src, filepath.Join(projectDir, name)); err != nil {
			return err
		}
	}
	return nil
}

// populateInstallCache moves projectDir's node_modules into cacheDir, records
// the lock files alongside it, and symlinks the tree back into projectDir. The
// cache directory is created atomically so a failed populate never leaves a
// partial cache.
func populateInstallCache(projectDir, cacheDir string) error {
	staging := cacheDir + ".tmp"
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return err
	}
	// Both paths are under the test's temporary directory, on one file system.
	//nolint:forbidigo // os.Rename within the same directory tree; test usage is OK
	if err := os.Rename(filepath.Join(projectDir, "node_modules"), filepath.Join(staging, "node_modules")); err != nil {
		contract.IgnoreError(os.RemoveAll(staging))
		return err
	}
	for _, name := range installCacheLockFiles {
		src := filepath.Join(projectDir, name)
		if _, err := os.Lstat(src); err != nil {
			continue
		}
		if err := copyFile(src, filepath.Join(staging, name)); err != nil {
			contract.IgnoreError(os.RemoveAll(staging))
			return err
		}
	}
	//nolint:forbidigo // os.Rename within the same directory tree; test usage is OK
	if err := os.Rename(staging, cacheDir); err != nil {
		return err
	}
	return os.Symlink(filepath.Join(cacheDir, "node_modules"), filepath.Join(projectDir, "node_modules"))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		contract.IgnoreClose(out)
		return err
	}
	return out.Close()
}
