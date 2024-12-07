// Copyright 2024, Pulumi Corporation.
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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	go_gen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	"github.com/pulumi/pulumi/pkg/v3/codegen/python"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Constructs the `pulumi package add` command.
func newPackageAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <provider|schema> [provider-parameter...]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Add a package to your Pulumi project",
		Long: `Add a package to your Pulumi project.

This command locally generates an SDK in the currently selected Pulumi language
and prints instructions on how to link it into your project. The SDK is based on
a Pulumi package schema extracted from a given resource plugin or provided
directly.

When <provider> is specified as a PLUGIN[@VERSION] reference, Pulumi attempts to
resolve a resource plugin first, installing it on-demand, similarly to:

  pulumi plugin install resource PLUGIN [VERSION]

When <provider> is specified as a local path, Pulumi executes the provider
binary to extract its package schema:

  pulumi package add ./my-provider

For parameterized providers, parameters may be specified as additional
arguments. The exact format of parameters is provider-specific; consult the
provider's documentation for more information. If the parameters include flags
that begin with dashes, you may need to use '--' to separate the provider name
from the parameters, as in:

  pulumi package add <provider> -- --provider-parameter-flag value

When <schema> is a path to a local file with a '.json', '.yml' or '.yaml'
extension, Pulumi package schema is read from it directly:

  pulumi package add ./my/schema.json`,
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			ws := pkgWorkspace.Instance
			proj, root, err := ws.ReadProject()
			if err != nil && errors.Is(err, workspace.ErrProjectNotFound) {
				return err
			}

			language := proj.Runtime.Name()

			ctx := cmd.Context()

			plugin := args[0]
			parameters := args[1:]

			pkg, err := schemaFromSchemaSource(ctx, plugin, parameters)
			if err != nil {
				return fmt.Errorf("failed to get schema: %w", err)
			}

			tempOut, err := os.MkdirTemp("", "pulumi-package-add-")
			if err != nil {
				return fmt.Errorf("failed to create temporary directory: %w", err)
			}

			local := true

			err = genSDK(
				language,
				tempOut,
				pkg,
				"",    /*overlays*/
				local, /*local*/
			)
			if err != nil {
				return fmt.Errorf("failed to generate SDK: %w", err)
			}

			out := filepath.Join(root, "sdks")
			err = os.MkdirAll(out, 0o755)
			if err != nil {
				return fmt.Errorf("failed to create directory for SDK: %w", err)
			}

			out = filepath.Join(out, pkg.Name)
			err = copyAll(out, filepath.Join(tempOut, language))
			if err != nil {
				return fmt.Errorf("failed to move SDK to project: %w", err)
			}

			err = os.RemoveAll(tempOut)
			if err != nil {
				return fmt.Errorf("failed to remove temporary directory: %w", err)
			}

			return printLinkInstructions(ws, language, root, pkg, out)
		}),
	}

	return cmd
}

// Prints instructions for linking a locally generated SDK to an existing
// project, in the absence of us attempting to perform this linking automatically.
func printLinkInstructions(
	ws pkgWorkspace.Context, language string, root string, pkg *schema.Package, out string,
) error {
	switch language {
	case "nodejs":
		return printNodejsLinkInstructions(ws, root, pkg, out)
	case "python":
		return printPythonLinkInstructions(ws, root, pkg, out)
	case "go":
		return printGoLinkInstructions(root, pkg, out)
	case "dotnet":
		return printDotnetLinkInstructions(root, pkg, out)
	case "java":
		return printJavaLinkInstructions(root, pkg, out)
	default:
		break
	}
	return nil
}

