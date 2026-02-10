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

// Package newcmd provides scaffold functions for creating Pulumi component projects.
package newcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// writeScaffoldFile writes a file with permissions appropriate for user-editable source files.
// We use 0644 because these are source files that users need to read and edit.
//
//nolint:gosec // G306: Scaffold files need 0644 for user editing
func writeScaffoldFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func pythonDistributableReadme(componentName, packageName, className string) string {
	return fmt.Sprintf(`# %s

A Pulumi component resource.

## Usage

This component can be used from any Pulumi language by referencing it:

`+"```bash"+`
pulumi package add %s
`+"```"+`

Then in your Pulumi program:

`+"```python"+`
import pulumi
import pulumi_%s as component

my_component = component.%s("%s-instance",
    message="Hello from component!")

pulumi.export("message", my_component.message)
`+"```"+`

## Development

Install dependencies:

`+"```bash"+`
pip install -r requirements.txt
`+"```"+`

Run the component provider:

`+"```bash"+`
pulumi package install
`+"```"+`
`, componentName, componentName, packageName, className, componentName)
}

func typescriptDistributableReadme(componentName, className string) string {
	return fmt.Sprintf(`# %s

A Pulumi component resource.

## Usage

This component can be used from any Pulumi language by referencing it:

`+"```bash"+`
pulumi package add %s
`+"```"+`

Then in your Pulumi program:

`+"```typescript"+`
import * as component from "@pulumi/%s";

const myComponent = new component.%s("%s-instance", {
    message: "Hello from component!",
});

export const message = myComponent.message;
`+"```"+`

## Development

Install dependencies:

`+"```bash"+`
npm install
`+"```"+`

Build:

`+"```bash"+`
npm run build
`+"```"+`
`, componentName, componentName, componentName, className, componentName)
}

func goDistributableReadme(componentName, className string) string {
	return fmt.Sprintf(`# %s

A Pulumi component resource.

## Usage

This component can be used from any Pulumi language by referencing it:

`+"```bash"+`
pulumi package add %s
`+"```"+`

Then in your Pulumi program:

`+"```go"+`
import (
	"%s"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		component, err := %s.New%s(ctx, "%s-instance", %s.%sArgs{
			Message: pulumi.String("Hello from component!"),
		})
		if err != nil {
			return err
		}

		ctx.Export("message", component.Message)
		return nil
	})
}
`+"```"+`

## Development

Install dependencies:

`+"```bash"+`
go mod download
`+"```"+`
`, componentName, componentName, componentName,
		componentName, className, componentName, componentName, className)
}

func csharpDistributableReadme(componentName, className string) string {
	return fmt.Sprintf(`# %s

A Pulumi component resource.

## Usage

`+"```csharp"+`
using Pulumi;
using %s;

return await Deployment.RunAsync(() =>
{
    var component = new %s("%s-instance", new %sArgs
    {
        Message = "Hello from component!",
    });

    return new Dictionary<string, object?>
    {
        ["message"] = component.Message,
    };
});
`+"```"+`

## Development

Build:

`+"```bash"+`
dotnet build
`+"```"+`
`, componentName, className, className, componentName, className)
}

func javaDistributableReadme(componentName, packageName, className string) string {
	return fmt.Sprintf(`# %s

A Pulumi component resource.

## Usage

`+"```java"+`
import com.pulumi.Pulumi;
import %s.%s;

public class App {
    public static void main(String[] args) {
        Pulumi.run(ctx -> {
            var component = new %s("%s-instance",
                new %s.%sArgs().message("Hello from component!"));

            ctx.export("message", component.message);
        });
    }
}
`+"```"+`

## Development

Build:

`+"```bash"+`
mvn package
`+"```"+`
`, componentName, packageName, className, className, componentName, className, className)
}

func yamlDistributableReadme(componentName string) string {
	return fmt.Sprintf(`# %s

A Pulumi component resource in YAML.

## Usage

Deploy with:

`+"```bash"+`
pulumi up
`+"```"+`

## Customization

Edit the `+"`Pulumi.yaml`"+` file to customize the component.
`, componentName)
}

