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

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
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
	var showConfig bool
	var showReplacementSteps bool
	var showSames bool
	var skipPreview bool
	var yes bool

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

			opts.Display = backend.DisplayOptions{
				Color:                cmdutil.GetGlobalColorization(),
				ShowConfig:           showConfig,
				ShowReplacementSteps: showReplacementSteps,
				ShowSameResources:    showSames,
				IsInteractive:        interactive,
				DiffDisplay:          diffDisplay,
				Debug:                debug,
			}

			var c config.Map
			var s backend.Stack
			hasStack := false

			if len(args) > 0 {
				url := args[0]

				if !workspace.IsTemplateURL(url) {
					return errors.Errorf("%s is not a valid URL", url)
				}

				// Retrieve the template repo.
				repo, err := workspace.RetrieveTemplates(url, false)
				if err != nil {
					return err
				}

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

				// TODO don't use template name/description for project name/description.
				// Consider prompting if they are ${PROJECT} and ${DESCRIPTION}.
				if err = template.CopyTemplateFiles(temp, true, template.Name, template.Description); err != nil {
					return err
				}

				// Delete the template repo.
				if err = repo.Delete(); err != nil {
					return err
				}

				// Change the working directory to the "virtual workspace".
				if err = os.Chdir(temp); err != nil {
					return errors.Wrap(err, "changing the working directory")
				}

				// If it's an existing stack, use it to fetch the latest config.
				var stackConfig config.Map
				if stack != "" {
					s, err = getStack(stack)
					// TODO: if not found, offer to create it.
					if err != nil {
						return err
					}
					hasStack = true

					if stackConfig, err = backend.GetLatestConfiguration(commandContext(), s); err != nil {
						return err
					}

					// TODO: if the stack exists, pull down the latest snapshot and see if it was
					// the initial deployment used to save the config from the service and use
					// that config as-is without prompting.
				}

				// Get config values passed on the command line.
				commandLineConfig, err := parseConfig(configArray)
				if err != nil {
					return err
				}

				// Prompt for config as needed.
				c, err = promptForConfig(template.Config, commandLineConfig, stackConfig, yes, opts.Display)
				if err != nil {
					return err
				}

				// Install dependencies.
				if err = installDependencies(); err != nil {
					return err
				}
			}

			if !hasStack {
				if s, err = requireStack(stack, true, opts.Display); err != nil {
					return err
				}
			}

			if c != nil {
				if err = saveConfig(s.Name().StackName(), c); err != nil {
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
			}

			// TODO for the URL case:
			// - suppress preview display/prompt unless error.
			// - attempt `destroy` on any update errors.
			// - show template.Quickstart?

			changes, err := s.Update(commandContext(), proj, root, m, opts, cancellationScopes)
			switch {
			case err == context.Canceled:
				return errors.New("update cancelled")
			case err != nil:
				return err
			case expectNop && changes != nil && changes.HasChanges():
				return errors.New("error: no changes were expected but changes occurred")
			default:
				return nil
			}
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
		&parallel, "parallel", "p", 0,
		"Allow P resource operations to run in parallel at once (<=1 for no parallelism)")
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

// getStack gets the stack from the current backend.
func getStack(stackName string) (backend.Stack, error) {
	opts := backend.DisplayOptions{
		Color: cmdutil.GetGlobalColorization(),
	}

	b, err := currentBackend(opts)
	if err != nil {
		return nil, err
	}

	stackRef, err := b.ParseStackReference(stackName)
	if err != nil {
		return nil, err
	}

	stack, err := b.GetStack(commandContext(), stackRef)
	if err != nil {
		return nil, err
	}
	if stack != nil {
		return stack, err
	}

	return nil, errors.Errorf("no stack named '%s' found", stackName)
}
