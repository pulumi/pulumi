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
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// policyGroupEditClient is the narrow subset of cloud-API operations the edit
// command needs. UpdatePolicyGroup carries a single mutation per call; the
// command issues one call per add/remove and one final GetPolicyGroup to
// render the result.
type policyGroupEditClient interface {
	UpdatePolicyGroup(
		ctx context.Context, orgName, policyGroup string, req apitype.UpdatePolicyGroupRequest,
	) error
	GetPolicyGroup(
		ctx context.Context, orgName, policyGroup string,
	) (apitype.GetPolicyGroupResponse, error)
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

	// changed records which of the mutation flags were set by the user.
	// Keys are the flag names: "name", "add-stack", "remove-stack",
	// "add-policy-pack", "remove-policy-pack", "add-insights-account",
	// "remove-insights-account".
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
		Use:   "edit <name>",
		Short: "[EXPERIMENTAL] Update a Policy Group's configuration",
		Long: "[EXPERIMENTAL] Update a Policy Group's configuration.\n" +
			"\n" +
			"Renames a Policy Group, adds or removes stacks, applies or detaches\n" +
			"Policy Packs, and adds or removes Insights accounts. At least one\n" +
			"mutation flag must be provided.\n" +
			"\n" +
			"Default output is a human-readable summary; pass --output=json for the\n" +
			"full response as JSON.",
		Example: "  # Rename a Policy Group\n" +
			"  pulumi policy group edit prod-policies --name production\n\n" +
			"  # Add a stack and a Policy Pack to a group\n" +
			"  pulumi policy group edit prod-policies " +
			"--add-stack web/prod --add-policy-pack aws-guardrails@3",
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

	return cmd
}

// mutationFlagNames are the flags whose presence triggers a PATCH. --org and
// --output are excluded because they are context, not mutations.
var mutationFlagNames = []string{
	"name",
	"add-stack", "remove-stack",
	"add-policy-pack", "remove-policy-pack",
	"add-insights-account", "remove-insights-account",
}

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
//
// The command issues a single batched PATCH. For each list field that the
// user mutated (stacks, policy packs, insights accounts) we GET the current
// group, apply the requested adds and removes, and send the resulting full
// list in the PATCH body — the service interprets a list value in a PATCH
// as a complete replacement of the prior list, so we cannot send isolated
// "add this one" deltas for those fields without losing the others.
func runPolicyGroupEdit(
	ctx context.Context, w io.Writer,
	factory policyGroupEditClientFactory, name string, args policyGroupEditArgs,
) error {
	if !anyMutationRequested(args) {
		return errors.New(
			"no changes specified; pass at least one of --name, --add-stack, --remove-stack, " +
				"--add-policy-pack, --remove-policy-pack, --add-insights-account, --remove-insights-account")
	}

	addStacks, removeStacks, err := parseStackReferences(args.addStack, args.removeStack)
	if err != nil {
		return err
	}
	addPacks, removePacks, err := parsePolicyPackRefs(args.addPolicyPack, args.removePolicyPack)
	if err != nil {
		return err
	}

	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	mutatesLists := args.changed["add-stack"] || args.changed["remove-stack"] ||
		args.changed["add-policy-pack"] || args.changed["remove-policy-pack"] ||
		args.changed["add-insights-account"] || args.changed["remove-insights-account"]

	patch := apitype.UpdatePolicyGroupRequest{}
	if args.changed["name"] {
		nn := args.rename
		patch.NewName = &nn
	}

	if mutatesLists {
		// Read the current group so we can compute the post-edit lists.
		current, err := c.GetPolicyGroup(ctx, org, name)
		if err != nil {
			return fmt.Errorf("reading policy group before edit: %w", err)
		}
		if args.changed["add-stack"] || args.changed["remove-stack"] {
			next := mergeStackList(current.Stacks, addStacks, removeStacks)
			patch.Stacks = &next
		}
		if args.changed["add-policy-pack"] || args.changed["remove-policy-pack"] {
			next := mergePolicyPackList(current.AppliedPolicyPacks, addPacks, removePacks)
			patch.PolicyPacks = &next
		}
		if args.changed["add-insights-account"] || args.changed["remove-insights-account"] {
			next := mergeStringList(current.Accounts, args.addInsightsAccount, args.removeInsightsAccount)
			patch.InsightsAccounts = &next
		}
	}

	if err := c.UpdatePolicyGroup(ctx, org, name, patch); err != nil {
		return err
	}

	finalName := name
	if patch.NewName != nil {
		finalName = *patch.NewName
	}
	resp, err := c.GetPolicyGroup(ctx, org, finalName)
	if err != nil {
		return fmt.Errorf("reading policy group after edit: %w", err)
	}

	return args.outputFormat.Get()(w, resp)
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
) ([]apitype.PulumiStackReference, []apitype.PulumiStackReference, error) {
	parsedAdds := make([]apitype.PulumiStackReference, 0, len(adds))
	for _, s := range adds {
		parsedAdds = append(parsedAdds, parseStackReference(s))
	}
	parsedRemoves := make([]apitype.PulumiStackReference, 0, len(removes))
	for _, s := range removes {
		parsedRemoves = append(parsedRemoves, parseStackReference(s))
	}
	return parsedAdds, parsedRemoves, nil
}

