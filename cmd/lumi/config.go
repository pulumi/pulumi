// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

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

			config := info.Target.Config
			if len(info.Args) == 0 {
				// If no args were supplied, we are just printing out the current configuration.
				if config != nil {
					fmt.Printf("%-32s %-32s\n", "KEY", "VALUE")
					for _, key := range info.Target.Config.StableKeys() {
						v := info.Target.Config[key]
						// TODO[pulumi/lumi#113]: print complex values.
						fmt.Printf("%-32s %-32s\n", key, v)
					}
				}
			} else {
				key := tokens.Token(info.Args[0])
				if config == nil {
					config = make(resource.ConfigMap)
					info.Target.Config = config
				}
				if len(info.Args) > 1 {
					// If there is a value, we are setting the configuration entry.
					// TODO[pulumi/lumi#113]: support values other than strings.
					config[key] = info.Args[1]
					saveEnv(info.Target, info.Snapshot, "", true)
				} else {
					// If there was no value supplied, we are either reading or unsetting the entry.
					if unset {
						delete(config, key)
						saveEnv(info.Target, info.Snapshot, "", true)
					} else if v, has := config[key]; has {
						// TODO[pulumi/lumi#113]: print complex values.
						fmt.Printf("%v\n", v)
					} else {
						return errors.Errorf(
							"configuration key '%v' not found for environment '%v'", key, info.Target.Name)
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
