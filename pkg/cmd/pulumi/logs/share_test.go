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
	"context"
	"crypto/rand"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testSessionKey = make([]byte, 32)

func init() {
	// Generate a random test session key.
	if _, err := rand.Read(testSessionKey); err != nil {
		panic(err)
	}

	createEncryptionSession = func(_ context.Context, _ pkgWorkspace.Context) (string, []byte, error) {
		return "mock-session-id", testSessionKey, nil
	}
}

func TestShareGzipLogIntegration(t *testing.T) {
	t.Parallel()

	e := newTestEnv(t)

	content := "I 12:00:00.000000 test log line for sharing\n"
	writeGzipFile(t, e, "test-20260401T120000.log", content)

	stdout, _ := e.RunCommand("pulumi", "logs", "share",
		filepath.Join(e.RootPath, "test-20260401T120000.log"))
	_ = stdout

	// The shared file should exist and be a PLOG file.
	shared := filepath.Join(e.RootPath, "test-20260401T120000.shared.log")
	assertFileHasPrefix(t, shared, "PLOG")

	// The original plaintext should not appear in the shared file.
	assertFileDoesNotContain(t, shared, "test log line for sharing")
}

func TestShareEncryptedLogIntegration(t *testing.T) {
	t.Parallel()

	e := newTestEnv(t)
	e.Env = append(e.Env, "PULUMI_ENABLE_AUTOMATIC_LOGGING=true")

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.WriteTestFile("Pulumi.yaml", `name: test-share
runtime: nodejs`)
	e.RunCommand("pulumi", "stack", "init", "dev")
	e.RunCommandExpectError("pulumi", "preview")

	logFile := findLogInDir(t, e.HomePath)
	e.RunCommand("pulumi", "logs", "share", logFile)

	shared := strings.TrimSuffix(logFile, ".log") + ".shared.log"
	assertFileHasPrefix(t, shared, "PLOG")
}

func TestRedactSecretsInLog(t *testing.T) {
	t.Parallel()

	// Build a log line with a secret property value.
	secret := map[string]any{
		resource.SigKey: resource.SecretSig,
		"ciphertext":    "encrypted-secret-value",
	}
	rec := map[string]any{
		"msg":    "registering resource",
		"inputs": map[string]any{"password": secret, "name": "mydb"},
	}
	line, err := json.Marshal(rec)
	require.NoError(t, err)
	logData := append(line, '\n')

	var redacted bytes.Buffer
	require.NoError(t, formatLogRecords(bytes.NewReader(logData), &redacted, true))

	var got map[string]any
	require.NoError(t, json.Unmarshal(redacted.Bytes(), &got))

	inputs := got["inputs"].(map[string]any)
	pw := inputs["password"].(map[string]any)
	assert.Equal(t, resource.SecretSig, pw[resource.SigKey])
	assert.Equal(t, "[secret]", pw["plaintext"])
	_, hasCiphertext := pw["ciphertext"]
	assert.False(t, hasCiphertext, "ciphertext should be removed")

	// Non-secret values should be preserved.
	assert.Equal(t, "mydb", inputs["name"])
	assert.Equal(t, "registering resource", got["msg"])
}

func TestRedactSecretsNested(t *testing.T) {
	t.Parallel()

	// Secrets nested inside arrays and deep maps.
	secret := map[string]any{
		resource.SigKey: resource.SecretSig,
		"plaintext":     `"super-secret"`,
	}
	rec := map[string]any{
		"msg": "outputs",
		"values": []any{
			map[string]any{"key": "public", "val": "ok"},
			map[string]any{"key": "private", "val": secret},
		},
	}
	line, err := json.Marshal(rec)
	require.NoError(t, err)

	var redacted bytes.Buffer
	require.NoError(t, formatLogRecords(bytes.NewReader(append(line, '\n')), &redacted, true))

	var got map[string]any
	require.NoError(t, json.Unmarshal(redacted.Bytes(), &got))

	values := got["values"].([]any)
	pub := values[0].(map[string]any)
	assert.Equal(t, "ok", pub["val"])

	priv := values[1].(map[string]any)
	privVal := priv["val"].(map[string]any)
	assert.Equal(t, "[secret]", privVal["plaintext"])
}

func TestRedactSecretsNonJSON(t *testing.T) {
	t.Parallel()

	// Non-JSON lines should pass through unchanged.
	input := []byte("plain text log line\n{bad json\n")
	var result bytes.Buffer
	require.NoError(t, formatLogRecords(bytes.NewReader(input), &result, true))
	assert.Equal(t, input, result.Bytes())
}

func TestRedactSecretsNoSecrets(t *testing.T) {
	t.Parallel()

	rec := map[string]any{"msg": "hello", "level": "info"}
	line, _ := json.Marshal(rec)
	input := append(line, '\n')

	var result bytes.Buffer
	require.NoError(t, formatLogRecords(bytes.NewReader(input), &result, true))

	var got map[string]any
	require.NoError(t, json.Unmarshal(result.Bytes(), &got))
	assert.Equal(t, "hello", got["msg"])
	assert.Equal(t, "info", got["level"])
}

func TestIsSecretValue(t *testing.T) {
	t.Parallel()

	assert.True(t, isSecretValue(map[string]any{
		resource.SigKey: resource.SecretSig,
		"ciphertext":    "...",
	}))

	// Non-secret signature.
	assert.False(t, isSecretValue(map[string]any{
		resource.SigKey: "some-other-sig",
	}))

	// No signature key.
	assert.False(t, isSecretValue(map[string]any{
		"ciphertext": "...",
	}))

	// Empty map.
	assert.False(t, isSecretValue(map[string]any{}))
}

func TestWriteEncryptedLog(t *testing.T) {
	t.Parallel()

	outPath := filepath.Join(t.TempDir(), "test.shared.log")
	sessionKey := make([]byte, 32)
	_, err := rand.Read(sessionKey)
	require.NoError(t, err)

	content := []byte("test log content\n")
	err = writeEncryptedLog(outPath, "test-session-123", sessionKey, content)
	require.NoError(t, err)

	// Verify it's a valid PLOG file.
	assertFileHasPrefix(t, outPath, "PLOG")
	assertFileDoesNotContain(t, outPath, "test log content")
}

func TestRedactSecretsPreservesMultipleLines(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	for _, msg := range []string{"first", "second", "third"} {
		line, _ := json.Marshal(map[string]any{"msg": msg})
		buf.Write(line)
		buf.WriteByte('\n')
	}

	var result bytes.Buffer
	require.NoError(t, formatLogRecords(bytes.NewReader(buf.Bytes()), &result, true))
	lines := strings.Split(strings.TrimSuffix(result.String(), "\n"), "\n")
	require.Len(t, lines, 3)

	for i, expected := range []string{"first", "second", "third"} {
		var got map[string]any
		require.NoError(t, json.Unmarshal([]byte(lines[i]), &got))
		assert.Equal(t, expected, got["msg"])
	}
}
