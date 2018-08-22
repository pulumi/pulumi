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
	"sort"
	"strings"
	"unicode"

	"github.com/pulumi/pulumi/pkg/backend"
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

// nolint: vetshadow, intentionally disabling here for cleaner err declaration/assignment.
func newNewCmd() *cobra.Command {
	var configArray []string
	var name string
	var description string
	var force bool
	var yes bool
	var offline bool
	var generateOnly bool
	var dir string

	cmd := &cobra.Command{
		Use:        "new [template]",
		SuggestFor: []string{"init", "create"},
		Short:      "Create a new Pulumi project",
		Args:       cmdutil.MaximumNArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Validate name (if specified) before further prompts/operations.
			if name != "" && !workspace.IsValidProjectName(name) {
				return errors.Errorf("'%s' is not a valid project name", name)
			}

			displayOpts := backend.DisplayOptions{
				Color: cmdutil.GetGlobalColorization(),
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
			var b backend.Backend
			if !generateOnly {
				if b, err = currentBackend(displayOpts); err != nil {
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
				if template, err = chooseTemplate(templates, displayOpts); err != nil {
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

			// Show instructions, if we're going to show at least one prompt.
			hasAtLeastOnePrompt := (name == "") || (description == "") || !generateOnly
			if !yes && hasAtLeastOnePrompt {
				fmt.Println("This command will walk you through creating a new Pulumi project.")
				fmt.Println()
				fmt.Println("Enter a value or leave blank to accept the default, and press <ENTER>.")
				fmt.Println("Press ^C at any time to quit.")
			}

			// Prompt for the project name, if it wasn't already specified.
			if name == "" {
				defaultValue := workspace.ValueOrSanitizedDefaultProjectName(name, filepath.Base(cwd))
				name, err = promptForValue(yes, "project name", defaultValue, false, workspace.IsValidProjectName, displayOpts)
				if err != nil {
					return err
				}
			}

			// Prompt for the project description, if it wasn't already specified.
			if description == "" {
				defaultValue := workspace.ValueOrDefaultProjectDescription(description, template.Description)
				description, err = promptForValue(yes, "project description", defaultValue, false, nil, displayOpts)
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

			fmt.Printf("Created project '%s'.\n", name)

			// Prompt for the stack name and create the stack.
			var stack backend.Stack
			if !generateOnly {
				defaultValue := getDevStackName(name)

				for {
					stackName, err := promptForValue(yes, "stack name", defaultValue, false, nil, displayOpts)
					if err != nil {
						return err
					}
					stack, err = stackInit(b, stackName)
					if err != nil {
						if !yes {
							// Let the user know about the error and loop around to try again.
							fmt.Printf("Sorry, could not create stack '%s': %v.\n", stackName, err)
							continue
						}
						return err
					}
					break
				}

				// The backend will print "Created stack '<stack>'." on success.
			}

			// Prompt for config values and save.
			if !generateOnly {
				// Get config values passed on the command line.
				commandLineConfig, err := parseConfig(configArray)
				if err != nil {
					return err
				}

				// Prompt for config as needed.
				c, err := promptForConfig(stack, template.Config, commandLineConfig, nil, yes, displayOpts)
				if err != nil {
					return err
				}

				// Save the config.
				if c != nil {
					if err = saveConfig(stack.Name().StackName(), c); err != nil {
						return errors.Wrap(err, "saving config")
					}

					fmt.Println("Saved config.")
				}
			}

			// Install dependencies.
			if !generateOnly {
				err = installDependencies("Installing dependencies...")
				if err != nil {
					return err
				}

				// Write a summary with next steps.
				fmt.Println(
					displayOpts.Color.Colorize(
						colors.BrightGreen+colors.Bold+"Your new project is configured and ready to go!"+colors.Reset) +
						" " + cmdutil.EmojiOr("✨", ""))

				// If the current working directory changed, add instructions to cd into the directory.
				var deployMsg string
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

					deployMsg = "To deploy it, '" + cd + "' and then run 'pulumi up'."
					deployMsg = colors.Highlight(deployMsg, cd, colors.BrightBlue+colors.Underline+colors.Bold)
				} else {
					deployMsg = "To deploy it, run 'pulumi up'."
				}

				// Colorize and print the next step deploy action.
				deployMsg = colors.Highlight(deployMsg, "pulumi up", colors.BrightBlue+colors.Underline+colors.Bold)
				fmt.Println(displayOpts.Color.Colorize(deployMsg))
			}

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
		&name, "name", "n", "",
		"The project name; if not specified, a prompt will request it")
	cmd.PersistentFlags().StringVarP(
		&description, "description", "d", "",
		"The project description; if not specified, a prompt will request it")
	cmd.PersistentFlags().BoolVarP(
		&force, "force", "f", false,
		"Forces content to be generated even if it would change existing files")
	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Skip prompts and proceed with default values")
	cmd.PersistentFlags().BoolVarP(
		&offline, "offline", "o", false,
		"Use locally cached templates without making any network requests")
	cmd.PersistentFlags().BoolVarP(
		&generateOnly, "generate-only", "g", false,
		"Generate the project only; do not create a stack, save config, or install dependencies")
	cmd.PersistentFlags().StringVar(
		&dir, "dir", "",
		"The location to place the generated project; if not specified, the current directory is used")

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

// getDevStackName returns the stack name suffixed with -dev.
func getDevStackName(name string) string {
	const suffix = "-dev"
	// Strip the suffix so we don't include two -dev suffixes
	// if the name already has it.
	return strings.TrimSuffix(name, suffix) + suffix
}

// stackInit creates the stack.
func stackInit(b backend.Backend, stackName string) (backend.Stack, error) {
	stackRef, err := b.ParseStackReference(stackName)
	if err != nil {
		return nil, err
	}
	return createStack(b, stackRef, nil, true /*setCurrent*/)
}

// saveConfig saves the config for the stack.
func saveConfig(stackName tokens.QName, c config.Map) error {
	ps, err := workspace.DetectProjectStack(stackName)
	if err != nil {
		return err
	}

	for k, v := range c {
		ps.Config[k] = v
	}

	return workspace.SaveProjectStack(stackName, ps)
}

// installDependencies will install dependencies for the project, e.g. by running
// `npm install` for nodejs projects or `pip install` for python projects.
func installDependencies(message string) error {
	proj, _, err := readProject()
	if err != nil {
		return err
	}

	// TODO[pulumi/pulumi#1307]: move to the language plugins so we don't have to hard code here.
	var command string
	var c *exec.Cmd
	if strings.EqualFold(proj.RuntimeInfo.Name(), "nodejs") {
		command = "npm install"
		c = exec.Command("npm", "install") // nolint: gas, intentionally launching with partial path
	} else if strings.EqualFold(proj.RuntimeInfo.Name(), "python") {
		command = "pip install -r requirements.txt"
		c = exec.Command("pip", "install", "-r", "requirements.txt") // nolint: gas, intentionally launching with partial path
	} else {
		return nil
	}

	if message != "" {
		fmt.Println(message)
	}

	// Run the command.
	if out, err := c.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "%s", out)
		return errors.Wrapf(err, "installing dependencies; rerun '%s' manually to try again", command)
	}

	return nil
}

// chooseTemplate will prompt the user to choose amongst the available templates.
func chooseTemplate(templates []workspace.Template, opts backend.DisplayOptions) (workspace.Template, error) {
	const chooseTemplateErr = "no template selected; please use `pulumi new` to choose one"
	if !cmdutil.Interactive() {
		return workspace.Template{}, errors.New(chooseTemplateErr)
	}

	// Customize the prompt a little bit (and disable color since it doesn't match our scheme).
	surveycore.DisableColor = true
	surveycore.QuestionIcon = ""
	surveycore.SelectFocusIcon = opts.Color.Colorize(colors.BrightGreen + ">" + colors.Reset)
	message := "\rPlease choose a template:"
	message = opts.Color.Colorize(colors.BrightWhite + message + colors.Reset)

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

		key, err := config.ParseKey(kvp[0])
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
	templateConfig map[config.Key]workspace.ProjectTemplateConfigValue,
	commandLineConfig config.Map,
	stackConfig config.Map,
	yes bool,
	opts backend.DisplayOptions) (config.Map, error) {

	c := make(config.Map)

	var keys config.KeyArray
	for k := range templateConfig {
		keys = append(keys, k)
	}
	sort.Sort(keys)

	var err error
	var crypter config.Crypter

	for _, k := range keys {
		// If it was passed as a command line flag, use it without prompting.
		if val, ok := commandLineConfig[k]; ok {
			c[k] = val
			continue
		}

		templateConfigValue := templateConfig[k]

		// Prepare a default value.
		var defaultValue string
		var secret bool
		if stackConfig != nil {
			// Use the stack's existing value as the default.
			if val, ok := stackConfig[k]; ok {
				secret = val.Secure()

				// Lazily get the crypter, only if needed, to avoid prompting for a password with the local backend.
				if secret && crypter == nil {
					if crypter, err = backend.GetStackCrypter(stack); err != nil {
						return nil, err
					}
				}

				// It's OK to pass a nil or non-nil crypter for non-secret values.
				value, err := val.Value(crypter)
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
		prompt := k.String()
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
			// Lazily get the crypter, only if needed, to avoid prompting for a password with the local backend.
			if crypter == nil {
				if crypter, err = backend.GetStackCrypter(stack); err != nil {
					return nil, err
				}
			}

			enc, err := crypter.EncryptValue(value)
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
// when specified, it will be run to validate that value entered. An invalid value will result in an error
// message followed by another prompt for the value.
func promptForValue(
	yes bool, prompt string, defaultValue string, secret bool,
	isValidFn func(value string) bool, opts backend.DisplayOptions) (string, error) {

	if yes {
		return defaultValue, nil
	}

	for {
		if defaultValue == "" {
			prompt = opts.Color.Colorize(
				fmt.Sprintf("%s%s:%s ", colors.BrightCyan, prompt, colors.Reset))
		} else {
			defaultValuePrompt := defaultValue
			if secret {
				defaultValuePrompt = "[secret]"
			}

			prompt = opts.Color.Colorize(
				fmt.Sprintf("%s%s: (%s)%s ", colors.BrightCyan, prompt, defaultValuePrompt, colors.Reset))
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
			if isValidFn == nil || isValidFn(value) {
				return value, nil
			}

			// The value is invalid, let the user know and try again
			fmt.Printf("Sorry, '%s' is not a valid %s.\n", value, prompt)
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
		option := fmt.Sprintf(fmt.Sprintf("%%%ds    %%s", -maxNameLength), template.Name, template.Description)

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
