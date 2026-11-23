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
	"strings"

	"github.com/spf13/cobra"

	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

func newPolicyExceptionApprovalCommentCmd(factory policyExceptionClientFactory) *cobra.Command {
	var org, comment string
	cmd := &cobra.Command{
		Use:     "comment",
		Short:   "[EXPERIMENTAL] Comment on a request for a policy exception",
		Example: "  pulumi policy exception approval comment <id> -m \"Which CDN needs this?\"",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			if strings.TrimSpace(comment) == "" {
				return cmdCmd.ConfigurationError{Message: "a comment is required: pass -m <comment>"}
			}
			c, org, err := factory(cmd.Context(), org)
			if err != nil {
				return err
			}
			id := posArgs[0]
			if _, err := getPolicyExceptionRequest(cmd.Context(), c, org, id); err != nil {
				return err
			}
			if err := c.changeRequests.AddChangeRequestComment(cmd.Context(), org, id, comment); err != nil {
				return fmt.Errorf("adding comment: %w", err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Added comment to policy exception request %s\n", id)
			return nil
		},
	}
	constrictor.AttachArguments(cmd, requestIDArgument)
	addPolicyExceptionApprovalOrgFlag(cmd, &org)
	cmd.Flags().StringVarP(&comment, "message", "m", "", "The comment")
	return cmd
}
