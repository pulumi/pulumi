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
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Account holds details about a Pulumi account: the shared Pulumi account plus the backend URL and
// default org the ESC CLI resolved it against.
type Account struct {
	workspace.Account

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
	case diy.IsDIYBackendURL(url):
		return fmt.Errorf("%s does not support Pulumi ESC. Pulumi ESC requires the Pulumi Cloud backend; "+
			"use `%s login` without arguments to log into the Pulumi Cloud backend", url, esc.command)
	default:
		return nil
	}
}

func (esc *escCommand) getCachedClient(ctx context.Context) error {
	// Resolve the current cloud URL exactly as the rest of the Pulumi CLI does (see
	// pkg/cmd/pulumi/cloud/resolve.go): honor PULUMI_BACKEND_URL, then PULUMI_API, then the
	// current stored account, then the default cloud.
	projectURL, err := pkgWorkspace.GetCurrentCloudURLWithAgentFallback(esc.ws, env.Global(), nil)
	if err != nil {
		projectURL = ""
	}
	backendURL := httpstate.ValueOrDefaultURL(esc.ws, projectURL)

	// Read the stored account for the backend from Pulumi's shared credentials.
	var acct *Account
	creds, err := esc.ws.GetStoredCredentials()
	if err != nil {
		if !workspace.AgentCredentialsFallbackEnabled() {
			return fmt.Errorf("could not determine current cloud: %w", err)
		}
		// Agent mode with unreadable credentials: treat as no stored account and defer to the
		// login manager below.
		creds = workspace.Credentials{}
	}
	if a, ok := creds.Accounts[backendURL]; ok {
		acct = &Account{Account: a, BackendURL: backendURL}
	}

	if acct == nil {
		nAccount, err := esc.login.Login(
			ctx,
			backendURL,
			false,
			"esc",
			"Pulumi ESC environments",
			nil,
			false,
			display.Options{Color: esc.colors},
		)
		if err != nil {
			return fmt.Errorf("could not determine current cloud: %w", err)
		}

		acct = &Account{
			Account: *nAccount,
		}
	}

	acct.BackendURL = backendURL
	if err := esc.checkBackendURL(acct.BackendURL); err != nil {
		return err
	}

	ok, err := esc.getCachedCredentials(ctx, acct.BackendURL, acct.Insecure)
	if err != nil {
		return fmt.Errorf("getting credentials: %w", err)
	}
	if !ok {
		return fmt.Errorf("no credentials, please run `%v login` to log in", esc.command)
	}

	esc.client = esc.newClient(esc.userAgent, acct.BackendURL, acct.AccessToken, acct.Insecure)

	defaultOrg, err := esc.lookupDefaultOrg(ctx, backendURL, acct.Username)
	if err != nil {
		return fmt.Errorf("looking up org to default to: %w", err)
	} else if defaultOrg != "" {
		esc.account.DefaultOrg = defaultOrg
	}

	return nil
}

func (esc *escCommand) getCachedCredentials(ctx context.Context, backendURL string, insecure bool) (bool, error) {
	account, err := esc.login.Current(ctx, backendURL, insecure, false)
	if err != nil {
		return false, err
	}
	if account == nil {
		return false, nil
	}

	esc.account = Account{
		Account:    *account,
		BackendURL: backendURL,
	}
	return true, nil
}
