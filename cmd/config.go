// Copyright 2016-2018, Pulumi Corporation.
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

package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"sort"
	"strings"

	zxcvbn "github.com/nbutton23/zxcvbn-go"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/secrets"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func newConfigCmd() *cobra.Command {
	var stack string
	var showSecrets bool
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long: "Lists all configuration values for a specific stack. To add a new configuration value, run\n" +
			"'pulumi config set'. To remove and existing value run 'pulumi config rm'. To get the value of\n" +
			"for a specific configuration key, use 'pulumi config get <key-name>'.",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			stack, err := requireStack(stack, true, opts, true /*setCurrent*/)
			if err != nil {
				return err
			}

			return listConfig(stack, showSecrets, jsonOut)
		}),
	}

	cmd.Flags().BoolVar(
		&showSecrets, "show-secrets", false,
		"Show secret values when listing config instead of displaying blinded values")
	cmd.Flags().BoolVarP(
		&jsonOut, "json", "j", false,
		"Emit output as JSON")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().StringVar(
		&stackConfigFile, "config-file", "",
		"Use the configuration values in the specified file rather than detecting the file name")

	cmd.AddCommand(newConfigGetCmd(&stack))
	cmd.AddCommand(newConfigRmCmd(&stack))
	cmd.AddCommand(newConfigSetCmd(&stack))
	cmd.AddCommand(newConfigRefreshCmd(&stack))

	return cmd
}

func newConfigGetCmd(stack *string) *cobra.Command {
	var jsonOut bool
	var path bool

	getCmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a single configuration value",
		Long: "Get a single configuration value.\n\n" +
			"The `--path` flag can be used to get a value inside a map or list:\n\n" +
			"    - `pulumi config get --path outer.inner` will get the value of the `inner` key, " +
			"if the value of `outer` is a map `inner: value`.\n" +
			"    - `pulumi config get --path names[0]` will get the value of the first item, " +
			"if the value of `names` is a list.",
		Args: cmdutil.SpecificArgs([]string{"key"}),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			s, err := requireStack(*stack, true, opts, true /*setCurrent*/)
			if err != nil {
				return err
			}

			key, err := parseConfigKey(args[0])
			if err != nil {
				return errors.Wrap(err, "invalid configuration key")
			}

			return getConfig(s, key, path, jsonOut)
		}),
	}
	getCmd.Flags().BoolVarP(
		&jsonOut, "json", "j", false,
		"Emit output as JSON")
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
			"    - `pulumi config rm --path outer.inner` will remove the `inner` key, " +
			"if the value of `outer` is a map `inner: value`.\n" +
			"    - `pulumi config rm --path names[0]` will remove the first item, " +
			"if the value of `names` is a list.",
		Args: cmdutil.SpecificArgs([]string{"key"}),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			s, err := requireStack(*stack, true, opts, true /*setCurrent*/)
			if err != nil {
				return err
			}

			key, err := parseConfigKey(args[0])
			if err != nil {
				return errors.Wrap(err, "invalid configuration key")
			}

			ps, err := loadProjectStack(s)
			if err != nil {
				return err
			}

			err = ps.Config.Remove(key, path)
			if err != nil {
				return err
			}

			return saveProjectStack(s, ps)
		}),
	}
	rmCmd.PersistentFlags().BoolVar(
		&path, "path", false,
		"The key contains a path to a property in a map or list to remove")

	return rmCmd
}

func newConfigRefreshCmd(stack *string) *cobra.Command {
	var force bool
	refreshCmd := &cobra.Command{
		Use:   "refresh",
		Short: "Update the local configuration based on the most recent deployment of the stack",
		Args:  cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// Ensure the stack exists.
			s, err := requireStack(*stack, false, opts, true /*setCurrent*/)
			if err != nil {
				return err
			}

			c, err := backend.GetLatestConfiguration(commandContext(), s)
			if err != nil {
				return err
			}

			configPath, err := getProjectStackPath(s)
			if err != nil {
				return err
			}

			ps, err := workspace.LoadProjectStack(configPath)
			if err != nil {
				return err
			}

			ps.Config = c

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
						return errors.Wrap(err, "backing up existing configuration file")
					}

					fmt.Printf("backed up existing configuration file to %s\n", backupFile)
					break
				} else if err != nil {
					return errors.Wrap(err, "backing up existing configuration file")
				}

				backupFile = backupFile + ".bak"
			}

			err = ps.Save(configPath)
			if err == nil {
				fmt.Printf("refreshed configuration for stack '%s'\n", s.Ref().Name())
			}
			return err
		}),
	}
	refreshCmd.PersistentFlags().BoolVarP(
		&force, "force", "f", false, "Overwrite configuration file, if it exists, without creating a backup")

	return refreshCmd
}

