// Copyright 2026, Pulumi Corporation.
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

package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func newEnvDefaultCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "default",
		Args:  cobra.NoArgs,
		Short: "Show the default environment.",
		Long: "Show the default environment\n" +
			"\n" +
			"This command prints the default environment for the working directory and the source\n" +
			"it was inferred from.\n" +
			"\n" +
			"The default environment is taken from the nearest .esc.yaml file in the working\n" +
			"directory or one of its parents. A .esc.yaml file declares an environment either as\n" +
			"a single reference:\n" +
			"\n" +
			"    environment: my-org/my-project/my-env\n" +
			"\n" +
			"or as an anonymous list of imports opened in a named organization:\n" +
			"\n" +
			"    environment:\n" +
			"      organization: my-org\n" +
			"      imports:\n" +
			"        - my-project/base\n" +
			"        - my-project/staging\n" +
			"\n" +
			"If there is no .esc.yaml file, the environments imported by the currently-selected\n" +
			"Pulumi stack are used instead. Note that this only works for stacks whose\n" +
			"configuration is stored in a Pulumi.<stack>.yaml file; stacks whose configuration is\n" +
			"stored in the Pulumi Cloud do not resolve to a default environment.\n",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			def, err := env.resolveDefaultEnvironment(ctx)
			if err != nil {
				return err
			}
			if def == nil {
				return errors.New("no default environment")
			}

			fmt.Fprint(env.esc.stdout, def.description())
			fmt.Fprintf(env.esc.stdout, "source: %v\n", def.source)
			return nil
		},
	}

	return cmd
}
