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

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

func newPolicyExceptionApprovalUnapproveCmd(factory policyExceptionClientFactory) *cobra.Command {
	var org string
	cmd := &cobra.Command{
		Use:   "unapprove",
		Short: "[EXPERIMENTAL] Remove an approval from a request for a policy exception",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			c, org, err := factory(cmd.Context(), org)
			if err != nil {
				return err
			}
			id := posArgs[0]
			if _, err := getPolicyExceptionRequest(cmd.Context(), c, org, id); err != nil {
				return err
			}
			if err := c.changeRequests.UnapproveChangeRequest(cmd.Context(), org, id); err != nil {
				return fmt.Errorf("removing approval: %w", err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Removed approval from policy exception request %s\n", id)
			return nil
		},
	}
	constrictor.AttachArguments(cmd, requestIDArgument)
	addPolicyExceptionApprovalOrgFlag(cmd, &org)
	return cmd
}
