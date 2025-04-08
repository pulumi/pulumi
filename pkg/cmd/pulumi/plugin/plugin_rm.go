// Copyright 2016-2024, Pulumi Corporation.
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

package plugin

import (
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"

	"github.com/blang/semver"
	"github.com/hashicorp/go-multierror"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newPluginRmCmd() *cobra.Command {
	var all bool
	var yes bool
	cmd := &cobra.Command{
		Use:   "rm [KIND [NAME [VERSION]]]",
		Args:  cmdutil.MaximumNArgs(3),
		Short: "Remove one or more plugins from the download cache",
		Long: "Remove one or more plugins from the download cache.\n" +
			"\n" +
			"Specify KIND, NAME, and/or VERSION to narrow down what will be removed.\n" +
			"If none are specified, the entire cache will be cleared.  If only KIND and\n" +
			"NAME are specified, but not VERSION, all versions of the plugin with the\n" +
			"given KIND and NAME will be removed.  VERSION may be a range.\n" +
			"\n" +
			"This removal cannot be undone.  If a deleted plugin is subsequently required\n" +
			"in order to execute a Pulumi program, it must be re-downloaded and installed\n" +
			"using the plugin install command.",
		RunE: func(cmd *cobra.Command, args []string) error {
			yes = yes || env.SkipConfirmations.Value()
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// Parse the filters.
			var kind apitype.PluginKind
			var name string
			var version *semver.Version
			if len(args) > 0 {
				if !apitype.IsPluginKind(args[0]) {
					return fmt.Errorf("unrecognized plugin kind: %s\n\n%v", args[0], cmd.UsageString())
				}
				kind = apitype.PluginKind(args[0])
			} else if !all {
				return errors.New("please pass --all if you'd like to remove all plugins")
			}
			if len(args) > 1 {
				name = args[1]
			}
			if len(args) > 2 {
				r, err := semver.Parse(args[2])
				if err != nil {
					return fmt.Errorf("invalid plugin semver: %w", err)
				}
				version = &r
			}

			// Now build a list of plugins that match.
			var deletes []workspace.PluginInfo
			plugins, err := workspace.GetPlugins()
			if err != nil {
				return fmt.Errorf("loading plugins: %w", err)
			}
			for _, plugin := range plugins {
				if (kind == "" || plugin.Kind == kind) &&
					(name == "" || plugin.Name == name) &&
					(version == nil || (plugin.Version != nil && version.EQ(*plugin.Version))) {
					deletes = append(deletes, plugin)
				}
			}

			if len(deletes) == 0 {
				cmdutil.Diag().Infof(
					diag.Message("", "no plugins found to uninstall"))
				return nil
			}

			// Confirm that the user wants to do this (unless --yes was passed).
			if !yes {
				var suffix string
				if len(deletes) != 1 {
					suffix = "s"
				}
				fmt.Print(
					opts.Color.Colorize(
						fmt.Sprintf("%sThis will remove %d plugin%s from the cache:%s\n",
							colors.SpecAttention, len(deletes), suffix, colors.Reset)))
				for _, del := range deletes {
					fmt.Printf("    %s %s\n", del.Kind, del.String())
				}
				if !ui.ConfirmPrompt("", "yes", opts) {
					return nil
				}
			}

			// Run the actual delete operations.
			var result error
			for _, plugin := range deletes {
				if err := plugin.Delete(); err == nil {
					fmt.Printf("removed: %s %v\n", plugin.Kind, plugin)
				} else {
					result = multierror.Append(
						result, fmt.Errorf("failed to delete %s plugin %s: %w", plugin.Kind, plugin, err))
				}
			}
			return result
		},
	}

	cmd.PersistentFlags().BoolVarP(
		&all, "all", "a", false,
		"Remove all plugins")
	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Skip confirmation prompts, and proceed with removal anyway")

	return cmd
}
