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

func newEnvScheduleCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Manage environment scheduled actions",
		Long: "[EXPERIMENTAL] Manage environment scheduled actions\n" +
			"\n" +
			"A scheduled action runs against an environment on a cron schedule or at a single\n" +
			"point in time. Today the CLI exposes secret-rotation schedules.",
		Args: cobra.NoArgs,
	}

	cmd.AddCommand(newEnvScheduleEditCmd(env))
	cmd.AddCommand(newEnvScheduleGetCmd(env))
	cmd.AddCommand(newEnvScheduleHistoryCmd(env))
	cmd.AddCommand(newEnvScheduleListCmd(env))
	cmd.AddCommand(newEnvScheduleNewCmd(env))
	cmd.AddCommand(newEnvScheduleRemoveCmd(env))

	return cmd
}
