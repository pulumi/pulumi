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

package operations

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/config"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/metadata"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func NewWatchCmd() *cobra.Command {
	var debug bool
	var message string
	var execKind string
	var stackName string
	var configArray []string
	var pathArray []string
	var configPath bool

	// Flags for engine.UpdateOptions.
	var policyPackPaths []string
	var policyPackConfigPaths []string
	var parallel int32
	var refresh bool
	var showConfig bool
	var showReplacementSteps bool
	var showSames bool
	var secretsProvider string

	cmd := &cobra.Command{
		Use:        "watch",
		SuggestFor: []string{"developer", "dev"},
		Short:      "Continuously update the resources in a stack",
		Long: "[EXPERIMENTAL] Continuously update the resources in a stack.\n" +
			"\n" +
			"This command watches the working directory or specified paths for the current project and updates\n" +
			"the active stack whenever the project changes.  In parallel, logs are collected for all resources\n" +
			"in the stack and displayed along with update progress.\n" +
			"\n" +
			"The program to watch is loaded from the project in the current directory by default. Use the `-C` or\n" +
			"`--cwd` flag to use a different directory.",
		Args: cmdutil.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ssml := cmdStack.NewStackSecretsManagerLoaderFromEnv()
			ws := pkgWorkspace.Instance

			opts, err := updateFlagsToOptions(false /* interactive */, true /* skipPreview */, true, /* autoApprove */
				false /* previewOnly */)
			if err != nil {
				return err
			}

			opts.Display = display.Options{
				Color:                cmdutil.GetGlobalColorization(),
				ShowConfig:           showConfig,
				ShowReplacementSteps: showReplacementSteps,
				ShowSameResources:    showSames,
				SuppressOutputs:      true,
				SuppressProgress:     true,
				SuppressPermalink:    false,
				IsInteractive:        false,
				Type:                 display.DisplayWatch,
				Debug:                debug,
			}

			if err := validatePolicyPackConfig(policyPackPaths, policyPackConfigPaths); err != nil {
				return err
			}

			s, err := cmdStack.RequireStack(
				ctx,
				cmdutil.Diag(),
				ws,
				cmdBackend.DefaultLoginManager,
				stackName,
				cmdStack.OfferNew,
				opts.Display,
			)
			if err != nil {
				return err
			}

			// Save any config values passed via flags.
			if err := parseAndSaveConfigArray(ctx, cmdutil.Diag(), ws, s, configArray, configPath); err != nil {
				return err
			}

			proj, root, err := ws.ReadProject()
			if err != nil {
				return err
			}

			cfg, sm, err := config.GetStackConfiguration(ctx, cmdutil.Diag(), ssml, s, proj)
			if err != nil {
				return fmt.Errorf("getting stack configuration: %w", err)
			}

			m, err := metadata.GetUpdateMetadata(message, root, execKind, "" /* execAgent */, false, cfg, cmd.Flags())
			if err != nil {
				return fmt.Errorf("gathering environment metadata: %w", err)
			}

			decrypter := sm.Decrypter()
			encrypter := sm.Encrypter()

			stackName := s.Ref().Name().String()
			configErr := workspace.ValidateStackConfigAndApplyProjectConfig(
				ctx,
				stackName,
				proj,
				cfg.Environment,
				cfg.Config,
				encrypter,
				decrypter)
			if configErr != nil {
				return fmt.Errorf("validating stack config: %w", configErr)
			}

			opts.Engine = engine.UpdateOptions{
				ParallelDiff:              env.ParallelDiff.Value(),
				LocalPolicyPacks:          engine.MakeLocalPolicyPacks(policyPackPaths, policyPackConfigPaths),
				Parallel:                  parallel,
				Debug:                     debug,
				Refresh:                   refresh,
				UseLegacyDiff:             env.EnableLegacyDiff.Value(),
				UseLegacyRefreshDiff:      env.EnableLegacyRefreshDiff.Value(),
				DisableProviderPreview:    env.DisableProviderPreview.Value(),
				DisableResourceReferences: env.DisableResourceReferences.Value(),
				DisableOutputValues:       env.DisableOutputValues.Value(),
				Experimental:              env.Experimental.Value(),
			}

			err = backend.WatchStack(ctx, s, backend.UpdateOperation{
				Proj:               proj,
				Root:               root,
				M:                  m,
				Opts:               opts,
				StackConfiguration: cfg,
				SecretsManager:     sm,
				SecretsProvider:    stack.DefaultSecretsProvider,
				Scopes:             backend.CancellationScopes,
			}, pathArray)

			switch {
			case err == context.Canceled:
				return errors.New("update cancelled")
			case err != nil:
				return err
			default:
				return nil
			}
		},
	}

	cmd.PersistentFlags().StringArrayVarP(
		&pathArray, "path", "", []string{""},
		"Specify one or more relative or absolute paths that need to be watched. "+
			"A path can point to a folder or a file. Defaults to working directory")
	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().StringVar(
		&cmdStack.ConfigFile, "config-file", "",
		"Use the configuration values in the specified file rather than detecting the file name")
	cmd.PersistentFlags().StringArrayVarP(
		&configArray, "config", "c", []string{},
		"Config to use during the update")
	cmd.PersistentFlags().BoolVar(
		&configPath, "config-path", false,
		"Config keys contain a path to a property in a map or list to set")
	cmd.PersistentFlags().StringVar(
		&secretsProvider, "secrets-provider", "default", "The type of the provider that should be used to encrypt and "+
			"decrypt secrets (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault). Only "+
			"used when creating a new stack from an existing template")

	cmd.PersistentFlags().StringVarP(
		&message, "message", "m", "",
		"Optional message to associate with each update operation")

	// Flags for engine.UpdateOptions.
	cmd.PersistentFlags().StringSliceVar(
		&policyPackPaths, "policy-pack", []string{},
		"Run one or more policy packs as part of each update")
	cmd.PersistentFlags().StringSliceVar(
		&policyPackConfigPaths, "policy-pack-config", []string{},
		`Path to JSON file containing the config for the policy pack of the corresponding "--policy-pack" flag`)
	cmd.PersistentFlags().Int32VarP(
		&parallel, "parallel", "p", defaultParallel(),
		"Allow P resource operations to run in parallel at once (1 for no parallelism).")
	cmd.PersistentFlags().BoolVarP(
		&refresh, "refresh", "r", false,
		"Refresh the state of the stack's resources before each update")
	cmd.PersistentFlags().BoolVar(
		&showConfig, "show-config", false,
		"Show configuration keys and variables")
	cmd.PersistentFlags().BoolVar(
		&showReplacementSteps, "show-replacement-steps", false,
		"Show detailed resource replacement creates and deletes instead of a single step")
	cmd.PersistentFlags().BoolVar(
		&showSames, "show-sames", false,
		"Show resources that don't need be updated because they haven't changed, alongside those that do")

	cmd.PersistentFlags().StringVar(&execKind, "exec-kind", "", "")
	// ignore err, only happens if flag does not exist
	_ = cmd.PersistentFlags().MarkHidden("exec-kind")

	return cmd
}
