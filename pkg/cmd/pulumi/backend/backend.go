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
	"log/slog"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/auth"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/httputil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// PrepareCurrentBackend resolves the configured backend and prepares it with the session.
func PrepareCurrentBackend(
	ctx context.Context, session *auth.Session, ws pkgWorkspace.Context, e env.Env, project *workspace.Project,
) (string, bool, error) {
	url, err := pkgWorkspace.GetCurrentCloudURLWithAgentFallback(ws, e, project)
	if err != nil {
		return "", false, err
	}
	setCurrent := url == ""
	if url == "" {
		url = e.GetString(env.APIURL)
	}
	logging.AddGlobalSecretFilter(httputil.URLSecrets(url), "[credential]")
	if url != "" {
		url, err = session.PrepareBackend(ctx, url)
	} else {
		url, err = session.PrepareBackendWithFallback(ctx, client.PulumiCloudURL)
	}
	if err != nil {
		return "", false, err
	}
	logging.AddGlobalSecretFilter(httputil.URLSecrets(url), "[credential]")
	slog.InfoContext(ctx, "Current cloud URL", "url", url)
	return url, setCurrent, nil
}

// acceptHelperSelection rejects a backend the credential helper selected that the CLI cannot open.
func acceptHelperSelection(backendURL string) error {
	if !diy.IsDIYBackendURL(backendURL) && !auth.IsHTTPBackend(backendURL) {
		return errors.New("credential helper returned an unsupported backendUrl")
	}
	return nil
}

func IsDIYBackend(ctx context.Context, ws pkgWorkspace.Context, lm LoginManager) (bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("getting current working directory: %w", err)
	}

	// Try to read the current project
	project, _, err := ws.ReadProject(cwd)
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return false, err
	}

	url, _, err := PrepareCurrentBackend(ctx, lm.Session(), ws, env.Global(), project)
	if err != nil {
		return false, fmt.Errorf("could not get cloud url: %w", err)
	}

	return diy.IsDIYBackendURL(url), nil
}

func NonInteractiveCurrentBackend(
	ctx context.Context, ws pkgWorkspace.Context, lm LoginManager, project *workspace.Project,
) (backend.Backend, error) {
	url, setCurrent, err := PrepareCurrentBackend(ctx, lm.Session(), ws, env.Global(), project)
	if err != nil {
		return nil, fmt.Errorf("could not get cloud url: %w", err)
	}

	return lm.Current(ctx, ws, cmdutil.Diag(), url, project, setCurrent)
}

func CurrentBackend(
	ctx context.Context, ws pkgWorkspace.Context, lm LoginManager, project *workspace.Project,
	opts display.Options,
) (backend.Backend, error) {
	url, setCurrent, err := PrepareCurrentBackend(ctx, lm.Session(), ws, env.Global(), project)
	if err != nil {
		return nil, fmt.Errorf("could not get cloud url: %w", err)
	}
	insecure := pkgWorkspace.GetCloudInsecure(ws, url)

	return lm.Login(ctx, ws, cmdutil.Diag(), url, project, setCurrent, insecure, opts.Color)
}
