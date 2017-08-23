// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi-fabric/pkg/util/cmdutil"
)

func newConfigCmd() *cobra.Command {
	var env string
	var unset bool
	cmd := &cobra.Command{
		Use:   "config [<key> [value]]",
		Short: "Query, set, replace, or unset configuration values",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return lumiEngine.ListConfig(env)
			} else if len(args) == 1 && !unset {
				return lumiEngine.GetConfig(env, args[0])
			} else if len(args) == 1 {
				return lumiEngine.DeleteConfig(env, args[0])
			}

			return lumiEngine.SetConfig(env, args[0], args[1])
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