func newConfigSetCmd(stack *string) *cobra.Command {
	var plaintext bool
	var secret bool
	var path bool

	setCmd := &cobra.Command{
		Use:   "set <key> [value]",
		Short: "Set configuration value",
		Long: "Configuration values can be accessed when a stack is being deployed and used to configure behavior. \n" +
			"If a value is not present on the command line, pulumi will prompt for the value. Multi-line values\n" +
			"may be set by piping a file to standard in.\n\n" +
			"The `--path` flag can be used to set a value inside a map or list:\n\n" +
			"    - `pulumi config set --path outer.inner value` " +
			"will set the value of `outer` to a map `inner: value`.\n" +
			"    - `pulumi config set --path names[0] a` " +
			"will set the value to a list with the first item `a`.",
		Args: cmdutil.RangeArgs(1, 2),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// Ensure the stack exists.
			s, err := requireStack(*stack, true, opts, true /*setCurrent*/)
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
				value = cmdutil.RemoveTrailingNewline(string(b))
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
				c, cerr := getStackEncrypter(s)
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

				// If we saved a plaintext configuration value, and --plaintext was not passed, warn the user.
				if !plaintext && looksLikeSecret(key, value) {
					return errors.Errorf(
						"config value '%s' looks like a secret; "+
							"rerun with --secret to encrypt it, or --plaintext if you meant to store in plaintext",
						value)
				}
			}

			ps, err := loadProjectStack(s)
			if err != nil {
				return err
			}

			err = ps.Config.Set(key, v, path)
			if err != nil {
				return err
			}

			return saveProjectStack(s, ps)
		}),
	}

	setCmd.PersistentFlags().BoolVar(
		&path, "path", false,
		"The key contains a path to a property in a map or list to set")
	setCmd.PersistentFlags().BoolVar(
		&plaintext, "plaintext", false,
		"Save the value as plaintext (unencrypted)")
	setCmd.PersistentFlags().BoolVar(
		&secret, "secret", false,
		"Encrypt the value instead of storing it in plaintext")

	return setCmd
}

var stackConfigFile string

func getProjectStackPath(stack backend.Stack) (string, error) {
	if stackConfigFile == "" {
		return workspace.DetectProjectStackPath(stack.Ref().Name())
	}
	return stackConfigFile, nil
}

func loadProjectStack(stack backend.Stack) (*workspace.ProjectStack, error) {
	if stackConfigFile == "" {
		return workspace.DetectProjectStack(stack.Ref().Name())
	}
	return workspace.LoadProjectStack(stackConfigFile)
}

func saveProjectStack(stack backend.Stack, ps *workspace.ProjectStack) error {
	if stackConfigFile == "" {
		return workspace.SaveProjectStack(stack.Ref().Name(), ps)
	}
	return ps.Save(stackConfigFile)
}

func parseConfigKey(key string) (config.Key, error) {
	// As a convience, we'll treat any key with no delimiter as if:
	// <program-name>:<key> had been written instead
	if !strings.Contains(key, tokens.TokenDelimiter) {
		proj, err := workspace.DetectProject()
		if err != nil {
			return config.Key{}, err
		}

		return config.ParseKey(fmt.Sprintf("%s:%s", proj.Name, key))
	}

	return config.ParseKey(key)
}

func prettyKey(k config.Key) string {
	proj, err := workspace.DetectProject()
	if err != nil {
		return fmt.Sprintf("%s:%s", k.Namespace(), k.Name())
	}

	return prettyKeyForProject(k, proj)
}

func prettyKeyForProject(k config.Key, proj *workspace.Project) string {
	if k.Namespace() == string(proj.Name) {
		return k.Name()
	}

	return fmt.Sprintf("%s:%s", k.Namespace(), k.Name())
}

