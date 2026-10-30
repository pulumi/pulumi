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
	"net/http"
	"slices"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// policyExceptionListRenderFunc writes the exceptions to stdout, and status messages to stderr.
type policyExceptionListRenderFunc func(stdout, stderr io.Writer, exceptions []apitype.PolicyExceptionInfo) error

type policyExceptionListArgs struct {
	org          string
	policyPack   string
	policy       string
	stack        string
	statuses     []string
	outputFormat outputflag.OutputFlag[policyExceptionListRenderFunc]
}

func defaultPolicyExceptionListOutputFormat() outputflag.OutputFlag[policyExceptionListRenderFunc] {
	return outputflag.OutputFlag[policyExceptionListRenderFunc]{
		RenderForTerminal: renderPolicyExceptionListTable,
		RenderJSON: func(w, _ io.Writer, exceptions []apitype.PolicyExceptionInfo) error {
			if exceptions == nil {
				exceptions = []apitype.PolicyExceptionInfo{}
			}
			return ui.FprintJSON(w, apitype.ListPolicyExceptionsResponse{Exceptions: exceptions})
		},
	}
}

func newPolicyExceptionListCmd(ws pkgWorkspace.Context, lm cmdBackend.LoginManager) *cobra.Command {
	var args policyExceptionListArgs
	args.outputFormat = defaultPolicyExceptionListOutputFormat()

	cmd := &cobra.Command{
		Use:     "list [<policy-pack>]",
		Aliases: []string{"ls"},
		Short:   "[EXPERIMENTAL] List policy exceptions for an organization",
		Long: "[EXPERIMENTAL] List policy exceptions for an organization.\n" +
			"\n" +
			"Lists the organization's active policy exceptions, most recently created first. Pass a\n" +
			"Policy Pack name to list only that Policy Pack's exceptions.\n" +
			"\n" +
			"Default output is a human-readable table; pass --output=json for the\n" +
			"full response as a JSON envelope.",
		Example: "  pulumi policy exception list\n" +
			"  pulumi policy exception list aws-security --stack web/prod\n" +
			"  pulumi policy exception list --status active --status removed --output json",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			if len(posArgs) > 0 {
				args.policyPack = posArgs[0]
			}
			return runPolicyExceptionList(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(),
				newPolicyExceptionClientFactory(ws, lm), args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "policy-pack"},
		},
	})

	cmd.Flags().StringVar(&args.org, "org", "", "The organization to list policy exceptions for")
	cmd.Flags().StringVar(&args.policy, "policy", "", "Only list exceptions that name this policy")
	cmd.Flags().StringVar(&args.stack, "stack", "",
		"Only list exceptions with targets in this stack, as <project>/<stack>")
	cmd.Flags().StringArrayVar(&args.statuses, "status", nil,
		"Only list exceptions with this status: active (default) or removed; may be repeated")
	outputflag.VarP(cmd.Flags(), &args.outputFormat)

	return cmd
}

func runPolicyExceptionList(
	ctx context.Context, w, stderr io.Writer,
	factory policyExceptionClientFactory, args policyExceptionListArgs,
) error {
	if args.stack != "" {
		project, stack, ok := strings.Cut(args.stack, "/")
		if !ok || project == "" || stack == "" || strings.Contains(stack, "/") {
			return cmdCmd.ConfigurationError{Message: fmt.Sprintf("--stack must be <project>/<stack>, got %q", args.stack)}
		}
	}

	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	var exceptions []apitype.PolicyExceptionInfo
	opts := client.ListPolicyExceptionsOptions{
		PolicyPack: args.policyPack,
		Policy:     args.policy,
		Stack:      args.stack,
		Status:     args.statuses,
	}
	for {
		resp, err := c.ListPolicyExceptions(ctx, org, opts)
		err = policyExceptionsError(err, org)
		if apiErr, ok := errors.AsType[*apitype.ErrorResponse](err); ok && apiErr.Code == http.StatusNotFound {
			return fmt.Errorf("policy exceptions aren't available for %s on this Pulumi Cloud", org)
		} else if err != nil {
			return err
		}
		exceptions = append(exceptions, resp.Exceptions...)
		if resp.ContinuationToken == "" {
			break
		}
		opts.ContinuationToken = resp.ContinuationToken
	}

	return args.outputFormat.Get()(w, stderr, exceptions)
}

func renderPolicyExceptionListTable(w, stderr io.Writer, exceptions []apitype.PolicyExceptionInfo) error {
	if len(exceptions) == 0 {
		fmt.Fprintln(stderr, "No policy exceptions found.")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"ID", "POLICY PACK", "STATUS", "POLICIES", "TARGETS", "REASON"})
	for _, e := range exceptions {
		var policies []string
		var targets []string
		for _, m := range e.Match {
			policies = append(policies, m.Policies...)
			targets = append(targets, m.Targets...)
		}
		slices.Sort(policies)
		slices.Sort(targets)
		t.AppendRow(table.Row{
			e.ID,
			e.PolicyPack,
			e.Status,
			strings.Join(slices.Compact(policies), ", "),
			len(slices.Compact(targets)),
			truncateMessage(e.Reason, policyIssueMessageMax),
		})
	}
	t.Render()
	return nil
}
