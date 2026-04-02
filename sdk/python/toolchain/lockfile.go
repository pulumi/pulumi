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

package toolchain

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/git-pkgs/manifests"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// listPackagesFromLockFile parses a Python lock file (poetry.lock or uv.lock) and returns the
// installed packages. If transitive is false, only direct dependencies listed in pyproject.toml
// in the same directory are returned.
func listPackagesFromLockFile(
	lockFilePath string, transitive bool, exclude map[string]bool,
) ([]plugin.DependencyInfo, error) {
	content, err := os.ReadFile(lockFilePath)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", lockFilePath, err)
	}

	result, err := manifests.Parse(filepath.Base(lockFilePath), content)
	if err != nil {
		return nil, fmt.Errorf("could not parse %s: %w", lockFilePath, err)
	}

	packages := make([]plugin.DependencyInfo, 0, len(result.Dependencies))
	for _, dep := range result.Dependencies {
		if exclude[normalizePythonPackageName(dep.Name)] {
			continue
		}
		packages = append(packages, plugin.DependencyInfo{
			Name:    normalizePythonPackageName(dep.Name),
			Version: dep.Version,
		})
	}

	if transitive {
		return packages, nil
	}
	return filterDirectPythonDependencies(filepath.Dir(lockFilePath), packages)
}

// filterDirectPythonDependencies reads pyproject.toml and returns only the packages that are
// listed as direct dependencies.
func filterDirectPythonDependencies(dir string, packages []plugin.DependencyInfo) ([]plugin.DependencyInfo, error) {
	pyprojectPath := filepath.Join(dir, "pyproject.toml")
	content, err := os.ReadFile(pyprojectPath)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", pyprojectPath, err)
	}

	result, err := manifests.Parse("pyproject.toml", content)
	if err != nil {
		return nil, fmt.Errorf("could not parse %s: %w", pyprojectPath, err)
	}

	directNames := make(map[string]bool, len(result.Dependencies))
	for _, dep := range result.Dependencies {
		directNames[normalizePythonPackageName(dep.Name)] = true
	}

	var direct []plugin.DependencyInfo
	for _, pkg := range packages {
		if directNames[pkg.Name] {
			direct = append(direct, pkg)
		}
	}
	return direct, nil
}
