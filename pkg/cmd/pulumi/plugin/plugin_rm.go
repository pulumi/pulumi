// Copyright 2016, Pulumi Corporation.
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
	"strings"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"

	"github.com/blang/semver"
	"github.com/hashicorp/go-multierror"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/pluginstorage"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newPluginRmCmd(pluginContext pluginstorage.Context) *cobra.Command {
	var all bool
	var yes bool
	var olderThan string
	var keepLatest int
	cmd := &cobra.Command{
		Use:     "remove",
		Aliases: []string{"rm"},
		Short:   "Remove one or more plugins from the download cache",
		Long: "Remove one or more plugins from the download cache.\n" +
			"\n" +
			"Specify KIND, NAME, and/or VERSION to narrow down what will be removed.\n" +
			"If none are specified, pass --all to clear the entire cache.  If only KIND\n" +
			"and NAME are specified, but not VERSION, all versions of the plugin with\n" +
			"the given KIND and NAME will be removed.  VERSION may be a range.\n" +
			"\n" +
			"Use --older-than to remove plugins last used longer ago than the given\n" +
			"duration. The duration accepts Go duration syntax, plus d for days and w\n" +
			"for weeks, such as 30d, 2w, or 72h. Plugins without a recorded last-used\n" +
			"time are skipped by --older-than.\n" +
			"\n" +
			"Use --keep-latest to preserve the newest matching versions per plugin\n" +
			"(kind, name). For example, `pulumi plugin rm resource aws --older-than\n" +
			"30d --keep-latest 2` removes old aws resource plugins while keeping the\n" +
			"two newest matching versions.\n" +
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
			keepLatestSet := cmd.Flags().Changed("keep-latest")
			if keepLatest < 0 {
				return errors.New("--keep-latest must be non-negative")
			}
			if keepLatestSet && keepLatest == 0 {
				return errors.New("--keep-latest 0 is equivalent to --all; pass --all instead")
			}
			if len(args) == 0 && !all && olderThan == "" && !keepLatestSet {
				return errors.New("please pass --all or a filter (--older-than, --keep-latest, or a kind)")
			}
			if all && (len(args) > 0 || olderThan != "" || keepLatestSet) {
				return errors.New("--all cannot be combined with filters")
			}

			var olderThanDuration *time.Duration
			if olderThan != "" {
				threshold, err := parseAgeDuration(olderThan)
				if err != nil {
					return fmt.Errorf("--older-than: %w", err)
				}
				olderThanDuration = &threshold
			}

			var kind apitype.PluginKind
			var name string
			var version *semver.Range
			if len(args) > 0 {
				if !apitype.IsPluginKind(args[0]) {
					return fmt.Errorf("unrecognized plugin kind: %s\n\n%v", args[0], cmd.UsageString())
				}
				kind = apitype.PluginKind(args[0])
			}
			if len(args) > 1 {
				name = args[1]
			}
			if len(args) > 2 {
				r, err := semver.ParseRange(args[2])
				if err != nil {
					return fmt.Errorf("invalid plugin semver: %w", err)
				}
				version = &r
			}

			// Now build a list of plugins that match.
			plugins, err := pluginContext.GetPlugins(cmd.Context())
			if err != nil {
				return fmt.Errorf("loading plugins: %w", err)
			}
			deletes := selectPluginsToDelete(plugins, kind, name, version, olderThanDuration, keepLatest, time.Now())

			if len(deletes) == 0 {
				cmdutil.Diag().Infof(
					diag.Message("", "no plugins found to uninstall"))
				return nil
			}

			out := cmd.OutOrStdout()
			// Confirm that the user wants to do this (unless --yes was passed).
			if !yes {
				var suffix string
				if len(deletes) != 1 {
					suffix = "s"
				}
				fmt.Fprint(out,
					opts.Color.Colorize(
						fmt.Sprintf("%sThis will remove %d plugin%s from the cache:%s\n",
							colors.SpecAttention, len(deletes), suffix, colors.Reset)))
				for _, del := range deletes {
					fmt.Fprintln(out, formatPluginDeleteLine(del))
				}
				if !ui.ConfirmPrompt("", "yes", opts) {
					return nil
				}
			}

			// Run the actual delete operations.
			var result error
			for _, del := range deletes {
				plugin := del.Plugin
				if err := plugin.Delete(); err == nil {
					fmt.Fprintf(out, "removed: %s %v\n", plugin.Kind, plugin)
				} else {
					result = multierror.Append(
						result, fmt.Errorf("failed to delete %s plugin %s: %w", plugin.Kind, plugin, err))
				}
			}
			return result
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "kind"},
			{Name: "name"},
			{Name: "version"},
		},
		Required: 0,
		Variadic: false,
	})

	cmd.PersistentFlags().BoolVarP(
		&all, "all", "a", false,
		"Remove all plugins")
	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Skip confirmation prompts, and proceed with removal anyway")
	cmd.PersistentFlags().StringVar(
		&olderThan, "older-than", "",
		"Only remove plugins last used longer ago than this duration. "+
			"Accepts a standalone Nd/Nw value (e.g. 30d, 2w) or a Go duration (e.g. 72h, 1h30m); "+
			"mixed forms like 7d12h are not supported. "+
			"Plugins with no recorded last-used time are skipped.")
	cmd.PersistentFlags().IntVar(
		&keepLatest, "keep-latest", 0,
		"Keep this many of the newest matching versions per plugin (kind, name).")

	return cmd
}

func formatPluginDeleteLine(del pluginDeleteSelection) string {
	reason := ""
	if len(del.Reasons) > 0 {
		reason = " (" + strings.Join(del.Reasons, "; ") + ")"
	}
	return fmt.Sprintf("    %s %s%s", del.Plugin.Kind, del.Plugin.String(), reason)
}
