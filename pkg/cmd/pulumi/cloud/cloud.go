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

package cloud

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

// NewCloudCmd creates the top-level `pulumi cloud` command group.
func NewCloudCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cloud",
		Short: "Interact with Pulumi Cloud",
		Long: "Interact with Pulumi Cloud.\n\n" +
			"The `api` subcommand calls any endpoint in the Pulumi Cloud REST API.",
	}
	constrictor.AttachArguments(cmd, constrictor.NoArgs)
	cmd.AddCommand(newAPICmd())
	return cmd
}
