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
	"hash/fnv"
	"path/filepath"
	"slices"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageinstallation"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
)

var _ packageinstallation.Workspace = invariantWorkspace{}

func newInvariantWorkspace(t *testing.T, plugins ...invariantPlugin) *invariantWorkspace {
	pluginMap := make(map[string]*invariantPlugin)
	for i := range plugins {
		p := filepath.Join("$HOME", ".pulumi", "plugins", plugins[i].d.Dir(), plugins[i].d.SubDir())
		pluginMap[p] = &plugins[i]
	}
	return &invariantWorkspace{
		t:           t,
		plugins:     pluginMap,
		binaryPaths: map[string]string{},
	}
}

type invariantWorkspace struct {
	t *testing.T
	// All plugins visible to the test
	plugins map[string]*invariantPlugin

	// A map of binary paths to the plugin directories they contain.
	binaryPaths map[string]string
}

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

type pluginID = uint64

func getPluginID(w workspace.PluginDescriptor) pluginID {
	h := fnv.New64a()
	h.Write([]byte(w.Name))
	h.Write([]byte(w.Kind))
	h.Write([]byte(w.PluginDownloadURL))
	return h.Sum64()
}

func (w invariantWorkspace) HasPlugin(spec workspace.PluginDescriptor) bool {
	for _, candidate := range w.plugins {
		if candidate.downloaded && candidate.installed &&
			candidate.d.Name == spec.Name &&
			candidate.d.Kind == spec.Kind &&
			(candidate.d.Version == nil && spec.Version == nil || candidate.d.Version.EQ(*spec.Version)) {
			return true
		}
	}
	return false
}

func (w invariantWorkspace) HasPluginGTE(spec workspace.PluginDescriptor) (bool, *semver.Version, error) {
	var gte *workspace.PluginDescriptor
	for _, candidate := range w.plugins {
		if candidate.downloaded && candidate.installed &&
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

func (w invariantWorkspace) GetLatestVersion(ctx context.Context, spec workspace.PluginDescriptor) (*semver.Version, error) {
	return &semver.Version{Major: getPluginID(spec) % 10}, nil
}

func (w invariantWorkspace) GetPluginPath(ctx context.Context, plugin workspace.PluginDescriptor) (string, error) {
	p := filepath.Join("$HOME", ".pulumi", "plugins", plugin.Dir(), plugin.SubDir())
	pl, ok := w.plugins[p]
	if !ok || !pl.downloaded {
		assert.Fail(w.t, "GetPluginPath() called on non-present plugin")
		return "", assert.AnError
	}
	pl.pathVisible = true
	return p, nil
}

func (w invariantWorkspace) InstallPluginAt(ctx context.Context, dirPath string, project *workspace.PluginProject) error {
	p, ok := w.plugins[dirPath]
	if !ok || !p.downloaded {
		assert.Fail(w.t, "InstallPluginAt(%q) called on non-revealed plugin dir", dirPath)
		return assert.AnError
	}
	assert.False(w.t, p.installed, "InstallPluginAt(%q) called in already installed dir", dirPath)
	p.installed = true
	return nil
}

func (w invariantWorkspace) IsExecutable(ctx context.Context, binaryPath string) (bool, error) {
	p := filepath.Dir(binaryPath)
	pl, ok := w.plugins[p]
	if !ok || !pl.pathVisible {
		assert.Fail(w.t, "IsExecutable(%q) called on non-existant plugin (pathVisible=%t) ", binaryPath, pl.pathVisible)
		return false, assert.AnError
	}
	if pl.hasBinary {
		w.binaryPaths[binaryPath] = p
		return true, nil
	}
	return false, nil
}

func (w invariantWorkspace) LoadPluginProject(ctx context.Context, path string) (*workspace.PluginProject, error) {
	pl, ok := w.plugins[path]
	if !ok || !pl.projectDetected {
		assert.Fail(w.t, "LoadPluginProject(%q) called on non-existent plugin (projectDetected=%t)", path, pl.projectDetected)
		return nil, assert.AnError
	}
	if pl.project == nil {
		return nil, workspace.ErrBaseProjectNotFound
	}
	return pl.project, nil
}

func (w invariantWorkspace) DownloadPlugin(ctx context.Context, plugin workspace.PluginDescriptor) (string, packageinstallation.MarkInstallationDone, error) {
	p := filepath.Join("$HOME", ".pulumi", "plugins", plugin.Dir(), plugin.SubDir())
	w.plugins[p].downloaded = true
	return p, func(success bool) { /* TODO: Make sure that this is always called with the correct value */ }, nil
}

func (w invariantWorkspace) DetectPluginPathAt(ctx context.Context, path string) (string, error) {
	pl, ok := w.plugins[path]
	if !ok || !pl.pathVisible {
		assert.Fail(w.t, "LoadPluginProject(%q) called on non-existent plugin (pathVisible=%t)", path)
		return "", assert.AnError
	}
	if pl.project == nil {
		return "", workspace.ErrBaseProjectNotFound
	}
	pl.projectDetected = true
	return filepath.Join(path, "PulumiPlugin.yaml"), nil
}

func (w invariantWorkspace) LinkPackage(
	ctx context.Context,
	project *workspace.ProjectRuntimeInfo, projectDir string, packageName tokens.Package,
	pluginPath string, params plugin.ParameterizeParameters,
	originalSpec workspace.PackageSpec,
) error {
	dst, ok := w.plugins[projectDir]
	if !ok || !dst.downloaded {
		assert.Fail(w.t, "LinkPackage(%q) called on non-existent dst (downloaded=%t)", projectDir, dst.downloaded)
		return assert.AnError
	}

	src, ok := w.plugins[pluginPath]
	if !ok || !src.installed {
		assert.Fail(w.t, "LinkPackage(%q) called on non-existent src (installed=%t)", projectDir, src.installed)
		return assert.AnError
	}

	if slices.Contains(dst.linked, pluginPath) {
		assert.Fail(w.t, "LinkPackage(%q) linked %q >1 time", projectDir, pluginPath)
		return assert.AnError
	}
	dst.linked = append(dst.linked, pluginPath)
	return nil
}

func (w invariantWorkspace) RunPackage(
	ctx context.Context,
	rootDir, pluginPath string, pkgName tokens.Package, params plugin.ParameterizeParameters,
) (plugin.Provider, error) {
	p, ok := w.binaryPaths[pluginPath]
	if ok {
		pluginPath = p
	}
	pl, ok := w.plugins[p]
	if !ok || !pl.installed {
		assert.Fail(w.t, "Could not find plugin %q", pluginPath)
		return nil, assert.AnError
	}
	return nil, nil
}
