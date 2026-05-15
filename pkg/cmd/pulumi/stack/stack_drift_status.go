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

package stack

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
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// driftStatusClient is the interface the drift status command needs.
type driftStatusClient interface {
	GetDriftStatus(
		ctx context.Context, stackID client.StackIdentifier,
	) (apitype.StackDriftStatus, error)
}

type driftStatusClientFactory func(
	ctx context.Context, stackFlag string,
) (driftStatusClient, client.StackIdentifier, error)

type driftStatusRender func(
	cmd *driftStatusCmd, status apitype.StackDriftStatus,
) error

type driftStatusCmd struct {
	stack  string
	output outputflag.OutputFlag[driftStatusRender]
	w      io.Writer
}

func newStackDriftStatusCmd() *cobra.Command {
	return newStackDriftStatusCmdWith(nil)
}

func newStackDriftStatusCmdWith(factory driftStatusClientFactory) *cobra.Command {
	dscmd := &driftStatusCmd{
		output: outputflag.OutputFlag[driftStatusRender]{
			RenderForTerminal: (*driftStatusCmd).renderText,
			RenderJSON:        (*driftStatusCmd).renderJSON,
		},
	}

	cmd := &cobra.Command{
		Use:   "status",
		Short: "[EXPERIMENTAL] Show the current drift detection status for a stack",
		Long: "Show the current drift detection status for a stack.\n" +
			"\n" +
			"Shows whether drift has been detected, the ID of the latest drift\n" +
			"detection run, and whether a run is currently in progress.",
		Example: "  # Show drift status for the current stack\n" +
			"  pulumi stack drift status\n\n" +
			"  # Show drift status as JSON\n" +
			"  pulumi stack drift status --output json\n\n" +
			"  # Show drift status for a specific stack\n" +
			"  pulumi stack drift status --stack org/project/dev",
		RunE: func(cmd *cobra.Command, args []string) error {
			if factory == nil {
				factory = defaultDriftStatusClientFactory
			}
			dscmd.w = cmd.OutOrStdout()
			return dscmd.run(cmd.Context(), factory)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&dscmd.stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	outputflag.VarP(cmd.Flags(), &dscmd.output)

	return cmd
}

func defaultDriftStatusClientFactory(
	ctx context.Context, stackFlag string,
) (driftStatusClient, client.StackIdentifier, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	s, err := RequireStack(ctx, cmdutil.Diag(), ws, cmdBackend.DefaultLoginManager,
		stackFlag, LoadOnly, opts)
	if err != nil {
		return nil, client.StackIdentifier{}, fmt.Errorf("resolving stack: %w", err)
	}

	cloudStack, ok := s.(httpstate.Stack)
	if !ok {
		return nil, client.StackIdentifier{},
			errors.New("drift commands require the Pulumi Cloud backend; run `pulumi login`")
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

	be := cloudStack.Backend().(httpstate.Backend)
	return be.Client(), stackID, nil
}

func (c *driftStatusCmd) run(ctx context.Context, factory driftStatusClientFactory) error {
	cl, stackID, err := factory(ctx, c.stack)
	if err != nil {
		return err
	}

	status, err := cl.GetDriftStatus(ctx, stackID)
	if err != nil {
		return fmt.Errorf("getting drift status: %w", err)
	}

	return c.output.Get()(c, status)
}

func (c *driftStatusCmd) renderJSON(status apitype.StackDriftStatus) error {
	enc := json.NewEncoder(c.w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(status)
}

func (c *driftStatusCmd) renderText(status apitype.StackDriftStatus) error {
	driftDetected := "no"
	if status.DriftDetected {
		driftDetected = "yes"
	}
	runInProgress := "no"
	if status.RunInProgress {
		runInProgress = "yes"
	}

	fmt.Fprintf(c.w, "Drift detected:    %s\n", driftDetected)
	if status.LatestDriftRun != "" {
		fmt.Fprintf(c.w, "Latest drift run:  %s\n", status.LatestDriftRun)
	}
	fmt.Fprintf(c.w, "Run in progress:   %s\n", runInProgress)
	return nil
}
