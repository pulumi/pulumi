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
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
)

func newPolicyExceptionApprovalApproveCmd(factory policyExceptionClientFactory) *cobra.Command {
	var args policyExceptionApprovalDecisionArgs
	cmd := &cobra.Command{
		Use:   "approve",
		Short: "[EXPERIMENTAL] Approve a request for a policy exception",
		Long: "[EXPERIMENTAL] Approve a request for a policy exception.\n" +
			"\n" +
			"Shows the exception the request creates and approves the request's latest revision. Once\n" +
			"the request has the approvals it needs, it's applied, which creates the exception, and the\n" +
			"new exception's ID, like EXC-12, is printed. You will be prompted to confirm unless --yes\n" +
			"is passed.",
		Example: "  pulumi policy exception approval approve <id> -m \"Looks good\"\n" +
			"  pulumi policy exception approval approve <id> --dry-run",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			args.yes = args.yes || env.SkipConfirmations.Value()
			return runPolicyExceptionApprovalApprove(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(),
				factory, posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, requestIDArgument)
	addPolicyExceptionApprovalOrgFlag(cmd, &args.org)
	cmd.Flags().StringVarP(&args.comment, "message", "m", "", "A comment to add with the approval")
	cmd.Flags().BoolVarP(&args.yes, "yes", "y", false, "Skip the confirmation prompt")
	cmd.Flags().BoolVar(&args.dryRun, "dry-run", false, "Show the request without approving it")
	return cmd
}

func runPolicyExceptionApprovalApprove(
	ctx context.Context, stdout, stderr io.Writer,
	factory policyExceptionClientFactory, id string, args policyExceptionApprovalDecisionArgs,
) error {
	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}
	r, err := getPolicyExceptionRequest(ctx, c, org, id)
	if err != nil {
		return err
	}
	headline := fmt.Sprintf("Policy exception request %s will create a policy exception for Policy Pack %s.",
		id, r.Exception.PolicyPack)
	if ok, err := confirmPolicyExceptionDecision(stderr, r, "approve", headline, args); !ok {
		return err
	}

	if err := c.changeRequests.ApproveChangeRequest(ctx, org, id, r.LatestRevisionNumber, args.comment); err != nil {
		return fmt.Errorf("approving policy exception request: %w", err)
	}
	fmt.Fprintf(stderr, "Approved policy exception request %s\n", id)
	// The request is applied, creating the exception, as soon as it has the approvals it needs. The approval went
	// through, so failing to read the result is only a warning.
	if r, err = getPolicyExceptionRequest(ctx, c, org, id); err != nil {
		warn(stderr, fmt.Sprintf("couldn't read policy exception request %s after approving it: %v", id, err))
		return nil
	}
	switch {
	case r.ExceptionID != "":
		fmt.Fprintln(stdout, r.ExceptionID)
		fmt.Fprintf(stderr, "Created policy exception %s\n", r.ExceptionID)
	case r.Status == changeRequestReady:
		fmt.Fprintln(stderr, "The request is pending to be applied.")
	}
	return nil
}
