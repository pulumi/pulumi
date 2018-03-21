// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func newPluginLsCmd() *cobra.Command {
	var projectOnly bool
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List plugins",
		Args:  cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Produce a list of plugins, sorted by name and version.
			var plugins []workspace.PluginInfo
			var err error
			if projectOnly {
				if plugins, err = getProjectPlugins(); err != nil {
					return errors.Wrapf(err, "loading project plugins")
				}
			} else {
				if plugins, err = workspace.GetPlugins(); err != nil {
					return errors.Wrapf(err, "loading plugins")
				}
			}

			// Devote 26 characters to the name width, unless there is a longer name.
			maxname := 26
			for _, plugin := range plugins {
				if len(plugin.Name) > maxname {
					maxname = len(plugin.Name)
				}
			}

			// Sort the plugins: by name first alphabetical ascending and version descending, so that plugins
			// with the same name/kind sort by newest to oldest.
			sort.Slice(plugins, func(i, j int) bool {
				pi, pj := plugins[i], plugins[j]
				if pi.Name < pj.Name {
					return true
				} else if pi.Name == pj.Name && pi.Kind == pj.Kind &&
					(pi.Version == nil || (pj.Version != nil && pi.Version.GT(*pj.Version))) {
					return true
				}
				return false
			})

			// And now pretty-print the list.
			var totalSize uint64
			fmt.Printf("%-"+strconv.Itoa(maxname)+"s %-12s %-26s %-18s %-18s %-18s\n",
				"NAME", "KIND", "VERSION", "SIZE", "INSTALLED", "LAST USED")
			for _, plugin := range plugins {
				var version string
				if plugin.Version != nil {
					version = plugin.Version.String()
				}
				var bytes string
				if plugin.Size == 0 {
					bytes = naString
				} else {
					bytes = humanize.Bytes(uint64(plugin.Size))
				}
				var installTime string
				if plugin.InstallTime.IsZero() {
					installTime = naString
				} else {
					installTime = humanize.Time(plugin.InstallTime)
				}
				var lastUsedTime string
				if plugin.LastUsedTime.IsZero() {
					lastUsedTime = humanNeverTime
				} else {
					lastUsedTime = humanize.Time(plugin.LastUsedTime)
				}
				fmt.Printf("%-"+strconv.Itoa(maxname)+"s %-12s %-26s %-18s %-18s %-18s\n",
					plugin.Name, plugin.Kind, version, bytes, installTime, lastUsedTime)
				totalSize += uint64(plugin.Size)
			}

			fmt.Printf("\n")
			fmt.Printf("TOTAL plugin cache size: %s\n", humanize.Bytes(totalSize))

			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&projectOnly, "project", "p", false,
		"List only the plugins used by the current project")

	return cmd
}

const humanNeverTime = "never"
const naString = "n/a"
