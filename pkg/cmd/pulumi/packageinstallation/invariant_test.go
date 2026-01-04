// Copyright 2025, Pulumi Corporation.
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

package packageinstallation_test

import (
	"context"
	"maps"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageinstallation"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

var _ packageinstallation.Workspace = invariantWorkspace{}

func newInvariantWorkspace(t *testing.T, workDirs []string, plugins []invariantPlugin) *invariantWorkspace {
	pluginMap := make(map[string]*invariantPlugin, len(plugins))
	for _, v := range plugins {
		p := filepath.ToSlash(filepath.Join("$HOME", ".pulumi", "plugins", v.d.Dir(), v.d.SubDir()))
		pluginMap[p] = &v
	}
	downloadedWorkspace := make(map[string]*invariantWorkDir, len(workDirs))
	for _, dir := range workDirs {
		downloadedWorkspace[filepath.ToSlash(dir)] = &invariantWorkDir{}
	}
	return &invariantWorkspace{
		t:                   t,
		plugins:             pluginMap,
		binaryPaths:         map[string]string{},
		downloadedWorkspace: downloadedWorkspace,
		rw:                  new(sync.RWMutex),
	}
}

func assertInvariantWorkspaceEqual(t *testing.T, a, b invariantWorkspace) {
	a.t = nil
	b.t = nil
	assert.Equal(t, a, b)
}

type invariantWorkspace struct {
	t *testing.T
	// All plugins visible to the test
	plugins map[string]*invariantPlugin

	// A map of binary paths to the plugin directories they contain.
	binaryPaths map[string]string

	// A list of paths where Link is allowed, but which are not plugins.
	downloadedWorkspace map[string]*invariantWorkDir

	// A mutex guarding the shape of plugins discover-able via HasPlugin or HasPluginGTE.
	//
	// It must be held for write when the results of HasPlugin change.
	rw *sync.RWMutex
}

type invariantWorkDir struct{ linked []string }

type invariantPlugin struct {
	d               workspace.PluginDescriptor
	downloaded      bool
	installed       bool
	pathVisible     bool
	hasBinary       bool
	projectDetected bool
	project         *workspace.PluginProject

	linked []string
}

func (w invariantWorkspace) HasPlugin(spec workspace.PluginDescriptor) bool {
	w.rw.RLock()
	defer w.rw.RUnlock()
	for _, candidate := range w.plugins {
		if candidate.installed &&
			candidate.d.Name == spec.Name &&
			candidate.d.Kind == spec.Kind &&
			(candidate.d.Version == nil && spec.Version == nil ||
				(candidate.d.Version != nil && spec.Version != nil && candidate.d.Version.EQ(*spec.Version))) {
			return true
		}
	}
	return false
}

func (w invariantWorkspace) HasPluginGTE(spec workspace.PluginDescriptor) (bool, *semver.Version, error) {
	w.rw.RLock()

	var gte *workspace.PluginDescriptor
	for _, candidate := range w.plugins {
		if candidate.installed &&
			candidate.d.Name == spec.Name &&
			candidate.d.Kind == spec.Kind && candidate.d.Version != nil {
			if gte == nil {
				gte = &candidate.d
				continue
			}

			if gte.Version.LT(*candidate.d.Version) {
				gte = &candidate.d
			}
		}
	}
	w.rw.RUnlock()
	if gte == nil {
		// We have found a version with no version
		spec.Version = nil
		if w.HasPlugin(spec) {
			return true, nil, nil
		}
		return false, nil, nil
	}
	if spec.Version != nil && gte.Version.LT(*spec.Version) {
		return false, nil, nil
	}
	return true, gte.Version, nil
}

func (w invariantWorkspace) IsExternalURL(source string) bool { return workspace.IsExternalURL(source) }

func (w invariantWorkspace) GetLatestVersion(
	ctx context.Context, spec workspace.PluginDescriptor,
) (*semver.Version, error) {
	var version *semver.Version
	for _, p := range w.plugins {
		if p.d.Name != spec.Name || p.d.Version == nil {
			continue
		}
		if version == nil || p.d.Version.GT(*version) {
			cp := *p.d.Version
			version = &cp
		}
	}
	return version, nil
}

func (w invariantWorkspace) GetPluginPath(ctx context.Context, plugin workspace.PluginDescriptor) (string, error) {
	p := filepath.ToSlash(filepath.Join("$HOME", ".pulumi", "plugins", plugin.Dir(), plugin.SubDir()))
	pl, ok := w.plugins[p]
	if !ok || !pl.downloaded {
		assert.Fail(w.t, "GetPluginPath() called on non-present plugin")
		return "", assert.AnError
	}
	pl.pathVisible = true
	return p, nil
}

func (w invariantWorkspace) InstallPluginAt(
	ctx context.Context, dirPath string, project *workspace.PluginProject,
) error {
	w.rw.Lock()
	defer w.rw.Unlock()
	dirPath = filepath.ToSlash(dirPath)
	p, ok := w.plugins[dirPath]
	if !ok || !p.downloaded {
		assert.Failf(w.t, "", "InstallPluginAt(%q) called on non-revealed plugin dir", dirPath)
		return assert.AnError
	}
	assert.False(w.t, p.installed, "InstallPluginAt(%q) called in already installed dir", dirPath)
	p.installed = true
	return nil
}

func (w invariantWorkspace) IsExecutable(ctx context.Context, binaryPath string) (bool, error) {
	p := filepath.ToSlash(filepath.Dir(binaryPath))
	pl, ok := w.plugins[p]
	if !ok || !pl.pathVisible {
		assert.Failf(w.t, "", "IsExecutable(%q) called on non-existent plugin (pathVisible=%t) ", binaryPath, pl.pathVisible)
		return false, assert.AnError
	}
	if pl.hasBinary {
		w.binaryPaths[filepath.ToSlash(binaryPath)] = p
		return true, nil
	}
	return false, nil
}

func (w invariantWorkspace) LoadPluginProject(ctx context.Context, path string) (*workspace.PluginProject, error) {
	pluginPath := filepath.ToSlash(filepath.Dir(path))
	pl, ok := w.plugins[pluginPath]
	if !ok {
		assert.Failf(w.t, "", "LoadPluginProject(%q) called on non-existent plugin", pluginPath)
		return nil, assert.AnError
	}
	if !pl.projectDetected {
		assert.Failf(w.t, "", "LoadPluginProject(%q) called on project before DetectPluginPathAt", path)
		return nil, assert.AnError
	}
	require.NotNil(w.t, pl.project, "We shouldn't be able to detect a nil project")
	return pl.project, nil
}

func (w invariantWorkspace) DownloadPlugin(
	ctx context.Context, plugin workspace.PluginDescriptor,
) (string, packageinstallation.MarkInstallationDone, error) {
	p := filepath.ToSlash(filepath.Join("$HOME", ".pulumi", "plugins", plugin.Dir(), plugin.SubDir()))
	pl, ok := w.plugins[p]
	if !ok {
		assert.Failf(w.t, "Unknown plugin", "could not find %q in %#v", p, slices.Collect(maps.Keys(w.plugins)))
		return "", nil, assert.AnError
	}
	pl.downloaded = true
	pl.pathVisible = true
	return p, func(success bool) {}, nil
}

func (w invariantWorkspace) DetectPluginPathAt(ctx context.Context, path string) (string, error) {
	path = filepath.ToSlash(path)
	pl, ok := w.plugins[path]
	if !ok || !pl.pathVisible {
		assert.Failf(w.t, "", "DetectPluginPathAt(%q) called on non-existent plugin (pathVisible=%t)", path, pl.pathVisible)
		return "", assert.AnError
	}
	if pl.project == nil {
		return "", workspace.ErrBaseProjectNotFound
	}
	pl.projectDetected = true
	return filepath.ToSlash(filepath.Join(path, "PulumiPlugin.yaml")), nil
}

func (w invariantWorkspace) LinkPackage(
	ctx context.Context,
	project *workspace.ProjectRuntimeInfo, projectDir string, packageName tokens.Package,
	pluginPath string, params plugin.ParameterizeParameters,
	originalSpec workspace.PackageSpec,
) error {
	projectDir = filepath.ToSlash(projectDir)
	pluginPath = filepath.ToSlash(pluginPath)

	var links *[]string
	if dst, ok := w.plugins[projectDir]; ok {
		if !dst.downloaded {
			assert.Failf(w.t, "", "LinkPackage(%q) called on non-downloaded dst", projectDir)
			return assert.AnError
		}
		links = &dst.linked
	} else if workDir, ok := w.downloadedWorkspace[projectDir]; ok {
		links = &workDir.linked
	} else {
		assert.Failf(w.t, "Unknown plugin", "could not find %q in %#v", pluginPath, slices.Collect(maps.Keys(w.plugins)))
		return assert.AnError
	}

	actualPluginPath := pluginPath
	if binPath, ok := w.binaryPaths[pluginPath]; ok {
		actualPluginPath = binPath
	}

	src, ok := w.plugins[actualPluginPath]
	if !ok || !src.downloaded {
		assert.Failf(w.t, "",
			"LinkPackage(%q) called on non-existent src (downloaded=%t)",
			actualPluginPath, src != nil && src.downloaded)
		return assert.AnError
	}

	w.rw.Lock()
	defer w.rw.Unlock()
	if slices.Contains(*links, actualPluginPath) {
		assert.Failf(w.t, "", "LinkPackage(%q) linked %q >1 time", projectDir, actualPluginPath)
		return assert.AnError
	}
	// Insert in sorted order to ensure deterministic comparison
	pos, _ := slices.BinarySearch(*links, actualPluginPath)
	*links = slices.Insert(*links, pos, actualPluginPath)
	return nil
}

func (w invariantWorkspace) RunPackage(
	ctx context.Context,
	rootDir, pluginPath string, pkgName tokens.Package, params plugin.ParameterizeParameters,
) (plugin.Provider, error) {
	pluginPath = filepath.ToSlash(pluginPath)

	if p, ok := w.binaryPaths[pluginPath]; ok {
		pluginPath = p
	}
	pl, ok := w.plugins[pluginPath]
	if !ok {
		assert.Failf(w.t, "Unknown plugin", "could not find %q in %#v", pluginPath, slices.Collect(maps.Keys(w.plugins)))
		return nil, assert.AnError
	}
	if !pl.installed && pl.project != nil {
		assert.Failf(w.t, "", "Missing setup for %q (installed=%t) (project=%t)",
			pluginPath, pl.installed, pl.project != nil)
		return nil, assert.AnError
	}
	return nil, nil
}
