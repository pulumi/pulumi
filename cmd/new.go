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
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode"

	"github.com/pulumi/pulumi/pkg/backend/state"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/workspace"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/diag/colors"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/spf13/cobra"

	survey "gopkg.in/AlecAivazis/survey.v1"
	surveycore "gopkg.in/AlecAivazis/survey.v1/core"
)

// Intentionally disabling here for cleaner err declaration/assignment.
// nolint: vetshadow
func newNewCmd() *cobra.Command {
	var configArray []string
	var description string
	var dir string
	var force bool
	var generateOnly bool
	var name string
	var offline bool
	var stack string
	var yes bool
	var secretsProvider string

	cmd := &cobra.Command{
		Use:        "new [template|url]",
		SuggestFor: []string{"init", "create"},
		Short:      "Create a new Pulumi project",
		Args:       cmdutil.MaximumNArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			interactive := cmdutil.Interactive()
			if !interactive {
				yes = true // auto-approve changes, since we cannot prompt.
			}

			// Prepare options.
			opts := display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: interactive,
			}

			// Validate name (if specified) before further prompts/operations.
			if name != "" && workspace.ValidateProjectName(name) != nil {
				return errors.Errorf("'%s' is not a valid project name. %s.", name, workspace.ValidateProjectName(name))
			}

			// Validate secrets provider type
			if err := validateSecretsProvider(secretsProvider); err != nil {
				return err
			}

			// Get the current working directory.
			cwd, err := os.Getwd()
			if err != nil {
				return errors.Wrap(err, "getting the working directory")
			}
			originalCwd := cwd

			// If dir was specified, ensure it exists and use it as the
			// current working directory.
			if dir != "" {
				// Ensure the directory exists.
				if err = os.MkdirAll(dir, os.ModePerm); err != nil {
					return errors.Wrap(err, "creating the directory")
				}

				// Change the working directory to the specified directory.
				if err = os.Chdir(dir); err != nil {
					return errors.Wrap(err, "changing the working directory")
				}

				// Get the new working directory.
				if cwd, err = os.Getwd(); err != nil {
					return errors.Wrap(err, "getting the working directory")
				}
			}

			// Return an error if the directory isn't empty.
			if !force {
				if err = errorIfNotEmptyDirectory(cwd); err != nil {
					return err
				}
			}

			// If we're going to be creating a stack, get the current backend, which
			// will kick off the login flow (if not already logged-in).
			if !generateOnly {
				if _, err = currentBackend(opts); err != nil {
					return err
				}
			}

			templateNameOrURL := ""
			if len(args) > 0 {
				templateNameOrURL = args[0]
			}

			// Retrieve the template repo.
			repo, err := workspace.RetrieveTemplates(templateNameOrURL, offline)
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

			// Do a dry run, if we're not forcing files to be overwritten.
			if !force {
				if err = template.CopyTemplateFilesDryRun(cwd); err != nil {
					if os.IsNotExist(err) {
						return errors.Wrapf(err, "template '%s' not found", templateNameOrURL)
					}
					return err
				}
			}

			// If a stack was specified via --stack, see if it already exists.
			var s backend.Stack
			if stack != "" {
				existingStack, existingName, existingDesc, err := getStack(stack, opts)
				if err != nil {
					return err
				}
				s = existingStack
				if name == "" {
					name = existingName
				}
				if description == "" {
					description = existingDesc
				}
			}

			// Show instructions, if we're going to show at least one prompt.
			hasAtLeastOnePrompt := (name == "") || (description == "") || (!generateOnly && stack == "")
			if !yes && hasAtLeastOnePrompt {
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
			if name == "" {
				defaultValue := workspace.ValueOrSanitizedDefaultProjectName(name, template.ProjectName, filepath.Base(cwd))
				name, err = promptForValue(
					yes, "project name", defaultValue, false, workspace.ValidateProjectName, opts)
				if err != nil {
					return err
				}
			}

			// Prompt for the project description, if it wasn't already specified.
			if description == "" {
				defaultValue := workspace.ValueOrDefaultProjectDescription(
					description, template.ProjectDescription, template.Description)
				description, err = promptForValue(
					yes, "project description", defaultValue, false, workspace.ValidateProjectDescription, opts)
				if err != nil {
					return err
				}
			}

			// Actually copy the files.
			if err = template.CopyTemplateFiles(cwd, force, name, description); err != nil {
				if os.IsNotExist(err) {
					return errors.Wrapf(err, "template '%s' not found", templateNameOrURL)
				}
				return err
			}

			fmt.Printf("Created project '%s'\n", name)
			fmt.Println()

			// Load the project, update the name & description, remove the template section, and save it.
			proj, _, err := readProject()
			if err != nil {
				return err
			}
			proj.Name = tokens.PackageName(name)
			proj.Description = &description
			proj.Template = nil
			if err = workspace.SaveProject(proj); err != nil {
				return errors.Wrap(err, "saving project")
			}

			// Create the stack, if needed.
			if !generateOnly && s == nil {
				if s, err = promptAndCreateStack(stack, name, true /*setCurrent*/, yes, opts, secretsProvider); err != nil {
					return err
				}
				// The backend will print "Created stack '<stack>'" on success.
				fmt.Println()
			}

			// Prompt for config values (if needed) and save.
			if !generateOnly {
				if err = handleConfig(s, templateNameOrURL, template, configArray, yes, opts); err != nil {
					return err
				}
			}

			// Ensure the stack is selected.
			if !generateOnly && s != nil {
				contract.IgnoreError(state.SetCurrentStack(s.Ref().String()))
			}

			// Install dependencies.
			if !generateOnly {
				if err := installDependencies(); err != nil {
					return err
				}
			}

			fmt.Println(
				opts.Color.Colorize(
					colors.BrightGreen+colors.Bold+"Your new project is ready to go!"+colors.Reset) +
					" " + cmdutil.EmojiOr("✨", ""))
			fmt.Println()

			// Print out next steps.
			printNextSteps(proj, originalCwd, cwd, generateOnly, opts)

			if template.Quickstart != "" {
				fmt.Println(template.Quickstart)
			}

			return nil
		}),
	}

	// Add additional help that includes a list of available templates.
	defaultHelp := cmd.HelpFunc()
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		// Show default help.
		defaultHelp(cmd, args)

		// Attempt to retrieve available templates.
		repo, err := workspace.RetrieveTemplates("", false /*offline*/)
		if err != nil {
			logging.Warningf("could not retrieve templates: %v", err)
			return
		}

		// Get the list of templates.
		templates, err := repo.Templates()
		if err != nil {
			logging.Warningf("could not list templates: %v", err)
			return
		}

		// If we have any templates, show them.
		if len(templates) > 0 {
			available, _ := templatesToOptionArrayAndMap(templates)
			fmt.Println("")
			fmt.Println("Available Templates:")
			for _, t := range available {
				fmt.Printf("  %s\n", t)
			}
		}
	})

	cmd.PersistentFlags().StringArrayVarP(
		&configArray, "config", "c", []string{},
		"Config to save")
	cmd.PersistentFlags().StringVarP(
		&description, "description", "d", "",
		"The project description; if not specified, a prompt will request it")
	cmd.PersistentFlags().StringVar(
		&dir, "dir", "",
		"The location to place the generated project; if not specified, the current directory is used")
	cmd.PersistentFlags().BoolVarP(
		&force, "force", "f", false,
		"Forces content to be generated even if it would change existing files")
	cmd.PersistentFlags().BoolVarP(
		&generateOnly, "generate-only", "g", false,
		"Generate the project only; do not create a stack, save config, or install dependencies")
	cmd.PersistentFlags().StringVarP(
		&name, "name", "n", "",
		"The project name; if not specified, a prompt will request it")
	cmd.PersistentFlags().BoolVarP(
		&offline, "offline", "o", false,
		"Use locally cached templates without making any network requests")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The stack name; either an existing stack or stack to create; if not specified, a prompt will request it")
	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Skip prompts and proceed with default values")
	cmd.PersistentFlags().StringVar(
		&secretsProvider, "secrets-provider", "default", "The type of the provider that should be used to encrypt and "+
			"decrypt secrets (possible choices: default, passphrase)")

	return cmd
}

