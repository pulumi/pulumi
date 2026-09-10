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

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/nodejs/npm"
)

// The install cache shares installed node_modules trees between projects with
// identical dependency manifests: the first project with a given manifest
// installs normally and donates its tree to the cache, and every later project
// gets a symlink to that tree, the same sharing model pnpm uses. It is enabled
// by the conformance test harness, where most test projects share a small
// number of distinct dependency sets and the per-project install dominates
// test wall time.
//
// Sharing is only safe because the harness guarantees projects never modify
// their installed dependencies. It must stay disabled for real projects: any
// later package-manager invocation would write through the symlink into the
// shared tree.

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
func installCacheKey(projectDir string) string {
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
	return hex.EncodeToString(hash[:])
}

// installShared installs the project's dependencies, sharing the installed
// node_modules tree with other projects that have an identical dependency
// manifest when the host's install cache is enabled.
func (host *nodeLanguageHost) installShared(
	ctx context.Context, packagemanager npm.PackageManagerType, workspaceRoot string, isPlugin bool, production bool,
	stdout, stderr io.Writer,
) error {
	cacheKey := ""
	// Production installs don't participate in the cache: a production
	// node_modules tree differs from a full one for the same manifest.
	if host.installCacheDir != "" && !isPlugin && !production {
		cacheKey = installCacheKey(workspaceRoot)
	}
	if cacheKey == "" {
		_, err := npm.Install(ctx, packagemanager, workspaceRoot, production, stdout, stderr)
		return err
	}

	// Decide under the per-key lock whether this install populates the cache
	// or consumes it. Consumers proceed concurrently; only the populating
	// install holds the lock, so concurrent projects with the same
	// dependencies wait for the cache instead of re-installing.
	lock, _ := host.installCacheLocks.LoadOrStore(cacheKey, &sync.Mutex{})
	lock.Lock()
	if _, err := os.Lstat(filepath.Join(workspaceRoot, "node_modules")); err == nil {
		// Already installed by an earlier call for this project.
		lock.Unlock()
		return nil
	}
	cacheDir := filepath.Join(host.installCacheDir, cacheKey)
	if _, err := os.Stat(cacheDir); err == nil {
		restoreErr := restoreInstallCache(cacheDir, workspaceRoot)
		lock.Unlock()
		return restoreErr
	}
	defer lock.Unlock()

	if _, err := npm.Install(ctx, packagemanager, workspaceRoot, false /*production*/, stdout, stderr); err != nil {
		return err
	}
	return populateInstallCache(workspaceRoot, cacheDir)
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
		if err := fsutil.CopyFile(filepath.Join(projectDir, name), src, nil); err != nil {
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
	// Both paths are under the harness's temporary directory, on one file
	// system.
	//nolint:forbidigo // os.Rename within the same directory tree; test-harness usage is OK
	if err := os.Rename(filepath.Join(projectDir, "node_modules"), filepath.Join(staging, "node_modules")); err != nil {
		contract.IgnoreError(os.RemoveAll(staging))
		return err
	}
	for _, name := range installCacheLockFiles {
		src := filepath.Join(projectDir, name)
		if _, err := os.Lstat(src); err != nil {
			continue
		}
		if err := fsutil.CopyFile(filepath.Join(staging, name), src, nil); err != nil {
			contract.IgnoreError(os.RemoveAll(staging))
			return err
		}
	}
	//nolint:forbidigo // os.Rename within the same directory tree; test-harness usage is OK
	if err := os.Rename(staging, cacheDir); err != nil {
		return err
	}
	return os.Symlink(filepath.Join(cacheDir, "node_modules"), filepath.Join(projectDir, "node_modules"))
}
