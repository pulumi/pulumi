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

package logs

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestDecryptGzipLog(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	content := "I 12:00:00.000000 test log line 1\nI 12:00:01.000000 test log line 2\n"
	logFile := filepath.Join(e.RootPath, "test-20260401T120000.log")
	f, err := os.Create(logFile)
	require.NoError(t, err)
	gz := gzip.NewWriter(f)
	_, err = gz.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, gz.Close())
	require.NoError(t, f.Close())

	stdout, _ := e.RunCommand("pulumi", "logs", "decrypt", logFile)
	assert.Equal(t, content, stdout)
}

func TestFormatLogRecordsFoldsArgs(t *testing.T) {
	t.Parallel()

	// Simulate a JSON log record produced by the slog sink handler.
	// The msg field contains a format string; pulumi.log.argN fields
	// hold the individual arguments that should be folded back in.
	input := map[string]any{
		"time":            "2026-04-30T10:00:00Z",
		"level":           "INFO",
		"msg":             "loading plugin %s version %s",
		"pulumi.log.arg0": "aws",
		"pulumi.log.arg1": "6.0.0",
		"v":               3,
	}
	line, err := json.Marshal(input)
	require.NoError(t, err)

	var out bytes.Buffer
	err = formatLogRecords(bytes.NewReader(append(line, '\n')), &out)
	require.NoError(t, err)

	var got map[string]any
	err = json.Unmarshal(out.Bytes(), &got)
	require.NoError(t, err)

	assert.Equal(t, "loading plugin aws version 6.0.0", got["msg"])

	assert.NotContains(t, got, "pulumi.log.arg0")
	assert.NotContains(t, got, "pulumi.log.arg1")

	assert.Equal(t, "INFO", got["level"])
	assert.EqualValues(t, 3, got["v"])
}

func TestFormatLogRecordsDecodesPropertyValues(t *testing.T) {
	t.Parallel()

	sv, err := structpb.NewValue(map[string]any{"key": "val"})
	require.NoError(t, err)
	encoded, err := logging.EncodeStructValueForLog(sv)
	require.NoError(t, err)

	input := map[string]any{
		"time":            "2026-04-30T10:00:00Z",
		"level":           "INFO",
		"msg":             "resource inputs: %v",
		"pulumi.log.arg0": string(encoded),
	}
	line, err := json.Marshal(input)
	require.NoError(t, err)

	var out bytes.Buffer
	err = formatLogRecords(bytes.NewReader(append(line, '\n')), &out)
	require.NoError(t, err)

	var got map[string]any
	err = json.Unmarshal(out.Bytes(), &got)
	require.NoError(t, err)

	assert.Equal(t, "resource inputs: map[key:val]", got["msg"])
	assert.NotContains(t, got, "pulumi.log.arg0")
}

// TestDecryptEncryptedLog verifies the full flow: automatic logging creates
// an encrypted log file during a `pulumi preview`, and `pulumi logs decrypt`
// can read it back.
func TestDecryptEncryptedLog(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	e.Env = append(e.Env, "PULUMI_ENABLE_AUTOMATIC_LOGGING=true")

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.WriteTestFile("Pulumi.yaml", `name: test-decrypt
runtime: nodejs`)
	e.RunCommand("pulumi", "stack", "init", "dev")

	e.RunCommandExpectError("pulumi", "preview")

	logsDir := filepath.Join(e.HomePath, "logs")
	entries, err := os.ReadDir(logsDir)
	require.NoError(t, err)

	var logFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".log") {
			logFiles = append(logFiles, filepath.Join(logsDir, entry.Name()))
		}
	}
	require.NotEmpty(t, logFiles, "expected at least one log file in %s", logsDir)

	logFile := logFiles[len(logFiles)-1]

	raw, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "Pulumi")

	stdout, _ := e.RunCommand("pulumi", "logs", "decrypt", logFile)

	assert.Contains(t, stdout, "Pulumi")
}

func TestFormatLogChoicesAligns(t *testing.T) {
	t.Parallel()

	now := time.Now()
	entries := []logEntry{
		{path: "/p/dev-1.log", stack: "dev", timestamp: now.Add(-2 * time.Hour), updateID: "abc"},
		{path: "/p/staging-1.log", stack: "staging", timestamp: now.Add(-3 * time.Hour), updateID: "def-update-1"},
		{path: "/p/cli-1.log", stack: "", cliLevel: true, timestamp: now.Add(-4 * time.Hour), updateID: "1234"},
	}

	options, optionMap := formatLogChoices(entries)
	require.Len(t, options, 3)
	require.Len(t, optionMap, 3)

	// Stack column should be left-padded to the widest stack name.
	for _, o := range options {
		assert.True(t, strings.HasPrefix(o, "dev    ") ||
			strings.HasPrefix(o, "staging") ||
			strings.HasPrefix(o, "(cli)  "),
			"unexpected option layout: %q", o)
	}

	// CLI-level entries should render as "(cli)".
	var cliOption string
	for _, o := range options {
		if strings.HasPrefix(o, "(cli)") {
			cliOption = o
		}
	}
	require.NotEmpty(t, cliOption)
	assert.Equal(t, "/p/cli-1.log", optionMap[cliOption])
	assert.Contains(t, cliOption, "1234")
}
