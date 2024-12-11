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

package packagecmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/blang/semver"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	pcmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func newGenSdkCommand() *cobra.Command {
	var overlays string
	var language string
	var out string
	var version string
	var local bool
	cmd := &cobra.Command{
		Use:   "gen-sdk <schema_source> [provider parameters]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Generate SDK(s) from a package or schema",
		Long: `Generate SDK(s) from a package or schema.

<schema_source> can be a package name or the path to a plugin binary or folder.
If a folder either the plugin binary must match the folder name (e.g. 'aws' and 'pulumi-resource-aws')` +
			` or it must have a PulumiPlugin.yaml file specifying the runtime to use.`,
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			source := args[0]

			d := diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{Color: cmdutil.GetGlobalColorization()})
			pkg, err := pcmd.SchemaFromSchemaSource(cmd.Context(), source, args[1:])
			if err != nil {
				return err
			}
			if version != "" {
				pkgVersion, err := semver.Parse(version)
				if err != nil {
					return fmt.Errorf("invalid version %q: %w", version, err)
				}
				if pkg.Version != nil {
					d.Infof(diag.Message("", "overriding package version %s with %s"), pkg.Version, pkgVersion)
				}
				pkg.Version = &pkgVersion
			}
			// Normalize from well known language names the the matching runtime names.
			switch language {
			case "csharp", "c#":
				language = "dotnet"
			case "typescript":
				language = "nodejs"
			}

			if language == "all" {
				for _, lang := range []string{"dotnet", "go", "java", "nodejs", "python"} {
					err := pcmd.GenSDK(lang, out, pkg, overlays, local)
					if err != nil {
						return err
					}
				}
				fmt.Fprintf(os.Stderr, "SDKs have been written to %s", out)
				return nil
			}
			err = pcmd.GenSDK(language, out, pkg, overlays, local)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "SDK has been written to %s", filepath.Join(out, language))
			return nil
		}),
	}
	cmd.Flags().StringVarP(&language, "language", "", "all",
		"The SDK language to generate: [nodejs|python|go|dotnet|java|all]")
	cmd.Flags().StringVarP(&out, "out", "o", "./sdk",
		"The directory to write the SDK to")
	cmd.Flags().StringVar(&overlays, "overlays", "", "A folder of extra overlay files to copy to the generated SDK")
	cmd.Flags().StringVar(&version, "version", "", "The provider plugin version to generate the SDK for")
	cmd.Flags().BoolVar(&local, "local", false, "Generate an SDK appropriate for local usage")
	contract.AssertNoErrorf(cmd.Flags().MarkHidden("overlays"), `Could not mark "overlay" as hidden`)
	return cmd
}
