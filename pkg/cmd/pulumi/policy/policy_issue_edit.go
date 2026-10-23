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
	"slices"
	"strings"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// policyIssueEditStatuses are the statuses a user may set; the API also
// defines by_design and fixed, which are assigned by the service.
var policyIssueEditStatuses = []cloudapitype.PolicyIssueStatus{
	cloudapitype.PolicyIssueStatusOpen,
	cloudapitype.PolicyIssueStatusInProgress,
	cloudapitype.PolicyIssueStatusIgnored,
}

type policyIssueEditClient interface {
	UpdatePolicyIssue(
		ctx context.Context, orgName, issueID string, req cloudapitype.UpdatePolicyIssueRequest,
	) (cloudapitype.GetPolicyIssueResponse, error)
}

type policyIssueEditClientFactory func(
	ctx context.Context, orgFlag string,
) (policyIssueEditClient, string, error)

type policyIssueEditArgs struct {
	org          string
	status       string
	priority     string
	assignee     string
	unassign     bool
	outputFormat outputflag.OutputFlag[policyIssueGetRenderFunc]

	// changed records which of policyIssueEditFlagNames were passed, so an
	// explicitly empty value is rejected rather than ignored.
	changed map[string]bool
}

var policyIssueEditFlagNames = []string{"status", "priority", "assignee"}

func newPolicyIssueEditCmd() *cobra.Command {
	return newPolicyIssueEditCmdWith(defaultPolicyIssueEditClientFactory)
}

func newPolicyIssueEditCmdWith(factory policyIssueEditClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "policyIssueEditClientFactory must not be nil")
	var args policyIssueEditArgs
	args.outputFormat = defaultPolicyIssueGetOutputFormat()

	cmd := &cobra.Command{
		Use:     "edit <issue-id>",
		Aliases: []string{"update"},
		Short:   "[EXPERIMENTAL] Edit the status, priority or assignee of a policy issue",
		Long: "[EXPERIMENTAL] Edit the status, priority or assignee of a policy issue.\n" +
			"\n" +
			"At least one of --status, --priority, --assignee or --unassign is required. Invalid\n" +
			"flag values or a missing change exit with code 2.\n" +
			"\n" +
			"Allowed values:\n" +
			"  --status    open, in_progress, ignored\n" +
			"  --priority  p0, p1, p2, p3, p4\n" +
			"  --assignee  the Pulumi login of an organization member\n" +
			"\n" +
			"Prints the updated issue in the same format as `pulumi policy issue get`.",
		Example: "  # Mark a policy issue as in progress\n" +
			"  pulumi policy issue edit <issue-id> --status in_progress\n\n" +
			"  # Set the priority and assign the issue\n" +
			"  pulumi policy issue edit <issue-id> --priority p1 --assignee <login>\n\n" +
			"  # Clear the assignee of a policy issue\n" +
			"  pulumi policy issue edit <issue-id> --unassign",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			args.changed = changedFlagSet(cmd, policyIssueEditFlagNames)
			return runPolicyIssueEdit(cmd.Context(), cmd.OutOrStdout(), factory, posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "issue-id"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&args.org, "org", "", "The organization that owns the issue")
	cmd.Flags().StringVar(&args.status, "status", "",
		"Set the status ("+joinPolicyIssueValues(policyIssueEditStatuses)+")")
	cmd.Flags().StringVar(&args.priority, "priority", "",
		"Set the priority ("+joinPolicyIssueValues(cloudapitype.PolicyIssuePriority("").AllValues())+")")
	cmd.Flags().StringVar(&args.assignee, "assignee", "",
		"Assign to the Pulumi login of an organization member")
	cmd.Flags().BoolVar(&args.unassign, "unassign", false, "Clear the assignee")
	outputflag.VarP(cmd.Flags(), &args.outputFormat)

	return cmd
}

func defaultPolicyIssueEditClientFactory(
	ctx context.Context, orgFlag string,
) (policyIssueEditClient, string, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	be, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, nil, opts)
	if err != nil {
		return nil, "", err
	}
	cloudBackend, ok := be.(httpstate.Backend)
	if !ok {
		return nil, "", errors.New(
			"updating a policy issue requires the Pulumi Cloud backend; run `pulumi login`")
	}

	userName, orgs, _, err := cloudBackend.CurrentUser()
	if err != nil {
		return nil, "", err
	}

	org := orgFlag
	if org == "" {
		defaultOrg, err := cloudBackend.GetDefaultOrg(ctx)
		if err != nil {
			return nil, "", err
		}
		org = defaultOrg
	}
	if org == "" {
		org = userName
	}

	if !slices.Contains(orgs, org) && org != userName {
		return nil, "", fmt.Errorf("user %s is not a member of organization %s", userName, org)
	}

	return cloudBackend.Client(), org, nil
}

func joinPolicyIssueValues[T ~string](values []T) string {
	names := make([]string, len(values))
	for i, v := range values {
		names[i] = string(v)
	}
	return strings.Join(names, ", ")
}

func validatePolicyIssueEditArgs(args policyIssueEditArgs) error {
	if args.changed["status"] &&
		!slices.Contains(policyIssueEditStatuses, cloudapitype.PolicyIssueStatus(args.status)) {
		return cmdCmd.ConfigurationError{Message: fmt.Sprintf("invalid status %q: must be one of %s",
			args.status, joinPolicyIssueValues(policyIssueEditStatuses))}
	}
	if priority := cloudapitype.PolicyIssuePriority(args.priority); args.changed["priority"] && !priority.IsValid() {
		return cmdCmd.ConfigurationError{Message: fmt.Sprintf("invalid priority %q: must be one of %s",
			args.priority, joinPolicyIssueValues(priority.AllValues()))}
	}
	if args.changed["assignee"] && args.unassign {
		return cmdCmd.ConfigurationError{Message: "--assignee and --unassign cannot be used together"}
	}
	if args.changed["assignee"] && args.assignee == "" {
		return cmdCmd.ConfigurationError{Message: "--assignee must not be empty; use --unassign to clear the assignee"}
	}
	if !args.changed["status"] && !args.changed["priority"] && !args.changed["assignee"] && !args.unassign {
		return cmdCmd.ConfigurationError{
			Message: "nothing to update: specify at least one of --status, --priority, --assignee or --unassign",
		}
	}
	return nil
}

func buildUpdatePolicyIssueRequest(args policyIssueEditArgs) cloudapitype.UpdatePolicyIssueRequest {
	var req cloudapitype.UpdatePolicyIssueRequest
	if args.changed["status"] {
		req.Status = new(cloudapitype.PolicyIssueStatus(args.status))
	}
	if args.changed["priority"] {
		req.Priority = new(cloudapitype.PolicyIssuePriority(args.priority))
	}

	switch {
	case args.unassign:
		req.AssignedTo = new("")
	case args.changed["assignee"]:
		req.AssignedTo = new(args.assignee)
	}
	return req
}

func runPolicyIssueEdit(
	ctx context.Context, w io.Writer,
	factory policyIssueEditClientFactory, issueID string, args policyIssueEditArgs,
) error {
	if err := validatePolicyIssueEditArgs(args); err != nil {
		return err
	}

	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	resp, err := c.UpdatePolicyIssue(ctx, org, issueID, buildUpdatePolicyIssueRequest(args))
	if err != nil {
		return err
	}

	return args.outputFormat.Get()(w, resp)
}
