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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newSchemaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Analyze package schemas",
		Long: `Analyze package schemas

Subcommands of this command can be used to analyze Pulumi package schemas. This can be useful to check hand-authored
package schemas for errors.`,
		Args: cmdutil.NoArgs,
	}

	cmd.AddCommand(newSchemaCheckCommand())
	cmd.AddCommand(newSchemaGenerateSDKCommand())
	return cmd
}

func loadSchemaSpec(path, kind string) (schema.PackageSpec, error) {
	// Read from stdin or a specified file
	reader := os.Stdin
	if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			return schema.PackageSpec{}, fmt.Errorf("could not open %v: %w", path, err)
		}
		defer f.Close()

		reader = f
	}
	schemaBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return schema.PackageSpec{}, fmt.Errorf("failed to read schema: %w", err)
	}

	var pkgSpec schema.PackageSpec
	if kind == "" {
		if ext := filepath.Ext(path); ext == ".yaml" || ext == ".yml" {
			kind = "yaml"
		} else {
			kind = "json"
		}
	}

	if kind == "yaml" {
		err = yaml.Unmarshal(schemaBytes, &pkgSpec)
	} else {
		err = json.Unmarshal(schemaBytes, &pkgSpec)
	}
	if err != nil {
		return schema.PackageSpec{}, fmt.Errorf("failed to unmarshal schema: %w", err)
	}
	return pkgSpec, nil
}
