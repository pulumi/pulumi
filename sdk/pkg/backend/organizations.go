// Copyright 2025, Pulumi Corporation.
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

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// GetDefaultOrg returns a user's default organization, if configured.
// It will prefer the organization that the user has configured locally, falling back to making an API
// call to the backend for the backend opinion on default organization if not manually set by the user.
// Returns an empty string if the user does not have a default org explicitly configured and if the backend
// does not have an opinion on user organizations.
func GetDefaultOrg(ctx context.Context, b Backend, currentProject *workspace.Project) (string, error) {
	return getDefaultOrg(ctx, b, currentProject, pkgWorkspace.GetBackendConfigDefaultOrg)
}

func getDefaultOrg(
	ctx context.Context,
	b Backend,
	currentProject *workspace.Project,
	getBackendConfigDefaultOrgF func(*workspace.Project) (string, error),
) (string, error) {
	userConfiguredDefaultOrg, err := getBackendConfigDefaultOrgF(currentProject)
	if err != nil || userConfiguredDefaultOrg != "" {
		return userConfiguredDefaultOrg, err
	}
	// if unset, defer to the backend's opinion of what the default org should be
	return b.GetDefaultOrg(ctx)
}

// GetLegacyDefaultOrgFallback returns the current user name as an org, if the user does not have
// a default org locally configured. Returns empty string otherwise, or if the backend does not support
// organizations.
//
// IMPORTANT NOTE: This function does not return a user's default org; callers should use `GetDefaultOrg`
// instead. `GetLegacyDefaultOrgFallback` emulates legacy fall back behavior, if a default org is not set.
//
// We preserve parts of this behavior in the interest of backwards compatibility, for users who are migrating
// from older versions of the Pulumi CLI that did not always store the current selected stack with a fully qualified
// stack name. For this class of existing users, we want to ensure that we are selecting the correct organization
// as their CLI is brought up-to-date.
func GetLegacyDefaultOrgFallback(b Backend, currentProject *workspace.Project) (string, error) {
	return getLegacyDefaultOrgFallback(b, currentProject, pkgWorkspace.GetBackendConfigDefaultOrg)
}

func getLegacyDefaultOrgFallback(
	b Backend,
	currentProject *workspace.Project,
	getBackendConfigDefaultOrgF func(*workspace.Project) (string, error),
) (string, error) {
	if !b.SupportsOrganizations() {
		return "", nil
	}

	// Check if the user has explicitly configured a default organization.
	// If so, return early -- behavior can be safely modeled with a call to `GetDefaultOrg`.
	userConfiguredDefaultOrg, err := getBackendConfigDefaultOrgF(currentProject)
	if err != nil {
		return "", err
	}

	// If the user does not have a default org configured, then return their username as their org, without
	// looking up the backend opinion. This was the legacy fallback behavior we are preserving for smooth
	// migrations between CLI versions.
	if userConfiguredDefaultOrg == "" {
		user, _, _, err := b.CurrentUser()
		if err != nil {
			return "", err
		}
		return user, nil
	}

	return "", nil
}
