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

package neo

import (
	"context"
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

func NewNeoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "neo",
		Short: "Manage Neo tasks",
		Long: "Manage Neo tasks.\n" +
			"\n" +
			"Neo is Pulumi's AI assistant for infrastructure management. " +
			"Use this command to create, list, and view Neo tasks.",
		Args: cmdutil.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newCreateCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newViewCmd())

	return cmd
}

// requireCloudBackendWithNeo ensures that the user is logged into a cloud backend
// and that the backend supports Neo tasks for the given organization.
func requireCloudBackendWithNeo(
	ctx context.Context,
	orgName string,
) (httpstate.Backend, string, error) {
	ws := pkgWorkspace.Instance
	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return nil, "", err
	}

	displayOpts := display.Options{
		Color: cmdutil.GetGlobalColorization(),
	}

	currentBe, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, project, displayOpts)
	if err != nil {
		return nil, "", err
	}

	cloudBackend, isCloud := currentBe.(httpstate.Backend)
	if !isCloud {
		return nil, "", errors.New("Neo is only available with Pulumi Cloud. Please run `pulumi login` to connect to Pulumi Cloud")
	}

	// Get default org if not specified
	if orgName == "" {
		var err error
		orgName, err = backend.GetDefaultOrg(ctx, cloudBackend, project)
		if err != nil {
			return nil, "", err
		}
	}

	// Check if user has access to the organization
	userName, orgs, _, err := cloudBackend.CurrentUser()
	if err != nil {
		return nil, "", err
	}

	if orgName == "" {
		orgName = userName
	}

	// Check if it's an individual account
	if orgName == userName {
		return nil, "", fmt.Errorf("Neo is only available for organizations, not individual accounts")
	}

	// Verify user is a member of the organization
	hasAccess := false
	for _, org := range orgs {
		if org == orgName {
			hasAccess = true
			break
		}
	}
	if !hasAccess {
		return nil, "", fmt.Errorf("you are not a member of organization %q", orgName)
	}

	// Check if Neo is enabled for this backend
	// TODO: Re-enable once backend returns neo-tasks capability
	// capabilities := cloudBackend.Capabilities(ctx)
	// if !capabilities.NeoTasks {
	// 	return nil, "", fmt.Errorf("Neo tasks are not enabled for organization %q. Please contact your organization administrator", orgName)
	// }

	return cloudBackend, orgName, nil
}
