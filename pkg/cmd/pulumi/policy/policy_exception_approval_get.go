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
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
)

type policyExceptionApprovalGetRenderFunc func(w io.Writer, r *policyExceptionRequest) error

type policyExceptionApprovalGetArgs struct {
	org          string
	outputFormat outputflag.OutputFlag[policyExceptionApprovalGetRenderFunc]
}

func newPolicyExceptionApprovalGetCmd(factory policyExceptionClientFactory) *cobra.Command {
	args := policyExceptionApprovalGetArgs{
		outputFormat: outputflag.OutputFlag[policyExceptionApprovalGetRenderFunc]{
			RenderForTerminal: renderPolicyExceptionRequest,
			RenderJSON: func(w io.Writer, r *policyExceptionRequest) error {
				return ui.FprintJSON(w, r)
			},
		},
	}
	cmd := &cobra.Command{
		Use:   "get",
		Short: "[EXPERIMENTAL] Show a request for a policy exception",
		Long: "[EXPERIMENTAL] Show a request for a policy exception.\n" +
			"\n" +
			"Shows the request's status and approvals, and the exception it creates.",
		Example: "  pulumi policy exception approval get <id>\n" +
			"  pulumi policy exception approval get <id> --output json",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			c, org, err := factory(cmd.Context(), args.org)
			if err != nil {
				return err
			}
			r, err := getPolicyExceptionRequest(cmd.Context(), c, org, posArgs[0])
			if err != nil {
				return err
			}
			return args.outputFormat.Get()(cmd.OutOrStdout(), r)
		},
	}

	constrictor.AttachArguments(cmd, requestIDArgument)
	addPolicyExceptionApprovalOrgFlag(cmd, &args.org)
	outputflag.VarP(cmd.Flags(), &args.outputFormat)
	return cmd
}

func renderPolicyExceptionRequest(w io.Writer, r *policyExceptionRequest) error {
	fmt.Fprintf(w, "%-14s %s\n", "ID:", r.ID)
	fmt.Fprintf(w, "%-14s %s\n", "Policy pack:", r.Exception.PolicyPack)
	fmt.Fprintf(w, "%-14s %s\n", "Status:", r.Status)
	if r.ExceptionID != "" {
		fmt.Fprintf(w, "%-14s %s\n", "Exception:", r.ExceptionID)
	}
	fmt.Fprintf(w, "%-14s %s by %s\n", "Requested:", r.CreatedAt.UTC().Format(time.RFC3339), r.CreatedBy.GithubLogin)
	if r.Exception.ExpiresInSeconds != nil {
		fmt.Fprintf(w, "%-14s %s after it's created\n", "Expires:", policyExceptionExpiresIn(*r.Exception.ExpiresInSeconds))
	}
	fmt.Fprintf(w, "%-14s %t\n", "Approved:", r.GateEvaluation.Satisfied)
	for _, line := range gateLines(r.GateEvaluation) {
		fmt.Fprintf(w, "  %s\n", line)
	}
	if r.URL != "" {
		fmt.Fprintf(w, "%-14s %s\n", "URL:", r.URL)
	}

	fmt.Fprintln(w, "\nPolicy Pack exceptions configuration:")
	return ui.FprintJSON(w, policyExceptionConfig(r.Exception))
}
