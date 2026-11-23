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

func newPolicyExceptionApprovalCloseCmd(factory policyExceptionClientFactory) *cobra.Command {
	var args policyExceptionApprovalDecisionArgs
	cmd := &cobra.Command{
		Use:   "close",
		Short: "[EXPERIMENTAL] Close a request for a policy exception without creating it",
		Long: "[EXPERIMENTAL] Close a request for a policy exception without creating it.\n" +
			"\n" +
			"You will be prompted to confirm unless --yes is passed.",
		Example: "  pulumi policy exception approval close <id> -m \"Fix the bucket instead\"",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			args.yes = args.yes || env.SkipConfirmations.Value()
			return runPolicyExceptionApprovalClose(cmd.Context(), cmd.ErrOrStderr(), factory, posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, requestIDArgument)
	addPolicyExceptionApprovalOrgFlag(cmd, &args.org)
	cmd.Flags().StringVarP(&args.comment, "message", "m", "", "Why the request is closed")
	cmd.Flags().BoolVarP(&args.yes, "yes", "y", false, "Skip the confirmation prompt")
	return cmd
}

func runPolicyExceptionApprovalClose(
	ctx context.Context, stderr io.Writer,
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
	if r.Status == changeRequestClosed {
		warnNoop(stderr, fmt.Sprintf("Policy exception request %s is already closed.", id), args.comment)
		return nil
	}
	headline := fmt.Sprintf("Policy exception request %s will be closed without creating a policy exception.", id)
	if ok, err := confirmPolicyExceptionDecision(stderr, r, "close", headline, args); !ok {
		return err
	}
	if err := c.changeRequests.CloseChangeRequest(ctx, org, id, args.comment); err != nil {
		return fmt.Errorf("closing policy exception request: %w", err)
	}
	fmt.Fprintf(stderr, "Closed policy exception request %s\n", id)
	return nil
}
