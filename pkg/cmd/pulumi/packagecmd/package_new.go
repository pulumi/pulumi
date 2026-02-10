// Copyright 2016-2026, Pulumi Corporation.
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
	"errors"
	"fmt"
	"os"
	"strings"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/newcmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

type packageNewArgs struct {
	language      string
	name          string
	dir           string
	componentType string
}

func newPackageNewCmd() *cobra.Command {
	args := &packageNewArgs{}

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new Pulumi component package",
		Long: `Create a new Pulumi component package with interactive selection.

Choose between:
  - distributable component: Multi-language component with auto-generated SDKs for cross-language consumption
  - local component: Single-language component for rapid prototyping

Supported languages: python, typescript, go, csharp, java, yaml

Examples:
  pulumi package new
  pulumi package new --type distributable --language python
  pulumi package new --name my-component --type local --language typescript --dir ./my-component`,
		Args: cmdutil.NoArgs,
		RunE: func(cmd *cobra.Command, cliArgs []string) error {
			return runPackageNew(args)
		},
	}

	cmd.Flags().StringVarP(&args.name, "name", "n", "",
		"The component name; if not specified, you will be prompted (default: NewComponent)")
	cmd.Flags().StringVar(&args.dir, "dir", "",
		"The location to place the generated component; if not specified, the current directory is used")
	cmd.Flags().StringVarP(&args.componentType, "type", "t", "",
		"The component type: 'distributable' or 'local'")
	cmd.Flags().StringVarP(&args.language, "language", "l", "",
		"The programming language: python, typescript, go, csharp, java, yaml")

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	return cmd
}

func runPackageNew(args *packageNewArgs) error {
	targetDir := args.dir
	if targetDir == "" {
		var err error
		targetDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}
	} else {
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", targetDir, err)
		}
	}

	componentType := args.componentType
	if componentType == "" {
		var err error
		componentType, err = promptForPackageComponentType()
		if err != nil {
			return err
		}
	}

	componentType = strings.ToLower(componentType)
	var scaffoldType string
	switch componentType {
	case "distributable", "multi-language":
		scaffoldType = "multi-language"
	case "local", "single-language":
		scaffoldType = "single-language"
	default:
		return fmt.Errorf("invalid component type: %s (must be 'distributable' or 'local')", componentType)
	}

	language := args.language
	if language == "" {
		var err error
		language, err = promptForLanguage()
		if err != nil {
			return err
		}
	}
	language = strings.ToLower(language)

	componentName := args.name
	if componentName == "" {
		var err error
		componentName, err = promptForComponentName()
		if err != nil {
			return err
		}
	}

	if componentName == "" || componentName == "." || componentName == ".." {
		return errors.New("invalid component name")
	}

	switch language {
	case "python", "py":
		if scaffoldType == "single-language" {
			return newcmd.ScaffoldPythonComponentSingleLanguage(targetDir, componentName)
		}
		return newcmd.ScaffoldPythonComponent(targetDir, componentName)
	case "typescript", "ts":
		if scaffoldType == "single-language" {
			return newcmd.ScaffoldTypeScriptComponentSingleLanguage(targetDir, componentName)
		}
		return newcmd.ScaffoldTypeScriptComponent(targetDir, componentName)
	case "go":
		if scaffoldType == "single-language" {
			return newcmd.ScaffoldGoComponentSingleLanguage(targetDir, componentName)
		}
		return newcmd.ScaffoldGoComponent(targetDir, componentName)
	case "csharp", "cs", "c#":
		if scaffoldType == "single-language" {
			return newcmd.ScaffoldCSharpComponentSingleLanguage(targetDir, componentName)
		}
		return newcmd.ScaffoldCSharpComponent(targetDir, componentName)
	case "java":
		if scaffoldType == "single-language" {
			return newcmd.ScaffoldJavaComponentSingleLanguage(targetDir, componentName)
		}
		return newcmd.ScaffoldJavaComponent(targetDir, componentName)
	case "yaml", "yml":
		if scaffoldType == "single-language" {
			return newcmd.ScaffoldYamlComponentSingleLanguage(targetDir, componentName)
		}
		return newcmd.ScaffoldYamlComponent(targetDir, componentName)
	default:
		return fmt.Errorf("unsupported language: %s (supported: python, typescript, go, csharp, java, yaml)", language)
	}
}

func promptForPackageComponentType() (string, error) {
	options := []string{"distributable", "local"}
	optionsDescriptionMap := map[string]string{
		"distributable": "Multi-language component with auto-generated SDKs for cross-language consumption",
		"local":         "Single-language component for rapid prototyping",
	}

	var componentType string
	if err := survey.AskOne(&survey.Select{
		Message: "What type of component package would you like to create?",
		Options: options,
		Description: func(opt string, _ int) string {
			return optionsDescriptionMap[opt]
		},
	}, &componentType, ui.SurveyIcons(cmdutil.GetGlobalColorization())); err != nil {
		return "", err
	}

	return componentType, nil
}

func promptForLanguage() (string, error) {
	options := []string{"python", "typescript", "go", "csharp", "java", "yaml"}
	optionsDescriptionMap := map[string]string{
		"python":     "Python",
		"typescript": "TypeScript",
		"go":         "Go",
		"csharp":     "C#",
		"java":       "Java",
		"yaml":       "YAML",
	}

	var language string
	if err := survey.AskOne(&survey.Select{
		Message: "Which language would you like to use?",
		Options: options,
		Description: func(opt string, _ int) string {
			return optionsDescriptionMap[opt]
		},
	}, &language, ui.SurveyIcons(cmdutil.GetGlobalColorization())); err != nil {
		return "", err
	}

	return language, nil
}

func promptForComponentName() (string, error) {
	var componentName string
	if err := survey.AskOne(&survey.Input{
		Message: "Component name:",
		Default: "NewComponent",
	}, &componentName, ui.SurveyIcons(cmdutil.GetGlobalColorization())); err != nil {
		return "", err
	}

	return componentName, nil
}
