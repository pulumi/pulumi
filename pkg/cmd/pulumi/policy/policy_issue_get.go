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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// policyIssueGetClient is the narrow subset of cloud-API operations the get
// command needs. Defined here so tests can stub a thin interface instead of
// the full HTTP client surface.
type policyIssueGetClient interface {
	GetPolicyIssue(
		ctx context.Context, orgName, issueID string,
	) (cloudapitype.GetPolicyIssueResponse, error)
}

// policyIssueGetClientFactory resolves a cloud client and the organization
// the issue lives in. orgFlag carries the raw value of `--org` (empty means
// "use the default org").
type policyIssueGetClientFactory func(
	ctx context.Context, orgFlag string,
) (policyIssueGetClient, string, error)

// policyIssueGetArgs collects the flag values for the get command, in one
// struct so Run can be driven directly from tests.
type policyIssueGetArgs struct {
	org          string
	outputFormat outputflag.OutputFlag[policyIssueGetRenderFunc]
}

// defaultPolicyIssueGetOutputFormat wires the OutputFlag to the per-format
// renderers so `--output` selects between them.
func defaultPolicyIssueGetOutputFormat() outputflag.OutputFlag[policyIssueGetRenderFunc] {
	return outputflag.OutputFlag[policyIssueGetRenderFunc]{
		RenderForTerminal: renderPolicyIssueGetText,
		RenderJSON:        renderPolicyIssueGetJSON,
	}
}

// newPolicyIssueGetCmd builds `pulumi policy issue get` with the production
// client factory. The factory is overridable via newPolicyIssueGetCmdWith for
// tests.
func newPolicyIssueGetCmd() *cobra.Command {
	return newPolicyIssueGetCmdWith(defaultPolicyIssueGetClientFactory)
}

func newPolicyIssueGetCmdWith(factory policyIssueGetClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "policyIssueGetClientFactory must not be nil")
	var args policyIssueGetArgs
	args.outputFormat = defaultPolicyIssueGetOutputFormat()

	cmd := &cobra.Command{
		Use:   "get <issue-id>",
		Short: "[EXPERIMENTAL] Get the details of a specific policy issue",
		Long: "[EXPERIMENTAL] Get the details of a specific policy issue.\n" +
			"\n" +
			"Returns the details of a single policy issue, including the violating\n" +
			"resource, the Policy Pack and policy that flagged the violation, the\n" +
			"enforcement level, severity, kind, priority, assignee, policy group,\n" +
			"and the message produced by the policy. When available, also shows\n" +
			"the policy description, remediation steps, framework, and URL.\n" +
			"\n" +
			"Default output is a human-readable summary; pass --output=json for the\n" +
			"full response as JSON.",
		Example: "  # Show a policy issue\n" +
			"  pulumi policy issue get <issue-id>\n\n" +
			"  # Show a policy issue as JSON\n" +
			"  pulumi policy issue get <issue-id> --output json",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			return runPolicyIssueGet(cmd.Context(), cmd.OutOrStdout(), factory, posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "issue-id"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&args.org, "org", "", "The organization that owns the issue")
	outputflag.VarP(cmd.Flags(), &args.outputFormat)

	return cmd
}

