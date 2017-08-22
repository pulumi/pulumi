// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi-fabric/pkg/resource"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
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
				return ListConfig(env)
			} else if len(args) == 1 && !unset {
				return GetConfig(env, args[0])
			} else if len(args) == 1 {
				return DeleteConfig(env, args[0])
			}

			return SetConfig(env, args[0], args[1])
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

func ListConfig(envName string) error {
	info, err := initEnvCmdName(tokens.QName(envName), "")
	if err != nil {
		return err
	}
	config := info.Target.Config

	if config != nil {
		fmt.Printf("%-32s %-32s\n", "KEY", "VALUE")
		for _, key := range info.Target.Config.StableKeys() {
			v := info.Target.Config[key]
			// TODO[pulumi/pulumi-fabric#113]: print complex values.
			fmt.Printf("%-32s %-32s\n", key, v)
		}
	}
	return nil
}

func GetConfig(envName string, key string) error {
	info, err := initEnvCmdName(tokens.QName(envName), "")
	if err != nil {
		return err
	}
	config := info.Target.Config

	if config != nil {
		if v, has := config[tokens.Token(key)]; has {
			fmt.Printf("%v\n", v)
			return nil
		}
	}

	return errors.Errorf("configuration key '%v' not found for environment '%v'", key, info.Target.Name)
}

func SetConfig(envName string, key string, value string) error {
	info, err := initEnvCmdName(tokens.QName(envName), "")
	if err != nil {
		return err
	}
	config := info.Target.Config

	if config == nil {
		config = make(resource.ConfigMap)
		info.Target.Config = config
	}

	config[tokens.Token(key)] = value

	if !saveEnv(info.Target, info.Snapshot, "", true) {
		return errors.Errorf("could not save configuration value")
	}

	return nil
}

func DeleteConfig(envName string, key string) error {
	info, err := initEnvCmdName(tokens.QName(envName), "")
	if err != nil {
		return err
	}
	config := info.Target.Config

	if config != nil {
		delete(config, tokens.Token(key))

		if !saveEnv(info.Target, info.Snapshot, "", true) {
			return errors.Errorf("could not save configuration value")
		}
	}

	return nil
}