func pythonLocalReadme(componentName, packageName, className string) string {
	return fmt.Sprintf(`# %s

A simple Pulumi component resource for single-language use.

## Usage

`+"```python"+`
import pulumi
import %s

component = %s.%s("%s-instance",
    args=%s.%sArgs(message="Hello from component!"))

pulumi.export("message", component.message)
`+"```"+`

## Development

Install dependencies:

`+"```bash"+`
pip install -r requirements.txt
`+"```"+`
`, componentName, packageName, packageName, className, componentName, packageName, className)
}

func typescriptLocalReadme(componentName string) string {
	pkgName := packageName(componentName)
	return fmt.Sprintf(`# %s

A simple Pulumi component resource for single-language use.

## Usage

`+"```typescript"+`
import * as %s from "./%s";

const component = new %s.%s("%s-instance", {
    message: "Hello from component!",
});

export const message = component.message;
`+"```"+`

## Development

Install dependencies:

`+"```bash"+`
npm install
`+"```"+`

Build:

`+"```bash"+`
npm run build
`+"```"+`
`, componentName, pkgName, componentName, pkgName, toTitleCase(componentName), componentName)
}

func goLocalReadme(componentName, className string) string {
	return fmt.Sprintf(`# %s

A simple Pulumi component resource for single-language use.

## Usage

`+"```go"+`
import (
	"%s"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		component, err := %s.New%s(ctx, "%s-instance", &%s.%sArgs{
			Message: pulumi.String("Hello from component!"),
		})
		if err != nil {
			return err
		}

		ctx.Export("message", component.Message)
		return nil
	})
}
`+"```"+`

## Development

Install dependencies:

`+"```bash"+`
go mod download
`+"```"+`
`, componentName, componentName, componentName, className, componentName, componentName, className)
}

func csharpLocalReadme(componentName, className string) string {
	return fmt.Sprintf(`# %s

A simple Pulumi component resource for single-language use.

## Usage

`+"```csharp"+`
using Pulumi;
using %s;

return await Deployment.RunAsync(() =>
{
    var component = new %s("%s-instance", new %sArgs
    {
        Message = "Hello from component!",
    });

    return new Dictionary<string, object?>
    {
        ["message"] = component.Message,
    };
});
`+"```"+`

## Development

Build:

`+"```bash"+`
dotnet build
`+"```"+`
`, componentName, className, className, componentName, className)
}

func javaLocalReadme(componentName, packageName, className string) string {
	return fmt.Sprintf(`# %s

A simple Pulumi component resource for single-language use.

## Usage

`+"```java"+`
import com.pulumi.Pulumi;
import %s.%s;

public class App {
    public static void main(String[] args) {
        Pulumi.run(ctx -> {
            var component = new %s("%s-instance",
                new %s.%sArgs().message("Hello from component!"));

            ctx.export("message", component.message);
        });
    }
}
`+"```"+`

## Development

Build:

`+"```bash"+`
mvn package
`+"```"+`
`, componentName, packageName, className, className, componentName, className, className)
}

func yamlLocalReadme(componentName string) string {
	return fmt.Sprintf(`# %s

A simple Pulumi component resource in YAML for single-language use.

## Usage

Deploy with:

`+"```bash"+`
pulumi up
`+"```"+`

## Customization

Edit the `+"`Pulumi.yaml`"+` file to customize the component.
`, componentName)
}

