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
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type orgWebhookRemoveCmd struct {
	orgName string
	yes     bool
	w       io.Writer

	currentBackend func(
		context.Context, pkgWorkspace.Context, cmdBackend.LoginManager,
		*workspace.Project, display.Options,
	) (backend.Backend, error)
}

func newOrgWebhookRemoveCmd() *cobra.Command {
	orcmd := &orgWebhookRemoveCmd{}

	cmd := &cobra.Command{
		Use:   "remove",
		Short: "[EXPERIMENTAL] Delete an organization webhook",
		Long: "[EXPERIMENTAL] Delete an organization webhook.\n" +
			"\n" +
			"Permanently removes the specified webhook from the organization.\n" +
			"This cannot be undone. You will be prompted to confirm unless\n" +
			"--yes is passed.\n" +
			"\n" +
			"Returns an error if the webhook does not exist.",
		Example: "  # Remove a webhook (will prompt for confirmation)\n" +
			"  pulumi org webhook remove my-webhook\n\n" +
			"  # Remove without confirmation\n" +
			"  pulumi org webhook remove my-webhook --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			orcmd.w = cmd.OutOrStdout()
			return orcmd.run(cmd.Context(), args[0])
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "name"}},
		Required:  1,
	})

	cmd.Flags().StringVar(&orcmd.orgName, "org", "",
		"The organization that owns the webhook. Defaults to the current org.")
	cmd.Flags().BoolVarP(&orcmd.yes, "yes", "y", false,
		"Skip confirmation prompts")

	return cmd
}

func (c *orgWebhookRemoveCmd) run(ctx context.Context, webhookName string) error {
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	if !c.yes {
		prompt := fmt.Sprintf(
			"This will permanently remove the webhook '%s'!", webhookName)
		if !ui.ConfirmPrompt(prompt, webhookName, opts) {
			return result.FprintBailf(c.w, "confirmation declined")
		}
	}

	currentBackend := c.currentBackend
	if currentBackend == nil {
		currentBackend = cmdBackend.CurrentBackend
	}

	ws := pkgWorkspace.Instance
	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return err
	}

	be, err := currentBackend(ctx, ws, cmdBackend.DefaultLoginManager, project, opts)
	if err != nil {
		return err
	}
	cloudBackend, ok := be.(httpstate.Backend)
	if !ok {
		return errors.New("this command requires the Pulumi Cloud backend; run `pulumi login`")
	}

	orgName := c.orgName
	if orgName == "" {
		orgName, err = cloudBackend.GetDefaultOrg(ctx)
		if err != nil {
			return fmt.Errorf("resolving default org: %w", err)
		}
		if orgName == "" {
			userName, _, _, err := cloudBackend.CurrentUser()
			if err != nil {
				return err
			}
			orgName = userName
		}
	}

	if err := cloudBackend.Client().DeleteOrgWebhook(ctx, orgName, webhookName); err != nil {
		return fmt.Errorf("removing organization webhook: %w", err)
	}

	fmt.Fprintf(c.w, "Webhook '%s' has been removed.\n", webhookName)
	return nil
}
