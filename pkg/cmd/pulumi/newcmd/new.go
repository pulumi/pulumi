// Copyright 2016-2024, Pulumi Corporation.
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
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	survey "github.com/AlecAivazis/survey/v2"
	surveycore "github.com/AlecAivazis/survey/v2/core"
	"github.com/opentracing/opentracing-go"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	cmdTemplates "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/templates"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

type promptForValueFunc func(yes bool, valueType string, defaultValue string, secret bool,
	isValidFn func(value string) error, opts display.Options) (string, error)

type chooseTemplateFunc func(templates []cmdTemplates.Template, opts display.Options) (cmdTemplates.Template, error)

type runtimeOptionsFunc func(ctx *plugin.Context, info *workspace.ProjectRuntimeInfo, main string,
	opts display.Options, yes, interactive bool, prompt promptForValueFunc) (map[string]interface{}, error)

type promptForAIProjectURLFunc func(ctx context.Context,
	ws pkgWorkspace.Context, args newArgs, opts display.Options) (string, error)

type newArgs struct {
	configArray           []string
	configPath            bool
	description           string
	dir                   string
	force                 bool
	generateOnly          bool
	interactive           bool
	name                  string
	offline               bool
	prompt                promptForValueFunc
	promptRuntimeOptions  runtimeOptionsFunc
	promptForAIProjectURL promptForAIProjectURLFunc
	chooseTemplate        chooseTemplateFunc
	secretsProvider       string
	stack                 string
	templateNameOrURL     string
	yes                   bool
	listTemplates         bool
	aiPrompt              string
	aiLanguage            httpstate.PulumiAILanguage
	templateMode          bool
	runtimeOptions        []string
	remoteStackConfig     bool
}

