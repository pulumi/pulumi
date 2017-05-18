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
	"errors"

	"github.com/spf13/cobra"

	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/cmdutil"
)

func newEnvInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "init <env>",
		Aliases: []string{"create"},
		Short:   "Create an empty environment with the given name, ready for deployments",
		Long: "Create an empty environment with the given name, ready for deployments\n" +
			"\n" +
			"This command creates an empty environment with the given name.  It has no resources,\n" +
			"but afterwards it can become the target of a deployment using the `deploy` command.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Read in the name of the environment to use.
			if len(args) == 0 {
				return errors.New("missing required environment name")
			}

			name := tokens.QName(args[0])
			createEnv(name)
			return nil
		}),
	}
}
