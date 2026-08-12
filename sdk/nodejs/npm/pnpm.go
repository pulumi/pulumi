// Copyright 2016, Pulumi Corporation.
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

package npm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/errutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
)

// pnpm is an alternative package manager for Node.js.
type pnpmManager struct {
	executable string
}

// Assert that pnpm is an instance of PackageManager.
var _ PackageManager = &pnpmManager{}

func newPnpm() (*pnpmManager, error) {
	pnpmPath, err := exec.LookPath("pnpm")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, errors.New("Could not find `pnpm` executable.\n" +
				"Install pnpm and make sure it is in your PATH.")
		}
		return nil, err
	}
	return &pnpmManager{
		executable: pnpmPath,
	}, nil
}

func (pnpm *pnpmManager) Name() string {
	return "pnpm"
}

func (pnpm *pnpmManager) Version() (semver.Version, error) {
	cmd := exec.Command(pnpm.executable, "--version") //nolint:gosec
	output, err := cmd.Output()
	if err != nil {
		return semver.Version{}, errutil.ErrorWithStderr(err, cmd.String())
	}
	versionStr := strings.TrimSpace(string(output))
	version, err := semver.Parse(versionStr)
	if err != nil {
		return semver.Version{}, err
	}
	return version, nil
}

func (pnpm *pnpmManager) Install(ctx context.Context, dir string, production bool, stdout, stderr io.Writer) error {
	command := pnpm.installCmd(ctx, production)
	command.Dir = dir
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}

func (pnpm *pnpmManager) installCmd(ctx context.Context, production bool) *exec.Cmd {
	args := []string{"install", "--use-stderr"}

	if production {
		args = append(args, "--production")
	}

	//nolint:gosec // False positive on tained command execution. We aren't accepting input from the user here.
	return exec.CommandContext(ctx, pnpm.executable, args...)
}

func (pnpm *pnpmManager) Link(ctx context.Context, dir, packageName, path string) error {
	packageSpecifier := getLinkPackageProperty(packageName, path)
	cmd := exec.CommandContext(ctx, "npm", "pkg", "set", packageSpecifier)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error executing npm command %s: %w, output: %s", cmd.String(), err, out)
	}

	// Local SDKs have a postInstall script that needs to run. By default pnpm does not run these scripts, so we have
	// to allowlist the package.
	version, err := pnpm.Version()
	if err != nil {
		return err
	}
	return pnpm.allowBuildScripts(ctx, version, dir, packageName, path)
}

// allowBuildScripts allowlists the linked local SDK so that its install/postInstall script (which compiles the SDK
// from TypeScript) is allowed to run.
//
// On pnpm >=11 the dependencies are listed in allowBuilds, on older versions in `onlyBuiltDependencies`.
//
// Local directory dependencies are keyed by their lockfile depPath ("name@file:path"). pnpm >= 10.34.2 requires this
// form (bare names are silently ignored), while versions before it reject it with ERR_PNPM_INVALID_VERSION_UNION.
func (pnpm *pnpmManager) allowBuildScripts(
	ctx context.Context, version semver.Version, dir, packageName, path string,
) error {
	depPath := packageName + "@file:" + filepath.ToSlash(path)

	if version.GTE(semver.MustParse("11.0.0")) {
		return pnpm.mergeProjectConfig(ctx, dir, "allowBuilds", func(current []byte) (any, error) {
			allowBuilds := map[string]bool{}
			_ = json.Unmarshal(current, &allowBuilds)
			allowBuilds[depPath] = true
			return allowBuilds, nil
		})
	}

	key := packageName
	if version.GTE(semver.MustParse("10.34.2")) {
		key = depPath
	}
	return pnpm.mergeProjectConfig(ctx, dir, "onlyBuiltDependencies", func(current []byte) (any, error) {
		var deps []string
		_ = json.Unmarshal(current, &deps)
		if !slices.Contains(deps, key) {
			deps = append(deps, key)
		}
		return deps, nil
	})
}

// mergeProjectConfig reads a pnpm project-level config setting (as JSON), applies update to it, and writes the result
// back. `pnpm config set --location project` stores it in pnpm-workspace.yaml, preserving any other keys. `current`
// is nil when the setting is unset.
func (pnpm *pnpmManager) mergeProjectConfig(
	ctx context.Context, dir, setting string, update func(current []byte) (any, error),
) error {
	get := exec.CommandContext(ctx, "pnpm", "config", "get", setting, "--json")
	get.Dir = dir
	out, err := get.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running %s: %w, output: %s", get.String(), err, out)
	}
	if out = bytes.TrimSpace(out); string(out) == "undefined" {
		out = nil
	}
	updated, err := update(out)
	if err != nil {
		return err
	}
	data, err := json.Marshal(updated)
	if err != nil {
		return fmt.Errorf("error marshaling %s to JSON: %w", setting, err)
	}
	//nolint:gosec // json data is escaped
	set := exec.CommandContext(ctx, "pnpm", "config", "set", setting, string(data), "--location", "project", "--json")
	set.Dir = dir
	if out, err := set.CombinedOutput(); err != nil {
		return fmt.Errorf("error running %s: %w, output: %s", set.String(), err, out)
	}
	return nil
}

func (pnpm *pnpmManager) ListPackages(
	ctx context.Context, dir string, transitive bool,
) ([]plugin.DependencyInfo, error) {
	return listPackagesFromLockFile(dir, "pnpm-lock.yaml", transitive)
}

func (pnpm *pnpmManager) Pack(ctx context.Context, dir string, stderr io.Writer) ([]byte, error) {
	//nolint:gosec // False positive on tained command execution. We aren't accepting input from the user here.
	command := exec.CommandContext(ctx, pnpm.executable, "pack", "--use-stderr")
	command.Dir = dir

	// We have to read the name of the file from stdout.
	var stdout bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = stderr
	err := command.Run()
	if err != nil {
		return nil, err
	}
	// Next, we try to read the name of the file from stdout. packfile is the name
	// of the file containing the tarball, as produced by `pnpm pack`. For pnpm
	// versions <9.13, pnpm pack outputs a single line with the name of the
	// tarball. On pnpm versions >=9.13, pnpm pack lists all the packed files, and
	// the last line is the tarball.
	packFilename := ""
	scanner := bufio.NewScanner(bytes.NewReader(stdout.Bytes()))
	for scanner.Scan() {
		packFilename = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read the output of 'pnpm pack': %w", err)
	}

	packfile := filepath.Join(dir, packFilename)
	defer os.Remove(packfile)

	packTarball, err := os.ReadFile(packfile)
	if err != nil {
		newErr := fmt.Errorf("'pnpm pack' completed successfully but the package .tgz file was not generated: %w", err)
		return nil, newErr
	}

	return packTarball, nil
}

// checkPnpmLock checks if there's a file named pnpm-lock.yaml in pwd.
// This function is used to indicate whether to prefer pnpm over
// other package managers.
func checkPnpmLock(pwd string) bool {
	_, err := fsutil.Searchup(pwd, "pnpm-lock.yaml")
	return err == nil
}
