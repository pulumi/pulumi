// Copyright 2016-2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/nbutton23/zxcvbn-go"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/pulumi/esc"
	"github.com/pulumi/esc/cmd/esc/cli"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets/cloud"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func NewConfigCmd() *cobra.Command {
	var stack string
	var showSecrets bool
	var jsonOut bool
	var open bool

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long: "Lists all configuration values for a specific stack. To add a new configuration value, run\n" +
			"`pulumi config set`. To remove an existing value run `pulumi config rm`. To get the value of\n" +
			"for a specific configuration key, use `pulumi config get <key-name>`.",
		Args: cmdutil.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			project, _, err := ws.ReadProject()
			if err != nil {
				return err
			}

			stack, err := cmdStack.RequireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				stack,
				cmdStack.OfferNew|cmdStack.SetCurrent,
				opts,
			)
			if err != nil {
				return err
			}

			ps, err := stack.Load(ctx, project, cmdStack.ConfigFile)
			if err != nil {
				return err
			}

			// If --open is explicitly set, use that value. Otherwise, default to true if --show-secrets is set.
			openSetByUser := cmd.Flags().Changed("open")

			var openEnvironment bool
			if openSetByUser {
				openEnvironment = open
			} else {
				openEnvironment = showSecrets
			}

			ssml := cmdStack.NewStackSecretsManagerLoaderFromEnv()

			return listConfig(
				ctx,
				ssml,
				os.Stdout,
				project,
				stack,
				ps,
				showSecrets,
				jsonOut,
				openEnvironment,
			)
		},
	}

	cmd.Flags().BoolVar(
		&showSecrets, "show-secrets", false,
		"Show secret values when listing config instead of displaying blinded values")
	cmd.Flags().BoolVar(
		&open, "open", false,
		"Open and resolve any environments listed in the stack configuration. "+
			"Defaults to true if --show-secrets is set, false otherwise")
	cmd.Flags().BoolVarP(
		&jsonOut, "json", "j", false,
		"Emit output as JSON")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().StringVar(
		&cmdStack.ConfigFile, "config-file", "",
		"Use the configuration values in the specified file rather than detecting the file name")

	cmd.AddCommand(newConfigGetCmd(&stack))
	cmd.AddCommand(newConfigRmCmd(&stack))
	cmd.AddCommand(newConfigRmAllCmd(&stack))
	cmd.AddCommand(newConfigSetCmd(&stack))
	cmd.AddCommand(newConfigSetAllCmd(&stack))
	cmd.AddCommand(newConfigRefreshCmd(&stack))
	cmd.AddCommand(newConfigCopyCmd(&stack))
	cmd.AddCommand(newConfigEnvCmd(&stack))

	return cmd
}

func newConfigCopyCmd(stack *string) *cobra.Command {
	var path bool
	var destinationStackName string

	cpCommand := &cobra.Command{
		Use:   "cp [key]",
		Short: "Copy config to another stack",
		Long: "Copies the config from the current stack to the destination stack. If `key` is omitted,\n" +
			"then all of the config from the current stack will be copied to the destination stack.",
		Args: cmdutil.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			project, _, err := ws.ReadProject()
			if err != nil {
				return err
			}

			// Get current stack and ensure that it is a different stack to the destination stack
			currentStack, err := cmdStack.RequireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				*stack,
				cmdStack.SetCurrent,
				opts,
			)
			if err != nil {
				return err
			}
			if currentStack.Ref().Name().String() == destinationStackName {
				return errors.New("current stack and destination stack are the same")
			}
			currentProjectStack, err := currentStack.Load(ctx, project, cmdStack.ConfigFile)
			if err != nil {
				return err
			}

			// Get the destination stack
			destinationStack, err := cmdStack.RequireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				destinationStackName,
				cmdStack.LoadOnly,
				opts,
			)
			if err != nil {
				return err
			}
			destinationProjectStack, err := destinationStack.Load(ctx, project, cmdStack.ConfigFile)
			if err != nil {
				return err
			}

			if _, isFileConfig := destinationStack.GetStackFilename(ctx); !isFileConfig {
				env := "<env>"
				imports := destinationProjectStack.Environment.Imports()
				if len(imports) == 1 {
					env = imports[0]
				}
				helpText := fmt.Sprintf("use `pulumi env set %s pulumiConfig.<key>`", env)
				return errors.New("config copy destination not supported for remote stack config: " + helpText)
			}

			ssml := cmdStack.NewStackSecretsManagerLoaderFromEnv()

			// Do we need to copy a single value or the entire map
			if len(args) > 0 {
				// A single key was specified so we only need to copy that specific value
				return copySingleConfigKey(
					ctx,
					ssml,
					args[0],
					path,
					currentStack,
					currentProjectStack,
					destinationStack,
					destinationProjectStack,
				)
			}

			requiresSaving, err := cmdStack.CopyEntireConfigMap(
				ctx,
				ssml,
				currentStack,
				currentProjectStack,
				destinationStack,
				destinationProjectStack,
			)
			if err != nil {
				return err
			}

			// The use of `requiresSaving` here ensures that there was actually some config
			// that needed saved, otherwise it's an unnecessary save call
			if requiresSaving {
				err := destinationStack.Save(ctx, destinationProjectStack)
				if err != nil {
					return err
				}
			}

			return nil
		},
	}

	cpCommand.PersistentFlags().BoolVar(
		&path, "path", false,
		"The key contains a path to a property in a map or list to set")
	cpCommand.PersistentFlags().StringVarP(
		&destinationStackName, "dest", "d", "",
		"The name of the new stack to copy the config to")

	return cpCommand
}

