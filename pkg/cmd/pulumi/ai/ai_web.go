// Copyright 2016-2026, Pulumi Corporation.
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

package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

type aiWebCmd struct {
	ws pkgWorkspace.Context

	Stdout io.Writer // defaults to os.Stdout

	// currentBackend is a reference to the top-level currentBackend function.
	// This is used to override the default implementation for testing purposes.
	currentBackend func(
		context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
	) (backend.Backend, error)
}

func (cmd *aiWebCmd) Run(ctx context.Context, args []string) error {
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}

	if len(args) == 0 {
		return errors.New(
			"prompt must be provided.\n" +
				"Example: 'pulumi ai web \"Create an S3 bucket in Python\"'",
		)
	}

	prompt := args[0]

	project, _, err := cmd.ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return err
	}

	if cmd.currentBackend == nil {
		cmd.currentBackend = cmdBackend.CurrentBackend
	}

	opts := display.Options{
		Color: cmdutil.GetGlobalColorization(),
	}

	b, err := cmd.currentBackend(ctx, cmd.ws, cmdBackend.DefaultLoginManager, project, opts)
	if err != nil {
		return fmt.Errorf("failed to get current backend: %w", err)
	}

	// Check if it's a cloud backend
	cloudBackend, isCloud := b.(httpstate.Backend)
	if !isCloud {
		return errors.New("Neo tasks are only available with the Pulumi Cloud backend. " +
			"Please run 'pulumi login' to connect to Pulumi Cloud.")
	}

	// Try to get the current stack for context
	var stackRef backend.StackReference
	stack, err := state.CurrentStack(ctx, cmd.ws, b)
	if err == nil && stack != nil {
		stackRef = stack.Ref()
	}

	neoURL, err := cloudBackend.CreateNeoTask(ctx, stackRef, prompt)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.Stdout, "\nNeo task created successfully!\n")
	fmt.Fprintf(cmd.Stdout, "View your task at:\n%s\n\n", neoURL)
	return nil
}

func newAIWebCommand(ws pkgWorkspace.Context) *cobra.Command {
	var aiwebcmd aiWebCmd
	aiwebcmd.ws = ws

	cmd := &cobra.Command{
		Use:   "web <prompt>",
		Short: "Create a Neo task with the given prompt",
		Long: `Create a Neo task with the given prompt

This command creates a new Neo task in Pulumi Cloud with your prompt and
provides you with a link to view it. Neo is Pulumi's AI assistant that
can help you with infrastructure as code tasks.

Example:
  pulumi ai web "Create an S3 bucket in Python"
  pulumi ai web "Deploy a containerized web app to AWS"
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return aiwebcmd.Run(ctx, args)
		},
	}
	return cmd
}