// defaultPolicyIssueGetClientFactory is the production wiring: resolve the
// cloud backend, pick the effective organization, and hand back the
// underlying *client.Client.
func defaultPolicyIssueGetClientFactory(
	ctx context.Context, orgFlag string,
) (policyIssueGetClient, string, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	be, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, nil, opts)
	if err != nil {
		return nil, "", err
	}
	cloudBackend, ok := be.(httpstate.Backend)
	if !ok {
		return nil, "", errors.New(
			"getting a policy issue requires the Pulumi Cloud backend; run `pulumi login`")
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

// runPolicyIssueGet is the cobra-decoupled command body so tests can drive
// it directly without spinning up the flag parser.
func runPolicyIssueGet(
	ctx context.Context, w io.Writer,
	factory policyIssueGetClientFactory, issueID string, args policyIssueGetArgs,
) error {
	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	resp, err := c.GetPolicyIssue(ctx, org, issueID)
	if err != nil {
		return fmt.Errorf("getting policy issue: %w", err)
	}

	return args.outputFormat.Get()(w, resp)
}

type policyIssueGetRenderFunc func(w io.Writer, resp cloudapitype.GetPolicyIssueResponse) error

func renderPolicyIssueGetText(w io.Writer, resp cloudapitype.GetPolicyIssueResponse) error {
	issue := resp.PolicyIssue
	pack := issue.PolicyPack
	if issue.PolicyPackTag != "" {
		pack = fmt.Sprintf("%s@%s", issue.PolicyPack, issue.PolicyPackTag)
	}
	stack := issue.EntityID
	if issue.EntityProject != "" && issue.EntityID != "" {
		stack = fmt.Sprintf("%s/%s", issue.EntityProject, issue.EntityID)
	}

	fmt.Fprintf(w, "%-20s %s\n", "ID:", issue.ID)
	fmt.Fprintf(w, "%-20s %s\n", "Policy pack:", pack)
	fmt.Fprintf(w, "%-20s %s\n", "Policy:", issue.PolicyName)
	fmt.Fprintf(w, "%-20s %s\n", "Enforcement level:", issue.Level)
	if issue.Severity != "" {
		fmt.Fprintf(w, "%-20s %s\n", "Severity:", string(issue.Severity))
	}
	if issue.Status != "" {
		fmt.Fprintf(w, "%-20s %s\n", "Status:", issue.Status)
	}
	if issue.Priority != "" {
		fmt.Fprintf(w, "%-20s %s\n", "Priority:", issue.Priority)
	}
	if assignee := policyIssueAssignee(issue); assignee != "" {
		fmt.Fprintf(w, "%-20s %s\n", "Assignee:", assignee)
	}
	if group := policyIssueGroupName(issue); group != "" {
		if groupType := policyIssueGroupType(issue); groupType != "" {
			group = fmt.Sprintf("%s (%s)", group, groupType)
		}
		fmt.Fprintf(w, "%-20s %s\n", "Policy group:", group)
	}
	if issue.Kind != "" {
		fmt.Fprintf(w, "%-20s %s\n", "Kind:", issue.Kind)
	}
	if stack != "" {
		fmt.Fprintf(w, "%-20s %s\n", "Stack:", stack)
	}
	if issue.ResourceURN != "" {
		fmt.Fprintf(w, "%-20s %s\n", "Resource URN:", issue.ResourceURN)
	}
	if issue.ResourceType != "" {
		fmt.Fprintf(w, "%-20s %s\n", "Resource type:", issue.ResourceType)
	}
	if observedAt := formatPolicyIssueTime(issue.ObservedAt); observedAt != "" {
		fmt.Fprintf(w, "%-20s %s\n", "Observed at:", observedAt)
	}
	if issue.Message != "" {
		fmt.Fprintf(w, "%-20s %s\n", "Message:", issue.Message)
	}
	if policy := resp.Policy; policy != nil {
		if policy.Description != "" {
			printPolicyIssueMultiline(w, "Description:", policy.Description)
		}
		if policy.RemediationSteps != "" {
			printPolicyIssueMultiline(w, "Remediation:", policy.RemediationSteps)
		}
		if framework := formatPolicyFramework(policy.Framework); framework != "" {
			fmt.Fprintf(w, "%-20s %s\n", "Framework:", framework)
		}
		if policy.URL != "" {
			fmt.Fprintf(w, "%-20s %s\n", "More info:", policy.URL)
		}
	}
	return nil
}

func formatPolicyIssueTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339Nano)
}

func printPolicyIssueMultiline(w io.Writer, label, value string) {
	value = strings.ReplaceAll(value, "\n", "\n"+strings.Repeat(" ", 21))
	fmt.Fprintf(w, "%-20s %s\n", label, value)
}

func policyIssueAssignee(issue cloudapitype.PolicyIssue) string {
	if issue.AssignedTo == nil {
		return ""
	}
	return issue.AssignedTo.GitHubLogin
}

func policyIssueGroupName(issue cloudapitype.PolicyIssue) string {
	if issue.PolicyGroupName == nil {
		return ""
	}
	return *issue.PolicyGroupName
}

func policyIssueGroupType(issue cloudapitype.PolicyIssue) string {
	if issue.PolicyGroupType == nil {
		return ""
	}
	return string(*issue.PolicyGroupType)
}

func formatPolicyFramework(framework *cloudapitype.AppPolicyComplianceFramework) string {
	if framework == nil {
		return ""
	}
	text := strings.TrimSpace(framework.Name + " " + framework.Version)
	switch {
	case framework.Reference == "":
		return text
	case text == "":
		return framework.Reference
	default:
		return text + " — " + framework.Reference
	}
}

func renderPolicyIssueGetJSON(w io.Writer, resp cloudapitype.GetPolicyIssueResponse) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}
