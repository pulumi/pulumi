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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// policyGroupGetClient is the narrow subset of cloud-API operations the get
// command needs. Defined here so tests can stub a thin interface instead of
// the full HTTP client surface.
type policyGroupGetClient interface {
	GetPolicyGroup(
		ctx context.Context, orgName, policyGroup string,
	) (apitype.GetPolicyGroupResponse, error)
	ListPolicyGroupStacks(
		ctx context.Context, orgName, policyGroup string, opts client.ListPolicyGroupMembersOptions,
	) (cloudapitype.ListPolicyGroupStacksResponse, error)
	ListPolicyGroupAccounts(
		ctx context.Context, orgName, policyGroup string, opts client.ListPolicyGroupMembersOptions,
	) (cloudapitype.ListPolicyGroupAccountsResponse, error)
}

// policyGroupGetClientFactory resolves a cloud client and the organization
// the Policy Group lives in. orgFlag carries the raw value of `--org` (empty
// means "use the default org").
type policyGroupGetClientFactory func(
	ctx context.Context, orgFlag string,
) (policyGroupGetClient, string, error)

// policyGroupGetArgs collects the flag values for the get command, in one
// struct so Run can be driven directly from tests.
type policyGroupGetArgs struct {
	org          string
	stacks       bool
	accounts     bool
	showConfig   bool
	source       string
	count        int
	all          bool
	outputFormat outputflag.OutputFlag[policyGroupGetRender]
}

// defaultPolicyGroupGetOutputFormat wires the OutputFlag to the per-format
// renderers so `--output` selects between them.
func defaultPolicyGroupGetOutputFormat() outputflag.OutputFlag[policyGroupGetRenderFunc] {
	return outputflag.OutputFlag[policyGroupGetRenderFunc]{
		RenderForTerminal: renderPolicyGroupGetText,
		RenderJSON:        renderPolicyGroupGetJSON,
	}
}

// policyGroupGetRender holds one renderer per thing `policy group get` can
// print, so a single --output flag selects the format for all of them.
type policyGroupGetRender struct {
	group           policyGroupGetRenderFunc
	groupWithConfig policyGroupGetRenderFunc
	stacks          func(w io.Writer, resp cloudapitype.ListPolicyGroupStacksResponse) error
	accounts        func(w io.Writer, resp cloudapitype.ListPolicyGroupAccountsResponse) error
}

func defaultPolicyGroupGetRenderers() outputflag.OutputFlag[policyGroupGetRender] {
	return outputflag.OutputFlag[policyGroupGetRender]{
		RenderForTerminal: policyGroupGetRender{
			group:           renderPolicyGroupGetText,
			groupWithConfig: renderPolicyGroupGetTextWithConfig,
			stacks:          renderPolicyGroupStacksTable,
			accounts:        renderPolicyGroupAccountsTable,
		},
		RenderJSON: policyGroupGetRender{
			group:           renderPolicyGroupGetJSON,
			groupWithConfig: renderPolicyGroupGetJSON,
			stacks:          renderPolicyGroupStacksJSON,
			accounts:        renderPolicyGroupAccountsJSON,
		},
	}
}

const (
	policyGroupMembersDefaultCount = 100
	policyGroupMembersPageSize     = 500
)

// newPolicyGroupGetCmd builds `pulumi policy group get` with the production
// client factory. The factory is overridable via newPolicyGroupGetCmdWith for
// tests.
func newPolicyGroupGetCmd() *cobra.Command {
	return newPolicyGroupGetCmdWith(defaultPolicyGroupGetClientFactory)
}

