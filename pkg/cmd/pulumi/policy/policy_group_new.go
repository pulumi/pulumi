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

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/pulumi/pulumi-cloud-sdk/go/apitype/jsonvalue"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// policyGroupNewClient is the narrow subset of cloud-API operations the new
// command needs.
type policyGroupNewClient interface {
	CreatePolicyGroup(ctx context.Context, orgName string, req cloudapitype.NewPolicyGroupRequest) error
	GetPolicyGroup(
		ctx context.Context, orgName, policyGroup string,
	) (cloudapitype.PolicyGroup, error)
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
	stackRule    string
	accountRule  string
	yes          bool
	outputFormat outputflag.OutputFlag[policyGroupGetRenderFunc]

	// changed records whether --stack-rule and --account-rule were set.
	changed map[string]bool
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
		Use:     "new <name>",
		Aliases: []string{"create", "setup"},
		Short:   "[EXPERIMENTAL] Create a new Policy Group",
		Long: "[EXPERIMENTAL] Create a new Policy Group.\n" +
			"\n" +
			"Creates a new Policy Group in the given organization. Policy Groups\n" +
			"define which Policy Packs are enforced on which stacks or cloud\n" +
			"accounts, with configurable enforcement levels per pack.\n" +
			"\n" +
			"When run interactively, prompts for required values that aren't\n" +
			"provided via flags. Pass --yes to skip prompts; --mode is then required.\n" +
			"\n" +
			matchRuleHelp,
		Example: "  # Create a Policy Group interactively\n" +
			"  pulumi policy group new prod-policies\n\n" +
			"  # Create a preventative Policy Group non-interactively\n" +
			"  pulumi policy group new prod-policies --mode preventative --yes\n\n" +
			"  # Create an audit Policy Group\n" +
			"  pulumi policy group new compliance --mode audit --yes\n\n" +
			"  # Create a preventative group that holds every stack tagged env=prod\n" +
			"  pulumi policy group new prod-policies --mode preventative \\\n" +
			"    --stack-rule env=prod --yes\n\n" +
			"  # Create an audit group for prod accounts not owned by the sandbox team\n" +
			"  pulumi policy group new prod-accounts --mode audit \\\n" +
			"    --account-rule 'env=prod,team!=sandbox' --yes\n\n" +
			"  # Create a group whose stack rule is read from a JSON file\n" +
			"  pulumi policy group new tagged --mode audit --stack-rule @rule.json --yes\n\n" +
			"  # Emit JSON\n" +
			"  pulumi policy group new prod-policies --mode preventative \\\n" +
			"    --yes --output json",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			args.changed = changedFlagSet(cmd, []string{"stack-rule", "account-rule"})
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
	_ = cmd.Flags().MarkDeprecated("entity-type", "use --mode instead")
	cmd.Flags().StringVar(&args.mode, "mode", "",
		"The enforcement mode: audit or preventative")
	cmd.Flags().StringVar(&args.agentPoolID, "agent-pool-id", "",
		"Agent pool ID for policy evaluation (optional)")
	cmd.Flags().StringVar(&args.stackRule, "stack-rule", "",
		"Stack match rule: tag conditions such as 'env=prod,team!=web OR owner', a JSON expression, "+
			"or @<file>")
	cmd.Flags().StringVar(&args.accountRule, "account-rule", "",
		"Insights account match rule, in the same syntax as --stack-rule. Audit groups only")
	cmd.Flags().BoolVarP(&args.yes, "yes", "y", false,
		"Skip prompts; --mode is then required")
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
	stackRule, err := parseMatchRuleFlag(stackMatchRule, args.stackRule, args.changed)
	if err != nil {
		return err
	}
	accountRule, err := parseMatchRuleFlag(accountMatchRule, args.accountRule, args.changed)
	if err != nil {
		return err
	}

	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	skipPrompts := args.yes || !cmdutil.Interactive()
	displayOpts := display.Options{Color: cmdutil.GetGlobalColorization()}

	entityType := args.entityType
	if entityType != "" && !slices.Contains(validEntityTypes, entityType) {
		return fmt.Errorf("invalid --entity-type %q; must be \"stacks\" or \"accounts\"", entityType)
	}

	mode := args.mode
	if mode == "" && !skipPrompts {
		defaultMode := "preventative"
		if entityType == "accounts" || accountRule != nil {
			defaultMode = "audit"
		}
		mode = ui.PromptUserSkippable(
			false, "Enforcement mode",
			validModes, defaultMode,
			displayOpts.Color)
	}
	// Without --mode, a deprecated --entity-type still lets the service pick the mode.
	if mode == "" && entityType == "" {
		return errors.New("--mode is required (use --mode preventative or --mode audit)")
	}
	if mode != "" && !slices.Contains(validModes, mode) {
		return fmt.Errorf("invalid --mode %q; must be \"audit\" or \"preventative\"", mode)
	}

	if accountRule != nil && !createsAuditGroup(mode, entityType) {
		return cmdCmd.ConfigurationError{Message: "--account-rule requires --mode audit"}
	}

	req := cloudapitype.NewPolicyGroupRequest{
		Name:             name,
		StackMatchRule:   stackRule,
		AccountMatchRule: accountRule,
	}
	if entityType != "" {
		req.EntityType = new(cloudapitype.PolicyGroupEntityType(entityType)) //nolint:staticcheck // deprecated flag
	}
	if mode != "" {
		req.Mode = new(cloudapitype.PolicyGroupMode(mode))
	}
	if args.agentPoolID != "" {
		req.AgentPoolID = jsonvalue.NotNull(args.agentPoolID)
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

// createsAuditGroup reports whether the service will create an audit group,
// which, without --mode, the deprecated --entity-type decides.
func createsAuditGroup(mode, entityType string) bool {
	if mode != "" {
		return mode == string(cloudapitype.PolicyGroupModeAudit)
	}
	return entityType == "accounts"
}
