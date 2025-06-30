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
	cmdConfig "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/config"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/deployment"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/metadata"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/plan"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/autonaming"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func NewUpAllCmd() *cobra.Command {
	var debug bool
	var expectNop bool
	var message string
	var execKind string
	var execAgent string
	var stackName string
	var configArray []string
	var path bool
	var client string

	// Flags for remote operations.
	remoteArgs := deployment.RemoteArgs{}

	// Flags for engine.UpdateOptions.
	var jsonDisplay bool
	var policyPackPaths []string
	var policyPackConfigPaths []string
	var diffDisplay bool
	var eventLogPath string
	var parallel int32
	var refresh string
	var runProgram bool
	var showConfig bool
	var showPolicyRemediations bool
	var showReplacementSteps bool
	var showSames bool
	var showSecrets bool
	var showReads bool
	var skipPreview bool
	var showFullOutput bool
	var suppressOutputs bool
	var suppressProgress bool
	var continueOnError bool
	var suppressPermalink string
	var yes bool
	var secretsProvider string
	var targets []string
	var excludes []string
	var replaces []string
	var targetReplaces []string
	var targetDependents bool
	var excludeDependents bool
	var planFilePath string
	var attachDebugger []string

	// Flags for Copilot.
	var copilotEnabled bool

	// up implementation used when the source of the Pulumi program is in the current working directory.
	upWorkingDirectory := func(
		ctx context.Context,
		ssml cmdStack.SecretsManagerLoader,
		ws pkgWorkspace.Context,
		lm cmdBackend.LoginManager,
		opts backend.UpdateOptions,
		cmd *cobra.Command,
	) error {
		s, err := cmdStack.RequireStack(
			ctx,
			cmdutil.Diag(),
			ws,
			lm,
			stackName,
			cmdStack.OfferNew,
			opts.Display,
		)
		if err != nil {
			return err
		}

		// Save any config values passed via flags.
		if err := parseAndSaveConfigArray(ctx, cmdutil.Diag(), ws, s, configArray, path); err != nil {
			return err
		}

		proj, root, err := readProjectForUpdate(ws, client)
		if err != nil {
			return err
		}

		cfg, sm, err := cmdConfig.GetStackConfiguration(ctx, cmdutil.Diag(), ssml, s, proj)
		if err != nil {
			return fmt.Errorf("getting stack configuration: %w", err)
		}

		m, err := metadata.GetUpdateMetadata(message, root, execKind, execAgent, planFilePath != "", cfg, cmd.Flags())
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

		targetURNs, replaceURNs, excludeURNs := []string{}, []string{}, []string{}
		targetURNs = append(targetURNs, targets...)
		excludeURNs = append(excludeURNs, excludes...)
		replaceURNs = append(replaceURNs, replaces...)

		for _, tr := range targetReplaces {
			targetURNs = append(targetURNs, tr)
			replaceURNs = append(replaceURNs, tr)
		}

		refreshOption, err := getRefreshOption(proj, refresh)
		if err != nil {
			return err
		}

		autonamer, err := autonaming.ParseAutonamingConfig(autonamingStackContext(proj, s), cfg.Config, decrypter)
		if err != nil {
			return fmt.Errorf("getting autonaming config: %w", err)
		}

		opts.Engine = engine.UpdateOptions{
			ParallelDiff:              env.ParallelDiff.Value(),
			LocalPolicyPacks:          engine.MakeLocalPolicyPacks(policyPackPaths, policyPackConfigPaths),
			Parallel:                  parallel,
			Debug:                     debug,
			Refresh:                   refreshOption,
			RefreshProgram:            runProgram,
			ReplaceTargets:            deploy.NewUrnTargets(replaceURNs),
			UseLegacyDiff:             env.EnableLegacyDiff.Value(),
			UseLegacyRefreshDiff:      env.EnableLegacyRefreshDiff.Value(),
			DisableProviderPreview:    env.DisableProviderPreview.Value(),
			DisableResourceReferences: env.DisableResourceReferences.Value(),
			DisableOutputValues:       env.DisableOutputValues.Value(),
			ShowSecrets:               showSecrets,
			Targets:                   deploy.NewUrnTargets(targetURNs),
			Excludes:                  deploy.NewUrnTargets(excludeURNs),
			TargetDependents:          targetDependents,
			ExcludeDependents:         excludeDependents,
			// Trigger a plan to be generated during the preview phase which can be constrained to during the
			// update phase.
			GeneratePlan:    true,
			Experimental:    env.Experimental.Value(),
			ContinueOnError: continueOnError,
			AttachDebugger:  attachDebugger,
			Autonamer:       autonamer,
		}

		if planFilePath != "" {
			dec := sm.Decrypter()
			p, err := plan.Read(planFilePath, dec)
			if err != nil {
				return err
			}
			opts.Engine.Plan = p
		}

		changes, err := backend.UpdateStack(ctx, backend.StackUpdateOperation{
			Stack:              s,
			Proj:               proj,
			Root:               root,
			StackConfiguration: cfg,
			SecretsManager:     sm,
			SecretsProvider:    stack.DefaultSecretsProvider,
		}, backend.UpdateConfiguration{
			M:      m,
			Opts:   opts,
			Scopes: backend.CancellationScopes,
		}, nil /* events */)
		switch {
		case err == context.Canceled:
			return errors.New("update cancelled")
		case err != nil:
			return err
		case expectNop && changes != nil && engine.HasChanges(changes):
			return errors.New("no changes were expected but changes occurred")
		default:
			return nil
		}
	}

	cmd := &cobra.Command{
		Use:   "up-all",
		Short: "Create or update the resources in multiple stacks",
		Long: "Create or update the resources in multiple stacks.\n" +
			"\n" +
			"This command creates or updates resources in multiple stacks. The new desired goal state for the target\n" +
			"stacks is computed by running the current Pulumi program and observing all resource allocations to produce a\n" +
			"resource graph. This goal state is then compared against the existing state to determine what create,\n" +
			"read, update, and/or delete operations must take place to achieve the desired goal state, in the most\n" +
			"minimally disruptive way. This command records a full transactional snapshot of the stacks' new states\n" +
			"afterwards so that stacks may be updated incrementally again later on.",
		Args: cmdutil.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ssml := cmdStack.NewStackSecretsManagerLoaderFromEnv()
			ws := pkgWorkspace.Instance

			// Remote implies we're skipping previews.
			if remoteArgs.Remote {
				skipPreview = true
			}

			yes = yes || skipPreview || env.SkipConfirmations.Value()

			interactive := cmdutil.Interactive()
			if !interactive && !yes {
				return errors.New(
					"--yes or --skip-preview must be passed in to proceed when running in non-interactive mode",
				)
			}

			err := validateAttachDebuggerFlag(attachDebugger)
			if err != nil {
				return err
			}

			opts, err := updateFlagsToOptions(interactive, skipPreview, yes, false /* previewOnly */)
			if err != nil {
				return err
			}

			if err = validatePolicyPackConfig(policyPackPaths, policyPackConfigPaths); err != nil {
				return err
			}

			displayType := display.DisplayProgress
			if diffDisplay {
				displayType = display.DisplayDiff
			}

			opts.Display = display.Options{
				Color:                  cmdutil.GetGlobalColorization(),
				ShowConfig:             showConfig,
				ShowPolicyRemediations: showPolicyRemediations,
				ShowReplacementSteps:   showReplacementSteps,
				ShowSameResources:      showSames,
				ShowReads:              showReads,
				SuppressOutputs:        suppressOutputs,
				SuppressProgress:       suppressProgress,
				TruncateOutput:         !showFullOutput,
				IsInteractive:          interactive,
				Type:                   displayType,
				EventLogPath:           eventLogPath,
				Debug:                  debug,
				JSONDisplay:            jsonDisplay,
				ShowSecrets:            showSecrets,
			}

			// we only suppress permalinks if the user passes true. the default is an empty string
			// which we pass as 'false'
			if suppressPermalink == "true" {
				opts.Display.SuppressPermalink = true
			} else {
				opts.Display.SuppressPermalink = false
			}

			if remoteArgs.Remote {
				err = deployment.ValidateUnsupportedRemoteFlags(expectNop, configArray, path, client, jsonDisplay, policyPackPaths,
					policyPackConfigPaths, refresh, showConfig, showPolicyRemediations, showReplacementSteps, showSames,
					showReads, suppressOutputs, secretsProvider, &targets, &excludes, replaces, targetReplaces,
					targetDependents, planFilePath, cmdStack.ConfigFile, runProgram)
				if err != nil {
					return err
				}

				var url string
				if len(args) > 0 {
					url = args[0]
				}

				if err = deployment.ValidateRemoteDeploymentFlags(url, remoteArgs); err != nil {
					return err
				}

				return deployment.RunDeployment(ctx, ws, cmd, opts.Display, apitype.Update, stackName, url, remoteArgs)
			}

			isDIYBackend, err := cmdBackend.IsDIYBackend(ws, opts.Display)
			if err != nil {
				return err
			}

			// by default, we are going to suppress the permalink when using DIY backends
			// this can be re-enabled by explicitly passing "false" to the `suppress-permalink` flag
			if suppressPermalink != "false" && isDIYBackend {
				opts.Display.SuppressPermalink = true
			}

			// Link to Copilot will be shown for orgs that have Copilot enabled, unless the user explicitly suppressed it.
			// Currently only available for `pulumi up`.
			logging.V(7).Infof("PULUMI_SUPPRESS_COPILOT_LINK=%v", env.SuppressCopilotLink.Value())
			opts.Display.ShowLinkToCopilot = !env.SuppressCopilotLink.Value()

			configureCopilotOptions(copilotEnabled, cmd, &opts.Display, isDIYBackend)

			return upWorkingDirectory(
				ctx,
				ssml,
				ws,
				cmdBackend.DefaultLoginManager,
				opts,
				cmd,
			)
		},
	}

	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().BoolVar(
		&expectNop, "expect-no-changes", false,
		"Return an error if any changes occur during this update. This check happens after the update is applied")
	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().StringVar(
		&cmdStack.ConfigFile, "config-file", "",
		"Use the configuration values in the specified file rather than detecting the file name")
	cmd.PersistentFlags().StringArrayVarP(
		&configArray, "config", "c", []string{},
		"Config to use during the update and save to the stack config file")
	cmd.PersistentFlags().BoolVar(
		&path, "config-path", false,
		"Config keys contain a path to a property in a map or list to set")
	cmd.PersistentFlags().StringVar(
		&secretsProvider, "secrets-provider", "default", "The type of the provider that should be used to encrypt and "+
			"decrypt secrets (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault). Only "+
			"used when creating a new stack from an existing template")

	cmd.PersistentFlags().StringVar(
		&client, "client", "", "The address of an existing language runtime host to connect to")
	_ = cmd.PersistentFlags().MarkHidden("client")

	cmd.PersistentFlags().StringVarP(
		&message, "message", "m", "",
		"Optional message to associate with the update operation")

	cmd.PersistentFlags().StringArrayVarP(
		&targets, "target", "t", []string{},
		"Specify a single resource URN to update. Other resources will not be updated."+
			" Multiple resources can be specified using --target urn1 --target urn2."+
			" Wildcards (*, **) are also supported")
	cmd.PersistentFlags().StringArrayVar(
		&excludes, "exclude", []string{},
		"Specify a resource URN to ignore. These resources will not be updated."+
			" Multiple resources can be specified using --exclude urn1 --exclude urn2."+
			" Wildcards (*, **) are also supported")
	cmd.PersistentFlags().StringArrayVar(
		&replaces, "replace", []string{},
		"Specify a single resource URN to replace. Multiple resources can be specified using --replace urn1 --replace urn2."+
			" Wildcards (*, **) are also supported")
	cmd.PersistentFlags().StringArrayVar(
		&targetReplaces, "target-replace", []string{},
		"Specify a single resource URN to replace. Other resources will not be updated."+
			" Shorthand for --target urn --replace urn.")
	cmd.PersistentFlags().BoolVar(
		&targetDependents, "target-dependents", false,
		"Allows updating of dependent targets discovered but not specified in --target list")
	cmd.PersistentFlags().BoolVar(
		&excludeDependents, "exclude-dependents", false,
		"Allows ignoring of dependent targets discovered but not specified in --exclude list")

	// Currently, we can't mix `--target` and `--exclude`.
	cmd.MarkFlagsMutuallyExclusive("target", "exclude")

	// Flags for engine.UpdateOptions.
	cmd.PersistentFlags().StringSliceVar(
		&policyPackPaths, "policy-pack", []string{},
		"Run one or more policy packs as part of this update")
	cmd.PersistentFlags().StringSliceVar(
		&policyPackConfigPaths, "policy-pack-config", []string{},
		`Path to JSON file containing the config for the policy pack of the corresponding "--policy-pack" flag`)
	cmd.PersistentFlags().BoolVar(
		&diffDisplay, "diff", false,
		"Display operation as a rich diff showing the overall change")
	cmd.Flags().BoolVarP(
		&jsonDisplay, "json", "j", false,
		"Serialize the update diffs, operations, and overall output as JSON")
	cmd.PersistentFlags().Int32VarP(
		&parallel, "parallel", "p", defaultParallel(),
		"Allow P resource operations to run in parallel at once (1 for no parallelism).")
	cmd.PersistentFlags().StringVarP(
		&refresh, "refresh", "r", "",
		"Refresh the state of the stack's resources before this update")
	cmd.PersistentFlags().Lookup("refresh").NoOptDefVal = "true"
	cmd.PersistentFlags().BoolVar(
		&runProgram, "run-program", env.RunProgram.Value(),
		"Run the program to determine up-to-date state for providers to refresh resources,"+
			" this only applies if --refresh is set")
	cmd.PersistentFlags().BoolVar(
		&showConfig, "show-config", false,
		"Show configuration keys and variables")
	cmd.PersistentFlags().BoolVar(
		&showPolicyRemediations, "show-policy-remediations", false,
		"Show per-resource policy remediation details instead of a summary")
	cmd.PersistentFlags().BoolVar(
		&showReplacementSteps, "show-replacement-steps", false,
		"Show detailed resource replacement creates and deletes instead of a single step")

	cmd.PersistentFlags().BoolVar(
		&showSames, "show-sames", false,
		"Show resources that don't need be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVar(
		&showSecrets, "show-secrets", false,
		"Show secret outputs in the CLI output")
	cmd.PersistentFlags().BoolVar(
		&showReads, "show-reads", false,
		"Show resources that are being read in, alongside those being managed directly in the stack")

	cmd.PersistentFlags().BoolVarP(
		&skipPreview, "skip-preview", "f", false,
		"Do not calculate a preview before performing the update")
	cmd.PersistentFlags().BoolVar(
		&suppressOutputs, "suppress-outputs", false,
		"Suppress display of stack outputs (in case they contain sensitive values)")
	cmd.PersistentFlags().BoolVar(
		&suppressProgress, "suppress-progress", false,
		"Suppress display of periodic progress dots")
	cmd.PersistentFlags().BoolVar(
		&showFullOutput, "show-full-output", true,
		"Display full length of stack outputs")
	cmd.PersistentFlags().StringVar(
		&suppressPermalink, "suppress-permalink", "",
		"Suppress display of the state permalink")
	cmd.Flag("suppress-permalink").NoOptDefVal = "false"
	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Automatically approve and perform the update after previewing it")
	cmd.PersistentFlags().BoolVar(
		&continueOnError, "continue-on-error", env.ContinueOnError.Value(),
		"Continue updating resources even if an error is encountered "+
			"(can also be set with PULUMI_CONTINUE_ON_ERROR environment variable)")
	//nolint:lll // long description
	cmd.PersistentFlags().StringArrayVar(
		&attachDebugger, "attach-debugger", []string{},
		"Enable the ability to attach a debugger to the program and source based plugins being executed. Can limit debug type to 'program', 'plugins', 'plugin:<name>' or 'all'.")
	cmd.Flag("attach-debugger").NoOptDefVal = "program"

	cmd.PersistentFlags().StringVar(
		&planFilePath, "plan", "",
		"[EXPERIMENTAL] Path to a plan file to use for the update. The update will not "+
			"perform operations that exceed its plan (e.g. replacements instead of updates, or updates instead"+
			"of sames).")
	if !env.Experimental.Value() {
		contract.AssertNoErrorf(cmd.PersistentFlags().MarkHidden("plan"), `Could not mark "plan" as hidden`)
	}

	cmd.PersistentFlags().BoolVar(
		&copilotEnabled, "copilot", false,
		"Enable Pulumi Copilot's assistance for improved CLI experience and insights."+
			"(can also be set with PULUMI_COPILOT environment variable)")

	// Currently, we can't mix `--target` and `--exclude`.
	cmd.MarkFlagsMutuallyExclusive("target", "exclude")

	// Remote flags
	remoteArgs.ApplyFlags(cmd)

	if env.DebugCommands.Value() {
		cmd.PersistentFlags().StringVar(
			&eventLogPath, "event-log", "",
			"Log events to a file at this path")
	}

	// internal flags
	cmd.PersistentFlags().StringVar(&execKind, "exec-kind", "", "")
	// ignore err, only happens if flag does not exist
	_ = cmd.PersistentFlags().MarkHidden("exec-kind")
	cmd.PersistentFlags().StringVar(&execAgent, "exec-agent", "", "")
	// ignore err, only happens if flag does not exist
	_ = cmd.PersistentFlags().MarkHidden("exec-agent")

	return cmd
}