// configValueJSON is the shape of the --json output for a configuration value.  While we can add fields to this
// structure in the future, we should not change existing fields.
type configValueJSON struct {
	// When the value is encrypted and --show-secrets was not passed, the value will not be set.
	Value       *string     `json:"value,omitempty"`
	Secret      bool        `json:"secret"`
	Object      bool        `json:"object"`
	ObjectValue interface{} `json:"objectValue,omitempty"`
}

func listConfig(stack backend.Stack, showSecrets bool, jsonOut bool) error {
	ps, err := loadProjectStack(stack)
	if err != nil {
		return err
	}

	cfg := ps.Config

	// By default, we will use a blinding decrypter to show "[secret]". If requested, display secrets in plaintext.
	decrypter := config.NewBlindingDecrypter()
	if cfg.HasSecureValue() && showSecrets {
		dec, decerr := getStackDencrypter(stack)
		if decerr != nil {
			return decerr
		}
		decrypter = dec
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
				Object: cfg[key].Object(),
			}

			decrypted, err := cfg[key].Value(decrypter)
			if err != nil {
				return errors.Wrap(err, "could not decrypt configuration value")
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
		out, err := json.MarshalIndent(configValues, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
	} else {
		rows := []cmdutil.TableRow{}
		for _, key := range keys {
			decrypted, err := cfg[key].Value(decrypter)
			if err != nil {
				return errors.Wrap(err, "could not decrypt configuration value")
			}

			rows = append(rows, cmdutil.TableRow{Columns: []string{prettyKey(key), decrypted}})
		}

		cmdutil.PrintTable(cmdutil.Table{
			Headers: []string{"KEY", "VALUE"},
			Rows:    rows,
		})
	}

	return nil
}

func getConfig(stack backend.Stack, key config.Key, path, jsonOut bool) error {
	ps, err := loadProjectStack(stack)
	if err != nil {
		return err
	}

	cfg := ps.Config

	v, ok, err := cfg.Get(key, path)
	if err != nil {
		return err
	}
	if ok {
		var d config.Decrypter
		if v.Secure() {
			var err error
			if d, err = getStackDencrypter(stack); err != nil {
				return errors.Wrap(err, "could not create a decrypter")
			}
		} else {
			d = config.NewPanicCrypter()
		}
		raw, err := v.Value(d)
		if err != nil {
			return errors.Wrap(err, "could not decrypt configuration value")
		}

		if jsonOut {
			value := configValueJSON{
				Value:  &raw,
				Secret: v.Secure(),
				Object: v.Object(),
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

		return nil
	}

	return errors.Errorf(
		"configuration key '%s' not found for stack '%s'", prettyKey(key), stack.Ref())
}

var (
	// keyPattern is the regular expression a configuration key must match before we check (and error) if we think
	// it is a password
	keyPattern = regexp.MustCompile("(?i)passwd|pass|password|pwd|secret|token")
)

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
	return (info.Entropy >= entropyThreshold ||
		(info.Entropy >= (entropyThreshold/2) && entropyPerChar >= entropyPerCharThreshold))
}

// getStackConfiguration loads configuration information for a given stack. If stackConfigFile is non empty,
// it is uses instead of the default configuration file for the stack
func getStackConfiguration(stack backend.Stack, sm secrets.Manager) (backend.StackConfiguration, error) {
	workspaceStack, err := loadProjectStack(stack)
	if err != nil {
		return backend.StackConfiguration{}, errors.Wrap(err, "loading stack configuration")
	}

	// If there are no secrets in the configuration, we should never use the decrypter, so it is safe to return
	// one which panics if it is used. This provides for some nice UX in the common case (since, for example, building
	// the correct decrypter for the local backend would involve prompting for a passphrase)
	if !workspaceStack.Config.HasSecureValue() {
		return backend.StackConfiguration{
			Config:    workspaceStack.Config,
			Decrypter: config.NewPanicCrypter(),
		}, nil
	}

	crypter, err := sm.Decrypter()
	if err != nil {
		return backend.StackConfiguration{}, errors.Wrap(err, "getting configuration decrypter")
	}

	return backend.StackConfiguration{
		Config:    workspaceStack.Config,
		Decrypter: crypter,
	}, nil
}
