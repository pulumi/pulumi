// Copyright 2023, Pulumi Corporation.
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
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/syntax/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// PulumiSkipConfirmationsEnvVar is an environment variable that can be used to skip confirmation prompts. This matches
// the variable used by the core Pulumi CLI.
const PulumiSkipConfirmationsEnvVar = "PULUMI_SKIP_CONFIRMATIONS"

func newEnvRmCmd(env *envCommand) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "rm [<org-name>/][<project-name>/]<environment-name> [path]",
		Args:  cobra.MaximumNArgs(2),
		Short: "Remove an environment or a value from an environment.",
		Long: "Remove an environment or a value from an environment\n" +
			"\n" +
			"This command removes an environment or a value from an environment." +
			"\n" +
			"When removing an environment, the environment will no longer be available\n" +
			"once this command completes.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			yes = yes || cmdutil.IsTruthy(os.Getenv(PulumiSkipConfirmationsEnvVar))

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, args, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return errors.New("the rm command does not accept versions")
			}

			// Are we removing the entire environment?
			if len(args) == 0 {
				envSlug := fmt.Sprintf("%v/%v/%v", ref.orgName, ref.projectName, ref.envName)

				// Ensure the user really wants to do this.
				prompt := fmt.Sprintf("This will permanently remove the %q environment!", envSlug)
				if !yes && !ui.ConfirmPrompt(prompt, envSlug, display.Options{
					Stdout: env.esc.stdout,
					Stdin:  env.esc.stdin,
					Color:  env.esc.colors,
				}) {
					return errors.New("confirmation declined")
				}

				err = env.esc.client.DeleteEnvironment(ctx, ref.orgName, ref.projectName, ref.envName)
				if err != nil {
					var errResp *apitype.ErrorResponse
					if errors.As(err, &errResp) && errResp.Code == http.StatusConflict &&
						strings.Contains(errResp.Message, "protect") {
						return fmt.Errorf(
							"cannot delete environment: deletion protection is enabled. Disable deletion protection with 'esc env settings set %s deletion-protected false' before deleting", //nolint:lll
							envSlug,
						)
					}
					return err
				}

				msg := fmt.Sprintf("%sEnvironment %q has been removed!%s", colors.SpecAttention, envSlug, colors.Reset)
				fmt.Fprintln(env.esc.stdout, env.esc.colors.Colorize(msg))
				return nil
			}

			// Otherwise, we're removing a single value.
			path, err := resource.ParsePropertyPath(args[0])
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}

			def, tag, _, err := env.esc.client.GetEnvironment(ctx, ref.orgName, ref.projectName, ref.envName, "", false)
			if err != nil {
				return fmt.Errorf("getting environment definition: %w", err)
			}

			var docNode yaml.Node
			if err := yaml.Unmarshal(def, &docNode); err != nil {
				return fmt.Errorf("unmarshaling environment definition: %w", err)
			}
			if docNode.Kind != yaml.DocumentNode {
				return nil
			}
			valuesNode, ok := encoding.YAMLSyntax{Node: &docNode}.Get(resource.PropertyPath{"values"})
			if !ok {
				return nil
			}
			err = encoding.YAMLSyntax{Node: valuesNode}.Delete(nil, path)
			if err != nil {
				return err
			}

			newYAML, err := yaml.Marshal(docNode.Content[0])
			if err != nil {
				return fmt.Errorf("marshaling definition: %w", err)
			}

			diags, err := env.esc.client.UpdateEnvironmentWithProject(
				ctx,
				ref.orgName,
				ref.projectName,
				ref.envName,
				newYAML,
				tag,
			)
			if err != nil {
				return fmt.Errorf("updating environment definition: %w", err)
			}
			if len(diags) != 0 {
				return env.writePropertyEnvironmentDiagnostics(env.esc.stderr, diags)
			}

			return nil
		},
	}

	cmd.PersistentFlags().
		BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts, and proceed with removal anyway")

	return cmd
}