// errorIfNotEmptyDirectory returns an error if path is not empty.
func errorIfNotEmptyDirectory(path string) error {
	infos, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}

	if len(infos) > 0 {
		return errors.Errorf("%s is not empty; "+
			"rerun in an empty directory, pass the path to an empty directory to --dir, or use --force", path)
	}

	return nil
}

// getStack gets a stack and the project name & description, or returns nil if the stack doesn't exist.
func getStack(stack string, opts display.Options) (backend.Stack, string, string, error) {
	b, err := currentBackend(opts)
	if err != nil {
		return nil, "", "", err
	}

	stackRef, err := b.ParseStackReference(stack)
	if err != nil {
		return nil, "", "", err
	}

	s, err := b.GetStack(commandContext(), stackRef)
	if err != nil {
		return nil, "", "", err
	}

	name := ""
	description := ""
	if s != nil {
		if cs, ok := s.(httpstate.Stack); ok {
			tags := cs.Tags()
			name = tags[apitype.ProjectNameTag]
			description = tags[apitype.ProjectDescriptionTag]
		}
	}

	return s, name, description, nil
}

// promptAndCreateStack creates and returns a new stack (prompting for the name as needed).
func promptAndCreateStack(
	stack string, projectName string, setCurrent bool, yes bool, opts display.Options,
	secretsProvider string) (backend.Stack, error) {

	b, err := currentBackend(opts)
	if err != nil {
		return nil, err
	}

	if stack != "" {
		s, err := stackInit(b, stack, setCurrent, secretsProvider)
		if err != nil {
			return nil, err
		}
		return s, nil
	}

	for {
		stackName, err := promptForValue(yes, "stack name", "dev", false, workspace.ValidateStackName, opts)
		if err != nil {
			return nil, err
		}
		s, err := stackInit(b, stackName, setCurrent, secretsProvider)
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
func stackInit(b backend.Backend, stackName string, setCurrent bool, secretsProvider string) (backend.Stack, error) {
	stackRef, err := b.ParseStackReference(stackName)
	if err != nil {
		return nil, err
	}
	return createStack(b, stackRef, nil, setCurrent, secretsProvider)
}

// saveConfig saves the config for the stack.
func saveConfig(stack backend.Stack, c config.Map) error {
	ps, err := loadProjectStack(stack)
	if err != nil {
		return err
	}

	for k, v := range c {
		ps.Config[k] = v
	}

	return saveProjectStack(stack, ps)
}

// installDependencies will install dependencies for the project, e.g. by running `npm install` for nodejs projects.
func installDependencies() error {
	proj, _, err := readProject()
	if err != nil {
		return err
	}

	// TODO[pulumi/pulumi#1307]: move to the language plugins so we don't have to hard code here.
	var command string
	var c *exec.Cmd
	if strings.EqualFold(proj.Runtime.Name(), "nodejs") {
		command = "npm install"
		// We pass `--loglevel=error` to prevent `npm` from printing warnings about missing
		// `description`, `repository`, and `license` fields in the package.json file.
		c = exec.Command("npm", "install", "--loglevel=error")
	} else {
		return nil
	}

	fmt.Println("Installing dependencies...")
	fmt.Println()

	// Run the command.
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return errors.Wrapf(err, "installing dependencies; rerun '%s' manually to try again, "+
			"then run 'pulumi up' to perform an initial deployment", command)
	}

	// Ensure the "node_modules" directory exists.
	if _, err := os.Stat("node_modules"); os.IsNotExist(err) {
		return errors.Errorf("installing dependencies; rerun '%s' manually to try again, "+
			"then run 'pulumi up' to perform an initial deployment", command)
	}

	fmt.Println("Finished installing dependencies")
	fmt.Println()

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

	if strings.EqualFold(proj.Runtime.Name(), "nodejs") && generateOnly {
		// If we're generating a NodeJS project, and we didn't install dependencies (generateOnly),
		// instruct the user to do so.
		commands = append(commands, "npm install")
	} else if strings.EqualFold(proj.Runtime.Name(), "python") {
		// If we're generating a Python project, instruct the user to set up and activate a virtual
		// environment.

		// Create the virtual environment.
		commands = append(commands, "virtualenv -p python3 venv")

		// Activate the virtual environment. Only active in the user's current shell, so we can't
		// just run it for the user here.
		switch runtime.GOOS {
		case "windows":
			commands = append(commands, "venv\\Scripts\\activate")
		default:
			commands = append(commands, "source venv/bin/activate")
		}

		// Install dependencies within the virtualenv
		commands = append(commands, "pip3 install -r requirements.txt")
	}

	// If we didn't create a stack, show that as a command to run before `pulumi up`.
	if generateOnly {
		commands = append(commands, "pulumi stack init")
	}

	if len(commands) == 0 { // No additional commands need to be run.
		deployMsg := "To perform an initial deployment, run 'pulumi up'"
		deployMsg = colors.Highlight(deployMsg, "pulumi up", colors.BrightBlue+colors.Bold)
		fmt.Println(opts.Color.Colorize(deployMsg))
		fmt.Println()
		return
	}

	if len(commands) == 1 { // Only one additional command need to be run.
		deployMsg := fmt.Sprintf("To perform an initial deployment, run '%s', then, run 'pulumi up'", commands[0])
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

	upMsg := colors.Highlight("Then, run 'pulumi up'", "pulumi up", colors.BrightBlue+colors.Bold)
	fmt.Println(opts.Color.Colorize(upMsg))
	fmt.Println()
}

// chooseTemplate will prompt the user to choose amongst the available templates.
func chooseTemplate(templates []workspace.Template, opts display.Options) (workspace.Template, error) {
	const chooseTemplateErr = "no template selected; please use `pulumi new` to choose one"
	if !opts.IsInteractive {
		return workspace.Template{}, errors.New(chooseTemplateErr)
	}

	// Customize the prompt a little bit (and disable color since it doesn't match our scheme).
	surveycore.DisableColor = true
	surveycore.QuestionIcon = ""
	surveycore.SelectFocusIcon = opts.Color.Colorize(colors.BrightGreen + ">" + colors.Reset)
	message := "\rPlease choose a template:"
	message = opts.Color.Colorize(colors.SpecPrompt + message + colors.Reset)

	options, optionToTemplateMap := templatesToOptionArrayAndMap(templates)

	var option string
	if err := survey.AskOne(&survey.Select{
		Message:  message,
		Options:  options,
		PageSize: len(options),
	}, &option, nil); err != nil {
		return workspace.Template{}, errors.New(chooseTemplateErr)
	}

	return optionToTemplateMap[option], nil
}

// parseConfig parses the config values passed via command line flags.
// These are passed as `-c aws:region=us-east-1 -c foo:bar=blah` and end up
// in configArray as ["aws:region=us-east-1", "foo:bar=blah"].
// This function converts the array into a config.Map.
func parseConfig(configArray []string) (config.Map, error) {
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

		configMap[key] = value
	}
	return configMap, nil
}

// promptForConfig will go through each config key needed by the template and prompt for a value.
// If a config value exists in commandLineConfig, it will be used without prompting.
// If stackConfig is non-nil and a config value exists in stackConfig, it will be used as the default
// value when prompting instead of the default value specified in templateConfig.
func promptForConfig(
	stack backend.Stack,
	templateConfig map[string]workspace.ProjectTemplateConfigValue,
	commandLineConfig config.Map,
	stackConfig config.Map,
	yes bool,
	opts display.Options) (config.Map, error) {

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

	sm, err := getStackSecretsManager(stack)
	if err != nil {
		return nil, err
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
		prompt := prettyKey(k)
		if templateConfigValue.Description != "" {
			prompt = prompt + ": " + templateConfigValue.Description
		}

		// Prompt.
		value, err := promptForValue(yes, prompt, defaultValue, secret, nil, opts)
		if err != nil {
			return nil, err
		}

		// Encrypt the value if needed.
		var v config.Value
		if secret {
			enc, err := encrypter.EncryptValue(value)
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
	isValidFn func(value string) error, opts display.Options) (string, error) {

	if yes {
		return defaultValue, nil
	}

	for {
		var prompt string

		if defaultValue == "" {
			prompt = opts.Color.Colorize(
				fmt.Sprintf("%s%s:%s ", colors.SpecPrompt, valueType, colors.Reset))
		} else {
			defaultValuePrompt := defaultValue
			if secret {
				defaultValuePrompt = "[secret]"
			}

			prompt = opts.Color.Colorize(
				fmt.Sprintf("%s%s:%s (%s) ", colors.SpecPrompt, valueType, colors.Reset, defaultValuePrompt))
		}
		fmt.Print(prompt)

		// Read the value.
		var err error
		var value string
		if secret {
			value, err = cmdutil.ReadConsoleNoEcho("")
			if err != nil {
				return "", err
			}
		} else {
			value, err = cmdutil.ReadConsole("")
			if err != nil {
				return "", err
			}
		}
		value = strings.TrimSpace(value)

		if value != "" {
			var validationError error
			if isValidFn != nil {
				validationError = isValidFn(value)
			}

			if validationError == nil {
				return value, nil
			}

			// The value is invalid, let the user know and try again
			fmt.Printf("Sorry, '%s' is not a valid %s. %s.\n", value, valueType, validationError)
			continue
		}
		return defaultValue, nil
	}
}

// templatesToOptionArrayAndMap returns an array of option strings and a map of option strings to templates.
// Each option string is made up of the template name and description with some padding in between.
func templatesToOptionArrayAndMap(templates []workspace.Template) ([]string, map[string]workspace.Template) {
	// Find the longest name length. Used to add padding between the name and description.
	maxNameLength := 0
	for _, template := range templates {
		if len(template.Name) > maxNameLength {
			maxNameLength = len(template.Name)
		}
	}

	// Build the array and map.
	var options []string
	nameToTemplateMap := make(map[string]workspace.Template)
	for _, template := range templates {
		// Create the option string that combines the name, padding, and description.
		desc := workspace.ValueOrDefaultProjectDescription("", template.ProjectDescription, template.Description)
		option := fmt.Sprintf(fmt.Sprintf("%%%ds    %%s", -maxNameLength), template.Name, desc)

		// Add it to the array and map.
		options = append(options, option)
		nameToTemplateMap[option] = template
	}
	sort.Strings(options)

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
