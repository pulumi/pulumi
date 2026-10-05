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

package httpstate

import (
	"context"
	"errors"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/auth"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// loginWithHTTPAuth validates supplied authentication and saves the backend selection and connection settings.
// The returned credentials share httpAuth; its token, headers, and authenticated identity are not stored.
func loginWithHTTPAuth(
	ctx context.Context, cloudURL string, insecure bool, httpAuth *auth.HTTPAuth, setCurrent bool,
) (*auth.Credentials, error) {
	contract.Requiref(httpAuth != nil, "httpAuth", "must not be nil")
	apiClient := client.NewClient(cloudURL, "", insecure, cmdutil.Diag()).WithHTTPAuth(httpAuth)
	username, organizations, tokenInfo, err := apiClient.GetPulumiAccountDetails(ctx)
	if errors.Is(err, backenderr.LoginRequiredError{}) {
		return nil, ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}
	if err := saveBackendConnection(cloudURL, insecure, setCurrent); err != nil {
		return nil, err
	}
	return &auth.Credentials{
		BackendURL: cloudURL,
		HTTPAuth:   httpAuth,
		Insecure:   insecure,
		Identity: auth.Identity{
			Username: username, Organizations: organizations, TokenInformation: tokenInfo,
		},
	}, nil
}

// LoadCredentials reads account credentials without validating them or starting a login flow.
// Stored credentials retain their refresh and persistence behavior.
func LoadCredentials(cloudURL string) (*auth.Credentials, error) {
	account, _, err := workspace.GetAccountWithAgentFallback(cloudURL)
	token := env.AccessToken.Value()
	if token != "" && (err != nil || token != account.AccessToken) {
		return newAccountCredentials(cloudURL, workspace.Account{
			AccessToken: token, Insecure: account.Insecure,
		}, nil), nil
	}
	if err != nil {
		return nil, err
	}
	if !account.HasCredential() {
		return &auth.Credentials{BackendURL: cloudURL, Insecure: account.Insecure}, nil
	}
	return newAccountCredentials(cloudURL, account, func(updated workspace.Account) error {
		return updated.Save(cloudURL, false)
	}), nil
}

func saveBackendConnection(backendURL string, insecure, setCurrent bool) error {
	credentials, err := workspace.GetStoredCredentials()
	if err != nil {
		// Unreadable credentials only matter when there is a setting to keep.
		if insecure || setCurrent {
			return err
		}
		return nil
	}
	account := credentials.Accounts[backendURL]
	if !setCurrent && account.Insecure == insecure {
		return nil
	}
	if setCurrent {
		credentials.Current = backendURL
	}
	if account.Insecure != insecure {
		if credentials.Accounts == nil {
			credentials.Accounts = make(map[string]workspace.Account)
		}
		account.Insecure = insecure
		credentials.Accounts[backendURL] = account
	}
	return workspace.StoreCredentials(credentials)
}

// newAccountCredentials keeps account refresh and persistence behind the shared HTTP authentication.
func newAccountCredentials(
	cloudURL string, account workspace.Account, persist func(workspace.Account) error,
) *auth.Credentials {
	identity := auth.Identity{
		Username: account.Username, Organizations: account.Organizations, TokenInformation: account.TokenInformation,
	}
	if account.TokenInformation != nil {
		tokenInfo := *account.TokenInformation
		account.TokenInformation = &tokenInfo
	}
	logging.AddGlobalSecretFilter([]string{account.AccessToken, account.RefreshToken}, "[credential]")
	apiClient := client.NewClient(cloudURL, account.AccessToken, account.Insecure, cmdutil.Diag())
	apiClient.WithRefresh(account.RefreshToken, func(at string, expiresAt time.Time, rt string) error {
		account.SetCredentials(at, expiresAt, rt)
		logging.AddGlobalSecretFilter([]string{account.AccessToken, account.RefreshToken}, "[credential]")
		if persist != nil {
			return persist(account)
		}
		return nil
	})
	return &auth.Credentials{
		BackendURL: cloudURL,
		Insecure:   account.Insecure,
		Identity:   identity,
		HTTPAuth:   apiClient.HTTPAuth(),
	}
}
