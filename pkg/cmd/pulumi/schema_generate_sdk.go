// Copyright 2016-2021, Pulumi Corporation.
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
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/cobra"

	dotnetgen "github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	gogen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	nodejsgen "github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	pythongen "github.com/pulumi/pulumi/pkg/v3/codegen/python"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func newSchemaGenerateSDKCommand() *cobra.Command {
	var target string
	var outputDirectory string

	cmd := &cobra.Command{
		Use:   "generate-sdk",
		Args:  cmdutil.ExactArgs(1),
		Short: "Generate language SDKs from a Pulumi package schema",
		Long: "Generate language SDKs from a Pulumi package schema.\n" +
			"\n" +
			"Given a Pulumi package schema, write out language SDKs for the package.\n" +
			"The schema is validated as if by `pulumi schema check` prior to generation.\n" +
			"\n" +
			"Valid targets are dotnet, go, nodejs, and python. The 'all' target will\n" +
			"generate an SDK for each suported target.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			if outputDirectory == "" {
				return fmt.Errorf("an output directory must be specified")
			}

			pkgSpec, err := loadSchemaSpec(args[0], "")
			if err != nil {
				return err
			}

			pkg, diags, err := schema.BindSpec(pkgSpec, nil)
			if len(diags) != 0 {
				diagWriter := hcl.NewDiagnosticTextWriter(os.Stderr, nil, 0, true)
				wrErr := diagWriter.WriteDiagnostics(diags)
				contract.IgnoreError(wrErr)

				if diags.HasErrors() {
					return fmt.Errorf("too many errors")
				}
			}

			var files map[string][]byte
			switch target {
			case "all":
				files, err = genAllSDKs(pkg)
			case "dotnet":
				files, err = genDotnetSDK(pkg)
			case "go":
				files, err = genGoSDK(pkg)
			case "nodejs":
				files, err = genNodeSDK(pkg)
			case "python":
				files, err = genPythonSDK(pkg)
			default:
				return fmt.Errorf("unknown target '%v'", target)
			}
			if err != nil {
				return err
			}

			if err = os.MkdirAll(outputDirectory, 0755); err != nil {
				return err
			}
			paths := make([]string, 0, len(files))
			for p := range files {
				paths = append(paths, p)
			}
			sort.Strings(paths)
			for _, path := range paths {
				realPath := filepath.Join(outputDirectory, filepath.FromSlash(path))
				if err = os.MkdirAll(filepath.Dir(realPath), 0755); err != nil {
					return err
				}
				if err = os.WriteFile(realPath, files[path], 0600); err != nil {
					return err
				}
			}

			return nil
		}),
	}

	cmd.PersistentFlags().StringVarP(&target, "target", "t", "all",
		"the target language for SDK generation")
	cmd.PersistentFlags().StringVarP(&outputDirectory, "output-directory", "o", "",
		"the output directory for the generated SDK")

	return cmd
}

const toolName = "'pulumi schema generate'"

func genAllSDKs(pkg *schema.Package) (map[string][]byte, error) {
	files := map[string][]byte{}

	gens := []struct {
		name string
		gen  func(pkg *schema.Package) (map[string][]byte, error)
	}{
		{name: "dotnet", gen: genDotnetSDK},
		{name: "go", gen: genGoSDK},
		{name: "nodejs", gen: genNodeSDK},
		{name: "python", gen: genPythonSDK},
	}
	for _, g := range gens {
		sdk, err := g.gen(pkg)
		if err != nil {
			return nil, fmt.Errorf("failed to generate %v SDK: %w", g.name, err)
		}
		for file, contents := range sdk {
			files[path.Join(g.name, file)] = contents
		}
	}
	return files, nil
}

func genDotnetSDK(pkg *schema.Package) (map[string][]byte, error) {
	return dotnetgen.GeneratePackage(toolName, pkg, nil)
}

func genGoSDK(pkg *schema.Package) (map[string][]byte, error) {
	return gogen.GeneratePackage(toolName, pkg)
}

func genNodeSDK(pkg *schema.Package) (map[string][]byte, error) {
	return nodejsgen.GeneratePackage(toolName, pkg, nil)
}

func genPythonSDK(pkg *schema.Package) (map[string][]byte, error) {
	return pythongen.GeneratePackage(toolName, pkg, nil)
}
