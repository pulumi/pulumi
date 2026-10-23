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
	"strconv"
	"strings"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// policyGroupEditClient is the narrow subset of cloud-API operations the edit
// command needs.
type policyGroupEditClient interface {
	BatchUpdatePolicyGroup(
		ctx context.Context, orgName, policyGroup string, reqs []cloudapitype.UpdatePolicyGroupRequest,
	) error
	GetPolicyGroup(
		ctx context.Context, orgName, policyGroup string,
	) (cloudapitype.PolicyGroup, error)
}

// policyGroupEditClientFactory resolves a cloud client and the organization
// the Policy Group lives in.
type policyGroupEditClientFactory func(
	ctx context.Context, orgFlag string,
) (policyGroupEditClient, string, error)

// policyGroupEditArgs collects the flag values for the edit command. Only the
// flags listed in changed are applied; this lets the run function distinguish
// an explicit empty --name from "user did not pass --name", and lets
// tests drive the command without spinning up cobra.
type policyGroupEditArgs struct {
	org          string
	outputFormat outputflag.OutputFlag[policyGroupGetRenderFunc]

	rename                string
	addStack              []string
	removeStack           []string
	addPolicyPack         []string
	removePolicyPack      []string
	addInsightsAccount    []string
	removeInsightsAccount []string
	stackRule             string
	removeStackRule       bool
	accountRule           string
	removeAccountRule     bool

	// changed records which of the mutation flags were set by the user.
	// Keys are the names in mutationFlagNames.
	changed map[string]bool
}

// newPolicyGroupEditCmd builds `pulumi policy group edit` with the production
// client factory.
func newPolicyGroupEditCmd() *cobra.Command {
	return newPolicyGroupEditCmdWith(defaultPolicyGroupEditClientFactory)
}

func newPolicyGroupEditCmdWith(factory policyGroupEditClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "policyGroupEditClientFactory must not be nil")
	var args policyGroupEditArgs
	args.outputFormat = defaultPolicyGroupGetOutputFormat()

	cmd := &cobra.Command{
		Use:     "edit <name>",
		Aliases: []string{"update", "modify"},
		Short:   "[EXPERIMENTAL] Update a Policy Group's configuration",
		Long: "[EXPERIMENTAL] Update a Policy Group's configuration.\n" +
			"\n" +
			"Renames a Policy Group, adds or removes stacks, applies or detaches\n" +
			"Policy Packs, adds or removes Insights accounts, and sets or removes\n" +
			"match rules. At least one mutation flag must be provided.\n" +
			"\n" +
			"Default output is a human-readable summary; pass --output=json for the\n" +
			"full response as JSON.\n" +
			"\n" +
			matchRuleHelp,
		Example: "  # Rename a Policy Group\n" +
			"  pulumi policy group edit prod-policies --name production\n\n" +
			"  # Add a stack and a Policy Pack to a group\n" +
			"  pulumi policy group edit prod-policies " +
			"--add-stack web/prod --add-policy-pack aws-guardrails@3\n\n" +
			"  # Replace the stack rule: stacks tagged env=prod, or any stack with a team tag\n" +
			"  pulumi policy group edit prod-policies --stack-rule 'env=prod OR team'\n\n" +
			"  # Set an account rule from a JSON file on an audit group\n" +
			"  pulumi policy group edit compliance --account-rule @account-rule.json\n\n" +
			"  # Remove the stack rule\n" +
			"  pulumi policy group edit prod-policies --remove-stack-rule",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			args.changed = changedFlagSet(cmd, mutationFlagNames)
			return runPolicyGroupEdit(cmd.Context(), cmd.OutOrStdout(), factory, posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "name"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&args.org, "org", "", "The organization that owns the Policy Group")
	outputflag.VarP(cmd.Flags(), &args.outputFormat)
	cmd.Flags().StringVar(&args.rename, "name", "", "Rename the Policy Group")
	cmd.Flags().StringArrayVar(&args.addStack, "add-stack", nil,
		"Add a stack to the Policy Group (repeatable). Format: 'project/stack' or 'stack'")
	cmd.Flags().StringArrayVar(&args.removeStack, "remove-stack", nil,
		"Remove a stack from the Policy Group (repeatable). Format: 'project/stack' or 'stack'")
	cmd.Flags().StringArrayVar(&args.addPolicyPack, "add-policy-pack", nil,
		"Add a Policy Pack to the Policy Group (repeatable). Format: 'name@version' or 'name'")
	cmd.Flags().StringArrayVar(&args.removePolicyPack, "remove-policy-pack", nil,
		"Remove a Policy Pack from the Policy Group (repeatable). Format: 'name@version' or 'name'")
	cmd.Flags().StringArrayVar(&args.addInsightsAccount, "add-insights-account", nil,
		"Add an Insights account to the Policy Group (repeatable)")
	cmd.Flags().StringArrayVar(&args.removeInsightsAccount, "remove-insights-account", nil,
		"Remove an Insights account from the Policy Group (repeatable)")
	cmd.Flags().StringVar(&args.stackRule, "stack-rule", "",
		"Replace the stack match rule: tag conditions such as 'env=prod,team!=web OR owner', "+
			"a JSON expression, or @<file>")
	cmd.Flags().BoolVar(&args.removeStackRule, "remove-stack-rule", false,
		"Remove the stack match rule")
	cmd.Flags().StringVar(&args.accountRule, "account-rule", "",
		"Replace the Insights account match rule, in the same syntax as --stack-rule. Audit groups only")
	cmd.Flags().BoolVar(&args.removeAccountRule, "remove-account-rule", false,
		"Remove the Insights account match rule")

	return cmd
}

