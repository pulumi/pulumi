// Copyright 2023, Pulumi Corporation. All rights reserved.

package workspace

import "github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

type PulumiWorkspace interface {
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
