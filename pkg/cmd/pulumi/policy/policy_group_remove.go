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

package policy

// AI Generated - needs human review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

// policyGroupRemoveClient is the narrow subset of cloud-API operations the
// remove command needs.
type policyGroupRemoveClient interface {
	DeletePolicyGroup(ctx context.Context, orgName, policyGroup string) error
}

// policyGroupRemoveClientFactory resolves a cloud client and the organization
// the Policy Group lives in. orgFlag carries the raw value of `--org` (empty
// means "use the default org").
type policyGroupRemoveClientFactory func(
	ctx context.Context, orgFlag string,
) (policyGroupRemoveClient, string, error)

// policyGroupRemoveArgs collects the flag values for the remove command.
type policyGroupRemoveArgs struct {
	org          string
	yes          bool
	outputFormat outputflag.OutputFlag[policyGroupRemoveRenderFunc]
}

// defaultPolicyGroupRemoveOutputFormat wires the OutputFlag to the per-format
// renderers so `--output` selects between them.
func defaultPolicyGroupRemoveOutputFormat() outputflag.OutputFlag[policyGroupRemoveRenderFunc] {
	return outputflag.OutputFlag[policyGroupRemoveRenderFunc]{
		RenderForTerminal: renderPolicyGroupRemoveText,
		RenderJSON:        renderPolicyGroupRemoveJSON,
	}
}

// newPolicyGroupRemoveCmd builds `pulumi policy group remove` with the
// production client factory.
func newPolicyGroupRemoveCmd() *cobra.Command {
	return newPolicyGroupRemoveCmdWith(defaultPolicyGroupRemoveClientFactory)
}

func newPolicyGroupRemoveCmdWith(factory policyGroupRemoveClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "policyGroupRemoveClientFactory must not be nil")
	var args policyGroupRemoveArgs
	args.outputFormat = defaultPolicyGroupRemoveOutputFormat()

	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "[EXPERIMENTAL] Delete a Policy Group",
		Long: "[EXPERIMENTAL] Delete a Policy Group.\n" +
			"\n" +
			"Deletes a Policy Group from an organization. This cannot be undone.\n" +
			"You will be prompted to confirm unless --yes is passed.\n" +
			"\n" +
			"The organization's default Policy Group cannot be deleted. Deleting\n" +
			"a Policy Group removes all policy enforcement associations for the\n" +
			"stacks that were assigned to it.",
		Example: "  # Remove a Policy Group (will prompt for confirmation)\n" +
			"  pulumi policy group remove prod-policies\n\n" +
			"  # Remove without confirmation\n" +
			"  pulumi policy group remove prod-policies --yes\n\n" +
			"  # Remove from a specific organization\n" +
			"  pulumi policy group remove prod-policies --org acme --yes",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			return runPolicyGroupRemove(cmd.Context(), cmd.OutOrStdout(), factory, posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "name"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&args.org, "org", "", "The organization that owns the Policy Group")
	cmd.Flags().BoolVarP(&args.yes, "yes", "y", false, "Skip confirmation prompts")
	outputflag.VarP(cmd.Flags(), &args.outputFormat)

	return cmd
}

// defaultPolicyGroupRemoveClientFactory is the production wiring: resolve the
// cloud backend, pick the effective organization, and hand back the
// underlying *client.Client.
func defaultPolicyGroupRemoveClientFactory(
	ctx context.Context, orgFlag string,
) (policyGroupRemoveClient, string, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	be, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, nil, opts)
	if err != nil {
		return nil, "", err
	}
	cloudBackend, ok := be.(httpstate.Backend)
	if !ok {
		return nil, "", errors.New(
			"removing a Policy Group requires the Pulumi Cloud backend; run `pulumi login`")
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

// runPolicyGroupRemove is the cobra-decoupled command body so tests can drive
// it directly without spinning up the flag parser.
func runPolicyGroupRemove(
	ctx context.Context, w io.Writer,
	factory policyGroupRemoveClientFactory, name string, args policyGroupRemoveArgs,
) error {
	if !cmdutil.Interactive() && !args.yes {
		return backenderr.NonInteractiveRequiresYesError{}
	}

	if !args.yes {
		opts := display.Options{Color: cmdutil.GetGlobalColorization()}
		prompt := fmt.Sprintf("This will permanently remove the policy group '%s'!", name)
		if !ui.ConfirmPrompt(prompt, name, opts) {
			return result.FprintBailf(w, "confirmation declined")
		}
	}

	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	if err := c.DeletePolicyGroup(ctx, org, name); err != nil {
		return fmt.Errorf("removing policy group: %w", err)
	}

	return args.outputFormat.Get()(w, org, name)
}

type policyGroupRemoveRenderFunc func(w io.Writer, org, name string) error

func renderPolicyGroupRemoveText(w io.Writer, org, name string) error {
	fmt.Fprintf(w, "Removed policy group %s from organization %s.\n", name, org)
	return nil
}

// policyGroupRemoveJSON is the JSON envelope emitted by
// `pulumi policy group remove --output=json`.
type policyGroupRemoveJSON struct {
	OrganizationName string `json:"organizationName"`
	Name             string `json:"name"`
}

func renderPolicyGroupRemoveJSON(w io.Writer, org, name string) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(policyGroupRemoveJSON{
		OrganizationName: org,
		Name:             name,
	})
}
