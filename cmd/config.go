// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pulumi/pulumi/pkg/pack"
	"github.com/pulumi/pulumi/pkg/resource/config"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}
	cmd.AddCommand(newConfigLsCmd())
	cmd.AddCommand(newConfigRmCmd())
	cmd.AddCommand(newConfigTextCmd())
	cmd.AddCommand(newConfigSecretCmd())

	return cmd
}

func newConfigLsCmd() *cobra.Command {
	var stack string

	lsCmd := &cobra.Command{
		Use:   "ls [key]",
		Short: "List configuration for a stack",
		Args:  cobra.MaximumNArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			stackName := tokens.QName(stack)

			if len(args) == 1 {
				key, err := parseConfigKey(args[0])
				if err != nil {
					return errors.Wrap(err, "invalid configuration key")
				}

				return getConfig(stackName, key)
			}

			return listConfig(stackName)
		}),
	}

	lsCmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Target a specific stack instead of all of this project's stacks")

	return lsCmd
}

func newConfigRmCmd() *cobra.Command {
	var stack string

	rmCmd := &cobra.Command{
		Use:   "rm <key>",
		Short: "Remove configuration value",
		Args:  cobra.ExactArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			stackName := tokens.QName(stack)

			key, err := parseConfigKey(args[0])
			if err != nil {
				return errors.Wrap(err, "invalid configuration key")
			}

			return deleteConfiguration(stackName, key)
		}),
	}

	rmCmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Target a specific stack instead of all of this project's stacks")

	return rmCmd
}

func newConfigTextCmd() *cobra.Command {
	var stack string

	textCmd := &cobra.Command{
		Use:   "text <key> <value>",
		Short: "Set configuration value",
		Args:  cobra.ExactArgs(2),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			stackName := tokens.QName(stack)

			key, err := parseConfigKey(args[0])
			if err != nil {
				return errors.Wrap(err, "invalid configuration key")
			}

			return setConfiguration(stackName, key, args[1], false /*secure*/)
		}),
	}

	textCmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Target a specific stack instead of all of this project's stacks")

	return textCmd
}

func newConfigSecretCmd() *cobra.Command {
	var stack string

	secretCmd := &cobra.Command{
		Use:   "secret <key> [value]",
		Short: "Set an encrypted configuration value",
		Args:  cobra.RangeArgs(1, 2),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			stackName := tokens.QName(stack)

			key, err := parseConfigKey(args[0])
			if err != nil {
				return errors.Wrap(err, "invalid configuration key")
			}

			var value string
			if len(args) == 2 {
				value = args[1]
			} else {
				value, err = readConsoleNoEchoWithPrompt("value")
				if err != nil {
					return err
				}
			}

			return setConfiguration(stackName, key, value, true /*secure*/)
		}),
	}

	secretCmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Target a specific stack instead of all of this project's stacks")

	return secretCmd
}

func parseConfigKey(key string) (tokens.ModuleMember, error) {
	// As a convience, we'll treat any key with no delimiter as if:
	// <program-name>:config:<key> had been written instead
	if !strings.Contains(key, tokens.TokenDelimiter) {
		pkg, err := getPackage()
		if err != nil {
			return "", err
		}

		return tokens.ParseModuleMember(fmt.Sprintf("%s:config:%s", pkg.Name, key))
	}

	return tokens.ParseModuleMember(key)
}

func prettyKey(key string) string {
	pkg, err := getPackage()
	if err != nil {
		return key
	}

	return prettyKeyForPackage(key, pkg)
}

func prettyKeyForPackage(key string, pkg *pack.Package) string {
	s := key
	defaultPrefix := fmt.Sprintf("%s:config:", pkg.Name)

	if strings.HasPrefix(s, defaultPrefix) {
		return s[len(defaultPrefix):]
	}

	return s
}

