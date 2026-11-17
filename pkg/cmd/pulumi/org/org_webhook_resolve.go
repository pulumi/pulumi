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

package org

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
)

// resolveOrgName returns the org name to use, falling back to the default
// org or current user if orgFlag is empty.
func resolveOrgName(ctx context.Context, orgFlag string, be httpstate.Backend) (string, error) {
	if orgFlag != "" {
		return orgFlag, nil
	}
	orgName, err := be.GetDefaultOrg(ctx)
	if err != nil {
		return "", fmt.Errorf("resolving default org: %w", err)
	}
	if orgName != "" {
		return orgName, nil
	}
	userName, _, _, err := be.CurrentUser()
	if err != nil {
		return "", err
	}
	return userName, nil
}
