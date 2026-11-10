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
	"bytes"
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func run(t *testing.T, args ...string) string {
	cmd := NewTelemetryCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs(args)
	require.NoError(t, cmd.Execute())
	return out.String()
}

func TestTelemetryCmd(t *testing.T) {
	home := t.TempDir()
	logPath := filepath.Join(home, "telemetry", "log", "events.jsonl")
	t.Setenv("PULUMI_HOME", home)
	t.Setenv("PULUMI_TELEMETRY", "")
	t.Setenv("PULUMI_TELEMETRY_LOG", "")
	t.Setenv("DO_NOT_TRACK", "")

	require.Equal(t, "Telemetry is enabled.\n", run(t))
	require.Equal(t, "Telemetry is disabled.\n", run(t, "disable"))
	require.Equal(t, "Telemetry is disabled.\nTelemetry events are logged to "+logPath+".\n", run(t, "log"))
	require.Equal(t, "Telemetry is enabled.\nTelemetry events are logged to "+logPath+".\n", run(t, "enable"))
	require.Equal(t, "Telemetry is enabled.\n", run(t, "log", "disable"))

	t.Setenv("DO_NOT_TRACK", "1")
	require.Equal(t, "Telemetry is enabled.\n", run(t))
	require.Equal(t, "Telemetry is disabled.\n", run(t, "disable"))

	t.Setenv("PULUMI_TELEMETRY", "1")
	t.Setenv("PULUMI_TELEMETRY_LOG", "1")
	require.Equal(t, "Telemetry is enabled because PULUMI_TELEMETRY is set.\n"+
		"Telemetry events are logged to "+logPath+" because PULUMI_TELEMETRY_LOG is set.\n", run(t))
}

func runErr(t *testing.T, args ...string) error {
	cmd := NewTelemetryCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestTelemetryCmdDoNotTrack(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("PULUMI_TELEMETRY", "")
	t.Setenv("PULUMI_TELEMETRY_LOG", "")
	t.Setenv("DO_NOT_TRACK", "1")

	require.Equal(t, "Telemetry is disabled because DO_NOT_TRACK is set.\n", run(t))
	require.ErrorContains(t, runErr(t, "log", "maybe"), `expected enable or disable, got "maybe"`)
}
