// Copyright 2016, Pulumi Corporation.
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

package logs

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

func NewLogsCmd(ws pkgWorkspace.Context) *cobra.Command {
	var stackName string

	logsCmd := &cobra.Command{
		Use:   "logs",
		Short: "Manage automatic log files",
		Long: "Manage the automatic log files that the Pulumi CLI writes.\n" +
			"\n" +
			"Use these commands to list, decrypt, share and remove the log files.",
	}

	constrictor.AttachArguments(logsCmd, constrictor.NoArgs)

	logsCmd.AddCommand(newDecryptCmd(ws))
	logsCmd.AddCommand(newListCmd())
	logsCmd.AddCommand(newRemoveCmd())
	logsCmd.AddCommand(newShareCmd(ws, &stackName, createEncryptionSessionFromAPI))

	logsCmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return logsCmd
}

func parseSince(since string, reference time.Time) (*time.Time, error) {
	startTimestamp, err := getTimestamp(since, reference)
	if err != nil {
		return nil, err
	}
	startTimeSec, startTimeNs, err := parseTimestamps(startTimestamp, 0)
	if err != nil {
		return nil, err
	}
	if startTimeSec == 0 && startTimeNs == 0 {
		return nil, nil
	}
	startTime := time.Unix(startTimeSec, startTimeNs)
	return &startTime, nil
}
