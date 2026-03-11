// Copyright 2019-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package npm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/errutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
)

type denoManager struct {
	executable string
}

// Assert that Deno is an instance of PackageManager.
var _ PackageManager = &denoManager{}

func newDeno() (*denoManager, error) {
	denoPath, err := exec.LookPath("deno")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, errors.New("Could not find `deno` executable.\n" +
				"Install deno and make sure it is in your PATH.")
		}
		return nil, err
	}
	return &denoManager{
		executable: denoPath,
	}, nil
}

func (d *denoManager) Name() string {
	return "deno"
}

func (d *denoManager) Version() (semver.Version, error) {
	// `deno --version` outputs multi-line text like:
	//   deno 2.x.y
	//   v8 x.y.z
	//   typescript x.y.z
	// We only care about the first line's version.
	cmd := exec.Command(d.executable, "--version") //nolint:gosec
	output, err := cmd.Output()
	if err != nil {
		return semver.Version{}, errutil.ErrorWithStderr(err, cmd.String())
	}
	firstLine := strings.SplitN(strings.TrimSpace(string(output)), "\n", 2)[0]
	parts := strings.Fields(firstLine)
	if len(parts) < 2 {
		return semver.Version{}, fmt.Errorf("unexpected deno --version output: %q", firstLine)
	}
	version, err := semver.Parse(parts[1])
	if err != nil {
		return semver.Version{}, fmt.Errorf("could not parse deno version %q: %w", parts[1], err)
	}
	return version, nil
}

func (d *denoManager) Install(ctx context.Context, dir string, production bool, stdout, stderr io.Writer) error {
	// `deno install` in Deno 2 caches npm dependencies to the global Deno cache.
	// It does not create a node_modules directory (unless nodeModulesDir is configured).
	//nolint:gosec // False positive on tainted command execution. We aren't accepting input from the user here.
	cmd := exec.CommandContext(ctx, d.executable, "install")
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func (d *denoManager) Link(ctx context.Context, dir, packageName, path string) error {
	return LinkDenoLocalPackage(dir, packageName, path)
}

func (d *denoManager) ListPackages(ctx context.Context, dir string, transitive bool) ([]plugin.DependencyInfo, error) {
	// Read direct deps from package.json if present; otherwise return empty.
	pkgPath := filepath.Join(dir, "package.json")
	content, err := os.ReadFile(pkgPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", pkgPath, err)
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(content, &pkg); err != nil {
		return nil, fmt.Errorf("could not parse %s: %w", pkgPath, err)
	}
	var deps []plugin.DependencyInfo
	for name, version := range pkg.Dependencies {
		deps = append(deps, plugin.DependencyInfo{Name: name, Version: version})
	}
	if transitive {
		for name, version := range pkg.DevDependencies {
			deps = append(deps, plugin.DependencyInfo{Name: name, Version: version})
		}
	}
	return deps, nil
}

func (d *denoManager) Pack(ctx context.Context, dir string, stderr io.Writer) ([]byte, error) {
	return nil, errors.New("deno does not support packing npm packages; use npm or another package manager")
}

// LinkDenoLocalPackage wires a local package directory into a Deno project so that
// `import "packageName"` resolves to the local copy instead of the npm registry.
//
// Strategy:
//  1. Add the local path to the "links" array in deno.json — Deno creates a
//     node_modules symlink pointing at localPath.
//  2. Enable nodeModulesDir: "auto" so Deno populates node_modules.
//  3. Remove any import-map entry for packageName that would override the symlink
//     with an npm: specifier.
//  4. Add the package to package.json dependencies so Deno's "not a dependency"
//     check is satisfied when a CJS module dynamically imports user TypeScript.
func LinkDenoLocalPackage(dir, packageName, localPath string) error {
	denoJSONPath := filepath.Join(dir, "deno.json")

	var config map[string]interface{}
	content, err := os.ReadFile(denoJSONPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not read deno.json: %w", err)
	}
	if len(content) > 0 {
		if err := json.Unmarshal(content, &config); err != nil {
			return fmt.Errorf("could not parse deno.json: %w", err)
		}
	}
	if config == nil {
		config = make(map[string]interface{})
	}

	// Add localPath to the links array (idempotent).
	var links []string
	if existing, ok := config["links"]; ok {
		if arr, ok := existing.([]interface{}); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					links = append(links, s)
				}
			}
		}
	}
	alreadyLinked := false
	for _, l := range links {
		if l == localPath {
			alreadyLinked = true
			break
		}
	}
	if !alreadyLinked {
		links = append(links, localPath)
		config["links"] = links
	}

	// "links" requires a node_modules directory so Deno can create the symlink.
	if _, ok := config["nodeModulesDir"]; !ok {
		config["nodeModulesDir"] = "auto"
	}

	// Remove the import-map entry for this package if it maps to npm:, so the
	// symlink created by "links" takes precedence over the registry version.
	if imports, ok := config["imports"]; ok {
		if importsMap, ok := imports.(map[string]interface{}); ok {
			if s, ok := importsMap[packageName].(string); ok && strings.HasPrefix(s, "npm:") {
				delete(importsMap, packageName)
			}
		}
	}

	out, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return fmt.Errorf("could not marshal deno.json: %w", err)
	}
	out = append(out, '\n')
	if err := os.WriteFile(denoJSONPath, out, 0o600); err != nil {
		return err
	}

	// Read the local package version so that the package.json dependency entry
	// matches exactly — Deno will then use the "links" symlink instead of
	// downloading the latest compatible version from the npm registry.
	localVersion := "*"
	if pkgContent, err := os.ReadFile(filepath.Join(localPath, "package.json")); err == nil {
		var localPkg struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(pkgContent, &localPkg) == nil && localPkg.Version != "" {
			localVersion = localPkg.Version
		}
	}

	// Add packageName to package.json dependencies so Deno recognises it as a
	// valid bare specifier (satisfies the "not a dependency" check at runtime).
	return linkPackageJSON(dir, packageName, localVersion)
}

// linkPackageJSON ensures packageName is listed in package.json's dependencies
// so Deno recognises it as a valid bare specifier at runtime.
// version should match the linked local package so that Deno's "links" symlink
// takes precedence over a fresh download from the npm registry.
func linkPackageJSON(dir, packageName, version string) error {
	pkgJSONPath := filepath.Join(dir, "package.json")
	var pkg map[string]interface{}
	content, err := os.ReadFile(pkgJSONPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not read package.json: %w", err)
	}
	if len(content) > 0 {
		if err := json.Unmarshal(content, &pkg); err != nil {
			return fmt.Errorf("could not parse package.json: %w", err)
		}
	}
	if pkg == nil {
		pkg = make(map[string]interface{})
	}

	deps, _ := pkg["dependencies"].(map[string]interface{})
	if deps == nil {
		deps = make(map[string]interface{})
	}
	deps[packageName] = version
	pkg["dependencies"] = deps

	out, err := json.MarshalIndent(pkg, "", "    ")
	if err != nil {
		return fmt.Errorf("could not marshal package.json: %w", err)
	}
	out = append(out, '\n')
	return os.WriteFile(pkgJSONPath, out, 0o600)
}

// checkDenoConfig checks if there is a file named deno.json or deno.jsonc in pwd.
// This function is used to indicate whether to prefer deno over other package managers.
func checkDenoConfig(pwd string) bool {
	if _, err := fsutil.Searchup(pwd, "deno.json"); err == nil {
		return true
	}
	_, err := fsutil.Searchup(pwd, "deno.jsonc")
	return err == nil
}


