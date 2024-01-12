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

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newWorkflowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Preview and run workflows",
		Long:  "WORKFLOWS",
		Args:  cmdutil.NoArgs,
	}

	cmd.AddCommand(newWorkflowPreviewCmd())

	return cmd
}

func newWorkflowPreviewCmd() *cobra.Command {
	previewCommand := &cobra.Command{
		Use:   "preview",
		Short: "Preview a pulumi workflow",
		Long:  "Workflow wooo.",
		Args:  cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			//ctx := commandContext()
			//opts := display.Options{
			//	Color: cmdutil.GetGlobalColorization(),
			//}

			project, _, err := readProject()
			if err != nil {
				return err
			}

			fmt.Printf("%s", project.Name)

			// We still need runtime options and name and things so this looks like probably want to use _a_ Pulumi.yaml
			// but not _the_ Pulumi.yaml.

			// We also need a way to tell preview the set of state to work with (i.e. triggers fired, jobs done). Not
			// sure if that should be via flags here, a json input, or binary input that's built up by other `pulumi
			// workflow` commands.

			return nil
		}),
	}

	return previewCommand
}
