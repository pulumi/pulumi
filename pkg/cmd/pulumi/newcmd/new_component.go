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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

type newComponentArgs struct {
	language      string
	name          string
	dir           string
	componentType string
}

func NewComponentCmd() *cobra.Command {
	args := &newComponentArgs{}

	cmd := &cobra.Command{
		Use:   "component <language>",
		Short: "Create a new Pulumi component in the specified language",
		Long: `Create a new Pulumi component in the specified language.

Scaffolds out a basic Pulumi component project structure in the specified language.
Supported languages: python, typescript, go, csharp, java, yaml

Component types:
  - single-language: Simple component for single language use (rapid prototyping)
  - multi-language: Component with auto-generated SDKs for cross-language consumption

Example:
  pulumi new component python --type multi-language
  pulumi new component typescript --name my-component --type single-language
  pulumi new component go --dir ./my-component`,
		Args: cmdutil.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, cliArgs []string) error {
			if len(cliArgs) > 0 {
				args.language = cliArgs[0]
			}

			if args.language == "" {
				return errors.New("language argument is required")
			}

			return runNewComponent(args)
		},
	}

	cmd.Flags().StringVarP(&args.name, "name", "n", "",
		"The component name; if not specified, defaults to the directory name")
	cmd.Flags().StringVar(&args.dir, "dir", "",
		"The location to place the generated component; if not specified, the current directory is used")
	cmd.Flags().StringVarP(&args.componentType, "type", "t", "",
		"The component type: 'single-language' or 'multi-language'")

	return cmd
}

func runNewComponent(args *newComponentArgs) error {
	// Normalize language
	language := strings.ToLower(args.language)

	// Get target directory
	targetDir := args.dir
	if targetDir == "" {
		var err error
		targetDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}
	} else {
		// Create the directory if it doesn't exist
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", targetDir, err)
		}
	}

	// Determine component name
	componentName := args.name
	if componentName == "" {
		componentName = filepath.Base(targetDir)
	}

	// Validate component name
	if componentName == "" || componentName == "." || componentName == ".." {
		return errors.New("invalid component name; please specify a name using --name")
	}

	// Determine component type
	componentType := args.componentType
	if componentType == "" {
		var err error
		componentType, err = promptForComponentType()
		if err != nil {
			return err
		}
	}

	// Validate component type
	if componentType != "single-language" && componentType != "multi-language" {
		return fmt.Errorf("invalid component type: %s (must be 'single-language' or 'multi-language')", componentType)
	}

	// Scaffold based on language and type
	switch language {
	case "python", "py":
		if componentType == "single-language" {
			return ScaffoldPythonComponentSingleLanguage(targetDir, componentName)
		}
		return ScaffoldPythonComponent(targetDir, componentName)
	case "typescript", "ts":
		if componentType == "single-language" {
			return ScaffoldTypeScriptComponentSingleLanguage(targetDir, componentName)
		}
		return ScaffoldTypeScriptComponent(targetDir, componentName)
	case "go":
		if componentType == "single-language" {
			return ScaffoldGoComponentSingleLanguage(targetDir, componentName)
		}
		return ScaffoldGoComponent(targetDir, componentName)
	case "csharp", "cs", "c#":
		if componentType == "single-language" {
			return ScaffoldCSharpComponentSingleLanguage(targetDir, componentName)
		}
		return ScaffoldCSharpComponent(targetDir, componentName)
	case "java":
		if componentType == "single-language" {
			return ScaffoldJavaComponentSingleLanguage(targetDir, componentName)
		}
		return ScaffoldJavaComponent(targetDir, componentName)
	case "yaml", "yml":
		if componentType == "single-language" {
			return ScaffoldYamlComponentSingleLanguage(targetDir, componentName)
		}
		return ScaffoldYamlComponent(targetDir, componentName)
	default:
		return fmt.Errorf("unsupported language: %s (supported: python, typescript, go, csharp, java, yaml)", language)
	}
}

