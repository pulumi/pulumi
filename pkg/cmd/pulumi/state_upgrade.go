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
	"fmt"
	"io"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/filestate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	"github.com/spf13/cobra"
)

func newStateUpgradeCommand() *cobra.Command {
	var sucmd stateUpgradeCmd
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Migrates the current backend to the latest supported version",
		Long: `Migrates the current backend to the latest supported version

This only has an effect on self-managed backends.
`,
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			if err := sucmd.Run(commandContext()); err != nil {
				return result.FromError(err)
			}
			return nil
		}),
	}
	return cmd
}

// stateUpgradeCmd implements the 'pulumi state upgrade' command.
type stateUpgradeCmd struct {
	Stdin  io.Reader // defaults to os.Stdin
	Stdout io.Writer // defaults to os.Stdout

	// Used to mock out the currentBackend function for testing.
	// Defaults to currentBackend function.
	currentBackend func(context.Context, *workspace.Project, display.Options) (backend.Backend, error)
}

func (cmd *stateUpgradeCmd) Run(ctx context.Context) error {
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}
	if cmd.Stdin == nil {
		cmd.Stdin = os.Stdin
	}

	if cmd.currentBackend == nil {
		cmd.currentBackend = currentBackend
	}
	currentBackend := cmd.currentBackend // shadow top-level currentBackend

	dopts := display.Options{
		Color:  cmdutil.GetGlobalColorization(),
		Stdin:  cmd.Stdin,
		Stdout: cmd.Stdout,
	}

	b, err := currentBackend(ctx, nil, dopts)
	if err != nil {
		return err
	}

	lb, ok := b.(filestate.Backend)
	if !ok {
		// Only the file state backend supports upgrades,
		// but we don't want to error out here.
		// Report the no-op.
		fmt.Fprintln(cmd.Stdout, "Nothing to do")
		return nil
	}

	prompt := "This will upgrade the current backend to the latest supported version.\n" +
		"Older versions of Pulumi will not be able to read the new format.\n" +
		"Are you sure you want to proceed?"
	if !confirmPrompt(prompt, "yes", dopts) {
		fmt.Fprintln(cmd.Stdout, "Upgrade cancelled")
		return nil
	}

	return lb.Upgrade(ctx)
}
