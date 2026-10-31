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

package backend

import (
	"context"
	"errors"
	"fmt"
	"strings"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// PluginCloudCredentialEnv resolves environment or stored credentials for a new plugin context.
func PluginCloudCredentialEnv(
	_ context.Context, ws pkgWorkspace.Context, _ LoginManager, project *workspace.Project,
) (map[string]string, error) {
	if project == nil {
		var err error
		project, _, err = ws.ReadProject("")
		if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
			return nil, err
		}
	}
	url, err := pkgWorkspace.GetCurrentCloudURLWithAgentFallback(ws, env.Global(), project)
	if err != nil {
		return nil, fmt.Errorf("could not get cloud url: %w", err)
	}
	if url == "" && workspace.AgentCredentialsFallbackEnabled() {
		if creds, err := workspace.GetAgentStoredCredentials(); err == nil {
			url = creds.Current
		}
	}
	return pulumiCloudCredentialEnv(env.Global(), url), nil
}

// pulumiCloudCredentialEnv resolves the Pulumi Cloud API address and token passed to plugins.
// It returns nil for non-cloud backends and when logged out.
func pulumiCloudCredentialEnv(store env.Env, url string) map[string]string {
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
