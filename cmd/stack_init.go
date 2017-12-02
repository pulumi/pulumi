// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/cloud"
	"github.com/pulumi/pulumi/pkg/backend/local"
	"github.com/pulumi/pulumi/pkg/backend/state"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newStackInitCmd() *cobra.Command {
	var cloudURL string
	var localBackend bool
	var ppc string
	cmd := &cobra.Command{
		Use:   "init <stack-name>",
		Args:  cmdutil.SpecificArgs([]string{"stack-name"}),
		Short: "Create an empty stack with the given name, ready for updates",
		Long: "Create an empty stack with the given name, ready for updates\n" +
			"\n" +
			"This command creates an empty stack with the given name.  It has no resources,\n" +
			"but afterwards it can become the target of a deployment using the `update` command.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			var b backend.Backend
			if localBackend {
				if ppc != "" {
					return errors.New("cannot pass both --local and --ppc; PPCs only available in cloud mode")
				}
				b = local.New()
			} else {
				// If no cloud URL override was given, fall back to the default.
				if cloudURL == "" {
					cloudURL = cloud.DefaultURL()
				}
				b = cloud.New(cloudURL)
			}

			stackName := tokens.QName(args[0])
			err := b.CreateStack(stackName, backend.StackCreateOptions{CloudName: ppc})
			if err != nil {
				return errors.Wrapf(err, "could not create stack")
			}

			return state.SetCurrentStack(stackName)
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
