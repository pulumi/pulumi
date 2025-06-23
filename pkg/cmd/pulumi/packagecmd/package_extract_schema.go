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

package packagecmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy/unauthenticatedregistry"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/cobra"
)

func newExtractSchemaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-schema <schema_source> [provider parameters]",
		Args:  cobra.MinimumNArgs(1),
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
			pctx, err := plugin.NewContext(cmd.Context(), sink, sink, nil, nil, wd, nil, false, nil)
			if err != nil {
				return err
			}
			defer func() {
				contract.IgnoreError(pctx.Close())
			}()

			pkg, _, err := SchemaFromSchemaSource(pctx, source, args[1:],
				registry.NewOnDemandRegistry(func() (registry.Registry, error) {
					b, err := cmdBackend.NonInteractiveCurrentBackend(
						cmd.Context(), pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, nil,
					)
					if err == nil && b != nil {
						return b.GetReadOnlyCloudRegistry(), nil
					}
					if b == nil || errors.Is(err, backenderr.ErrLoginRequired) {
						return unauthenticatedregistry.New(cmdutil.Diag(), env.Global()), nil
					}
					return nil, fmt.Errorf("could not get registry backend: %w", err)
				}))
			if err != nil {
				return err
			}
			spec, err := pkg.MarshalSpec()
			if err != nil {
				return err
			}
			bytes, err := json.MarshalIndent(spec, "", "  ")
			if err != nil {
				return err
			}
			bytes = append(bytes, '\n')
			n, err := os.Stdout.Write(bytes)
			if err != nil {
				return err
			}
			if len(bytes) != n {
				return fmt.Errorf("only wrote %d/%d bytes of the schema", len(bytes), n)
			}
			return nil
		},
	}
	return cmd
}
