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

package deployment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// deploymentGetClient is the subset of cloud-API operations the get command
// needs. Defined here so tests can stub a thin interface instead of the full
// HTTP client surface.
type deploymentGetClient interface {
	GetDeployment(
		ctx context.Context, stack client.StackIdentifier, id string,
	) (apitype.GetDeploymentResponse, error)
	GetDeploymentByVersion(
		ctx context.Context, stack client.StackIdentifier, version string,
	) (apitype.GetDeploymentResponse, error)
}

// deploymentGetClientFactory resolves a cloud client and the StackIdentifier
// to get a deployment for. stackFlag carries the raw value of `--stack`
// (empty means "use the current stack").
type deploymentGetClientFactory func(
	ctx context.Context, stackFlag string,
) (deploymentGetClient, client.StackIdentifier, error)

// deploymentGetArgs collects the flag values for the get command, in one
// struct so Run can be driven directly from tests.
type deploymentGetArgs struct {
	stack        string
	outputFormat outputflag.OutputFlag[deploymentGetRenderFunc]
}

// defaultDeploymentGetOutputFormat wires the OutputFlag to the per-format
// renderers so `--output` selects between them.
func defaultDeploymentGetOutputFormat() outputflag.OutputFlag[deploymentGetRenderFunc] {
	return outputflag.OutputFlag[deploymentGetRenderFunc]{
		RenderForTerminal: renderDeploymentGetText,
		RenderJSON:        renderDeploymentGetJSON,
	}
}

// newDeploymentGetCmd builds `pulumi deployment get` with the production
// client factory. The factory is overridable via newDeploymentGetCmdWith for
// tests.
func newDeploymentGetCmd() *cobra.Command {
	return newDeploymentGetCmdWith(defaultDeploymentGetClientFactory)
}

func newDeploymentGetCmdWith(factory deploymentGetClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "deploymentGetClientFactory must not be nil")
	var args deploymentGetArgs
	args.outputFormat = defaultDeploymentGetOutputFormat()

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "get <deployment-version>",
		Short:  "[EXPERIMENTAL] Get details for a specific deployment",
		Long: "[EXPERIMENTAL] Get details for a specific deployment.\n" +
			"\n" +
			"The deployment may be referenced by its UUID or by its per-stack version\n" +
			"number (the integer shown in the Pulumi Cloud UI, e.g. 9410 or #9410).\n" +
			"\n" +
			"Retrieves detailed information about a single Pulumi Deployments execution.\n" +
			"The response includes the deployment's current status, creation and\n" +
			"modification timestamps, version number, the user who requested the\n" +
			"deployment, the Pulumi operation type, the list of jobs (with their\n" +
			"step-level status), and any stack updates produced by the deployment.\n" +
			"\n" +
			"Default output is a human-readable summary; pass --output=json for the full\n" +
			"response as JSON.",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			return runDeploymentGet(cmd.Context(), cmd.OutOrStdout(), factory, posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "deployment-version"},
		},
		Required: 1,
	})

	cmd.Flags().StringVarP(&args.stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	outputflag.VarP(cmd.Flags(), &args.outputFormat)

	return cmd
}

// defaultDeploymentGetClientFactory is the production wiring: resolve the
// stack via RequireStack (non-prompting beyond the standard select flow),
// cast to the cloud-backend types, and hand back the underlying
// *client.Client.
func defaultDeploymentGetClientFactory(
	ctx context.Context, stackFlag string,
) (deploymentGetClient, client.StackIdentifier, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	s, err := cmdStack.RequireStack(ctx, cmdutil.Diag(), ws, cmdBackend.DefaultLoginManager,
		stackFlag, cmdStack.LoadOnly, opts)
	if err != nil {
		return nil, client.StackIdentifier{}, fmt.Errorf("resolving stack: %w", err)
	}

	cloudStack, ok := s.(httpstate.Stack)
	if !ok {
		return nil, client.StackIdentifier{},
			errors.New("getting a deployment requires the Pulumi Cloud backend; run `pulumi login`")
	}

	ref := cloudStack.Ref()
	project := ""
	if p, ok := ref.Project(); ok {
		project = string(p)
	}
	stackID := client.StackIdentifier{
		Owner:   cloudStack.OrgName(),
		Project: project,
		Stack:   ref.Name(),
	}

	be, ok := cloudStack.Backend().(httpstate.Backend)
	if !ok {
		return nil, client.StackIdentifier{},
			errors.New("getting a deployment requires the Pulumi Cloud backend; run `pulumi login`")
	}
	return be.Client(), stackID, nil
}

