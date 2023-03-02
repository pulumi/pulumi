// Copyright 2016-2022, Pulumi Corporation.
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
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	javagen "github.com/pulumi/pulumi-java/pkg/codegen/java"

	"github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	gogen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	"github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/v3/codegen/python"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func newGenSdkCommand() *cobra.Command {
	var overlays string
	var language string
	var out string
	cmd := &cobra.Command{
		Use:   "gen-sdk <schema_source>",
		Args:  cobra.ExactArgs(1),
		Short: "Generate SDK(s) from a package or schema",
		Long: `Generate SDK(s) from a package or schema.

<schema_source> can be a package name, the path to a plugin binary, or the path to a schema file.`,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			source := args[0]

			pkg, err := schemaFromSchemaSource(source)
			if err != nil {
				return err
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
					// If we're generating all the languages place each one in a subdirectory of the output
					// directory, and grab the overlays from a subdirectory of the overlays directory.
					langOut := filepath.Join(out, language)
					langOverlays := ""
					if overlays != "" {
						langOverlays = filepath.Join(overlays, language)
					}
					err := genSDK(lang, langOut, pkg, langOverlays)
					if err != nil {
						return err
					}
				}
				return nil
			}
			return genSDK(language, out, pkg, overlays)
		}),
	}
	cmd.Flags().StringVarP(&language, "language", "", "all",
		"The SDK language to generate: [nodejs|python|go|dotnet|java|all]")
	cmd.Flags().StringVarP(&out, "out", "o", "./sdk",
		"The directory to write the SDK to")
	cmd.Flags().StringVar(&overlays, "overlays", "", "A folder of extra overlay files to copy to the generated SDK")
	contract.AssertNoErrorf(cmd.Flags().MarkHidden("overlays"), `Could not mark "overlay" as hidden`)
	return cmd
}

func genSDK(language, out string, pkg *schema.Package, overlays string) error {
	var f func(string, *schema.Package, map[string][]byte) (map[string][]byte, error)
	switch language {
	case "dotnet":
		f = dotnet.GeneratePackage
	case "go":
		if overlays != "" {
			return errors.New("overlays are not supported for Go")
		}
		f = func(s string, p *schema.Package, m map[string][]byte) (map[string][]byte, error) {
			return gogen.GeneratePackage(s, pkg)
		}
	case "nodejs":
		f = nodejs.GeneratePackage
	case "python": //nolint:goconst
		f = python.GeneratePackage
	case "java":
		f = javagen.GeneratePackage
	default:
		return fmt.Errorf("unknown language %q", language)
	}

	extraFiles := make(map[string][]byte)
	if overlays != "" {
		fsys := os.DirFS(overlays)
		err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}

			contents, err := fs.ReadFile(fsys, path)
			if err != nil {
				return fmt.Errorf("read overlay file %q: %w", path, err)
			}

			extraFiles[path] = contents
			return nil
		})
		if err != nil {
			return fmt.Errorf("read overlay directory %q: %w", overlays, err)
		}
	}

	m, err := f("pulumi", pkg, extraFiles)
	if err != nil {
		return err
	}
	err = os.RemoveAll(out)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for k, v := range m {
		path := filepath.Join(out, k)
		err := os.MkdirAll(filepath.Dir(path), 0o700)
		if err != nil {
			return err
		}
		err = os.WriteFile(path, v, 0o600)
		if err != nil {
			return err
		}
	}
	return nil
}