// Prints instructions for linking a locally generated SDK to an existing NodeJS
// project, in the absence of us attempting to perform this linking automatically.
func printNodejsLinkInstructions(ws pkgWorkspace.Context, root string, pkg *schema.Package, out string) error {
	fmt.Printf("Successfully generated a Nodejs SDK for the %s package at %s\n", pkg.Name, out)
	fmt.Println()
	fmt.Println("To use this SDK in your Nodejs project, run the following command:")
	fmt.Println()
	proj, _, err := ws.ReadProject()
	if err != nil {
		return err
	}
	relOut, err := filepath.Rel(root, out)
	if err != nil {
		return err
	}
	packageSpecifier := fmt.Sprintf("%s@file:%s", pkg.Name, relOut)
	var addCmd string
	options := proj.Runtime.Options()
	if packagemanager, ok := options["packagemanager"]; ok {
		if pm, ok := packagemanager.(string); ok {
			switch pm {
			case "npm":
				fallthrough
			case "yarn":
				fallthrough
			case "pnpm":
				addCmd = pm + " add " + packageSpecifier
			default:
				return fmt.Errorf("unsupported package manager: %s", pm)
			}
		} else {
			fmt.Println("packagemanager", packagemanager)
			return fmt.Errorf("packagemanager option must be a string: %v", packagemanager)
		}
	} else {
		// Assume npm if no packagemanager is specified
		addCmd = "npm add " + packageSpecifier
	}
	fmt.Println("  " + addCmd)
	fmt.Println()
	useTypescript := true
	if typescript, ok := options["typescript"]; ok {
		if val, ok := typescript.(bool); ok {
			useTypescript = val
		}
	}
	if useTypescript {
		fmt.Println("You can then import the SDK in your TypeScript code with:")
		fmt.Println()
		fmt.Printf("  import * as %s from \"%s\";\n", pkg.Name, pkg.Name)
	} else {
		fmt.Println("You can then import the SDK in your Javascript code with:")
		fmt.Println()
		fmt.Printf("  const %s = require(\"%s\");\n", pkg.Name, pkg.Name)
	}
	fmt.Println()
	return nil
}

// Prints instructions for linking a locally generated SDK to an existing Python
// project, in the absence of us attempting to perform this linking automatically.
func printPythonLinkInstructions(ws pkgWorkspace.Context, root string, pkg *schema.Package, out string) error {
	fmt.Printf("Successfully generated a Python SDK for the %s package at %s\n", pkg.Name, out)
	fmt.Println()
	fmt.Println("To use this SDK in your Python project, run the following command:")
	fmt.Println()
	proj, _, err := ws.ReadProject()
	if err != nil {
		return err
	}
	packageSpecifier, err := filepath.Rel(root, out)
	if err != nil {
		return err
	}
	pipInstructions := func() {
		fmt.Printf("  echo %s >> requirements.txt\n\n", packageSpecifier)
		fmt.Printf("  pulumi install\n")
	}
	options := proj.Runtime.Options()
	if toolchain, ok := options["toolchain"]; ok {
		if tc, ok := toolchain.(string); ok {
			switch tc {
			case "pip":
				pipInstructions()
			case "poetry":
				fmt.Println("  poetry add " + packageSpecifier)
			default:
				return fmt.Errorf("unsupported package manager: %s", tc)
			}
		} else {
			return fmt.Errorf("packagemanager option must be a string: %v", toolchain)
		}
	} else {
		// Assume pip if no packagemanager is specified
		pipInstructions()
	}

	pyInfo, ok := pkg.Language["python"].(python.PackageInfo)
	var importName string
	if ok && pyInfo.PackageName != "" {
		importName = pyInfo.PackageName
	} else {
		importName = strings.ReplaceAll(pkg.Name, "-", "_")
	}

	fmt.Println()
	fmt.Println("You can then import the SDK in your Python code with:")
	fmt.Println()
	fmt.Printf("  import pulumi_%s as %s\n", importName, importName)
	fmt.Println()
	return nil
}

// Prints instructions for linking a locally generated SDK to an existing Go
// project, in the absence of us attempting to perform this linking automatically.
func printGoLinkInstructions(root string, pkg *schema.Package, out string) error {
	fmt.Printf("Successfully generated a Go SDK for the %s package at %s\n", pkg.Name, out)
	fmt.Println()
	fmt.Println("To use this SDK in your Go project, run the following command:")
	fmt.Println()

	relOut, err := filepath.Rel(root, out)
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		relOut = ".\\" + relOut
	} else {
		relOut = "./" + relOut
	}

	if err := pkg.ImportLanguages(map[string]schema.Language{"go": go_gen.Importer}); err != nil {
		return err
	}
	goInfo, ok := pkg.Language["go"].(go_gen.GoPackageInfo)
	if !ok {
		return errors.New("failed to import go language info")
	}

	fmt.Printf("   go mod edit -replace %s=%s\n", goInfo.ModulePath, relOut)
	fmt.Println()
	fmt.Println("You can then use the SDK in your Go code with:")
	fmt.Println()
	fmt.Printf("  import \"%s\"\n", goInfo.ModulePath)
	fmt.Println()
	return nil
}

