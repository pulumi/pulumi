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
	"cmp"
	"context"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type policyExceptionListRenderFunc func(w io.Writer, exceptions []apitype.PolicyExceptionInfo) error

type policyExceptionListArgs struct {
	pack     string
	org      string
	stack    string
	policy   string
	statuses []string
	output   outputflag.OutputFlag[policyExceptionListRenderFunc]
}

func newPolicyExceptionListCmd(factory policyExceptionEnvFactory) *cobra.Command {
	args := policyExceptionListArgs{output: outputflag.OutputFlag[policyExceptionListRenderFunc]{
		RenderForTerminal: renderPolicyExceptionListText,
		RenderJSON: func(w io.Writer, exceptions []apitype.PolicyExceptionInfo) error {
			return writeJSON(w, apitype.ListPolicyExceptionsResponse{Exceptions: exceptions})
		},
	}}

	cmd := &cobra.Command{
		Use:   "list [<policy-pack>]",
		Short: "[EXPERIMENTAL] List policy exceptions",
		Long: "[EXPERIMENTAL] List policy exceptions.\n" +
			"\n" +
			"Lists the organization's active policy exceptions, optionally only those of one policy pack.",
		Example: "  pulumi policy exception list\n" +
			"  pulumi policy exception list aws-security --stack web/prod\n" +
			"  pulumi policy exception list --status active --status removed --output json",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			if len(posArgs) > 0 {
				args.pack = posArgs[0]
			}
			env, err := factory(cmd.Context())
			if err != nil {
				return err
			}
			return runPolicyExceptionList(cmd.Context(), cmd.OutOrStdout(), env, args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "policy-pack"}},
	})

	cmd.Flags().StringVar(&args.org, "org", "", "The organization to list exceptions of. Defaults to the default org")
	cmd.Flags().StringVarP(&args.stack, "stack", "s", "",
		"Only list exceptions with targets in this stack, as <project>/<stack>")
	cmd.Flags().StringVar(&args.policy, "policy", "", "Only list exceptions that name this policy")
	cmd.Flags().StringArrayVar(&args.statuses, "status", nil,
		"Only list exceptions with this status: active (default) or removed; repeatable")
	outputflag.VarP(cmd.Flags(), &args.output)
	return cmd
}

func runPolicyExceptionList(
	ctx context.Context, w io.Writer, env policyExceptionEnv, args policyExceptionListArgs,
) error {
	packOrg, pack, err := parsePolicyPackArg(args.pack)
	if err != nil {
		return err
	}
	statuses := args.statuses
	if len(statuses) == 0 {
		statuses = []string{"active"}
	}
	org := cmp.Or(packOrg, args.org, env.org)
	exceptions, err := env.client.ListPolicyExceptions(ctx, org, client.ListPolicyExceptionsQuery{
		PolicyPack: pack,
		Policy:     args.policy,
		Stack:      args.stack,
		Status:     statuses,
	})
	if err != nil {
		return fmt.Errorf("listing policy exceptions: %w", err)
	}
	return args.output.Get()(w, exceptions)
}

func renderPolicyExceptionListText(w io.Writer, exceptions []apitype.PolicyExceptionInfo) error {
	if len(exceptions) == 0 {
		_, err := fmt.Fprintln(w, "No policy exceptions found.")
		return err
	}
	rows := make([]cmdutil.TableRow, 0, len(exceptions))
	for _, e := range exceptions {
		var policies, targets []string
		for _, m := range e.Match {
			policies = append(policies, m.Policies...)
			targets = append(targets, m.Targets...)
		}
		rows = append(rows, cmdutil.TableRow{Columns: []string{
			e.ID, e.PolicyPack, e.Status, summarize(policies), strconv.Itoa(len(targets)), e.Reason,
		}})
	}
	return cmdutil.FprintTable(w, cmdutil.Table{
		Headers: []string{"ID", "POLICY PACK", "STATUS", "POLICIES", "TARGETS", "REASON"},
		Rows:    rows,
	})
}

// summarize lists the distinct values, up to two, then how many more there are.
func summarize(values []string) string {
	var distinct []string
	for _, v := range values {
		if !slices.Contains(distinct, v) {
			distinct = append(distinct, v)
		}
	}
	if len(distinct) <= 2 {
		return strings.Join(distinct, ", ")
	}
	return fmt.Sprintf("%s, +%d", strings.Join(distinct[:2], ", "), len(distinct)-2)
}
