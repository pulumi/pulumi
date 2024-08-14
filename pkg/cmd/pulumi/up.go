// Copyright 2016-2023, Pulumi Corporation.
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
	"os"
	"runtime"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// The default number of parallel resource operations to run at once during an update, if --parallel is unset.
// See https://github.com/pulumi/pulumi/issues/14989 for context around the cpu * 4 choice.
var defaultParallel = runtime.NumCPU() * 4

//nolint:lll
type UpArgs struct {
	ConfigFile      string `argsUsage:"Use the configuration values in the specified file rather than detecting the file name"`
	Debug           bool   `argsShort:"d" argsUsage:"Print detailed debugging output during resource operations"`
	ExpectNoChanges bool   `args:"expect-no-changes" argsUsage:"Return an error if any changes occur during this update"`
	Message         string `args:"message" argsShort:"m" argsUsage:"Optional message to associate with the update operation"`
	ExecKind        string
	ExecAgent       string
	StackName       string   `args:"stack" argsShort:"s" argsUsage:"The name of the stack to operate on. Defaults to the current stack"`
	ConfigArray     []string `args:"config" argsCommaSplit:"false" argsShort:"c" argsUsage:"Config to use during the update and save to the stack config file"`
	Path            bool     `args:"config-path" argsUsage:"Config keys contain a path to a property in a map or list to set"`
	Client          string   `argsUsage:"The address of an existing language runtime host to connect to"`

	// Flags for engine.UpdateOptions.
	JSON                   bool     `args:"json" argsShort:"j" argsUsage:"Serialize the update diffs, operations, and overall output as JSON"`
	PolicyPackPaths        []string `args:"policy-pack" argsUsage:"Run one or more policy packs as part of this update"`
	PolicyPackConfigPaths  []string "args:\"policy-pack-config\" argsUsage:\"Path to JSON file containing the config for the policy pack of the corresponding \\\"--policy-pack\\\" flag\""
	DiffDisplay            bool     `args:"diff" argsUsage:"Display operation as a rich diff showing the overall change"`
	Parallel               int      `argsShort:"p" argsUsage:"Allow P resource operations to run in parallel at once (1 for no parallelism)."`
	Refresh                string   `argsShort:"r" argsUsage:"Refresh the state of the stack's resources before this update"`
	ShowConfig             bool     `argsUsage:"Show configuration keys and variables"`
	ShowPolicyRemediations bool     `argsUsage:"Show per-resource policy remediation details instead of a summary"`
	ShowReplacementSteps   bool     `argsUsage:"Show detailed resource replacement creates and deletes instead of a single step"`
	ShowSames              bool     `argsUsage:"Show resources that don't need be updated because they haven't changed, alongside those that do"`
	ShowReads              bool     `argsUsage:"Show resources that are being read in, alongside those being managed directly in the stack"`
	SkipPreview            bool     `argsShort:"f" argsUsage:"Do not calculate a preview before performing the update"`
	ShowFullOutput         bool     `argsDefault:"true" argsUsage:"Display full length of stack outputs"`
	SuppressOutputs        bool     `argsUsage:"Suppress display of stack outputs (in case they contain sensitive values)"`
	SuppressProgress       bool     `argsUsage:"Suppress display of periodic progress dots"`
	ContinueOnError        bool     `argsUsage:"Continue updating resources even if an error is encountered (can also be set with PULUMI_CONTINUE_ON_ERROR environment variable)"`
	SuppressPermalink      string   `argsUsage:"Suppress display of the state permalink"`
	Yes                    bool     `argsShort:"y" argsUsage:"Automatically approve and perform the update after previewing it"`
	SecretsProvider        string   `argsDefault:"default" argsUsage:"The type of the provider that should be used to encrypt and decrypt secrets (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault). Only used when creating a new stack from an existing template"`
	Targets                []string `args:"target" argsCommaSplit:"false" argsShort:"t" argsUsage:"Specify a single resource URN to update. Other resources will not be updated. Multiple resources can be specified using --target urn1 --target urn2. Wildcards (*, **) are also supported"`
	Replaces               []string `args:"replace" argsCommaSplit:"false" argsUsage:"Specify a single resource URN to replace. Multiple resources can be specified using --replace urn1 --replace urn2. Wildcards (*, **) are also supported"`
	TargetReplaces         []string `args:"target-replace" argsCommaSplit:"false" argsUsage:"Specify a single resource URN to replace. Other resources will not be updated. Shorthand for --target urn --replace urn."`
	TargetDependents       bool     `argsUsage:"Allows updating of dependent targets discovered but not specified in --target list"`
	PlanFilePath           string   `args:"plan" argsUsage:"[EXPERIMENTAL] Path to a plan file to use for the update. The update will not perform operations that exceed its plan (e.g. replacements instead of updates, or updates instead of sames)."`
}

