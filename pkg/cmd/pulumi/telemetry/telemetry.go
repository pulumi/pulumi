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

package telemetry

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	pkgTelemetry "github.com/pulumi/pulumi/pkg/v3/telemetry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func NewTelemetryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "telemetry",
		Short: "Show or change whether the CLI collects usage telemetry",
		Long: "Show or change whether the CLI collects usage telemetry.\n" +
			"\n" +
			"The Pulumi CLI collects anonymous usage telemetry, such as which commands and flags are used " +
			"and whether they succeed. It never collects stack, project or resource names, flag values, " +
			"or error messages.\n" +
			"\n" +
			"The settings are saved for this Pulumi installation. The PULUMI_TELEMETRY environment variable " +
			"(true or false) overrides them. If neither is set, setting DO_NOT_TRACK disables telemetry.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printStatus(cmd.OutOrStdout())
		},
	}
	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(
		newSetModeCmd("enable", "Enable usage telemetry", pkgTelemetry.ModeEnabled),
		newSetModeCmd("disable", "Disable usage telemetry", pkgTelemetry.ModeDisabled),
		newLogCmd(),
	)
	return cmd
}

func newSetModeCmd(use, short string, mode pkgTelemetry.Mode) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := workspace.GetPulumiHomeDir()
			if err != nil {
				return err
			}
			if err := pkgTelemetry.SetMode(home, mode); err != nil {
				return err
			}
			return printStatus(cmd.OutOrStdout())
		},
	}
	constrictor.AttachArguments(cmd, constrictor.NoArgs)
	return cmd
}

func newLogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log [enable|disable]",
		Short: "Write usage telemetry events to a local log file",
		Long: "Write usage telemetry events to a local log file.\n" +
			"\n" +
			"Every event is written to $PULUMI_HOME/telemetry/log/events.jsonl, marked with whether it is " +
			"sent. Logging works whether or not telemetry is enabled. The PULUMI_TELEMETRY_LOG environment " +
			"variable (true or false) overrides this setting.",
		RunE: func(cmd *cobra.Command, args []string) error {
			enable := len(args) == 0 || args[0] == "enable"
			if !enable && args[0] != "disable" {
				return fmt.Errorf("expected enable or disable, got %q", args[0])
			}
			home, err := workspace.GetPulumiHomeDir()
			if err != nil {
				return err
			}
			if err := pkgTelemetry.SetLog(home, enable); err != nil {
				return err
			}
			return printStatus(cmd.OutOrStdout())
		},
	}
	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "enable|disable"}},
		Required:  0,
	})
	return cmd
}

func printStatus(w io.Writer) error {
	home, err := workspace.GetPulumiHomeDir()
	if err != nil {
		return err
	}
	s, err := pkgTelemetry.Status(home, env.Global())
	if err != nil {
		return err
	}

	var status string
	switch s.Mode {
	case pkgTelemetry.ModeEnabled:
		status = "Telemetry is enabled"
	case pkgTelemetry.ModeDisabled:
		status = "Telemetry is disabled"
	}
	status += because(s.ModeSource, "PULUMI_TELEMETRY")
	if _, err := fmt.Fprintln(w, status+"."); err != nil {
		return err
	}

	if !s.Log {
		return nil
	}
	_, err = fmt.Fprintf(w, "Telemetry events are logged to %s%s.\n", s.LogPath,
		because(s.LogSource, "PULUMI_TELEMETRY_LOG"))
	return err
}

func because(source pkgTelemetry.Source, envVar string) string {
	switch source {
	case pkgTelemetry.SourceEnv:
		return " because " + envVar + " is set"
	case pkgTelemetry.SourceDoNotTrack:
		return " because DO_NOT_TRACK is set"
	case pkgTelemetry.SourceConfig, pkgTelemetry.SourceDefault:
		return ""
	default:
		return ""
	}
}
