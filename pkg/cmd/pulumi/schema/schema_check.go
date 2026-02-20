// Copyright 2016-2025, Pulumi Corporation.
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
	"errors"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/cobra"

	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packages"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type checkArgs struct {
	allowDanglingReferences bool
}

func newSchemaCheckCommand() *cobra.Command {
	schemaCheckArgs := checkArgs{}

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check a Pulumi package schema for errors",
		Long: `Check a Pulumi package schema for errors.

Ensure that a Pulumi package schema meets the requirements imposed by the
schema spec as well as additional requirements imposed by the supported
target languages.

<schema_source> can be a package name, the path to a plugin binary or folder,
or a JSON/YAML schema file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			source := args[0]

			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			sink := cmdutil.Diag()
			pctx, err := plugin.NewContext(cmd.Context(), sink, sink, nil, nil, wd, nil, false,
				nil, schema.NewLoaderServerFromHost)
			if err != nil {
				return err
			}
			defer func() {
				contract.IgnoreError(pctx.Close())
			}()

			parameters := &plugin.ParameterizeArgs{Args: args[1:]}
			spec, _, err := packages.SchemaFromSchemaSource(pctx, source, parameters,
				cmdCmd.NewDefaultRegistry(cmd.Context(), pkgWorkspace.Instance, nil, cmdutil.Diag(), env.Global()),
				env.Global(), 0 /* unbounded concurrency */)
			if err != nil {
				return err
			}

			_, diags, err := schema.BindSpec(*spec, nil, schema.ValidationOptions{
				AllowDanglingReferences: schemaCheckArgs.allowDanglingReferences,
			})
			diagWriter := hcl.NewDiagnosticTextWriter(os.Stderr, nil, 0, true)
			wrErr := diagWriter.WriteDiagnostics(diags)
			contract.IgnoreError(wrErr)
			if err == nil && diags.HasErrors() {
				return errors.New("schema validation failed")
			}
			return err
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "schema-source"},
			{Name: "provider-parameter"},
		},
		Required: 1,
		Variadic: true,
	})

	// It's worth mentioning the `--`, as it means that Cobra will stop parsing flags.
	// In other words, a provider parameter can be `--foo` as long as it's after `--`.
	cmd.Use = "check <schema-source> [flags] [--] [provider-parameter]..."

	cmd.PersistentFlags().BoolVar(&schemaCheckArgs.allowDanglingReferences, "allow-dangling-references", false,
		"Whether references to nonexistent types should be considered errors")

	return cmd
}
