// Copyright 2016-2019, Pulumi Corporation.
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

package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

//nolint:lll
type WatchArgs struct {
	Debug                 bool   `argsShort:"d" argsUsage:"Print detailed debugging output during resource operations"`
	Message               string `argsShort:"m" argsUsage:"Optional message to associate with each update operation"`
	ExecKind              string
	Stack                 string   `argsShort:"s" argsUsage:"The name of the stack to operate on. Defaults to the current stack"`
	ConfigArray           []string `args:"config" argsShort:"c" argsUsage:"Config to use during the update"`
	PathArray             []string `args:"path" argsUsage:"Specify one or more relative or absolute paths that need to be watched. A path can point to a folder or a file. Defaults to working directory"`
	ConfigPath            bool     `argsUsage:"Config keys contain a path to a property in a map or list to set"`
	PolicyPackPaths       []string `args:"policy-pack" argsUsage:"Run one or more policy packs as part of each update"`
	PolicyPackConfigPaths []string `args:"policy-pack-config" argsUsage:"Path to JSON file containing the config for the policy pack of the corresponding \"--policy-pack\" flag"`
	Parallel              int      `argsShort:"p" argsUsage:"Allow P resource operations to run in parallel at once (1 for no parallelism)"`
	Refresh               bool     `argsShort:"r" argsUsage:"Refresh the state of the stack's resources before each update"`
	ShowConfig            bool     `argsUsage:"Show configuration keys and variables"`
	ShowReplacementSteps  bool     `argsUsage:"Show detailed resource replacement creates and deletes instead of a single step"`
	ShowSames             bool     `argsUsage:"Show resources that don't need be updated because they haven't changed, alongside those that do"`
	SecretsProvider       string   `argsUsage:"The type of the provider that should be used to encrypt and decrypt secrets (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault). Only used when creating a new stack from an existing template" argsDefault:"default"`
}

// intentionally disabling here for cleaner err declaration/assignment.
//
//nolint:vetshadow
func newWatchCmd(
	v *viper.Viper,
	parentPulumiCmd *cobra.Command,
) *cobra.Command {
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
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, cmdArgs []string) result.Result {
			args := UnmarshalArgs[WatchArgs](v, cmd)

			ctx := cmd.Context()

			opts, err := updateFlagsToOptions(false /* interactive */, true /* skipPreview */, true, /* autoApprove */
				false /* previewOnly */)
			if err != nil {
				return result.FromError(err)
			}

			opts.Display = display.Options{
				Color:                cmdutil.GetGlobalColorization(),
				ShowConfig:           args.ShowConfig,
				ShowReplacementSteps: args.ShowReplacementSteps,
				ShowSameResources:    args.ShowSames,
				SuppressOutputs:      true,
				SuppressProgress:     true,
				SuppressPermalink:    true,
				IsInteractive:        false,
				Type:                 display.DisplayWatch,
				Debug:                args.Debug,
			}

			if err := validatePolicyPackConfig(args.PolicyPackPaths, args.PolicyPackConfigPaths); err != nil {
				return result.FromError(err)
			}

			s, err := requireStack(ctx, args.Stack, stackOfferNew, opts.Display)
			if err != nil {
				return result.FromError(err)
			}

			// Save any config values passed via flags.
			if err := parseAndSaveConfigArray(s, args.ConfigArray, args.ConfigPath); err != nil {
				return result.FromError(err)
			}

			proj, root, err := readProject()
			if err != nil {
				return result.FromError(err)
			}

			m, err := getUpdateMetadata(args.Message, root, args.ExecKind, "" /* execAgent */, false, cmd.Flags())
			if err != nil {
				return result.FromError(fmt.Errorf("gathering environment metadata: %w", err))
			}

			cfg, sm, err := getStackConfiguration(ctx, s, proj, nil)
			if err != nil {
				return result.FromError(fmt.Errorf("getting stack configuration: %w", err))
			}

			decrypter, err := sm.Decrypter()
			if err != nil {
				return result.FromError(fmt.Errorf("getting stack decrypter: %w", err))
			}
			encrypter, err := sm.Encrypter()
			if err != nil {
				return result.FromError(fmt.Errorf("getting stack encrypter: %w", err))
			}

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
				return result.FromError(fmt.Errorf("validating stack config: %w", configErr))
			}

			opts.Engine = engine.UpdateOptions{
				LocalPolicyPacks:          engine.MakeLocalPolicyPacks(args.PolicyPackPaths, args.PolicyPackConfigPaths),
				Parallel:                  args.Parallel,
				Debug:                     args.Debug,
				Refresh:                   args.Refresh,
				UseLegacyDiff:             useLegacyDiff(),
				UseLegacyRefreshDiff:      useLegacyRefreshDiff(),
				DisableProviderPreview:    disableProviderPreview(),
				DisableResourceReferences: disableResourceReferences(),
				DisableOutputValues:       disableOutputValues(),
				Experimental:              hasExperimentalCommands(),
			}

			res := s.Watch(ctx, backend.UpdateOperation{
				Proj:               proj,
				Root:               root,
				M:                  m,
				Opts:               opts,
				StackConfiguration: cfg,
				SecretsManager:     sm,
				SecretsProvider:    stack.DefaultSecretsProvider,
				Scopes:             backend.CancellationScopes,
			}, args.PathArray)

			switch {
			case res != nil && res.Error() == context.Canceled:
				return result.FromError(errors.New("update cancelled"))
			case res != nil:
				return PrintEngineResult(res)
			default:
				return nil
			}
		}),
	}

	parentPulumiCmd.AddCommand(cmd)
	BindFlags[WatchArgs](v, cmd)

	cmd.PersistentFlags().Lookup("parallel").DefValue = strconv.Itoa(defaultParallel)

	// TODO: hack/pulumirc stackConfigFile is missing/broken

	// TODO: hack/pulumirc hidden flags
	// ignore err, only happens if flag does not exist
	_ = cmd.PersistentFlags().MarkHidden("exec-kind")

	return cmd
}