func runNew(ctx context.Context, args newArgs) error {
	if !args.interactive && !args.yes {
		return errors.New("--yes must be passed in to proceed when running in non-interactive mode")
	}

	// Prepare options.
	opts := display.Options{
		Color:         cmdutil.GetGlobalColorization(),
		IsInteractive: args.interactive,
	}

	ssml := cmdStack.NewStackSecretsManagerLoaderFromEnv()
	ws := pkgWorkspace.Instance

	// Validate name (if specified) before further prompts/operations.
	if args.name != "" && pkgWorkspace.ValidateProjectName(args.name) != nil {
		return fmt.Errorf("'%s' is not a valid project name: %w", args.name, pkgWorkspace.ValidateProjectName(args.name))
	}

	// Validate secrets provider type
	if err := cmdStack.ValidateSecretsProvider(args.secretsProvider); err != nil {
		return err
	}

	// Get the current working directory.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting the working directory: %w", err)
	}
	originalCwd := cwd

	// If dir was specified, ensure it exists and use it as the
	// current working directory.
	if args.dir != "" {
		cwd, err = UseSpecifiedDir(args.dir)
		if err != nil {
			return err
		}
	}

	// Return an error if the directory isn't empty.
	if !args.force {
		if err = ErrorIfNotEmptyDirectory(cwd); err != nil {
			return err
		}
	}

	// If we're going to be creating a stack, get the current backend, which
	// will kick off the login flow (if not already logged-in).
	var b backend.Backend
	if !args.generateOnly {
		// There is no current project at this point to pass into currentBackend
		b, err = cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, nil, opts)
		if err != nil {
			return err
		}

		// Check project name and stack reference project name are the same, we skip this check if
		// --generate-only is set because we're not going to actually use the --stack argument given.
		if err := compareStackProjectName(b, args.stack, args.name); err != nil {
			return err
		}
	}

	// Ensure the project doesn't already exist.
	if args.name != "" {
		// There is no --org flag at the moment. The backend determines the orgName value if it is "".
		if err := validateProjectName(ctx, b, "" /* orgName */, args.name, args.generateOnly, opts); err != nil {
			return err
		}
	}

	if args.templateNameOrURL == "" && args.promptForAIProjectURL != nil {
		aiURL, err := args.promptForAIProjectURL(ctx, ws, args, opts)
		if err != nil {
			return err
		}
		args.templateNameOrURL = aiURL
	}

	// Retrieve the template repo.
	scope := cmdTemplates.ScopeAll
	if args.offline {
		scope = cmdTemplates.ScopeLocal
	}
	templateSource := cmdTemplates.New(ctx,
		args.templateNameOrURL, scope, workspace.TemplateKindPulumiProject)
	defer func() { contract.IgnoreError(templateSource.Close()) }()

	// List the templates from the repo.
	templates, err := templateSource.Templates()
	if err != nil {
		return err
	}

	var cmdTemplate cmdTemplates.Template
	if len(templates) == 0 {
		return errors.New("no templates")
	} else if len(templates) == 1 {
		cmdTemplate = templates[0]
	} else {
		if cmdTemplate, err = args.chooseTemplate(templates, opts); err != nil {
			return err
		}
	}

	template, err := cmdTemplate.Download(ctx)
	if err != nil {
		return err
	}

	if template.Errored() {
		return fmt.Errorf("template '%s' is currently broken: %w", template.Name, template.Error)
	}

	// Do a dry run, if we're not forcing files to be overwritten.
	if !args.force {
		if err = workspace.CopyTemplateFilesDryRun(template.Dir, cwd, args.name); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("template '%s' not found: %w", args.templateNameOrURL, err)
			}
			return err
		}
	}

	// If a stack was specified via --stack, see if it already exists.
	// Only do the lookup for fully-qualified stack names `org/project/stack` because
	// otherwise `getStack` will fail to detect the project folder and fail.
	// The main purpose of this lookup is getting a proper start with a project
	// created via the web app.
	var s backend.Stack
	var orgName string
	if !args.generateOnly && args.stack != "" && strings.Count(args.stack, "/") == 2 {
		parts := strings.SplitN(args.stack, "/", 3)

		// Set the org name for future use.
		orgName = parts[0]
		projectName := parts[1]

		stackName, err := buildStackName(ctx, b, args.stack)
		if err != nil {
			return err
		}

		existingStack, _, existingDesc, err := GetStack(ctx, b, stackName, opts)
		if err != nil {
			return err
		}
		if existingStack != nil {
			s = existingStack
			if args.description == "" {
				args.description = existingDesc
			}
		}
		args.name = projectName
	}

	// Show instructions, if we're going to show at least one prompt.
	hasAtLeastOnePrompt := (args.name == "") || (args.description == "") || (!args.generateOnly && args.stack == "")
	if !args.yes && hasAtLeastOnePrompt {
		fmt.Println("This command will walk you through creating a new Pulumi project.")
		fmt.Println()
		fmt.Println(
			opts.Color.Colorize(
				colors.Highlight("Enter a value or leave blank to accept the (default), and press <ENTER>.",
					"<ENTER>", colors.BrightCyan+colors.Bold)))
		fmt.Println(
			opts.Color.Colorize(
				colors.Highlight("Press ^C at any time to quit.", "^C", colors.BrightCyan+colors.Bold)))
		fmt.Println()
	}

	// Prompt for the project name, if it wasn't already specified.
	if args.name == "" {
		defaultValue := pkgWorkspace.ValueOrSanitizedDefaultProjectName(args.name, template.ProjectName, filepath.Base(cwd))
		err := validateProjectName(
			ctx, b, orgName, defaultValue, args.generateOnly, opts.WithIsInteractive(false))
		if err != nil {
			// If --yes is given error out now that the default value is invalid. If we allow prompt to catch
			// this case it can lead to a confusing error message because we set the defaultValue to "" below.
			// See https://github.com/pulumi/pulumi/issues/8747.
			if args.yes {
				return fmt.Errorf("'%s' is not a valid project name. %w", defaultValue, err)
			}
		}
		validate := func(s string) error {
			return validateProjectName(ctx, b, orgName, s, args.generateOnly, opts)
		}
		args.name, err = args.prompt(args.yes, "Project name", defaultValue, false, validate, opts)
		if err != nil {
			return err
		}
	}

	// Prompt for the project description, if it wasn't already specified.
	if args.description == "" {
		defaultValue := pkgWorkspace.ValueOrDefaultProjectDescription(
			args.description, template.ProjectDescription, template.Description)
		args.description, err = args.prompt(
			args.yes, "Project description", defaultValue, false, pkgWorkspace.ValidateProjectDescription, opts)
		if err != nil {
			return err
		}
	}

	// Actually copy the files.
	if err = workspace.CopyTemplateFiles(template.Dir, cwd, args.force, args.name, args.description); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("template '%s' not found: %w", args.templateNameOrURL, err)
		}
		return err
	}

	fmt.Printf("Created project '%s'\n", args.name)
	fmt.Println()

	// Load the project, update the name & description, remove the template section, and save it.
	proj, root, err := ws.ReadProject()
	if err != nil {
		return err
	}
	proj.Name = tokens.PackageName(args.name)
	proj.Description = &args.description
	proj.Template = nil

	// Set the pulumi:template tag to the template name or URL.
	templateTag := template.Name
	if args.templateNameOrURL != "" {
		templateTag = sanitizeTemplate(args.templateNameOrURL)
	}
	proj.AddConfigStackTags(map[string]string{
		apitype.ProjectTemplateTag: templateTag,
	})

	for _, opt := range args.runtimeOptions {
		parts := strings.Split(strings.TrimSpace(opt), "=")
		if len(parts) != 2 {
			return fmt.Errorf("invalid runtime option: %s", opt)
		}
		proj.Runtime.SetOption(parts[0], parts[1])
	}

	if err = workspace.SaveProject(proj); err != nil {
		return fmt.Errorf("saving project: %w", err)
	}
	if b != nil {
		b.SetCurrentProject(proj)
	}

	appendFileName := "Pulumi.yaml.append"
	appendFile := filepath.Join(root, appendFileName)
	err = os.Remove(appendFile)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	// Create the stack, if needed.
	if !args.generateOnly && s == nil {
		if s, err = PromptAndCreateStack(ctx, cmdutil.Diag(), ws, b, args.prompt,
			args.stack, root, true /*setCurrent*/, args.yes, opts, args.secretsProvider, args.remoteStackConfig); err != nil {
			return err
		}
		// The backend will print "Created stack '<stack>'" on success.
		fmt.Println()
	}

	// Query the language runtime for additional options.
	if args.promptRuntimeOptions != nil {
		span := opentracing.SpanFromContext(ctx)
		projinfo := &engine.Projinfo{Proj: proj, Root: root}
		_, entryPoint, pluginCtx, err := engine.ProjectInfoContext(
			projinfo,
			nil,
			cmdutil.Diag(),
			cmdutil.Diag(),
			nil,
			false,
			span,
			nil,
		)
		if err != nil {
			return err
		}
		defer pluginCtx.Close()
		options, err := args.promptRuntimeOptions(pluginCtx, &proj.Runtime, entryPoint, opts,
			args.yes, args.interactive, args.prompt)
		if err != nil {
			return err
		}
		if len(options) > 0 {
			// Save the new options
			for k, v := range options {
				proj.Runtime.SetOption(k, v)
			}
			if err = workspace.SaveProject(proj); err != nil {
				return fmt.Errorf("saving project: %w", err)
			}
		}
	}

	// Prompt for config values (if needed) and save.
	if !args.generateOnly {
		err = HandleConfig(
			ctx,
			cmdutil.Diag(),
			ssml,
			ws,
			args.prompt,
			proj,
			s,
			args.templateNameOrURL,
			template,
			args.configArray,
			args.yes,
			args.configPath,
			opts,
		)
		if err != nil {
			return err
		}
	}

	// Ensure the stack is selected.
	if !args.generateOnly && s != nil {
		contract.IgnoreError(state.SetCurrentStack(s.Ref().FullyQualifiedName().String()))
	}

	// Install dependencies.
	if !args.generateOnly {
		span := opentracing.SpanFromContext(ctx)
		projinfo := &engine.Projinfo{Proj: proj, Root: root}
		_, entryPoint, pluginCtx, err := engine.ProjectInfoContext(
			projinfo,
			nil,
			cmdutil.Diag(),
			cmdutil.Diag(),
			nil,
			false,
			span,
			nil,
		)
		if err != nil {
			return err
		}

		defer pluginCtx.Close()

		if err := InstallDependencies(pluginCtx, &proj.Runtime, entryPoint); err != nil {
			return err
		}
	}

	fmt.Println(
		opts.Color.Colorize(
			colors.BrightGreen+colors.Bold+"Your new project is ready to go!"+colors.Reset) +
			" " + cmdutil.EmojiOr("✨", ""))
	fmt.Println()

	// Print out next steps.
	printNextSteps(proj, originalCwd, cwd, args.generateOnly, opts)

	if template.Quickstart != "" {
		fmt.Println(template.Quickstart)
	}

	return nil
}

