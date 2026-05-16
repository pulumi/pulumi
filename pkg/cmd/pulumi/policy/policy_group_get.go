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

// policyGroupGetClient is the narrow subset of cloud-API operations the get
// command needs. Defined here so tests can stub a thin interface instead of
// the full HTTP client surface.
type policyGroupGetClient interface {
	GetPolicyGroup(
		ctx context.Context, orgName, policyGroup string,
	) (apitype.GetPolicyGroupResponse, error)
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
	outputFormat outputflag.OutputFlag[policyGroupGetRenderFunc]
}

// defaultPolicyGroupGetOutputFormat wires the OutputFlag to the per-format
// renderers so `--output` selects between them.
func defaultPolicyGroupGetOutputFormat() outputflag.OutputFlag[policyGroupGetRenderFunc] {
	return outputflag.OutputFlag[policyGroupGetRenderFunc]{
		RenderForTerminal: renderPolicyGroupGetText,
		RenderJSON:        renderPolicyGroupGetJSON,
	}
}

// newPolicyGroupGetCmd builds `pulumi policy group get` with the production
// client factory. The factory is overridable via newPolicyGroupGetCmdWith for
// tests.
func newPolicyGroupGetCmd() *cobra.Command {
	return newPolicyGroupGetCmdWith(defaultPolicyGroupGetClientFactory)
}

func newPolicyGroupGetCmdWith(factory policyGroupGetClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "policyGroupGetClientFactory must not be nil")
	var args policyGroupGetArgs
	args.outputFormat = defaultPolicyGroupGetOutputFormat()

	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "[EXPERIMENTAL] Get the details of a Policy Group",
		Long: "[EXPERIMENTAL] Get the details of a Policy Group.\n" +
			"\n" +
			"Retrieves detailed information about a single Policy Group in an\n" +
			"organization, including the list of Policy Packs applied to it and\n" +
			"the stacks or Insights accounts that are members of the group.\n" +
			"\n" +
			"Default output is a human-readable summary; pass --output=json for the\n" +
			"full response as JSON.",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			return runPolicyGroupGet(cmd.Context(), cmd.OutOrStdout(), factory, posArgs[0], args)
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
	ctx context.Context, w io.Writer,
	factory policyGroupGetClientFactory, name string, args policyGroupGetArgs,
) error {
	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	resp, err := c.GetPolicyGroup(ctx, org, name)
	if err != nil {
		return fmt.Errorf("getting policy group: %w", err)
	}

	return args.outputFormat.Get()(w, resp)
}

type policyGroupGetRenderFunc func(w io.Writer, resp apitype.GetPolicyGroupResponse) error

func renderPolicyGroupGetText(w io.Writer, resp apitype.GetPolicyGroupResponse) error {
	isDefault := "no"
	if resp.IsOrgDefault {
		isDefault = "yes"
	}

	fmt.Fprintf(w, "%-22s %s\n", "Name:", resp.Name)
	fmt.Fprintf(w, "%-22s %s\n", "Org default:", isDefault)
	fmt.Fprintf(w, "%-22s %s\n", "Entity type:", string(resp.EntityType))
	fmt.Fprintf(w, "%-22s %s\n", "Mode:", string(resp.Mode))
	if resp.AgentPoolID != "" {
		fmt.Fprintf(w, "%-22s %s\n", "Agent pool:", resp.AgentPoolID)
	}

	fmt.Fprintf(w, "%-22s %d\n", "Applied policy packs:", len(resp.AppliedPolicyPacks))
	for _, pack := range resp.AppliedPolicyPacks {
		version := pack.VersionTag
		if version == "" {
			version = fmt.Sprintf("v%d", pack.Version)
		}
		fmt.Fprintf(w, "  - %s (%s)\n", pack.Name, version)
	}

	fmt.Fprintf(w, "%-22s %d\n", "Stacks:", len(resp.Stacks))
	for _, s := range resp.Stacks {
		if s.RoutingProject != "" {
			fmt.Fprintf(w, "  - %s/%s\n", s.RoutingProject, s.Name)
		} else {
			fmt.Fprintf(w, "  - %s\n", s.Name)
		}
	}

	fmt.Fprintf(w, "%-22s %d\n", "Accounts:", len(resp.Accounts))
	for _, a := range resp.Accounts {
		fmt.Fprintf(w, "  - %s\n", a)
	}
	return nil
}

// policyGroupGetJSON is the JSON envelope emitted by
// `pulumi policy group get --output=json`. Nil slices are normalized to empty
// arrays so scripts can rely on the array keys always existing.
type policyGroupGetJSON struct {
	Name               string                         `json:"name"`
	IsOrgDefault       bool                           `json:"isOrgDefault"`
	EntityType         apitype.EntityType             `json:"entityType"`
	Mode               apitype.PolicyGroupMode        `json:"mode"`
	Stacks             []apitype.PulumiStackReference `json:"stacks"`
	AppliedPolicyPacks []apitype.PolicyPackMetadata   `json:"appliedPolicyPacks"`
	Accounts           []string                       `json:"accounts"`
	AgentPoolID        string                         `json:"agentPoolId,omitempty"`
}

func toPolicyGroupGetJSON(resp apitype.GetPolicyGroupResponse) policyGroupGetJSON {
	stacks := resp.Stacks
	if stacks == nil {
		stacks = []apitype.PulumiStackReference{}
	}
	packs := resp.AppliedPolicyPacks
	if packs == nil {
		packs = []apitype.PolicyPackMetadata{}
	}
	accounts := resp.Accounts
	if accounts == nil {
		accounts = []string{}
	}
	return policyGroupGetJSON{
		Name:               resp.Name,
		IsOrgDefault:       resp.IsOrgDefault,
		EntityType:         resp.EntityType,
		Mode:               resp.Mode,
		Stacks:             stacks,
		AppliedPolicyPacks: packs,
		Accounts:           accounts,
		AgentPoolID:        resp.AgentPoolID,
	}
}

func renderPolicyGroupGetJSON(w io.Writer, resp apitype.GetPolicyGroupResponse) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(toPolicyGroupGetJSON(resp))
}
