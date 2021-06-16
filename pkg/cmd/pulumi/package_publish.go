package main

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

func newPackagePublishCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "publish [version]",
		Args:  cmdutil.ExactArgs(1),
		Short: "Publish a plugin and SDKs.",
		Long:  "Publish a plugin and SDKs.",
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			version := args[0]
			yes := true
			skipPreview := true
			interactive := cmdutil.Interactive()

			opts, err := updateFlagsToOptions(interactive, skipPreview, yes)
			if err != nil {
				return result.FromError(err)
			}

			var displayType = display.DisplayProgress

			opts.Display = display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: interactive,
				Type:          displayType,
			}
			opts.Display.SuppressPermaLink = false

			return doPkgUp(opts, Publish, version)
		}),
	}
	return cmd
}