// isInteractive lets us force interactive mode for testing by setting PULUMI_TEST_INTERACTIVE.
func isInteractive() bool {
	test, ok := os.LookupEnv("PULUMI_TEST_INTERACTIVE")
	return cmdutil.Interactive() || ok && cmdutil.IsTruthy(test)
}

// NewNewCmd creates a New command with default dependencies.
func NewNewCmd() *cobra.Command {
	args := newArgs{
		prompt:                ui.PromptForValue,
		chooseTemplate:        ChooseTemplate,
		promptRuntimeOptions:  promptRuntimeOptions,
		promptForAIProjectURL: promptForAIProjectURL,
	}

	getTemplates := func(ctx context.Context) ([]cmdTemplates.Template, io.Closer, error) {
		scope := cmdTemplates.ScopeAll
		if args.offline {
			scope = cmdTemplates.ScopeLocal
		}
		// Attempt to retrieve available templates.
		s := cmdTemplates.New(ctx, "", scope, workspace.TemplateKindPulumiProject)
		t, err := s.Templates()
		return t, s, err
	}

	cmd := &cobra.Command{
		Use:        "new [template|url]",
		SuggestFor: []string{"init", "create"},
		Short:      "Create a new Pulumi project",
		Long: "Create a new Pulumi project and stack from a template.\n" +
			"\n" +
			"To create a project from a specific template, pass the template name (such as `aws-typescript`\n" +
			"or `azure-python`). If no template name is provided, a list of suggested templates will be presented\n" +
			"which can be selected interactively.\n" +
			"For testing, a path to a local template may be passed instead (such as `~/templates/aws-typescript`)\n" +
			"\n" +
			"By default, a stack created using the pulumi.com backend will use the pulumi.com secrets\n" +
			"provider and a stack created using the local or cloud object storage backend will use the\n" +
			"`passphrase` secrets provider.  A different secrets provider can be selected by passing the\n" +
			"`--secrets-provider` flag.\n" +
			"\n" +
			"To use the `passphrase` secrets provider with the pulumi.com backend, use:\n" +
			"* `pulumi new --secrets-provider=passphrase`\n" +
			"\n" +
			"To use a cloud secrets provider with any backend, use one of the following:\n" +
			"* `pulumi new --secrets-provider=\"awskms://alias/ExampleAlias?region=us-east-1\"`\n" +
			"* `pulumi new --secrets-provider=\"awskms://1234abcd-12ab-34cd-56ef-1234567890ab?region=us-east-1\"`\n" +
			"* `pulumi new --secrets-provider=\"azurekeyvault://mykeyvaultname.vault.azure.net/keys/mykeyname\"`\n" +
			"* `pulumi new --secrets-provider=\"gcpkms://projects/p/locations/l/keyRings/r/cryptoKeys/k\"`\n" +
			"* `pulumi new --secrets-provider=\"hashivault://mykey\"`" +
			"\n\n" +
			"To create a project from a specific source control location, pass the url as follows e.g.\n" +
			"* `pulumi new https://gitlab.com/<user>/<repo>`\n" +
			"* `pulumi new https://bitbucket.org/<user>/<repo>`\n" +
			"* `pulumi new https://github.com/<user>/<repo>`\n" +
			"\n" +
			"  Note: If the URL doesn't follow the usual scheme of the given host (e.g. for GitLab subprojects)\n" +
			"        you can append `.git` to the repository to disambiguate and point to the correct repository.\n" +
			"        For example `https://gitlab.com/<project>/<subproject>/<repository>.git`.\n" +
			"\n" +
			"To create the project from a branch of a specific source control location, pass the url to the branch, e.g.\n" +
			"* `pulumi new https://gitlab.com/<user>/<repo>/tree/<branch>`\n" +
			"* `pulumi new https://bitbucket.org/<user>/<repo>/tree/<branch>`\n" +
			"* `pulumi new https://github.com/<user>/<repo>/tree/<branch>`\n" +
			"\n" +
			"To use a private repository as a template source, provide an HTTPS or SSH URL with relevant credentials.\n" +
			"Ensure your SSH agent has the correct identity (ssh-add) or you may be prompted for your key's passphrase.\n" +
			"* `pulumi new git@github.com:<user>/<private-repo>`\n" +
			"* `pulumi new https://<user>:<password>@<hostname>/<project>/<repo>`\n" +
			"* `pulumi new <user>@<hostname>:<project>/<repo>`\n" +
			"* `PULUMI_GITSSH_PASSPHRASE=<passphrase> pulumi new ssh://<user>@<hostname>/<project>/<repo>`\n" +
			"To create a project using Pulumi AI, either select `ai` from the first selection, " +
			"or provide any of the following:\n" +
			"* `pulumi new --ai \"<prompt>\"`\n" +
			"* `pulumi new --language <language>`\n" +
			"* `pulumi new --ai \"<prompt>\" --language <language>`\n" +
			"Any missing but required information will be prompted for.\n",
		Args: cmdutil.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, cliArgs []string) error {
			ctx := cmd.Context()
			if len(cliArgs) > 0 {
				args.templateNameOrURL = cliArgs[0]
			}
			if args.listTemplates {
				templates, closer, err := getTemplates(ctx)
				defer contract.IgnoreClose(closer)
				if err != nil {
					logging.Warningf("could not list templates: %v", err)
					return err
				}
				available, _ := templatesToOptionArrayAndMap(templates)
				fmt.Fprintln(cmd.OutOrStdout())
				fmt.Fprintln(cmd.OutOrStdout(), "Available Templates:")
				for _, t := range available {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", t)
				}
				return nil
			}

			args.yes = args.yes || env.SkipConfirmations.Value()
			args.interactive = isInteractive()
			return runNew(ctx, args)
		},
	}

	// Add additional help that includes a list of available templates.
	defaultHelp := cmd.HelpFunc()
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		// Show default help.
		defaultHelp(cmd, args)

		// You'd think you could use cmd.Context() here but cobra doesn't set context on the cmd even though
		// the parent help command has it. If https://github.com/spf13/cobra/issues/2240 gets fixed we can
		// change back to cmd.Context() here.
		templates, closer, err := getTemplates(context.Background())
		contract.IgnoreClose(closer)
		if err != nil {
			logging.Warningf("could not list templates: %v", err)
			return
		}

		// If we have any templates, show them.
		if len(templates) > 0 {
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintf(cmd.OutOrStdout(), "There are %d available templates.\n", len(templates))
		}
	})

	cmd.PersistentFlags().StringArrayVarP(
		&args.configArray, "config", "c", []string{},
		"Config to save")
	cmd.PersistentFlags().BoolVar(
		&args.configPath, "config-path", false,
		"Config keys contain a path to a property in a map or list to set")
	cmd.PersistentFlags().StringVarP(
		&args.description, "description", "d", "",
		"The project description; if not specified, a prompt will request it")
	cmd.PersistentFlags().StringVar(
		&args.dir, "dir", "",
		"The location to place the generated project; if not specified, the current directory is used")
	cmd.PersistentFlags().BoolVarP(
		&args.force, "force", "f", false,
		"Forces content to be generated even if it would change existing files")
	cmd.PersistentFlags().BoolVarP(
		&args.generateOnly, "generate-only", "g", false,
		"Generate the project only; do not create a stack, save config, or install dependencies")
	cmd.PersistentFlags().StringVarP(
		&args.name, "name", "n", "",
		"The project name; if not specified, a prompt will request it")
	cmd.PersistentFlags().BoolVarP(
		&args.offline, "offline", "o", false,
		"Use locally cached templates without making any network requests")
	cmd.PersistentFlags().StringVarP(
		&args.stack, "stack", "s", "",
		"The stack name; either an existing stack or stack to create; if not specified, a prompt will request it")
	cmd.PersistentFlags().BoolVarP(
		&args.yes, "yes", "y", false,
		"Skip prompts and proceed with default values")
	cmd.PersistentFlags().StringVar(
		&args.secretsProvider, "secrets-provider", "default", "The type of the provider that should be used to encrypt and "+
			"decrypt secrets (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault)")
	cmd.PersistentFlags().BoolVarP(
		&args.listTemplates, "list-templates", "l", false,
		"List locally installed templates and exit")
	cmd.PersistentFlags().StringVar(
		&args.aiPrompt, "ai", "", "Prompt to use for Pulumi AI",
	)
	cmd.PersistentFlags().Var(
		&args.aiLanguage, "language", "Language to use for Pulumi AI "+
			fmt.Sprintf("(must be one of %s)", httpstate.PulumiAILanguagesClause),
	)
	cmd.PersistentFlags().BoolVarP(
		&args.templateMode, "template-mode", "t", false,
		"Run in template mode, which will skip prompting for AI or Template functionality",
	)
	cmd.PersistentFlags().StringSliceVar(
		&args.runtimeOptions, "runtime-options", []string{},
		"Additional options for the language runtime (format: key1=value1,key2=value2)",
	)

	cmd.PersistentFlags().BoolVar(
		&args.remoteStackConfig, "remote-stack-config", false,
		"Store stack configuration remotely",
	)
	_ = cmd.PersistentFlags().MarkHidden("remote-stack-config")

	return cmd
}

