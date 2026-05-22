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

package otel_logging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNodeJSFullProgramLogging runs a complete Pulumi up with a
// Node.js program that sends a log via the OTLP logger with a known
// property value. It verifies the log arrives in the encrypted log
// via the full pipeline: Node.js SDK → OTLP → receiver →
// SlogLogExporter → slog → encrypted log.
func TestNodeJSFullProgramLogging(t *testing.T) {
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.Env = append(e.Env, "PULUMI_ENABLE_AUTOMATIC_LOGGING=true")

	e.ImportDirectory(filepath.Join("testdata", "nodejs-otel-logging"))

	e.RunCommand("yarn", "install")
	e.RunCommand("yarn", "link", "@pulumi/pulumi")

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "dev")
	e.RunCommand("pulumi", "up", "--yes", "--skip-preview",
		"--otel-traces", "file:///dev/null")

	// Find the log files.
	logsDir := filepath.Join(e.HomePath, "logs")
	entries, err := os.ReadDir(logsDir)
	require.NoError(t, err)

	var allLogs strings.Builder
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		logFile := filepath.Join(logsDir, entry.Name())
		stdout, _ := e.RunCommand("pulumi", "logs", "decrypt", logFile)
		t.Logf("Log file: %s (%d bytes decrypted)", entry.Name(), len(stdout))
		allLogs.WriteString(stdout)
	}

	logContent := allLogs.String()
	require.NotEmpty(t, logContent, "expected log content")

	// Find the specific OTLP log line from the Node.js SDK.
	var otelLine string
	for _, line := range strings.Split(logContent, "\n") {
		if strings.Contains(line, "nodejs-otel-marker-message") {
			otelLine = line
			break
		}
	}
	require.NotEmpty(t, otelLine,
		"expected a log line containing 'nodejs-otel-marker-message' from the Node.js OTLP logger")
	t.Logf("OTLP log line: %s", otelLine)

	// Parse and compare the full log line (except time which varies).
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(otelLine), &got))
	delete(got, "time")

	expected := map[string]any{
		"level":  "INFO",
		"msg":    "nodejs-otel-marker-message",
		"inputs": `{"bucketName":"otel-logging-test-bucket-xyzzy","region":"us-west-2"}`,
	}
	assert.Equal(t, expected, got)
}
