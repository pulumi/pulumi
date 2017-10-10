// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"sort"

	"github.com/pulumi/pulumi/pkg/util/contract"

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
			envName, err := explicitOrCurrent(env)
			if err != nil {
				return err
			}

			if len(args) == 0 {
				return listConfig(envName)
			}

			key, err := tokens.ParseModuleMember(args[0])
			if err != nil {
				return errors.Wrap(err, "invalid configuration key")
			}

			if len(args) == 1 {
				if !unset {
					return getConfig(envName, key)
				}
				return deleteConfiguration(envName, key)
			}

			return setConfiguration(envName, key, args[1])
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

func listConfig(envName tokens.QName) error {
	config, err := getConfiguration(envName)
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

func getConfig(envName tokens.QName, key tokens.ModuleMember) error {
	config, err := getConfiguration(envName)
	if err != nil {
		return err
	}

	if config != nil {
		if v, ok := config[key]; ok {
			fmt.Printf("%v\n", v)
			return nil
		}
	}

	return errors.Errorf("configuration key '%v' not found for environment '%v'", key, envName)
}

func getConfiguration(envName tokens.QName) (map[tokens.ModuleMember]string, error) {
	target, _, err := lumiEngine.Environment.GetEnvironment(envName)
	if err != nil {
		return nil, err
	}

	contract.Assert(target != nil)
	return target.Config, nil
}

func deleteConfiguration(envName tokens.QName, key tokens.ModuleMember) error {
	target, snapshot, err := lumiEngine.Environment.GetEnvironment(envName)
	if err != nil {
		return err
	}

	contract.Assert(target != nil)

	if target.Config != nil {
		delete(target.Config, key)
	}

	return lumiEngine.Environment.SaveEnvironment(target, snapshot)
}

func setConfiguration(envName tokens.QName, key tokens.ModuleMember, value string) error {
	target, snapshot, err := lumiEngine.Environment.GetEnvironment(envName)
	if err != nil {
		return err
	}

	contract.Assert(target != nil)

	if target.Config == nil {
		target.Config = make(map[tokens.ModuleMember]string)
	}

	target.Config[key] = value

	return lumiEngine.Environment.SaveEnvironment(target, snapshot)
}
