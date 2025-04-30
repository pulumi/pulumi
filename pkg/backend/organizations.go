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
