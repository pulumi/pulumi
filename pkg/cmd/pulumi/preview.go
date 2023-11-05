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

package main

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newPreviewCmd() *cobra.Command {
	var debug bool
	var expectNop bool
	var message string
	var execKind string
	var execAgent string
	var stackName string
	var configArray []string
	var configPath bool
	var client string
	var planFilePath string
	var showSecrets bool

	// Flags for remote operations.
	remoteArgs := RemoteArgs{}

	// Flags for engine.UpdateOptions.
	var jsonDisplay bool
	var policyPackPaths []string
	var policyPackConfigPaths []string
	var diffDisplay bool
	var eventLogPath string
	var parallel int
	var refresh string
	var showConfig bool
	var showPolicyRemediations bool
	var showReplacementSteps bool
	var showSames bool
	var showReads bool
	var suppressOutputs bool
	var suppressPermalink string
	var targets []string
	var replaces []string
	var targetReplaces []string
	var targetDependents bool

	use, cmdArgs := "preview", cmdutil.NoArgs
	if remoteSupported() {
		use, cmdArgs = "preview [url]", cmdutil.MaximumNArgs(1)
	}

	cmd := &cobra.Command{
		Use:        use,
		Aliases:    []string{"pre"},
		SuggestFor: []string{"build", "plan"},
		Short:      "Show a preview of updates to a stack's resources",
		Long: "Show a preview of updates a stack's resources.\n" +
			"\n" +
			"This command displays a preview of the updates to an existing stack whose state is\n" +
			"represented by an existing state file. The new desired state is computed by running\n" +
			"a Pulumi program, and extracting all resource allocations from its resulting object graph.\n" +
			"These allocations are then compared against the existing state to determine what\n" +
			"operations must take place to achieve the desired state. No changes to the stack will\n" +
			"actually take place.\n" +
			"\n" +
			"The program to run is loaded from the project in the current directory. Use the `-C` or\n" +
			"`--cwd` flag to use a different directory.",
		Args: cmdArgs,
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			ctx := commandContext()
			displayType := display.DisplayProgress
			if diffDisplay {
				displayType = display.DisplayDiff
			}

			displayOpts := display.Options{
				Color:                  cmdutil.GetGlobalColorization(),
				ShowConfig:             showConfig,
				ShowPolicyRemediations: showPolicyRemediations,
				ShowReplacementSteps:   showReplacementSteps,
				ShowSameResources:      showSames,
				ShowReads:              showReads,
				SuppressOutputs:        suppressOutputs,
				IsInteractive:          cmdutil.Interactive(),
				Type:                   displayType,
				JSONDisplay:            jsonDisplay,
				EventLogPath:           eventLogPath,
				Debug:                  debug,
			}

			// we only suppress permalinks if the user passes true. the default is an empty string
			// which we pass as 'false'
			if suppressPermalink == "true" {
				displayOpts.SuppressPermalink = true
			} else {
				displayOpts.SuppressPermalink = false
			}

			if remoteArgs.remote {
				if len(args) == 0 {
					return result.FromError(errors.New("must specify remote URL"))
				}

				err := validateUnsupportedRemoteFlags(expectNop, configArray, configPath, client, jsonDisplay,
					policyPackPaths, policyPackConfigPaths, refresh, showConfig, showPolicyRemediations,
					showReplacementSteps, showSames, showReads, suppressOutputs, "default", &targets, replaces,
					targetReplaces, targetDependents, planFilePath, stackConfigFile)
				if err != nil {
					return result.FromError(err)
				}

				return runDeployment(ctx, displayOpts, apitype.Preview, stackName, args[0], remoteArgs)
			}

			filestateBackend, err := isFilestateBackend(displayOpts)
			if err != nil {
				return result.FromError(err)
			}

			// by default, we are going to suppress the permalink when using self-managed backends
			// this can be re-enabled by explicitly passing "false" to the `suppress-permalink` flag
			if suppressPermalink != "false" && filestateBackend {
				displayOpts.SuppressPermalink = true
			}

			if err := validatePolicyPackConfig(policyPackPaths, policyPackConfigPaths); err != nil {
				return result.FromError(err)
			}

			s, err := requireStack(ctx, stackName, stackOfferNew, displayOpts)
			if err != nil {
				return result.FromError(err)
			}

			// Save any config values passed via flags.
			if err = parseAndSaveConfigArray(s, configArray, configPath); err != nil {
				return result.FromError(err)
			}

			proj, root, err := readProjectForUpdate(client)
			if err != nil {
				return result.FromError(err)
			}

			ps, err := loadProjectStack(proj, s)
			if err != nil {
				return result.FromError(err)
			}

			hasPlan := planFilePath != ""
			m, err := getUpdateMetadata(message, root, execKind, execAgent, hasPlan, ps.Environment, cmd.Flags())
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
				stackName,
				proj,
				cfg.Environment,
				cfg.Config,
				encrypter,
				decrypter)
			if configErr != nil {
				return result.FromError(fmt.Errorf("validating stack config: %w", configErr))
			}

			targetURNs := []string{}
			targetURNs = append(targetURNs, targets...)

			replaceURNs := []string{}
			replaceURNs = append(replaceURNs, replaces...)

			for _, tr := range targetReplaces {
				targetURNs = append(targetURNs, tr)
				replaceURNs = append(replaceURNs, tr)
			}

			refreshOption, err := getRefreshOption(proj, refresh)
			if err != nil {
				return result.FromError(err)
			}

			opts := backend.UpdateOptions{
				Engine: engine.UpdateOptions{
					LocalPolicyPacks:          engine.MakeLocalPolicyPacks(policyPackPaths, policyPackConfigPaths),
					Parallel:                  parallel,
					Debug:                     debug,
					Refresh:                   refreshOption,
					ReplaceTargets:            deploy.NewUrnTargets(replaceURNs),
					UseLegacyDiff:             useLegacyDiff(),
					DisableProviderPreview:    disableProviderPreview(),
					DisableResourceReferences: disableResourceReferences(),
					DisableOutputValues:       disableOutputValues(),
					Targets:                   deploy.NewUrnTargets(targetURNs),
					TargetDependents:          targetDependents,
					// If we're trying to save a plan then we _need_ to generate it. We also turn this on in
					// experimental mode to just get more testing of it.
					GeneratePlan: hasExperimentalCommands() || planFilePath != "",
					Experimental: hasExperimentalCommands(),
				},
				Display: displayOpts,
			}

			plan, changes, res := s.Preview(ctx, backend.UpdateOperation{
				Proj:               proj,
				Root:               root,
				M:                  m,
				Opts:               opts,
				StackConfiguration: cfg,
				SecretsManager:     sm,
				SecretsProvider:    stack.DefaultSecretsProvider,
				Scopes:             backend.CancellationScopes,
			})

			switch {
			case res != nil:
				return PrintEngineResult(res)
			case expectNop && changes != nil && engine.HasChanges(changes):
				return result.FromError(errors.New("error: no changes were expected but changes were proposed"))
			default:
				if planFilePath != "" {
					encrypter, err := sm.Encrypter()
					if err != nil {
						return result.FromError(err)
					}
					if err = writePlan(planFilePath, plan, encrypter, showSecrets); err != nil {
						return result.FromError(err)
					}

					// Write out message on how to use the plan (if not writing out --json)
					if !jsonDisplay {
						var buf bytes.Buffer
						fprintf(&buf, "Update plan written to '%s'", planFilePath)
						fprintf(
							&buf,
							"\nRun `pulumi up --plan='%s'` to constrain the update to the operations planned by this preview",
							planFilePath)
						cmdutil.Diag().Infof(diag.RawMessage("" /*urn*/, buf.String()))
					}
				}
				return nil
			}
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().BoolVar(
		&expectNop, "expect-no-changes", false,
		"Return an error if any changes are proposed by this preview")
	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().StringVar(
		&stackConfigFile, "config-file", "",
		"Use the configuration values in the specified file rather than detecting the file name")
	cmd.PersistentFlags().StringArrayVarP(
		&configArray, "config", "c", []string{},
		"Config to use during the preview")
	cmd.PersistentFlags().BoolVar(
		&configPath, "config-path", false,
		"Config keys contain a path to a property in a map or list to set")
	cmd.PersistentFlags().StringVar(
		&planFilePath, "save-plan", "",
		"[EXPERIMENTAL] Save the operations proposed by the preview to a plan file at the given path")
	if !hasExperimentalCommands() {
		contract.AssertNoErrorf(cmd.PersistentFlags().MarkHidden("save-plan"), `Could not mark "save-plan" as hidden`)
	}
	cmd.Flags().BoolVarP(
		&showSecrets, "show-secrets", "", false, "Emit secrets in plaintext in the plan file. Defaults to `false`")

	cmd.PersistentFlags().StringVar(
		&client, "client", "", "The address of an existing language runtime host to connect to")
	_ = cmd.PersistentFlags().MarkHidden("client")

	cmd.PersistentFlags().StringVarP(
		&message, "message", "m", "",
		"Optional message to associate with the preview operation")

	cmd.PersistentFlags().StringArrayVarP(
		&targets, "target", "t", []string{},
		"Specify a single resource URN to update. Other resources will not be updated."+
			" Multiple resources can be specified using --target urn1 --target urn2")
	cmd.PersistentFlags().StringArrayVar(
		&replaces, "replace", []string{},
		"Specify resources to replace. Multiple resources can be specified using --replace urn1 --replace urn2")
	cmd.PersistentFlags().StringArrayVar(
		&targetReplaces, "target-replace", []string{},
		"Specify a single resource URN to replace. Other resources will not be updated."+
			" Shorthand for --target urn --replace urn.")
	cmd.PersistentFlags().BoolVar(
		&targetDependents, "target-dependents", false,
		"Allows updating of dependent targets discovered but not specified in --target list")

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
		"Serialize the preview diffs, operations, and overall output as JSON")
	cmd.PersistentFlags().IntVarP(
		&parallel, "parallel", "p", defaultParallel,
		"Allow P resource operations to run in parallel at once (1 for no parallelism). Defaults to unbounded.")
	cmd.PersistentFlags().StringVarP(
		&refresh, "refresh", "r", "",
		"Refresh the state of the stack's resources before this update")
	cmd.PersistentFlags().Lookup("refresh").NoOptDefVal = "true"
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
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVar(
		&showReads, "show-reads", false,
		"Show resources that are being read in, alongside those being managed directly in the stack")
	cmd.PersistentFlags().BoolVar(
		&suppressOutputs, "suppress-outputs", false,
		"Suppress display of stack outputs (in case they contain sensitive values)")

	cmd.PersistentFlags().StringVar(
		&suppressPermalink, "suppress-permalink", "",
		"Suppress display of the state permalink")
	cmd.Flag("suppress-permalink").NoOptDefVal = "false"

	// Remote flags
	remoteArgs.applyFlags(cmd)

	if hasDebugCommands() {
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
