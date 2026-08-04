// Copyright 2026, Pulumi Corporation.
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

package auth

import (
	"context"
	"fmt"
	"io"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/pkg/browser"

	pkgBackend "github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/project/newcmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

const (
	getStartedURL    = "https://www.pulumi.com/docs/get-started/"
	newProjectAnswer = "Create a new Pulumi project"
	guideAnswer      = "View the getting started guide"
	skipAnswer       = "Skip for now"
	runNewHint       = "To get started, run `pulumi new` in an empty directory"
)

// offerFirstStep offers a user with no stacks a path into their first project. It runs after login
// has succeeded, so it fails only when the user asked for something and that something failed.
func offerFirstStep(
	ctx context.Context,
	be pkgBackend.Backend,
	cwd string,
	out io.Writer,
	opts display.Options,
	interactive bool,
) error {
	if !interactive {
		return nil
	}

	stacks, _, err := be.ListStackNames(ctx, pkgBackend.ListStackNamesFilter{}, nil)
	if err != nil || len(stacks) > 0 {
		return nil
	}

	message := "\nYou don't have any stacks yet. What would you like to do?"
	message = opts.Color.Colorize(colors.SpecPrompt + message + colors.Reset)

	var answer string
	if err := survey.AskOne(&survey.Select{
		Message: message,
		Options: []string{newProjectAnswer, guideAnswer, skipAnswer},
	}, &answer, ui.SurveyIcons(opts.Color)); err != nil {
		// An interrupt at an optional prompt means "skip", not "fail the login".
		return nil
	}

	switch answer {
	case newProjectAnswer:
		return runNew(ctx, cwd, opts)
	case guideAnswer:
		fmt.Fprintf(out, "\n%s\n", getStartedURL)
		contract.IgnoreError(browser.OpenURL(getStartedURL))
		return nil
	default:
		fmt.Fprintf(out, "\n%s\n", runNewHint)
		return nil
	}
}

// runNew runs `pulumi new` in cwd, asking where to put the project when cwd already has files in
// it. `pulumi new` rejects a directory that isn't empty, so the answer is validated for us.
func runNew(ctx context.Context, cwd string, opts display.Options) error {
	// An empty, non-nil slice: cobra parses os.Args when a command's args are nil.
	args := []string{}
	if newcmd.ErrorIfNotEmptyDirectory(cwd) != nil {
		message := opts.Color.Colorize(colors.SpecPrompt +
			"Current directory is not empty. Please choose an empty directory:" + colors.Reset)

		var dir string
		if err := survey.AskOne(&survey.Input{
			Message: message,
			Help:    "The directory is created if it does not already exist.",
		}, &dir, survey.WithValidator(survey.Required), ui.SurveyIcons(opts.Color)); err != nil {
			return nil
		}
		args = []string{"--dir", dir}
	}

	newCmd := newcmd.NewNewCmd()
	newCmd.SetArgs(args)
	// `pulumi new`'s usage text and error reporting belong to `pulumi login`'s caller here.
	newCmd.SilenceUsage = true
	newCmd.SilenceErrors = true
	return newCmd.ExecuteContext(ctx)
}
