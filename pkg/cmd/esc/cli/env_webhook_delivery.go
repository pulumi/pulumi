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

func newEnvWebhookDeliveryCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delivery",
		Short: "Manage environment webhook deliveries",
		Long: "[EXPERIMENTAL] Manage environment webhook deliveries\n" +
			"\n" +
			"A delivery is a single attempt to deliver a webhook payload.",
		Args: cobra.NoArgs,
	}

	cmd.AddCommand(newEnvWebhookDeliveryListCmd(env))

	return cmd
}
