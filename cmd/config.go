// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/state"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/pack"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func newConfigCmd() *cobra.Command {
	var stack string
	var showSecrets bool

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long: "Lists all configuration values for a specific stack. To add a new configuration value, run\n" +
			"'pulumi config set', to remove and existing value run 'pulumi config rm'. To get the value of\n" +
			"for a specific configuration key, use 'pulumi config get <key-name>'.",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			stack, err := requireStack(tokens.QName(stack))
			if err != nil {
				return err
			}

			return listConfig(stack, showSecrets)
		}),
	}

	cmd.Flags().BoolVar(
		&showSecrets, "show-secrets", false,
		"Show secret values when listing config instead of displaying blinded values")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Operate on a different stack than the currently selected stack")

	cmd.AddCommand(newConfigGetCmd(&stack))
	cmd.AddCommand(newConfigRmCmd(&stack))
	cmd.AddCommand(newConfigSetCmd(&stack))

	return cmd
}

func newConfigGetCmd(stack *string) *cobra.Command {
	getCmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a single configuration value",
		Args:  cmdutil.SpecificArgs([]string{"key"}),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			s, err := requireStack(tokens.QName(*stack))
			if err != nil {
				return err
			}

			key, err := parseConfigKey(args[0])
			if err != nil {
				return errors.Wrap(err, "invalid configuration key")
			}

			return getConfig(s, key)
		}),
	}

	return getCmd
}

func newConfigRmCmd(stack *string) *cobra.Command {
	var all bool
	var save bool

	rmCmd := &cobra.Command{
		Use:   "rm <key>",
		Short: "Remove configuration value",
		Args:  cmdutil.SpecificArgs([]string{"key"}),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			stackName := tokens.QName(*stack)
			if all && stackName != "" {
				return errors.New("if --all is specified, an explicit stack can not be provided")
			}

			// Ensure the stack exists.
			s, err := requireStack(stackName)
			if err != nil {
				return err
			}

			key, err := parseConfigKey(args[0])
			if err != nil {
				return errors.Wrap(err, "invalid configuration key")
			}

			var stackToSave tokens.QName
			if !all {
				stackToSave = s.Name()
			}

			if save {
				return deleteProjectConfiguration(stackToSave, key)
			}

			return deleteWorkspaceConfiguration(stackToSave, key)
		}),
	}

	rmCmd.PersistentFlags().BoolVar(
		&all, "all", false,
		"Remove a project wide configuration value that applies to all stacks")
	rmCmd.PersistentFlags().BoolVar(
		&save, "save", true,
		"Remove the configuration value from the project file (if false, it is private to your workspace)")

	return rmCmd
}

func newConfigSetCmd(stack *string) *cobra.Command {
	var all bool
	var plaintext bool
	var save bool
	var secret bool

	setCmd := &cobra.Command{
		Use:   "set <key> [value]",
		Short: "Set configuration value",
		Args:  cmdutil.RangeArgs(1, 2),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			stackName := tokens.QName(*stack)
			if all && stackName != "" {
				return errors.New("if --all is specified, an explicit stack can not be provided")
			}

			// Ensure the stack exists.
			s, err := requireStack(stackName)
			if err != nil {
				return err
			}

			key, err := parseConfigKey(args[0])
			if err != nil {
				return errors.Wrap(err, "invalid configuration key")
			}

			// Read the value from an arg or the console, disabling echoing if a secret.
			var value string
			if len(args) == 2 {
				value = args[1]
			} else if secret {
				value, err = cmdutil.ReadConsoleNoEcho("value")
				if err != nil {
					return err
				}
			} else {
				value, err = cmdutil.ReadConsole("value")
				if err != nil {
					return err
				}
			}

			// Encrypt the config value if needed.
			var v config.Value
			if secret {
				c, cerr := state.SymmetricCrypter()
				if cerr != nil {
					return cerr
				}
				enc, eerr := c.EncryptValue(value)
				if eerr != nil {
					return eerr
				}
				v = config.NewSecureValue(enc)
			} else {
				v = config.NewValue(value)
			}

			// And now save it.
			var stackToSave tokens.QName
			if !all {
				stackToSave = s.Name()
			}
			err = setConfiguration(stackToSave, key, v, save)
			if err != nil {
				return err
			}

			// If we saved a plaintext configuration value, and --plaintext was not passed, warn the user.
			if !secret && !plaintext && save {
				cmdutil.Diag().Warningf(
					diag.Message(
						"saved config key '%s' value '%s' as plaintext; "+
							"re-run with --secret to encrypt the value instead"),
					key, value)
			}

			return nil
		}),
	}

	setCmd.PersistentFlags().BoolVar(
		&all, "all", false,
		"Set a configuration value for all stacks for this project")
	setCmd.PersistentFlags().BoolVar(
		&plaintext, "plaintext", false,
		"Save the value as plaintext (unencrypted)")
	setCmd.PersistentFlags().BoolVar(
		&save, "save", true,
		"Save the configuration value in the project file (if false, it is private to your workspace)")
	setCmd.PersistentFlags().BoolVar(
		&secret, "secret", false,
		"Encrypt the value instead of storing it in plaintext")

	return setCmd
}