// csharpPackageName converts a package name to a C#-friendly package name.
// for example "aws-api-gateway" becomes "AwsApiGateway".
func csharpPackageName(pkgName string) string {
	title := cases.Title(language.English)
	parts := strings.Split(pkgName, "-")
	for i, part := range parts {
		parts[i] = title.String(part)
	}
	return strings.Join(parts, "")
}

// Prints instructions for linking a locally generated SDK to an existing .NET
// project, in the absence of us attempting to perform this linking automatically.
func printDotnetLinkInstructions(root string, pkg *schema.Package, out string) error {
	fmt.Printf("Successfully generated a .NET SDK for the %s package at %s\n", pkg.Name, out)
	fmt.Println()
	fmt.Println("To use this SDK in your .NET project, run the following command:")
	fmt.Println()
	relOut, err := filepath.Rel(root, out)
	if err != nil {
		return err
	}

	fmt.Printf("  dotnet add reference %s\n", filepath.Join(".", relOut))
	fmt.Println()
	fmt.Printf("You also need to add the following to your .csproj file of the program:\n")
	fmt.Println()
	fmt.Println("  <DefaultItemExcludes>$(DefaultItemExcludes);sdks/**/*.cs</DefaultItemExcludes>")
	fmt.Println()
	fmt.Println("You can then use the SDK in your .NET code with:")
	fmt.Println()
	fmt.Printf("  using Pulumi.%s;\n", csharpPackageName(pkg.Name))
	fmt.Println()
	return nil
}

// Prints instructions for linking a locally generated SDK to an existing Java
// project, in the absence of us attempting to perform this linking automatically.
func printJavaLinkInstructions(root string, pkg *schema.Package, out string) error {
	fmt.Printf("Successfully generated a Java SDK for the %s package at %s\n", pkg.Name, out)
	fmt.Println()
	fmt.Println("To use this SDK in your Java project, complete the following steps:")
	fmt.Println()
	fmt.Println("1. Copy the contents of the generated SDK to your Java project:")
	fmt.Printf("     cp -r %s/src/* %s/src\n", out, root)
	fmt.Println()
	fmt.Println("2. Add the SDK's dependencies to your Java project's build configuration.")
	fmt.Println("   If you are using Maven, add the following dependencies to your pom.xml:")
	fmt.Println()
	fmt.Println("     <dependencies>")
	fmt.Println("         <dependency>")
	fmt.Println("             <groupId>com.google.code.findbugs</groupId>")
	fmt.Println("             <artifactId>jsr305</artifactId>")
	fmt.Println("             <version>3.0.2</version>")
	fmt.Println("         </dependency>")
	fmt.Println("         <dependency>")
	fmt.Println("             <groupId>com.google.code.gson</groupId>")
	fmt.Println("             <artifactId>gson</artifactId>")
	fmt.Println("             <version>2.8.9</version>")
	fmt.Println("         </dependency>")
	fmt.Println("     </dependencies>")
	fmt.Println()
	return nil
}

// copyAll copies src to dst. If src is a directory, its contents will be copied
// recursively.
func copyAll(dst string, src string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		// Recursively copy all files in a directory.
		files, err := os.ReadDir(src)
		if err != nil {
			return fmt.Errorf("read dir: %w", err)
		}
		for _, file := range files {
			name := file.Name()
			copyerr := copyAll(filepath.Join(dst, name), filepath.Join(src, name))
			if copyerr != nil {
				return copyerr
			}
		}
	} else if info.Mode().IsRegular() {
		// Copy files by reading and rewriting their contents.  Skip other special files.
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		dstdir := filepath.Dir(dst)
		if err = os.MkdirAll(dstdir, 0o700); err != nil {
			return err
		}
		if err = os.WriteFile(dst, data, info.Mode()); err != nil {
			return err
		}
	}

	return nil
}
