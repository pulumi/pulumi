// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"sort"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newConfigCmd() *cobra.Command {
	var env string
	var unset bool
	cmd := &cobra.Command{
		Use:   "config [<key> [value]]",
		Short: "Query, set, replace, or unset configuration values",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return listConfig(env)
			}

			key, err := tokens.ParseModuleMember(args[0])
			if err != nil {
				return errors.Wrap(err, "invalid configuration key")
			}

			if len(args) == 1 {
				if !unset {
					return getConfig(env, key)
				}
				return lumiEngine.DeleteConfig(env, key)
			}

			return lumiEngine.SetConfig(env, key, args[1])
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

func listConfig(env string) error {
	config, err := lumiEngine.GetConfiguration(env)
	if err != nil {
		return err
	}

	if config != nil {
		fmt.Printf("%-32s %-32s\n", "KEY", "VALUE")
		var keys []string
		for key := range config {
			keys = append(keys, string(key))
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Printf("%-32s %-32s\n", key, config[tokens.ModuleMember(key)])
		}
	}

	return nil
}

func getConfig(env string, key tokens.ModuleMember) error {
	config, err := lumiEngine.GetConfiguration(env)
	if err != nil {
		return err
	}

	if config != nil {
		if v, ok := config[key]; ok {
			fmt.Printf("%v\n", v)
			return nil
		}
	}

	return errors.Errorf("configuration key '%v' not found for environment '%v'", key, env)

}
