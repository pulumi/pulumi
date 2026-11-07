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
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

const (
	confirmYes    = "Yes, create the project"
	confirmChange = "Change these values"
)

var errNoProjectCreated = errors.New("no project created; please use `pulumi new` to start again")

// setupAnswers is what `pulumi new` learned from its questions, through either the guided
// confirmation or the sequential prompts.
type setupAnswers struct {
	name        string
	description string
	// guided is set when the guided confirmation showed these answers.
	guided bool
	// stackName is the stack to create. When empty, the stack is the one named by --stack, or is
	// asked for when it is created.
	stackName string
	// config is the template config gathered for the new stack. When nil, it is asked for once the
	// stack exists.
	config *askedConfig
	// runtimeOptions answer the language runtime's questions, such as which package manager to use.
	runtimeOptions map[string]any
	// secrets is the new stack's secrets manager, prepared so its passphrase is asked with the other
	// questions. Nil when no stack is created, or with --force.
	secrets *cmdStack.NewStackSecrets
}

// askedConfig is template config gathered before the stack exists, held as plaintext until the
// stack's secrets manager can encrypt it.
type askedConfig struct {
	values            []templateConfigValue
	commandLineConfig config.Map
}

// createStack creates the stack named by the answers, by --stack, or by a prompt, in that order. It
// returns the stack and, when the answers named it, its full name.
func (c *setupAnswers) createStack(
	ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, b backend.Backend,
	root string, args newArgs, opts display.Options,
) (backend.Stack, string, error) {
	createOpts := cmdStack.CreateStackOptions{
		SetCurrent:         true,
		SecretsProvider:    args.secretsProvider,
		UseRemoteConfig:    args.remoteStackConfig,
		ManagedEnvironment: args.escConfig,
		Secrets:            c.secrets,
		// The confirmation already showed the stack name, and the summary repeats it and names any
		// managed environment.
		Quiet: c.guided && c.stackName != "",
	}

	var s backend.Stack
	var name string
	var err error
	if c.stackName != "" {
		s, name, err = createStackWithRetry(ctx, sink, args.stdout, ws, b, args.prompt, c.stackName, root,
			args.yes, opts, createOpts)
	} else {
		s, err = promptAndCreateStack(ctx, sink, ws, b, args.prompt, args.stack, root, args.yes, opts, createOpts)
	}
	if err != nil {
		return nil, "", err
	}
	if !createOpts.Quiet {
		// cmdStack.CreateStack prints "Created stack '<stack>'" on success.
		fmt.Fprintln(args.stdout)
	}
	return s, name, nil
}

// saveConfig saves the config gathered with the answers or, when there is none, asks for it and
// saves it.
func (c *setupAnswers) saveConfig(
	ctx context.Context, sink diag.Sink, ssml cmdStack.SecretsManagerLoader, ws pkgWorkspace.Context,
	proj *workspace.Project, s backend.Stack, template cmdTemplates.ProjectTemplate,
	args newArgs, opts display.Options,
) (config.Map, error) {
	if c.config != nil {
		return saveTemplateConfig(ctx, sink, ssml, ws, proj, s, c.config.values, c.config.commandLineConfig, "")
	}
	return handleConfig(ctx, sink, ssml, ws, args.prompt, proj, s,
		args.templateNameOrURL, template, args.configArray, args.yes, args.configPath, opts, "")
}

// askSetupQuestions asks what `pulumi new` needs to know about the project. Nothing has been written
// when it returns, so a cancelled question leaves nothing behind.
func askSetupQuestions(
	ctx context.Context, ws pkgWorkspace.Context, b backend.Backend, orgName string, existing backend.Stack,
	template cmdTemplates.ProjectTemplate, args newArgs, opts display.Options, cwd string,
) (*setupAnswers, error) {
	defaults := resolveProjectDefaults(ctx, b, orgName, template, args, opts, cwd)
	answers, err := confirmGuidedValues(ctx, b, orgName, defaults, template, args, opts)
	if err != nil {
		return nil, err
	}
	createsStack := !args.generateOnly && existing == nil
	if answers == nil {
		answers = &setupAnswers{}
		if answers.name, answers.description, err = promptSequentialValues(
			ctx, b, orgName, defaults, args, opts,
		); err != nil {
			return nil, err
		}
		if createsStack && args.stack == "" {
			// As in confirmGuidedValues: a DIY backend needs a current project to validate a stack name.
			b.SetCurrentProject(&workspace.Project{Name: tokens.PackageName(answers.name)})
			if answers.stackName, err = promptStackName(
				args.stdout, b, args.prompt, defaultStackName, args.yes, opts, b.ValidateStackName,
			); err != nil {
				return nil, err
			}
		}
	}
	// Ask for config if nothing has yet: the sequential prompts never do, and the guided
	// confirmation does only when it shows the stack name (not with --stack).
	if createsStack && answers.config == nil {
		if answers.config, err = answers.resolveConfigDefaults(template, args); err != nil {
			return nil, err
		}
		if err = askTemplateConfig(answers.config.values, args.prompt, args.yes, askAll, opts); err != nil {
			return nil, err
		}
	}
	if answers.runtimeOptions, err = promptTemplateRuntimeOptions(ctx, ws, template, args, opts); err != nil {
		return nil, err
	}
	// With --force the stack's config file may already exist, and its secrets settings win at stack
	// creation, so a passphrase asked now would be thrown away.
	if createsStack && !args.force {
		if answers.secrets, err = cmdStack.PrepareNewStackSecrets(ctx, b, args.secretsProvider); err != nil {
			return nil, err
		}
	}
	return answers, nil
}

