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

// AI Generated - needs human review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// policyIssueGetClient is the narrow subset of cloud-API operations the get
// command needs. Defined here so tests can stub a thin interface instead of
// the full HTTP client surface.
type policyIssueGetClient interface {
	GetPolicyIssue(
		ctx context.Context, orgName, issueID string,
	) (apitype.PolicyIssue, error)
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
			"enforcement level, severity, and the human-readable message produced\n" +
			"by the policy.\n" +
			"\n" +
			"Default output is a human-readable summary; pass --output=json for the\n" +
			"full response as JSON.",
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

type policyIssueGetRenderFunc func(w io.Writer, resp apitype.PolicyIssue) error

func renderPolicyIssueGetText(w io.Writer, issue apitype.PolicyIssue) error {
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
	if stack != "" {
		fmt.Fprintf(w, "%-20s %s\n", "Stack:", stack)
	}
	if issue.ResourceURN != "" {
		fmt.Fprintf(w, "%-20s %s\n", "Resource URN:", issue.ResourceURN)
	}
	if issue.ResourceType != "" {
		fmt.Fprintf(w, "%-20s %s\n", "Resource type:", issue.ResourceType)
	}
	if issue.ObservedAt != "" {
		fmt.Fprintf(w, "%-20s %s\n", "Observed at:", issue.ObservedAt)
	}
	if issue.Message != "" {
		fmt.Fprintf(w, "%-20s %s\n", "Message:", issue.Message)
	}
	return nil
}

// policyIssueGetJSON is the JSON envelope emitted by
// `pulumi policy issue get --output=json`.
type policyIssueGetJSON struct {
	ID            string                 `json:"id"`
	PolicyName    string                 `json:"policyName"`
	PolicyPack    string                 `json:"policyPack"`
	PolicyPackTag string                 `json:"policyPackTag,omitempty"`
	Level         string                 `json:"level"`
	Severity      apitype.PolicySeverity `json:"severity,omitempty"`
	Status        string                 `json:"status,omitempty"`
	ResourceURN   string                 `json:"resourceURN,omitempty"`
	ResourceType  string                 `json:"resourceType,omitempty"`
	EntityProject string                 `json:"entityProject,omitempty"`
	EntityID      string                 `json:"entityId,omitempty"`
	Message       string                 `json:"message,omitempty"`
	ObservedAt    string                 `json:"observedAt,omitempty"`
}

func toPolicyIssueGetJSON(issue apitype.PolicyIssue) policyIssueGetJSON {
	return policyIssueGetJSON{
		ID:            issue.ID,
		PolicyName:    issue.PolicyName,
		PolicyPack:    issue.PolicyPack,
		PolicyPackTag: issue.PolicyPackTag,
		Level:         issue.Level,
		Severity:      issue.Severity,
		Status:        issue.Status,
		ResourceURN:   issue.ResourceURN,
		ResourceType:  issue.ResourceType,
		EntityProject: issue.EntityProject,
		EntityID:      issue.EntityID,
		Message:       issue.Message,
		ObservedAt:    issue.ObservedAt,
	}
}

func renderPolicyIssueGetJSON(w io.Writer, issue apitype.PolicyIssue) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(toPolicyIssueGetJSON(issue))
}