func promptForComponentType() (string, error) {
	options := []string{"single-language", "multi-language"}
	optionsDescriptionMap := map[string]string{
		"single-language": "Simple component for single language use (rapid prototyping)",
		"multi-language":  "Component with auto-generated SDKs for cross-language consumption",
	}

	var componentType string
	if err := survey.AskOne(&survey.Select{
		Message: "What type of component would you like to create?",
		Options: options,
		Description: func(opt string, _ int) string {
			return optionsDescriptionMap[opt]
		},
	}, &componentType, ui.SurveyIcons(cmdutil.GetGlobalColorization())); err != nil {
		return "", err
	}

	return componentType, nil
}

// ScaffoldPythonComponent scaffolds a multi-language Python component.
func ScaffoldPythonComponent(targetDir, componentName string) error {
	// Convert component name to Python package name (replace hyphens with underscores)
	packageName := strings.ReplaceAll(componentName, "-", "_")
	className := toTitleCase(packageName)

	files := map[string]string{
		fmt.Sprintf("%s.py", packageName): fmt.Sprintf(`from typing import Optional, TypedDict

import pulumi
from pulumi import ResourceOptions


class %[1]sArgs(TypedDict):
    message: pulumi.Input[str]
    """The message to output."""


class %[1]s(pulumi.ComponentResource):
    message: pulumi.Output[str]
    """The message output."""

    def __init__(self,
                 name: str,
                 args: %[1]sArgs,
                 opts: Optional[ResourceOptions] = None) -> None:

        super().__init__('components:index:%[1]s', name, {}, opts)

        # Store the message as an output
        self.message = pulumi.Output.from_input(args.get("message", "Hello, Pulumi!"))

        # Register outputs
        self.register_outputs({
            'message': self.message,
        })
`, className),
		"__main__.py": fmt.Sprintf(`from pulumi.provider.experimental import component_provider_host
from %s import %s

if __name__ == "__main__":
    component_provider_host(
        name="%s", components=[%s]
    )
`, packageName, className, componentName, className),
		"PulumiPlugin.yaml": fmt.Sprintf(`name: %s
version: 0.1.0
runtime: python
`, componentName),
		"requirements.txt": `pulumi>=3.0.0,<4.0.0
`,
		"README.md": fmt.Sprintf("# %s\n\nA Pulumi component resource.\n\n## Usage\n\nThis component can be used from any Pulumi language by referencing it:\n\n```bash\npulumi package add %s\n```\n\nThen in your Pulumi program:\n\n```python\nimport pulumi\nimport pulumi_%s as component\n\nmy_component = component.%s(\"%s-instance\",\n    message=\"Hello from component!\")\n\npulumi.export(\"message\", my_component.message)\n```\n\n## Development\n\nInstall dependencies:\n\n```bash\npip install -r requirements.txt\n```\n\nRun the component provider:\n\n```bash\npulumi package install\n```\n", componentName, componentName, packageName, className, componentName),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
	}

	fmt.Printf("Created Python component '%s' in %s\n", componentName, targetDir)
	return nil
}

