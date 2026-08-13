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

package newcmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdConfig "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/config"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	cmdTemplates "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/templates"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

const (
	confirmYes    = "Yes, create the project"
	confirmChange = "Change these values"
)

var errConfirmationInterrupted = errors.New("no project created; please use `pulumi new` to start again")

// A nil *confirmedNew means the guided path did not run and the sequential prompts own the flow.
type confirmedNew struct {
	name        string
	description string
	// Empty when the confirmation did not cover the stack.
	stackName         string
	config            []templateConfigValue
	commandLineConfig config.Map
}

func (c *confirmedNew) createsStack() bool {
	return c != nil && c.stackName != ""
}

// createStack creates the stack confirmed earlier, falling through to the sequential
// prompt-and-create flow when the confirmation did not cover the stack.
func (c *confirmedNew) createStack(
	ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, b backend.Backend,
	root string, args newArgs, opts display.Options,
) (backend.Stack, string, error) {
	if c.createsStack() {
		// Quiet: the confirmation already showed the stack name, and the summary repeats it.
		return createStackWithRetry(ctx, sink, args.stdout, ws, b, args.prompt, c.stackName, root, args.yes, opts,
			cmdStack.CreateStackOptions{
				SetCurrent:      true,
				SecretsProvider: args.secretsProvider,
				UseRemoteConfig: args.remoteStackConfig,
				Quiet:           true,
			})
	}
	s, err := PromptAndCreateStack(ctx, sink, ws, b, args.prompt,
		args.stack, root, true /*setCurrent*/, args.yes, opts, args.secretsProvider,
		args.remoteStackConfig, "")
	if err != nil {
		return nil, "", err
	}
	// cmdStack.CreateStack prints "Created stack '<stack>'" on success.
	fmt.Fprintln(args.stdout)
	return s, "", nil
}

// saveConfig saves the config gathered at confirmation, falling through to the sequential
// prompt-and-save flow when the confirmation did not cover the stack.
func (c *confirmedNew) saveConfig(
	ctx context.Context, sink diag.Sink, ssml cmdStack.SecretsManagerLoader, ws pkgWorkspace.Context,
	proj *workspace.Project, s backend.Stack, template cmdTemplates.ProjectTemplate,
	args newArgs, opts display.Options,
) error {
	if c.createsStack() {
		return saveTemplateConfig(ctx, sink, ssml, ws, proj, s, c.config, c.commandLineConfig, "")
	}
	return HandleConfig(ctx, sink, ssml, ws, args.prompt, proj, s,
		args.templateNameOrURL, template, args.configArray, args.yes, args.configPath, opts, "")
}

func printPreamble(w io.Writer, opts display.Options) {
	fmt.Fprintln(w, "This command will walk you through creating a new Pulumi project.")
	fmt.Fprintln(w)
	fmt.Fprintln(w,
		opts.Color.Colorize(
			colors.Highlight("Enter a value or leave blank to accept the (default), and press <ENTER>.",
				"<ENTER>", colors.BrightCyan+colors.Bold),
		))
	fmt.Fprintln(w,
		opts.Color.Colorize(
			colors.Highlight("Press ^C at any time to quit.", "^C", colors.BrightCyan+colors.Bold),
		))
	fmt.Fprintln(w)
}

type projectDefaults struct {
	name        string
	description string
	// Non-nil when args.name was empty and the default name failed validation.
	nameErr error
}

func resolveProjectDefaults(
	ctx context.Context, b backend.Backend, orgName string,
	template cmdTemplates.ProjectTemplate, args newArgs, opts display.Options, cwd string,
) projectDefaults {
	d := projectDefaults{
		name: pkgWorkspace.ValueOrSanitizedDefaultProjectName(
			args.name, template.ProjectName, filepath.Base(cwd),
		),
		description: pkgWorkspace.ValueOrDefaultProjectDescription(
			args.description, template.ProjectDescription, template.Description,
		),
	}
	if args.name == "" {
		d.nameErr = validateProjectName(ctx, b, orgName, d.name, args.generateOnly, opts.WithIsInteractive(false))
	}
	return d
}

func promptSequentialValues(
	ctx context.Context, b backend.Backend, orgName string, defaults projectDefaults,
	args newArgs, opts display.Options,
) (string, string, error) {
	// The guided flow's questions already set the tone, so no preamble when falling back from it.
	hasAtLeastOnePrompt := (args.name == "") || (args.description == "") || (!args.generateOnly && args.stack == "")
	if !args.yes && hasAtLeastOnePrompt && !args.useGuidedFlow() {
		printPreamble(args.stdout, opts)
	}

	if defaults.nameErr != nil && args.yes {
		// If --yes is given error out now that the default value is invalid. If we allow prompt to catch
		// this case it can lead to a confusing error message.
		// See https://github.com/pulumi/pulumi/issues/8747.
		return "", "", fmt.Errorf("'%s' is not a valid project name. %w", defaults.name, defaults.nameErr)
	}

	return promptNameAndDescription(ctx, b, orgName, defaults.name, defaults.description, args, opts)
}

func promptNameAndDescription(
	ctx context.Context, b backend.Backend, orgName, defaultName, defaultDescription string,
	args newArgs, opts display.Options,
) (string, string, error) {
	name, description := args.name, args.description

	var err error
	if name == "" {
		validate := func(s string) error {
			return validateProjectName(ctx, b, orgName, s, args.generateOnly, opts)
		}
		if name, err = args.prompt(args.yes, "Project name", defaultName, false, validate, opts); err != nil {
			return "", "", err
		}
	}

	if description == "" {
		if description, err = args.prompt(
			args.yes, "Project description", defaultDescription, false, pkgWorkspace.ValidateProjectDescription, opts,
		); err != nil {
			return "", "", err
		}
	}

	return name, description, nil
}

