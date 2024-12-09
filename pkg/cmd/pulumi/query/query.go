// Copyright 2016-2024, Pulumi Corporation.
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

package query

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// intentionally disabling here for cleaner err declaration/assignment.
//
//nolint:vetshadow
func NewQueryCmd() *cobra.Command {
	var stackName string

	cmd := &cobra.Command{
		Use:   "query",
		Short: "Run query program against cloud resources",
		Long: "[EXPERIMENTAL] Run query program against cloud resources.\n" +
			"\n" +
			"This command loads a Pulumi query program and executes it. In \"query mode\", Pulumi provides various\n" +
			"useful data sources for querying, such as the resource outputs for a stack. Query mode also disallows\n" +
			"all resource operations, so users cannot declare resource definitions as they would in normal Pulumi\n" +
			"programs.\n" +
			"\n" +
			"The program to run is loaded from the project in the current directory by default. Use the `-C` or\n" +
			"`--cwd` flag to use a different directory.",
		Args:   cmdutil.NoArgs,
		Hidden: !env.Experimental.Value() && !env.DebugCommands.Value(),
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			interactive := cmdutil.Interactive()

			cmdutil.Diag().Warningf(diag.RawMessage("" /*urn*/, `
================================================================================
query was an experimental command that we have opted to discontinue. It is due
to be removed in a future release before the end of this year (2024).
If you have any feedback or concerns, please let us know by commenting on the
issue at https://github.com/pulumi/pulumi/issues/16964.
================================================================================`))

			opts := backend.UpdateOptions{}
			opts.Display = display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: interactive,
				Type:          display.DisplayQuery,
			}

			// Try to read the current project
			ws := pkgWorkspace.Instance
			project, root, err := ws.ReadProject()
			if err != nil {
				return err
			}

			b, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, project, opts.Display)
			if err != nil {
				return err
			}

			opts.Engine = engine.UpdateOptions{
				Experimental: env.Experimental.Value(),
			}

			err = b.Query(ctx, backend.QueryOperation{
				Proj:            project,
				Root:            root,
				Opts:            opts,
				Scopes:          backend.CancellationScopes,
				SecretsProvider: stack.DefaultSecretsProvider,
			})
			switch {
			case err == context.Canceled:
				return nil
			case err != nil:
				return err
			default:
				return nil
			}
		}),
	}

	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}
