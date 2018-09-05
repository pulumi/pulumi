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
	"context"
	"io/ioutil"
	"os"

	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

const (
	defaultParallel = 10
)

// nolint: vetshadow, intentionally disabling here for cleaner err declaration/assignment.
func newUpCmd() *cobra.Command {
	var debug bool
	var expectNop bool
	var message string
	var stack string
	var configArray []string

	// Flags for engine.UpdateOptions.
	var analyzers []string
	var diffDisplay bool
	var nonInteractive bool
	var parallel int
	var refresh bool
	var showConfig bool
	var showReplacementSteps bool
	var showSames bool
	var skipPreview bool
	var yes bool

	// up implementation used when the source of the Pulumi program is in the current working directory.
	upWorkingDirectory := func(opts backend.UpdateOptions) error {
		s, err := requireStack(stack, true, opts.Display, true /*setCurrent*/)
		if err != nil {
			return err
		}

		// Save any config values passed via flags.
		if len(configArray) > 0 {
			commandLineConfig, err := parseConfig(configArray)
			if err != nil {
				return err
			}

			if err = saveConfig(s.Ref().Name(), commandLineConfig); err != nil {
				return errors.Wrap(err, "saving config")
			}
		}

		proj, root, err := readProject()
		if err != nil {
			return err
		}

		m, err := getUpdateMetadata(message, root)
		if err != nil {
			return errors.Wrap(err, "gathering environment metadata")
		}

		opts.Engine = engine.UpdateOptions{
			Analyzers: analyzers,
			Parallel:  parallel,
			Debug:     debug,
			Refresh:   refresh,
		}

		changes, err := s.Update(commandContext(), backend.UpdateOperation{
			Proj:   proj,
			Root:   root,
			M:      m,
			Opts:   opts,
			Scopes: cancellationScopes,
		})
		switch {
		case err == context.Canceled:
			return errors.New("update cancelled")
		case err != nil:
			return PrintEngineError(err)
		case expectNop && changes != nil && changes.HasChanges():
			return errors.New("error: no changes were expected but changes occurred")
		default:
			return nil
		}
	}

	// up implementation used when the source of the Pulumi program is a URL.
	upURL := func(url string, opts backend.UpdateOptions) error {
		if !workspace.IsTemplateURL(url) {
			return errors.Errorf("%s is not a valid URL", url)
		}

		// Retrieve the template repo.
		repo, err := workspace.RetrieveTemplates(url, false)
		if err != nil {
			return err
		}
		defer func() {
			contract.IgnoreError(repo.Delete())
		}()

		// List the templates from the repo.
		templates, err := repo.Templates()
		if err != nil {
			return err
		}

		// Make sure only a single template is found.
		// Alternatively, we could consider prompting to choose one instead of failing.
		if len(templates) != 1 {
			return errors.Errorf("more than one application found at %s", url)
		}
		template := templates[0]

		// Create temp directory for the "virtual workspace".
		temp, err := ioutil.TempDir("", "pulumi-up-")
		if err != nil {
			return err
		}
		defer func() {
			contract.IgnoreError(os.RemoveAll(temp))
		}()

		// Get the project name/description.
		projectName := workspace.ValueOrSanitizedDefaultProjectName("", template.ProjectName, template.Name)
		projectDescription := workspace.ValueOrDefaultProjectDescription(
			"", template.ProjectDescription, template.Description)

		// Copy the template files from the repo to the temporary "virtual workspace" directory.
		if err = template.CopyTemplateFiles(temp, true, projectName, projectDescription); err != nil {
			return err
		}

		// Change the working directory to the "virtual workspace" directory.
		if err = os.Chdir(temp); err != nil {
			return errors.Wrap(err, "changing the working directory")
		}

		// Load the project, update the name & description, and save it.
		proj, _, err := readProject()
		if err != nil {
			return err
		}
		proj.Name = tokens.PackageName(projectName)
		proj.Description = &projectDescription
		if err = workspace.SaveProject(proj); err != nil {
			return errors.Wrap(err, "saving project")
		}

		// Get a stack, but don't set it as the current stack, to avoid writing to ~/.pulumi/workspaces.
		s, err := requireStack(stack, true, opts.Display, false /*setCurrent*/)
		if err != nil {
			return err
		}

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
		if isPreconfiguredEmptyStack(url, template.Config, stackConfig, snap) {
			c = stackConfig
			// TODO consider warning if the specific URL is different from templateURL.
		} else {
			// Get config values passed on the command line.
			commandLineConfig, err := parseConfig(configArray)
			if err != nil {
				return err
			}

			// Prompt for config as needed.
			c, err = promptForConfig(s, template.Config, commandLineConfig, stackConfig, yes, opts.Display)
			if err != nil {
				return err
			}
		}

		// Save the config locally.
		if c != nil {
			if err = saveConfig(s.Ref().Name(), c); err != nil {
				return errors.Wrap(err, "saving config")
			}
		}

		// Install dependencies.
		if err = installDependencies("Installing dependencies..."); err != nil {
			return err
		}

		proj, root, err := readProject()
		if err != nil {
			return err
		}

		m, err := getUpdateMetadata(message, root)
		if err != nil {
			return errors.Wrap(err, "gathering environment metadata")
		}

		opts.Engine = engine.UpdateOptions{
			Analyzers: analyzers,
			Parallel:  parallel,
			Debug:     debug,
			Refresh:   refresh,
		}

		// TODO for the URL case:
		// - suppress preview display/prompt unless error.
		// - attempt `destroy` on any update errors.
		// - show template.Quickstart?

		changes, err := s.Update(commandContext(), backend.UpdateOperation{
			Proj:   proj,
			Root:   root,
			M:      m,
			Opts:   opts,
			Scopes: cancellationScopes,
		})
		switch {
		case err == context.Canceled:
			return errors.New("update cancelled")
		case err != nil:
			return PrintEngineError(err)
		case expectNop && changes != nil && changes.HasChanges():
			return errors.New("error: no changes were expected but changes occurred")
		default:
			return nil
		}
	}

	var cmd = &cobra.Command{
		Use:        "up [url]",
		Aliases:    []string{"update"},
		SuggestFor: []string{"deploy", "push"},
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
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			interactive := isInteractive(nonInteractive)
			if !interactive {
				yes = true // auto-approve changes, since we cannot prompt.
			}

			opts, err := updateFlagsToOptions(interactive, skipPreview, yes)
			if err != nil {
				return err
			}

			opts.Display = display.Options{
				Color:                cmdutil.GetGlobalColorization(),
				ShowConfig:           showConfig,
				ShowReplacementSteps: showReplacementSteps,
				ShowSameResources:    showSames,
				IsInteractive:        interactive,
				DiffDisplay:          diffDisplay,
				Debug:                debug,
			}

			if len(args) > 0 {
				return upURL(args[0], opts)
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
	cmd.PersistentFlags().StringArrayVarP(
		&configArray, "config", "c", []string{},
		"Config to use during the update")

	cmd.PersistentFlags().StringVarP(
		&message, "message", "m", "",
		"Optional message to associate with the update operation")

	// Flags for engine.UpdateOptions.
	cmd.PersistentFlags().StringSliceVar(
		&analyzers, "analyzer", []string{},
		"Run one or more analyzers as part of this update")
	cmd.PersistentFlags().BoolVar(
		&diffDisplay, "diff", false,
		"Display operation as a rich diff showing the overall change")
	cmd.PersistentFlags().BoolVar(
		&nonInteractive, "non-interactive", false, "Disable interactive mode")
	cmd.PersistentFlags().IntVarP(
		&parallel, "parallel", "p", defaultParallel,
		"Allow P resource operations to run in parallel at once (<=1 for no parallelism)")
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
		&skipPreview, "skip-preview", false,
		"Do not perform a preview before performing the update")
	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Automatically approve and perform the update after previewing it")

	return cmd
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
	stackResource, _ := stack.GetRootStackResource(snap)
	if stackResource == nil {
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