func validateProjectName(ctx context.Context, b backend.Backend,
	orgName string, projectName string, generateOnly bool, opts display.Options,
) error {
	handleExistingProjectName := func(projectName string) error {
		prompt := "\b\n" + opts.Color.Colorize(colors.SpecPrompt+
			fmt.Sprintf("A project with the name `%s` already exists.", projectName)+
			colors.Reset)

		accept := fmt.Sprintf("Use '%s' anyway", projectName)
		retry := "Specify a different project name"
		response := ui.PromptUser(prompt, []string{accept, retry}, accept, opts.Color)

		if response != accept {
			return ui.ErrRetryPromptForValue
		}
		return nil
	}
	if !opts.IsInteractive {
		handleExistingProjectName = func(projectName string) error {
			return fmt.Errorf("a project with this name already exists: %s", projectName)
		}
	}
	return validateProjectNameInternal(
		ctx, b, orgName, projectName, generateOnly, opts, handleExistingProjectName)
}

func validateProjectNameInternal(ctx context.Context, b backend.Backend,
	orgName string, projectName string, generateOnly bool, opts display.Options,
	handleExistingProjectName func(string) error,
) error {
	err := pkgWorkspace.ValidateProjectName(projectName)
	if err != nil {
		return err
	}

	if !generateOnly {
		contract.Requiref(b != nil, "b", "must not be nil")

		exists, err := b.DoesProjectExist(ctx, orgName, projectName)
		if err != nil {
			return err
		}

		if exists {
			return handleExistingProjectName(projectName)
		}
	}

	return nil
}

