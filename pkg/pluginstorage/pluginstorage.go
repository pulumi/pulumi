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

// Package pluginstorage will be the definitive source for how plugins are stored and
// managed on disk.
//
// Right now, this is pending a refactor to move methods like [(workspace.PluginSpec).Dir]
// and all functions that deal with <name>.lock & <name>.partial files to this package.
package pluginstorage

import (
	"context"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

var Instance Context = defaultContext{}

type Context interface {
	HasPlugin(ctx context.Context, spec workspace.PluginDescriptor) InstallState
	HasPluginGTE(ctx context.Context, spec workspace.PluginDescriptor) (bool, *semver.Version, error)
	GetLatestVersion(ctx context.Context, spec workspace.PluginDescriptor) (*semver.Version, error)
	GetPlugins(ctx context.Context) ([]workspace.PluginInfo, error)
}

// InstallState describes if a plugin is available to run, and how.
type InstallState struct{ int }

var (
	// The plugin is not installed.
	PluginNotInstalled = InstallState{0}
	// The plugin is known to be installed on disk.
	PluginInstalled = InstallState{1}
	// The plugin is known to be attached, so it runs outside of the plugin cache.
	PluginAttached = InstallState{2}
)

// Available reports if the plugin can be used without a download.
func (s InstallState) Available() bool {
	return s == PluginInstalled || s == PluginAttached
}

func (s InstallState) GoString() string {
	switch s {
	case PluginInstalled:
		return "pluginstorage.PluginInstalled"
	case PluginAttached:
		return "pluginstorage.PluginAttached"
	case PluginNotInstalled:
		return "pluginstorage.PluginNotInstalled"
	default:
		contract.Failf("Impossible InstallState value: %#v", s.int)
		return ""
	}
}

type defaultContext struct{}

// HasPlugin reports if the plugin is available to run. A resource provider attached
// through PULUMI_DEBUG_PROVIDERS is already running, so it is available but it has no
// directory in the plugin cache.
func (defaultContext) HasPlugin(_ context.Context, spec workspace.PluginDescriptor) InstallState {
	if spec.Kind == apitype.ResourcePlugin {
		if port, err := plugin.GetProviderAttachPort(tokens.Package(spec.Name)); err == nil && port != nil {
			return PluginAttached
		}
	}
	if workspace.HasPlugin(spec) {
		return PluginInstalled
	}
	return PluginNotInstalled
}

func (defaultContext) HasPluginGTE(_ context.Context, spec workspace.PluginDescriptor) (bool, *semver.Version, error) {
	return workspace.HasPluginGTE(spec)
}

func (defaultContext) GetLatestVersion(ctx context.Context, spec workspace.PluginDescriptor) (*semver.Version, error) {
	return spec.GetLatestVersion(ctx)
}

func (defaultContext) GetPlugins(_ context.Context) ([]workspace.PluginInfo, error) {
	return workspace.GetPlugins()
}

var _ Context = MockContext{}

type MockContext struct {
	HasPluginF        func(ctx context.Context, spec workspace.PluginDescriptor) InstallState
	HasPluginGTEF     func(ctx context.Context, spec workspace.PluginDescriptor) (bool, *semver.Version, error)
	GetLatestVersionF func(ctx context.Context, spec workspace.PluginDescriptor) (*semver.Version, error)
	GetPluginsF       func(ctx context.Context) ([]workspace.PluginInfo, error)
}

func (m MockContext) HasPlugin(ctx context.Context, spec workspace.PluginDescriptor) InstallState {
	if m.HasPluginF != nil {
		return m.HasPluginF(ctx, spec)
	}
	return PluginNotInstalled
}

func (m MockContext) HasPluginGTE(ctx context.Context, spec workspace.PluginDescriptor) (bool, *semver.Version, error) {
	if m.HasPluginGTEF != nil {
		return m.HasPluginGTEF(ctx, spec)
	}
	return false, nil, nil
}

func (m MockContext) GetLatestVersion(ctx context.Context, spec workspace.PluginDescriptor) (*semver.Version, error) {
	if m.GetLatestVersionF != nil {
		return m.GetLatestVersionF(ctx, spec)
	}
	return nil, workspace.ErrGetLatestVersionNotSupported
}

func (m MockContext) GetPlugins(ctx context.Context) ([]workspace.PluginInfo, error) {
	if m.GetPluginsF != nil {
		return m.GetPluginsF(ctx)
	}
	return nil, nil
}