func newConfigGetCmd(stack *string) *cobra.Command {
	var jsonOut bool
	var open bool
	var path bool

	getCmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a single configuration value",
		Long: "Get a single configuration value.\n\n" +
			"The `--path` flag can be used to get a value inside a map or list:\n\n" +
			"  - `pulumi config get --path outer.inner` will get the value of the `inner` key, " +
			"if the value of `outer` is a map `inner: value`.\n" +
			"  - `pulumi config get --path 'names[0]'` will get the value of the first item, " +
			"if the value of `names` is a list.",
		Args: cmdutil.SpecificArgs([]string{"key"}),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			s, err := cmdStack.RequireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				*stack,
				cmdStack.OfferNew|cmdStack.SetCurrent,
				opts,
			)
			if err != nil {
				return err
			}

			key, err := ParseConfigKey(ws, args[0], path)
			if err != nil {
				return fmt.Errorf("invalid configuration key: %w", err)
			}

			ssml := cmdStack.NewStackSecretsManagerLoaderFromEnv()
			return getConfig(ctx, ssml, ws, s, key, path, jsonOut, open)
		},
	}
	getCmd.Flags().BoolVarP(
		&jsonOut, "json", "j", false,
		"Emit output as JSON")
	getCmd.Flags().BoolVar(
		&open, "open", true,
		"Open and resolve any environments listed in the stack configuration")
	getCmd.PersistentFlags().BoolVar(
		&path, "path", false,
		"The key contains a path to a property in a map or list to get")

	return getCmd
}

func newConfigRmCmd(stack *string) *cobra.Command {
	var path bool

	rmCmd := &cobra.Command{
		Use:   "rm <key>",
		Short: "Remove configuration value",
		Long: "Remove configuration value.\n\n" +
			"The `--path` flag can be used to remove a value inside a map or list:\n\n" +
			"  - `pulumi config rm --path outer.inner` will remove the `inner` key, " +
			"if the value of `outer` is a map `inner: value`.\n" +
			"  - `pulumi config rm --path 'names[0]'` will remove the first item, " +
			"if the value of `names` is a list.",
		Args: cmdutil.SpecificArgs([]string{"key"}),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			project, _, err := ws.ReadProject()
			if err != nil {
				return err
			}

			stack, err := cmdStack.RequireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				*stack,
				cmdStack.OfferNew|cmdStack.SetCurrent,
				opts,
			)
			if err != nil {
				return err
			}

			key, err := ParseConfigKey(ws, args[0], path)
			if err != nil {
				return fmt.Errorf("invalid configuration key: %w", err)
			}

			ps, err := stack.Load(ctx, project, cmdStack.ConfigFile)
			if err != nil {
				return err
			}

			if _, isFileConfig := stack.GetStackFilename(ctx); !isFileConfig {
				env := "<env>"
				imports := ps.Environment.Imports()
				if len(imports) == 1 {
					env = imports[0]
				}
				helpText := fmt.Sprintf("use `pulumi env rm %s pulumiConfig.%s`", env, key.String())
				return errors.New("config rm not supported for remote stack config: " + helpText)
			}

			err = ps.Config.Remove(key, path)
			if err != nil {
				return err
			}

			return stack.Save(ctx, ps)
		},
	}
	rmCmd.PersistentFlags().BoolVar(
		&path, "path", false,
		"The key contains a path to a property in a map or list to remove")

	return rmCmd
}