// promptTemplateRuntimeOptions asks the template's language runtime for its options. The language host
// runs in the template directory because the project directory is still empty; runNew starts another
// in the project directory for Template and the installs.
func promptTemplateRuntimeOptions(
	ctx context.Context, ws pkgWorkspace.Context, template cmdTemplates.ProjectTemplate,
	args newArgs, opts display.Options,
) (map[string]any, error) {
	if args.promptRuntimeOptions == nil {
		return nil, nil
	}
	proj, err := workspace.LoadProject(filepath.Join(template.Dir, "Pulumi.yaml"))
	if err != nil {
		return nil, err
	}
	if proj.Runtime.Name() == "" {
		return nil, nil
	}
	if err := setRuntimeOptionFlags(&proj.Runtime, args.runtimeOptions); err != nil {
		return nil, err
	}

	pluginHost, pluginCtx, entryPoint, err := newProjectPluginContext(ctx, ws, proj, template.Dir)
	if err != nil {
		return nil, err
	}
	defer contract.IgnoreClose(pluginHost)
	defer pluginCtx.Close()

	lang, err := pluginCtx.Host.LanguageRuntime(pluginCtx, proj.Runtime.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to load language plugin %s: %w", proj.Runtime.Name(), err)
	}
	defer lang.Close()

	return args.promptRuntimeOptions(pluginCtx, lang, &proj.Runtime, entryPoint, opts,
		args.yes, args.interactive, args.prompt)
}