// mutationFlagNames are the flags whose presence triggers a PATCH. --org and
// --output are excluded because they are context, not mutations.
var mutationFlagNames = []string{
	"name",
	"add-stack", "remove-stack",
	"add-policy-pack", "remove-policy-pack",
	"add-insights-account", "remove-insights-account",
	"stack-rule", "remove-stack-rule",
	"account-rule", "remove-account-rule",
}

var errNoPolicyGroupChanges = errors.New(
	"no changes specified; pass at least one of --name, --add-stack, --remove-stack, " +
		"--add-policy-pack, --remove-policy-pack, --add-insights-account, --remove-insights-account, " +
		"--stack-rule, --remove-stack-rule, --account-rule, --remove-account-rule")

// changedFlagSet snapshots which of the named flags were set on the command
// line. Cobra resets `.Changed` after parsing, so we capture it inside RunE.
func changedFlagSet(cmd *cobra.Command, names []string) map[string]bool {
	out := make(map[string]bool, len(names))
	for _, n := range names {
		f := cmd.Flag(n)
		out[n] = f != nil && f.Changed
	}
	return out
}

// defaultPolicyGroupEditClientFactory is the production wiring: resolve the
// cloud backend, pick the effective organization, and hand back the underlying
// *client.Client.
func defaultPolicyGroupEditClientFactory(
	ctx context.Context, orgFlag string,
) (policyGroupEditClient, string, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	be, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, nil, opts)
	if err != nil {
		return nil, "", err
	}
	cloudBackend, ok := be.(httpstate.Backend)
	if !ok {
		return nil, "", errors.New(
			"editing a Policy Group requires the Pulumi Cloud backend; run `pulumi login`")
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

// runPolicyGroupEdit is the cobra-decoupled command body so tests can drive it
// directly without spinning up the flag parser.
func runPolicyGroupEdit(
	ctx context.Context, w io.Writer,
	factory policyGroupEditClientFactory, name string, args policyGroupEditArgs,
) error {
	if !anyMutationRequested(args) {
		return errNoPolicyGroupChanges
	}
	if args.changed["stack-rule"] && args.changed["remove-stack-rule"] {
		return cmdCmd.ConfigurationError{Message: "--stack-rule and --remove-stack-rule cannot be used together"}
	}
	if args.changed["account-rule"] && args.changed["remove-account-rule"] {
		return cmdCmd.ConfigurationError{Message: "--account-rule and --remove-account-rule cannot be used together"}
	}

	addStacks, removeStacks, err := parseStackReferences(args.addStack, args.removeStack)
	if err != nil {
		return err
	}
	addPacks, removePacks, err := parsePolicyPackRefs(args.addPolicyPack, args.removePolicyPack)
	if err != nil {
		return err
	}
	stackRule, err := parseMatchRuleFlag(stackMatchRule, args.stackRule, args.changed)
	if err != nil {
		return err
	}
	accountRule, err := parseMatchRuleFlag(accountMatchRule, args.accountRule, args.changed)
	if err != nil {
		return err
	}

	updates := policyGroupUpdates(args, addStacks, removeStacks, addPacks, removePacks)
	updates = append(updates, matchRuleUpdates(args, stackRule, accountRule)...)
	if len(updates) == 0 {
		return errNoPolicyGroupChanges
	}

	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	if accountRule != nil {
		group, err := c.GetPolicyGroup(ctx, org, name)
		if err != nil {
			return fmt.Errorf("reading policy group: %w", err)
		}
		if group.Mode != cloudapitype.PolicyGroupModeAudit {
			return cmdCmd.ConfigurationError{Message: fmt.Sprintf(
				"--account-rule requires an audit policy group; %q is %s", name, group.Mode)}
		}
	}

	if err := c.BatchUpdatePolicyGroup(ctx, org, name, updates); err != nil {
		return err
	}

	finalName := name
	if args.changed["name"] {
		finalName = args.rename
	}
	resp, err := c.GetPolicyGroup(ctx, org, finalName)
	if err != nil {
		return fmt.Errorf("reading policy group after edit: %w", err)
	}

	return args.outputFormat.Get()(w, resp)
}

// policyGroupUpdates converts the requested edits into one update per
// mutation, because the service reads only the singular fields of an update.
func policyGroupUpdates(
	args policyGroupEditArgs,
	addStacks, removeStacks []cloudapitype.AppPulumiStackReference,
	addPacks, removePacks []cloudapitype.AppPolicyPackMetadata,
) []cloudapitype.UpdatePolicyGroupRequest {
	var updates []cloudapitype.UpdatePolicyGroupRequest

	if args.changed["name"] {
		updates = append(updates, policyGroupUpdate(cloudapitype.AppUpdatePolicyGroupRequest{NewName: &args.rename}))
	}
	for _, s := range addStacks {
		updates = append(updates, policyGroupUpdate(cloudapitype.AppUpdatePolicyGroupRequest{AddStack: &s}))
	}
	for _, s := range removeStacks {
		updates = append(updates, policyGroupUpdate(cloudapitype.AppUpdatePolicyGroupRequest{RemoveStack: &s}))
	}
	for _, a := range args.addInsightsAccount {
		updates = append(updates, policyGroupUpdate(cloudapitype.AppUpdatePolicyGroupRequest{
			AddInsightsAccount: &cloudapitype.AppInsightsAccountReference{Name: a},
		}))
	}
	for _, a := range args.removeInsightsAccount {
		updates = append(updates, policyGroupUpdate(cloudapitype.AppUpdatePolicyGroupRequest{
			RemoveInsightsAccount: &cloudapitype.AppInsightsAccountReference{Name: a},
		}))
	}
	for _, p := range addPacks {
		updates = append(updates, policyGroupUpdate(cloudapitype.AppUpdatePolicyGroupRequest{AddPolicyPack: &p}))
	}
	for _, p := range removePacks {
		updates = append(updates, policyGroupUpdate(cloudapitype.AppUpdatePolicyGroupRequest{RemovePolicyPack: &p}))
	}
	return updates
}

// matchRuleUpdates converts the rule flags into one update per rule change.
func matchRuleUpdates(
	args policyGroupEditArgs, stackRule, accountRule cloudapitype.PermissionBooleanExpression,
) []cloudapitype.UpdatePolicyGroupRequest {
	var updates []cloudapitype.UpdatePolicyGroupRequest
	if stackRule != nil {
		updates = append(updates, cloudapitype.UpdatePolicyGroupRequest{StackMatchRule: stackRule})
	}
	if args.changed["remove-stack-rule"] && args.removeStackRule {
		updates = append(updates, cloudapitype.UpdatePolicyGroupRequest{RemoveStackMatchRule: true})
	}
	if accountRule != nil {
		updates = append(updates, cloudapitype.UpdatePolicyGroupRequest{AccountMatchRule: accountRule})
	}
	if args.changed["remove-account-rule"] && args.removeAccountRule {
		updates = append(updates, cloudapitype.UpdatePolicyGroupRequest{RemoveAccountMatchRule: true})
	}
	return updates
}

func policyGroupUpdate(u cloudapitype.AppUpdatePolicyGroupRequest) cloudapitype.UpdatePolicyGroupRequest {
	return cloudapitype.UpdatePolicyGroupRequest{AppUpdatePolicyGroupRequest: u}
}

// anyMutationRequested returns true when at least one of the mutation flags
// was set by the user.
func anyMutationRequested(args policyGroupEditArgs) bool {
	for _, name := range mutationFlagNames {
		if args.changed[name] {
			return true
		}
	}
	return false
}

// parseStackReferences pre-parses every add and remove stack value so the
// command surfaces parse errors before any network call.
func parseStackReferences(
	adds, removes []string,
) ([]cloudapitype.AppPulumiStackReference, []cloudapitype.AppPulumiStackReference, error) {
	parsedAdds := make([]cloudapitype.AppPulumiStackReference, 0, len(adds))
	for _, s := range adds {
		parsedAdds = append(parsedAdds, parseStackReference(s))
	}
	parsedRemoves := make([]cloudapitype.AppPulumiStackReference, 0, len(removes))
	for _, s := range removes {
		parsedRemoves = append(parsedRemoves, parseStackReference(s))
	}
	return parsedAdds, parsedRemoves, nil
}

// parsePolicyPackRefs pre-parses every add and remove policy-pack value so
// invalid references surface before any network call.
func parsePolicyPackRefs(
	adds, removes []string,
) ([]cloudapitype.AppPolicyPackMetadata, []cloudapitype.AppPolicyPackMetadata, error) {
	parsedAdds := make([]cloudapitype.AppPolicyPackMetadata, 0, len(adds))
	for _, p := range adds {
		meta, err := parsePolicyPackRef(p)
		if err != nil {
			return nil, nil, err
		}
		parsedAdds = append(parsedAdds, meta)
	}
	parsedRemoves := make([]cloudapitype.AppPolicyPackMetadata, 0, len(removes))
	for _, p := range removes {
		meta, err := parsePolicyPackRef(p)
		if err != nil {
			return nil, nil, err
		}
		parsedRemoves = append(parsedRemoves, meta)
	}
	return parsedAdds, parsedRemoves, nil
}

// parseStackReference splits "project/stack" into an AppPulumiStackReference. A
// bare "stack" leaves RoutingProject empty.
func parseStackReference(s string) cloudapitype.AppPulumiStackReference {
	if before, after, ok := strings.Cut(s, "/"); ok {
		return cloudapitype.AppPulumiStackReference{
			RoutingProject: before,
			Name:           after,
		}
	}
	return cloudapitype.AppPulumiStackReference{Name: s}
}

// parsePolicyPackRef parses "name@version" or "name". When a version is
// present and parses as an integer it populates Version; otherwise it goes to
// VersionTag.
func parsePolicyPackRef(s string) (cloudapitype.AppPolicyPackMetadata, error) {
	if s == "" {
		return cloudapitype.AppPolicyPackMetadata{}, errors.New("policy pack reference must not be empty")
	}
	before, after, ok := strings.Cut(s, "@")
	if !ok {
		return cloudapitype.AppPolicyPackMetadata{Name: s}, nil
	}
	name, ver := before, after
	if name == "" {
		return cloudapitype.AppPolicyPackMetadata{}, fmt.Errorf("policy pack reference %q is missing a name", s)
	}
	meta := cloudapitype.AppPolicyPackMetadata{Name: name}
	if n, err := strconv.Atoi(ver); err == nil {
		meta.Version = n
	} else {
		meta.VersionTag = ver
	}
	return meta, nil
}