func newConfigRmAllCmd(stack *string) *cobra.Command {
	var path bool

	rmAllCmd := &cobra.Command{
		Use:   "rm-all <key1> <key2> <key3> ...",
		Short: "Remove multiple configuration values",
		Long: "Remove multiple configuration values.\n\n" +
			"The `--path` flag indicates that keys should be parsed within maps or lists:\n\n" +
			"  - `pulumi config rm-all --path  outer.inner 'foo[0]' key1` will remove the \n" +
			"    `inner` key of the `outer` map, the first key of the `foo` list and `key1`.\n" +
			"  - `pulumi config rm-all outer.inner 'foo[0]' key1` will remove the literal" +
			"    `outer.inner`, `foo[0]` and `key1` keys",
		Args: cmdutil.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			project, _, err := ws.ReadProject()
			if err != nil {
				return err
			}

			stack, err := cmdStack.RequireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				*stack,
				cmdStack.OfferNew,
				opts,
			)
			if err != nil {
				return err
			}

			ps, err := stack.Load(ctx, project, cmdStack.ConfigFile)
			if err != nil {
				return err
			}

			if _, isFileConfig := stack.GetStackFilename(ctx); !isFileConfig {
				env := "<env>"
				imports := ps.Environment.Imports()
				if len(imports) == 1 {
					env = imports[0]
				}
				helpText := fmt.Sprintf("use `pulumi env rm %s pulumiConfig.<key>`", env)
				return errors.New("config rm-all not supported for remote stack config: " + helpText)
			}

			for _, arg := range args {
				key, err := ParseConfigKey(ws, arg, path)
				if err != nil {
					return fmt.Errorf("invalid configuration key: %w", err)
				}

				err = ps.Config.Remove(key, path)
				if err != nil {
					return err
				}
			}

			return stack.Save(ctx, ps)
		},
	}
	rmAllCmd.PersistentFlags().BoolVar(
		&path, "path", false,
		"Parse the keys as paths in a map or list rather than raw strings")

	return rmAllCmd
}

func newConfigRefreshCmd(stk *string) *cobra.Command {
	var force bool
	refreshCmd := &cobra.Command{
		Use:   "refresh",
		Short: "Update the local configuration based on the most recent deployment of the stack",
		Args:  cmdutil.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			project, _, err := ws.ReadProject()
			if err != nil {
				return err
			}

			// Ensure the stack exists.
			s, err := cmdStack.RequireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				*stk,
				cmdStack.LoadOnly,
				opts,
			)
			if err != nil {
				return err
			}

			configPath, isFile := s.GetStackFilename(ctx)
			if !isFile {
				return errors.New("config refresh not supported for remote stack config")
			}

			c, err := backend.GetLatestConfiguration(ctx, s)
			if err != nil {
				return err
			}

			ps, err := s.Load(ctx, project, cmdStack.ConfigFile)
			if err != nil {
				return err
			}

			ps.Config = c
			// Also restore the secrets provider from state
			untypedDeployment, err := s.ExportDeployment(ctx)
			if err != nil {
				return fmt.Errorf("getting deployment: %w", err)
			}
			deployment, err := stack.UnmarshalUntypedDeployment(ctx, untypedDeployment)
			if err != nil {
				return fmt.Errorf("unmarshaling deployment: %w", err)
			}
			if deployment.SecretsProviders != nil {
				// TODO: It would be really nice if the format of secrets state in the config file matched
				// what we kept in the statefile. That would go well with the pluginification of secret
				// providers as well, but for now just switch on the secret provider type and ask it to fill in
				// the config file for us.
				switch deployment.SecretsProviders.Type {
				case passphrase.Type:
					err = passphrase.EditProjectStack(ps, deployment.SecretsProviders.State)
				case cloud.Type:
					err = cloud.EditProjectStack(ps, deployment.SecretsProviders.State)
				default:
					// Anything else assume we can just clear all the secret bits
					ps.EncryptionSalt = ""
					ps.SecretsProvider = ""
					ps.EncryptedKey = ""
				}

				if err != nil {
					return err
				}
			}

			// If the configuration file doesn't exist, or force has been passed, save it in place.
			if _, err = os.Stat(configPath); os.IsNotExist(err) || force {
				return ps.Save(configPath)
			}

			// Otherwise we'll create a backup, let's figure out what name to use by adding ".bak" over and over
			// until we get to a name not in use.
			backupFile := configPath + ".bak"
			for {
				_, err = os.Stat(backupFile)
				if os.IsNotExist(err) {
					if err = os.Rename(configPath, backupFile); err != nil {
						return fmt.Errorf("backing up existing configuration file: %w", err)
					}

					fmt.Printf("backed up existing configuration file to %s\n", backupFile)
					break
				} else if err != nil {
					return fmt.Errorf("backing up existing configuration file: %w", err)
				}

				backupFile = backupFile + ".bak"
			}

			err = ps.Save(configPath)
			if err == nil {
				fmt.Printf("refreshed configuration for stack '%s'\n", s.Ref().Name())
			}
			return err
		},
	}
	refreshCmd.PersistentFlags().BoolVarP(
		&force, "force", "f", false, "Overwrite configuration file, if it exists, without creating a backup")

	return refreshCmd
}