// ScaffoldTypeScriptComponent scaffolds a multi-language TypeScript component.
func ScaffoldTypeScriptComponent(targetDir, componentName string) error {
	className := toTitleCase(componentName)

	files := map[string]string{
		"index.ts": fmt.Sprintf(`import * as pulumi from "@pulumi/pulumi";

export interface %[1]sArgs {
    message?: pulumi.Input<string>;
}

export class %[1]s extends pulumi.ComponentResource {
    public readonly message: pulumi.Output<string>;

    constructor(name: string, args: %[1]sArgs, opts?: pulumi.ComponentResourceOptions) {
        super("components:index:%[1]s", name, args, opts);

        this.message = pulumi.output(args?.message ?? "Hello, Pulumi!");
    }
}
`, className),
		"package.json": fmt.Sprintf(`{
  "name": "%s",
  "version": "0.1.0",
  "description": "A Pulumi component resource",
  "main": "index.ts",
  "scripts": {
    "build": "tsc"
  },
  "devDependencies": {
    "@types/node": "^20.0.0",
    "typescript": "^5.0.0"
  },
  "dependencies": {
    "@pulumi/pulumi": "^3.0.0"
  }
}
`, componentName),
		"tsconfig.json": `{
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "moduleResolution": "node",
    "declaration": true,
    "outDir": "./bin",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true
  },
  "include": [
    "**/*.ts"
  ],
  "exclude": [
    "node_modules"
  ]
}
`,
		"PulumiPlugin.yaml": fmt.Sprintf(`name: %s
runtime: nodejs
version: 0.1.0
`, componentName),
		"README.md": fmt.Sprintf("# %s\n\nA Pulumi component resource.\n\n## Usage\n\nThis component can be used from any Pulumi language by referencing it:\n\n```bash\npulumi package add %s\n```\n\nThen in your Pulumi program:\n\n```typescript\nimport * as component from \"@pulumi/%s\";\n\nconst myComponent = new component.%s(\"%s-instance\", {\n    message: \"Hello from component!\",\n});\n\nexport const message = myComponent.message;\n```\n\n## Development\n\nInstall dependencies:\n\n```bash\nnpm install\n```\n\nBuild:\n\n```bash\nnpm run build\n```\n", componentName, componentName, componentName, className, componentName),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
	}

	fmt.Printf("Created TypeScript component '%s' in %s\n", componentName, targetDir)
	return nil
}

// ScaffoldGoComponent scaffolds a multi-language Go component.
func ScaffoldGoComponent(targetDir, componentName string) error {
	className := toTitleCase(componentName)

	files := map[string]string{
		"component.go": fmt.Sprintf(`package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type %s struct {
	pulumi.ResourceState
	%sArgs
	Message pulumi.StringOutput ` + "`pulumi:\"message\"`" + `
}

type %sArgs struct {
	Message pulumi.StringInput ` + "`pulumi:\"message\"`" + `
}

func New%s(ctx *pulumi.Context, name string, args %sArgs, opts ...pulumi.ResourceOption) (*%s, error) {
	comp := &%s{}
	err := ctx.RegisterComponentResource("components:index:%s", name, comp, opts...)
	if err != nil {
		return nil, err
	}

	message := pulumi.String("Hello, Pulumi!")
	if args.Message != nil {
		message = args.Message
	}

	comp.Message = message.ToStringOutput()
	return comp, nil
}
`, className, className, className, className, className, className, className, className),
		"main.go": fmt.Sprintf(`package main

import (
	"github.com/pulumi/pulumi-go-provider/infer"
)

func main() {
	err := infer.NewProviderBuilder().
		WithName("%s").
		WithNamespace("components").
		AddComponent(infer.Component[%sArgs, %s, struct{}](New%s)).
		Build().
		Run()
	if err != nil {
		panic(err)
	}
}
`, componentName, className, className, className),
		"go.mod": fmt.Sprintf(`module %s

go 1.21

require (
	github.com/pulumi/pulumi/sdk/v3 v3.0.0
	github.com/pulumi/pulumi-go-provider v0.0.0
)
`, componentName),
		"PulumiPlugin.yaml": fmt.Sprintf(`name: %s
runtime: go
version: 0.1.0
`, componentName),
		"README.md": fmt.Sprintf("# %s\n\nA Pulumi component resource.\n\n## Usage\n\nThis component can be used from any Pulumi language by referencing it:\n\n```bash\npulumi package add %s\n```\n\nThen in your Pulumi program:\n\n```go\nimport (\n\t\"%s\"\n\t\"github.com/pulumi/pulumi/sdk/v3/go/pulumi\"\n)\n\nfunc main() {\n\tpulumi.Run(func(ctx *pulumi.Context) error {\n\t\tcomponent, err := %s.New%s(ctx, \"%s-instance\", %s.%sArgs{\n\t\t\tMessage: pulumi.String(\"Hello from component!\"),\n\t\t})\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\n\t\tctx.Export(\"message\", component.Message)\n\t\treturn nil\n\t})\n}\n```\n\n## Development\n\nInstall dependencies:\n\n```bash\ngo mod download\n```\n", componentName, componentName, componentName, componentName, className, componentName, componentName, className),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
	}

	fmt.Printf("Created Go component '%s' in %s\n", componentName, targetDir)
	return nil
}

