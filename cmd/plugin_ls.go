// Copyright 2016-2018, Pulumi Corporation.
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

package cmd

import (
	"fmt"
	"sort"

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

			table := []cmdutil.TableRow{}
			table = append(table, cmdutil.TableRow{
				Columns: []string{"NAME", "KIND", "VERSION", "SIZE", "INSTALLED", "LAST USED"},
			})

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

				table = append(table, cmdutil.TableRow{
					Columns: []string{plugin.Name, string(plugin.Kind), version, bytes, installTime, lastUsedTime},
				})

				totalSize += uint64(plugin.Size)
			}

			cmdutil.PrintTable(table)
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
