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

package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type checkArgs struct {
	forbidDanglingReferences bool
}

func newSchemaCheckCommand() *cobra.Command {
	schemaCheckArgs := checkArgs{}

	cmd := &cobra.Command{
		Use:   "check",
		Args:  cmdutil.ExactArgs(1),
		Short: "Check a Pulumi package schema for errors",
		Long: "Check a Pulumi package schema for errors.\n" +
			"\n" +
			"Ensure that a Pulumi package schema meets the requirements imposed by the\n" +
			"schema spec as well as additional requirements imposed by the supported\n" +
			"target languages.",
		RunE: func(cmd *cobra.Command, args []string) error {
			file := args[0]

			// Read from stdin or a specified file
			reader := os.Stdin
			if file != "-" {
				f, err := os.Open(file)
				if err != nil {
					return fmt.Errorf("could not open file %v: %w", file, err)
				}
				reader = f
			}
			schemaBytes, err := io.ReadAll(reader)
			if err != nil {
				return fmt.Errorf("failed to read schema: %w", err)
			}

			var pkgSpec schema.PackageSpec
			if ext := filepath.Ext(file); ext == ".yaml" || ext == ".yml" {
				err = yaml.Unmarshal(schemaBytes, &pkgSpec)
			} else {
				err = json.Unmarshal(schemaBytes, &pkgSpec)
			}
			if err != nil {
				return fmt.Errorf("failed to unmarshal schema: %w", err)
			}

			_, diags, err := schema.BindSpec(pkgSpec, nil, schemaCheckArgs.forbidDanglingReferences)
			diagWriter := hcl.NewDiagnosticTextWriter(os.Stderr, nil, 0, true)
			wrErr := diagWriter.WriteDiagnostics(diags)
			contract.IgnoreError(wrErr)
			if err == nil && diags.HasErrors() {
				return errors.New("schema validation failed")
			}
			return err
		},
	}

	cmd.PersistentFlags().BoolVar(&schemaCheckArgs.forbidDanglingReferences, "no-dangling-references", false,
		"Whether references to nonexistent types should be considered errors")

	return cmd
}
