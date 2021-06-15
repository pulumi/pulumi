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
	"runtime"
	"github.com/pulumi/pulumi/pkg/v3/version"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

func newDiagnosticEnvironmentCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "environment",
		Short: "Display diagnostic environment information",
		Long: "Display the Pulumi version, OS, \n" +
			"console and backend URLs",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			var os string = runtime.GOOS
			// TODO: See how to get actual OS version numbers
			switch os {
		    case "windows":
		        os = "Windows"
		    case "darwin":
		        os = "MacOs"
		    case "linux":
		        os = "Linux"
		    }

			b, err := currentBackend(opts)
			if err != nil {
				return err
			}

			cloudURL, err := workspace.GetCurrentCloudURL()
			if err != nil {
				return err
			}

			fmt.Printf("Pulumi Version: %s\n", version.Version)
			fmt.Printf("OS: %s %s\n", os, runtime.GOARCH)
			fmt.Printf("Console URL: %s\n", b.URL())
			fmt.Printf("Backend URL: %s\n", cloudURL)

			return nil
		}),
	}

	return cmd
}
