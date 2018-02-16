// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

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
			var b backend.Backend
			var opts interface{}
			if localBackend {
				if ppc != "" {
					return errors.New("cannot pass both --local and --ppc; PPCs only available in cloud mode")
				}
				b = local.New(cmdutil.Diag())
			} else {
				// If no cloud URL override was given, fall back to the default.
				if cloudURL == "" {
					cloudURL = cloud.DefaultURL()
				}

				// Check to see if the user is logged in, if they are not, fail with a nicer message
				if creds, err := workspace.GetAccessToken(cloudURL); err != nil || creds == "" {
					return errors.New("you must be logged in to create stacks in the Pulumi Cloud. Run " +
						"`pulumi login` to log in or pass `--local` to create the stack locally.")
				}

				b = cloud.New(cmdutil.Diag(), cloudURL)
				opts = cloud.CreateStackOptions{CloudName: ppc}
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
	cmd.PersistentFlags().StringVarP(
		&ppc, "ppc", "p", "", "A Pulumi Private Cloud (PPC) name to initialize this stack in (if not --local)")
	return cmd
}
