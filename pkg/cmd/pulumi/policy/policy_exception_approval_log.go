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
	"strings"
	"time"

	"github.com/spf13/cobra"

	esc_client "github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
)

type policyExceptionApprovalLogRenderFunc func(w io.Writer, events []esc_client.ChangeRequestEvent) error

func newPolicyExceptionApprovalLogCmd(factory policyExceptionClientFactory) *cobra.Command {
	var org string
	outputFormat := outputflag.OutputFlag[policyExceptionApprovalLogRenderFunc]{
		RenderForTerminal: func(w io.Writer, events []esc_client.ChangeRequestEvent) error {
			for _, e := range events {
				fprintChangeRequestEvent(w, e)
			}
			return nil
		},
		RenderJSON: func(w io.Writer, events []esc_client.ChangeRequestEvent) error {
			return ui.FprintJSON(w, struct {
				Events []esc_client.ChangeRequestEvent `json:"events"`
			}{events})
		},
	}
	cmd := &cobra.Command{
		Use:   "log",
		Short: "[EXPERIMENTAL] Show the history of a request for a policy exception",
		Long: "[EXPERIMENTAL] Show the history of a request for a policy exception: its approvals,\n" +
			"comments and status changes, oldest first.",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			ctx := cmd.Context()
			c, org, err := factory(ctx, org)
			if err != nil {
				return err
			}
			id := posArgs[0]
			if _, err := getPolicyExceptionRequest(ctx, c, org, id); err != nil {
				return err
			}
			events := []esc_client.ChangeRequestEvent{}
			token := ""
			for {
				page, next, err := c.changeRequests.ListChangeRequestEvents(ctx, org, id, token)
				if err != nil {
					return fmt.Errorf("listing policy exception request events: %w", err)
				}
				for _, e := range page {
					if e.ReplacedBy == "" {
						events = append(events, e)
					}
				}
				if next == "" || next == token {
					break
				}
				token = next
			}
			return outputFormat.Get()(cmd.OutOrStdout(), events)
		},
	}
	constrictor.AttachArguments(cmd, requestIDArgument)
	addPolicyExceptionApprovalOrgFlag(cmd, &org)
	outputflag.VarP(cmd.Flags(), &outputFormat)
	return cmd
}

// fprintChangeRequestEvent writes one line per event, and its comment below it. It follows `pulumi env approval log`.
func fprintChangeRequestEvent(w io.Writer, e esc_client.ChangeRequestEvent) {
	var what string
	switch e.EventType {
	case "approved_by_user":
		what = fmt.Sprintf("approved revision %d", e.RevisionNumber)
	case "unapproved_by_user":
		what = "removed their approval"
	case "commented":
		what = "commented"
	case "description_updated":
		what = "updated the description"
	case "revision_added":
		what = fmt.Sprintf("added revision %d", e.RevisionNumber)
	case "status_changed":
		switch {
		case e.PreviousStatus == "" && e.NewStatus == changeRequestDraft:
			what = "requested the exception"
		case e.Reason == "submitted_for_approval":
			what = "submitted the request for approval"
		case e.Reason == "manually_closed":
			what = "closed the request"
		case e.Reason == "applied successfully":
			what = "created the exception"
		case e.Reason == "gate_evaluation" && e.NewStatus == changeRequestReady:
			what = "marked the request approved (approval rules met)"
		case e.Reason == "gate_evaluation" && e.NewStatus == changeRequestPending:
			what = "marked the request pending (approval rules no longer met)"
		default:
			what = fmt.Sprintf("changed the status from %s to %s", e.PreviousStatus, e.NewStatus)
			if e.Reason != "" {
				what += " (" + e.Reason + ")"
			}
		}
	default:
		what = e.EventType
	}
	fmt.Fprintf(w, "%s  %s %s\n", e.CreatedAt.UTC().Format(time.RFC3339), e.CreatedBy.GithubLogin, what)
	if e.Comment != "" {
		fmt.Fprintf(w, "    %s\n", strings.ReplaceAll(e.Comment, "\n", "\n    "))
	}
}
