// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/diag"
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
			stack, err := requireStack(tokens.QName(stack), true)
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
			s, err := requireStack(tokens.QName(*stack), true)
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
			s, err := requireStack(stackName, true)
			if err != nil {
				return err
			}

			key, err := parseConfigKey(args[0])
			if err != nil {
				return errors.Wrap(err, "invalid configuration key")
			}

			ps, err := workspace.DetectProjectStack(s.Name())
			if err != nil {
				return err
			}

			if ps.Config != nil {
				delete(ps.Config, key)
			}

			return workspace.SaveProjectStack(s.Name(), ps)
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
	var plaintext bool
	var secret bool

	setCmd := &cobra.Command{
		Use:   "set <key> [value]",
		Short: "Set configuration value",
		Long: "Configuration values can be accessed when a stack is being deployed and used to configure behavior. \n" +
			"If a value is not present on the command line, pulumi will prompt for the value. Multi-line values\n" +
			"may be set by piping a file to standard in.",
		Args: cmdutil.RangeArgs(1, 2),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			stackName := tokens.QName(*stack)

			// Ensure the stack exists.
			s, err := requireStack(stackName, true)
			if err != nil {
				return err
			}

			key, err := parseConfigKey(args[0])
			if err != nil {
				return errors.Wrap(err, "invalid configuration key")
			}

			var value string
			switch {
			case len(args) == 2:
				value = args[1]
			case !terminal.IsTerminal(int(os.Stdin.Fd())):
				b, readerr := ioutil.ReadAll(os.Stdin)
				if readerr != nil {
					return readerr
				}
				value = cmdutil.RemoveTralingNewline(string(b))
			case secret:
				value, err = cmdutil.ReadConsoleNoEcho("value")
				if err != nil {
					return err
				}
			default:
				value, err = cmdutil.ReadConsole("value")
				if err != nil {
					return err
				}
			}

			// Encrypt the config value if needed.
			var v config.Value
			if secret {
				c, cerr := backend.GetStackCrypter(s)
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

			ps, err := workspace.DetectProjectStack(s.Name())
			if err != nil {
				return err
			}

			ps.Config[key] = v

			err = workspace.SaveProjectStack(s.Name(), ps)
			if err != nil {
				return err
			}

			// If we saved a plaintext configuration value, and --plaintext was not passed, warn the user.
			if !secret && !plaintext {
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
		&plaintext, "plaintext", false,
		"Save the value as plaintext (unencrypted)")
	setCmd.PersistentFlags().BoolVar(
		&secret, "secret", false,
		"Encrypt the value instead of storing it in plaintext")

	return setCmd
}

func parseConfigKey(key string) (config.Key, error) {
	// As a convience, we'll treat any key with no delimiter as if:
	// <program-name>:config:<key> had been written instead
	if !strings.Contains(key, tokens.TokenDelimiter) {
		proj, err := workspace.DetectProject()
		if err != nil {
			return "", err
		}

		return config.ParseKey(fmt.Sprintf("%s:config:%s", proj.Name, key))
	}

	return config.ParseKey(key)
}

func prettyKey(key string) string {
	proj, err := workspace.DetectProject()
	if err != nil {
		return key
	}

	return prettyKeyForProject(key, proj)
}

func prettyKeyForProject(key string, proj *workspace.Project) string {
	s := key
	defaultPrefix := fmt.Sprintf("%s:config:", proj.Name)

	if strings.HasPrefix(s, defaultPrefix) {
		return s[len(defaultPrefix):]
	}

	return s
}

func listConfig(stack backend.Stack, showSecrets bool) error {
	ps, err := workspace.DetectProjectStack(stack.Name())
	if err != nil {
		return err
	}

	cfg := ps.Config

	// By default, we will use a blinding decrypter to show '******'.  If requested, display secrets in plaintext.
	var decrypter config.Decrypter
	if cfg.HasSecureValue() && showSecrets {
		decrypter, err = backend.GetStackCrypter(stack)
		if err != nil {
			return err
		}
	} else {
		decrypter = config.NewBlindingDecrypter()
	}

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
		decrypted, err := cfg[config.Key(key)].Value(decrypter)
		if err != nil {
			return errors.Wrap(err, "could not decrypt configuration value")
		}

		fmt.Printf("%-"+strconv.Itoa(maxkey)+"s %-48s\n", prettyKey(key), decrypted)
	}

	return nil
}

func getConfig(stack backend.Stack, key config.Key) error {
	ps, err := workspace.DetectProjectStack(stack.Name())
	if err != nil {
		return err
	}

	cfg := ps.Config

	if v, ok := cfg[key]; ok {
		var d config.Decrypter
		if v.Secure() {
			var err error
			if d, err = backend.GetStackCrypter(stack); err != nil {
				return errors.Wrap(err, "could not create a decrypter")
			}
		} else {
			d = config.NewPanicCrypter()
		}
		raw, err := v.Value(d)
		if err != nil {
			return errors.Wrap(err, "could not decrypt configuration value")
		}
		fmt.Printf("%v\n", raw)
		return nil
	}

	return errors.Errorf(
		"configuration key '%v' not found for stack '%v'", prettyKey(key.String()), stack.Name())
}