type configSetCmd struct {
	Stdin *os.File

	Plaintext bool
	Secret    bool
	Path      bool
	Type      string
}

func newConfigSetCmd(stack *string) *cobra.Command {
	configSetCmd := &configSetCmd{}

	setCmd := &cobra.Command{
		Use:   "set <key> [value]",
		Short: "Set configuration value",
		Long: "Configuration values can be accessed when a stack is being deployed and used to configure behavior. \n" +
			"If a value is not present on the command line, pulumi will prompt for the value. Multi-line values\n" +
			"may be set by piping a file to standard in.\n\n" +
			"The `--path` flag can be used to set a value inside a map or list:\n\n" +
			"  - `pulumi config set --path 'names[0]' a` " +
			"will set the value to a list with the first item `a`.\n" +
			"  - `pulumi config set --path parent.nested value` " +
			"will set the value of `parent` to a map `nested: value`.\n" +
			"  - `pulumi config set --path '[\"parent.name\"][\"nested.name\"]' value` will set the value of \n" +
			"    `parent.name` to a map `nested.name: value`.\n\n" +
			"When setting the config for a path, \"true\" and \"false\" are treated as boolean values, and\n" +
			"integers are treated as numbers. All other values are treated as strings.  Top level entries\n" +
			"are always treated as strings.",
		Args: cmdutil.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			project, _, err := ws.ReadProject()
			if err != nil {
				return err
			}

			// Ensure the stack exists.
			s, err := cmdStack.RequireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				*stack,
				cmdStack.OfferNew|cmdStack.SetCurrent,
				opts,
			)
			if err != nil {
				return err
			}

			return configSetCmd.Run(ctx, args, project, s)
		},
	}

	setCmd.PersistentFlags().BoolVar(
		&configSetCmd.Path, "path", false,
		"The key contains a path to a property in a map or list to set")
	setCmd.PersistentFlags().BoolVar(
		&configSetCmd.Plaintext, "plaintext", false,
		"Save the value as plaintext (unencrypted)")
	setCmd.PersistentFlags().BoolVar(
		&configSetCmd.Secret, "secret", false,
		"Encrypt the value instead of storing it in plaintext")
	setCmd.PersistentFlags().StringVar(
		&configSetCmd.Type, "type", "", "Save the value as the given type.  Allowed values are string, bool, int, and float")
	setCmd.MarkFlagsMutuallyExclusive("secret", "plaintext", "type")

	return setCmd
}

