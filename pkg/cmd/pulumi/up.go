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
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"os"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

const (
	defaultParallel = math.MaxInt32
)

// intentionally disabling here for cleaner err declaration/assignment.
// nolint: vetshadow
func newUpCmd() *cobra.Command {
	var debug bool
	var expectNop bool
	var message string
	var execKind string
	var execAgent string
	var stack string
	var configArray []string
	var path bool
	var client string

	// Flags for engine.UpdateOptions.
	var policyPackPaths []string
	var policyPackConfigPaths []string
	var diffDisplay bool
	var eventLogPath string
	var parallel int
	var refresh bool
	var showConfig bool
	var showReplacementSteps bool
	var showSames bool
	var showReads bool
	var skipPreview bool
	var suppressOutputs bool
	var suppressPermaLink string
	var yes bool
	var secretsProvider string
	var targets []string
	var replaces []string
	var targetReplaces []string
	var targetDependents bool

	// up implementation used when the source of the Pulumi program is in the current working directory.
	upWorkingDirectory := func(opts backend.UpdateOptions) result.Result {
		s, err := requireStack(stack, true, opts.Display, false /*setCurrent*/)
		if err != nil {
			return result.FromError(err)
		}

		// Save any config values passed via flags.
		if err := parseAndSaveConfigArray(s, configArray, path); err != nil {
			return result.FromError(err)
		}

		proj, root, err := readProjectForUpdate(client)
		if err != nil {
			return result.FromError(err)
		}

		m, err := getUpdateMetadata(message, root, execKind, execAgent)
		if err != nil {
			return result.FromError(errors.Wrap(err, "gathering environment metadata"))
		}

		sm, err := getStackSecretsManager(s)
		if err != nil {
			return result.FromError(errors.Wrap(err, "getting secrets manager"))
		}

		cfg, err := getStackConfiguration(s, sm)
		if err != nil {
			return result.FromError(errors.Wrap(err, "getting stack configuration"))
		}

		targetURNs := []resource.URN{}
		for _, t := range targets {
			targetURNs = append(targetURNs, resource.URN(t))
		}

		replaceURNs := []resource.URN{}
		for _, r := range replaces {
			replaceURNs = append(replaceURNs, resource.URN(r))
		}

		for _, tr := range targetReplaces {
			targetURNs = append(targetURNs, resource.URN(tr))
			replaceURNs = append(replaceURNs, resource.URN(tr))
		}

		opts.Engine = engine.UpdateOptions{
			LocalPolicyPacks:          engine.MakeLocalPolicyPacks(policyPackPaths, policyPackConfigPaths),
			Parallel:                  parallel,
			Debug:                     debug,
			Refresh:                   refresh,
			RefreshTargets:            targetURNs,
			ReplaceTargets:            replaceURNs,
			UseLegacyDiff:             useLegacyDiff(),
			DisableProviderPreview:    disableProviderPreview(),
			DisableResourceReferences: disableResourceReferences(),
			UpdateTargets:             targetURNs,
			TargetDependents:          targetDependents,
		}

		changes, res := s.Update(commandContext(), backend.UpdateOperation{
			Proj:               proj,
			Root:               root,
			M:                  m,
			Opts:               opts,
			StackConfiguration: cfg,
			SecretsManager:     sm,
			Scopes:             cancellationScopes,
		})
		switch {
		case res != nil && res.Error() == context.Canceled:
			return result.FromError(errors.New("update cancelled"))
		case res != nil:
			return PrintEngineResult(res)
		case expectNop && changes != nil && changes.HasChanges():
			return result.FromError(errors.New("error: no changes were expected but changes occurred"))
		default:
			return nil
		}
	}

	// up implementation used when the source of the Pulumi program is a template name or a URL to a template.
	upTemplateNameOrURL := func(templateNameOrURL string, opts backend.UpdateOptions) result.Result {
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
		if err := validateSecretsProvider(secretsProvider); err != nil {
			return result.FromError(err)
		}

		// Create temp directory for the "virtual workspace".
		temp, err := ioutil.TempDir("", "pulumi-up-")
		if err != nil {
			return result.FromError(err)
		}
		defer func() {
			contract.IgnoreError(os.RemoveAll(temp))
		}()

		// Change the working directory to the "virtual workspace" directory.
		if err = os.Chdir(temp); err != nil {
			return result.FromError(errors.Wrap(err, "changing the working directory"))
		}

		// If a stack was specified via --stack, see if it already exists.
		var name string
		var description string
		var s backend.Stack
		if stack != "" {
			if s, name, description, err = getStack(stack, opts.Display); err != nil {
				return result.FromError(err)
			}
		}

		// Prompt for the project name, if we don't already have one from an existing stack.
		if name == "" {
			defaultValue := workspace.ValueOrSanitizedDefaultProjectName(name, template.ProjectName, template.Name)
			name, err = promptForValue(
				yes, "project name", defaultValue, false, workspace.ValidateProjectName, opts.Display)
			if err != nil {
				return result.FromError(err)
			}
		}

		// Prompt for the project description, if we don't already have one from an existing stack.
		if description == "" {
			defaultValue := workspace.ValueOrDefaultProjectDescription(
				description, template.ProjectDescription, template.Description)
			description, err = promptForValue(
				yes, "project description", defaultValue, false, workspace.ValidateProjectDescription, opts.Display)
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
			return result.FromError(errors.Wrap(err, "saving project"))
		}

		// Create the stack, if needed.
		if s == nil {
			if s, err = promptAndCreateStack(promptForValue, stack, name, false /*setCurrent*/, yes,
				opts.Display, secretsProvider); err != nil {
				return result.FromError(err)
			}
			// The backend will print "Created stack '<stack>'." on success.
		}

		// Prompt for config values (if needed) and save.
		if err = handleConfig(s, templateNameOrURL, template, configArray, yes, path, opts.Display); err != nil {
			return result.FromError(err)
		}

		// Install dependencies.
		if err = installDependencies(proj, root); err != nil {
			return result.FromError(err)
		}

		m, err := getUpdateMetadata(message, root, execKind, execAgent)
		if err != nil {
			return result.FromError(errors.Wrap(err, "gathering environment metadata"))
		}

		sm, err := getStackSecretsManager(s)
		if err != nil {
			return result.FromError(errors.Wrap(err, "getting secrets manager"))
		}

		cfg, err := getStackConfiguration(s, sm)
		if err != nil {
			return result.FromError(errors.Wrap(err, "getting stack configuration"))
		}

		opts.Engine = engine.UpdateOptions{
			LocalPolicyPacks: engine.MakeLocalPolicyPacks(policyPackPaths, policyPackConfigPaths),
			Parallel:         parallel,
			Debug:            debug,
			Refresh:          refresh,
		}

		// TODO for the URL case:
		// - suppress preview display/prompt unless error.
		// - attempt `destroy` on any update errors.
		// - show template.Quickstart?

		changes, res := s.Update(commandContext(), backend.UpdateOperation{
			Proj:               proj,
			Root:               root,
			M:                  m,
			Opts:               opts,
			StackConfiguration: cfg,
			SecretsManager:     sm,
			Scopes:             cancellationScopes,
		})
		switch {
		case res != nil && res.Error() == context.Canceled:
			return result.FromError(errors.New("update cancelled"))
		case res != nil:
			return PrintEngineResult(res)
		case expectNop && changes != nil && changes.HasChanges():
			return result.FromError(errors.New("error: no changes were expected but changes occurred"))
		default:
			return nil
		}
	}

	var cmd = &cobra.Command{
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
			yes = yes || skipConfirmations()

			interactive := cmdutil.Interactive()
			if !interactive && !yes {
				return result.FromError(errors.New("--yes must be passed in to proceed when running in non-interactive mode"))
			}

			opts, err := updateFlagsToOptions(interactive, skipPreview, yes)
			if err != nil {
				return result.FromError(err)
			}

			if err = validatePolicyPackConfig(policyPackPaths, policyPackConfigPaths); err != nil {
				return result.FromError(err)
			}

			var displayType = display.DisplayProgress
			if diffDisplay {
				displayType = display.DisplayDiff
			}

			opts.Display = display.Options{
				Color:                cmdutil.GetGlobalColorization(),
				ShowConfig:           showConfig,
				ShowReplacementSteps: showReplacementSteps,
				ShowSameResources:    showSames,
				ShowReads:            showReads,
				SuppressOutputs:      suppressOutputs,
				IsInteractive:        interactive,
				Type:                 displayType,
				EventLogPath:         eventLogPath,
				Debug:                debug,
			}

			// we only suppress permalinks if the user passes true. the default is an empty string
			// which we pass as 'false'
			if suppressPermaLink == "true" {
				opts.Display.SuppressPermaLink = true
			} else {
				opts.Display.SuppressPermaLink = false
			}

			filestateBackend, err := isFilestateBackend(opts.Display)
			if err != nil {
				return result.FromError(err)
			}

			// by default, we are going to suppress the permalink when using self-managed backends
			// this can be re-enabled by explicitly passing "false" to the `supppress-permalink` flag
			if suppressPermaLink != "false" && filestateBackend {
				opts.Display.SuppressPermaLink = true
			}

			if len(args) > 0 {
				return upTemplateNameOrURL(args[0], opts)
			}

			return upWorkingDirectory(opts)
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().BoolVar(
		&expectNop, "expect-no-changes", false,
		"Return an error if any changes occur during this update")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().StringVar(
		&stackConfigFile, "config-file", "",
		"Use the configuration values in the specified file rather than detecting the file name")
	cmd.PersistentFlags().StringArrayVarP(
		&configArray, "config", "c", []string{},
		"Config to use during the update")
	cmd.PersistentFlags().BoolVar(
		&path, "config-path", false,
		"Config keys contain a path to a property in a map or list to set")
	cmd.PersistentFlags().StringVar(
		&secretsProvider, "secrets-provider", "default", "The type of the provider that should be used to encrypt and "+
			"decrypt secrets (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault). Only"+
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
	cmd.PersistentFlags().IntVarP(
		&parallel, "parallel", "p", defaultParallel,
		"Allow P resource operations to run in parallel at once (1 for no parallelism). Defaults to unbounded.")
	cmd.PersistentFlags().BoolVarP(
		&refresh, "refresh", "r", false,
		"Refresh the state of the stack's resources before this update")
	cmd.PersistentFlags().BoolVar(
		&showConfig, "show-config", false,
		"Show configuration keys and variables")
	cmd.PersistentFlags().BoolVar(
		&showReplacementSteps, "show-replacement-steps", false,
		"Show detailed resource replacement creates and deletes instead of a single step")

	cmd.PersistentFlags().BoolVar(
		&showSames, "show-sames", false,
		"Show resources that don't need be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVar(
		&showReads, "show-reads", false,
		"Show resources that are being read in, alongside those being managed directly in the stack")

	cmd.PersistentFlags().BoolVarP(
		&skipPreview, "skip-preview", "f", false,
		"Do not perform a preview before performing the update")
	cmd.PersistentFlags().BoolVar(
		&suppressOutputs, "suppress-outputs", false,
		"Suppress display of stack outputs (in case they contain sensitive values)")
	cmd.PersistentFlags().StringVar(
		&suppressPermaLink, "suppress-permalink", "",
		"Suppress display of the state permalink")
	cmd.Flag("suppress-permalink").NoOptDefVal = "false"
	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Automatically approve and perform the update after previewing it")

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
	s backend.Stack,
	templateNameOrURL string,
	template workspace.Template,
	configArray []string,
	yes bool,
	path bool,
	opts display.Options) error {

	// Get the existing config. stackConfig will be nil if there wasn't a previous deployment.
	stackConfig, err := backend.GetLatestConfiguration(commandContext(), s)
	if err != nil && err != backend.ErrNoPreviousDeployment {
		return err
	}

	// Get the existing snapshot.
	snap, err := s.Snapshot(commandContext())
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
		c, err = promptForConfig(s, template.Config, commandLineConfig, stackConfig, yes, opts)
		if err != nil {
			return err
		}
	}

	// Save the config.
	if len(c) > 0 {
		if err = saveConfig(s, c); err != nil {
			return errors.Wrap(err, "saving config")
		}

		fmt.Println("Saved config")
		fmt.Println()
	}

	return nil
}

var (
	templateKey = config.MustMakeKey("pulumi", "template")
)

// isPreconfiguredEmptyStack returns true if the url matches the value of `pulumi:template` in stackConfig,
// the stackConfig values satisfy the config requirements of templateConfig, and the snapshot is empty.
// This is the state of an initial preconfigured empty stack (i.e. a stack that's been created and configured
// in the Pulumi Console).
func isPreconfiguredEmptyStack(
	url string,
	templateConfig map[string]workspace.ProjectTemplateConfigValue,
	stackConfig config.Map,
	snap *deploy.Snapshot) bool {

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
