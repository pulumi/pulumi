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
	"net/http"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// policyExceptionDetails is an exception and where it allowed violations.
type policyExceptionDetails struct {
	Exception apitype.PolicyExceptionInfo         `json:"exception"`
	Usage     []apitype.PolicyExceptionMatchUsage `json:"usage"`
	URL       string                              `json:"url"`
}

type policyExceptionGetRenderFunc func(w io.Writer, details policyExceptionDetails) error

type policyExceptionGetArgs struct {
	org    string
	output outputflag.OutputFlag[policyExceptionGetRenderFunc]
}

func newPolicyExceptionGetCmd(factory policyExceptionEnvFactory) *cobra.Command {
	args := policyExceptionGetArgs{output: outputflag.OutputFlag[policyExceptionGetRenderFunc]{
		RenderForTerminal: renderPolicyExceptionGetText,
		RenderJSON: func(w io.Writer, details policyExceptionDetails) error {
			return writeJSON(w, details)
		},
	}}

	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "[EXPERIMENTAL] Show a policy exception",
		Long: "[EXPERIMENTAL] Show a policy exception.\n" +
			"\n" +
			"Shows what a policy exception allows, and where it has allowed violations.",
		Example: "  pulumi policy exception get EXC-12\n" +
			"  pulumi policy exception get EXC-12 --output json",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			env, err := factory(cmd.Context())
			if err != nil {
				return err
			}
			return runPolicyExceptionGet(cmd.Context(), cmd.OutOrStdout(), env, posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "id"}},
		Required:  1,
	})

	cmd.Flags().StringVar(&args.org, "org", "", "The organization of the exception. Defaults to the default org")
	outputflag.VarP(cmd.Flags(), &args.output)
	return cmd
}

func runPolicyExceptionGet(
	ctx context.Context, w io.Writer, env policyExceptionEnv, id string, args policyExceptionGetArgs,
) error {
	org := cmp.Or(args.org, env.org)
	exception, err := env.client.GetPolicyException(ctx, org, id)
	if apiErrorCode(err) == http.StatusNotFound {
		return fmt.Errorf("policy exception %s not found in %s", id, org)
	} else if err != nil {
		return fmt.Errorf("getting policy exception: %w", err)
	}
	usage, err := env.client.ListPolicyExceptionMatchUsage(ctx, org, id)
	if err != nil {
		return fmt.Errorf("getting where policy exception %s applied: %w", id, err)
	}
	return args.output.Get()(w, policyExceptionDetails{
		Exception: exception,
		Usage:     usage,
		URL:       exceptionConsoleURL(env, org, id),
	})
}

func renderPolicyExceptionGetText(w io.Writer, d policyExceptionDetails) error {
	e := d.Exception
	fmt.Fprintf(w, "%-14s %s\n", "ID:", e.ID)
	fmt.Fprintf(w, "%-14s %s\n", "Policy pack:", e.PolicyPack)
	fmt.Fprintf(w, "%-14s %s\n", "Status:", e.Status)
	if e.CreatedBy != nil {
		fmt.Fprintf(w, "%-14s %s, %s\n", "Created by:", e.CreatedBy.GitHubLogin, e.CreatedAt)
	}
	if e.RemovedBy != nil && e.RemovedAt != nil {
		fmt.Fprintf(w, "%-14s %s, %s\n", "Removed by:", e.RemovedBy.GitHubLogin, *e.RemovedAt)
	}
	fmt.Fprintf(w, "%-14s %s\n", "URL:", d.URL)
	fmt.Fprintf(w, "\n%s", configSnippet(e.ID, exceptionDefinition{Reason: e.Reason, Match: e.Match}))

	if len(d.Usage) == 0 {
		_, err := fmt.Fprintln(w, "\nIt hasn't allowed a violation yet.")
		return err
	}
	rows := make([]cmdutil.TableRow, 0, len(d.Usage))
	for _, u := range d.Usage {
		lastSeen, update := "-", "-"
		if u.LastSeenAt != nil {
			lastSeen = *u.LastSeenAt
		}
		if u.LastUpdateVersion != nil {
			update = strconv.Itoa(*u.LastUpdateVersion)
		}
		rows = append(rows, cmdutil.TableRow{Columns: []string{u.Policy, u.URN, lastSeen, update}})
	}
	fmt.Fprintln(w, "\nWhere it allowed violations:")
	return cmdutil.FprintTable(w, cmdutil.Table{
		Headers: []string{"POLICY", "URN", "LAST SEEN", "UPDATE"},
		Rows:    rows,
	})
}
