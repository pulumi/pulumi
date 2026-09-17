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
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// uvLockFile is a minimal representation of the parts of uv.lock we need to determine which
// packages `uv sync` installs for a given project.
type uvLockFile struct {
	Package []uvLockPackage `toml:"package"`
}

type uvLockPackage struct {
	Name    string       `toml:"name"`
	Version string       `toml:"version"`
	Source  uvLockSource `toml:"source"`
	// Dependencies are the package's runtime dependencies.
	Dependencies []uvLockDependency `toml:"dependencies"`
	// OptionalDependencies maps an extra's name to the dependencies it adds.
	OptionalDependencies map[string][]uvLockDependency `toml:"optional-dependencies"`
	// DevDependencies maps a dependency group's name to its dependencies.
	DevDependencies map[string][]uvLockDependency `toml:"dev-dependencies"`
}

// uvLockSource describes where a package comes from. Workspace members are recorded with a
// `virtual` (not installed, no build backend) or `editable` source whose value is the member's
// path relative to the directory containing uv.lock.
type uvLockSource struct {
	Virtual  string `toml:"virtual"`
	Editable string `toml:"editable"`
}

type uvLockDependency struct {
	Name string `toml:"name"`
	// Extra holds the extras requested of this dependency, e.g. `coverage[toml]` is recorded as
	// { name = "coverage", extra = ["toml"] }.
	Extra []string `toml:"extra"`
}

func parseUvLock(content []byte) (*uvLockFile, error) {
	var lock uvLockFile
	if _, err := toml.Decode(string(content), &lock); err != nil {
		return nil, err
	}
	return &lock, nil
}

// virtualPackages returns the names of packages that are virtual (i.e. the project root or
// workspace members without a build backend). Virtual packages are not installed into the
// virtualenv.
func (l *uvLockFile) virtualPackages() map[string]bool {
	virtual := make(map[string]bool)
	for _, pkg := range l.Package {
		if pkg.Source.Virtual != "" {
			virtual[normalizePythonPackageName(pkg.Name)] = true
		}
	}
	return virtual
}

// findMember locates the workspace member that owns dir, returning the member's directory and its
// entry in the lock file. dir may be nested inside the member, in which case we walk up towards
// lockDir looking for a match. Returns a nil package if dir does not belong to any member.
func (l *uvLockFile) findMember(lockDir, dir string) (string, *uvLockPackage) {
	byPath := make(map[string]*uvLockPackage, len(l.Package))
	for i := range l.Package {
		pkg := &l.Package[i]
		for _, source := range []string{pkg.Source.Virtual, pkg.Source.Editable} {
			if source != "" {
				byPath[path.Clean(filepath.ToSlash(source))] = pkg
			}
		}
	}

	for {
		rel, err := filepath.Rel(lockDir, dir)
		if err != nil {
			return "", nil
		}
		// We've walked above the directory holding uv.lock, no member can match.
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", nil
		}
		if pkg, ok := byPath[path.Clean(filepath.ToSlash(rel))]; ok {
			return dir, pkg
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

// uvDependencySelection describes the extras and dependency groups of a project that `uv sync`
// installs when it is invoked without any extra or group flags.
type uvDependencySelection struct {
	extras    []string
	allExtras bool
	groups    []string
	allGroups bool
}

// uvDefaultSelection reads `tool.uv.default-extras` and `tool.uv.default-groups` from the project's
// pyproject.toml. uv installs no extras and the `dev` group by default.
func uvDefaultSelection(projectDir string) uvDependencySelection {
	selection := uvDependencySelection{groups: []string{"dev"}}

	pyproject, err := LoadPyproject(projectDir)
	if err != nil {
		return selection
	}
	uvTool, ok := pyproject.Tool["uv"].(map[string]any)
	if !ok {
		return selection
	}
	if value, ok := uvTool["default-extras"]; ok {
		selection.extras, selection.allExtras = uvNameListSetting(value)
	}
	if value, ok := uvTool["default-groups"]; ok {
		selection.groups, selection.allGroups = uvNameListSetting(value)
	}
	return selection
}

// uvNameListSetting parses a uv setting that is either a list of names or the string "all".
func uvNameListSetting(value any) (names []string, all bool) {
	switch value := value.(type) {
	case string:
		if value == "all" {
			return nil, true
		}
		return []string{value}, false
	case []any:
		for _, item := range value {
			if name, ok := item.(string); ok {
				names = append(names, name)
			}
		}
		return names, false
	}
	return nil, false
}

// selectedNames resolves a selection against the sections a package actually declares.
func selectedNames(available map[string][]uvLockDependency, names []string, all bool) []string {
	if all {
		return slices.Sorted(maps.Keys(available))
	}
	return names
}

// uvDependencyEdge is a package reached through the dependency graph, optionally via one of its
// extras. A package can be reached both plain and through several extras, and each of those
// contributes a different set of onward dependencies.
type uvDependencyEdge struct {
	name  string
	extra string
}

// dependenciesOf walks the lock file's dependency graph starting at project and returns the
// packages `uv sync` installs for it. If transitive is false only the project's direct
// dependencies are returned. The project itself and virtual workspace members are omitted, as
// neither is installed into the virtualenv as a distribution.
func (l *uvLockFile) dependenciesOf(
	project *uvLockPackage, selection uvDependencySelection, transitive bool,
) []plugin.DependencyInfo {
	byName := make(map[string]*uvLockPackage, len(l.Package))
	for i := range l.Package {
		byName[normalizePythonPackageName(l.Package[i].Name)] = &l.Package[i]
	}

	var queue []uvDependencyEdge
	enqueue := func(dependencies []uvLockDependency) {
		for _, dependency := range dependencies {
			name := normalizePythonPackageName(dependency.Name)
			queue = append(queue, uvDependencyEdge{name: name})
			for _, extra := range dependency.Extra {
				queue = append(queue, uvDependencyEdge{name: name, extra: extra})
			}
		}
	}

	enqueue(project.Dependencies)
	for _, extra := range selectedNames(project.OptionalDependencies, selection.extras, selection.allExtras) {
		enqueue(project.OptionalDependencies[extra])
	}
	for _, group := range selectedNames(project.DevDependencies, selection.groups, selection.allGroups) {
		enqueue(project.DevDependencies[group])
	}

	projectName := normalizePythonPackageName(project.Name)
	seenEdges := make(map[uvDependencyEdge]bool)
	seenPackages := make(map[string]bool)
	packages := []plugin.DependencyInfo{}
	for len(queue) > 0 {
		edge := queue[0]
		queue = queue[1:]
		if seenEdges[edge] {
			continue
		}
		seenEdges[edge] = true

		pkg, ok := byName[edge.name]
		if !ok {
			continue
		}
		if !seenPackages[edge.name] && edge.name != projectName && pkg.Source.Virtual == "" {
			seenPackages[edge.name] = true
			packages = append(packages, plugin.DependencyInfo{
				Name:    edge.name,
				Version: pkg.Version,
			})
		}

		if !transitive {
			continue
		}
		if edge.extra == "" {
			enqueue(pkg.Dependencies)
		} else {
			enqueue(pkg.OptionalDependencies[edge.extra])
		}
	}
	return packages
}
