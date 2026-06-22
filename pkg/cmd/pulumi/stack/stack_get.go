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
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// `pulumi stack get` shows the same information as `pulumi stack` (same flags and output), but only reads an existing
// stack, it does not offer to create one. This command exists for verb-noun consistency with the other `get` commands.
func newStackGetCmd() *cobra.Command {
	var stackName string
	args := stackArgs{}

	output := outputflag.OutputFlag[stackRenderFunc]{
		RenderForTerminal: runStackText,
		RenderJSON:        runStackJSON,
	}

	cmd := &cobra.Command{
		Use:   "get",
		Short: "[EXPERIMENTAL] Retrieve detailed information about a stack",
		Long: "[EXPERIMENTAL] Retrieve detailed information about a stack.\n" +
			"\n" +
			"Shows the current stack's state: the owning organization, the resources in\n" +
			"the stack, stack outputs, the last update, and (on the Pulumi Cloud backend)\n" +
			"any in-progress operation. This is the same information shown by `pulumi\n" +
			"stack`",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			s, err := RequireStack(ctx, cmdutil.Diag(), pkgWorkspace.Instance, cmdBackend.DefaultLoginManager,
				stackName, LoadOnly, display.Options{Color: cmdutil.GetGlobalColorization()}, "")
			if err != nil {
				return err
			}

			args.fullyQualifyStackNames = cmdutil.FullyQualifyStackNames
			if args.showStackName {
				writeStackName(cmd.OutOrStdout(), s, args.fullyQualifyStackNames)
				return nil
			}
			return output.Get()(ctx, s, cmd.OutOrStdout(), args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&stackName, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().BoolVarP(&args.showIDs, "show-ids", "i", false,
		"Display each resource's provider-assigned unique ID")
	cmd.Flags().BoolVarP(&args.showURNs, "show-urns", "u", false,
		"Display each resource's Pulumi-assigned globally unique URN")
	cmd.Flags().BoolVar(&args.showSecrets, "show-secrets", false,
		"Display stack outputs which are marked as secret in plaintext")
	cmd.Flags().BoolVar(&args.showStackName, "show-name", false,
		"Display only the stack name")
	outputflag.VarP(cmd.Flags(), &output)

	return cmd
}
