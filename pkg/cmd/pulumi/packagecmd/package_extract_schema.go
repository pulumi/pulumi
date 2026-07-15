// Copyright 2016, Pulumi Corporation.
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
	"context"
	"encoding/json"
	"fmt"
	"os"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packages"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageworkspace"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkghost "github.com/pulumi/pulumi/pkg/v3/host"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/cobra"
)

func newExtractSchemaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-schema",
		Short: "Get the schema.json from a package",
		Long: `Get the schema.json from a package.

<schema_source> can be a package name or the path to a plugin binary or folder.
If a folder either the plugin binary must match the folder name (e.g. 'aws' and 'pulumi-resource-aws')` +
			` or it must have a PulumiPlugin.yaml file specifying the runtime to use.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			source := args[0]

			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			sink := cmdutil.Diag()
			registry := cmdCmd.NewDefaultRegistry(
				cmd.Context(), cmdBackend.DefaultLoginManager, pkgWorkspace.Instance, nil, sink, env.Global())
			pluginHost, err := pkghost.New(context.WithoutCancel(cmd.Context()), sink, sink, nil,
				pkgWorkspace.EnsureLanguageInstalled, schema.NewLoaderServerFromContext, convert.NewMapperServerFromContext,
				packageworkspace.NewResolverServer(registry))
			if err != nil {
				return err
			}
			// host is owned here, closed after the context
			defer contract.IgnoreClose(pluginHost)
			pctx, err := plugin.NewContext(
				cmd.Context(), sink, sink, pluginHost, nil, wd, nil, false,
				nil)
			if err != nil {
				return err
			}
			defer contract.IgnoreClose(pctx)

			parameters := &plugin.ParameterizeArgs{Args: args[1:]}
			spec, _, err := packages.SchemaFromSchemaSource(pkgWorkspace.Instance, pctx, source, parameters,
				registry, env.Global(), 0 /* unbounded concurrency */)
			if err != nil {
				return err
			}
			bytes, err := json.MarshalIndent(spec, "", "  ")
			if err != nil {
				return err
			}
			bytes = append(bytes, '\n')
			n, err := cmd.OutOrStdout().Write(bytes)
			if err != nil {
				return err
			}
			if len(bytes) != n {
				return fmt.Errorf("only wrote %d/%d bytes of the schema", len(bytes), n)
			}

			// Also try to bind the schema to warn about any diagnostics:
			_, err = packages.BindSpec(*spec, schema.NewPluginLoader(pctx))
			if err != nil {
				return fmt.Errorf("failed to bind schema: %w", err)
			}

			return nil
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
	cmd.Use = "get-schema <schema-source> [flags] [--] [provider-parameter]..."

	return cmd
}
