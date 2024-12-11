// Copyright 2016-2024, Pulumi Corporation.
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
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/nbutton23/zxcvbn-go"
	"github.com/spf13/cobra"

	"github.com/pulumi/esc"
	"github.com/pulumi/esc/cmd/esc/cli"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
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
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
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

			ps, err := cmdStack.LoadProjectStack(project, stack)
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
		}),
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

	cmd.AddCommand(newConfigGetCmd(&cmdStack.ConfigFile, &stack))
	cmd.AddCommand(newConfigRmCmd(&stack))
	cmd.AddCommand(newConfigRmAllCmd(&stack))
	cmd.AddCommand(newConfigSetCmd(&cmdStack.ConfigFile, &stack))
	cmd.AddCommand(newConfigSetAllCmd(&stack))
	cmd.AddCommand(newConfigRefreshCmd(&stack))
	cmd.AddCommand(newConfigCopyCmd(&stack))
	cmd.AddCommand(newConfigEnvCmd(&stack))

	return cmd
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
			if err = cmdStack.SaveProjectStack(stack, ps); err != nil {
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
			if err = cmdStack.SaveProjectStack(stack, ps); err != nil {
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
