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

package packagecmd

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/cobra"
)

func newExtractMappingCommand() *cobra.Command {
	var out string

	cmd := &cobra.Command{
		Use:   "get-mapping <key> <schema_source> [provider key] [provider parameters]",
		Args:  cobra.MinimumNArgs(2),
		Short: "Get the mapping information for a given key from a package",
		Long: `Get the mapping information for a given key from a package.

<schema_source> can be a package name or the path to a plugin binary. [provider key]
is the name of the source provider (e.g. "terraform", if a mapping was being requested
from Terraform to Pulumi). If you need to pass parameters, you must provide a provider
key. In the event that you wish to pass none, you must therefore explicitly pass an
empty string.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			source := args[1]
			var provider string
			if len(args) > 2 {
				provider = args[2]
			}

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

			p, err := ProviderFromSource(pctx, source)
			if err != nil {
				return fmt.Errorf("load provider: %w", err)
			}
			defer p.Close()

			// If provider parameters have been provided, parameterize the provider with them before requesting a mapping.
			if len(args) > 3 {
				parameters := args[3:]
				_, err := p.Parameterize(pctx.Request(), plugin.ParameterizeRequest{
					Parameters: &plugin.ParameterizeArgs{Args: parameters},
				})
				if err != nil {
					return fmt.Errorf("parameterize: %w", err)
				}
			}

			mapping, err := p.GetMapping(cmd.Context(), plugin.GetMappingRequest{
				Key:      key,
				Provider: provider,
			})
			if err != nil {
				return fmt.Errorf("get mapping: %w", err)
			}

			if mapping.Provider == "" {
				return fmt.Errorf("no mapping found for key %q", key)
			}

			fmt.Fprintf(os.Stderr, "%s maps to provider %s\n", source, mapping.Provider)

			// If the user has specified out, then write out the mapping data
			// to a file.
			if out != "" {
				err := os.WriteFile(out, mapping.Data, 0o600)
				if err != nil {
					return fmt.Errorf("write mapping data file: %w", err)
				}
			} else {
				// Otherwise, just write it to stdout
				_, err := os.Stdout.Write(mapping.Data)
				if err != nil {
					return fmt.Errorf("failed to write mapping data to stdout: %w", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&out, "out", "o", "", "The file to write the mapping data to")

	return cmd
}
