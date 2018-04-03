// Copyright 2017-2018, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/pkg/backend/cloud"
	"github.com/pulumi/pulumi/pkg/backend/local"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func newStackInitCmd() *cobra.Command {
	var cloudURL string
	var localBackend bool
	var remoteBackend bool
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
			// If --cloud-url was passed, infer that the user wanted --remote.
			remoteBackend = remoteBackend || cloudURL != ""

			var b backend.Backend
			var opts interface{}
			if localBackend {
				if ppc != "" {
					return errors.New("cannot pass both --local and --ppc; PPCs only available in cloud mode")
				}
				if remoteBackend {
					return errors.New("cannot pass both --local with either --remote or --cloud-url")
				}
				b = local.New(cmdutil.Diag())
			} else if url := cloud.ValueOrDefaultURL(cloudURL); isLoggedIn(url) {
				c, err := cloud.New(cmdutil.Diag(), url)
				if err != nil {
					return errors.Wrap(err, "creating API client")
				}
				b = c
				opts = cloud.CreateStackOptions{CloudName: ppc}
			} else {
				// If the user is not logged in and --remote or --cloud-url was passed, fail.
				if remoteBackend {
					return errors.Errorf("you must be logged in to create stacks in the Pulumi Cloud. Run " +
						"`pulumi login` to log in.")
				}
				b = local.New(cmdutil.Diag())
			}

			var stackName tokens.QName
			if len(args) > 0 {
				stackName = tokens.QName(args[0])
			} else if cmdutil.Interactive() {
				name, err := cmdutil.ReadConsole("Enter a stack name")
				if err != nil {
					return err
				}
				stackName = tokens.QName(name)
			}

			if stackName == "" {
				return errors.New("missing stack name")
			}

			_, err := createStack(b, stackName, opts)
			return err
		}),
	}
	cmd.PersistentFlags().StringVarP(
		&cloudURL, "cloud-url", "c", "", "A URL for the Pulumi Cloud in which to initialize this stack")
	cmd.PersistentFlags().BoolVarP(
		&localBackend, "local", "l", false, "Initialize this stack locally instead of in the Pulumi Cloud")
	cmd.PersistentFlags().BoolVarP(
		&remoteBackend, "remote", "r", false, "Initialize this stack in the Pulumi Cloud instead of locally")
	cmd.PersistentFlags().StringVarP(
		&ppc, "ppc", "p", "", "A Pulumi Private Cloud (PPC) name to initialize this stack in (if not --local)")
	return cmd
}

func isLoggedIn(cloudURL string) bool {
	creds, err := workspace.GetAccessToken(cloudURL)
	return err == nil && creds != ""
}