// intentionally disabling here for cleaner err declaration/assignment.
//
//nolint:vetshadow
func newUpCmd(v *viper.Viper) *cobra.Command {
	// Flags for remote operations.
	remoteArgs := RemoteArgs{}

	// up implementation used when the source of the Pulumi program is in the current working directory.
	upWorkingDirectory := func(ctx context.Context, opts backend.UpdateOptions, cmd *cobra.Command) result.Result {
		config := UnmarshalArgs[UpArgs](v, cmd)
		s, err := requireStack(ctx, config.StackName, stackOfferNew, opts.Display)
		if err != nil {
			return result.FromError(err)
		}

		// Save any config values passed via flags.
		if err := parseAndSaveConfigArray(s, config.ConfigArray, config.Path); err != nil {
			return result.FromError(err)
		}

		proj, root, err := readProjectForUpdate(config.Client)
		if err != nil {
			return result.FromError(err)
		}

		m, err := getUpdateMetadata(
			config.Message,
			root,
			config.ExecKind,
			config.ExecAgent,
			config.PlanFilePath != "",
			cmd.Flags(),
		)
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

		targetURNs, replaceURNs := []string{}, []string{}
		targetURNs = append(targetURNs, config.Targets...)
		replaceURNs = append(replaceURNs, config.Replaces...)

		for _, tr := range config.TargetReplaces {
			targetURNs = append(targetURNs, tr)
			replaceURNs = append(replaceURNs, tr)
		}

		refreshOption, err := getRefreshOption(proj, config.Refresh)
		if err != nil {
			return result.FromError(err)
		}
		opts.Engine = engine.UpdateOptions{
			LocalPolicyPacks:          engine.MakeLocalPolicyPacks(config.PolicyPackPaths, config.PolicyPackConfigPaths),
			Parallel:                  config.Parallel,
			Debug:                     config.Debug,
			Refresh:                   refreshOption,
			ReplaceTargets:            deploy.NewUrnTargets(replaceURNs),
			UseLegacyDiff:             useLegacyDiff(),
			UseLegacyRefreshDiff:      useLegacyRefreshDiff(),
			DisableProviderPreview:    disableProviderPreview(),
			DisableResourceReferences: disableResourceReferences(),
			DisableOutputValues:       disableOutputValues(),
			Targets:                   deploy.NewUrnTargets(targetURNs),
			TargetDependents:          config.TargetDependents,
			// Trigger a plan to be generated during the preview phase which can be constrained to during the
			// update phase.
			GeneratePlan:    true,
			Experimental:    hasExperimentalCommands(),
			ContinueOnError: config.ContinueOnError,
		}

		if config.PlanFilePath != "" {
			dec, err := sm.Decrypter()
			if err != nil {
				return result.FromError(err)
			}
			enc, err := sm.Encrypter()
			if err != nil {
				return result.FromError(err)
			}
			plan, err := readPlan(config.PlanFilePath, dec, enc)
			if err != nil {
				return result.FromError(err)
			}
			opts.Engine.Plan = plan
		}

		changes, res := s.Update(ctx, backend.UpdateOperation{
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
		case res != nil && res.Error() == context.Canceled:
			return result.FromError(errors.New("update cancelled"))
		case res != nil:
			return PrintEngineResult(res)
		case config.ExpectNoChanges && changes != nil && engine.HasChanges(changes):
			return result.FromError(errors.New("no changes were expected but changes occurred"))
		default:
			return nil
		}
	}

	// up implementation used when the source of the Pulumi program is a template name or a URL to a template.
	upTemplateNameOrURL := func(ctx context.Context,
		templateNameOrURL string, opts backend.UpdateOptions, cmd *cobra.Command,
	) result.Result {
		config := UnmarshalArgs[UpArgs](v, cmd)
		// Retrieve the template repo.
		repo, err := workspace.RetrieveTemplates(templateNameOrURL, false, workspace.TemplateKindPulumiProject)
		if err != nil {
			return result.FromError(err)
		}
		defer func() {
			contract.IgnoreError(repo.Delete())
		}()

		// List the templates from the repo.
		templates, err := repo.Templates()
		if err != nil {
			return result.FromError(err)
		}

		var template workspace.Template
		if len(templates) == 0 {
			return result.FromError(errors.New("no template found"))
		} else if len(templates) == 1 {
			template = templates[0]
		} else {
			if template, err = chooseTemplate(templates, opts.Display); err != nil {
				return result.FromError(err)
			}
		}

		// Validate secrets provider type
		if err := validateSecretsProvider(config.SecretsProvider); err != nil {
			return result.FromError(err)
		}

		// Create temp directory for the "virtual workspace".
		temp, err := os.MkdirTemp("", "pulumi-up-")
		if err != nil {
			return result.FromError(err)
		}
		defer func() {
			contract.IgnoreError(os.RemoveAll(temp))
		}()

		// Change the working directory to the "virtual workspace" directory.
		if err = os.Chdir(temp); err != nil {
			return result.FromError(fmt.Errorf("changing the working directory: %w", err))
		}

		// There is no current project at this point to pass into currentBackend
		b, err := currentBackend(ctx, nil, opts.Display)
		if err != nil {
			return result.FromError(err)
		}

		// If a stack was specified via --stack, see if it already exists.
		var name string
		var description string
		var s backend.Stack
		if config.StackName != "" {
			if s, name, description, err = getStack(ctx, b, config.StackName, opts.Display); err != nil {
				return result.FromError(err)
			}
		}

		// Prompt for the project name, if we don't already have one from an existing stack.
		if name == "" {
			defaultValue := pkgWorkspace.ValueOrSanitizedDefaultProjectName(name, template.ProjectName, template.Name)
			name, err = promptForValue(
				config.Yes, "project name", defaultValue, false, pkgWorkspace.ValidateProjectName, opts.Display)
			if err != nil {
				return result.FromError(err)
			}
		}

		// Prompt for the project description, if we don't already have one from an existing stack.
		if description == "" {
			defaultValue := pkgWorkspace.ValueOrDefaultProjectDescription(
				description, template.ProjectDescription, template.Description)
			description, err = promptForValue(
				config.Yes, "project description", defaultValue, false, pkgWorkspace.ValidateProjectDescription, opts.Display)
			if err != nil {
				return result.FromError(err)
			}
		}

		// Copy the template files from the repo to the temporary "virtual workspace" directory.
		if err = workspace.CopyTemplateFiles(template.Dir, temp, true, name, description); err != nil {
			return result.FromError(err)
		}

		// Load the project, update the name & description, remove the template section, and save it.
		proj, root, err := readProject()
		if err != nil {
			return result.FromError(err)
		}
		proj.Name = tokens.PackageName(name)
		proj.Description = &description
		proj.Template = nil
		if err = workspace.SaveProject(proj); err != nil {
			return result.FromError(fmt.Errorf("saving project: %w", err))
		}

		// Create the stack, if needed.
		if s == nil {
			if s, err = promptAndCreateStack(ctx, b, promptForValue, config.StackName, root, false /*setCurrent*/, config.Yes,
				opts.Display, config.SecretsProvider); err != nil {
				return result.FromError(err)
			}
			// The backend will print "Created stack '<stack>'." on success.
		}

		// Prompt for config values (if needed) and save.
		if err = handleConfig(
			ctx, promptForValue, proj, s,
			templateNameOrURL, template, config.ConfigArray,
			config.Yes, config.Path, opts.Display); err != nil {
			return result.FromError(err)
		}

		// Install dependencies.

		projinfo := &engine.Projinfo{Proj: proj, Root: root}
		_, main, pctx, err := engine.ProjectInfoContext(projinfo, nil, cmdutil.Diag(), cmdutil.Diag(), false, nil, nil)
		if err != nil {
			return result.FromError(fmt.Errorf("building project context: %w", err))
		}

		defer pctx.Close()

		if err = installDependencies(pctx, &proj.Runtime, main); err != nil {
			return result.FromError(err)
		}

		m, err := getUpdateMetadata(
			config.Message,
			root,
			config.ExecKind,
			config.ExecAgent,
			config.PlanFilePath != "",
			cmd.Flags(),
		)
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

		stackName := s.Ref().String()
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

		refreshOption, err := getRefreshOption(proj, config.Refresh)
		if err != nil {
			return result.FromError(err)
		}

		opts.Engine = engine.UpdateOptions{
			LocalPolicyPacks: engine.MakeLocalPolicyPacks(config.PolicyPackPaths, config.PolicyPackConfigPaths),
			Parallel:         config.Parallel,
			Debug:            config.Debug,
			Refresh:          refreshOption,

			// If we're in experimental mode then we trigger a plan to be generated during the preview phase
			// which will be constrained to during the update phase.
			GeneratePlan: hasExperimentalCommands(),
			Experimental: hasExperimentalCommands(),

			UseLegacyRefreshDiff: useLegacyRefreshDiff(),
			ContinueOnError:      config.ContinueOnError,
		}

		// TODO for the URL case:
		// - suppress preview display/prompt unless error.
		// - attempt `destroy` on any update errors.
		// - show template.Quickstart?

		changes, res := s.Update(ctx, backend.UpdateOperation{
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
		case res != nil && res.Error() == context.Canceled:
			return result.FromError(errors.New("update cancelled"))
		case res != nil:
			return PrintEngineResult(res)
		case config.ExpectNoChanges && changes != nil && engine.HasChanges(changes):
			return result.FromError(errors.New("no changes were expected but changes occurred"))
		default:
			return nil
		}
	}

	cmd := &cobra.Command{
		Use:        "up [template|url]",
		Aliases:    []string{"update"},
		SuggestFor: []string{"apply", "deploy", "push"},
		Short:      "Create or update the resources in a stack",
		Long: "Create or update the resources in a stack.\n" +
			"\n" +
			"This command creates or updates resources in a stack. The new desired goal state for the target stack\n" +
			"is computed by running the current Pulumi program and observing all resource allocations to produce a\n" +
			"resource graph. This goal state is then compared against the existing state to determine what create,\n" +
			"read, update, and/or delete operations must take place to achieve the desired goal state, in the most\n" +
			"minimally disruptive way. This command records a full transactional snapshot of the stack's new state\n" +
			"afterwards so that the stack may be updated incrementally again later on.\n" +
			"\n" +
			"The program to run is loaded from the project in the current directory by default. Use the `-C` or\n" +
			"`--cwd` flag to use a different directory.",
		Args: cmdutil.MaximumNArgs(1),
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			ctx := cmd.Context()
			config := UnmarshalArgs[UpArgs](v, cmd)

			// Remote implies we're skipping previews.
			if remoteArgs.remote {
				config.SkipPreview = true
			}

			yes := config.Yes || config.SkipPreview || skipConfirmations()

			interactive := cmdutil.Interactive()
			if !interactive && !yes {
				return result.FromError(
					errors.New("--yes or --skip-preview must be passed in to proceed when running in non-interactive mode"))
			}

			opts, err := updateFlagsToOptions(interactive, config.SkipPreview, yes, false /* previewOnly */)
			if err != nil {
				return result.FromError(err)
			}

			if err = validatePolicyPackConfig(config.PolicyPackPaths, config.PolicyPackConfigPaths); err != nil {
				return result.FromError(err)
			}

			displayType := display.DisplayProgress
			if config.DiffDisplay {
				displayType = display.DisplayDiff
			}

			opts.Display = display.Options{
				Color:                  cmdutil.GetGlobalColorization(),
				ShowConfig:             config.ShowConfig,
				ShowPolicyRemediations: config.ShowPolicyRemediations,
				ShowReplacementSteps:   config.ShowReplacementSteps,
				ShowSameResources:      config.ShowSames,
				ShowReads:              config.ShowReads,
				SuppressOutputs:        config.SuppressOutputs,
				SuppressProgress:       config.SuppressProgress,
				TruncateOutput:         !config.ShowFullOutput,
				IsInteractive:          interactive,
				Type:                   displayType,
				Debug:                  config.Debug,
				JSONDisplay:            config.JSON,
			}

			// we only suppress permalinks if the user passes true. the default is an empty string
			// which we pass as 'false'
			if config.SuppressPermalink == "true" {
				opts.Display.SuppressPermalink = true
			} else {
				opts.Display.SuppressPermalink = false
			}

			if remoteArgs.remote {
				err = validateUnsupportedRemoteFlags(
					config.ExpectNoChanges,
					config.ConfigArray,
					config.Path,
					config.Client,
					config.JSON,
					config.PolicyPackPaths,
					config.PolicyPackConfigPaths,
					config.Refresh,
					config.ShowConfig,
					config.ShowPolicyRemediations,
					config.ShowReplacementSteps,
					config.ShowSames,
					config.ShowReads,
					config.SuppressOutputs,
					config.SecretsProvider,
					&config.Targets,
					config.Replaces,
					config.TargetReplaces,
					config.TargetDependents,
					config.PlanFilePath,
					stackConfigFile,
				)
				if err != nil {
					return result.FromError(err)
				}

				var url string
				if len(args) > 0 {
					url = args[0]
				}

				if errResult := validateRemoteDeploymentFlags(url, remoteArgs); errResult != nil {
					return errResult
				}

				return runDeployment(ctx, cmd, opts.Display, apitype.Update, config.StackName, url, remoteArgs)
			}

			isDIYBackend, err := isDIYBackend(opts.Display)
			if err != nil {
				return result.FromError(err)
			}

			// by default, we are going to suppress the permalink when using DIY backends
			// this can be re-enabled by explicitly passing "false" to the `suppress-permalink` flag
			if config.SuppressPermalink != "false" && isDIYBackend {
				opts.Display.SuppressPermalink = true
			}

			if len(args) > 0 {
				return upTemplateNameOrURL(ctx, args[0], opts, cmd)
			}

			return upWorkingDirectory(ctx, opts, cmd)
		}),
	}

	BindFlags[UpArgs](v, cmd)

	_ = cmd.PersistentFlags().MarkHidden("client")
	cmd.PersistentFlags().Lookup("refresh").NoOptDefVal = "true"
	cmd.Flag("suppress-permalink").NoOptDefVal = "false"
	cmd.PersistentFlags().Lookup("parallel").DefValue = strconv.Itoa(defaultParallel)
	if !hasExperimentalCommands() {
		contract.AssertNoErrorf(cmd.PersistentFlags().MarkHidden("plan"), `Could not mark "plan" as hidden`)
	}

	// Remote flags
	remoteArgs.applyFlags(cmd)

	if !hasDebugCommands() {
		_ = cmd.PersistentFlags().MarkHidden("event-log")
	}
	_ = cmd.PersistentFlags().MarkHidden("exec-kind")
	_ = cmd.PersistentFlags().MarkHidden("exec-agent")

	return cmd
}

