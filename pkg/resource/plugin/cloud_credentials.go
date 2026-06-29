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

package plugin

import (
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// pulumiCloudCredentialEnv resolves the Pulumi Cloud API address and access token to expose to
// plugins launched with this context, so trusted providers can reach the cloud on the user's
// behalf. It returns nil for non-cloud logins and when logged out, so plugins only ever receive
// credentials they can actually use.
func pulumiCloudCredentialEnv(store env.Env, project *workspace.Project) map[string]string {
	url := currentCloudURL(store, project)
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return nil
	}

	token := store.GetString(env.AccessToken)
	if token == "" {
		if account, _, err := workspace.GetAccountWithAgentFallback(url); err == nil {
			token = account.AccessToken
		}
	}
	if token == "" {
		return nil
	}

	return map[string]string{
		env.APIURL.Var().Name():      url,
		env.AccessToken.Var().Name(): token,
	}
}

// currentCloudURL mirrors the CLI's backend selection: PULUMI_BACKEND_URL wins, then the project's
// backend, then the active stored login, falling back to shared agent credentials.
func currentCloudURL(store env.Env, project *workspace.Project) string {
	if url := store.GetString(env.BackendURL); url != "" {
		return url
	}
	if project != nil && project.Backend != nil && project.Backend.URL != "" {
		return project.Backend.URL
	}
	if creds, err := workspace.GetStoredCredentials(); err == nil && creds.Current != "" {
		return creds.Current
	}
	if workspace.AgentCredentialsFallbackEnabled() {
		if creds, err := workspace.GetAgentStoredCredentials(); err == nil {
			return creds.Current
		}
	}
	return ""
}