// ScaffoldPythonComponent scaffolds a multi-language Python component.
func ScaffoldPythonComponent(targetDir, componentName string) error {
	packageName := strings.ReplaceAll(componentName, "-", "_")
	className := toTitleCase(packageName)

	files := map[string]string{
		packageName + ".py": fmt.Sprintf(`from typing import Optional, TypedDict

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
		"README.md": pythonDistributableReadme(componentName, packageName, className),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := writeScaffoldFile(filePath, content); err != nil {
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
		"README.md": typescriptDistributableReadme(componentName, className),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := writeScaffoldFile(filePath, content); err != nil {
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
	Message pulumi.StringOutput `+"`pulumi:\"message\"`"+`
}

type %sArgs struct {
	Message pulumi.StringInput `+"`pulumi:\"message\"`"+`
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
		"README.md": goDistributableReadme(componentName, className),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := writeScaffoldFile(filePath, content); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
	}

	fmt.Printf("Created Go component '%s' in %s\n", componentName, targetDir)
	return nil
}

// ScaffoldCSharpComponent scaffolds a multi-language C# component.
func ScaffoldCSharpComponent(targetDir, componentName string) error {
	className := toTitleCase(componentName)

	files := map[string]string{
		className + ".cs": fmt.Sprintf(`using Pulumi;

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
		className + ".csproj": `<Project Sdk="Microsoft.NET.Sdk">

  <PropertyGroup>
    <OutputType>Exe</OutputType>
    <TargetFramework>net8.0</TargetFramework>
    <Nullable>enable</Nullable>
  </PropertyGroup>

  <ItemGroup>
    <PackageReference Include="Pulumi" Version="3.*" />
  </ItemGroup>

</Project>
`,
		"README.md": csharpDistributableReadme(componentName, className),
		"PulumiPlugin.yaml": fmt.Sprintf(`name: %s
runtime: dotnet
version: 0.1.0
`, componentName),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := writeScaffoldFile(filePath, content); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
	}

	fmt.Printf("Created C# component '%s' in %s\n", componentName, targetDir)
	return nil
}

// ScaffoldJavaComponent scaffolds a multi-language Java component.
func ScaffoldJavaComponent(targetDir, componentName string) error {
	className := toTitleCase(componentName)
	packageName := strings.ToLower(strings.ReplaceAll(componentName, "-", ""))

	srcDir := filepath.Join("src", "main", "java", packageName)
	if err := os.MkdirAll(filepath.Join(targetDir, srcDir), 0o755); err != nil {
		return fmt.Errorf("creating source directory: %w", err)
	}

	files := map[string]string{
		filepath.Join(srcDir, className+".java"): fmt.Sprintf(`package %s;

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
`, packageName, className, className, className, packageName,
			className, className, className, className, className, className, className),
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
		"README.md": javaDistributableReadme(componentName, packageName, className),
		"PulumiPlugin.yaml": fmt.Sprintf(`name: %s
runtime: java
version: 0.1.0
`, componentName),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := writeScaffoldFile(filePath, content); err != nil {
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
		"README.md": yamlDistributableReadme(componentName),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := writeScaffoldFile(filePath, content); err != nil {
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

// ScaffoldPythonComponentSingleLanguage scaffolds a single-language Python component.
func ScaffoldPythonComponentSingleLanguage(targetDir, componentName string) error {
	packageName := strings.ReplaceAll(componentName, "-", "_")
	className := toTitleCase(packageName)

	files := map[string]string{
		packageName + ".py": fmt.Sprintf(`from typing import Optional
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
		"README.md": pythonLocalReadme(componentName, packageName, className),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := writeScaffoldFile(filePath, content); err != nil {
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
		"README.md": typescriptLocalReadme(componentName),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := writeScaffoldFile(filePath, content); err != nil {
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

	Message pulumi.StringOutput `+"`pulumi:\"message\"`"+`
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
		"README.md": goLocalReadme(componentName, className),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := writeScaffoldFile(filePath, content); err != nil {
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
		className + ".cs": fmt.Sprintf(`using Pulumi;
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
		className + ".csproj": `<Project Sdk="Microsoft.NET.Sdk">

  <PropertyGroup>
    <OutputType>Library</OutputType>
    <TargetFramework>net8.0</TargetFramework>
    <Nullable>enable</Nullable>
  </PropertyGroup>

  <ItemGroup>
    <PackageReference Include="Pulumi" Version="3.*" />
  </ItemGroup>

</Project>
`,
		"README.md": csharpLocalReadme(componentName, className),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := writeScaffoldFile(filePath, content); err != nil {
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
	if err := os.MkdirAll(filepath.Join(targetDir, srcDir), 0o755); err != nil {
		return fmt.Errorf("creating source directory: %w", err)
	}

	files := map[string]string{
		filepath.Join(srcDir, className+".java"): fmt.Sprintf(`package %s;

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
`, packageName, className, className, className, packageName,
			className, className, className, className, className, className, className),
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
		"README.md": javaLocalReadme(componentName, packageName, className),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := writeScaffoldFile(filePath, content); err != nil {
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
		"README.md": yamlLocalReadme(componentName),
	}

	for filename, content := range files {
		filePath := filepath.Join(targetDir, filename)
		if err := writeScaffoldFile(filePath, content); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
	}

	fmt.Printf("Created YAML single-language component '%s' in %s\n", componentName, targetDir)
	return nil
}

func packageName(componentName string) string {
	return strings.ReplaceAll(componentName, "-", "_")
}