func listConfig(stackName tokens.QName) error {
	cfg, err := getConfiguration(stackName)
	if err != nil {
		return err
	}

	var decrypter config.ValueDecrypter = panicCrypter{}

	if hasSecureValue(cfg) {
		decrypter, err = getSymmetricCrypter()
		if err != nil {
			return err
		}
	}

	if cfg != nil {
		fmt.Printf("%-32s %-32s\n", "KEY", "VALUE")
		var keys []string
		for key := range cfg {
			// Note that we use the fully qualified module member here instead of a `prettyKey`, this lets us ensure that all the config
			// values for the current program are displayed next to one another in the output.
			keys = append(keys, string(key))
		}
		sort.Strings(keys)
		for _, key := range keys {
			decrypted, err := cfg[tokens.ModuleMember(key)].Value(decrypter)
			if err != nil {
				return errors.Wrap(err, "could not decrypt configuration value")
			}

			fmt.Printf("%-32s %-32s\n", prettyKey(key), decrypted)
		}
	}

	return nil
}

func getConfig(stackName tokens.QName, key tokens.ModuleMember) error {
	cfg, err := getConfiguration(stackName)
	if err != nil {
		return err
	}

	if cfg != nil {
		if v, ok := cfg[key]; ok {
			var decrypter config.ValueDecrypter = panicCrypter{}

			if v.Secure() {
				decrypter, err = getSymmetricCrypter()
				if err != nil {
					return err
				}
			}

			decrypted, err := v.Value(decrypter)
			if err != nil {
				return errors.Wrap(err, "could not decrypt configuation value")
			}

			fmt.Printf("%v\n", decrypted)

			return nil
		}
	}

	return errors.Errorf("configuration key '%v' not found for stack '%v'", prettyKey(key.String()), stackName)
}

func getConfiguration(stackName tokens.QName) (map[tokens.ModuleMember]config.Value, error) {
	pkg, err := getPackage()
	if err != nil {
		return nil, err
	}

	if stackName == "" {
		return pkg.Config, nil
	}

	return mergeConfigs(pkg.Config, pkg.Stacks[stackName].Config), nil
}

func deleteConfiguration(stackName tokens.QName, key tokens.ModuleMember) error {
	pkg, err := getPackage()
	if err != nil {
		return err
	}

	if stackName == "" {
		if pkg.Config != nil {
			delete(pkg.Config, key)
		}
	} else {
		if pkg.Stacks[stackName].Config != nil {
			delete(pkg.Stacks[stackName].Config, key)
		}
	}

	return savePackage(pkg)
}

func setConfiguration(stackName tokens.QName, key tokens.ModuleMember, value string, secure bool) error {
	pkg, err := getPackage()
	if err != nil {
		return err
	}

	var v config.Value
	if secure {
		c, err := getSymmetricCrypterForPackage(pkg)
		if err != nil {
			return err
		}

		secret, err := c.EncryptValue(value)
		if err != nil {
			return err
		}

		v = config.NewSecureValue(secret)
	} else {
		v = config.NewValue(value)
	}

	if stackName == "" {
		if pkg.Config == nil {
			pkg.Config = make(map[tokens.ModuleMember]config.Value)
		}

		pkg.Config[key] = v
	} else {
		if pkg.Stacks == nil {
			pkg.Stacks = make(map[tokens.QName]pack.StackInfo)
		}

		if pkg.Stacks[stackName].Config == nil {
			si := pkg.Stacks[stackName]
			si.Config = make(map[tokens.ModuleMember]config.Value)
			pkg.Stacks[stackName] = si
		}

		pkg.Stacks[stackName].Config[key] = v
	}

	return savePackage(pkg)
}

func mergeConfigs(global, stack map[tokens.ModuleMember]config.Value) map[tokens.ModuleMember]config.Value {
	if stack == nil {
		return global
	}

	if global == nil {
		return stack
	}

	merged := make(map[tokens.ModuleMember]config.Value)
	for key, value := range global {
		merged[key] = value
	}

	for key, value := range stack {
		merged[key] = value
	}

	return merged
}
