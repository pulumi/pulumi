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
// an explicit empty --new-name from "user did not pass --new-name", and lets
// tests drive the command without spinning up cobra.
type policyGroupEditArgs struct {
	org          string
	outputFormat outputflag.OutputFlag[policyGroupGetRenderFunc]

	newName               string
	addStack              []string
	removeStack           []string
	addPolicyPack         []string
	removePolicyPack      []string
	addInsightsAccount    []string
	removeInsightsAccount []string

	// changed records which of the mutation flags were set by the user.
	// Keys are the flag names: "new-name", "add-stack", "remove-stack",
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
		Hidden: true,
		Use:    "edit <name>",
		Short:  "[EXPERIMENTAL] Update a Policy Group's configuration",
		Long: "[EXPERIMENTAL] Update a Policy Group's configuration.\n" +
			"\n" +
			"Renames a Policy Group, adds or removes stacks, applies or detaches\n" +
			"Policy Packs, and adds or removes Insights accounts. At least one\n" +
			"mutation flag must be provided. Changes are applied in the order\n" +
			"new-name, adds, removes, and the command stops on the first error.\n" +
			"\n" +
			"Default output is a human-readable summary; pass --output=json for the\n" +
			"full response as JSON.",
		Example: "  # Rename a Policy Group\n" +
			"  pulumi policy group edit prod-policies --new-name production\n\n" +
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
	cmd.Flags().StringVar(&args.newName, "new-name", "", "Rename the Policy Group")
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
	"new-name",
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
func runPolicyGroupEdit(
	ctx context.Context, w io.Writer,
	factory policyGroupEditClientFactory, name string, args policyGroupEditArgs,
) error {
	patches, err := buildPolicyGroupEditPatches(args)
	if err != nil {
		return err
	}
	if len(patches) == 0 {
		return errors.New(
			"no changes specified; pass at least one of --new-name, --add-stack, --remove-stack, " +
				"--add-policy-pack, --remove-policy-pack, --add-insights-account, --remove-insights-account")
	}

	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	current := name
	for _, p := range patches {
		if err := c.UpdatePolicyGroup(ctx, org, current, p); err != nil {
			return err
		}
		if p.NewName != nil {
			current = *p.NewName
		}
	}

	resp, err := c.GetPolicyGroup(ctx, org, current)
	if err != nil {
		return fmt.Errorf("reading policy group after edit: %w", err)
	}

	return args.outputFormat.Get()(w, resp)
}

// buildPolicyGroupEditPatches expands the edit args into the ordered sequence
// of PATCHes to send: rename first, then adds, then removes. Each PATCH
// carries a single mutation, matching the service's UpdatePolicyGroup
// contract.
func buildPolicyGroupEditPatches(args policyGroupEditArgs) ([]apitype.UpdatePolicyGroupRequest, error) {
	var patches []apitype.UpdatePolicyGroupRequest

	if args.changed["new-name"] {
		name := args.newName
		patches = append(patches, apitype.UpdatePolicyGroupRequest{NewName: &name})
	}

	if args.changed["add-stack"] {
		for _, s := range args.addStack {
			ref := parseStackReference(s)
			patches = append(patches, apitype.UpdatePolicyGroupRequest{AddStack: &ref})
		}
	}
	if args.changed["add-policy-pack"] {
		for _, p := range args.addPolicyPack {
			meta, err := parsePolicyPackRef(p)
			if err != nil {
				return nil, err
			}
			patches = append(patches, apitype.UpdatePolicyGroupRequest{AddPolicyPack: &meta})
		}
	}
	if args.changed["add-insights-account"] {
		for _, a := range args.addInsightsAccount {
			acct := a
			patches = append(patches, apitype.UpdatePolicyGroupRequest{AddInsightsAccount: &acct})
		}
	}

	if args.changed["remove-stack"] {
		for _, s := range args.removeStack {
			ref := parseStackReference(s)
			patches = append(patches, apitype.UpdatePolicyGroupRequest{RemoveStack: &ref})
		}
	}
	if args.changed["remove-policy-pack"] {
		for _, p := range args.removePolicyPack {
			meta, err := parsePolicyPackRef(p)
			if err != nil {
				return nil, err
			}
			patches = append(patches, apitype.UpdatePolicyGroupRequest{RemovePolicyPack: &meta})
		}
	}
	if args.changed["remove-insights-account"] {
		for _, a := range args.removeInsightsAccount {
			acct := a
			patches = append(patches, apitype.UpdatePolicyGroupRequest{RemoveInsightsAccount: &acct})
		}
	}

	return patches, nil
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
