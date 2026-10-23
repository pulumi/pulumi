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

package stack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type stackPolicyListClient interface {
	GetStackPolicyGroups(
		ctx context.Context, stackID client.StackIdentifier,
	) (cloudapitype.AppListPolicyGroupsResponse, error)
	GetStackPolicyPacksForMode(
		ctx context.Context, stackID client.StackIdentifier, mode cloudapitype.PolicyGroupMode,
	) (cloudapitype.AppGetStackPolicyPacksResponse, error)
}

type stackPolicyListClientFactory func(
	ctx context.Context, stackFlag string,
) (stackPolicyListClient, client.StackIdentifier, error)

type stackPolicyListRender func(w io.Writer, result stackPolicyListResult) error

// stackPolicyListResult is the --output json shape of `pulumi stack policy ls`.
type stackPolicyListResult struct {
	Stack        string                                                            `json:"stack"`
	PolicyGroups []cloudapitype.AppPolicyGroupSummary                              `json:"policyGroups"`
	PolicyPacks  map[cloudapitype.PolicyGroupMode][]cloudapitype.AppRequiredPolicy `json:"policyPacks"`
}

// stackPolicyModes are the modes whose packs are listed, in display order.
var stackPolicyModes = []cloudapitype.PolicyGroupMode{
	cloudapitype.PolicyGroupModePreventative,
	cloudapitype.PolicyGroupModeAudit,
}

type stackPolicyListArgs struct {
	stack  string
	output outputflag.OutputFlag[stackPolicyListRender]
}

func newStackPolicyListCmd() *cobra.Command {
	return newStackPolicyListCmdWith(defaultStackPolicyListClientFactory)
}

func newStackPolicyListCmdWith(factory stackPolicyListClientFactory) *cobra.Command {
	args := stackPolicyListArgs{output: defaultStackPolicyListOutput()}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "[EXPERIMENTAL] List the Policy Groups and Policy Packs that apply to a stack",
		Long: "[EXPERIMENTAL] List the Policy Groups and Policy Packs that apply to a stack.\n" +
			"\n" +
			"Lists the Policy Groups the stack is a member of and the Policy Packs\n" +
			"those groups enable for it. Each pack is listed with the mode of the\n" +
			"groups that enable it: preventative packs run during\n" +
			"`pulumi preview` and `pulumi up`; audit packs report policy issues\n" +
			"without blocking deployments.\n" +
			"\n" +
			"Requires the Pulumi Cloud backend.",
		Example: "  # List the policies that apply to the current stack\n" +
			"  pulumi stack policy ls\n\n" +
			"  # List the policies that apply to another stack as JSON\n" +
			"  pulumi stack policy ls --stack acme/web/prod --output json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runStackPolicyList(cmd.Context(), cmd.OutOrStdout(), factory, args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&args.stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	outputflag.Var(cmd.Flags(), &args.output)

	return cmd
}

func defaultStackPolicyListOutput() outputflag.OutputFlag[stackPolicyListRender] {
	return outputflag.OutputFlag[stackPolicyListRender]{
		RenderForTerminal: renderStackPolicyListTable,
		RenderJSON:        renderStackPolicyListJSON,
	}
}

func defaultStackPolicyListClientFactory(
	ctx context.Context, stackFlag string,
) (stackPolicyListClient, client.StackIdentifier, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	s, err := RequireStack(ctx, cmdutil.Diag(), ws, cmdBackend.DefaultLoginManager,
		stackFlag, LoadOnly, opts, "")
	if err != nil {
		return nil, client.StackIdentifier{}, fmt.Errorf("resolving stack: %w", err)
	}

	cloudStack, ok := s.(httpstate.Stack)
	if !ok {
		return nil, client.StackIdentifier{},
			errors.New("stack policy commands require the Pulumi Cloud backend; run `pulumi login`")
	}

	ref := cloudStack.Ref()
	project := ""
	if p, ok := ref.Project(); ok {
		project = string(p)
	}
	stackID := client.StackIdentifier{
		Owner:   cloudStack.OrgName(),
		Project: project,
		Stack:   ref.Name(),
	}

	be := cloudStack.Backend().(httpstate.Backend)
	return be.Client(), stackID, nil
}

func runStackPolicyList(
	ctx context.Context, w io.Writer, factory stackPolicyListClientFactory, args stackPolicyListArgs,
) error {
	c, stackID, err := factory(ctx, args.stack)
	if err != nil {
		return err
	}

	groups, err := c.GetStackPolicyGroups(ctx, stackID)
	if err != nil {
		return err
	}

	result := stackPolicyListResult{
		Stack:        stackID.String(),
		PolicyGroups: groups.PolicyGroups,
		PolicyPacks:  map[cloudapitype.PolicyGroupMode][]cloudapitype.AppRequiredPolicy{},
	}
	if result.PolicyGroups == nil {
		result.PolicyGroups = []cloudapitype.AppPolicyGroupSummary{}
	}

	for _, mode := range stackPolicyModes {
		packs, err := c.GetStackPolicyPacksForMode(ctx, stackID, mode)
		if err != nil {
			return err
		}
		if packs.RequiredPolicies == nil {
			packs.RequiredPolicies = []cloudapitype.AppRequiredPolicy{}
		}
		result.PolicyPacks[mode] = packs.RequiredPolicies
	}

	return args.output.Get()(w, result)
}

func renderStackPolicyListJSON(w io.Writer, result stackPolicyListResult) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func renderStackPolicyListTable(w io.Writer, result stackPolicyListResult) error {
	if len(result.PolicyGroups) == 0 {
		fmt.Fprintf(w, "No Policy Groups apply to stack %s.\n", result.Stack)
	} else {
		rows := make([]cmdutil.TableRow, 0, len(result.PolicyGroups))
		for _, g := range result.PolicyGroups {
			isDefault := "N"
			if g.IsOrgDefault {
				isDefault = "Y"
			}
			rows = append(rows, cmdutil.TableRow{Columns: []string{g.Name, string(g.Mode), isDefault}})
		}
		ui.FprintTable(w, cmdutil.Table{
			Headers: []string{"POLICY GROUP", "MODE", "DEFAULT"},
			Rows:    rows,
		}, nil)
	}

	fmt.Fprintln(w)

	var rows []cmdutil.TableRow
	for _, mode := range stackPolicyModes {
		for _, p := range result.PolicyPacks[mode] {
			version := p.VersionTag
			if version == "" {
				version = fmt.Sprintf("v%d", p.Version)
			}
			rows = append(rows, cmdutil.TableRow{Columns: []string{p.Name, version, string(mode)}})
		}
	}
	if len(rows) == 0 {
		fmt.Fprintf(w, "No Policy Packs apply to stack %s.\n", result.Stack)
		return nil
	}
	ui.FprintTable(w, cmdutil.Table{
		Headers: []string{"POLICY PACK", "VERSION", "MODE"},
		Rows:    rows,
	}, nil)
	return nil
}