func newPolicyGroupGetCmdWith(factory policyGroupGetClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "policyGroupGetClientFactory must not be nil")
	var args policyGroupGetArgs
	args.outputFormat = defaultPolicyGroupGetRenderers()

	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "[EXPERIMENTAL] Get the details of a Policy Group",
		Long: "[EXPERIMENTAL] Get the details of a Policy Group.\n" +
			"\n" +
			"Retrieves detailed information about a single Policy Group in an\n" +
			"organization, including the list of Policy Packs applied to it, the\n" +
			"stacks or Insights accounts added to the group individually, and the\n" +
			"stack and account match rules, if any. Match rules are shown in the\n" +
			"syntax `pulumi policy group edit --stack-rule` accepts, or as JSON when\n" +
			"that syntax cannot express them.\n" +
			"\n" +
			"Pass --stacks or --accounts to list every stack or Insights account that\n" +
			"is a member of the group instead: both those added individually and\n" +
			"those matched by the group's match rule. --source narrows the list\n" +
			"to one of the two. The first 100 members are shown by default; use\n" +
			"--count or --all to fetch more.\n" +
			"\n" +
			"Each applied Policy Pack shows how many of its policies the group configures;\n" +
			"pass --show-config to list them.\n" +
			"\n" +
			"Default output is a human-readable summary; pass --output=json for the\n" +
			"full response as JSON.",
		Example: "  # Show a Policy Group\n" +
			"  pulumi policy group get prod-policies\n\n" +
			"  # Show a Policy Group with each pack's policy configuration\n" +
			"  pulumi policy group get prod-policies --show-config\n\n" +
			"  # Show a Policy Group in another organization as JSON\n" +
			"  pulumi policy group get prod-policies --org acme --output json\n\n" +
			"  # List every stack in a Policy Group, including those matched by its match rule\n" +
			"  pulumi policy group get prod-policies --stacks --all\n\n" +
			"  # List only the Insights accounts matched by a Policy Group's match rule\n" +
			"  pulumi policy group get compliance --accounts --source rule",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			return runPolicyGroupGet(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), factory, posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "name"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&args.org, "org", "", "The organization that owns the Policy Group")
	cmd.Flags().BoolVar(&args.stacks, "stacks", false,
		"List every stack in the group, added individually or matched by its match rule")
	cmd.Flags().BoolVar(&args.accounts, "accounts", false,
		"List every Insights account in the group, added individually or matched by its match rule")
	cmd.Flags().BoolVar(&args.showConfig, "show-config", false,
		"List each policy pack's configured policies under the pack")
	cmd.Flags().StringVar(&args.source, "source", "",
		"With --stacks or --accounts, list only members that were added individually (explicit) "+
			"or matched by the match rule (rule)")
	cmd.Flags().IntVar(&args.count, "count", 0,
		"With --stacks or --accounts, the maximum number of members to return (default 100)")
	cmd.Flags().BoolVar(&args.all, "all", false,
		"With --stacks or --accounts, return every member; mutually exclusive with --count")
	cmd.MarkFlagsMutuallyExclusive("stacks", "accounts")
	cmd.MarkFlagsMutuallyExclusive("count", "all")
	cmd.MarkFlagsMutuallyExclusive("show-config", "stacks")
	cmd.MarkFlagsMutuallyExclusive("show-config", "accounts")
	outputflag.VarP(cmd.Flags(), &args.outputFormat)

	return cmd
}

