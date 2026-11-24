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

func newEnvWebhookPingCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ping [<org-name>/][<project-name>/]<environment-name> <webhook-name>",
		Short: "Send a test delivery to an environment webhook.",
		Long: "[EXPERIMENTAL] Send a test delivery to an environment webhook\n" +
			"\n" +
			"This command triggers a synthetic delivery against the named webhook and prints\n" +
			"the resulting delivery record.\n",
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
				return errors.New("the ping command does not accept versions")
			}

			webhookName := args[0]
			if webhookName == "" {
				return errors.New("webhook name cannot be empty")
			}

			d, err := env.esc.client.PingEnvironmentWebhook(
				ctx, ref.orgName, ref.projectName, ref.envName, webhookName)
			if err != nil {
				return err
			}

			fmt.Fprintf(env.esc.stdout, "ID: %s\n", d.ID)
			fmt.Fprintf(env.esc.stdout, "Kind: %s\n", d.Kind)
			fmt.Fprintf(env.esc.stdout, "Response code: %d\n", d.ResponseCode)
			fmt.Fprintf(env.esc.stdout, "Duration (ms): %d\n", d.Duration)
			return nil
		},
	}

	return cmd
}