func promptRuntimeOptions(ctx *plugin.Context, info *workspace.ProjectRuntimeInfo,
	main string, opts display.Options, yes, interactive bool, prompt promptForValueFunc,
) (map[string]interface{}, error) {
	programInfo := plugin.NewProgramInfo(ctx.Root, ctx.Pwd, main, info.Options())
	lang, err := ctx.Host.LanguageRuntime(info.Name(), programInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to load language plugin %s: %w", info.Name(), err)
	}
	defer lang.Close()

	options := make(map[string]interface{}, len(info.Options()))
	for k, v := range info.Options() {
		options[k] = v
	}

	// Keep querying for prompts until there are no more.
	for {
		// Update the program info with the latest runtime options.
		programInfo := plugin.NewProgramInfo(ctx.Root, ctx.Pwd, main, options)
		prompts, err := lang.RuntimeOptionsPrompts(programInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to get runtime options prompts: %w", err)
		}

		if len(prompts) == 0 {
			break
		}

		surveycore.DisableColor = true
		for _, optionPrompt := range prompts {
			if yes {
				if optionPrompt.Default == nil {
					return nil, fmt.Errorf("must provide a runtime option for '%s' in non-interactive mode", optionPrompt.Key)
				}
				options[optionPrompt.Key] = optionPrompt.Default.Value()
				continue
			}

			if len(optionPrompt.Choices) == 1 {
				// We got exactly one choice, so use that as default value without prompting the user.
				choice := optionPrompt.Choices[0]
				options[optionPrompt.Key] = choice.Value()
			} else if len(optionPrompt.Choices) > 0 {
				// Pick one among the choices
				choices := make([]string, 0, len(optionPrompt.Choices))
				// Map choice display string to the actual value
				choiceMap := make(map[string]interface{}, len(optionPrompt.Choices))
				for _, choice := range optionPrompt.Choices {
					displayName := choice.DisplayName
					if displayName == "" {
						displayName = choice.String()
					}
					choices = append(choices, displayName)
					choiceMap[displayName] = choice.Value()
				}

				var response string
				message := opts.Color.Colorize(colors.SpecPrompt + "\r" + optionPrompt.Description + colors.Reset)
				if err := survey.AskOne(&survey.Select{
					Message: message,
					Options: choices,
				}, &response, ui.SurveyIcons(opts.Color), nil); err != nil {
					return nil, err
				}
				options[optionPrompt.Key] = choiceMap[response]
			} else {
				// Free form input
				val, err := prompt(yes, optionPrompt.Description, "", false, makePromptValidator(optionPrompt), opts)
				if err != nil {
					return nil, err
				}
				r, err := plugin.RuntimeOptionValueFromString(optionPrompt.PromptType, val)
				if err != nil {
					return nil, err
				}
				options[optionPrompt.Key] = r.Value()
			}
		}
	}

	return options, nil
}

