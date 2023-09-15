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
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode"

	survey "github.com/AlecAivazis/survey/v2"
	surveycore "github.com/AlecAivazis/survey/v2/core"
	"github.com/opentracing/opentracing-go"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	brokenTemplateDescription = "(This template is currently broken)"
)

type promptForValueFunc func(yes bool, valueType string, defaultValue string, secret bool,
	isValidFn func(value string) error, opts display.Options) (string, error)

type newArgs struct {
	configArray       []string
	configPath        bool
	description       string
	dir               string
	force             bool
	generateOnly      bool
	interactive       bool
	name              string
	offline           bool
	prompt            promptForValueFunc
	secretsProvider   string
	stack             string
	templateNameOrURL string
	yes               bool
	listTemplates     bool
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

	// Validate name (if specified) before further prompts/operations.
	if args.name != "" && workspace.ValidateProjectName(args.name) != nil {
		return fmt.Errorf("'%s' is not a valid project name: %w", args.name, workspace.ValidateProjectName(args.name))
	}

	// Validate secrets provider type
	if err := validateSecretsProvider(args.secretsProvider); err != nil {
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
		cwd, err = useSpecifiedDir(args.dir)
		if err != nil {
			return err
		}
	}

	// Return an error if the directory isn't empty.
	if !args.force {
		if err = errorIfNotEmptyDirectory(cwd); err != nil {
			return err
		}
	}

	// If we're going to be creating a stack, get the current backend, which
	// will kick off the login flow (if not already logged-in).
	var b backend.Backend
	if !args.generateOnly {
		// There is no current project at this point to pass into currentBackend
		b, err = currentBackend(ctx, nil, opts)
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

	// Retrieve the template repo.
	repo, err := workspace.RetrieveTemplates(args.templateNameOrURL, args.offline, workspace.TemplateKindPulumiProject)
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

	var template workspace.Template
	if len(templates) == 0 {
		return errors.New("no templates")
	} else if len(templates) == 1 {
		template = templates[0]
	} else {
		if template, err = chooseTemplate(templates, opts); err != nil {
			return err
		}
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

		stackName, err := buildStackName(args.stack)
		if err != nil {
			return err
		}

		existingStack, _, existingDesc, err := getStack(ctx, b, stackName, opts)
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
		defaultValue := workspace.ValueOrSanitizedDefaultProjectName(args.name, template.ProjectName, filepath.Base(cwd))
		if err := validateProjectName(ctx, b, orgName, defaultValue, args.generateOnly, opts); err != nil {
			// If --yes is given error out now that the default value is invalid. If we allow prompt to catch
			// this case it can lead to a confusing error message because we set the defaultValue to "" below.
			// See https://github.com/pulumi/pulumi/issues/8747.
			if args.yes {
				return fmt.Errorf("'%s' is not a valid project name. %w", defaultValue, err)
			}

			// Do not suggest an invalid or existing name as the default project name.
			defaultValue = ""
		}
		validate := func(s string) error { return validateProjectName(ctx, b, orgName, s, args.generateOnly, opts) }
		args.name, err = args.prompt(args.yes, "project name", defaultValue, false, validate, opts)
		if err != nil {
			return err
		}
	}

	// Prompt for the project description, if it wasn't already specified.
	if args.description == "" {
		defaultValue := workspace.ValueOrDefaultProjectDescription(
			args.description, template.ProjectDescription, template.Description)
		args.description, err = args.prompt(
			args.yes, "project description", defaultValue, false, workspace.ValidateProjectDescription, opts)
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
	proj, root, err := readProject()
	if err != nil {
		return err
	}
	proj.Name = tokens.PackageName(args.name)
	proj.Description = &args.description
	proj.Template = nil
	// Workaround for python, most of our templates don't specify a venv but we want to use one
	if proj.Runtime.Name() == "python" {
		// If the template does give virtualenv use it, else default to "venv"
		if _, has := proj.Runtime.Options()["virtualenv"]; !has {
			proj.Runtime.SetOption("virtualenv", "venv")
		}
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
		if s, err = promptAndCreateStack(ctx, b, args.prompt,
			args.stack, root, true /*setCurrent*/, args.yes, opts, args.secretsProvider); err != nil {
			return err
		}
		// The backend will print "Created stack '<stack>'" on success.
		fmt.Println()
	}

	// Prompt for config values (if needed) and save.
	if !args.generateOnly {
		err = handleConfig(
			ctx, args.prompt, proj, s,
			args.templateNameOrURL, template, args.configArray,
			args.yes, args.configPath, opts)
		if err != nil {
			return err
		}
	}

	// Ensure the stack is selected.
	if !args.generateOnly && s != nil {
		contract.IgnoreError(state.SetCurrentStack(s.Ref().String()))
	}

	// Install dependencies.
	if !args.generateOnly {
		span := opentracing.SpanFromContext(ctx)
		projinfo := &engine.Projinfo{Proj: proj, Root: root}
		pwd, _, pluginCtx, err := engine.ProjectInfoContext(
			projinfo,
			nil,
			cmdutil.Diag(),
			cmdutil.Diag(),
			false,
			span,
			nil,
		)
		if err != nil {
			return err
		}

		defer pluginCtx.Close()

		if err := installDependencies(pluginCtx, &proj.Runtime, pwd); err != nil {
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

// Ensure the directory exists and uses it as the current working
// directory.
func useSpecifiedDir(dir string) (string, error) {
	// Ensure the directory exists.
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", fmt.Errorf("creating the directory: %w", err)
	}

	// Change the working directory to the specified directory.
	if err := os.Chdir(dir); err != nil {
		return "", fmt.Errorf("changing the working directory: %w", err)
	}

	// Get the new working directory.
	var cwd string
	var err error
	if cwd, err = os.Getwd(); err != nil {
		return "", fmt.Errorf("getting the working directory: %w", err)
	}
	return cwd, nil
}

// newNewCmd creates a New command with default dependencies.
// Intentionally disabling here for cleaner err declaration/assignment.
//
//nolint:vetshadow
func newNewCmd() *cobra.Command {
	args := newArgs{
		interactive: cmdutil.Interactive(),
		prompt:      promptForValue,
	}

	getTemplates := func() ([]workspace.Template, error) {
		// Attempt to retrieve available templates.
		repo, err := workspace.RetrieveTemplates("", false /*offline*/, workspace.TemplateKindPulumiProject)
		if err != nil {
			logging.Warningf("could not retrieve templates: %v", err)
			return []workspace.Template{}, err
		}

		// Get the list of templates.
		return repo.Templates()
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
			"* `PULUMI_GITSSH_PASSPHRASE=<passphrase> pulumi new ssh://<user>@<hostname>/<project>/<repo>`\n",
		Args: cmdutil.MaximumNArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, cliArgs []string) error {
			ctx := commandContext()
			if len(cliArgs) > 0 {
				args.templateNameOrURL = cliArgs[0]
			}
			if args.listTemplates {
				templates, err := getTemplates()
				if err != nil {
					logging.Warningf("could not list templates: %v", err)
					return err
				}
				available, _ := templatesToOptionArrayAndMap(templates, true)
				fmt.Println("")
				fmt.Println("Available Templates:")
				for _, t := range available {
					fmt.Printf("  %s\n", t)
				}
				return nil
			}

			args.yes = args.yes || skipConfirmations()
			return runNew(ctx, args)
		}),
	}

	// Add additional help that includes a list of available templates.
	defaultHelp := cmd.HelpFunc()
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		// Show default help.
		defaultHelp(cmd, args)

		templates, err := getTemplates()
		if err != nil {
			logging.Warningf("could not list templates: %v", err)
			return
		}

		// If we have any templates, show them.
		if len(templates) > 0 {
			fmt.Println()
			fmt.Printf("There are %d locally installed templates.\n", len(templates))
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

	return cmd
}

// File or directory names that are considered invisible
// when considering whether a directory is empty.
var invisibleDirEntries = map[string]struct{}{
	".git": {},
	".hg":  {},
	".bzr": {},
}

// errorIfNotEmptyDirectory returns an error if path is not empty.
func errorIfNotEmptyDirectory(path string) error {
	infos, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	var nonEmpty bool
	for _, info := range infos {
		if _, ignore := invisibleDirEntries[info.Name()]; ignore {
			continue
		}
		nonEmpty = true
		break
	}

	if nonEmpty {
		return fmt.Errorf("%s is not empty; "+
			"rerun in an empty directory, pass the path to an empty directory to --dir, or use --force", path)
	}

	return nil
}

func validateProjectName(ctx context.Context, b backend.Backend,
	orgName string, projectName string, generateOnly bool, opts display.Options,
) error {
	err := workspace.ValidateProjectName(projectName)
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
			return fmt.Errorf("a project with this name already exists: %s", projectName)
		}
	}

	return nil
}

// getStack gets a stack and the project name & description, or returns nil if the stack doesn't exist.
func getStack(ctx context.Context, b backend.Backend,
	stack string, opts display.Options,
) (backend.Stack, string, string, error) {
	contract.Requiref(b != nil, "b", "must not be nil")

	stackRef, err := b.ParseStackReference(stack)
	if err != nil {
		return nil, "", "", err
	}

	s, err := b.GetStack(ctx, stackRef)
	if err != nil {
		return nil, "", "", err
	}

	name := ""
	description := ""
	if s != nil {
		tags := s.Tags()
		// Tags might be nil/empty, but if it has name and description use them
		name = tags[apitype.ProjectNameTag]
		description = tags[apitype.ProjectDescriptionTag]
	}

	return s, name, description, nil
}

// promptAndCreateStack creates and returns a new stack (prompting for the name as needed).
func promptAndCreateStack(ctx context.Context, b backend.Backend, prompt promptForValueFunc,
	stack string, root string, setCurrent bool, yes bool, opts display.Options,
	secretsProvider string,
) (backend.Stack, error) {
	contract.Requiref(b != nil, "b", "must not be nil")
	contract.Requiref(root != "", "root", "must not be empty")

	if stack != "" {
		stackName, err := buildStackName(stack)
		if err != nil {
			return nil, err
		}
		s, err := stackInit(ctx, b, stackName, root, setCurrent, secretsProvider)
		if err != nil {
			return nil, err
		}
		return s, nil
	}

	if b.SupportsOrganizations() {
		fmt.Print("Please enter your desired stack name.\n" +
			"To create a stack in an organization, " +
			"use the format <org-name>/<stack-name> (e.g. `acmecorp/dev`).\n")
	}

	for {
		stackName, err := prompt(yes, "stack name", "dev", false, b.ValidateStackName, opts)
		if err != nil {
			return nil, err
		}
		formattedStackName, err := buildStackName(stackName)
		if err != nil {
			return nil, err
		}
		s, err := stackInit(ctx, b, formattedStackName, root, setCurrent, secretsProvider)
		if err != nil {
			if !yes {
				// Let the user know about the error and loop around to try again.
				fmt.Printf("Sorry, could not create stack '%s': %v\n", stackName, err)
				continue
			}
			return nil, err
		}
		return s, nil
	}
}

// stackInit creates the stack.
func stackInit(
	ctx context.Context, b backend.Backend, stackName string,
	root string, setCurrent bool, secretsProvider string,
) (backend.Stack, error) {
	stackRef, err := b.ParseStackReference(stackName)
	if err != nil {
		return nil, err
	}
	return createStack(ctx, b, stackRef, root, nil, setCurrent, secretsProvider)
}

// saveConfig saves the config for the stack.
func saveConfig(stack backend.Stack, c config.Map) error {
	project, _, err := readProject()
	if err != nil {
		return err
	}

	ps, err := loadProjectStack(project, stack)
	if err != nil {
		return err
	}

	for k, v := range c {
		ps.Config[k] = v
	}

	return saveProjectStack(stack, ps)
}

// installDependencies will install dependencies for the project, e.g. by running `npm install` for nodejs projects.
func installDependencies(ctx *plugin.Context, runtime *workspace.ProjectRuntimeInfo, directory string) error {
	// First make sure the language plugin is present.  We need this to load the required resource plugins.
	// TODO: we need to think about how best to version this.  For now, it always picks the latest.
	lang, err := ctx.Host.LanguageRuntime(ctx.Root, ctx.Pwd, runtime.Name(), runtime.Options())
	if err != nil {
		return fmt.Errorf("failed to load language plugin %s: %w", runtime.Name(), err)
	}

	if err = lang.InstallDependencies(directory); err != nil {
		return fmt.Errorf("installing dependencies failed; rerun manually to try again, "+
			"then run `pulumi up` to perform an initial deployment: %w", err)
	}

	return nil
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

		cd = fmt.Sprintf("cd %s", cd)
		commands = append(commands, cd)
	}

	if generateOnly {
		// We didn't install dependencies, so instruct the user to do so.
		if strings.EqualFold(proj.Runtime.Name(), "nodejs") {
			commands = append(commands, "npm install")
		} else if strings.EqualFold(proj.Runtime.Name(), "python") {
			commands = append(commands, pythonCommands()...)
		}
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

// pythonCommands returns the set of Python commands to create a virtual environment, activate it, and
// install dependencies.
func pythonCommands() []string {
	var commands []string

	// Create the virtual environment.
	switch runtime.GOOS {
	case "windows":
		commands = append(commands, "python -m venv venv")
	default:
		commands = append(commands, "python3 -m venv venv")
	}

	// Activate the virtual environment. Only active in the user's current shell, so we can't
	// just run it for the user here.
	switch runtime.GOOS {
	case "windows":
		commands = append(commands, "venv\\Scripts\\activate")
	default:
		commands = append(commands, "source venv/bin/activate")
	}

	// Update pip, setuptools, and wheel within the virtualenv.
	commands = append(commands, "python -m pip install --upgrade pip setuptools wheel")

	// Install dependencies within the virtualenv.
	commands = append(commands, "python -m pip install -r requirements.txt")

	return commands
}

// chooseTemplate will prompt the user to choose amongst the available templates.
func chooseTemplate(templates []workspace.Template, opts display.Options) (workspace.Template, error) {
	const chooseTemplateErr = "no template selected; please use `pulumi new` to choose one"
	if !opts.IsInteractive {
		return workspace.Template{}, errors.New(chooseTemplateErr)
	}

	// Customize the prompt a little bit (and disable color since it doesn't match our scheme).
	surveycore.DisableColor = true

	options, optionToTemplateMap := templatesToOptionArrayAndMap(templates, true)
	nopts := len(options)
	pageSize := optimalPageSize(optimalPageSizeOpts{nopts: nopts})
	message := fmt.Sprintf("\rPlease choose a template (%d/%d shown):\n", pageSize, nopts)
	message = opts.Color.Colorize(colors.SpecPrompt + message + colors.Reset)

	var option string
	if err := survey.AskOne(&survey.Select{
		Message:  message,
		Options:  options,
		PageSize: pageSize,
	}, &option, surveyIcons(opts.Color)); err != nil {
		return workspace.Template{}, errors.New(chooseTemplateErr)
	}

	return optionToTemplateMap[option], nil
}

// parseConfig parses the config values passed via command line flags.
// These are passed as `-c aws:region=us-east-1 -c foo:bar=blah` and end up
// in configArray as ["aws:region=us-east-1", "foo:bar=blah"].
// This function converts the array into a config.Map.
func parseConfig(configArray []string, path bool) (config.Map, error) {
	configMap := make(config.Map)
	for _, c := range configArray {
		kvp := strings.SplitN(c, "=", 2)

		key, err := parseConfigKey(kvp[0])
		if err != nil {
			return nil, err
		}

		value := config.NewValue("")
		if len(kvp) == 2 {
			value = config.NewValue(kvp[1])
		}

		if err = configMap.Set(key, value, path); err != nil {
			return nil, err
		}
	}
	return configMap, nil
}

// promptForConfig will go through each config key needed by the template and prompt for a value.
// If a config value exists in commandLineConfig, it will be used without prompting.
// If stackConfig is non-nil and a config value exists in stackConfig, it will be used as the default
// value when prompting instead of the default value specified in templateConfig.
func promptForConfig(
	ctx context.Context,
	prompt promptForValueFunc,
	project *workspace.Project,
	stack backend.Stack,
	templateConfig map[string]workspace.ProjectTemplateConfigValue,
	commandLineConfig config.Map,
	stackConfig config.Map,
	yes bool,
	opts display.Options,
) (config.Map, error) {
	// Convert `string` keys to `config.Key`. If a string key is missing a delimiter,
	// the project name will be prepended.
	parsedTemplateConfig := make(map[config.Key]workspace.ProjectTemplateConfigValue)
	for k, v := range templateConfig {
		parsedKey, parseErr := parseConfigKey(k)
		if parseErr != nil {
			return nil, parseErr
		}
		parsedTemplateConfig[parsedKey] = v
	}

	// Sort keys. Note that we use the fully qualified module member here instead of a `prettyKey` so that
	// all config values for the current program are prompted one after another.
	var keys config.KeyArray
	for k := range parsedTemplateConfig {
		keys = append(keys, k)
	}
	sort.Sort(keys)

	// We need to load the stack config here for the secret manager
	ps, err := loadProjectStack(project, stack)
	if err != nil {
		return nil, fmt.Errorf("loading stack config: %w", err)
	}

	sm, needsSave, err := getStackSecretsManager(stack, ps)
	if err != nil {
		return nil, err
	}
	if needsSave {
		if err = saveProjectStack(stack, ps); err != nil {
			return nil, fmt.Errorf("saving stack config: %w", err)
		}
	}
	encrypter, err := sm.Encrypter()
	if err != nil {
		return nil, err
	}
	decrypter, err := sm.Decrypter()
	if err != nil {
		return nil, err
	}

	c := make(config.Map)

	for _, k := range keys {
		// If it was passed as a command line flag, use it without prompting.
		if val, ok := commandLineConfig[k]; ok {
			c[k] = val
			continue
		}

		templateConfigValue := parsedTemplateConfig[k]

		// Prepare a default value.
		var defaultValue string
		var secret bool
		if stackConfig != nil {
			// Use the stack's existing value as the default.
			if val, ok := stackConfig[k]; ok {
				// It's OK to pass a nil or non-nil crypter for non-secret values.
				value, err := val.Value(decrypter)
				if err != nil {
					return nil, err
				}
				defaultValue = value
			}
		}
		if defaultValue == "" {
			defaultValue = templateConfigValue.Default
		}
		if !secret {
			secret = templateConfigValue.Secret
		}

		// Prepare the prompt.
		promptText := prettyKey(k)
		if templateConfigValue.Description != "" {
			promptText = promptText + ": " + templateConfigValue.Description
		}

		// Prompt.
		value, err := prompt(yes, promptText, defaultValue, secret, nil, opts)
		if err != nil {
			return nil, err
		}

		if value == "" {
			// Don't add empty values to the config.
			continue
		}

		// Encrypt the value if needed.
		var v config.Value
		if secret {
			enc, err := encrypter.EncryptValue(ctx, value)
			if err != nil {
				return nil, err
			}
			v = config.NewSecureValue(enc)
		} else {
			v = config.NewValue(value)
		}

		// Save it.
		c[k] = v
	}

	// Add any other config values from the command line.
	for k, v := range commandLineConfig {
		if _, ok := c[k]; !ok {
			c[k] = v
		}
	}

	return c, nil
}

// promptForValue prompts the user for a value with a defaultValue preselected. Hitting enter accepts the
// default. If yes is true, defaultValue is returned without prompting. isValidFn is an optional parameter;
// when specified, it will be run to validate that value entered. When this function returns a non nil error
// validation is assumed to have failed and an error is printed. The error returned by isValidFn is also displayed
// to provide information about why the validation failed. A period is appended to this message. `promptForValue` then
// prompts again.
func promptForValue(
	yes bool, valueType string, defaultValue string, secret bool,
	isValidFn func(value string) error, opts display.Options,
) (string, error) {
	var value string
	for {
		// If we are auto-accepting the default (--yes), just set it and move on to validating.
		// Otherwise, prompt the user interactively for a value.
		if yes {
			value = defaultValue
		} else {
			var prompt string
			if defaultValue == "" {
				prompt = opts.Color.Colorize(
					fmt.Sprintf("%s%s%s", colors.SpecPrompt, valueType, colors.Reset))
			} else {
				defaultValuePrompt := defaultValue
				if secret {
					defaultValuePrompt = "[secret]"
				}

				prompt = opts.Color.Colorize(
					fmt.Sprintf("%s%s%s (%s)", colors.SpecPrompt, valueType, colors.Reset, defaultValuePrompt))
			}

			// Read the value.
			var err error
			if secret {
				value, err = cmdutil.ReadConsoleNoEcho(prompt)
				if err != nil {
					return "", err
				}
			} else {
				value, err = cmdutil.ReadConsole(prompt)
				if err != nil {
					return "", err
				}
			}
			value = strings.TrimSpace(value)

			// If the user simply hit ENTER, choose the default value.
			if value == "" {
				value = defaultValue
			}
		}

		// Ensure the resulting value is valid; note that we even validate the default, since sometimes
		// we will have invalid default values, like "" for the project name.
		if isValidFn != nil {
			if validationError := isValidFn(value); validationError != nil {
				// If validation failed, let the user know. If interactive, we will print the error and
				// prompt the user again; otherwise, in the case of --yes, we fail and report an error.
				err := fmt.Errorf("Sorry, '%s' is not a valid %s. %w", value, valueType, validationError)
				if yes {
					return "", err
				}
				fmt.Printf("%s\n", err)
				continue
			}
		}

		break
	}

	return value, nil
}

// templatesToOptionArrayAndMap returns an array of option strings and a map of option strings to templates.
// Each option string is made up of the template name and description with some padding in between.
func templatesToOptionArrayAndMap(templates []workspace.Template,
	showAll bool,
) ([]string, map[string]workspace.Template) {
	// Find the longest name length. Used to add padding between the name and description.
	maxNameLength := 0
	for _, template := range templates {
		if len(template.Name) > maxNameLength {
			maxNameLength = len(template.Name)
		}
	}

	// Build the array and map.
	var options []string
	var brokenOptions []string
	nameToTemplateMap := make(map[string]workspace.Template)
	for _, template := range templates {
		// If showAll is false, then only include templates marked Important
		if !showAll && !template.Important {
			continue
		}
		// If template is broken, indicate it in the project description.
		if template.Errored() {
			template.ProjectDescription = brokenTemplateDescription
		}

		// Create the option string that combines the name, padding, and description.
		desc := workspace.ValueOrDefaultProjectDescription("", template.ProjectDescription, template.Description)
		option := fmt.Sprintf(fmt.Sprintf("%%%ds    %%s", -maxNameLength), template.Name, desc)

		nameToTemplateMap[option] = template
		if template.Errored() {
			brokenOptions = append(brokenOptions, option)
		} else {
			options = append(options, option)
		}
	}
	// After sorting the options, add the broken templates to the end
	sort.Strings(options)
	options = append(options, brokenOptions...)

	if !showAll {
		// If showAll is false, include an option to show all
		option := "Show additional templates"
		options = append(options, option)
	}

	return options, nameToTemplateMap
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
