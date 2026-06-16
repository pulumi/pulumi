// Copyright 2024, Pulumi Corporation.
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

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// BackendInstance is used to inject a backend mock from tests.
var BackendInstance backend.Backend

func IsDIYBackend(ws pkgWorkspace.Context, opts display.Options) (bool, error) {
	if BackendInstance != nil {
		return false, nil
	}

	// Try to read the current project
	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return false, err
	}

	url, err := pkgWorkspace.GetCurrentCloudURL(ws, env.Global(), project)
	if err != nil {
		return false, fmt.Errorf("could not get cloud url: %w", err)
	}

	return diy.IsDIYBackendURL(url), nil
}

// ResolveResourceProviderEnv resolves the API address and access token to inject into resource
// provider plugins launched at parameterize time (package add, gen-sdk). The token prefers
// PULUMI_ACCESS_TOKEN, falling back to stored credentials for the active cloud URL, to match the
// cloud backend's runtime injection. Empty for DIY and logged-out logins.
func ResolveResourceProviderEnv(ws pkgWorkspace.Context) map[string]string {
	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		project = nil
	}
	url, err := pkgWorkspace.GetCurrentCloudURL(ws, env.Global(), project)
	if err != nil || url == "" || diy.IsDIYBackendURL(url) {
		return nil
	}
	token := env.AccessToken.Value()
	if token == "" {
		if creds, err := ws.GetStoredCredentials(); err == nil {
			token = creds.AccessTokens[url]
		}
	}
	return backend.ResourceProviderCredentialEnv(url, token)
}

func NonInteractiveCurrentBackend(
	ctx context.Context, ws pkgWorkspace.Context, lm LoginManager, project *workspace.Project,
) (backend.Backend, error) {
	if BackendInstance != nil {
		return BackendInstance, nil
	}

	url, err := pkgWorkspace.GetCurrentCloudURLWithAgentFallback(ws, env.Global(), project)
	if err != nil {
		return nil, fmt.Errorf("could not get cloud url: %w", err)
	}
	logging.V(7).Infof("Current cloud URL: %q", url)

	// Only set current if we don't currently have a cloud URL set.
	return lm.Current(ctx, ws, cmdutil.Diag(), url, project, url == "")
}

func CurrentBackend(
	ctx context.Context, ws pkgWorkspace.Context, lm LoginManager, project *workspace.Project,
	opts display.Options,
) (backend.Backend, error) {
	if BackendInstance != nil {
		return BackendInstance, nil
	}

	url, err := pkgWorkspace.GetCurrentCloudURLWithAgentFallback(ws, env.Global(), project)
	if err != nil {
		return nil, fmt.Errorf("could not get cloud url: %w", err)
	}
	logging.V(7).Infof("Current cloud URL: %q", url)
	insecure := pkgWorkspace.GetCloudInsecure(ws, url)

	// Only set current if we don't currently have a cloud URL set.
	return lm.Login(ctx, ws, cmdutil.Diag(), url, project, url == "", insecure, opts.Color)
}
