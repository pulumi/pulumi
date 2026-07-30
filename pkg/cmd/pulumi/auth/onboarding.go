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
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
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

// offerFirstStep offers a user with no stacks a path into their first project. A user with no
// stacks at all is treated as new: the CLI cannot distinguish creating an organization at signup
// from joining one, because the account details the backend returns carry only organization names.
//
// It is called after login has already succeeded, so it reports an error only when the user asked
// for something and that something failed: a backend that cannot tell us whether the account has
// stacks is treated as "say nothing" rather than as a failed login.
func offerFirstStep(
	ctx context.Context,
	be pkgBackend.Backend,
	cwd string,
	out io.Writer,
	opts display.Options,
	interactive bool,
) error {
	if !interactive || diy.IsDIYBackendURL(be.URL()) {
		return nil
	}

	stacks, _, err := be.ListStackNames(ctx, pkgBackend.ListStackNamesFilter{}, nil)
	if err != nil || len(stacks) > 0 {
		return nil
	}

	if newcmd.ErrorIfNotEmptyDirectory(cwd) != nil {
		fmt.Fprintf(out, "\n%s\n", runNewHint)
		return nil
	}

	return promptFirstStep(ctx, out, opts)
}

// promptFirstStep asks the user what they would like to do and carries out their choice.
func promptFirstStep(ctx context.Context, out io.Writer, opts display.Options) error {
	message := "\nYour organization is empty. What would you like to do?"
	message = opts.Color.Colorize(colors.SpecPrompt + message + colors.Reset)

	var answer string
	err := survey.AskOne(&survey.Select{
		Message: message,
		Options: []string{newProjectAnswer, guideAnswer, skipAnswer},
	}, &answer, ui.SurveyIcons(opts.Color))
	if err != nil {
		// An interrupt at an optional prompt means "skip", not "fail the login".
		return nil
	}

	switch answer {
	case newProjectAnswer:
		newCmd := newcmd.NewNewCmd()
		newCmd.SetArgs([]string{})
		// `pulumi new`'s usage text and error reporting belong to `pulumi login`'s caller here.
		newCmd.SilenceUsage = true
		newCmd.SilenceErrors = true
		return newCmd.ExecuteContext(ctx)
	case guideAnswer:
		fmt.Fprintf(out, "\n%s\n", getStartedURL)
		contract.IgnoreError(browser.OpenURL(getStartedURL))
		return nil
	default:
		fmt.Fprintf(out, "\n%s\n", runNewHint)
		return nil
	}
}