// parsePolicyPackRefs pre-parses every add and remove policy-pack value so
// invalid references surface before any network call.
func parsePolicyPackRefs(adds, removes []string) ([]apitype.PolicyPackMetadata, []apitype.PolicyPackMetadata, error) {
	parsedAdds := make([]apitype.PolicyPackMetadata, 0, len(adds))
	for _, p := range adds {
		meta, err := parsePolicyPackRef(p)
		if err != nil {
			return nil, nil, err
		}
		parsedAdds = append(parsedAdds, meta)
	}
	parsedRemoves := make([]apitype.PolicyPackMetadata, 0, len(removes))
	for _, p := range removes {
		meta, err := parsePolicyPackRef(p)
		if err != nil {
			return nil, nil, err
		}
		parsedRemoves = append(parsedRemoves, meta)
	}
	return parsedAdds, parsedRemoves, nil
}

// mergeStackList returns the current stacks plus adds, minus any that match a
// removal. Equality is by RoutingProject + Name.
func mergeStackList(
	current []apitype.PulumiStackReference,
	adds, removes []apitype.PulumiStackReference,
) []apitype.PulumiStackReference {
	stackEq := func(a, b apitype.PulumiStackReference) bool {
		return a.Name == b.Name && a.RoutingProject == b.RoutingProject
	}
	out := slices.Clone(current)
	for _, r := range removes {
		out = slices.DeleteFunc(out, func(s apitype.PulumiStackReference) bool { return stackEq(s, r) })
	}
	for _, a := range adds {
		if !slices.ContainsFunc(out, func(s apitype.PulumiStackReference) bool { return stackEq(s, a) }) {
			out = append(out, a)
		}
	}
	return out
}

// mergePolicyPackList returns the current applied packs plus adds, minus any
// that match a removal. Equality is by Name and version (Version + VersionTag).
func mergePolicyPackList(
	current []apitype.PolicyPackMetadata,
	adds, removes []apitype.PolicyPackMetadata,
) []apitype.PolicyPackMetadata {
	packEq := func(a, b apitype.PolicyPackMetadata) bool {
		return a.Name == b.Name && a.Version == b.Version && a.VersionTag == b.VersionTag
	}
	out := slices.Clone(current)
	for _, r := range removes {
		out = slices.DeleteFunc(out, func(p apitype.PolicyPackMetadata) bool { return packEq(p, r) })
	}
	for _, a := range adds {
		if !slices.ContainsFunc(out, func(p apitype.PolicyPackMetadata) bool { return packEq(p, a) }) {
			out = append(out, a)
		}
	}
	return out
}

// mergeStringList returns current plus adds, minus any in removes.
func mergeStringList(current, adds, removes []string) []string {
	out := slices.Clone(current)
	for _, r := range removes {
		out = slices.DeleteFunc(out, func(s string) bool { return s == r })
	}
	for _, a := range adds {
		if !slices.Contains(out, a) {
			out = append(out, a)
		}
	}
	return out
}

// parseStackReference splits "project/stack" into a PulumiStackReference. A
// bare "stack" leaves RoutingProject empty.
func parseStackReference(s string) apitype.PulumiStackReference {
	if i := strings.Index(s, "/"); i >= 0 {
		return apitype.PulumiStackReference{
			RoutingProject: s[:i],
			Name:           s[i+1:],
		}
	}
	return apitype.PulumiStackReference{Name: s}
}

// parsePolicyPackRef parses "name@version" or "name". When a version is
// present and parses as an integer it populates Version; otherwise it goes to
// VersionTag.
func parsePolicyPackRef(s string) (apitype.PolicyPackMetadata, error) {
	if s == "" {
		return apitype.PolicyPackMetadata{}, errors.New("policy pack reference must not be empty")
	}
	at := strings.Index(s, "@")
	if at < 0 {
		return apitype.PolicyPackMetadata{Name: s}, nil
	}
	name, ver := s[:at], s[at+1:]
	if name == "" {
		return apitype.PolicyPackMetadata{}, fmt.Errorf("policy pack reference %q is missing a name", s)
	}
	meta := apitype.PolicyPackMetadata{Name: name}
	if n, err := strconv.Atoi(ver); err == nil {
		meta.Version = n
	} else {
		meta.VersionTag = ver
	}
	return meta, nil
}
