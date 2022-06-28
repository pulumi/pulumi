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

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newPluginLinkCmd() *cobra.Command {
	var reinstall bool

	var cmd = &cobra.Command{
		Use:   "link KIND NAME VERSION DIRECTORY",
		Args:  cmdutil.ExactArgs(4),
		Short: "Link a local plugin into the global plugins directory",
		Long: "Link a local plugin into the global plugins directory.\n" +
			"\n" +
			"The given directory will be be symbolically linked into the " +
			"Pulumi plugins directory at the correct path for the given kind, " +
			"name, and version.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {

			var kind workspace.PluginKind
			if !workspace.IsPluginKind(args[0]) {
				return fmt.Errorf("unrecognized plugin kind: %s", args[0])
			} else {
				kind = workspace.PluginKind(args[0])
			}

			name := args[1]

			parsedVersion, err := semver.ParseTolerant(args[2])
			version := &parsedVersion
			if err != nil {
				return fmt.Errorf("invalid plugin semver: %w", err)
			}

			sourceDirectory, err := filepath.Abs(args[3])
			if err != nil {
				return fmt.Errorf("invalid source directory: %w", err)
			}

			pluginInfo := &workspace.PluginInfo{
				Name:    name,
				Kind:    kind,
				Version: version,
			}

			targetDirectory, err := pluginInfo.DirPath()
			if err != nil {
				return err
			}

			if fi, _ := os.Stat(targetDirectory); fi != nil {
				if !reinstall {
					return fmt.Errorf("%s@%s is already installed in the plugin directory", name, version)
				} else {
					err := os.RemoveAll(targetDirectory)
					if err != nil {
						return err
					}
				}
			}

			return os.Symlink(sourceDirectory, targetDirectory)
		}),
	}

	cmd.PersistentFlags().BoolVar(&reinstall,
		"reinstall", false, "Reinstall a plugin even if it already exists")

	return cmd
}
