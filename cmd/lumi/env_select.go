// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
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

	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/cmdutil"
)

func newEnvSelectCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "select [<env>]",
		Aliases: []string{"checkout", "switch"},
		Short:   "Switch the current workspace to the given environment",
		Long: "Switch the current workspace to the given environment.  This allows you to use\n" +
			"other commands like `config`, `plan`, and `deploy` without needing to specify the\n" +
			"environment name each and every time.\n" +
			"\n" +
			"If no <env> argument is supplied, the current environment is printed.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Read in the name of the environment to switch to.
			if len(args) == 0 {
				if name := getCurrentEnv(); name != "" {
					fmt.Println(name)
				}
			} else {
				name := tokens.QName(args[0])
				setCurrentEnv(name, true)
			}
			return nil
		}),
	}
}
