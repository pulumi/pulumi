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
	"slices"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// orgMemberRemoveClient is the narrow subset of cloud-API operations the
// remove command needs.
type orgMemberRemoveClient interface {
	RemoveOrganizationMember(ctx context.Context, orgName, userLogin string) error
}

// orgMemberRemoveClientFactory resolves a cloud client and the organization
// the member belongs to. orgFlag carries the raw value of `--org` (empty
// means "use the default org").
type orgMemberRemoveClientFactory func(
	ctx context.Context, orgFlag string,
) (orgMemberRemoveClient, string, error)

// orgMemberRemoveArgs collects the flag values for the remove command.
type orgMemberRemoveArgs struct {
	org          string
	yes          bool
	outputFormat outputflag.OutputFlag[orgMemberRemoveRenderFunc]
}

// defaultOrgMemberRemoveOutputFormat wires the OutputFlag to the per-format
// renderers so `--output` selects between them.
func defaultOrgMemberRemoveOutputFormat() outputflag.OutputFlag[orgMemberRemoveRenderFunc] {
	return outputflag.OutputFlag[orgMemberRemoveRenderFunc]{
		RenderForTerminal: renderOrgMemberRemoveText,
		RenderJSON:        renderOrgMemberRemoveJSON,
	}
}

// newOrgMemberRemoveCmd builds `pulumi org member remove` with the production
// client factory.
func newOrgMemberRemoveCmd() *cobra.Command {
	return newOrgMemberRemoveCmdWith(defaultOrgMemberRemoveClientFactory)
}

func newOrgMemberRemoveCmdWith(factory orgMemberRemoveClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "orgMemberRemoveClientFactory must not be nil")
	var args orgMemberRemoveArgs
	args.outputFormat = defaultOrgMemberRemoveOutputFormat()

	cmd := &cobra.Command{
		Use:     "remove <user-login>",
		Aliases: []string{"rm", "delete"},
		Short:   "[EXPERIMENTAL] Remove a member from an organization",
		Long: "[EXPERIMENTAL] Remove a member from an organization.\n" +
			"\n" +
			"Removes a user from an organization. The removed user loses access to\n" +
			"all organization resources including stacks, teams, and projects.\n" +
			"This cannot be undone. You will be prompted to confirm unless\n" +
			"--yes is passed.",
		Example: "  # Remove a member (will prompt for confirmation)\n" +
			"  pulumi org member remove alice\n\n" +
			"  # Remove without confirmation\n" +
			"  pulumi org member remove alice --yes",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			args.yes = args.yes || env.SkipConfirmations.Value()
			return runOrgMemberRemove(cmd.Context(), cmd.OutOrStdout(), factory, posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "user-login"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&args.org, "org", "", "The organization that owns the member")
	cmd.Flags().BoolVarP(&args.yes, "yes", "y", false, "Skip confirmation prompts")
	outputflag.VarP(cmd.Flags(), &args.outputFormat)

	return cmd
}

// defaultOrgMemberRemoveClientFactory is the production wiring: resolve the
// cloud backend, pick the effective organization, and hand back the
// underlying *client.Client.
func defaultOrgMemberRemoveClientFactory(
	ctx context.Context, orgFlag string,
) (orgMemberRemoveClient, string, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	be, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, nil, opts)
	if err != nil {
		return nil, "", err
	}
	cloudBackend, ok := be.(httpstate.Backend)
	if !ok {
		return nil, "", errors.New(
			"removing an organization member requires the Pulumi Cloud backend; run `pulumi login`")
	}

	userName, orgs, _, err := cloudBackend.CurrentUser()
	if err != nil {
		return nil, "", err
	}

	org := orgFlag
	if org == "" {
		defaultOrg, err := cloudBackend.GetDefaultOrg(ctx)
		if err != nil {
			return nil, "", err
		}
		org = defaultOrg
	}
	if org == "" {
		org = userName
	}

	if !slices.Contains(orgs, org) && org != userName {
		return nil, "", fmt.Errorf("user %s is not a member of organization %s", userName, org)
	}

	return cloudBackend.Client(), org, nil
}

// runOrgMemberRemove is the cobra-decoupled command body so tests can drive it
// directly without spinning up the flag parser.
func runOrgMemberRemove(
	ctx context.Context, w io.Writer,
	factory orgMemberRemoveClientFactory, userLogin string, args orgMemberRemoveArgs,
) error {
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}
	prompt := fmt.Sprintf(
		"This will permanently remove member '%s' from the organization!", userLogin)
	if err := ui.ConfirmDeletion(args.yes, cmdutil.Interactive(), prompt, userLogin, w, opts); err != nil {
		return err
	}

	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	if err := c.RemoveOrganizationMember(ctx, org, userLogin); err != nil {
		return fmt.Errorf("removing organization member: %w", err)
	}

	return args.outputFormat.Get()(w, org, userLogin)
}

type orgMemberRemoveRenderFunc func(w io.Writer, org, userLogin string) error

func renderOrgMemberRemoveText(w io.Writer, org, userLogin string) error {
	fmt.Fprintf(w, "Removed member %s from organization %s.\n", userLogin, org)
	return nil
}

// orgMemberRemoveJSON is the JSON envelope emitted by
// `pulumi org member remove --output=json`.
type orgMemberRemoveJSON struct {
	OrganizationName string `json:"organizationName"`
	UserLogin        string `json:"userLogin"`
}

func renderOrgMemberRemoveJSON(w io.Writer, org, userLogin string) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(orgMemberRemoveJSON{
		OrganizationName: org,
		UserLogin:        userLogin,
	})
}
