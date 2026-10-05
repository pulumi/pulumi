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
	"context"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Credentials contains the selected backend's authentication and identity.
type Credentials struct {
	BackendURL string
	Insecure   bool
	Identity   Identity
	HTTPAuth   *HTTPAuth
	// Info describes credential origins for diagnostic output.
	Info CredentialInfo
}

// CredentialInfo describes authentication without exposing credential values.
type CredentialInfo struct {
	HelperPath        string
	AccessTokenSource string
}

// ResolveCredentials obtains credentials for a fixed backend URL without logging in.
type ResolveCredentials func(context.Context, string) (*Credentials, error)

// Identity describes the authenticated user and token without retaining credentials.
type Identity struct {
	Username         string
	Organizations    []string
	TokenInformation *workspace.TokenInformation
}

// IsHTTPBackend reports whether url names an HTTP backend, such as Pulumi Cloud, rather than a DIY one.
func IsHTTPBackend(url string) bool {
	return strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://")
}
