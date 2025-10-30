// Copyright 2016-2024, Pulumi Corporation.
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
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Context is an interface that represents the context of a workspace. It provides access to loading projects and
// plugins.
type Context interface {
	// ReadProject attempts to detect and read a Pulumi project for the current workspace. If the
	// project is successfully detected and read, it is returned along with the path to its containing
	// directory, which will be used as the root of the project's Pulumi program.
	ReadProject() (*workspace.Project, string, error)

	// GetStoredCredentials returns any credentials stored on the local machine.
	GetStoredCredentials() (workspace.Credentials, error)
}

var Instance Context = &workspaceContext{}

type workspaceContext struct{}

func (c *workspaceContext) ReadProject() (*workspace.Project, string, error) {
	proj, path, err := workspace.DetectProjectAndPath()
	if err != nil {
		return nil, "", err
	}

	return proj, filepath.Dir(path), nil
}

func (c *workspaceContext) GetStoredCredentials() (workspace.Credentials, error) {
	return workspace.GetStoredCredentials()
}