// validatePolicyPackConfig validates the `--policy-pack-config` and `--policy-pack` flags. These two flags are
// order-dependent, e.g., the first `--policy-pack-config` flag value corresponds to the first `--policy-pack`
// flag value, and so on for the second, third, etc. An error is returned if `--policy-pack-config` is specified
// and there isn't a `--policy-pack-config` for every `--policy-pack` that was set.
func validatePolicyPackConfig(policyPackPaths []string, policyPackConfigPaths []string) error {
	if len(policyPackConfigPaths) > 0 {
		if len(policyPackPaths) == 0 {
			return errors.New(`"--policy-pack-config" must be specified with "--policy-pack"`)
		}
		if len(policyPackConfigPaths) != len(policyPackPaths) {
			return errors.New(
				`the number of "--policy-pack-config" flags must match the number of "--policy-pack" flags`)
		}
	}
	return nil
}

// handleConfig handles prompting for config values (as needed) and saving config.
func handleConfig(
	ctx context.Context,
	prompt promptForValueFunc,
	project *workspace.Project,
	s backend.Stack,
	templateNameOrURL string,
	template workspace.Template,
	configArray []string,
	yes bool,
	path bool,
	opts display.Options,
) error {
	// Get the existing config. stackConfig will be nil if there wasn't a previous deployment.
	stackConfig, err := backend.GetLatestConfiguration(ctx, s)
	if err != nil && err != backend.ErrNoPreviousDeployment {
		return err
	}

	// Get the existing snapshot.
	snap, err := s.Snapshot(ctx, stack.DefaultSecretsProvider)
	if err != nil {
		return err
	}

	// Handle config.
	// If this is an initial preconfigured empty stack (i.e. configured in the Pulumi Console),
	// use its config without prompting.
	// Otherwise, use the values specified on the command line and prompt for new values.
	// If the stack already existed and had previous config, those values will be used as the defaults.
	var c config.Map
	if isPreconfiguredEmptyStack(templateNameOrURL, template.Config, stackConfig, snap) {
		c = stackConfig
		// TODO[pulumi/pulumi#1894] consider warning if templateNameOrURL is different from
		// the stack's `pulumi:template` config value.
	} else {
		// Get config values passed on the command line.
		commandLineConfig, parseErr := parseConfig(configArray, path)
		if parseErr != nil {
			return parseErr
		}

		// Prompt for config as needed.
		c, err = promptForConfig(ctx, prompt, project, s, template.Config, commandLineConfig, stackConfig, yes, opts)
		if err != nil {
			return err
		}
	}

	// Save the config.
	if len(c) > 0 {
		if err = saveConfig(s, c); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Println("Saved config")
		fmt.Println()
	}

	return nil
}

