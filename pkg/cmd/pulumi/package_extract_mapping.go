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
	"fmt"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/cobra"
)

func newExtractMappingCommand() *cobra.Command {
	var out string

	cmd := &cobra.Command{
		Use:   "get-mapping <key> <schema_source> [<provider key>]",
		Args:  cobra.RangeArgs(2, 3),
		Short: "Get the mapping information for a given key from a package",
		Long: `Get the mapping information for a given key from a package.

<schema_source> can be a package name or the path to a plugin binary.`,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			key := args[0]
			source := args[1]
			provider := args[2]

			p, err := providerFromSource(source)
			if err != nil {
				return fmt.Errorf("load provider: %w", err)
			}
			defer p.Close()

			data, mapped, err := p.GetMapping(key, provider)
			if err != nil {
				return fmt.Errorf("get mapping: %w", err)
			}

			if mapped == "" {
				return fmt.Errorf("no mapping found for key %q", key)
			}

			fmt.Printf("%s maps to provider %s\n", source, mapped)

			err = os.WriteFile(out, data, 0o600)
			if err != nil {
				return fmt.Errorf("write mapping data file: %s", err)
			}

			return nil
		}),
	}

	cmd.Flags().StringVarP(&out, "out", "o", "", "The file to write the mapping data to")
	contract.AssertNoErrorf(cmd.MarkFlagRequired("out"), `Could not mark "out" as required`)

	return cmd
}
