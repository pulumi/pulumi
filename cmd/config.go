// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pulumi/pulumi/pkg/pack"

	"github.com/pulumi/pulumi/pkg/util/contract"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newConfigCmd() *cobra.Command {
	var stack string
	var unset bool
	cmd := &cobra.Command{
		Use:   "config [<key> [value]]",
		Short: "Query, set, replace, or unset configuration values",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			stackName, err := explicitOrCurrent(stack)
			if err != nil {
				return err
			}

			if len(args) == 0 {
				return listConfig(stackName)
			}

			key, err := parseConfigKey(args[0])
			if err != nil {
				return errors.Wrap(err, "invalid configuration key")
			}

			if len(args) == 1 {
				if !unset {
					return getConfig(stackName, key)
				}
				return deleteConfiguration(stackName, key)
			}

			return setConfiguration(stackName, key, args[1])
		}),
	}

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Choose an stack other than the currently selected one")
	cmd.PersistentFlags().BoolVar(
		&unset, "unset", false,
		"Unset a configuration value")

	return cmd
}

func parseConfigKey(key string) (tokens.ModuleMember, error) {

	// As a convience, we'll treat any key with no delimiter as if:
	// <program-name>:config:<key> had been written instead
	if !strings.Contains(key, tokens.TokenDelimiter) {
		_, pkg, err := getPackage()
		if err != nil {
			return "", err
		}

		return tokens.ParseModuleMember(fmt.Sprintf("%s:config:%s", pkg.Name, key))
	}

	return tokens.ParseModuleMember(key)
}

func prettyKey(key string) string {
	_, pkg, err := getPackage()
	if err != nil {
		return key
	}

	return prettyKeyForPackage(key, pkg)
}

func prettyKeyForPackage(key string, pkg pack.Package) string {
	s := key
	defaultPrefix := fmt.Sprintf("%s:config:", pkg.Name)

	if strings.HasPrefix(s, defaultPrefix) {
		return s[len(defaultPrefix):]
	}

	return s
}

func listConfig(stackName tokens.QName) error {
	config, err := getConfiguration(stackName)
	if err != nil {
		return err
	}

	if config != nil {
		fmt.Printf("%-32s %-32s\n", "KEY", "VALUE")
		var keys []string
		for key := range config {
			// Note that we use the fully qualified module member here instead of a `prettyKey`, this lets us ensure that all the config
			// values for the current program are displayed next to one another in the output.
			keys = append(keys, string(key))
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Printf("%-32s %-32s\n", prettyKey(key), config[tokens.ModuleMember(key)])
		}
	}

	return nil
}

func getConfig(stackName tokens.QName, key tokens.ModuleMember) error {
	config, err := getConfiguration(stackName)
	if err != nil {
		return err
	}

	if config != nil {
		if v, ok := config[key]; ok {
			fmt.Printf("%v\n", v)
			return nil
		}
	}

	return errors.Errorf("configuration key '%v' not found for stack '%v'", prettyKey(key.String()), stackName)
}

func getConfiguration(stackName tokens.QName) (map[tokens.ModuleMember]string, error) {
	target, _, err := getStack(stackName)
	if err != nil {
		return nil, err
	}

	contract.Assert(target != nil)
	return target.Config, nil
}

func deleteConfiguration(stackName tokens.QName, key tokens.ModuleMember) error {
	target, snapshot, err := getStack(stackName)
	if err != nil {
		return err
	}

	contract.Assert(target != nil)

	if target.Config != nil {
		delete(target.Config, key)
	}

	return saveStack(target, snapshot)
}

func setConfiguration(stackName tokens.QName, key tokens.ModuleMember, value string) error {
	target, snapshot, err := getStack(stackName)
	if err != nil {
		return err
	}

	contract.Assert(target != nil)

	if target.Config == nil {
		target.Config = make(map[tokens.ModuleMember]string)
	}

	target.Config[key] = value

	return saveStack(target, snapshot)
}
