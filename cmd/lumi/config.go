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

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/cmdutil"
)

func newConfigCmd() *cobra.Command {
	var env string
	var unset bool
	cmd := &cobra.Command{
		Use:   "config [<key> [value]]",
		Short: "Query, set, replace, or unset configuration values",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			info, err := initEnvCmdName(tokens.QName(env), args)
			if err != nil {
				return err
			}
			defer info.Close() // ensure we clean up resources before exiting.

			config := info.Env.Config
			if len(info.Args) == 0 {
				// If no args were supplied, we are just printing out the current configuration.
				if config != nil {
					fmt.Printf("%-32s %-32s\n", "KEY", "VALUE")
					for _, key := range resource.StableConfigKeys(info.Env.Config) {
						v := info.Env.Config[key]
						// TODO[pulumi/lumi#113]: print complex values.
						fmt.Printf("%-32s %-32s\n", key, v)
					}
				}
			} else {
				key := tokens.Token(info.Args[0])
				if config == nil {
					config = make(resource.ConfigMap)
					info.Env.Config = config
				}
				if len(info.Args) > 1 {
					// If there is a value, we are setting the configuration entry.
					// TODO[pulumi/lumi#113]: support values other than strings.
					config[key] = info.Args[1]
					saveEnv(info.Env, info.Old, "", true)
				} else {
					// If there was no value supplied, we are either reading or unsetting the entry.
					if unset {
						delete(config, key)
						saveEnv(info.Env, info.Old, "", true)
					} else if v, has := config[key]; has {
						// TODO[pulumi/lumi#113]: print complex values.
						fmt.Printf("%v\n", v)
					} else {
						return errors.Errorf(
							"configuration key '%v' not found for environment '%v'", key, info.Env.Name)
					}
				}
			}

			return nil
		}),
	}

	cmd.PersistentFlags().StringVarP(
		&env, "env", "e", "",
		"Choose an environment other than the currently selected one")
	cmd.PersistentFlags().BoolVar(
		&unset, "unset", false,
		"Unset a configuration value")

	return cmd
}