// ScaffoldCSharpComponent scaffolds a multi-language C# component.
func ScaffoldCSharpComponent(targetDir, componentName string) error {
	// Convert component name to C# class name (PascalCase)
	className := toTitleCase(componentName)

	files := map[string]string{
		fmt.Sprintf("%s.cs", className): fmt.Sprintf(`using Pulumi;

namespace %s
{
    public class %sArgs : ResourceArgs
    {
        [Input("message")]
        public Input<string>? Message { get; set; }
    }

    public class %s : ComponentResource
    {
        [Output("message")]
        public Output<string> Message { get; private set; } = null!;

        public %s(string name, %sArgs? args = null, ComponentResourceOptions? opts = null)
            : base("pkg:%s:%s", name, args, opts)
        {
            this.Message = args?.Message ?? Output.Create("Hello, Pulumi!");

            this.RegisterOutputs(new Dictionary<string, object?>
            {
                ["message"] = this.Message,
            });
        }
    }
}
`, className, className, className, className, className, componentName, className),
		fmt.Sprintf("%s.csproj", className): fmt.Sprintf(`<Project Sdk="Microsoft.NET.Sdk">

  <PropertyGroup>
    <OutputType>Exe</OutputType>
    <TargetFramework>net8.0</TargetFramework>
    <Nullable>enable</Nullable>
  </PropertyGroup>

  <ItemGroup>
    <PackageReference Include="Pulumi" Version="3.*" />
  </ItemGroup>

</Project>
`),
		"README.md": fmt.Sprintf("# %s\n\nA Pulumi component resource.\n\n## Usage\n\n```csharp\nusing Pulumi;\nusing %s;\n\nreturn await Deployment.RunAsync(() =>\n{\n    var component = new %s(\"%s-instance\", new %sArgs\n    {\n        Message = \"Hello from component!\",\n    });\n\n    return new Dictionary<string, object?>\n    {\n        [\"message\"] = component.Message,\n    };\n});\n```\n\n## Development\n\nBuild:\n\n```bash\ndotnet build\n```\n", componentName, className, className, componentName, className),
		"PulumiPlugin.yaml": fmt.Sprintf(`name: %s
runtime: dotnet
version: 0.1.0
`, componentName),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
	}

	fmt.Printf("Created C# component '%s' in %s\n", componentName, targetDir)
	return nil
}

