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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

const YMLExt = ".yml"

func newSchemaGenCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "generate",
		Short:   "Generate SDKs from providers or schema",
		Args:    cmdutil.ExactArgs(1),
		Aliases: []string{"gen"},
		Long: "Generate SDKs from providers or schema\n" +
			"\n" +
			"Leverage CrossCode to generate a local copy of the SDK provided by the given provider or schema.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			name := args[0]
			version, err := cmd.Flags().GetString("version")
			contract.AssertNoError(err)
			var v *semver.Version
			if version != "" {
				ver, err := semver.Parse(version)
				if err != nil {
					return fmt.Errorf("failed to parse version: %w", err)
				}
				v = &ver
			}

			pkg, err := getPackage(name, v)
			if err != nil {
				return err
			}

			proj, _, err := readProject()
			if err != nil {
				return err
			}

			var files map[string][]byte
			runtime := proj.Runtime.Name()
			switch runtime {
			case "nodejs":
				files, err = nodejs.GeneratePackage("pulumi", pkg, nil)
			default:
				return fmt.Errorf("unable to generate an SDK for the '%s' runtime", runtime)
			}
			if err != nil {
				return fmt.Errorf("unable to generate SDK: %w", err)
			}

			if pkg.Name == "" {
				return fmt.Errorf("package has no name")
			}

			return writeFiles(filepath.Join(".", "pulumi_sdks", pkg.Name), files)
		}),
	}
	cmd.PersistentFlags().String("version", "", "The plugin version to use")

	return cmd
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

func getPackage(name string, version *semver.Version) (*schema.Package, error) {
	// This is a schema file
	if ext := filepath.Ext(name); ext == ".json" || ext == encoding.YAMLExt || ext == YMLExt {
		schemaBytes, err := ioutil.ReadFile(name)
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

	// This is a plugin
	host, err := defaultPluginHost()
	if err != nil {
		return nil, fmt.Errorf("failed to load plugin host: %w", err)
	}
	loader := schema.NewPluginLoader(host)
	pkg, err := loader.LoadPackage(name, version)
	if err != nil {
		return nil, fmt.Errorf("failed to load package: %w", err)
	}
	return pkg, nil

}

func defaultPluginHost() (plugin.Host, error) {
	var cfg plugin.ConfigSource
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	sink := diag.DefaultSink(&stdio{false}, &stdio{false}, diag.FormatOptions{
		Color: colors.Never,
	})
	context, err := plugin.NewContext(sink, sink, nil, cfg, pwd, nil, false, nil)
	if err != nil {
		return nil, err
	}
	return plugin.NewDefaultHost(context, nil, nil, false)
}

// An io.ReadWriteCloser, whose value indicates if the closer is closed.
type stdio struct{ bool }

func (s *stdio) Read(p []byte) (n int, err error) {
	if s.bool {
		return 0, io.EOF
	}
	return os.Stdin.Read(p)
}

func (s *stdio) Write(p []byte) (n int, err error) {
	if s.bool {
		return 0, io.EOF
	}
	return os.Stdout.Write(p)
}

func (s *stdio) Close() error {
	s.bool = true
	return nil
}