// defaultPolicyGroupGetClientFactory is the production wiring: resolve the
// cloud backend, pick the effective organization, and hand back the
// underlying *client.Client.
func defaultPolicyGroupGetClientFactory(
	ctx context.Context, orgFlag string,
) (policyGroupGetClient, string, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	be, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, nil, opts)
	if err != nil {
		return nil, "", err
	}
	cloudBackend, ok := be.(httpstate.Backend)
	if !ok {
		return nil, "", errors.New(
			"getting a Policy Group requires the Pulumi Cloud backend; run `pulumi login`")
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

// runPolicyGroupGet is the cobra-decoupled command body so tests can drive
// it directly without spinning up the flag parser.
func runPolicyGroupGet(
	ctx context.Context, w, errW io.Writer,
	factory policyGroupGetClientFactory, name string, args policyGroupGetArgs,
) error {
	source, err := validatePolicyGroupGetArgs(args)
	if err != nil {
		return err
	}

	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	render := args.outputFormat.Get()
	want := policyGroupMembersDefaultCount
	if args.count > 0 {
		want = args.count
	} else if args.all {
		want = -1
	}

	switch {
	case args.stacks:
		stacks, more, err := fetchPolicyGroupMembers(ctx, want, source,
			func(ctx context.Context, opts client.ListPolicyGroupMembersOptions) (
				[]cloudapitype.PolicyGroupStack, *string, error,
			) {
				resp, err := c.ListPolicyGroupStacks(ctx, org, name, opts)
				return resp.Stacks, resp.ContinuationToken, err
			})
		if err != nil {
			return err
		}
		if err := render.stacks(w, cloudapitype.ListPolicyGroupStacksResponse{Stacks: stacks}); err != nil {
			return err
		}
		printMorePolicyGroupMembersHint(errW, more, "stacks")
		return nil
	case args.accounts:
		accounts, more, err := fetchPolicyGroupMembers(ctx, want, source,
			func(ctx context.Context, opts client.ListPolicyGroupMembersOptions) (
				[]cloudapitype.PolicyGroupAccount, *string, error,
			) {
				resp, err := c.ListPolicyGroupAccounts(ctx, org, name, opts)
				return resp.Accounts, resp.ContinuationToken, err
			})
		if err != nil {
			return err
		}
		if err := render.accounts(w, cloudapitype.ListPolicyGroupAccountsResponse{Accounts: accounts}); err != nil {
			return err
		}
		printMorePolicyGroupMembersHint(errW, more, "accounts")
		return nil
	}

	resp, err := c.GetPolicyGroup(ctx, org, name)
	if err != nil {
		return fmt.Errorf("getting policy group: %w", err)
	}

	if args.showConfig {
		return render.groupWithConfig(w, resp)
	}
	return render.group(w, resp)
}

func validatePolicyGroupGetArgs(args policyGroupGetArgs) (cloudapitype.PolicyGroupMemberSource, error) {
	if !args.stacks && !args.accounts {
		for _, f := range []struct {
			name string
			set  bool
		}{{"source", args.source != ""}, {"count", args.count != 0}, {"all", args.all}} {
			if f.set {
				return "", fmt.Errorf("--%s requires --stacks or --accounts", f.name)
			}
		}
		return "", nil
	}
	if args.count < 0 {
		return "", fmt.Errorf("--count must be non-negative, got %d", args.count)
	}
	source := cloudapitype.PolicyGroupMemberSource(args.source)
	if args.source != "" && !source.IsValid() {
		return "", fmt.Errorf("--source must be one of explicit or rule, got %q", args.source)
	}
	return source, nil
}

// fetchPolicyGroupMembers follows continuation tokens until it has want
// members or the service has no more. want < 0 fetches every page. more
// reports whether members were left unfetched.
func fetchPolicyGroupMembers[T any](
	ctx context.Context, want int, source cloudapitype.PolicyGroupMemberSource,
	list func(context.Context, client.ListPolicyGroupMembersOptions) ([]T, *string, error),
) (members []T, more bool, err error) {
	var token *string
	for {
		pageSize := policyGroupMembersPageSize
		if want > 0 && want-len(members) < pageSize {
			pageSize = want - len(members)
		}
		page, next, err := list(ctx, client.ListPolicyGroupMembersOptions{
			Source:            source,
			Count:             pageSize,
			ContinuationToken: token,
		})
		if err != nil {
			return nil, false, err
		}
		members = append(members, page...)

		hasNext := next != nil && *next != ""
		if want > 0 && len(members) >= want {
			return members[:want], hasNext || len(members) > want, nil
		}
		// An empty page that still carries a token would otherwise loop forever.
		if !hasNext || len(page) == 0 {
			return members, false, nil
		}
		token = next
	}
}

func printMorePolicyGroupMembersHint(w io.Writer, more bool, noun string) {
	if more {
		fmt.Fprintf(w, "More %s available; re-run with --count <N> or --all to fetch more.\n", noun)
	}
}

func formatMemberTags(tags map[string]string) string {
	if len(tags) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	pairs := make([]string, len(keys))
	for i, k := range keys {
		pairs[i] = k + "=" + tags[k]
	}
	return strings.Join(pairs, ", ")
}

func renderPolicyGroupStacksTable(w io.Writer, resp cloudapitype.ListPolicyGroupStacksResponse) error {
	if len(resp.Stacks) == 0 {
		fmt.Fprintln(w, "No stacks found.")
		return nil
	}
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"NAME", "SOURCE", "TAGS"})
	for _, s := range resp.Stacks {
		name := s.Name
		if s.RoutingProject != "" {
			name = s.RoutingProject + "/" + s.Name
		}
		t.AppendRow(table.Row{name, string(s.Source), formatMemberTags(s.Tags)})
	}
	t.Render()
	return nil
}

func renderPolicyGroupAccountsTable(w io.Writer, resp cloudapitype.ListPolicyGroupAccountsResponse) error {
	if len(resp.Accounts) == 0 {
		fmt.Fprintln(w, "No accounts found.")
		return nil
	}
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"NAME", "SOURCE", "TAGS"})
	for _, a := range resp.Accounts {
		t.AppendRow(table.Row{a.Name, string(a.Source), formatMemberTags(a.Tags)})
	}
	t.Render()
	return nil
}

func renderPolicyGroupStacksJSON(w io.Writer, resp cloudapitype.ListPolicyGroupStacksResponse) error {
	if resp.Stacks == nil {
		resp.Stacks = []cloudapitype.PolicyGroupStack{}
	}
	return encodePolicyGroupGetJSON(w, resp)
}

func renderPolicyGroupAccountsJSON(w io.Writer, resp cloudapitype.ListPolicyGroupAccountsResponse) error {
	if resp.Accounts == nil {
		resp.Accounts = []cloudapitype.PolicyGroupAccount{}
	}
	return encodePolicyGroupGetJSON(w, resp)
}

func encodePolicyGroupGetJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

type policyGroupGetRenderFunc func(w io.Writer, resp apitype.GetPolicyGroupResponse) error

func renderPolicyGroupGetText(w io.Writer, resp apitype.GetPolicyGroupResponse) error {
	return writePolicyGroupText(w, resp, false)
}

