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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi-java/pkg/codegen/java"
	"github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	golang "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	"github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/v3/codegen/python"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

const YMLExt = ".yml"

func newSchemaGenCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "generate-sdk",
		Short:      "Generate the SDK for a provider or schema",
		Args:       cmdutil.ExactArgs(1),
		SuggestFor: []string{"generate"},
		Long: "Generate SDKs from providers or schema\n" +
			"\n" +
			"Leverage CrossCode to generate a local copy of the SDK described by the given provider or schema.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			name := args[0]
			version, err := cmd.Flags().GetString("version")
			contract.AssertNoError(err)
			runtime, err := cmd.Flags().GetString("language")
			contract.AssertNoError(err)
			out, err := cmd.Flags().GetString("out")
			contract.AssertNoError(err)

			var v *semver.Version
			if version != "" {
				ver, err := semver.Parse(version)
				if err != nil {
					return fmt.Errorf("failed to parse version: %w", err)
				}
				v = &ver
			}

			pkg, err := fetchSchema(name, v)
			if err != nil {
				return err
			}

			// This means that you can run `pulumi schema generate` without being in a
			// project if you specify the runtime.
			if runtime == "" {
				proj, _, err := readProject()
				if err != nil {
					return err
				}

				runtime = proj.Runtime.Name()
			}

			if pkg.Name == "" {
				return fmt.Errorf("package has no name")
			}

			callGen := func(gen generateSDKFunc, after sdkCleanupFunc) (map[string][]byte, error) {
				files, err := gen("pulumi", pkg, nil)
				if err != nil {
					return nil, err
				}
				if after != nil {
					after(files)
				}
				return files, nil
			}

			if runtime == "all" {
				for _, t := range []string{"nodejs", "dotnet", "python", "go", "java"} {
					wrapErr := func(err error) error {
						return fmt.Errorf("unable to generate %s SDK: %w", t, err)
					}
					gen, after, err := generateSDK(t, pkg.Version)
					if err != nil {
						return wrapErr(err)
					}
					files, err := callGen(gen, after)
					if err != nil {
						return wrapErr(err)
					}

					err = writeFiles(filepath.Join(out, t, pkg.Name), files)
					if err != nil {
						return wrapErr(err)
					}
				}
				return nil
			} else {
				generatePackage, after, err := generateSDK(runtime, pkg.Version)
				if err != nil {
					return fmt.Errorf("unable to generate SDK: %w", err)
				}
				files, err := callGen(generatePackage, after)
				if err != nil {
					return fmt.Errorf("unable to generate SDK: %w", err)
				}

				return writeFiles(filepath.Join(out, pkg.Name), files)
			}
		}),
	}
	cmd.PersistentFlags().String("version", "", "The plugin version to use")
	cmd.PersistentFlags().String("runtime", "", "The pulumi runtime to generate the SDK in. "+
		"One of 'nodejs', 'dotnet', 'python', 'go', 'java' or 'all'.")
	defaultOut := filepath.Join(".", "pulumi-sdks")
	cmd.PersistentFlags().String("out", defaultOut,
		fmt.Sprintf("The output directory to write the SDK to. Default is %s.", defaultOut))

	return cmd
}

type generateSDKFunc func(string, *schema.Package, map[string][]byte) (map[string][]byte, error)
type sdkCleanupFunc func(map[string][]byte)

func generateSDK(runtime string, version *semver.Version) (generateSDKFunc, sdkCleanupFunc, error) {
	noAfter := func(map[string][]byte) {}
	switch runtime {
	case "nodejs":
		return nodejs.GeneratePackage, func(m map[string][]byte) {
			if version != nil {
				pkgJSON := "package.json"
				b, ok := m[pkgJSON]
				contract.Assert(ok)
				m[pkgJSON] = []byte(strings.ReplaceAll(string(b), "${VERSION}", version.String()))
			}
		}, nil
	case "dotnet":
		return dotnet.GeneratePackage, noAfter, nil
	case "python":
		return python.GeneratePackage, noAfter, nil
	case "go":
		return func(tool string, pkg *schema.Package, _ map[string][]byte) (map[string][]byte, error) {
			return golang.GeneratePackage(tool, pkg)
		}, noAfter, nil
	case "java":
		return java.GeneratePackage, noAfter, nil
	default:
		return nil, nil, fmt.Errorf("unable to generate an SDK for the '%s' runtime", runtime)
	}
}

func writeFiles(root string, files map[string][]byte) error {
	for file, bytes := range files {
		relPath := filepath.FromSlash(file)
		path := filepath.Join(root, relPath)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil && !os.IsExist(err) {
			return err
		}

		if err := ioutil.WriteFile(path, bytes, 0600); err != nil {
			return err
		}
	}
	return nil
}

// fetchSchema gets the package.Schema described by nameOrPath and version. If nameOrPath
// is a path, it loads the schema from a file and verifies it is valid. If nameOrPath does
// not end in a valid schema suffix (json, yaml, yml) it attempts to get the schema from
// the provider called nameOrPath.
func fetchSchema(nameOrPath string, version *semver.Version) (*schema.Package, error) {
	// This is a schema file
	if ext := filepath.Ext(nameOrPath); ext == ".json" || ext == encoding.YAMLExt || ext == YMLExt {
		return fetchSchemaFromFile(nameOrPath, version)
	}

	// This is a plugin
	host, err := defaultPluginHost()
	if err != nil {
		return nil, fmt.Errorf("failed to load plugin host: %w", err)
	}
	loader := schema.NewPluginLoader(host)
	pkg, err := loader.LoadPackage(nameOrPath, version)
	if err != nil {
		return nil, fmt.Errorf("failed to load package: %w", err)
	}
	return pkg, nil
}

// fetchSchemaFromFile retrieves a schema from a file at `path`. Schemas can be in either
// JSON or YAML.
func fetchSchemaFromFile(path string, version *semver.Version) (*schema.Package, error) {
	ext := filepath.Ext(path)
	schemaBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not get schema file: %w", err)
	}

	var pkgSpec schema.PackageSpec
	if ext == encoding.YAMLExt || ext == YMLExt {
		err = yaml.Unmarshal(schemaBytes, &pkgSpec)
	} else {
		err = json.Unmarshal(schemaBytes, &pkgSpec)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema: %w", err)
	}

	pkg, diags, err := schema.BindSpec(pkgSpec, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to bind schema: %w", err)
	}
	diagWriter := hcl.NewDiagnosticTextWriter(os.Stderr, nil, 0, true)
	wrErr := diagWriter.WriteDiagnostics(diags)
	contract.IgnoreError(wrErr)
	if diags.HasErrors() {
		return nil, fmt.Errorf("invalid schema")
	}

	if pkg.Version != nil && version != nil && !pkg.Version.EQ(*version) {
		return nil, fmt.Errorf("schema version did not match version flag: %s != %s", pkg.Version, version)
	}
	return pkg, nil
}
