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

package workspace

import (
	"context"
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Context is an interface that represents the context of a workspace. It provides access to loading projects and
// plugins.
type Context interface {
	// New creates a new workspace rooted at the given directory. Requires a Pulumi.yaml file be present
	// in the folder hierarchy between dir and the .pulumi folder. If dir is empty or relative, it is
	// resolved against the process working directory.
	New(dir string) (W, error)

	// ReadProject attempts to detect and read a Pulumi project by searching upwards from the given
	// directory. If the project is successfully detected and read, it is returned along with the path
	// to its containing directory, which will be used as the root of the project's Pulumi program.
	// If dir is empty or relative, it is resolved against the process working directory.
	ReadProject(dir string) (*workspace.Project, string, error)

	// LoadPluginProjectAt reads a plugin project definition in the given directory. If no project is found,
	// [workspace.ErrPluginNotFound] is returned.
	//
	// LoadPluginProjectAt does not search upwards from path.
	LoadPluginProjectAt(ctx context.Context, path string) (*workspace.PluginProject, string, error)

	// Detect the nearest enclosing Pulumi Project or Pulumi Plugin root directory.
	//
	// The returned [BaseProject] will be one of:
	// - *[PluginProject]
	// - *[Project]
	//
	// The returned string is the path to the returned file. If no plugin or project is found
	// upwards of path, then [ErrBaseProjectNotFound] will be returned.
	LoadBaseProjectFrom(ctx context.Context, path string) (workspace.BaseProject, string, error)

	// GetStoredCredentials returns any credentials stored on the local machine.
	GetStoredCredentials() (Credentials, error)
}

var Instance Context = &workspaceContext{}

type workspaceContext struct{}

func (*workspaceContext) New(dir string) (W, error) {
	return newW(dir)
}

func (*workspaceContext) ReadProject(dir string) (*workspace.Project, string, error) {
	path, err := workspace.DetectProjectPathFrom(dir)
	if err != nil {
		return nil, "", err
	}
	proj, err := workspace.LoadProject(path)
	if err != nil {
		return nil, "", err
	}

	return proj, filepath.Dir(path), nil
}

func (*workspaceContext) GetStoredCredentials() (Credentials, error) {
	return GetStoredCredentials()
}

func (*workspaceContext) LoadPluginProjectAt(_ context.Context, path string) (*workspace.PluginProject, string, error) {
	path, err := workspace.DetectPluginPathAt(path)
	if err != nil {
		return nil, "", err
	}
	proj, err := workspace.LoadPluginProject(path)
	if err != nil {
		return nil, "", err
	}
	return proj, path, err
}

func (*workspaceContext) LoadBaseProjectFrom(ctx context.Context, path string) (workspace.BaseProject, string, error) {
	return workspace.LoadBaseProjectFrom(path)
}
