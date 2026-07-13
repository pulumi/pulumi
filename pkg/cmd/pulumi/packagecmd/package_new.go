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

package packagecmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	survey "github.com/AlecAivazis/survey/v2"
	surveycore "github.com/AlecAivazis/survey/v2/core"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/policy"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/project/newcmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

var validPackageNameRegexp = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

func validatePackageName(s string) error {
	switch {
	case s == "":
		return errors.New("package names may not be empty")
	case len(s) > 100:
		return errors.New("package names are limited to 100 characters")
	case !validPackageNameRegexp.MatchString(s):
		return errors.New("package names must start with a lowercase letter " +
			"and may only contain lowercase letters, digits, hyphens, and underscores")
	case s == "pulumi" || strings.HasPrefix(s, "pulumi-"):
		return errors.New("package name must not be `pulumi` and must not start with the prefix `pulumi-` " +
			"to avoid collision with standard libraries")
	}
	return nil
}

func defaultPackageNameFromCwd(cwd string) string {
	name := strings.ToLower(filepath.Base(cwd))
	name = regexp.MustCompile(`[^a-z0-9_-]`).ReplaceAllString(name, "")
	name = strings.TrimLeft(name, "0123456789-_")
	if validatePackageName(name) == nil {
		return name
	}
	return "package"
}

type newPackageArgs struct {
	description       string
	dir               string
	force             bool
	generateOnly      bool
	name              string
	offline           bool
	templateNameOrURL string
	yes               bool
}

func newPackageNewCmd() *cobra.Command {
	args := newPackageArgs{}

	cmd := &cobra.Command{
		Use:        "new [template|url]",
		Aliases:    []string{"create", "setup"},
		SuggestFor: []string{"init"},
		Short:      "[EXPERIMENTAL] Create a new Pulumi package",
		Long: "[EXPERIMENTAL] Create a new Pulumi package from a template.\n" +
			"\n" +
			"To create a package from a specific template, pass the template name (such as\n" +
			"`component-nodejs` or `component-python`). If no template name is provided, a list of\n" +
			"suggested templates will be presented which can be selected interactively.\n" +
			"\n" +
			"A path to a local template directory or a git URL may be passed instead.\n" +
			"\n" +
			"Once the package is generated, edit the example sources to implement your package.",
		RunE: func(cmd *cobra.Command, cliArgs []string) error {
			ctx := cmd.Context()
			if len(cliArgs) > 0 {
				args.templateNameOrURL = cliArgs[0]
			}
			return runNewPackage(ctx, cmd.OutOrStdout(), args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "template", Usage: "[template|url]"},
		},
		Required: 0,
	})

	cmd.PersistentFlags().StringVarP(
		&args.description, "description", "d", "",
		"The package description; if not specified, a prompt will request it")
	cmd.PersistentFlags().StringVar(
		&args.dir, "dir", "",
		"The location to place the generated package; if not specified, the current directory is used")
	cmd.PersistentFlags().BoolVarP(
		&args.force, "force", "f", false,
		"Forces content to be generated in a non-empty directory")
	cmd.PersistentFlags().BoolVarP(
		&args.generateOnly, "generate-only", "g", false,
		"Generate the package only; do not install dependencies")
	cmd.PersistentFlags().StringVarP(
		&args.name, "name", "n", "",
		"The package name; if not specified, a prompt will request it")
	cmd.PersistentFlags().BoolVarP(
		&args.offline, "offline", "o", false,
		"Use locally cached templates without making any network requests")
	cmd.PersistentFlags().BoolVarP(
		&args.yes, "yes", "y", false,
		"Skip prompts and proceed with default values")

	return cmd
}

