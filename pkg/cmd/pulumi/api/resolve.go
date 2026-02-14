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

package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

// ResolvedContext contains the resolved auth and org context for API calls.
type ResolvedContext struct {
	CloudURL string
	Token    string
	OrgName  string
}

// ResolveContext resolves auth credentials and organization from local config.
// orgFlag is the value of --org if provided; if empty, falls back to default org.
// needsOrg indicates whether this operation requires an org (has {orgName} in path).
func ResolveContext(orgFlag string, needsOrg bool) (*ResolvedContext, error) {
	ctx := context.Background()

	// 1. Get cloud URL.
	cloudURL := httpstate.ValueOrDefaultURL(pkgWorkspace.Instance, "")
	if cloudURL == "" {
		return nil, errors.New("not logged in; run 'pulumi login' first")
	}

	// 2. Get token via LoginManager, which handles PULUMI_ACCESS_TOKEN
	// precedence, token validation, and credential storage.
	account, err := httpstate.NewLoginManager().Current(ctx, cloudURL, false, true)
	if err != nil {
		return nil, fmt.Errorf("resolving credentials: %w", err)
	}
	if account == nil || account.AccessToken == "" {
		return nil, fmt.Errorf("no access token for %s; run 'pulumi login' first", cloudURL)
	}

	// 3. Resolve org.
	var orgName string
	if orgFlag != "" {
		orgName = orgFlag
	} else if needsOrg {
		orgName, err = pkgWorkspace.GetBackendConfigDefaultOrg(nil)
		if err != nil {
			return nil, fmt.Errorf("resolving default org: %w", err)
		}
		if orgName == "" {
			return nil, errors.New("provide --org flag or set a default with 'pulumi org set-default'")
		}
	}

	return &ResolvedContext{
		CloudURL: cloudURL,
		Token:    account.AccessToken,
		OrgName:  orgName,
	}, nil
}
