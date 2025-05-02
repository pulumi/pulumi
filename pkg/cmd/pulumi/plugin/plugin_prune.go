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
	"fmt"
	"sort"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// pluginPruneCmd implements the `plugin prune` command
type pluginPruneCmd struct {
	diag                   diag.Sink
	getPluginsWithMetadata func() ([]workspace.PluginInfo, error)
	dryRun                 bool
	yes                    bool
	latestOnly             bool
	deletePlugin           func(workspace.PluginInfo) error
}

// newPluginPruneCmd creates a new cobra command for pruning plugins
func newPluginPruneCmd() *cobra.Command {
	var dryRun bool
	var yes bool
	var latestOnly bool

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove old versions of plugins from the download cache",
		Long: "Remove old versions of plugins from the download cache.\n" +
			"\n" +
			"By default, this command keeps the latest version of each plugin for each\n" +
			"major version and removes older patch and minor versions.\n" +
			"\n" +
			"This helps reclaim disk space from old plugin versions that are no longer needed.\n" +
			"\n" +
			"Use --latest-only to keep only the latest version regardless of major version.\n" +
			"\n" +
			"This removal cannot be undone. If a deleted plugin is subsequently required\n" +
			"to execute a Pulumi program, it must be re-downloaded and installed\n" +
			"using the plugin install command.",
		Args: cmdutil.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			yes = yes || env.SkipConfirmations.Value()

			pruneCmd := &pluginPruneCmd{
				diag:                   cmdutil.Diag(),
				getPluginsWithMetadata: workspace.GetPluginsWithMetadata,
				dryRun:                 dryRun,
				yes:                    yes,
				latestOnly:             latestOnly,
				deletePlugin:           func(plugin workspace.PluginInfo) error { return plugin.Delete() },
			}

			return pruneCmd.Run()
		},
	}

	cmd.PersistentFlags().BoolVarP(
		&dryRun, "dry-run", "n", false,
		"Show what would be pruned without actually removing anything")
	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Skip confirmation prompts, and proceed with pruning anyway")
	cmd.PersistentFlags().BoolVarP(
		&latestOnly, "latest-only", "l", false,
		"Keep only the absolute latest version of each plugin (ignoring major version differences)")

	return cmd
}

