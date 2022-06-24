// Copyright 2016-2019, Pulumi Corporation.
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

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

// intentionally disabling here for cleaner err declaration/assignment.
// nolint: vetshadow
func newQueryCmd() *cobra.Command {
	var stack string

	var cmd = &cobra.Command{
		Use:   "query",
		Short: "Run query program against cloud resources",
		Long: "Run query program against cloud resources.\n" +
			"\n" +
			"This command loads a Pulumi query program and executes it. In \"query mode\", Pulumi provides various\n" +
			"useful data sources for querying, such as the resource outputs for a stack. Query mode also disallows\n" +
			"all resource operations, so users cannot declare resource definitions as they would in normal Pulumi\n" +
			"programs.\n" +
			"\n" +
			"The program to run is loaded from the project in the current directory by default. Use the `-C` or\n" +
			"`--cwd` flag to use a different directory.",
		Args:   cmdutil.NoArgs,
		Hidden: !hasExperimentalCommands() && !hasDebugCommands(),
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			interactive := cmdutil.Interactive()

			opts := backend.UpdateOptions{}
			opts.Display = display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: interactive,
				Type:          display.DisplayQuery,
			}

			b, err := currentBackend(opts.Display)
			if err != nil {
				return result.FromError(err)
			}

			proj, root, err := readProject()
			if err != nil {
				return result.FromError(err)
			}

			opts.Engine = engine.UpdateOptions{}

			res := b.Query(commandContext(), backend.QueryOperation{
				Proj:   proj,
				Root:   root,
				Opts:   opts,
				Scopes: cancellationScopes,
			})
			switch {
			case res != nil && res.Error() == context.Canceled:
				return nil
			case res != nil:
				return PrintEngineResult(res)
			default:
				return nil
			}
		}),
	}

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}
