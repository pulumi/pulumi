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

package stack

import (
	"context"

	"github.com/spf13/cobra"

	pkgBackend "github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const stackOrgFlagName = "org"

func getStackOrg(cmd *cobra.Command) string {
	org, err := cmd.Flags().GetString(stackOrgFlagName)
	if err != nil {
		return ""
	}
	return org
}

func currentBackendWithOrg(
	ctx context.Context,
	ws pkgWorkspace.Context,
	lm cmdBackend.LoginManager,
	project *workspace.Project,
	opts display.Options,
	org string,
) (pkgBackend.Backend, error) {
	return cmdBackend.CurrentBackendWithOrg(ctx, ws, lm, project, opts, org)
}