func promptForAIProjectURL(ctx context.Context, ws pkgWorkspace.Context, args newArgs,
	opts display.Options,
) (string, error) {
	// Try to read the current project
	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return "", err
	}

	b, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, project, opts)
	if err != nil {
		return "", err
	}

	var aiOrTemplate string
	if shouldPromptForAIOrTemplate(args, b) {
		aiOrTemplate, err = chooseWithAIOrTemplate(opts)
	} else {
		aiOrTemplate = deriveAIOrTemplate(args)
	}
	if err != nil {
		return "", err
	}
	if aiOrTemplate != "ai" {
		return "", nil
	}

	checkedBackend, ok := b.(httpstate.Backend)
	if !ok {
		if args.aiLanguage != "" && args.aiPrompt == "" {
			return "", errors.New(
				"--language is used to generate a template with Pulumi AI. " +
					"Please log in to Pulumi Cloud to use Pulumi AI.\n" +
					"Use --template to create a project from a template, " +
					"or no flags to choose one interactively.")
		}
		return "", errors.New("please log in to Pulumi Cloud to use Pulumi AI")
	}
	return runAINew(ctx, args, opts, checkedBackend)
}

func makePromptValidator(prompt plugin.RuntimeOptionPrompt) func(string) error {
	return func(value string) error {
		switch prompt.PromptType {
		case plugin.PromptTypeInt32:
			_, err := strconv.ParseInt(value, 10, 32)
			if err != nil {
				return fmt.Errorf("expected an integer value, got %q", value)
			}
		case plugin.PromptTypeString:
			// No validation needed for strings.
		default:
			return fmt.Errorf("unexpected prompt type: %v", prompt.PromptType)
		}
		return nil
	}
}