func (c *configSetCmd) Run(ctx context.Context, args []string, project *workspace.Project, s backend.Stack) error {
	stdin := c.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	key, err := ParseConfigKey(pkgWorkspace.Instance, args[0], c.Path)
	if err != nil {
		return fmt.Errorf("invalid configuration key: %w", err)
	}

	var value string
	switch {
	case len(args) == 2:
		value = args[1]
	//nolint:gosec // os.Stdin.Fd() == 0: uintptr -> int conversion is always safe
	case !term.IsTerminal(int(stdin.Fd())):
		b, readerr := io.ReadAll(stdin)
		if readerr != nil {
			return readerr
		}
		value = cmdutil.RemoveTrailingNewline(string(b))
	case !cmdutil.Interactive():
		return errors.New("config value must be specified in non-interactive mode")
	case c.Secret:
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

	ps, err := s.Load(ctx, project, cmdStack.ConfigFile)
	if err != nil {
		return err
	}

	ssml := cmdStack.NewStackSecretsManagerLoaderFromEnv()

	// Encrypt the config value if needed.
	var v config.Value
	if c.Secret {
		// We're always going to save, so can ignore the bool for if getStackEncrypter changed the
		// config data.
		c, _, cerr := ssml.GetEncrypter(ctx, s, ps)
		if cerr != nil {
			return cerr
		}
		enc, eerr := c.EncryptValue(ctx, value)
		if eerr != nil {
			return eerr
		}
		v = config.NewSecureValue(enc)
	} else {
		var t config.Type
		switch c.Type {
		case "":
			t = config.TypeUnknown
		case "string":
			t = config.TypeString
		case "int":
			t = config.TypeInt
		case "bool":
			t = config.TypeBool
		case "float":
			t = config.TypeFloat
		default:
			return fmt.Errorf("invalid type %q; must be one of string, int, bool, or float", c.Type)
		}
		v = config.NewTypedValue(value, t)

		// If we saved a plaintext configuration value, and --plaintext was not passed, warn the user.
		if !c.Plaintext && looksLikeSecret(key, value) {
			return fmt.Errorf("config value for '%s' looks like a secret; "+
				"rerun with --secret to encrypt it, or --plaintext if you meant to store in plaintext",
				key)
		}
	}

	if _, isFileConfig := s.GetStackFilename(ctx); !isFileConfig {
		exampleValue := "--secret <value>"
		if !c.Secret {
			exampleValue = value
		}
		env := "<env>"
		imports := ps.Environment.Imports()
		if len(imports) == 1 {
			env = imports[0]
		}
		helpText := fmt.Sprintf("use `pulumi env set %s pulumiConfig.%s %s`", env, key.String(), exampleValue)
		return errors.New("config set not supported for remote stack config: " + helpText)
	}

	err = ps.Config.Set(key, v, c.Path)
	if err != nil {
		return fmt.Errorf("could not set config: %w", err)
	}

	return s.Save(ctx, ps)
}

func newConfigSetAllCmd(stack *string) *cobra.Command {
	var plaintextArgs []string
	var secretArgs []string
	var path bool

	setCmd := &cobra.Command{
		Use:   "set-all --plaintext key1=value1 --plaintext key2=value2 --secret key3=value3",
		Short: "Set multiple configuration values",
		Long: "pulumi set-all allows you to set multiple configuration values in one command.\n\n" +
			"Each key-value pair must be preceded by either the `--secret` or the `--plaintext` flag to denote whether \n" +
			"it should be encrypted:\n\n" +
			"  - `pulumi config set-all --secret key1=value1 --plaintext key2=value --secret key3=value3`\n\n" +
			"The `--path` flag can be used to set values inside a map or list:\n\n" +
			"  - `pulumi config set-all --path --plaintext \"names[0]\"=a --plaintext \"names[1]\"=b` \n" +
			"    will set the value to a list with the first item `a` and second item `b`.\n" +
			"  - `pulumi config set-all --path --plaintext parent.nested=value --plaintext parent.other=value2` \n" +
			"    will set the value of `parent` to a map `{nested: value, other: value2}`.\n" +
			"  - `pulumi config set-all --path --plaintext '[\"parent.name\"].[\"nested.name\"]'=value` will set the \n" +
			"    value of `parent.name` to a map `nested.name: value`.",
		Args: cmdutil.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			project, _, err := ws.ReadProject()
			if err != nil {
				return err
			}

			// Ensure the stack exists.
			stack, err := cmdStack.RequireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				*stack,
				cmdStack.OfferNew,
				opts,
			)
			if err != nil {
				return err
			}

			ps, err := stack.Load(ctx, project, cmdStack.ConfigFile)
			if err != nil {
				return err
			}

			for _, ptArg := range plaintextArgs {
				key, value, err := parseKeyValuePair(ptArg, path)
				if err != nil {
					return err
				}
				v := config.NewValue(value)

				err = ps.Config.Set(key, v, path)
				if err != nil {
					return err
				}
			}

			ssml := cmdStack.NewStackSecretsManagerLoaderFromEnv()

			for _, sArg := range secretArgs {
				key, value, err := parseKeyValuePair(sArg, path)
				if err != nil {
					return err
				}
				// We're always going to save, so can ignore the bool for if getStackEncrypter changed the
				// config data.
				c, _, cerr := ssml.GetEncrypter(ctx, stack, ps)
				if cerr != nil {
					return cerr
				}
				enc, eerr := c.EncryptValue(ctx, value)
				if eerr != nil {
					return eerr
				}
				v := config.NewSecureValue(enc)

				err = ps.Config.Set(key, v, path)
				if err != nil {
					return err
				}
			}

			return stack.Save(ctx, ps)
		},
	}

	setCmd.PersistentFlags().BoolVar(
		&path, "path", false,
		"Parse the keys as paths in a map or list rather than raw strings")
	setCmd.PersistentFlags().StringArrayVar(
		&plaintextArgs, "plaintext", []string{},
		"Marks a value as plaintext (unencrypted)")
	setCmd.PersistentFlags().StringArrayVar(
		&secretArgs, "secret", []string{},
		"Marks a value as secret to be encrypted")

	return setCmd
}