// runDeploymentGet is the cobra-decoupled command body so tests can drive it
// directly without spinning up the flag parser. `deploymentRef` is either a
// UUID or a per-stack version number (see deploymentVersionRef); the two
// forms hit different endpoints but return the same shape.
func runDeploymentGet(
	ctx context.Context, w io.Writer,
	factory deploymentGetClientFactory, deploymentRef string, args deploymentGetArgs,
) error {
	c, stackID, err := factory(ctx, args.stack)
	if err != nil {
		return err
	}

	var resp apitype.GetDeploymentResponse
	if version, ok := deploymentVersionRef(deploymentRef); ok {
		resp, err = c.GetDeploymentByVersion(ctx, stackID, version)
	} else {
		resp, err = c.GetDeployment(ctx, stackID, deploymentRef)
	}
	if err != nil {
		return fmt.Errorf("getting deployment: %w", err)
	}

	return args.outputFormat.Get()(w, resp)
}

type deploymentGetRenderFunc func(w io.Writer, resp apitype.GetDeploymentResponse) error

// renderDeploymentGetText prints a human-readable summary of a single
// deployment as aligned key/value pairs.
func renderDeploymentGetText(w io.Writer, resp apitype.GetDeploymentResponse) error {
	initiatedBy := resp.RequestedBy.GitHubLogin
	if initiatedBy == "" {
		initiatedBy = resp.RequestedBy.Name
	}

	fmt.Fprintf(w, "%-18s %s\n", "ID:", resp.ID)
	fmt.Fprintf(w, "%-18s %s\n", "Status:", resp.Status)
	fmt.Fprintf(w, "%-18s %s\n", "Operation:", string(resp.PulumiOperation))
	fmt.Fprintf(w, "%-18s %d\n", "Version:", resp.Version)
	if resp.ProjectName != "" {
		fmt.Fprintf(w, "%-18s %s\n", "Project:", resp.ProjectName)
	}
	if resp.StackName != "" {
		fmt.Fprintf(w, "%-18s %s\n", "Stack:", resp.StackName)
	}
	fmt.Fprintf(w, "%-18s %s\n", "Created:", resp.Created)
	fmt.Fprintf(w, "%-18s %s\n", "Modified:", resp.Modified)
	fmt.Fprintf(w, "%-18s %s\n", "Initiated by:", initiatedBy)
	if resp.Initiator != "" {
		fmt.Fprintf(w, "%-18s %s\n", "Initiator:", resp.Initiator)
	}
	if resp.AgentPool != nil {
		fmt.Fprintf(w, "%-18s %s\n", "Agent pool:", resp.AgentPool.Name)
	}
	fmt.Fprintf(w, "%-18s %d\n", "Jobs:", len(resp.Jobs))
	fmt.Fprintf(w, "%-18s %d\n", "Updates:", len(resp.Updates))
	return nil
}

// getDeploymentJSON is the JSON envelope emitted by
// `pulumi deployment get --output=json`. The shape mirrors the API response
// but normalizes nil slices to empty arrays so scripts can rely on the
// `jobs` and `updates` keys always being JSON arrays.
type getDeploymentJSON struct {
	ID              string                                   `json:"id"`
	Created         string                                   `json:"created"`
	Modified        string                                   `json:"modified"`
	Status          string                                   `json:"status"`
	Version         int64                                    `json:"version"`
	RequestedBy     apitype.UserInfo                         `json:"requestedBy"`
	ProjectName     string                                   `json:"projectName"`
	StackName       string                                   `json:"stackName"`
	Paused          bool                                     `json:"paused"`
	PulumiOperation apitype.PulumiOperation                  `json:"pulumiOperation"`
	Updates         []apitype.DeploymentNestedUpdate         `json:"updates"`
	Jobs            []apitype.DeploymentJob                  `json:"jobs"`
	Initiator       string                                   `json:"initiator"`
	AgentPool       *apitype.ListDeploymentSnapshotAgentPool `json:"agentPool,omitempty"`
	InheritSettings bool                                     `json:"inheritSettings"`
}

func toGetDeploymentJSON(resp apitype.GetDeploymentResponse) getDeploymentJSON {
	updates := resp.Updates
	if updates == nil {
		updates = []apitype.DeploymentNestedUpdate{}
	}
	jobs := resp.Jobs
	if jobs == nil {
		jobs = []apitype.DeploymentJob{}
	}
	return getDeploymentJSON{
		ID:              resp.ID,
		Created:         resp.Created,
		Modified:        resp.Modified,
		Status:          resp.Status,
		Version:         resp.Version,
		RequestedBy:     resp.RequestedBy,
		ProjectName:     resp.ProjectName,
		StackName:       resp.StackName,
		Paused:          resp.Paused,
		PulumiOperation: resp.PulumiOperation,
		Updates:         updates,
		Jobs:            jobs,
		Initiator:       resp.Initiator,
		AgentPool:       resp.AgentPool,
		InheritSettings: resp.InheritSettings,
	}
}

func renderDeploymentGetJSON(w io.Writer, resp apitype.GetDeploymentResponse) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(toGetDeploymentJSON(resp))
}