// ScaffoldJavaComponent scaffolds a multi-language Java component.
func ScaffoldJavaComponent(targetDir, componentName string) error {
	// Convert component name to Java class name (PascalCase)
	className := toTitleCase(componentName)
	packageName := strings.ToLower(strings.ReplaceAll(componentName, "-", ""))

	srcDir := filepath.Join("src", "main", "java", packageName)
	if err := os.MkdirAll(filepath.Join(targetDir, srcDir), 0755); err != nil {
		return fmt.Errorf("creating source directory: %w", err)
	}

	files := map[string]string{
		filepath.Join(srcDir, fmt.Sprintf("%s.java", className)): fmt.Sprintf(`package %s;

import com.pulumi.ComponentResource;
import com.pulumi.ComponentResourceOptions;
import com.pulumi.Output;
import com.pulumi.resources.ResourceArgs;

public class %s extends ComponentResource {
    public final Output<String> message;

    public %s(String name, %sArgs args, ComponentResourceOptions options) {
        super("pkg:%s:%s", name, args, options);

        this.message = args.message != null ? args.message : Output.of("Hello, Pulumi!");

        this.registerOutputs(java.util.Map.of(
            "message", this.message
        ));
    }

    public %s(String name, %sArgs args) {
        this(name, args, null);
    }

    public static class %sArgs extends ResourceArgs {
        public Output<String> message;

        public %sArgs() {}

        public %sArgs message(Output<String> message) {
            this.message = message;
            return this;
        }

        public %sArgs message(String message) {
            return message(Output.of(message));
        }
    }
}
`, packageName, className, className, className, packageName, className, className, className, className, className, className, className),
		"pom.xml": fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>

    <groupId>com.pulumi</groupId>
    <artifactId>%s</artifactId>
    <version>1.0-SNAPSHOT</version>

    <properties>
        <maven.compiler.source>11</maven.compiler.source>
        <maven.compiler.target>11</maven.compiler.target>
        <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
    </properties>

    <dependencies>
        <dependency>
            <groupId>com.pulumi</groupId>
            <artifactId>pulumi</artifactId>
            <version>0.9.0</version>
        </dependency>
    </dependencies>
</project>
`, componentName),
		"README.md": fmt.Sprintf("# %s\n\nA Pulumi component resource.\n\n## Usage\n\n```java\nimport com.pulumi.Pulumi;\nimport %s.%s;\n\npublic class App {\n    public static void main(String[] args) {\n        Pulumi.run(ctx -> {\n            var component = new %s(\"%s-instance\", \n                new %s.%sArgs().message(\"Hello from component!\"));\n\n            ctx.export(\"message\", component.message);\n        });\n    }\n}\n```\n\n## Development\n\nBuild:\n\n```bash\nmvn package\n```\n", componentName, packageName, className, className, componentName, className, className),
		"PulumiPlugin.yaml": fmt.Sprintf(`name: %s
runtime: java
version: 0.1.0
`, componentName),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
	}

	fmt.Printf("Created Java component '%s' in %s\n", componentName, targetDir)
	return nil
}

// ScaffoldYamlComponent scaffolds a multi-language YAML component.
func ScaffoldYamlComponent(targetDir, componentName string) error {
	files := map[string]string{
		"Pulumi.yaml": fmt.Sprintf(`name: %s
runtime: yaml
description: A Pulumi component resource

resources:
  component:
    type: pulumi:pulumi:ComponentResource
    properties:
      message: Hello, Pulumi!
    options:
      parent: $\{pulumi.parent}

outputs:
  message: $\{component.message}
`, componentName),
		"README.md": fmt.Sprintf("# %s\n\nA Pulumi component resource in YAML.\n\n## Usage\n\nDeploy with:\n\n```bash\npulumi up\n```\n\n## Customization\n\nEdit the `Pulumi.yaml` file to customize the component.\n", componentName),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
	}

	fmt.Printf("Created YAML component '%s' in %s\n", componentName, targetDir)
	return nil
}

// toTitleCase converts a string to TitleCase (first letter uppercase)
func toTitleCase(s string) string {
	if s == "" {
		return s
	}
	// Remove hyphens and underscores, capitalize first letter of each word
	words := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_'
	})
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, "")
}

// Single-language component scaffolding functions

// ScaffoldPythonComponentSingleLanguage scaffolds a single-language Python component.
func ScaffoldPythonComponentSingleLanguage(targetDir, componentName string) error {
	packageName := strings.ReplaceAll(componentName, "-", "_")
	className := toTitleCase(packageName)

	files := map[string]string{
		fmt.Sprintf("%s.py", packageName): fmt.Sprintf(`from typing import Optional
import pulumi


class %[1]sArgs:
    """Arguments for the %[1]s component."""

    def __init__(self, message: Optional[pulumi.Input[str]] = None):
        self.message = message


class %[1]s(pulumi.ComponentResource):
    """A simple Pulumi component resource."""

    message: pulumi.Output[str]
    """The message output."""

    def __init__(self,
                 name: str,
                 args: Optional[%[1]sArgs] = None,
                 opts: Optional[pulumi.ResourceOptions] = None) -> None:
        super().__init__('pkg:%[2]s:%[1]s', name, {}, opts)

        if args is None:
            args = %[1]sArgs()

        self.message = pulumi.Output.from_input(args.message if args.message else "Hello, Pulumi!")

        self.register_outputs({
            'message': self.message,
        })
`, className, componentName),
		"requirements.txt": `pulumi>=3.0.0,<4.0.0
`,
		"README.md": fmt.Sprintf("# %s\n\nA simple Pulumi component resource for single-language use.\n\n## Usage\n\n```python\nimport pulumi\nimport %s\n\ncomponent = %s.%s(\"%s-instance\",\n    args=%s.%sArgs(message=\"Hello from component!\"))\n\npulumi.export(\"message\", component.message)\n```\n\n## Development\n\nInstall dependencies:\n\n```bash\npip install -r requirements.txt\n```\n", componentName, packageName, packageName, className, componentName, packageName, className),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
	}

	fmt.Printf("Created Python single-language component '%s' in %s\n", componentName, targetDir)
	return nil
}

// ScaffoldTypeScriptComponentSingleLanguage scaffolds a single-language TypeScript component.
func ScaffoldTypeScriptComponentSingleLanguage(targetDir, componentName string) error {
	className := toTitleCase(componentName)

	files := map[string]string{
		"index.ts": fmt.Sprintf(`import * as pulumi from "@pulumi/pulumi";

export interface %[1]sArgs {
    message?: pulumi.Input<string>;
}

export class %[1]s extends pulumi.ComponentResource {
    public readonly message: pulumi.Output<string>;

    constructor(name: string, args?: %[1]sArgs, opts?: pulumi.ComponentResourceOptions) {
        super("pkg:%[2]s:%[1]s", name, args, opts);

        this.message = pulumi.output(args?.message ?? "Hello, Pulumi!");

        this.registerOutputs({
            message: this.message,
        });
    }
}
`, className, componentName),
		"package.json": fmt.Sprintf(`{
  "name": "%s",
  "version": "0.1.0",
  "description": "A simple Pulumi component resource",
  "main": "index.ts",
  "scripts": {
    "build": "tsc"
  },
  "devDependencies": {
    "@types/node": "^20.0.0",
    "typescript": "^5.0.0"
  },
  "dependencies": {
    "@pulumi/pulumi": "^3.0.0"
  }
}
`, componentName),
		"tsconfig.json": `{
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "moduleResolution": "node",
    "declaration": true,
    "outDir": "./bin",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true
  },
  "include": [
    "**/*.ts"
  ],
  "exclude": [
    "node_modules"
  ]
}
`,
		"README.md": fmt.Sprintf("# %s\n\nA simple Pulumi component resource for single-language use.\n\n## Usage\n\n```typescript\nimport * as %s from \"./%s\";\n\nconst component = new %s.%s(\"%s-instance\", {\n    message: \"Hello from component!\",\n});\n\nexport const message = component.message;\n```\n\n## Development\n\nInstall dependencies:\n\n```bash\nnpm install\n```\n\nBuild:\n\n```bash\nnpm run build\n```\n", componentName, packageName(componentName), componentName, packageName(componentName), className, componentName),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
	}

	fmt.Printf("Created TypeScript single-language component '%s' in %s\n", componentName, targetDir)
	return nil
}

// ScaffoldGoComponentSingleLanguage scaffolds a single-language Go component.
func ScaffoldGoComponentSingleLanguage(targetDir, componentName string) error {
	className := toTitleCase(componentName)

	files := map[string]string{
		"component.go": fmt.Sprintf(`package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type %s struct {
	pulumi.ResourceState

	Message pulumi.StringOutput ` + "`pulumi:\"message\"`" + `
}

type %sArgs struct {
	Message pulumi.StringPtrInput
}

func New%s(ctx *pulumi.Context, name string, args *%sArgs, opts ...pulumi.ResourceOption) (*%s, error) {
	if args == nil {
		args = &%sArgs{}
	}

	component := &%s{}
	err := ctx.RegisterComponentResource("pkg:%s:%s", name, component, opts...)
	if err != nil {
		return nil, err
	}

	message := args.Message
	if message == nil {
		message = pulumi.String("Hello, Pulumi!")
	}

	component.Message = message.ToStringOutput().ApplyT(func(v string) string {
		return v
	}).(pulumi.StringOutput)

	if err := ctx.RegisterResourceOutputs(component, pulumi.Map{
		"message": component.Message,
	}); err != nil {
		return nil, err
	}

	return component, nil
}
`, className, className, className, className, className, className, className, componentName, className),
		"go.mod": fmt.Sprintf(`module %s

go 1.21

require (
	github.com/pulumi/pulumi/sdk/v3 v3.0.0
)
`, componentName),
		"README.md": fmt.Sprintf("# %s\n\nA simple Pulumi component resource for single-language use.\n\n## Usage\n\n```go\nimport (\n\t\"%s\"\n\t\"github.com/pulumi/pulumi/sdk/v3/go/pulumi\"\n)\n\nfunc main() {\n\tpulumi.Run(func(ctx *pulumi.Context) error {\n\t\tcomponent, err := %s.New%s(ctx, \"%s-instance\", &%s.%sArgs{\n\t\t\tMessage: pulumi.String(\"Hello from component!\"),\n\t\t})\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\n\t\tctx.Export(\"message\", component.Message)\n\t\treturn nil\n\t})\n}\n```\n\n## Development\n\nInstall dependencies:\n\n```bash\ngo mod download\n```\n", componentName, componentName, componentName, className, componentName, componentName, className),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
	}

	fmt.Printf("Created Go single-language component '%s' in %s\n", componentName, targetDir)
	return nil
}

// ScaffoldCSharpComponentSingleLanguage scaffolds a single-language C# component.
func ScaffoldCSharpComponentSingleLanguage(targetDir, componentName string) error {
	className := toTitleCase(componentName)

	files := map[string]string{
		fmt.Sprintf("%s.cs", className): fmt.Sprintf(`using Pulumi;
using System.Collections.Generic;

namespace %s
{
    public class %sArgs : ResourceArgs
    {
        [Input("message")]
        public Input<string>? Message { get; set; }
    }

    public class %s : ComponentResource
    {
        [Output("message")]
        public Output<string> Message { get; private set; } = null!;

        public %s(string name, %sArgs? args = null, ComponentResourceOptions? opts = null)
            : base("pkg:%s:%s", name, args, opts)
        {
            this.Message = args?.Message ?? Output.Create("Hello, Pulumi!");

            this.RegisterOutputs(new Dictionary<string, object?>
            {
                ["message"] = this.Message,
            });
        }
    }
}
`, className, className, className, className, className, componentName, className),
		fmt.Sprintf("%s.csproj", className): fmt.Sprintf(`<Project Sdk="Microsoft.NET.Sdk">

  <PropertyGroup>
    <OutputType>Library</OutputType>
    <TargetFramework>net8.0</TargetFramework>
    <Nullable>enable</Nullable>
  </PropertyGroup>

  <ItemGroup>
    <PackageReference Include="Pulumi" Version="3.*" />
  </ItemGroup>

</Project>
`),
		"README.md": fmt.Sprintf("# %s\n\nA simple Pulumi component resource for single-language use.\n\n## Usage\n\n```csharp\nusing Pulumi;\nusing %s;\n\nreturn await Deployment.RunAsync(() =>\n{\n    var component = new %s(\"%s-instance\", new %sArgs\n    {\n        Message = \"Hello from component!\",\n    });\n\n    return new Dictionary<string, object?>\n    {\n        [\"message\"] = component.Message,\n    };\n});\n```\n\n## Development\n\nBuild:\n\n```bash\ndotnet build\n```\n", componentName, className, className, componentName, className),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
	}

	fmt.Printf("Created C# single-language component '%s' in %s\n", componentName, targetDir)
	return nil
}

// ScaffoldJavaComponentSingleLanguage scaffolds a single-language Java component.
func ScaffoldJavaComponentSingleLanguage(targetDir, componentName string) error {
	className := toTitleCase(componentName)
	packageName := strings.ToLower(strings.ReplaceAll(componentName, "-", ""))

	srcDir := filepath.Join("src", "main", "java", packageName)
	if err := os.MkdirAll(filepath.Join(targetDir, srcDir), 0755); err != nil {
		return fmt.Errorf("creating source directory: %w", err)
	}

	files := map[string]string{
		filepath.Join(srcDir, fmt.Sprintf("%s.java", className)): fmt.Sprintf(`package %s;

import com.pulumi.ComponentResource;
import com.pulumi.ComponentResourceOptions;
import com.pulumi.Output;
import com.pulumi.resources.ResourceArgs;

public class %s extends ComponentResource {
    public final Output<String> message;

    public %s(String name, %sArgs args, ComponentResourceOptions options) {
        super("pkg:%s:%s", name, args, options);

        this.message = args.message != null ? args.message : Output.of("Hello, Pulumi!");

        this.registerOutputs(java.util.Map.of(
            "message", this.message
        ));
    }

    public %s(String name, %sArgs args) {
        this(name, args, null);
    }

    public static class %sArgs extends ResourceArgs {
        public Output<String> message;

        public %sArgs() {}

        public %sArgs message(Output<String> message) {
            this.message = message;
            return this;
        }

        public %sArgs message(String message) {
            return message(Output.of(message));
        }
    }
}
`, packageName, className, className, className, packageName, className, className, className, className, className, className, className),
		"pom.xml": fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>

    <groupId>com.pulumi</groupId>
    <artifactId>%s</artifactId>
    <version>1.0-SNAPSHOT</version>

    <properties>
        <maven.compiler.source>11</maven.compiler.source>
        <maven.compiler.target>11</maven.compiler.target>
        <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
    </properties>

    <dependencies>
        <dependency>
            <groupId>com.pulumi</groupId>
            <artifactId>pulumi</artifactId>
            <version>0.9.0</version>
        </dependency>
    </dependencies>
</project>
`, componentName),
		"README.md": fmt.Sprintf("# %s\n\nA simple Pulumi component resource for single-language use.\n\n## Usage\n\n```java\nimport com.pulumi.Pulumi;\nimport %s.%s;\n\npublic class App {\n    public static void main(String[] args) {\n        Pulumi.run(ctx -> {\n            var component = new %s(\"%s-instance\", \n                new %s.%sArgs().message(\"Hello from component!\"));\n\n            ctx.export(\"message\", component.message);\n        });\n    }\n}\n```\n\n## Development\n\nBuild:\n\n```bash\nmvn package\n```\n", componentName, packageName, className, className, componentName, className, className),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
	}

	fmt.Printf("Created Java single-language component '%s' in %s\n", componentName, targetDir)
	return nil
}

// ScaffoldYamlComponentSingleLanguage scaffolds a single-language YAML component.
func ScaffoldYamlComponentSingleLanguage(targetDir, componentName string) error {
	files := map[string]string{
		"Pulumi.yaml": fmt.Sprintf(`name: %s
runtime: yaml
description: A simple Pulumi component resource

configuration:
  message:
    type: string
    default: Hello, Pulumi!

resources:
  component:
    type: pulumi:pulumi:ComponentResource
    properties:
      message: $\{message}
    options:
      parent: $\{pulumi.parent}

outputs:
  message: $\{component.message}
`, componentName),
		"README.md": fmt.Sprintf("# %s\n\nA simple Pulumi component resource in YAML for single-language use.\n\n## Usage\n\nDeploy with:\n\n```bash\npulumi up\n```\n\n## Customization\n\nEdit the `Pulumi.yaml` file to customize the component.\n", componentName),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
	}

	fmt.Printf("Created YAML single-language component '%s' in %s\n", componentName, targetDir)
	return nil
}

func packageName(componentName string) string {
	return strings.ReplaceAll(componentName, "-", "_")
}
