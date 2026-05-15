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

import (
	"context"
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// policyGroupNewClient is the narrow subset of cloud-API operations the new
// command needs.
type policyGroupNewClient interface {
	CreatePolicyGroup(ctx context.Context, orgName string, req apitype.CreatePolicyGroupRequest) error
	GetPolicyGroup(
		ctx context.Context, orgName, policyGroup string,
	) (apitype.GetPolicyGroupResponse, error)
}

// policyGroupNewClientFactory resolves a cloud client and the organization the
// new Policy Group will be created in.
type policyGroupNewClientFactory func(
	ctx context.Context, orgFlag string,
) (policyGroupNewClient, string, error)

// policyGroupNewArgs collects the flag values for the new command.
type policyGroupNewArgs struct {
	org          string
	entityType   string
	mode         string
	agentPoolID  string
	yes          bool
	outputFormat outputflag.OutputFlag[policyGroupGetRenderFunc]
}

var (
	validEntityTypes = []string{"stacks", "accounts"}
	validModes       = []string{"audit", "preventative"}
)

// newPolicyGroupNewCmd builds `pulumi policy group new` with the production
// client factory.
func newPolicyGroupNewCmd() *cobra.Command {
	return newPolicyGroupNewCmdWith(defaultPolicyGroupNewClientFactory)
}

func newPolicyGroupNewCmdWith(factory policyGroupNewClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "policyGroupNewClientFactory must not be nil")
	var args policyGroupNewArgs
	args.outputFormat = defaultPolicyGroupGetOutputFormat()

	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "[EXPERIMENTAL] Create a new Policy Group",
		Long: "[EXPERIMENTAL] Create a new Policy Group.\n" +
			"\n" +
			"Creates a new Policy Group in the given organization. Policy Groups\n" +
			"define which Policy Packs are enforced on which stacks or cloud\n" +
			"accounts, with configurable enforcement levels per pack.\n" +
			"\n" +
			"When run interactively, prompts for required values that aren't\n" +
			"provided via flags. Pass --yes to accept defaults without prompting.",
		Example: "  # Create a Policy Group interactively\n" +
			"  pulumi policy group new prod-policies\n\n" +
			"  # Create a stack Policy Group non-interactively\n" +
			"  pulumi policy group new prod-policies --entity-type stacks --yes\n\n" +
			"  # Create an audit-mode account Policy Group\n" +
			"  pulumi policy group new compliance \\\n" +
			"    --entity-type accounts --mode audit --yes\n\n" +
			"  # Emit JSON\n" +
			"  pulumi policy group new prod-policies --entity-type stacks \\\n" +
			"    --yes --output json",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			return runPolicyGroupNew(cmd.Context(), cmd.OutOrStdout(), factory, posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "name"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&args.org, "org", "",
		"The organization to create the Policy Group in")
	cmd.Flags().StringVar(&args.entityType, "entity-type", "",
		"The type of entities: stacks or accounts")
	cmd.Flags().StringVar(&args.mode, "mode", "",
		"The enforcement mode: audit or preventative")
	cmd.Flags().StringVar(&args.agentPoolID, "agent-pool-id", "",
		"Agent pool ID for policy evaluation (optional)")
	cmd.Flags().BoolVarP(&args.yes, "yes", "y", false,
		"Skip prompts and proceed with default values")
	outputflag.VarP(cmd.Flags(), &args.outputFormat)

	return cmd
}

func defaultPolicyGroupNewClientFactory(
	ctx context.Context, orgFlag string,
) (policyGroupNewClient, string, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	be, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, nil, opts)
	if err != nil {
		return nil, "", err
	}
	cloudBackend, ok := be.(httpstate.Backend)
	if !ok {
		return nil, "", errors.New(
			"creating a Policy Group requires the Pulumi Cloud backend; run `pulumi login`")
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

func runPolicyGroupNew(
	ctx context.Context, w io.Writer,
	factory policyGroupNewClientFactory, name string, args policyGroupNewArgs,
) error {
	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	skipPrompts := args.yes || !cmdutil.Interactive()
	displayOpts := display.Options{Color: cmdutil.GetGlobalColorization()}

	// Entity type is required.
	entityType := args.entityType
	if entityType == "" {
		if skipPrompts {
			return errors.New("--entity-type is required (use --entity-type stacks or --entity-type accounts)")
		}
		entityType = ui.PromptUser(
			"Entity type",
			validEntityTypes, "stacks",
			displayOpts.Color)
	}
	if !slices.Contains(validEntityTypes, entityType) {
		return fmt.Errorf("invalid --entity-type %q; must be \"stacks\" or \"accounts\"", entityType)
	}

	// Mode: prompt if interactive, otherwise use server default.
	mode := args.mode
	if mode == "" && !skipPrompts {
		defaultMode := "preventative"
		if entityType == "accounts" {
			defaultMode = "audit"
		}
		mode = ui.PromptUserSkippable(
			false, "Enforcement mode",
			validModes, defaultMode,
			displayOpts.Color)
	}
	if mode != "" && !slices.Contains(validModes, mode) {
		return fmt.Errorf("invalid --mode %q; must be \"audit\" or \"preventative\"", mode)
	}

	req := apitype.CreatePolicyGroupRequest{
		Name:        name,
		EntityType:  entityType,
		Mode:        mode,
		AgentPoolID: args.agentPoolID,
	}

	if err := c.CreatePolicyGroup(ctx, org, req); err != nil {
		return fmt.Errorf("creating policy group: %w", err)
	}

	resp, err := c.GetPolicyGroup(ctx, org, name)
	if err != nil {
		return fmt.Errorf("reading policy group after create: %w", err)
	}

	return args.outputFormat.Get()(w, resp)
}