func renderPolicyGroupGetTextWithConfig(w io.Writer, resp apitype.GetPolicyGroupResponse) error {
	return writePolicyGroupText(w, resp, true)
}

func writePolicyGroupText(w io.Writer, resp apitype.GetPolicyGroupResponse, showConfig bool) error {
	isDefault := "no"
	if resp.IsOrgDefault {
		isDefault = "yes"
	}

	fmt.Fprintf(w, "%-22s %s\n", "Name:", resp.Name)
	fmt.Fprintf(w, "%-22s %s\n", "Org default:", isDefault)
	fmt.Fprintf(w, "%-22s %s\n", "Mode:", string(resp.Mode))
	if resp.AgentPoolID != nil {
		fmt.Fprintf(w, "%-22s %s\n", "Agent pool:", *resp.AgentPoolID)
	}

	fmt.Fprintf(w, "%-22s %d\n", "Applied policy packs:", len(resp.AppliedPolicyPacks))
	for _, pack := range resp.AppliedPolicyPacks {
		version := pack.VersionTag
		if version == "" {
			version = fmt.Sprintf("v%d", pack.Version)
		}
		fmt.Fprintf(w, "  - %s (%s)%s\n", pack.Name, version, formatPackConfigSummary(pack.Config))
		if showConfig {
			for _, name := range packConfigNames(pack.Config) {
				fmt.Fprintf(w, "      %s: %s\n", name, formatPackConfigValue(pack.Config[name]))
			}
		}
	}

	if resp.StackMatchRule != nil {
		fmt.Fprintf(w, "%-22s %s\n", "Stack rule:", formatMatchRule(stackMatchRule, resp.StackMatchRule))
	}
	fmt.Fprintf(w, "%-22s %d\n", "Stacks:", len(resp.Stacks))
	for _, s := range resp.Stacks {
		if s.RoutingProject != "" {
			fmt.Fprintf(w, "  - %s/%s\n", s.RoutingProject, s.Name)
		} else {
			fmt.Fprintf(w, "  - %s\n", s.Name)
		}
	}

	if resp.AccountMatchRule != nil {
		fmt.Fprintf(w, "%-22s %s\n", "Account rule:", formatMatchRule(accountMatchRule, resp.AccountMatchRule))
	}
	fmt.Fprintf(w, "%-22s %d\n", "Accounts:", len(resp.Accounts))
	for _, a := range resp.Accounts {
		fmt.Fprintf(w, "  - %s\n", a)
	}
	return nil
}

const packConfigAllKey = "all"

func formatPackConfigSummary(config map[string]*json.RawMessage) string {
	policies := 0
	for name := range config {
		if name != packConfigAllKey {
			policies++
		}
	}
	var summary string
	switch policies {
	case 0:
	case 1:
		summary = ", 1 configured policy"
	default:
		summary = fmt.Sprintf(", %d configured policies", policies)
	}
	if _, ok := config[packConfigAllKey]; ok {
		summary += ", settings for all policies"
	}
	return summary
}

// packConfigNames returns the config keys with "all" first and the rest sorted.
func packConfigNames(config map[string]*json.RawMessage) []string {
	names := make([]string, 0, len(config))
	for name := range config {
		if name != packConfigAllKey {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	if _, ok := config[packConfigAllKey]; ok {
		names = append([]string{packConfigAllKey}, names...)
	}
	return names
}

func formatPackConfigValue(value *json.RawMessage) string {
	if value == nil {
		return "null"
	}
	var s string
	if err := json.Unmarshal(*value, &s); err == nil {
		return s
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(*value, &obj); err != nil || len(obj) == 0 {
		return compactJSON(*value)
	}
	var parts []string
	if level, ok := obj["enforcementLevel"]; ok {
		var levelStr string
		if err := json.Unmarshal(level, &levelStr); err == nil {
			parts = append(parts, levelStr)
		} else {
			parts = append(parts, "enforcementLevel="+compactJSON(level))
		}
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		if k != "enforcementLevel" {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	for _, k := range keys {
		parts = append(parts, k+"="+compactJSON(obj[k]))
	}
	return strings.Join(parts, ", ")
}

func compactJSON(raw json.RawMessage) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return string(raw)
	}
	return buf.String()
}

// renderPolicyGroupGetJSON normalizes nil slices to empty arrays so scripts can
// rely on the array keys always existing.
func renderPolicyGroupGetJSON(w io.Writer, resp apitype.GetPolicyGroupResponse) error {
	if resp.Stacks == nil {
		resp.Stacks = []apitype.PulumiStackReference{}
	}
	if resp.AppliedPolicyPacks == nil {
		resp.AppliedPolicyPacks = []apitype.PolicyPackMetadata{}
	}
	if resp.Accounts == nil {
		resp.Accounts = []string{}
	}
	return encodePolicyGroupGetJSON(w, resp)
}
