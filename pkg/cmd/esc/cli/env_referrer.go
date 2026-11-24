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

func newEnvReferrerCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "referrer",
		Short: "Manage environment referrers",
		Long: "Manage environment referrers\n" +
			"\n" +
			"A referrer is an entity that references an environment, such as another environment\n" +
			"that imports it, a Pulumi IaC stack that opens it, or a Pulumi Insights account\n" +
			"that consumes it.\n",
		Args: cobra.NoArgs,
	}

	cmd.AddCommand(newEnvReferrerListCmd(env))

	return cmd
}
