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

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newPackageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Work with Pulumi packages",
		Long: `Work with Pulumi packages

Subcommands of this command are useful to package authors during development.`,
		Args: cmdutil.NoArgs,
	}

	cmd.AddCommand(newSchemaCheckCommand())
	return cmd
}

type invalidPackageSource struct {
	message    string
	source     string
	underlying error
}

// schemaFromPackageSource takes a schema source and returns its associated schema. A
// schema source is either a file (ending with .[json|y[a]ml]) or a plugin with an
// optional version:
//
//		FILE.[json|y[a]ml] | PLUGIN[@VERSION] | PATH_TO_PLUGIN
//
func schemaFromSchemaSource(packageSource string, loader schema.Loader) (*schema.Package, error) {
	var spec schema.PackageSpec
	bind := func(spec schema.PackageSpec) (*schema.Package, error) {
		// TODO, should use the loader, but that doesn't look available.
		// Probably needs to alter schema.BindSpec to accept a loader.
		panic("unimplemented")
	}
	if ext := filepath.Ext(packageSource); ext == ".yaml" || ext == ".yml" {
		f, err := ioutil.ReadFile(packageSource)
		if err != nil {
			return nil, err
		}
		err = yaml.Unmarshal(f, &spec)
		if err != nil {
			return nil, err
		}
		return bind(spec)
	} else if ext == ".json" {
		f, err := ioutil.ReadFile(packageSource)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(f, &spec)
		if err != nil {
			return nil, err
		}
		return bind(spec)
	}

	info, err := os.Stat(packageSource)
	var version *semver.Version
	pkg := packageSource
	if s := strings.SplitN(pkg, "@", 1); len(s) == 2 {
		// TODO: parse out version, updating
	}
	if os.IsNotExist(err) {
		if strings.ContainsRune(packageSource, filepath.Separator) {
			// We infer that we were given a file path, so we assume that this is a file.
			// The file was not found, so we exit.
			return schema.PackageSpec{}, &invalidPackageSource{
				message:    "",
				source:     packageSource,
				underlying: err,
			}
		}
		// We assume this was a plugin and not a path, so load the plugin.
		pkg, err := loader.LoadPackage(pkg, version)

	}

}
