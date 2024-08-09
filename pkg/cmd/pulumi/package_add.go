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

			out := root
			local := true

			err = genSDK(
				language,
				out,
				pkg,
				"",    /*overlays*/
				local, /*local*/
			)
			if err != nil {
				return fmt.Errorf("failed to generate SDK: %w", err)
			}

			printLinkInstructions(language, plugin, out)

			return nil
		}),
	}

	return cmd
}

// Prints instructions for linking a locally generated SDK to an existing
// project, in the absence of us attempting to perform this linking automatically.
func printLinkInstructions(language string, plugin string, out string) {
	switch language {
	case "nodejs":
		printNodejsLinkInstructions(plugin, out)
	case "python":
		printPythonLinkInstructions(plugin, out)
	case "go":
		printGoLinkInstructions(plugin, out)
	case "dotnet":
		printDotnetLinkInstructions(plugin, out)
	case "java":
		printJavaLinkInstructions(plugin, out)
	default:
		break
	}
}

// Prints instructions for linking a locally generated SDK to an existing NodeJS
// project, in the absence of us attempting to perform this linking automatically.
func printNodejsLinkInstructions(plugin string, out string) {
	// TODO: Codify NodeJS linking instructions
}

// Prints instructions for linking a locally generated SDK to an existing Python
// project, in the absence of us attempting to perform this linking automatically.
func printPythonLinkInstructions(plugin string, out string) {
	// TODO: Codify Python linking instructions
}

// Prints instructions for linking a locally generated SDK to an existing Go
// project, in the absence of us attempting to perform this linking automatically.
func printGoLinkInstructions(plugin string, out string) {
	// TODO: Codify Go linking instructions
}

// Prints instructions for linking a locally generated SDK to an existing .NET
// project, in the absence of us attempting to perform this linking automatically.
func printDotnetLinkInstructions(plugin string, out string) {
	// TODO: Codify .NET linking instructions
}

// Prints instructions for linking a locally generated SDK to an existing Java
// project, in the absence of us attempting to perform this linking automatically.
func printJavaLinkInstructions(plugin string, out string) {
	fmt.Printf("Successfully generated a Java SDK for the %s plugin at %s\n", plugin, out)
	fmt.Println()
	fmt.Println("To use this SDK in your Java project, complete the following steps:")
	fmt.Println()
	fmt.Println("1. Copy the contents of the generated SDK to your Java project:")
	fmt.Printf("     cp -r %[1]s/java/src/* %[1]s/src\n", out)
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
}
