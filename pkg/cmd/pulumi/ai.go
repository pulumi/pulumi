// Copyright 2016-2023, Pulumi Corporation.
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

package main

import (
	"context"
	"io"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

type aiCmd struct {
	Stdout io.Writer // defaults to os.Stdout

	// currentBackend is a reference to the top-level currentBackend function.
	// This is used to override the default implementation for testing purposes.
	currentBackend func(
		context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
	) (backend.Backend, error)
}

func (cmd *aiCmd) Run(ctx context.Context, args []string) error {
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}

	if cmd.currentBackend == nil {
		cmd.currentBackend = cmdBackend.CurrentBackend
	}
	return nil
}

func newAICommand() *cobra.Command {
	var aiCommand aiCmd
	cmd := &cobra.Command{
		Use:    "ai",
		Short:  "Basic Pulumi AI CLI commands.",
		Long:   "Contains the current set of supported CLI functionality for the Pulumi AI service.",
		Hidden: !hasExperimentalCommands(),
		Args:   cmdutil.NoArgs,
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if len(args) == 0 {
				return cmd.Help()
			}
			return aiCommand.Run(ctx, args)
		},
		),
	}
	cmd.AddCommand(newAIWebCommand())
	return cmd
}
