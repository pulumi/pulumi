// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"

	"github.com/blang/semver"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func newPluginPruneCmd() *cobra.Command {
	var all bool
	var yes bool
	var cmd = &cobra.Command{
		Use:   "prune [KIND [NAME [VERSION]]]",
		Args:  cmdutil.MaximumNArgs(3),
		Short: "Prune one or more plugins from the download cache",
		Long: "Prune one or more plugins from the download cache.\n" +
			"\n" +
			"Specify KIND, NAME, and/or VERSION to narrow down what will be pruned.\n" +
			"If none are specified, the entire cache will be pruned.  If only KIND and\n" +
			"NAME are specified, but not VERSION, all versions of the plugin with the\n" +
			"given KIND and NAME will be pruned.  VERSION may be a range.\n" +
			"\n" +
			"If a pruned plugin is subsequently required in order to execute a Pulumi\n" +
			"program, it will be automatically re-downloaded.  Plugins may be explicitly\n" +
			"downloaded and installed using the plugin install command.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Parse the filters.
			var kind workspace.PluginKind
			var name string
			var version *semver.Range
			if len(args) > 0 {
				if !workspace.IsPluginKind(args[0]) {
					return errors.Errorf("unrecognized plugin kind: %s", kind)
				}
				kind = workspace.PluginKind(args[0])
			}
			if len(args) > 1 {
				name = args[1]
			}
			if len(args) > 2 {
				r, err := semver.ParseRange(args[2])
				if err != nil {
					return errors.Wrap(err, "invalid plugin semver")
				}
				version = &r
			}

			// Now build a list of plugins that match.
			var deletes []workspace.PluginInfo
			plugins, err := workspace.GetPlugins()
			if err != nil {
				return errors.Wrap(err, "loading plugins")
			}
			for _, plugin := range plugins {
				if (kind == "" || plugin.Kind == kind) &&
					(name == "" || plugin.Name == name) &&
					(version == nil || (plugin.Version != nil && (*version)(*plugin.Version))) {
					deletes = append(deletes, plugin)
				}
			}

			// Confirm that the user wants to do this (unless --yes was passed), and do the deletes.
			var suffix string
			if len(deletes) != 1 {
				suffix = "s"
			}
			prompt := fmt.Sprintf("This will remove %d plugin%s from the cache.", len(deletes), suffix)
			if yes || confirmPrompt(prompt, "yes") {
				var result error
				for _, plugin := range deletes {
					if err := plugin.Delete(); err != nil {
						result = multierror.Append(
							result, errors.Wrapf(err, "failed to delete plugin %s", plugin.Path))
					}
				}
				if result != nil {
					return result
				}
			}

			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(&all, "all", "a", false, "Remove all plugins")
	cmd.PersistentFlags().BoolVar(&yes, "yes", false, "Skip confirmation prompts, and proceed with removal anyway")

	return cmd
}
