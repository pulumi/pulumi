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
	"github.com/spf13/cobra"

	"github.com/pulumi/lumi/pkg/util/cmdutil"
	"github.com/pulumi/lumi/pkg/util/contract"
)

func newPackGetCmd() *cobra.Command {
	var global bool
	var save bool
	var cmd = &cobra.Command{
		Use:   "get [deps]",
		Short: "Download a package",
		Long: "Get downloads a package by name.  If run without arguments, get will attempt\n" +
			"to download dependencies referenced by the current package.  Otherwise, if one\n" +
			"or more specific dependencies are provided, only those will be downloaded.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			contract.Failf("Get command is not yet implemented")
			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&global, "global", "g", false,
		"Install to a shared location on this machine")
	cmd.PersistentFlags().BoolVarP(
		&save, "save", "s", false,
		"Save new dependencies in the current package")

	return cmd
}