// printNextSteps prints out a series of commands that the user needs to run before their stack is able to be updated.
func printNextSteps(proj *workspace.Project, originalCwd, cwd string, generateOnly bool, opts display.Options) {
	var commands []string

	// If the target working directory is not the same as our current WD, tell the user to
	// CD to the target directory.
	if originalCwd != cwd {
		// If we can determine a relative path, use that, otherwise use the full path.
		var cd string
		if rel, err := filepath.Rel(originalCwd, cwd); err == nil {
			cd = rel
		} else {
			cd = cwd
		}

		// Surround the path with double quotes if it contains whitespace.
		if containsWhiteSpace(cd) {
			cd = fmt.Sprintf("\"%s\"", cd)
		}

		cd = "cd " + cd
		commands = append(commands, cd)
	}

	if generateOnly {
		// We didn't install dependencies, so instruct the user to do so.
		commands = append(commands, "pulumi install")
		// We didn't create a stack so show that as a command to run before `pulumi up`.
		commands = append(commands, "pulumi stack init")
	}

	if len(commands) == 0 { // No additional commands need to be run.
		deployMsg := "To perform an initial deployment, run `pulumi up`"
		deployMsg = colors.Highlight(deployMsg, "pulumi up", colors.BrightBlue+colors.Bold)
		fmt.Println(opts.Color.Colorize(deployMsg))
		fmt.Println()
		return
	}

	if len(commands) == 1 { // Only one additional command need to be run.
		deployMsg := fmt.Sprintf("To perform an initial deployment, run '%s', then, run `pulumi up`", commands[0])
		deployMsg = colors.Highlight(deployMsg, commands[0], colors.BrightBlue+colors.Bold)
		deployMsg = colors.Highlight(deployMsg, "pulumi up", colors.BrightBlue+colors.Bold)
		fmt.Println(opts.Color.Colorize(deployMsg))
		fmt.Println()
		return
	}

	// One or more additional commands needs to be run.
	fmt.Println("To perform an initial deployment, run the following commands:")
	fmt.Println()
	for i, cmd := range commands {
		cmdColors := colors.BrightBlue + colors.Bold + cmd + colors.Reset
		fmt.Printf("   %d. %s\n", i+1, opts.Color.Colorize(cmdColors))
	}
	fmt.Println()

	upMsg := colors.Highlight("Then, run `pulumi up`", "pulumi up", colors.BrightBlue+colors.Bold)
	fmt.Println(opts.Color.Colorize(upMsg))
	fmt.Println()
}

// containsWhiteSpace returns true if the string contains whitespace.
func containsWhiteSpace(value string) bool {
	for _, c := range value {
		if unicode.IsSpace(c) {
			return true
		}
	}
	return false
}

// compareStackProjectName takes a stack name and a project name and returns an error if they are not the same.
//   - projectName comes from the --name flag.
//   - stackName comes from the --stack flag. stackName can be a stack name or a fully qualified stack reference
//     i.e org/project/stack
func compareStackProjectName(b backend.Backend, stackName, projectName string) error {
	if stackName == "" || projectName == "" {
		// No potential for conflicting project names.
		return nil
	}

	// Catch the case where the user has specified a fully qualified stack reference.
	ref, err := b.ParseStackReference(stackName)
	if err != nil {
		// If we can't parse the stack reference, we can't compare the project names.
		// We assume the project names don't conflict and that this parsing issue will be handled downstream.
		return nil
	}
	stackProjectName, hasProject := ref.Project()
	if !hasProject {
		return nil
	}
	if projectName == stackProjectName.String() {
		return nil
	}
	return fmt.Errorf("project name (--name %s) and stack reference project name (--stack %s) must be the same",
		projectName, stackProjectName)
}
