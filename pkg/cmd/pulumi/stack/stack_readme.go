// Copyright 2018-2025, Pulumi Corporation.
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
	"os"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func newStackReadmeCmd() *cobra.Command {
	var stack string
	var template string

	cmd := &cobra.Command{
		Use:     "readme",
		Aliases: []string{},
		Short:   "Generate a Stack README file using Copilot.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			opts := backend.GenerateStackReadmeOptions{
				Options: display.Options{
					Color: cmdutil.GetGlobalColorization(),
				},
			}

			s, err := RequireStack(
				ctx,
				cmdutil.Diag(),
				ws,
				cmdBackend.DefaultLoginManager,
				stack,
				LoadOnly,
				opts.Options,
			)
			if err != nil {
				return err
			}
			b := s.Backend()

			if template != "" {
				data, err := os.ReadFile(template)
				if err != nil {
					return err
				}
				opts.Template = string(data)
			}

			content, err := b.GenerateStackReadme(ctx, s, opts)

			stdout := opts.Stdout
			if stdout == nil {
				stdout = os.Stdout
			}
			if err != nil {
				_, err = stdout.Write([]byte(
					opts.Color.Colorize(
						"An error occurred while generating the Stack README:\n" +
							colors.BrightRed + err.Error() + colors.Reset + "\n\n")))
				contract.IgnoreError(err)
				return nil
			}
			_, err = stdout.Write([]byte(content + "\n"))
			contract.IgnoreError(err)
			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Choose a stack other than the currently selected one")
	cmd.PersistentFlags().StringVarP(
		&template, "template-file", "f", "",
		"The template file to use for the README.")
	return cmd
}
