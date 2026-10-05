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

package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/auth"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Account holds the authenticated identity and backend settings for the ESC CLI.
type Account struct {
	auth.Identity
	Insecure bool

	// The URL of the account's backend.
	BackendURL string
	// The default org for the backend.
	DefaultOrg string
}

func isInvalidSelfHostedURL(url string) bool {
	url = strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
	return strings.HasPrefix(url, "app.pulumi.com/") || strings.HasPrefix(url, "pulumi.com")
}

func (esc *escCommand) checkBackendURL(url string) error {
	switch {
	case isInvalidSelfHostedURL(url):
		return fmt.Errorf("%s is not a valid self-hosted backend, "+
			"use `%s login` without arguments to log into the Pulumi Cloud backend", url, esc.command)
	case !auth.IsHTTPBackend(url):
		return fmt.Errorf("%s does not support Pulumi ESC. Pulumi ESC requires the Pulumi Cloud backend; "+
			"use `%s login` without arguments to log into the Pulumi Cloud backend", url, esc.command)
	default:
		return nil
	}
}

func (esc *escCommand) getCachedClient(ctx context.Context) error {
	// Resolve the current backend, falling back to PULUMI_API and then Pulumi Cloud.
	projectURL, err := pkgWorkspace.GetCurrentCloudURLWithAgentFallback(esc.ws, env.Global(), nil)
	if err != nil {
		projectURL = ""
	}
	// A credential helper supplies the backend's credentials, and may select the backend when nothing
	// else has. A project's backend is not used here, but still keeps the helper from selecting one.
	session := esc.session
	if session == nil {
		session = auth.NewSessionWithHelperFunc(nil)
	}
	fallback := httpstate.ValueOrDefaultURL(esc.ws, projectURL)
	selecting := projectURL == "" && env.APIURL.Value() == "" && !esc.projectSelectsBackend()
	var backendURL string
	if selecting {
		backendURL, err = session.SelectBackend(ctx, fallback, esc.checkBackendURL)
	} else {
		backendURL, err = session.PrepareBackend(ctx, fallback)
	}
	if err != nil {
		return fmt.Errorf("preparing backend: %w", err)
	}

	// Read the stored account for the backend from Pulumi's shared credentials.
	helperAuth := session.HelperHTTPAuth(backendURL)
	creds, err := esc.ws.GetStoredCredentials()
	if err != nil {
		if helperAuth == nil && !workspace.AgentCredentialsFallbackEnabled() {
			return fmt.Errorf("could not determine current cloud: %w", err)
		}
		// Supplied authentication can work without a stored account. Login handles any required persistence.
		creds = workspace.Credentials{}
	}
	if err := esc.checkBackendURL(backendURL); err != nil {
		return err
	}

	var credentials *auth.Credentials
	account, hasAccount := creds.Accounts[backendURL]
	setCurrent := selecting && backendURL != fallback
	login := valueOrDefault(esc.login, httpstate.NewLoginManager(session))
	if hasAccount {
		credentials, err = login.Current(ctx, backendURL, account.Insecure, setCurrent)
	} else {
		credentials, err = login.Login(ctx, backendURL, false, "esc", "Pulumi ESC environments",
			nil, setCurrent, display.Options{Color: esc.colors})
	}
	if err != nil {
		if hasAccount {
			return fmt.Errorf("getting credentials: %w", err)
		}
		return fmt.Errorf("could not determine current cloud: %w", err)
	}
	if credentials == nil {
		return fmt.Errorf("no credentials, please run `%v login` to log in", esc.command)
	}

	esc.account = Account{
		Identity: credentials.Identity, Insecure: credentials.Insecure, BackendURL: credentials.BackendURL,
	}
	esc.client = esc.newClient(esc.userAgent, *credentials)

	// Use the authenticated backend identity.
	defaultOrg, err := esc.lookupDefaultOrg(ctx, backendURL, esc.account.Username)
	if err != nil {
		return fmt.Errorf("looking up org to default to: %w", err)
	} else if defaultOrg != "" {
		esc.account.DefaultOrg = defaultOrg
	}

	return nil
}

// projectSelectsBackend reports whether the project in the working directory names a backend.
func (esc *escCommand) projectSelectsBackend() bool {
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}
	project, _, err := esc.ws.ReadProject(cwd)
	return err == nil && project.Backend != nil && project.Backend.URL != ""
}
