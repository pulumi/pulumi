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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
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

	cmd.AddCommand(
		newExtractSchemaCommand(),
		newGenSdkCommand(),
	)
	return cmd
}

// schemaFromPackageSource takes a schema source and returns its associated schema. A
// schema source is either a file (ending with .[json|y[a]ml]) or a plugin with an
// optional version:
//
//		FILE.[json|y[a]ml] | PLUGIN[@VERSION] | PATH_TO_PLUGIN
//
func schemaFromSchemaSource(packageSource string) (*schema.Package, error) {
	var spec schema.PackageSpec
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	pCtx, err := plugin.NewContext(nil, nil, nil, nil, wd, nil, false, nil)
	if err != nil {
		return nil, err
	}
	bind := func(spec schema.PackageSpec) (*schema.Package, error) {
		var loader schema.Loader

		pkg, diags, err := schema.BindSpec(spec, loader)
		if err != nil {
			return nil, err
		}
		if diags.HasErrors() {
			return nil, diags
		}
		return pkg, nil
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

	var version *semver.Version
	pkg := packageSource
	if s := strings.SplitN(packageSource, "@", 2); len(s) == 2 {
		pkg = s[0]
		v, err := semver.ParseTolerant(s[1])
		if err != nil {
			return nil, fmt.Errorf("VERSION must be valid semver: %w", err)
		}
		version = &v
	}
	info, err := os.Stat(pkg)
	if os.IsNotExist(err) {
		if strings.ContainsRune(pkg, filepath.Separator) {
			// We infer that we were given a file path, so we assume that this is a file.
			// The file was not found, so we exit.
			return nil, err
		}
		host, err := plugin.NewDefaultHost(pCtx, nil, false, nil)
		if err != nil {
			return nil, err
		}
		// We assume this was a plugin and not a path, so load the plugin.
		return schema.NewPluginLoader(host).LoadPackage(pkg, version)
	} else if err != nil {
		return nil, err
	}
	// We were given a path to a binary, so invoke that.
	if info.Mode()&0111 == 0 {
		return nil, fmt.Errorf("plugin at path %q not executable", pkg)
	}

	host, err := plugin.NewDefaultHost(pCtx, nil, false, nil)
	if err != nil {
		return nil, err
	}
	p, err := plugin.NewProviderFromPath(host, pCtx, pkg)
	if err != nil {
		return nil, err
	}
	defer p.Close()
	bytes, err := p.GetSchema(0)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bytes, &spec)
	if err != nil {
		return nil, err
	}
	return bind(spec)
}