var templateKey = config.MustMakeKey("pulumi", "template")

// isPreconfiguredEmptyStack returns true if the url matches the value of `pulumi:template` in stackConfig,
// the stackConfig values satisfy the config requirements of templateConfig, and the snapshot is empty.
// This is the state of an initial preconfigured empty stack (i.e. a stack that's been created and configured
// in the Pulumi Console).
func isPreconfiguredEmptyStack(
	url string,
	templateConfig map[string]workspace.ProjectTemplateConfigValue,
	stackConfig config.Map,
	snap *deploy.Snapshot,
) bool {
	// Does stackConfig have a `pulumi:template` value and does it match url?
	if stackConfig == nil {
		return false
	}
	templateURLValue, hasTemplateKey := stackConfig[templateKey]
	if !hasTemplateKey {
		return false
	}
	templateURL, err := templateURLValue.Value(nil)
	if err != nil {
		contract.IgnoreError(err)
		return false
	}
	if templateURL != url {
		return false
	}

	// Does the snapshot only contain a single root resource?
	if len(snap.Resources) != 1 {
		return false
	}
	stackResource, err := stack.GetRootStackResource(snap)
	if err != nil || stackResource == nil {
		return false
	}

	// Can stackConfig satisfy the config requirements of templateConfig?
	for templateKey, templateVal := range templateConfig {
		parsedTemplateKey, parseErr := parseConfigKey(templateKey)
		if parseErr != nil {
			contract.IgnoreError(parseErr)
			return false
		}

		stackVal, ok := stackConfig[parsedTemplateKey]
		if !ok {
			return false
		}

		if templateVal.Secret != stackVal.Secure() {
			return false
		}
	}

	return true
}
