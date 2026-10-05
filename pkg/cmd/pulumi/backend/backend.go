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
	"os"
	"slices"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/auth"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	backendSecrets "github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func configuredBackendURL(ws pkgWorkspace.Context, e env.Env, project *workspace.Project) (string, bool, error) {
	url, err := pkgWorkspace.GetCurrentCloudURLWithAgentFallback(ws, e, project)
	if err != nil {
		return "", false, err
	}
	setCurrent := url == ""
	if url == "" {
		url = e.GetString(env.APIURL)
	}
	return url, setCurrent, nil
}

// PrepareBackend reads the current project and prepares its selected backend, returning the backend URL.
func PrepareBackend(ctx context.Context, ws pkgWorkspace.Context, lm LoginManager) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current working directory: %w", err)
	}

	// Try to read the current project
	project, _, err := ws.ReadProject(cwd)
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return "", err
	}

	url, _, err := PrepareCurrentBackend(ctx, lm, ws, env.Global(), project)
	if err != nil {
		return "", fmt.Errorf("could not get cloud url: %w", err)
	}

	return url, nil
}

func NonInteractiveCurrentBackend(
	ctx context.Context, ws pkgWorkspace.Context, lm LoginManager, project *workspace.Project,
) (backend.Backend, error) {
	url, setCurrent, err := PrepareCurrentBackend(ctx, lm, ws, env.Global(), project)
	if err != nil {
		return nil, fmt.Errorf("could not get cloud url: %w", err)
	}

	return lm.Current(ctx, ws, cmdutil.Diag(), url, project, setCurrent)
}

func CurrentBackend(
	ctx context.Context, ws pkgWorkspace.Context, lm LoginManager, project *workspace.Project,
	opts display.Options,
) (backend.Backend, error) {
	url, setCurrent, err := PrepareCurrentBackend(ctx, lm, ws, env.Global(), project)
	if err != nil {
		return nil, fmt.Errorf("could not get cloud url: %w", err)
	}
	insecure := pkgWorkspace.GetCloudInsecure(ws, url)

	return lm.Login(ctx, ws, cmdutil.Diag(), url, project, LoginOptions{
		SetCurrent: setCurrent, Insecure: insecure, Color: opts.Color,
	})
}

// PluginCredentials returns the opened backend's cloud credentials, or nil for other backends.
func PluginCredentials(be backend.Backend) *auth.Credentials {
	if cloud, ok := be.(httpstate.Backend); ok {
		return &auth.Credentials{
			BackendURL: cloud.CloudURL(),
			HTTPAuth:   cloud.Client().HTTPAuth(),
		}
	}
	return nil
}

// PrepareCurrentBackend resolves the selected backend without logging in or saving the selection.
func PrepareCurrentBackend(ctx context.Context, lm LoginManager, ws pkgWorkspace.Context, e env.Env,
	project *workspace.Project,
) (string, bool, error) {
	url, setCurrent, err := configuredBackendURL(ws, e, project)
	if err != nil {
		return "", false, err
	}
	credentials, err := lm.ResolveCredentials(ctx, ws, url, project)
	if err != nil {
		return "", false, err
	}
	return credentials.BackendURL, setCurrent, nil
}

// ResolveCurrentCredentials prepares the current project's credentials for plugins without logging in.
// A nil project discovers the project in the current directory.
func ResolveCurrentCredentials(ctx context.Context, ws pkgWorkspace.Context, lm LoginManager, e env.Env,
	project *workspace.Project,
) (*auth.Credentials, error) {
	if project == nil {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		project, _, err = ws.ReadProject(cwd)
		if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
			return nil, err
		}
	}
	url, _, err := configuredBackendURL(ws, e, project)
	if err != nil {
		return nil, err
	}
	return lm.ResolveCredentials(ctx, ws, url, project)
}

// ValidateLoginBackendURL checks the backend formats accepted by pulumi login.
func ValidateLoginBackendURL(cloudURL string) error {
	url := strings.TrimPrefix(strings.TrimPrefix(cloudURL, "https://"), "http://")
	if strings.HasPrefix(url, "app.pulumi.com/") || strings.HasPrefix(url, "pulumi.com") {
		return fmt.Errorf("%s is not a valid self-hosted backend, "+
			"use `pulumi login` without arguments to log into the Pulumi Cloud backend", cloudURL)
	}
	return validateCloudBackendType(cloudURL)
}

// validateCloudBackendType checks whether the backend scheme is supported.
func validateCloudBackendType(typ string) error {
	kind, _, _ := strings.Cut(typ, ":")
	supportedKinds := []string{"azblob", "gs", "s3", "file", "https", "http", "postgres"}
	if slices.Contains(supportedKinds, kind) {
		return nil
	}
	return fmt.Errorf("unknown backend cloudUrl format '%s' (supported Url formats are: "+
		"azblob://, gs://, s3://, file://, https://, http:// and postgres://)",
		kind)
}

// SecretsProvider reconstructs secrets managers with credentials from the command's login manager.
func SecretsProvider(lm LoginManager) secrets.Provider {
	return backendSecrets.NewProvider(func(ctx context.Context, url string) (*auth.Credentials, error) {
		return lm.ResolveCredentials(ctx, pkgWorkspace.Instance, url, nil)
	})
}