// Run executes the plugin prune command
func (cmd *pluginPruneCmd) Run() error {
	opts := display.Options{
		Color: cmdutil.GetGlobalColorization(),
	}

	// Get all plugins
	plugins, err := cmd.getPluginsWithMetadata()
	if err != nil {
		return fmt.Errorf("loading plugins: %w", err)
	}

	if len(plugins) == 0 {
		cmd.diag.Infof(
			diag.Message("", "no plugins found to prune"))
		return nil
	}

	// Group plugins by kind, name, and major version
	groups := make(map[string][]workspace.PluginInfo)
	for _, plugin := range plugins {
		// Skip bundled plugins - we don't want to mess with these
		if workspace.IsPluginBundled(plugin.Kind, plugin.Name) {
			continue
		}

		// Create a key that includes kind, name, and major version (if available)
		var key string
		if plugin.Version != nil {
			if cmd.latestOnly {
				// When using latestOnly, only group by kind and name
				key = fmt.Sprintf("%s|%s", plugin.Kind, plugin.Name)
			} else {
				// Group by kind, name, and major version
				key = fmt.Sprintf("%s|%s|v%d", plugin.Kind, plugin.Name, plugin.Version.Major)
			}
		} else {
			// If no version, just use kind and name
			key = fmt.Sprintf("%s|%s", plugin.Kind, plugin.Name)
		}

		groups[key] = append(groups[key], plugin)
	}

	// For each group, identify plugins to remove (all but the latest version)
	toRemove := make([]workspace.PluginInfo, 0, len(plugins))
	toKeep := make([]workspace.PluginInfo, 0, len(plugins))
	var totalSizeRemoved uint64

	for _, group := range groups {
		if len(group) <= 1 {
			// Only one version, keep it
			toKeep = append(toKeep, group[0])
			continue
		}

		// Sort versions in descending order
		sort.Slice(group, func(i, j int) bool {
			// If either version is nil, keep the one with a version
			if group[i].Version == nil {
				return false
			}
			if group[j].Version == nil {
				return true
			}
			// Otherwise sort by version (newer versions first)
			return group[i].Version.GT(*group[j].Version)
		})

		// Keep only the latest version in each group
		toKeep = append(toKeep, group[0])

		// Remove the rest
		for i := 1; i < len(group); i++ {
			toRemove = append(toRemove, group[i])
			totalSizeRemoved += group[i].Size
		}
	}

	if len(toRemove) == 0 {
		cmd.diag.Infof(
			diag.Message("", "no plugins found to prune"))
		return nil
	}

	// Confirm that the user wants to do this (unless --yes was passed)
	if !cmd.dryRun && !cmd.yes {
		var suffix string
		if len(toRemove) != 1 {
			suffix = "s"
		}
		fmt.Print(
			opts.Color.Colorize(
				fmt.Sprintf("%sThis will remove %d plugin%s from the cache, reclaiming %s:%s\n",
					colors.SpecAttention, len(toRemove), suffix,
					humanize.Bytes(totalSizeRemoved), colors.Reset)))

		fmt.Println("Plugins to remove:")
		// Sort by name and version for a nice display
		sort.Slice(toRemove, func(i, j int) bool {
			if toRemove[i].Name != toRemove[j].Name {
				return toRemove[i].Name < toRemove[j].Name
			}
			if toRemove[i].Kind != toRemove[j].Kind {
				return toRemove[i].Kind < toRemove[j].Kind
			}
			// Sort by version if available
			if toRemove[i].Version != nil && toRemove[j].Version != nil {
				return toRemove[i].Version.GT(*toRemove[j].Version)
			}
			return false
		})

		for _, plugin := range toRemove {
			versionStr := "n/a"
			if plugin.Version != nil {
				versionStr = plugin.Version.String()
			}
			fmt.Printf("    %s %s v%s (%s)\n",
				plugin.Kind, plugin.Name, versionStr, humanize.Bytes(plugin.Size))
		}

		fmt.Println("\nPlugins to keep:")
		// Sort the kept plugins as well
		sort.Slice(toKeep, func(i, j int) bool {
			if toKeep[i].Name != toKeep[j].Name {
				return toKeep[i].Name < toKeep[j].Name
			}
			if toKeep[i].Kind != toKeep[j].Kind {
				return toKeep[i].Kind < toKeep[j].Kind
			}
			// Sort by version if available
			if toKeep[i].Version != nil && toKeep[j].Version != nil {
				return toKeep[i].Version.GT(*toKeep[j].Version)
			}
			return false
		})

		displayedKinds := make(map[string]bool)
		for _, plugin := range toKeep {
			key := fmt.Sprintf("%s|%s", plugin.Kind, plugin.Name)
			if displayedKinds[key] {
				continue // Only show one version of each kind|name combo to avoid long lists
			}

			versionStr := "n/a"
			if plugin.Version != nil {
				versionStr = plugin.Version.String()
			}

			fmt.Printf("    %s %s v%s (%s)\n",
				plugin.Kind, plugin.Name, versionStr, humanize.Bytes(plugin.Size))

			// Mark as displayed
			displayedKinds[key] = true
		}

		// Add a note if we hid some kept plugins
		if len(displayedKinds) < len(toKeep) {
			fmt.Printf("    ... and %d more\n", len(toKeep)-len(displayedKinds))
		}

		if !ui.ConfirmPrompt("Do you want to proceed?", "yes", opts) {
			return nil
		}
	}

	if cmd.dryRun {
		fmt.Println("Dry run - no changes made")
		fmt.Printf("Would remove %d plugins, reclaiming %s\n", len(toRemove), humanize.Bytes(totalSizeRemoved))
		return nil
	}

	// Remove the plugins
	var failed int
	for _, plugin := range toRemove {
		versionStr := "n/a"
		if plugin.Version != nil {
			versionStr = plugin.Version.String()
		}

		if err := cmd.deletePlugin(plugin); err == nil {
			fmt.Printf("removed: %s %s v%s\n", plugin.Kind, plugin.Name, versionStr)
		} else {
			fmt.Printf("failed to remove: %s %s v%s: %v\n", plugin.Kind, plugin.Name, versionStr, err)
			failed++
		}
	}

	fmt.Printf("Successfully removed %d plugins, reclaimed %s\n",
		len(toRemove)-failed, humanize.Bytes(totalSizeRemoved))

	if failed > 0 {
		return fmt.Errorf("failed to remove %d plugins", failed)
	}

	return nil
}
