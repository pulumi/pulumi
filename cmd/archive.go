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
	"io/ioutil"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/util/archive"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// newArchiveCommand creates a command which just builds the archive we would ship to Pulumi.com to
// do a deployment.
func newArchiveCommand() *cobra.Command {
	var forceNoDefaultIgnores bool
	var forceDefaultIgnores bool

	cmd := &cobra.Command{
		Use:   "archive <path-to-archive>",
		Short: "create an archive suitable for deployment",
		Args:  cmdutil.SpecificArgs([]string{"path-to-archive"}),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			if forceDefaultIgnores && forceNoDefaultIgnores {
				return errors.New("can't specify --no-default-ignores and --default-ignores at the same time")
			}

			proj, path, err := workspace.DetectProjectAndPath()
			if err != nil {
				return err
			}

			useDeafultIgnores := proj.UseDefaultIgnores()

			if forceDefaultIgnores {
				useDeafultIgnores = true
			} else if forceNoDefaultIgnores {
				useDeafultIgnores = false
			}

			// path is the path to the Pulumi.yaml file.  Need its parent directory.
			dir := filepath.Dir(path)
			archiveContents, err := archive.Process(dir, useDeafultIgnores)
			if err != nil {
				return errors.Wrap(err, "creating archive")
			}

			return ioutil.WriteFile(args[0], archiveContents.Bytes(), 0644)
		}),
	}
	cmd.PersistentFlags().BoolVar(
		&forceNoDefaultIgnores, "--no-default-ignores", false,
		"Do not use default ignores, regardless of Pulumi.yaml")
	cmd.PersistentFlags().BoolVar(
		&forceDefaultIgnores, "--default-ignores", false,
		"Use default ignores, regardless of Pulumi.yaml")

	return cmd
}
