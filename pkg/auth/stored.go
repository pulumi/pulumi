// Copyright 2026, Pulumi Corporation.
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

package auth

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func SavedHelper() (*workspace.CredentialHelper, error) {
	credentials, err := workspace.GetStoredCredentials()
	if err != nil && (!workspace.AgentCredentialsFallbackEnabled() || workspace.IsUndecryptableCredentials(err)) {
		return nil, err
	}
	return credentials.CredentialHelper, nil
}

// SaveHelper saves or removes the helper configuration while preserving accounts and the current backend.
func SaveHelper(helper *workspace.CredentialHelper) error {
	credentials, err := workspace.GetStoredCredentials()
	if err != nil {
		return err
	}
	credentials.CredentialHelper = helper
	if err := workspace.StoreCredentials(credentials); err != nil {
		return fmt.Errorf("saving credential helper configuration: %w", err)
	}
	return nil
}

// SaveCurrentBackend changes the saved selection without storing helper credentials.
func SaveCurrentBackend(backendURL string) error {
	credentials, err := workspace.GetStoredCredentials()
	if err != nil {
		return err
	}
	credentials.Current = backendURL
	if err := workspace.StoreCredentials(credentials); err != nil {
		return fmt.Errorf("saving current backend: %w", err)
	}
	return nil
}
