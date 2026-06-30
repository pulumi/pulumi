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
	"github.com/spf13/cobra"
)

func newEnvWebhookCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhook",
		Short: "Manage environment webhooks",
		Long: "[EXPERIMENTAL] Manage environment webhooks\n" +
			"\n" +
			"A webhook delivers JSON event payloads to a URL whenever the environment changes.",
		Args: cobra.NoArgs,
	}

	cmd.AddCommand(newEnvWebhookListCmd(env))
	cmd.AddCommand(newEnvWebhookGetCmd(env))
	cmd.AddCommand(newEnvWebhookNewCmd(env))
	cmd.AddCommand(newEnvWebhookEditCmd(env))
	cmd.AddCommand(newEnvWebhookRmCmd(env))
	cmd.AddCommand(newEnvWebhookPingCmd(env))
	cmd.AddCommand(newEnvWebhookDeliveryCmd(env))

	return cmd
}