// configValueJSON is the shape of the --json output for a configuration value.  While we can add fields to this
// structure in the future, we should not change existing fields.
type configValueJSON struct {
	// When the value is encrypted and --show-secrets was not passed, the value will not be set.
	// If the value is an object, ObjectValue will be set.
	Value       *string     `json:"value,omitempty"`
	ObjectValue interface{} `json:"objectValue,omitempty"`
	Secret      bool        `json:"secret"`
}

func listConfig(
	ctx context.Context,
	ssml cmdStack.SecretsManagerLoader,
	stdout io.Writer,
	project *workspace.Project,
	stack backend.Stack,
	ps *workspace.ProjectStack,
	showSecrets bool,
	jsonOut bool,
	openEnvironment bool,
) error {
	var env *esc.Environment
	var diags []apitype.EnvironmentDiagnostic
	var err error
	if openEnvironment {
		env, diags, err = openStackEnv(ctx, stack, ps)
	} else {
		env, diags, err = checkStackEnv(ctx, stack, ps)
	}
	if err != nil {
		return err
	}

	var pulumiEnv esc.Value
	var envCrypter config.Encrypter
	if env != nil {
		pulumiEnv = env.Properties["pulumiConfig"]

		stackEncrypter, state, err := ssml.GetEncrypter(ctx, stack, ps)
		if err != nil {
			return err
		}
		// This may have setup the stack's secrets provider, so save the stack if needed.
		if state != cmdStack.SecretsManagerUnchanged {
			if err = stack.Save(ctx, ps); err != nil {
				return fmt.Errorf("save stack config: %w", err)
			}
		}
		envCrypter = stackEncrypter
	}

	stackName := stack.Ref().Name().String()

	cfg, err := ps.Config.Copy(config.NopDecrypter, config.NopEncrypter)
	if err != nil {
		return fmt.Errorf("copying config: %w", err)
	}

	// when listing configuration values
	// also show values coming from the project and environment
	err = workspace.ApplyProjectConfig(ctx, stackName, project, pulumiEnv, cfg, envCrypter)
	if err != nil {
		return err
	}

	// By default, we will use a blinding decrypter to show "[secret]". If requested, display secrets in plaintext.
	decrypter := config.NewBlindingDecrypter()
	if cfg.HasSecureValue() && showSecrets {
		stackDecrypter, state, err := ssml.GetDecrypter(ctx, stack, ps)
		if err != nil {
			return err
		}
		// This may have setup the stack's secrets provider, so save the stack if needed.
		if state != cmdStack.SecretsManagerUnchanged {
			if err = stack.Save(ctx, ps); err != nil {
				return fmt.Errorf("save stack config: %w", err)
			}
		}
		decrypter = stackDecrypter
	}

	var keys config.KeyArray
	for key := range cfg {
		// Note that we use the fully qualified module member here instead of a `prettyKey`, this lets us ensure
		// that all the config values for the current program are displayed next to one another in the output.
		keys = append(keys, key)
	}
	sort.Sort(keys)

	if jsonOut {
		configValues := make(map[string]configValueJSON)
		for _, key := range keys {
			entry := configValueJSON{
				Secret: cfg[key].Secure(),
			}

			decrypted, err := cfg[key].Value(decrypter)
			if err != nil {
				return fmt.Errorf("could not decrypt configuration value: %w", err)
			}
			entry.Value = &decrypted

			if cfg[key].Object() {
				var obj interface{}
				if err := json.Unmarshal([]byte(decrypted), &obj); err != nil {
					return err
				}
				entry.ObjectValue = obj
			}

			// If the value was a secret value and we aren't showing secrets, then the above would have set value
			// to "[secret]" which is reasonable when printing for human display, but for our JSON output, we'd rather
			// just elide the value.
			if cfg[key].Secure() && !showSecrets {
				entry.Value = nil
				entry.ObjectValue = nil
			}

			configValues[key.String()] = entry
		}
		err := ui.FprintJSON(stdout, configValues)
		if err != nil {
			return err
		}
	} else {
		rows := []cmdutil.TableRow{}
		for _, key := range keys {
			decrypted, err := cfg[key].Value(decrypter)
			if err != nil {
				return fmt.Errorf("could not decrypt configuration value: %w", err)
			}

			rows = append(rows, cmdutil.TableRow{Columns: []string{PrettyKey(key), decrypted}})
		}

		ui.FprintTable(stdout, cmdutil.Table{
			Headers: []string{"KEY", "VALUE"},
			Rows:    rows,
		}, nil)

		if env != nil {
			_, environ, _, err := cli.PrepareEnvironment(env, &cli.PrepareOptions{
				Pretend: !openEnvironment,
				Redact:  !showSecrets,
			})
			if err != nil {
				return err
			}

			if len(environ) != 0 {
				environRows := make([]cmdutil.TableRow, len(environ))
				for i, kvp := range environ {
					key, value, _ := strings.Cut(kvp, "=")
					environRows[i] = cmdutil.TableRow{Columns: []string{key, value}}
				}

				fmt.Fprintln(stdout)
				ui.FprintTable(stdout, cmdutil.Table{
					Headers: []string{"ENVIRONMENT VARIABLE", "VALUE"},
					Rows:    environRows,
				}, nil)
			}

			if len(diags) != 0 {
				fmt.Fprintln(stdout)
				fmt.Fprintln(stdout, "Environment diagnostics:")
				printESCDiagnostics(stdout, diags)
			}

			warnOnNoEnvironmentEffects(stdout, env)
		}
	}

	if showSecrets {
		cmdStack.Log3rdPartySecretsProviderDecryptionEvent(ctx, stack, "", "pulumi config")
	}

	return nil
}

