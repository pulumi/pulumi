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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/blang/semver"
	"github.com/spf13/cobra"

	cmdDiag "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/diag"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"

	"github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
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
			pkg, err := schemaFromSchemaSource(cmd.Context(), source, args[1:])
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
					err := genSDK(lang, out, pkg, overlays, local)
					if err != nil {
						return err
					}
				}
				fmt.Fprintf(os.Stderr, "SDKs have been written to %s", out)
				return nil
			}
			err = genSDK(language, out, pkg, overlays, local)
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

func genSDK(language, out string, pkg *schema.Package, overlays string, local bool) error {
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
		generatePackage = writeWrapper(func(t string, p *schema.Package, e map[string][]byte) (map[string][]byte, error) {
			return dotnet.GeneratePackage(t, p, e, nil)
		})
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
			programInfo := plugin.NewProgramInfo(cwd, cwd, ".", nil)
			languagePlugin, err := pCtx.Host.LanguageRuntime(language, programInfo)
			if err != nil {
				return err
			}

			loader := schema.NewPluginLoader(pCtx.Host)
			loaderServer := schema.NewLoaderServer(loader)
			grpcServer, err := plugin.NewServer(pCtx, schema.LoaderRegistration(loaderServer))
			if err != nil {
				return err
			}
			defer contract.IgnoreClose(grpcServer)

			diags, err := languagePlugin.GeneratePackage(directory, string(jsonBytes), extraFiles, grpcServer.Addr(), nil, local)
			if err != nil {
				return err
			}

			// These diagnostics come directly from the converter and so _should_ be user friendly. So we're just
			// going to print them.
			cmdDiag.PrintDiagnostics(pCtx.Diag, diags)
			if diags.HasErrors() {
				// If we've got error diagnostics then package generation failed, we've printed the error above so
				// just return a plain message here.
				return errors.New("generation failed")
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

func newPluginContext(cwd string) (*plugin.Context, error) {
	sink := diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{
		Color: cmdutil.GetGlobalColorization(),
	})
	pluginCtx, err := plugin.NewContext(sink, sink, nil, nil, cwd, nil, true, nil)
	if err != nil {
		return nil, err
	}
	return pluginCtx, nil
}
