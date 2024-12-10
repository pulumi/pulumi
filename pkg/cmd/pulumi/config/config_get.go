// Copyright 2024, Pulumi Corporation.
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

	"github.com/spf13/cobra"

	"github.com/pulumi/esc"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type configGetCmd struct {
	// Parsed arguments to the command.
	Args *configGetArgs

	// The command's standard output.
	Stdout io.Writer

	// The workspace to operate on.
	Workspace pkgWorkspace.Context
	// The login manager to use for authenticating with and loading backends.
	LoginManager cmdBackend.LoginManager
	// The project stack manager to use for loading and saving project stack configuration.
	ProjectStackManager cmdStack.ProjectStackManager
}

// A set of arguments for the `config get` command.
type configGetArgs struct {
	Colorizer colors.Colorization
	JSON      bool
	Open      bool
	Path      bool
	Stack     string
}

func newConfigGetCmd(configFile *string, stack *string) *cobra.Command {
	configGet := &configGetCmd{
		Args: &configGetArgs{
			Colorizer: cmdutil.GetGlobalColorization(),
		},
		Stdout:       os.Stdout,
		Workspace:    pkgWorkspace.Instance,
		LoginManager: cmdBackend.DefaultLoginManager,
	}

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
		Run: cmd.RunCmdFunc(func(command *cobra.Command, args []string) error {
			configGet.Args.Stack = *stack
			configGet.ProjectStackManager = cmdStack.NewProjectStackManager(*configFile)

			ctx := command.Context()
			err := configGet.run(ctx, args[0])

			return err
		}),
	}

	getCmd.Flags().BoolVarP(
		&configGet.Args.JSON, "json", "j", false,
		"Emit output as JSON")
	getCmd.Flags().BoolVar(
		&configGet.Args.Open, "open", true,
		"Open and resolve any environments listed in the stack configuration")
	getCmd.PersistentFlags().BoolVar(
		&configGet.Args.Path, "path", false,
		"The key contains a path to a property in a map or list to get")

	return getCmd
}

func (cmd *configGetCmd) run(ctx context.Context, keyArg string) error {
	opts := display.Options{
		Color: cmd.Args.Colorizer,
	}

	s, err := cmdStack.RequireStack(
		ctx,
		cmd.Workspace,
		cmd.LoginManager,
		cmd.Args.Stack,
		cmdStack.OfferNew|cmdStack.SetCurrent,
		opts,
	)
	if err != nil {
		return err
	}

	key, err := ParseConfigKey(keyArg)
	if err != nil {
		return fmt.Errorf("invalid configuration key: %w", err)
	}

	ssml := cmdStack.NewStackSecretsManagerLoaderFromEnv()

	project, _, err := cmd.Workspace.ReadProject()
	if err != nil {
		return err
	}
	ps, err := cmd.ProjectStackManager.Load(project, s)
	if err != nil {
		return err
	}

	var env *esc.Environment
	var diags []apitype.EnvironmentDiagnostic
	if cmd.Args.Open {
		env, diags, err = openStackEnv(ctx, s, ps)
	} else {
		env, diags, err = checkStackEnv(ctx, s, ps)
	}
	if err != nil {
		return err
	}

	var pulumiEnv esc.Value
	var envCrypter config.Encrypter
	if env != nil {
		pulumiEnv = env.Properties["pulumiConfig"]

		stackEncrypter, state, err := ssml.GetEncrypter(ctx, s, ps)
		if err != nil {
			return err
		}
		// This may have setup the stack's secrets provider, so save the stack if needed.
		if state != cmdStack.SecretsManagerUnchanged {
			if err = cmd.ProjectStackManager.Save(s, ps); err != nil {
				return fmt.Errorf("save stack config: %w", err)
			}
		}
		envCrypter = stackEncrypter
	}

	stackName := s.Ref().Name().String()

	cfg, err := ps.Config.Copy(config.NopDecrypter, config.NopEncrypter)
	if err != nil {
		return fmt.Errorf("copying config: %w", err)
	}

	// when asking for a configuration value, include values from the project and environment
	err = workspace.ApplyProjectConfig(ctx, stackName, project, pulumiEnv, cfg, envCrypter)
	if err != nil {
		return err
	}

	v, ok, err := cfg.Get(key, cmd.Args.Path)
	if err != nil {
		return err
	}
	if ok {
		var d config.Decrypter
		if v.Secure() {
			var err error
			var state cmdStack.SecretsManagerState
			if d, state, err = ssml.GetDecrypter(ctx, s, ps); err != nil {
				return fmt.Errorf("could not create a decrypter: %w", err)
			}
			// This may have setup the stack's secrets provider, so save the stack if needed.
			if state != cmdStack.SecretsManagerUnchanged {
				if err = cmd.ProjectStackManager.Save(s, ps); err != nil {
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

		if cmd.Args.JSON {
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
			fmt.Fprintln(cmd.Stdout, string(out))
		} else {
			fmt.Fprintf(cmd.Stdout, "%v\n", raw)
		}

		if len(diags) != 0 {
			fmt.Fprintln(cmd.Stdout)
			fmt.Fprintln(cmd.Stdout, "Environment diagnostics:")
			printESCDiagnostics(cmd.Stdout, diags)
		}

		cmdStack.Log3rdPartySecretsProviderDecryptionEvent(ctx, s, key.Name(), "")

		return nil
	}

	return fmt.Errorf("configuration key '%s' not found for stack '%s'", PrettyKey(key), s.Ref())
}