func parseConfigKey(key string) (tokens.ModuleMember, error) {
	// As a convience, we'll treat any key with no delimiter as if:
	// <program-name>:config:<key> had been written instead
	if !strings.Contains(key, tokens.TokenDelimiter) {
		pkg, err := workspace.GetPackage()
		if err != nil {
			return "", err
		}

		return tokens.ParseModuleMember(fmt.Sprintf("%s:config:%s", pkg.Name, key))
	}

	return tokens.ParseModuleMember(key)
}

func prettyKey(key string) string {
	pkg, err := workspace.GetPackage()
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

func setConfiguration(stackName tokens.QName, key tokens.ModuleMember, value config.Value, save bool) error {
	if save {
		return setProjectConfiguration(stackName, key, value)
	}

	return setWorkspaceConfiguration(stackName, key, value)
}

func listConfig(stack backend.Stack, showSecrets bool) error {
	cfg, err := state.Configuration(cmdutil.Diag(), stack.Name())
	if err != nil {
		return err
	}

	// By default, we will use a blinding decrypter to show '******'.  If requested, display secrets in plaintext.
	var decrypter config.Decrypter
	if cfg.HasSecureValue() && showSecrets {
		decrypter, err = state.SymmetricCrypter()
		if err != nil {
			return err
		}
	} else {
		decrypter = config.NewBlindingDecrypter()
	}

	if cfg != nil {
		// Devote 48 characters to the config key, unless there's a key longer, in which case use that.
		maxkey := 48
		for key := range cfg {
			if len(key) > maxkey {
				maxkey = len(key)
			}
		}

		fmt.Printf("%-"+strconv.Itoa(maxkey)+"s %-48s\n", "KEY", "VALUE")
		var keys []string
		for key := range cfg {
			// Note that we use the fully qualified module member here instead of a `prettyKey`, this lets us ensure
			// that all the config values for the current program are displayed next to one another in the output.
			keys = append(keys, string(key))
		}
		sort.Strings(keys)
		for _, key := range keys {
			decrypted, err := cfg[tokens.ModuleMember(key)].Value(decrypter)
			if err != nil {
				return errors.Wrap(err, "could not decrypt configuration value")
			}

			fmt.Printf("%-"+strconv.Itoa(maxkey)+"s %-48s\n", prettyKey(key), decrypted)
		}
	}

	return nil
}

func getConfig(stack backend.Stack, key tokens.ModuleMember) error {
	cfg, err := state.Configuration(cmdutil.Diag(), stack.Name())
	if err != nil {
		return err
	}

	if cfg != nil {
		if v, ok := cfg[key]; ok {
			var d config.Decrypter
			if v.Secure() {
				var err error
				if d, err = state.DefaultCrypter(cfg); err != nil {
					return errors.Wrap(err, "could not create a decrypter")
				}
			} else {
				d = config.NewPanicCrypter()
			}
			raw, err := v.Value(d)
			if err != nil {
				return errors.Wrap(err, "could not decrypt configuation value")
			}
			fmt.Printf("%v\n", raw)
			return nil
		}
	}

	return errors.Errorf(
		"configuration key '%v' not found for stack '%v'", prettyKey(key.String()), stack.Name())
}

func deleteProjectConfiguration(stackName tokens.QName, key tokens.ModuleMember) error {
	pkg, err := workspace.GetPackage()
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

	return workspace.SavePackage(pkg)
}

func deleteWorkspaceConfiguration(stackName tokens.QName, key tokens.ModuleMember) error {
	w, err := workspace.New()
	if err != nil {
		return err
	}

	if config, has := w.Settings().Config[stackName]; has {
		delete(config, key)
	}

	return w.Save()
}

func setProjectConfiguration(stackName tokens.QName, key tokens.ModuleMember, value config.Value) error {
	pkg, err := workspace.GetPackage()
	if err != nil {
		return err
	}

	if stackName == "" {
		if pkg.Config == nil {
			pkg.Config = make(map[tokens.ModuleMember]config.Value)
		}

		pkg.Config[key] = value
	} else {
		if pkg.Stacks == nil {
			pkg.Stacks = make(map[tokens.QName]pack.StackInfo)
		}

		if pkg.Stacks[stackName].Config == nil {
			si := pkg.Stacks[stackName]
			si.Config = make(map[tokens.ModuleMember]config.Value)
			pkg.Stacks[stackName] = si
		}

		pkg.Stacks[stackName].Config[key] = value
	}

	return workspace.SavePackage(pkg)
}

func setWorkspaceConfiguration(stackName tokens.QName, key tokens.ModuleMember, value config.Value) error {
	w, err := workspace.New()
	if err != nil {
		return err
	}

	if _, has := w.Settings().Config[stackName]; !has {
		w.Settings().Config[stackName] = make(map[tokens.ModuleMember]config.Value)
	}

	w.Settings().Config[stackName][key] = value

	return w.Save()
}