// A nil result means this run is not on the guided path, or that its defaults are unusable, and
// the sequential prompts should run instead.
func confirmGuidedValues(
	ctx context.Context, b backend.Backend, orgName string, defaults projectDefaults,
	template cmdTemplates.ProjectTemplate, args newArgs, opts display.Options,
) (*confirmedNew, error) {
	if !args.useGuidedFlow() {
		return nil, nil
	}
	shows := struct{ name, description, stack bool }{
		name:        args.name == "",
		description: args.description == "",
		stack:       !args.generateOnly && args.stack == "",
	}
	if !shows.name && !shows.description && !shows.stack {
		return nil, nil
	}
	if shows.name && defaults.nameErr != nil {
		// The sequential prompts already handle an unusable default coherently.
		return nil, nil
	}

	c := &confirmedNew{name: defaults.name, description: defaults.description}

	var err error
	if shows.stack {
		if c.stackName, err = buildStackName(ctx, b, defaultStackName); err != nil {
			return nil, err
		}
		if err = c.promptUnsetConfig(template, args, opts); err != nil {
			return nil, err
		}
	}

	rows := make([]field, 0, 3)
	if shows.name {
		rows = append(rows, field{"Project name", c.name})
	}
	if shows.description {
		rows = append(rows, field{"Description", c.description})
	}
	if shows.stack {
		rows = append(rows, field{"Stack name", c.stackName})
	}

	accepted, err := askConfirmation(args.stdout, opts, args.selectOne, rows, c.configRows())
	if err != nil {
		return nil, friendlyInterrupt(err, errConfirmationInterrupted)
	}
	if accepted {
		return c, nil
	}

	// Declined: every prompt, in today's order, pre-filled with the values just shown.
	fmt.Fprintln(args.stdout)
	if c.name, c.description, err = promptNameAndDescription(
		ctx, b, orgName, c.name, c.description, args, opts,
	); err != nil {
		return nil, err
	}
	if shows.stack {
		if c.stackName, err = promptStackName(args.stdout, b, args.prompt, c.stackName, false, opts); err != nil {
			return nil, err
		}
		if err = c.repromptConfig(template, args, opts); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// resolveConfigDefaults resolves defaults under the project name chosen so far, including any key
// declared without a namespace.
func (c *confirmedNew) resolveConfigDefaults(
	template cmdTemplates.ProjectTemplate, args newArgs,
) ([]templateConfigValue, error) {
	projectName := tokens.PackageName(c.name)

	var err error
	if c.commandLineConfig, err = ParseConfigForProject(
		projectName, args.configArray, args.configPath,
	); err != nil {
		return nil, err
	}
	return resolveTemplateConfig(projectName, template.Config, c.commandLineConfig, nil, nil)
}

// promptUnsetConfig asks only for unset keys, so accepting the confirmation leaves nothing to ask.
func (c *confirmedNew) promptUnsetConfig(
	template cmdTemplates.ProjectTemplate, args newArgs, opts display.Options,
) error {
	values, err := c.resolveConfigDefaults(template, args)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(values, templateConfigValue.unset) {
		fmt.Fprintln(args.stdout)
	}
	if err = askTemplateConfig(values, args.prompt, args.yes, askUnset, opts); err != nil {
		return err
	}
	c.config = values
	return nil
}

// repromptConfig re-asks every key: the project name may have changed, re-resolving keys declared
// without a namespace.
func (c *confirmedNew) repromptConfig(
	template cmdTemplates.ProjectTemplate, args newArgs, opts display.Options,
) error {
	prior := make(map[string]string, len(c.config))
	for _, v := range c.config {
		if !v.fromFlag {
			prior[v.templateKey] = v.value
		}
	}
	values, err := c.resolveConfigDefaults(template, args)
	if err != nil {
		return err
	}
	for i, v := range values {
		if p, ok := prior[v.templateKey]; ok && p != "" && !v.fromFlag {
			values[i].value = p
		}
	}
	if slices.ContainsFunc(values, func(v templateConfigValue) bool { return !v.fromFlag }) {
		fmt.Fprintln(args.stdout)
	}
	if err = askTemplateConfig(values, args.prompt, args.yes, askAll, opts); err != nil {
		return err
	}
	c.config = values
	return nil
}

func (c *confirmedNew) configRows() []field {
	rows := make([]field, 0, len(c.config))
	for _, v := range c.config {
		value := v.value
		if v.secret {
			value = "[secret]"
		}
		rows = append(rows, field{cmdConfig.PrettyKeyForProject(v.key, tokens.PackageName(c.name)), value})
	}
	return rows
}

func askConfirmation(
	w io.Writer, opts display.Options, sel selectFunc, rows, configRows []field,
) (bool, error) {
	fmt.Fprintln(w)
	printFields(w, opts.Color, "", rows)
	if len(configRows) > 0 {
		fmt.Fprintln(w, opts.Color.Colorize(colors.SpecSubHeadline+"Config:"+colors.Reset))
		printFields(w, opts.Color, "  ", configRows)
	}
	i, err := sel("Do these look good?", []string{confirmYes, confirmChange}, opts)
	return i == 0, err
}

type field struct{ label, value string }

func printFields(w io.Writer, color colors.Colorization, indent string, fields []field) {
	width := 0
	for _, f := range fields {
		width = max(width, len(f.label))
	}
	for _, f := range fields {
		label := f.label + ":" + strings.Repeat(" ", width-len(f.label))
		fmt.Fprintln(w, indent+color.Colorize(colors.SpecSubHeadline+label+colors.Reset)+"  "+f.value)
	}
}
