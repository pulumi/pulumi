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
	"errors"
	"fmt"
	"time"
)

// AuthContext describes the token grant used to log in to a backend.
type AuthContext struct {
	GrantType    string
	Organization string
	Scope        string
	Token        string
	TokenExpired bool
	Expiration   time.Duration
}

// AuthContextGrantTypeTokenExchange identifies an OAuth token-exchange grant.
//
//nolint:gosec // This is an OAuth grant type URN, not a credential
const AuthContextGrantTypeTokenExchange = "urn:ietf:params:oauth:grant-type:token-exchange"

// NewAuthContextForTokenExchange validates OIDC login inputs and prepares the token exchange.
// accessToken is the access token supplied by the caller's environment.
func NewAuthContextForTokenExchange(
	organization, team, user, token, expirationDuration, accessToken string,
) (AuthContext, error) {
	if token == "" {
		return AuthContext{}, errors.New("oidc token must be specified for token exchange")
	}
	if accessToken != "" {
		return AuthContext{}, errors.New("cannot perform token exchange when an access token is set as environment variable")
	}
	if organization == "" {
		return AuthContext{}, errors.New("organization must be specified for token exchange")
	}
	if team != "" && user != "" {
		return AuthContext{}, errors.New("only one of team or user may be specified for token exchange")
	}
	scope := ""
	if team != "" {
		scope = "team:" + team
	}
	if user != "" {
		scope = "user:" + user
	}
	expiration := 2 * time.Hour
	if expirationDuration != "" {
		duration, err := time.ParseDuration(expirationDuration)
		if err != nil {
			return AuthContext{}, fmt.Errorf("could not parse expiration duration: %w", err)
		}
		expiration = duration
	}
	return AuthContext{
		GrantType:    AuthContextGrantTypeTokenExchange,
		Organization: organization,
		Scope:        scope,
		Token:        token,
		Expiration:   expiration,
	}, nil
}
