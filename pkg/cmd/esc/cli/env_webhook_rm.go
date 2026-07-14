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

func newEnvWebhookRmCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove [<org-name>/][<project-name>/]<environment-name> <webhook-name>",
		Aliases: []string{"rm", "delete"},
		Short:   "Remove an environment webhook.",
		Long: "[EXPERIMENTAL] Remove an environment webhook\n" +
			"\n" +
			"This command removes the named webhook from the environment.\n",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

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

			webhookName := args[0]
			if webhookName == "" {
				return errors.New("webhook name cannot be empty")
			}

			if err := env.esc.client.DeleteEnvironmentWebhook(
				ctx, ref.orgName, ref.projectName, ref.envName, webhookName); err != nil {
				return err
			}

			fmt.Fprintf(env.esc.stdout, "Removed webhook %s from %s/%s/%s\n",
				webhookName, ref.orgName, ref.projectName, ref.envName)
			return nil
		},
	}

	return cmd
}
