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

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

// Constructs the `pulumi package add` command.
func newPackageAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <provider> [provider-parameter...]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Add a package to your Pulumi project",
		Long: `Add a package to your Pulumi project.

For parameterized providers, parameters should be specified as additional arguments.
The exact format of parameters is provider-specific; consult the provider's
documentation for more information. If the parameters include flags that begin with
dashes, you may need to use '--' to separate the provider name from the parameters,
as in:

  pulumi package add <provider> -- --provider-parameter-flag value`,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			proj, root, err := readProject()
			if err != nil && errors.Is(err, workspace.ErrProjectNotFound) {
				return err
			}

			language := proj.Runtime.Name()
			if language == "yaml" {
				return errors.New("pulumi package add does not currently support YAML projects")
			}

			ctx := cmd.Context()

			plugin := args[0]
			parameters := args[1:]

			var pluginInstallCommand pluginInstallCmd
			err = pluginInstallCommand.Run(ctx, []string{
				"resource", /*plugin kind*/
				plugin,
			})
			if err != nil {
				return fmt.Errorf("failed to install plugin: %w", err)
			}

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

			return printLinkInstructions(language, root, pkg.Name, out)
		}),
	}

	return cmd
}

// Prints instructions for linking a locally generated SDK to an existing
// project, in the absence of us attempting to perform this linking automatically.
func printLinkInstructions(language string, root string, pkg string, out string) error {
	switch language {
	case "nodejs":
		return printNodejsLinkInstructions(root, pkg, out)
	case "python":
		return printPythonLinkInstructions(root, pkg, out)
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
func printNodejsLinkInstructions(root string, pkg string, out string) error {
	fmt.Printf("Successfully generated a Nodejs SDK for the %s package at %s\n", pkg, out)
	fmt.Println()
	fmt.Println("To use this SDK in your Nodejs project, run the following command:")
	fmt.Println()
	proj, _, err := readProject()
	if err != nil {
		return err
	}
	relOut, err := filepath.Rel(root, out)
	if err != nil {
		return err
	}
	packageSpecifier := fmt.Sprintf("%s@file:%s", pkg, relOut)
	addCmd := ""
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
	return nil
}

// Prints instructions for linking a locally generated SDK to an existing Python
// project, in the absence of us attempting to perform this linking automatically.
func printPythonLinkInstructions(root string, pkg string, out string) error {
	fmt.Printf("Successfully generated a Python SDK for the %s package at %s\n", pkg, out)
	fmt.Println()
	fmt.Println("To use this SDK in your Python project, run the following command:")
	fmt.Println()
	proj, _, err := readProject()
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
	fmt.Println()
	return nil
}

// Prints instructions for linking a locally generated SDK to an existing Go
// project, in the absence of us attempting to perform this linking automatically.
func printGoLinkInstructions(root string, pkg string, out string) error {
	return nil
	// TODO: Codify Go linking instructions
}

// Prints instructions for linking a locally generated SDK to an existing .NET
// project, in the absence of us attempting to perform this linking automatically.
func printDotnetLinkInstructions(root string, pkg string, out string) error {
	return nil
	// TODO: Codify .NET linking instructions
}

// Prints instructions for linking a locally generated SDK to an existing Java
// project, in the absence of us attempting to perform this linking automatically.
func printJavaLinkInstructions(root string, pkg string, out string) error {
	fmt.Printf("Successfully generated a Java SDK for the %s package at %s\n", pkg, out)
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
