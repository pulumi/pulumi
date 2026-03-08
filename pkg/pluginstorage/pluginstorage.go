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

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

var Instance Context = defaultContext{}

type Context interface {
	HasPlugin(ctx context.Context, spec workspace.PluginDescriptor) bool
	HasPluginGTE(ctx context.Context, spec workspace.PluginDescriptor) (bool, *semver.Version, error)
	GetLatestVersion(ctx context.Context, spec workspace.PluginDescriptor) (*semver.Version, error)
}

type defaultContext struct{}

func (defaultContext) HasPlugin(_ context.Context, spec workspace.PluginDescriptor) bool {
	return workspace.HasPlugin(spec)
}

func (defaultContext) HasPluginGTE(_ context.Context, spec workspace.PluginDescriptor) (bool, *semver.Version, error) {
	return workspace.HasPluginGTE(spec)
}

func (defaultContext) GetLatestVersion(ctx context.Context, spec workspace.PluginDescriptor) (*semver.Version, error) {
	return spec.GetLatestVersion(ctx)
}

var _ Context = MockContext{}

type MockContext struct {
	HasPluginF        func(ctx context.Context, spec workspace.PluginDescriptor) bool
	HasPluginGTEF     func(ctx context.Context, spec workspace.PluginDescriptor) (bool, *semver.Version, error)
	GetLatestVersionF func(ctx context.Context, spec workspace.PluginDescriptor) (*semver.Version, error)
}

func (m MockContext) HasPlugin(ctx context.Context, spec workspace.PluginDescriptor) bool {
	if m.HasPluginF != nil {
		return m.HasPluginF(ctx, spec)
	}
	return false
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
