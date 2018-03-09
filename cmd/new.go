// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pulumi/pulumi/pkg/backend/cloud"
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
	var offline bool

	cmd := &cobra.Command{
		Use:   "new <template>",
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

			releases := cloud.New(cmdutil.Diag(), getCloudURL(cloudURL))

			// Get the selected template.
			var template workspace.Template
			if len(args) > 0 {
				template = workspace.Template{Name: strings.ToLower(args[0])}
			} else {
				if template, err = chooseTemplate(releases, offline); err != nil {
					return err
				}
			}

			// Download and install the template to the local template cache.
			if !offline {
				var tarball io.ReadCloser
				source := releases.CloudURL()
				if tarball, err = releases.DownloadTemplate(template.Name, false); err != nil {
					message := ""
					// If the local template is available locally, provide a nicer error message.
					if localTemplates, localErr := workspace.ListLocalTemplates(); localErr == nil && len(localTemplates) > 0 {
						_, m := templateArrayToStringArrayAndMap(localTemplates)
						if _, ok := m[template.Name]; ok {
							message = fmt.Sprintf(
								"; rerun the command and pass --offline to use locally cached template '%s'",
								template.Name)
						}
					}

					return errors.Wrapf(err, "downloading template '%s' from %s%s", template.Name, source, message)
				}
				if err = workspace.InstallTemplate(template.Name, tarball); err != nil {
					return errors.Wrapf(err, "installing template '%s' from %s", template.Name, source)
				}
			}

			// Get the values to fill in.
			name = workspace.ValueOrSanitizedDefaultProjectName(name, filepath.Base(cwd))
			description = workspace.ValueOrDefaultProjectDescription(description, template.Description)

			// Do a dry run if we're not forcing files to be overwritten.
			if !force {
				if err = workspace.CopyTemplateFilesDryRun(template.Name, cwd); err != nil {
					if os.IsNotExist(err) {
						return errors.Wrapf(err, "template not found")
					}
					return err
				}
			}

			// Actually copy the files.
			if err = workspace.CopyTemplateFiles(template.Name, cwd, force, name, description); err != nil {
				if os.IsNotExist(err) {
					return errors.Wrapf(err, "template not found")
				}
				return err
			}

			fmt.Println("Your project was created successfully.")
			return nil
		}),
	}

	cmd.PersistentFlags().StringVarP(&cloudURL,
		"cloud-url", "c", "", "A cloud URL to download releases from")
	cmd.PersistentFlags().StringVarP(
		&name, "name", "n", "",
		"The project name; if not specified, the name of the current working directory is used")
	cmd.PersistentFlags().StringVarP(
		&description, "description", "d", "",
		"The project description; f not specified, a default description is used")
	cmd.PersistentFlags().BoolVarP(
		&force, "force", "f", false,
		"Forces content to be generated even if it would change existing files")
	cmd.PersistentFlags().BoolVarP(
		&offline, "offline", "o", false,
		"Allows offline use of cached templates without making any network requests")

	return cmd
}

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
func chooseTemplate(backend cloud.Backend, offline bool) (workspace.Template, error) {
	const chooseTemplateErr = "no template selected; please use `pulumi new` to choose one"
	if !cmdutil.Interactive() {
		return workspace.Template{}, errors.New(chooseTemplateErr)
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

			return workspace.Template{}, errors.Wrap(err, message)
		}
	} else {
		if templates, err = workspace.ListLocalTemplates(); err != nil || len(templates) == 0 {
			return workspace.Template{}, errors.Wrap(err, chooseTemplateErr)
		}
	}

	// Customize the prompt a little bit (and disable color since it doesn't match our scheme).
	surveycore.DisableColor = true
	surveycore.QuestionIcon = ""
	surveycore.SelectFocusIcon = colors.ColorizeText(colors.BrightGreen + ">" + colors.Reset)
	message := "\rPlease choose a template:"
	message = colors.ColorizeText(colors.BrightWhite + message + colors.Reset)

	options, nameToTemplateMap := templateArrayToStringArrayAndMap(templates)

	var option string
	if err := survey.AskOne(&survey.Select{
		Message: message,
		Options: options,
	}, &option, nil); err != nil {
		return workspace.Template{}, errors.New(chooseTemplateErr)
	}

	return nameToTemplateMap[option], nil
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