func printPreamble(w io.Writer, opts display.Options) {
	fmt.Fprintln(w, "This command will walk you through creating a new Pulumi project.")
	fmt.Fprintln(w)
	fmt.Fprintln(w,
		opts.Color.Colorize(
			colors.Highlight("Enter a value or leave blank to accept the (default), and press <ENTER>.",
				"<ENTER>", colors.SpecPrompt),
		))
	fmt.Fprintln(w,
		opts.Color.Colorize(
			colors.Highlight("Press ^C at any time to quit.", "^C", colors.SpecPrompt),
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
	hasAtLeastOnePrompt := (args.name == "") || (args.description == "") || (!args.generateOnly && args.stack == "")
	if !args.yes && hasAtLeastOnePrompt {
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
		if name, err = promptProjectName(ctx, b, orgName, defaultName, args, opts); err != nil {
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

func promptProjectName(
	ctx context.Context, b backend.Backend, orgName, defaultName string,
	args newArgs, opts display.Options,
) (string, error) {
	validate := func(s string) error {
		return validateProjectName(ctx, b, orgName, s, args.generateOnly, opts)
	}
	return args.prompt(args.yes, "Project name", defaultName, false, validate, opts)
}

// A nil result means this run is not on the guided path, and the sequential prompts should run
// instead.
func confirmGuidedValues(
	ctx context.Context, b backend.Backend, orgName string, defaults projectDefaults,
	template cmdTemplates.ProjectTemplate, args newArgs, opts display.Options,
) (*setupAnswers, error) {
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

	c := &setupAnswers{name: defaults.name, description: defaults.description, guided: true}

	var err error
	projectIsNew := shows.name && defaults.nameErr == nil
	if shows.name && defaults.nameErr != nil {
		if c.name, err = promptProjectName(ctx, b, orgName, c.name, args, opts); err != nil {
			return nil, err
		}
	}
	if shows.stack {
		// A DIY backend validates a stack name without a project (e.g. `prod`) against its current
		// project, which isn't saved yet. Set it before any stack-name prompt: the one below for a taken
		// default, and the one after "Change these values". Any project name will do, even if renamed.
		b.SetCurrentProject(&workspace.Project{Name: tokens.PackageName(c.name)})
		if c.stackName, err = buildStackName(ctx, b, defaultStackName); err != nil {
			return nil, err
		}
		// A project the backend has never heard of has no stacks to collide with.
		if !projectIsNew {
			if err = c.avoidExistingStack(ctx, b, args, opts); err != nil {
				return nil, err
			}
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
		return nil, err
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
		if c.stackName, err = promptStackName(
			args.stdout, b, args.prompt, c.stackName, false, opts, b.ValidateStackName,
		); err != nil {
			return nil, err
		}
		if err = c.repromptConfig(template, args, opts); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// avoidExistingStack asks for another stack name when the default is already taken. The answer
// can still go stale before creation, which createStackWithRetry handles.
func (c *setupAnswers) avoidExistingStack(
	ctx context.Context, b backend.Backend, args newArgs, opts display.Options,
) error {
	org, stack := splitStackName(c.stackName)
	taken, err := projectStackNames(ctx, b, org, c.name)
	if err != nil || !taken[stack] {
		// A backend that cannot list the project's stacks leaves the collision to creation time.
		return nil
	}

	fmt.Fprintf(args.stdout, "\nStack '%s' already exists.\n", c.stackName)
	validate := func(s string) error {
		if err := b.ValidateStackName(s); err != nil {
			return err
		}
		if _, name := splitStackName(s); taken[name] {
			return errors.New("a stack with that name already exists in this project")
		}
		return nil
	}
	name, err := promptStackName(args.stdout, b, args.prompt, "", args.yes, opts, validate)
	if err != nil {
		return err
	}
	c.stackName, err = buildStackName(ctx, b, name)
	return err
}

func projectStackNames(
	ctx context.Context, b backend.Backend, orgName, projectName string,
) (map[string]bool, error) {
	filter := backend.ListStackNamesFilter{Project: &projectName}
	if orgName != "" {
		filter.Organization = &orgName
	}

	names := map[string]bool{}
	var token backend.ContinuationToken
	for {
		refs, next, err := b.ListStackNames(ctx, filter, token)
		if err != nil {
			return nil, err
		}
		for _, ref := range refs {
			names[ref.Name().String()] = true
		}
		if next == nil {
			return names, nil
		}
		token = next
	}
}

func splitStackName(stackName string) (string, string) {
	if i := strings.LastIndex(stackName, "/"); i >= 0 {
		return stackName[:i], stackName[i+1:]
	}
	return "", stackName
}

// resolveConfigDefaults resolves defaults under the project name chosen so far, including any key
// declared without a namespace.
func (c *setupAnswers) resolveConfigDefaults(
	template cmdTemplates.ProjectTemplate, args newArgs,
) (*askedConfig, error) {
	projectName := tokens.PackageName(c.name)

	commandLineConfig, err := ParseConfigForProject(projectName, args.configArray, args.configPath)
	if err != nil {
		return nil, err
	}
	values, err := resolveTemplateConfig(projectName, template.Config, commandLineConfig, nil, nil)
	if err != nil {
		return nil, err
	}
	return &askedConfig{values: values, commandLineConfig: commandLineConfig}, nil
}

// promptUnsetConfig asks only for unset keys, so accepting the confirmation leaves nothing to ask.
func (c *setupAnswers) promptUnsetConfig(
	template cmdTemplates.ProjectTemplate, args newArgs, opts display.Options,
) error {
	asked, err := c.resolveConfigDefaults(template, args)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(asked.values, templateConfigValue.unset) {
		fmt.Fprintln(args.stdout)
	}
	if err = askTemplateConfig(asked.values, args.prompt, args.yes, askUnset, opts); err != nil {
		return err
	}
	c.config = asked
	return nil
}

// repromptConfig re-asks every key: the project name may have changed, re-resolving keys declared
// without a namespace.
func (c *setupAnswers) repromptConfig(
	template cmdTemplates.ProjectTemplate, args newArgs, opts display.Options,
) error {
	prior := make(map[string]string, len(c.config.values))
	for _, v := range c.config.values {
		if !v.fromFlag {
			prior[v.templateKey] = v.value
		}
	}
	asked, err := c.resolveConfigDefaults(template, args)
	if err != nil {
		return err
	}
	for i, v := range asked.values {
		if p, ok := prior[v.templateKey]; ok && p != "" && !v.fromFlag {
			asked.values[i].value = p
		}
	}
	if slices.ContainsFunc(asked.values, func(v templateConfigValue) bool { return !v.fromFlag }) {
		fmt.Fprintln(args.stdout)
	}
	if err = askTemplateConfig(asked.values, args.prompt, args.yes, askAll, opts); err != nil {
		return err
	}
	c.config = asked
	return nil
}

func (c *setupAnswers) configRows() []field {
	if c.config == nil {
		return nil
	}
	rows := make([]field, 0, len(c.config.values))
	for _, v := range c.config.values {
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