func getConfig(
	ctx context.Context,
	ssml cmdStack.SecretsManagerLoader,
	ws pkgWorkspace.Context,
	stack backend.Stack,
	key config.Key,
	path, jsonOut,
	openEnvironment bool,
) error {
	project, _, err := ws.ReadProject()
	if err != nil {
		return err
	}
	ps, err := stack.Load(ctx, project, cmdStack.ConfigFile)
	if err != nil {
		return err
	}

	var env *esc.Environment
	var diags []apitype.EnvironmentDiagnostic
	if openEnvironment {
		env, diags, err = openStackEnv(ctx, stack, ps)
	} else {
		env, diags, err = checkStackEnv(ctx, stack, ps)
	}
	if err != nil {
		return err
	}

	var pulumiEnv esc.Value
	var envCrypter config.Encrypter
	if env != nil {
		pulumiEnv = env.Properties["pulumiConfig"]

		stackEncrypter, state, err := ssml.GetEncrypter(ctx, stack, ps)
		if err != nil {
			return err
		}
		// This may have setup the stack's secrets provider, so save the stack if needed.
		if state != cmdStack.SecretsManagerUnchanged {
			if err = stack.Save(ctx, ps); err != nil {
				return fmt.Errorf("save stack config: %w", err)
			}
		}
		envCrypter = stackEncrypter
	}

	stackName := stack.Ref().Name().String()

	cfg, err := ps.Config.Copy(config.NopDecrypter, config.NopEncrypter)
	if err != nil {
		return fmt.Errorf("copying config: %w", err)
	}

	// when asking for a configuration value, include values from the project and environment
	err = workspace.ApplyProjectConfig(ctx, stackName, project, pulumiEnv, cfg, envCrypter)
	if err != nil {
		return err
	}

	v, ok, err := cfg.Get(key, path)
	if err != nil {
		return err
	}
	if ok {
		var d config.Decrypter
		if v.Secure() {
			var err error
			var state cmdStack.SecretsManagerState
			if d, state, err = ssml.GetDecrypter(ctx, stack, ps); err != nil {
				return fmt.Errorf("could not create a decrypter: %w", err)
			}
			// This may have setup the stack's secrets provider, so save the stack if needed.
			if state != cmdStack.SecretsManagerUnchanged {
				if err = stack.Save(ctx, ps); err != nil {
					return fmt.Errorf("save stack config: %w", err)
				}
			}
		} else {
			d = config.NewPanicCrypter()
		}
		raw, err := v.Value(d)
		if err != nil {
			return fmt.Errorf("could not decrypt configuration value: %w", err)
		}

		if jsonOut {
			value := configValueJSON{
				Value:  &raw,
				Secret: v.Secure(),
			}

			if v.Object() {
				var obj interface{}
				if err := json.Unmarshal([]byte(raw), &obj); err != nil {
					return err
				}
				value.ObjectValue = obj
			}

			out, err := json.MarshalIndent(value, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(out))
		} else {
			fmt.Printf("%v\n", raw)
		}

		if len(diags) != 0 {
			fmt.Println()
			fmt.Println("Environment diagnostics:")
			printESCDiagnostics(os.Stdout, diags)
		}

		cmdStack.Log3rdPartySecretsProviderDecryptionEvent(ctx, stack, key.Name(), "")

		return nil
	}

	return fmt.Errorf("configuration key '%s' not found for stack '%s'", PrettyKey(key), stack.Ref())
}

