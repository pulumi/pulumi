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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	javagen "github.com/pulumi/pulumi-java/pkg/codegen/java"

	"github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
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
					err := genSDK(lang, out, pkg, overlays)
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
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current working directory: %w", err)
	}

	writeWrapper := func(
		generatePackage func(string, *schema.Package, map[string][]byte) (map[string][]byte, error),
	) func(string, *schema.Package, map[string][]byte) error {
		return func(directory string, p *schema.Package, extraFiles map[string][]byte) error {
			m, err := generatePackage("pulumi", p, extraFiles)
			if err != nil {
				return err
			}

			err = os.RemoveAll(directory)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			for k, v := range m {
				path := filepath.Join(directory, k)
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
	}

	var generatePackage func(string, *schema.Package, map[string][]byte) error
	switch language {
	case "dotnet":
		generatePackage = writeWrapper(dotnet.GeneratePackage)
	case "python":
		generatePackage = writeWrapper(python.GeneratePackage)
	case "java":
		generatePackage = writeWrapper(javagen.GeneratePackage)
	default:
		generatePackage = func(directory string, pkg *schema.Package, extraFiles map[string][]byte) error {
			// Ensure the target directory is clean, but created.
			err = os.RemoveAll(directory)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			err := os.MkdirAll(directory, 0o700)
			if err != nil {
				return err
			}

			jsonBytes, err := pkg.MarshalJSON()
			if err != nil {
				return err
			}

			pCtx, err := newPluginContext(cwd)
			if err != nil {
				return fmt.Errorf("create plugin context: %w", err)
			}
			defer contract.IgnoreClose(pCtx.Host)

			languagePlugin, err := pCtx.Host.LanguageRuntime(cwd, cwd, language, nil)
			if err != nil {
				return err
			}

			err = languagePlugin.GeneratePackage(directory, string(jsonBytes), extraFiles)
			if err != nil {
				return err
			}

			return nil
		}
	}

	extraFiles := make(map[string][]byte)
	if overlays != "" {
		fsys := os.DirFS(filepath.Join(overlays, language))
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

	root := filepath.Join(out, language)
	err = generatePackage(root, pkg, extraFiles)
	if err != nil {
		return err
	}
	return nil
}
