// Copyright 2016-2021, Pulumi Corporation.
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
	"runtime"
	"github.com/pulumi/pulumi/pkg/v3/version"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
	"os/exec"
)

func newDiagnosticEnvironmentCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "environment",
		Short: "Display diagnostic environment information",
		Long:  "Display the Pulumi version, OS, runtime info, backend URL and stack data",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// OS Info
			// TODO: See how to get actual OS version numbers
			var os string = runtime.GOOS
			switch os {
		    case "windows":
		        os = "Windows"
		    case "darwin":
		        os = "MacOs"
		    case "linux":
		        os = "Linux"
		    }

		    // Console URL Info
			b, err := currentBackend(opts)
			if err != nil {
				return err
			}

			// Stack Info
			s, err := requireStack("", true, opts, false /*setCurrent*/)
			if err != nil {
				return err
			}

			snap, err := s.Snapshot(commandContext())
			if err != nil {
				return err
			}

			var rescnt int
			if snap != nil {
				rescnt = len(snap.Resources)
			}

			// Runtime versions info
			proj, _, err := readProject()
			if err != nil {
				return err
			}

			var runtimeName string = proj.Runtime.Name()
			var runtimeVersion string = ""
			switch runtimeName {
			case "dotnet":
				cmdOutput, _ := exec.Command("dotnet", "--version").Output()
				runtimeVersion = string(cmdOutput[:])
			case "go":
				cmdOutput, _ := exec.Command("go", "version").Output()
				runtimeVersion = string(cmdOutput[:])
			case "nodejs":
				cmdOutput, _ := exec.Command("npm", "--version").Output()
				runtimeVersion = string(cmdOutput[:])
			case "python":
				cmdOutput, _ := exec.Command("python3", "--version").Output()
				runtimeVersion = string(cmdOutput[:])

			}

			// Outputs
			fmt.Printf("Pulumi Version: %s\n", version.Version)
			fmt.Printf("OS: %s %s\n", os, runtime.GOARCH)
			fmt.Printf("Runtime: %s %s", runtimeName, runtimeVersion)
			fmt.Printf("Backend URL: %s\n", b.URL())
			fmt.Printf("Current stack resources (%d):\n", rescnt)
			if rescnt == 0 {
				fmt.Printf("    No resources currently in this stack\n")
			} else {
				rows, ok := renderTree(snap, false /*showURNs*/, false /*showIDs*/)
				if !ok {
					for _, res := range snap.Resources {
						rows = append(rows, renderResourceRow(res, "", "    ", false /*showURNs*/, false /*showIDs*/))
					}
				}

				cmdutil.PrintTable(cmdutil.Table{
					Headers: []string{"TYPE", "NAME"},
					Rows:    rows,
					Prefix:  "    ",
				})

				outputs, err := getStackOutputs(snap, false /*showSecrets*/)
				if err == nil {
					fmt.Printf("\n")
					printStackOutputs(outputs)
				}
			}

			return nil
		}),
	}

	return cmd
}
