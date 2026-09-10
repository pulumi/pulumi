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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/pkg/browser"

	pkgBackend "github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/project/newcmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

const (
	getStartedURL              = "https://www.pulumi.com/docs/get-started/"
	getStartedURLWithCLISource = getStartedURL + "?utm_source=cli"
	newProjectAnswer           = "Create a new Pulumi project"
	guideAnswer                = "View the getting started guide"
	skipAnswer                 = "Skip for now"
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

	message := opts.Color.Colorize(
		colors.SpecPrompt + "\n\rYou don't have any stacks yet. What would you like to do?" + colors.Reset,
	)

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
		fmt.Fprintln(out, "\nOpening the getting started guide in your web browser...")
		if err := browser.OpenURL(getStartedURLWithCLISource); err != nil {
			fmt.Fprintf(out, "\nWe couldn't launch your web browser for some reason. Please visit:\n\n"+
				"%s\n\nto get started.\n", getStartedURL)
		}
		return nil
	default:
		fmt.Fprintln(out, "\nTo get started, run `pulumi new` in an empty directory")
		return nil
	}
}

// runNew runs `pulumi new` in cwd, asking where to put the project when cwd already has files in
// it.
func runNew(ctx context.Context, cwd string, opts display.Options) error {
	// Cobra parses os.Args when a command's args are nil, so pass an empty slice instead.
	args := []string{}
	target := cwd
	if newcmd.ErrorIfNotEmptyDirectory(cwd) != nil {
		message := opts.Color.Colorize(colors.SpecPrompt +
			"\rCurrent directory is not empty. Enter or create an empty directory:" + colors.Reset)

		var dir string
		if err := survey.AskOne(&survey.Input{
			Message: message,
			Help:    "The directory is created if it does not already exist.",
			Suggest: suggestDirectories,
		}, &dir, survey.WithValidator(validateProjectDirectory), ui.SurveyIcons(opts.Color)); err != nil {
			return nil
		}
		args = []string{"--dir", dir}
		target = dir
	}

	// `pulumi new --dir` changes the working directory, so resolve the target while we still can.
	if abs, err := filepath.Abs(target); err == nil {
		target = abs
	}

	newCmd := newcmd.NewNewCmd()
	newCmd.SetArgs(args)
	// `pulumi new`'s usage text and error reporting belong to `pulumi login`'s caller here.
	newCmd.SilenceUsage = true
	newCmd.SilenceErrors = true

	// `pulumi new` can fail before it writes anything or long after, and returns a bare error
	// either way. We don't have to tell those apart: target was empty before this ran, so anything
	// in it now is a half-written project the user should know about.
	err := newCmd.ExecuteContext(ctx)
	if err != nil && newcmd.ErrorIfNotEmptyDirectory(target) != nil {
		return fmt.Errorf("%w\nYour new project in %s is incomplete", err, target)
	}
	return err
}

// suggestDirectories completes a partially typed path against the directories it could name, so
// the prompt answers <tab> the way a shell would.
func suggestDirectories(toComplete string) []string {
	// Glob only fails on a malformed pattern, for which no suggestions is the right answer.
	matches, _ := filepath.Glob(toComplete + "*")

	dirs := make([]string, 0, len(matches))
	for _, match := range matches {
		if info, err := os.Stat(match); err == nil && info.IsDir() {
			dirs = append(dirs, match)
		}
	}
	return dirs
}

// validateProjectDirectory rejects what `pulumi new` would reject, so the user can correct it at
// the prompt.
func validateProjectDirectory(answer any) error {
	dir, _ := answer.(string)
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return errors.New("please enter a directory")
	}

	// A directory that does not exist yet is the common answer; `pulumi new --dir` creates it.
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if newcmd.ErrorIfNotEmptyDirectory(dir) != nil {
		return fmt.Errorf("%s is not empty, please enter an empty or new directory", dir)
	}
	return nil
}
