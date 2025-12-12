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

// Package pluginstorage provides a unified interface to pulumi managed plugin storage and
// retrieval.
package pluginstorage

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func Default() (Store, error) {
	pluginDir, err := workspace.GetPluginDir()
	if err != nil {
		return nil, err
	}
	return NewFs(pluginDir), nil
}

// Store describes the imperative layer of our plugin storage, abstracted from the file
// system.
//
// All [workspace.PluginSpec]s are assumed to be fully resolved.
type Store interface {
	// Acquire a global cross-process lock on a spec.
	//
	// If a non-nil error is returned, then you **must** call Unlock to avoid leaking
	// the lock.
	LockSpec(ctx context.Context, spec workspace.PluginSpec) (Lock, error)

	// Mark a plugin as in partial status.
	SetPartial(ctx context.Context, spec workspace.PluginSpec) error
	// Remove partial status from a plugin.
	RemovePartial(ctx context.Context, spec workspace.PluginSpec) error
	// Check if a plugin is currently partial.
	IsPartial(ctx context.Context, spec workspace.PluginSpec) (bool, error)

	// List installed plugins.
	List(ctx context.Context) ([]workspace.PluginInfo, error)

	// Dir returns the absolute file path to an installed plugin.
	Dir(ctx context.Context, spec workspace.PluginSpec) (string, error)

	// Write contents for a spec.
	//
	// Write takes ownership of [Content], closing it when done.
	Write(ctx context.Context, spec workspace.PluginSpec, content Content) error
}

type Lock interface {
	Unlock() error
	isLock()
}
