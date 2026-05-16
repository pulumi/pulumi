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
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func newOrgRoleRemoveCmd() *cobra.Command {
	return newOrgRoleRemoveCmdWith(defaultOrgRoleClientFactory, defaultConfirmFunc)
}

// confirmFunc abstracts the interactive confirmation prompt so tests can stub
// it out without standing up a TTY. It is expected to also reject non-interactive
// sessions by returning (false, an error explaining --yes is required).
type confirmFunc func(prompt, name string) (bool, error)

// defaultConfirmFunc uses cmdutil.Interactive + ui.ConfirmPrompt and returns an
// error when the session is non-interactive (so callers must pass --yes).
func defaultConfirmFunc(prompt, name string) (bool, error) {
	if !cmdutil.Interactive() {
		return false, errors.New(
			"--yes is required when not running in a terminal (non-interactive)")
	}
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}
	return ui.ConfirmPrompt(prompt, name, opts), nil
}

// roleRemoveRender renders the outcome of a remove operation.
type roleRemoveRender func(w io.Writer, orgName, roleID string, force bool) error

func defaultRoleRemoveOutput() outputflag.OutputFlag[roleRemoveRender] {
	return outputflag.OutputFlag[roleRemoveRender]{
		RenderForTerminal: renderRoleRemoveText,
		RenderJSON:        renderRoleRemoveJSON,
	}
}

func newOrgRoleRemoveCmdWith(factory orgRoleClientFactory, confirm confirmFunc) *cobra.Command {
	contract.Assertf(factory != nil, "factory must not be nil")
	contract.Assertf(confirm != nil, "confirm must not be nil")

	var (
		org   string
		force bool
		yes   bool
	)
	output := defaultRoleRemoveOutput()

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "remove <role-id>",
		Short:  "Delete a custom role from an organization",
		Long: "[EXPERIMENTAL] Delete a custom role from an organization.\n" +
			"\n" +
			"Removing a role revokes any permissions it had granted to members and\n" +
			"teams. If the role is currently assigned, the service rejects the delete\n" +
			"unless --force is passed.\n" +
			"\n" +
			"By default the command asks for confirmation; pass --yes to skip the\n" +
			"prompt. Both --output default and --output json report the deletion\n" +
			"outcome, with the JSON form including the organization and role id for\n" +
			"scripting.",
		Example: "  # Delete a role interactively\n" +
			"  pulumi org role remove role-123\n\n" +
			"  # Delete a role non-interactively, even if it is assigned\n" +
			"  pulumi org role remove role-123 --force --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOrgRoleRemove(
				cmd.Context(), cmd.OutOrStdout(), factory, confirm,
				org, args[0], force, yes, output.Get())
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "role-id"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&org, "org", "",
		"The organization that owns the role. Defaults to the current default organization")
	cmd.Flags().BoolVar(&force, "force", false,
		"Force deletion even if the role is currently assigned to members or teams")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false,
		"Skip the confirmation prompt and proceed with deletion")
	outputflag.VarP(cmd.Flags(), &output)

	return cmd
}

func runOrgRoleRemove(
	ctx context.Context,
	w io.Writer,
	factory orgRoleClientFactory,
	confirm confirmFunc,
	orgFlag, roleID string,
	force, yes bool,
	render roleRemoveRender,
) error {
	c, orgName, err := factory(ctx, orgFlag)
	if err != nil {
		return err
	}

	if !yes {
		prompt := fmt.Sprintf(
			"This will permanently delete role %q from organization %q.", roleID, orgName)
		ok, err := confirm(prompt, roleID)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("confirmation declined")
		}
	}

	if err := c.DeleteOrgRole(ctx, orgName, roleID, force); err != nil {
		return err
	}

	return render(w, orgName, roleID, force)
}

type roleRemoveEnvelope struct {
	Organization string `json:"organization"`
	Action       string `json:"action"`
	RoleID       string `json:"roleId"`
	Forced       bool   `json:"forced"`
}

func renderRoleRemoveJSON(w io.Writer, orgName, roleID string, force bool) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(roleRemoveEnvelope{
		Organization: orgName,
		Action:       "Removed",
		RoleID:       roleID,
		Forced:       force,
	})
}

func renderRoleRemoveText(w io.Writer, _ /*orgName*/, roleID string, _ /*force*/ bool) error {
	fmt.Fprintf(w, "Removed role %q\n", roleID)
	return nil
}
