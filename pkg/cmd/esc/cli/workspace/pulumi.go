// Copyright 2023, Pulumi Corporation.
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

import "github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

type PulumiWorkspace interface {
	DeleteAccount(backendURL string) error
	DeleteAllAccounts() error
	SetBackendConfigDefaultOrg(backendURL, defaultOrg string) error
	GetPulumiConfig() (workspace.PulumiConfig, error)
	GetPulumiPath(elem ...string) (string, error)
	GetStoredCredentials() (workspace.Credentials, error)
	StoreAccount(key string, account workspace.Account, current bool) error
	GetAccount(key string) (workspace.Account, error)
}

type defaultPulumiWorkspace int

func DefaultPulumiWorkspace() PulumiWorkspace {
	return defaultPulumiWorkspace(0)
}

func (defaultPulumiWorkspace) DeleteAccount(backendURL string) error {
	return workspace.DeleteAccount(backendURL)
}

func (defaultPulumiWorkspace) DeleteAllAccounts() error {
	return workspace.DeleteAllAccounts()
}

func (defaultPulumiWorkspace) SetBackendConfigDefaultOrg(backendURL, defaultOrg string) error {
	return workspace.SetBackendConfigDefaultOrg(backendURL, defaultOrg)
}

func (defaultPulumiWorkspace) GetPulumiConfig() (workspace.PulumiConfig, error) {
	return workspace.GetPulumiConfig()
}

func (defaultPulumiWorkspace) GetPulumiPath(elem ...string) (string, error) {
	return workspace.GetPulumiPath(elem...)
}

func (defaultPulumiWorkspace) GetStoredCredentials() (workspace.Credentials, error) {
	return workspace.GetStoredCredentials()
}

func (defaultPulumiWorkspace) StoreAccount(key string, account workspace.Account, current bool) error {
	return workspace.StoreAccount(key, account, current)
}

func (defaultPulumiWorkspace) GetAccount(key string) (workspace.Account, error) {
	return workspace.GetAccount(key)
}