func runNewPackage(ctx context.Context, out io.Writer, args newPackageArgs) error {
	opts := display.Options{
		Color:         cmdutil.GetGlobalColorization(),
		IsInteractive: cmdutil.Interactive(),
	}

	if !opts.IsInteractive && !args.yes {
		return backenderr.ErrNonInteractiveRequiresYes
	}

	if args.name != "" {
		if err := validatePackageName(args.name); err != nil {
			return fmt.Errorf("'%s' is not a valid package name: %w", args.name, err)
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting the working directory: %w", err)
	}
	originalCwd := cwd

	if args.dir != "" {
		cwd, err = newcmd.UseSpecifiedDir(args.dir)
		if err != nil {
			return err
		}
	}

	if !args.force {
		if err := newcmd.ErrorIfNotEmptyDirectory(cwd); err != nil {
			return err
		}
	}

	repo, err := workspace.RetrieveTemplates(
		ctx, args.templateNameOrURL, args.offline, workspace.TemplateKindPackage)
	if err != nil {
		return err
	}
	defer func() {
		contract.IgnoreError(repo.Delete())
	}()

	templates, err := repo.PackageTemplates()
	if err != nil {
		return err
	}

	var template workspace.PackageTemplate
	switch {
	case len(templates) == 0:
		return errors.New("no templates")
	case len(templates) == 1:
		template = templates[0]
	case !opts.IsInteractive:
		return backenderr.NonInteractiveInputRequiredError{
			Detail: "a template must be provided when running in non-interactive mode",
		}
	default:
		if template, err = choosePackageTemplate(templates, opts); err != nil {
			return err
		}
	}
	if template.Errored() {
		return fmt.Errorf("template '%s' is currently broken: %w", template.Name, template.Error)
	}

	if !args.force {
		if err := workspace.CopyTemplateFilesDryRun(template.Dir, cwd, args.name); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("template '%s' not found: %w", args.templateNameOrURL, err)
			}
			return err
		}
	}

	hasAtLeastOnePrompt := args.name == "" || args.description == ""
	if !args.yes && hasAtLeastOnePrompt {
		fmt.Fprintln(out, "This command will walk you through creating a new Pulumi package.")
		fmt.Fprintln(out)
		fmt.Fprintln(out,
			opts.Color.Colorize(
				colors.Highlight("Enter a value or leave blank to accept the (default), and press <ENTER>.",
					"<ENTER>", colors.BrightCyan+colors.Bold)))
		fmt.Fprintln(out,
			opts.Color.Colorize(
				colors.Highlight("Press ^C at any time to quit.", "^C", colors.BrightCyan+colors.Bold)))
		fmt.Fprintln(out)
	}

	if args.name == "" {
		args.name, err = ui.PromptForValue(
			args.yes, "Package name", defaultPackageNameFromCwd(cwd), false, validatePackageName, opts)
		if err != nil {
			return err
		}
	}

	if args.description == "" {
		defaultValue := pkgWorkspace.ValueOrDefaultProjectDescription("", "${DESCRIPTION}", template.Description)
		args.description, err = ui.PromptForValue(
			args.yes, "Package description", defaultValue, false, pkgWorkspace.ValidateProjectDescription, opts)
		if err != nil {
			return err
		}
	}

	if err := workspace.CopyTemplateFiles(template.Dir, cwd, args.force, args.name, args.description); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("template '%s' not found: %w", args.templateNameOrURL, err)
		}
		return err
	}

	fmt.Fprintf(out, "Created package '%s'\n", args.name)
	fmt.Fprintln(out)

	pluginPath, err := workspace.DetectPluginPathAt(cwd)
	if err != nil {
		return fmt.Errorf("locating PulumiPlugin.yaml after copy: %w", err)
	}
	plugin, err := workspace.LoadPluginProject(pluginPath)
	if err != nil {
		return fmt.Errorf("loading PulumiPlugin.yaml: %w", err)
	}
	plugin.Template = nil
	if err := plugin.Save(pluginPath); err != nil {
		return fmt.Errorf("saving PulumiPlugin.yaml: %w", err)
	}

	if !args.generateOnly {
		if err := policy.InstallPluginDependencies(ctx, out, out, cwd, plugin.Runtime); err != nil {
			return err
		}
	}

	fmt.Fprintln(out,
		opts.Color.Colorize(
			colors.BrightGreen+colors.Bold+"Your new package is ready to go!"+colors.Reset)+
			" "+cmdutil.EmojiOr("✨", ""))
	fmt.Fprintln(out)

	var commands []string
	if originalCwd != cwd {
		cd, err := filepath.Rel(originalCwd, cwd)
		if err != nil {
			cd = cwd
		}
		if ui.ContainsWhiteSpace(cd) {
			cd = fmt.Sprintf("%q", cd)
		}
		commands = append(commands, "cd "+cd)
	}
	if args.generateOnly {
		commands = append(commands, "pulumi install")
	}
	if len(commands) > 0 {
		fmt.Fprintln(out, "To get started, run:")
		fmt.Fprintln(out)
		for i, c := range commands {
			cmdColors := colors.BrightBlue + colors.Bold + c + colors.Reset
			fmt.Fprintf(out, "   %d. %s\n", i+1, opts.Color.Colorize(cmdColors))
		}
		fmt.Fprintln(out)
	}

	if template.Quickstart != "" {
		fmt.Fprintln(out, template.Quickstart)
	}

	return nil
}

func choosePackageTemplate(
	templates []workspace.PackageTemplate, opts display.Options,
) (workspace.PackageTemplate, error) {
	const chooseTemplateErr = "no template selected; please use `pulumi package new` to choose one"
	if !opts.IsInteractive {
		return workspace.PackageTemplate{}, errors.New(chooseTemplateErr)
	}

	surveycore.DisableColor = true
	message := "\rPlease choose a template:"
	message = opts.Color.Colorize(colors.SpecPrompt + message + colors.Reset)

	options, optionToTemplateMap := packageTemplatesToOptions(templates)

	var option string
	if err := survey.AskOne(&survey.Select{
		Message:  message,
		Options:  options,
		PageSize: cmd.OptimalPageSize(cmd.OptimalPageSizeOpts{Nopts: len(options)}),
	}, &option, ui.SurveyIcons(opts.Color)); err != nil {
		return workspace.PackageTemplate{}, errors.New(chooseTemplateErr)
	}
	return optionToTemplateMap[option], nil
}

func packageTemplatesToOptions(
	templates []workspace.PackageTemplate,
) ([]string, map[string]workspace.PackageTemplate) {
	maxNameLength := 0
	for _, t := range templates {
		if len(t.Name) > maxNameLength {
			maxNameLength = len(t.Name)
		}
	}

	var options, brokenOptions []string
	nameToTemplateMap := make(map[string]workspace.PackageTemplate)
	for _, t := range templates {
		if t.Errored() {
			t.Description = newcmd.BrokenTemplateDescription
		}
		option := fmt.Sprintf(fmt.Sprintf("%%%ds    %%s", -maxNameLength), t.Name, t.Description)
		nameToTemplateMap[option] = t
		if t.Errored() {
			brokenOptions = append(brokenOptions, option)
		} else {
			options = append(options, option)
		}
	}
	sort.Strings(options)
	options = append(options, brokenOptions...)
	return options, nameToTemplateMap
}
