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
	"io"
	"os"
	"slices"

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

// featureInheritance mirrors the (unexported) requiredFeatures marker in pkg/codegen/schema that a sparse,
// non-flattened schema declares so that consumers which do not understand component inheritance reject it rather than
// silently dropping inherited members.
const featureInheritance = "inheritance"

// schemaUsesInheritance reports whether a package spec relies on component inheritance and therefore needs to be
// normalized to its flattened canonical form before publication: either some resource carries an `extends` reference,
// or the package declares the inheritance requiredFeatures marker.
func schemaUsesInheritance(spec *schema.PackageSpec) bool {
	if slices.Contains(spec.RequiredFeatures, featureInheritance) {
		return true
	}
	for _, res := range spec.Resources {
		if res.Extends != nil {
			return true
		}
	}
	return false
}

// writeExtractedSchema emits the schema for `pulumi package get-schema`. Schemas that use component inheritance may
// arrive in the sparse form analyzers emit (inherited members omitted, marked with requiredFeatures: ["inheritance"]),
// but published schemas must be in the canonical flattened form. For those we bind — which materializes inherited
// members — and re-marshal the bound package, whose output carries the flattened member set and drops the transient
// requiredFeatures marker. Every other schema is printed byte-for-byte as received and bound only afterward to surface
// diagnostics, so its output (and the emit-then-warn ordering) is preserved exactly.
func writeExtractedSchema(out io.Writer, spec *schema.PackageSpec, loader schema.Loader) error {
	normalize := schemaUsesInheritance(spec)
	if normalize {
		bound, err := packages.BindSpec(*spec, loader)
		if err != nil {
			return fmt.Errorf("failed to bind schema: %w", err)
		}
		spec, err = bound.MarshalSpec()
		if err != nil {
			return err
		}
	}

	bytes, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}
	bytes = append(bytes, '\n')
	n, err := out.Write(bytes)
	if err != nil {
		return err
	}
	if len(bytes) != n {
		return fmt.Errorf("only wrote %d/%d bytes of the schema", len(bytes), n)
	}

	if !normalize {
		// Also try to bind the schema to warn about any diagnostics:
		if _, err := packages.BindSpec(*spec, loader); err != nil {
			return fmt.Errorf("failed to bind schema: %w", err)
		}
	}

	return nil
}

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

			return writeExtractedSchema(cmd.OutOrStdout(), spec, schema.NewPluginLoader(pctx))
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
