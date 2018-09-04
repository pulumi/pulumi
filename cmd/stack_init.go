// Copyright 2016-2018, Pulumi Corporation.
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

package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newStackInitCmd() *cobra.Command {
	var ppc string
	cmd := &cobra.Command{
		Use:   "init <stack-name>",
		Args:  cmdutil.MaximumNArgs(1),
		Short: "Create an empty stack with the given name, ready for updates",
		Long: "Create an empty stack with the given name, ready for updates\n" +
			"\n" +
			"This command creates an empty stack with the given name.  It has no resources,\n" +
			"but afterwards it can become the target of a deployment using the `update` command.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			opts := backend.DisplayOptions{
				Color: cmdutil.GetGlobalColorization(),
			}

			b, err := currentBackend(opts)
			if err != nil {
				return err
			}

			var createOpts interface{}
			if _, ok := b.(httpstate.Backend); ok {
				createOpts = httpstate.CreateStackOptions{
					CloudName: ppc,
				}
			}

			var stackName string
			if len(args) > 0 {
				stackName = args[0]
			} else if cmdutil.Interactive() {
				name, nameErr := cmdutil.ReadConsole("Enter a stack name")
				if nameErr != nil {
					return nameErr
				}
				stackName = name
			}

			if stackName == "" {
				return errors.New("missing stack name")
			}

			stackRef, err := b.ParseStackReference(stackName)
			if err != nil {
				return err
			}

			_, err = createStack(b, stackRef, createOpts, true /*setCurrent*/)
			return err
		}),
	}
	cmd.PersistentFlags().StringVarP(
		&ppc, "ppc", "p", "", "An optional Pulumi Private Cloud (PPC) name to initialize this stack in")
	return cmd
}
