package main

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

func newPackageInstallCmd() *cobra.Command {

	var cmd = &cobra.Command{
		Use:   "install",
		Args:  cmdutil.MaximumNArgs(3),
		Short: "Install plugin and SDKs for development on your local machine.",
		Long:  "Install plugin and SDKs for development on your local machine.",
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
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

			return doPkgUp(opts, Install, "")
		}),
	}
	return cmd
}
