// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/cloud"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/workspace"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/diag/colors"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/spf13/cobra"

	survey "gopkg.in/AlecAivazis/survey.v1"
	surveycore "gopkg.in/AlecAivazis/survey.v1/core"
)

const defaultURLEnvVar = "PULUMI_TEMPLATE_API"

func newNewCmd() *cobra.Command {
	var cloudURL string
	var name string
	var description string
	var force bool
	var yes bool
	var offline bool
	var generateOnly bool

	cmd := &cobra.Command{
		Use:   "new [template]",
		Short: "Create a new Pulumi project",
		Args:  cmdutil.MaximumNArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			var err error

			// Validate name (if specified) before further prompts/operations.
			if name != "" && !workspace.IsValidProjectName(name) {
				return errors.Errorf("'%s' is not a valid project name", name)
			}

			// Get the current working directory.
			var cwd string
			if cwd, err = os.Getwd(); err != nil {
				return errors.Wrap(err, "getting the working directory")
			}

			releases, err := cloud.New(cmdutil.Diag(), getCloudURL(cloudURL))
			if err != nil {
				return errors.Wrap(err, "creating API client")
			}

			// If we're going to be creating a stack, get the current backend, which
			// will kick off the login flow (if not already logged-in).
			var b backend.Backend
			if !generateOnly {
				b, err = currentBackend()
				if err != nil {
					return err
				}
			}

			// Get the selected template.
			var templateName string
			if len(args) > 0 {
				templateName = strings.ToLower(args[0])
			} else {
				if templateName, err = chooseTemplate(releases, offline); err != nil {
					return err
				}
			}

			// Download and install the template to the local template cache.
			if !offline {
				var tarball io.ReadCloser
				source := releases.CloudURL()
				if tarball, err = releases.DownloadTemplate(templateName, false); err != nil {
					message := ""
					// If the local template is available locally, provide a nicer error message.
					if localTemplates, localErr := workspace.ListLocalTemplates(); localErr == nil && len(localTemplates) > 0 {
						_, m := templateArrayToStringArrayAndMap(localTemplates)
						if _, ok := m[templateName]; ok {
							message = fmt.Sprintf(
								"; rerun the command and pass --offline to use locally cached template '%s'",
								templateName)
						}
					}

					return errors.Wrapf(err, "downloading template '%s' from %s%s", templateName, source, message)
				}
				if err = workspace.InstallTemplate(templateName, tarball); err != nil {
					return errors.Wrapf(err, "installing template '%s' from %s", templateName, source)
				}
			}

			// Load the local template.
			var template workspace.Template
			if template, err = workspace.LoadLocalTemplate(templateName); err != nil {
				return errors.Wrapf(err, "template '%s' not found", templateName)
			}

			// Do a dry run, if we're not forcing files to be overwritten.
			if !force {
				if err = template.CopyTemplateFilesDryRun(cwd); err != nil {
					if os.IsNotExist(err) {
						return errors.Wrapf(err, "template '%s' not found", templateName)
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
				name = promptForValue(yes, "project name", defaultValue, workspace.IsValidProjectName)
			}

			// Prompt for the project description, if it wasn't already specified.
			if description == "" {
				defaultValue := workspace.ValueOrDefaultProjectDescription(description, template.Description)
				description = promptForValue(yes, "project description", defaultValue, nil)
			}

			// Actually copy the files.
			if err = template.CopyTemplateFiles(cwd, force, name, description); err != nil {
				if os.IsNotExist(err) {
					return errors.Wrapf(err, "template '%s' not found", templateName)
				}
				return err
			}

			fmt.Printf("Created project '%s'.\n", name)

			// Prompt for the stack name and create the stack.
			var stack backend.Stack
			if !generateOnly {
				defaultValue := getDevStackName(name)

				for {
					stackName := promptForValue(yes, "stack name", defaultValue, nil)
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
				var keys config.KeyArray
				for k := range template.Config {
					keys = append(keys, k)
				}
				if len(keys) > 0 {
					sort.Sort(keys)

					c := make(config.Map)
					for _, k := range keys {
						value := promptForValue(yes, k.String(), template.Config[k], nil)
						c[k] = config.NewValue(value)
					}

					if err = saveConfig(stack.Name().StackName(), c); err != nil {
						return errors.Wrap(err, "saving config")
					}

					fmt.Println("Saved config.")
				}
			}

			// Install dependencies.
			if !generateOnly && template.InstallDependencies {
				fmt.Println("Installing dependencies...")
				err = installDependencies()
				if err != nil {
					return errors.Wrap(err, "installing dependencies")
				}
				fmt.Println("Finished installing dependencies.")

				// Write a summary with next steps.
				fmt.Println("New project is configured and ready to deploy with 'pulumi update'.")
			}

			return nil
		}),
	}

	cmd.PersistentFlags().StringVarP(&cloudURL,
		"cloud-url", "c", "", "A cloud URL to download templates from")
	cmd.PersistentFlags().StringVarP(
		&name, "name", "n", "",
		"The project name; if not specified, a prompt will request it")
	cmd.PersistentFlags().StringVarP(
		&description, "description", "d", "",
		"The project description; if not specified, a prompt will request it")
	cmd.PersistentFlags().BoolVarP(
		&force, "force", "f", false,
		"Forces content to be generated even if it would change existing files")
	cmd.PersistentFlags().BoolVar(
		&yes, "yes", false,
		"Skip prompts and proceed with default values")
	cmd.PersistentFlags().BoolVarP(
		&offline, "offline", "o", false,
		"Use locally cached templates without making any network requests")
	cmd.PersistentFlags().BoolVar(
		&generateOnly, "generate-only", false,
		"Generate the project only; do not create a stack, save config, or install dependencies")

	return cmd
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
	return createStack(b, stackRef, nil)
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
func installDependencies() error {
	proj, _, err := readProject()
	if err != nil {
		return err
	}

	// TODO[pulumi/pulumi#1307]: move to the language plugins so we don't have to hard code here.
	var c *exec.Cmd
	if strings.EqualFold(proj.Runtime, "nodejs") {
		c = exec.Command("npm", "install") // nolint: gas, intentionally launching with partial path
	} else if strings.EqualFold(proj.Runtime, "python") {
		c = exec.Command("pip", "install", "-r", "requirements.txt") // nolint: gas, intentionally launching with partial path
	} else {
		return nil
	}

	// Run the command.
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// getCloudURL returns the URL used to download the template.
func getCloudURL(cloudURL string) string {
	// If we have a cloud URL, just return it.
	if cloudURL != "" {
		return cloudURL
	}

	// Otherwise, respect the PULUMI_TEMPLATE_API override.
	if fromEnv := os.Getenv(defaultURLEnvVar); fromEnv != "" {
		return fromEnv
	}

	// Otherwise, use the default.
	return cloud.DefaultURL()
}

// chooseTemplate will prompt the user to choose amongst the available templates.
func chooseTemplate(backend cloud.Backend, offline bool) (string, error) {
	const chooseTemplateErr = "no template selected; please use `pulumi new` to choose one"
	if !cmdutil.Interactive() {
		return "", errors.New(chooseTemplateErr)
	}

	var templates []workspace.Template
	var err error

	if !offline {
		if templates, err = backend.ListTemplates(); err != nil {
			message := "could not fetch list of remote templates"

			// If we couldn't fetch the list, see if there are any local templates
			if localTemplates, localErr := workspace.ListLocalTemplates(); localErr == nil && len(localTemplates) > 0 {
				options, _ := templateArrayToStringArrayAndMap(localTemplates)
				message = message + "\nrerun the command and pass --offline to use locally cached templates: " +
					strings.Join(options, ", ")
			}

			return "", errors.Wrap(err, message)
		}
	} else {
		if templates, err = workspace.ListLocalTemplates(); err != nil || len(templates) == 0 {
			return "", errors.Wrap(err, chooseTemplateErr)
		}
	}

	// Customize the prompt a little bit (and disable color since it doesn't match our scheme).
	surveycore.DisableColor = true
	surveycore.QuestionIcon = ""
	surveycore.SelectFocusIcon = colors.ColorizeText(colors.BrightGreen + ">" + colors.Reset)
	message := "\rPlease choose a template:"
	message = colors.ColorizeText(colors.BrightWhite + message + colors.Reset)

	options, _ := templateArrayToStringArrayAndMap(templates)

	var option string
	if err := survey.AskOne(&survey.Select{
		Message: message,
		Options: options,
	}, &option, nil); err != nil {
		return "", errors.New(chooseTemplateErr)
	}

	return option, nil
}

// promptForValue prompts the user for a value with a defaultValue preselected. Hitting enter accepts the
// default. If yes is true, defaultValue is returned without prompting. isValidFn is an optional parameter;
// when specified, it will be run to validate that value entered. An invalid value will result in an error
// message followed by another prompt for the value.
func promptForValue(yes bool, prompt string, defaultValue string, isValidFn func(value string) bool) string {
	if yes {
		return defaultValue
	}

	for {
		prompt = colors.ColorizeText(
			fmt.Sprintf("%s%s: (%s)%s ", colors.BrightCyan, prompt, defaultValue, colors.Reset))
		fmt.Print(prompt)

		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		value := strings.TrimSpace(line)

		if value != "" {
			if isValidFn == nil || isValidFn(value) {
				return value
			}

			// The value is invalid, let the user know and try again
			fmt.Printf("Sorry, '%s' is not a valid %s.\n", value, prompt)
			continue
		}
		return defaultValue
	}
}

// templateArrayToStringArrayAndMap returns an array of template names and map of names to templates
// from an array of templates.
func templateArrayToStringArrayAndMap(templates []workspace.Template) ([]string, map[string]workspace.Template) {
	var options []string
	nameToTemplateMap := make(map[string]workspace.Template)
	for _, template := range templates {
		options = append(options, template.Name)
		nameToTemplateMap[template.Name] = template
	}
	sort.Strings(options)

	return options, nameToTemplateMap
}