// keyPattern is the regular expression a configuration key must match before we check (and error) if we think
// it is a password
var keyPattern = regexp.MustCompile("(?i)passwd|pass|password|pwd|secret|token")

const (
	// maxEntropyCheckLength is the maximum length of a possible secret for entropy checking.
	maxEntropyCheckLength = 16
	// entropyThreshold is the total entropy threshold a potential secret needs to pass before being flagged.
	entropyThreshold = 80.0
	// entropyCharThreshold is the per-char entropy threshold a potential secret needs to pass before being flagged.
	entropyPerCharThreshold = 3.0
)

// looksLikeSecret returns true if a configuration value "looks" like a secret. This is always going to be a heuristic
// that suffers from false positives, but is better (a) than our prior approach of unconditionally printing a warning
// for all plaintext values, and (b)  to be paranoid about such things. Inspired by the gas linter and securego project.
func looksLikeSecret(k config.Key, v string) bool {
	if !keyPattern.MatchString(k.Name()) {
		return false
	}

	if len(v) > maxEntropyCheckLength {
		v = v[:maxEntropyCheckLength]
	}

	// Compute the strength use the resulting entropy to flag whether this looks like a secret.
	info := zxcvbn.PasswordStrength(v, nil)
	entropyPerChar := info.Entropy / float64(len(v))
	return info.Entropy >= entropyThreshold ||
		(info.Entropy >= (entropyThreshold/2) && entropyPerChar >= entropyPerCharThreshold)
}

func checkStackEnv(
	ctx context.Context,
	stack backend.Stack,
	workspaceStack *workspace.ProjectStack,
) (*esc.Environment, []apitype.EnvironmentDiagnostic, error) {
	yaml := workspaceStack.EnvironmentBytes()
	if len(yaml) == 0 {
		return nil, nil, nil
	}

	envs, ok := stack.Backend().(backend.EnvironmentsBackend)
	if !ok {
		return nil, nil, fmt.Errorf("backend %v does not support environments", stack.Backend().Name())
	}
	orgNamer, ok := stack.(interface{ OrgName() string })
	if !ok {
		return nil, nil, fmt.Errorf("cannot determine organzation for stack %v", stack.Ref())
	}
	orgName := orgNamer.OrgName()

	return envs.CheckYAMLEnvironment(ctx, orgName, yaml)
}

func warnOnNoEnvironmentEffects(out io.Writer, env *esc.Environment) {
	hasEnvVars := len(env.GetEnvironmentVariables()) != 0
	hasFiles := len(env.GetTemporaryFiles()) != 0
	_, hasPulumiConfig := env.Properties["pulumiConfig"].Value.(map[string]esc.Value)

	//nolint:lll
	if !hasEnvVars && !hasFiles && !hasPulumiConfig {
		color := cmdutil.GetGlobalColorization()
		fmt.Fprintln(out, color.Colorize(colors.SpecWarning+"The stack's environment does not define the `environmentVariables`, `files`, or `pulumiConfig` properties."))
		fmt.Fprintln(out, color.Colorize(colors.SpecWarning+"Without at least one of these properties, the environment will not affect the stack's behavior."+colors.Reset))
		fmt.Fprintln(out)
	}
}

func printESCDiagnostics(out io.Writer, diags []apitype.EnvironmentDiagnostic) {
	for _, d := range diags {
		if d.Range != nil {
			fmt.Fprintf(out, "%v:", d.Range.Environment)
			if d.Range.Begin.Line != 0 {
				fmt.Fprintf(out, "%v:%v:", d.Range.Begin.Line, d.Range.Begin.Column)
			}
			fmt.Fprintf(out, " ")
		}
		fmt.Fprintf(out, "%v\n", d.Summary)
	}
}
