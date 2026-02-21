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
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type MockContext struct {
	NewF                  func() (W, error)
	NewFromDirF           func(dir string) (W, error)
	ReadProjectF          func() (*workspace.Project, string, error)
	GetStoredCredentialsF func() (workspace.Credentials, error)
	LoadPluginProjectAtF  func(ctx context.Context, path string) (*workspace.PluginProject, string, error)
	LoadBaseProjectFromF  func(ctx context.Context, path string) (workspace.BaseProject, string, error)
}

func (c *MockContext) New() (W, error) {
	if c.NewF != nil {
		return c.NewF()
	}
	return nil, workspace.ErrProjectNotFound
}

func (c *MockContext) NewFromDir(dir string) (W, error) {
	if c.NewFromDirF != nil {
		return c.NewFromDirF(dir)
	}
	return nil, workspace.ErrProjectNotFound
}

func (c *MockContext) ReadProject() (*workspace.Project, string, error) {
	if c.ReadProjectF != nil {
		return c.ReadProjectF()
	}
	return nil, "", workspace.ErrProjectNotFound
}

func (c *MockContext) GetStoredCredentials() (workspace.Credentials, error) {
	if c.GetStoredCredentialsF != nil {
		return c.GetStoredCredentialsF()
	}
	return workspace.Credentials{}, nil
}

func (c *MockContext) LoadPluginProjectAt(ctx context.Context, path string) (*workspace.PluginProject, string, error) {
	if c.LoadPluginProjectAtF != nil {
		return c.LoadPluginProjectAtF(ctx, path)
	}
	return nil, "", workspace.ErrPluginNotFound
}

func (c *MockContext) LoadBaseProjectFrom(ctx context.Context, path string) (workspace.BaseProject, string, error) {
	if c.LoadBaseProjectFromF != nil {
		return c.LoadBaseProjectFromF(ctx, path)
	}
	return nil, "", workspace.ErrBaseProjectNotFound
}

type MockW struct {
	SettingsF func() *Settings
	SaveF     func() error
}

func (m *MockW) Settings() *Settings {
	if m.SettingsF != nil {
		return m.SettingsF()
	}
	return &Settings{}
}

func (m *MockW) Save() error {
	if m.SaveF != nil {
		return m.SaveF()
	}
	return nil
}
