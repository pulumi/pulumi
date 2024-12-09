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
	"fmt"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
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
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			source := args[0]

			pkg, err := schemaFromSchemaSource(cmd.Context(), source, args[1:])
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
		}),
	}
	return cmd
}
