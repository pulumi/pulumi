package main

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

func newPackageNewCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "new",
		Short: "Create a new pulumi package.",
		Long:  "Create a new pulumi package.",
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			pNewArgs := newArgs{
				interactive:       cmdutil.Interactive(),
				prompt:            promptForValue,
				templateNameOrURL: "package-go", // TODO: add more
				secretsProvider:   "default",
				offline:           true, // hack to use my local template branch
			}

			err := runNew(pNewArgs)
			if err != nil {
				return result.FromError(err)
			}

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

			return doPkgUp(opts, New, "")
		}),
	}
	return cmd
}
