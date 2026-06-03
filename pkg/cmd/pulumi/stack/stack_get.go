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
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// newStackGetCmd registers `pulumi stack get`, a thin convenience alias
// for `pulumi stack`. It behaves identically to `pulumi stack` on both
// the Pulumi Cloud and DIY backends, including the default value of
// `--output` (human-readable text). Pass `--output=json` for the
// stable, machine-readable envelope built by stack_json.go.
func newStackGetCmd() *cobra.Command {
	var (
		stackName   string
		output      string
		showSecrets bool
	)

	cmd := &cobra.Command{
		Use:   "get",
		Short: "[EXPERIMENTAL] Retrieve detailed information about a stack",
		Long: "[EXPERIMENTAL] Retrieve detailed information about a stack.\n" +
			"\n" +
			"`pulumi stack get` is a convenience alias for `pulumi stack --output=json`.\n" +
			"On the Pulumi Cloud backend it surfaces the organization, project, and\n" +
			"stack name, the current version, all associated tags, any active update\n" +
			"operation (with its kind, author, and start time), the active update\n" +
			"UUID, and (when available) the local resource snapshot. On DIY backends\n" +
			"the cloud-only fields are omitted; everything else is rendered.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := RequireStack(ctx, cmdutil.Diag(), pkgWorkspace.Instance, cmdBackend.DefaultLoginManager,
				stackName, LoadOnly, display.Options{Color: cmdutil.GetGlobalColorization()}, "")
			if err != nil {
				return err
			}
			return runStack(ctx, s, cmd.OutOrStdout(), stackArgs{
				output:      output,
				showSecrets: showSecrets,
			})
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&stackName, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().StringVar(&output, "output", "default",
		"The output format: default (human-readable) or json")
	cmd.Flags().BoolVar(&showSecrets, "show-secrets", false,
		"Display stack outputs which are marked as secret in plaintext")

	return cmd
}
