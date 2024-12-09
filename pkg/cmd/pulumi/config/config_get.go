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
	"os"

	"github.com/spf13/cobra"

	"github.com/pulumi/esc"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

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
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
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

			key, err := ParseConfigKey(args[0])
			if err != nil {
				return fmt.Errorf("invalid configuration key: %w", err)
			}

			ssml := cmdStack.NewStackSecretsManagerLoaderFromEnv()
			return getConfig(ctx, ssml, ws, s, key, path, jsonOut, open)
		}),
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
	ps, err := cmdStack.LoadProjectStack(project, stack)
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
				if err = cmdStack.SaveProjectStack(stack, ps); err != nil {
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
