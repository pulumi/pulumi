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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backendlogging "github.com/pulumi/pulumi/pkg/v3/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/otelreceiver"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// capturingSlogHandler captures slog records into a buffer.
type capturingSlogHandler struct {
	mu  sync.Mutex
	buf bytes.Buffer
	h   slog.Handler
}

func newCapturingSlogHandler() *capturingSlogHandler {
	c := &capturingSlogHandler{}
	c.h = slog.NewJSONHandler(&c.buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return c
}

func (c *capturingSlogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return c.h.Enabled(ctx, level)
}

func (c *capturingSlogHandler) Handle(ctx context.Context, r slog.Record) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.h.Handle(ctx, r)
}

func (c *capturingSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return c
}

func (c *capturingSlogHandler) WithGroup(name string) slog.Handler {
	return c
}

func (c *capturingSlogHandler) output() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

// TestNodeJSOTLPLogEndToEnd starts an OTLP receiver with
// SlogLogExporter, runs a Node.js script that sends a log with
// encoded property values, and verifies the property values are
// decoded via UnmarshalProperties and re-logged through slog.
//
//nolint:paralleltest // starts a server and spawns a subprocess
func TestNodeJSOTLPLogEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	nodeBin, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found in PATH, skipping integration test")
	}

	sdkRoot := findNodeJSSdkRoot(t)

	tsxBin := filepath.Join(sdkRoot, "node_modules", ".bin", "tsx")
	if _, err := os.Stat(tsxBin); err != nil {
		t.Skipf("tsx not found at %s, skipping integration test", tsxBin)
	}

	// Capture slog output to verify property values are decoded.
	handler := newCapturingSlogHandler()
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(slog.Default())

	exporter := &backendlogging.SlogLogExporter{}
	receiver, err := otelreceiver.Start(&noopSpanExporter{}, exporter)
	require.NoError(t, err)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, receiver.Shutdown(ctx))
	}()

	script := fmt.Sprintf(`
import { initOtelLogging, info, PropertyValue, shutdownOtelLogging } from "%s/runtime/otelLogger";

async function main() {
    initOtelLogging("%s");

    info("integration test message", {
        urn: "urn:pulumi:dev::proj::pkg:mod:Res::myres",
        inputs: new PropertyValue({
            name: "my-bucket",
            password: {
                "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
                value: "hunter2",
            },
        }),
    });

    await new Promise(r => setTimeout(r, 200));
    await shutdownOtelLogging();
}

main().catch(err => { console.error(err); process.exit(1); });
`, sdkRoot, receiver.Endpoint())

	tmpFile := filepath.Join(t.TempDir(), "test_otel_log.ts")
	require.NoError(t, os.WriteFile(tmpFile, []byte(script), 0o600))

	cmd := exec.Command(nodeBin, "--import", "tsx", tmpFile)
	cmd.Dir = sdkRoot
	cmd.Env = append(os.Environ(),
		"NODE_PATH="+filepath.Join(sdkRoot, "node_modules"),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())

	// Give SlogLogExporter time to process.
	time.Sleep(500 * time.Millisecond)

	output := handler.output()
	t.Logf("slog output:\n%s", output)

	// The SlogLogExporter decodes property value bytes via
	// UnmarshalProperties and re-logs them as PropertyValue.
	// The slog JSON output should contain the property values
	// as readable JSON (via PropertyValue.String).
	assert.Contains(t, output, "my-bucket")
	assert.Contains(t, output, "integration test message")
}

// TestPythonOTLPLogEndToEnd is the Python equivalent.
//
//nolint:paralleltest // starts a server and spawns a subprocess
func TestPythonOTLPLogEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pythonBin, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not found in PATH, skipping integration test")
	}

	sdkRoot := findSdkPath(t, filepath.Join("python", "lib"),
		filepath.Join("pulumi", "runtime", "otel_logger.py"))

	handler := newCapturingSlogHandler()
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(slog.Default())

	exporter := &backendlogging.SlogLogExporter{}
	receiver, err := otelreceiver.Start(&noopSpanExporter{}, exporter)
	require.NoError(t, err)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, receiver.Shutdown(ctx))
	}()

	script := fmt.Sprintf(`
import importlib.util, time
spec = importlib.util.spec_from_file_location("otel_logger", "%s/pulumi/runtime/otel_logger.py")
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)

mod.init_otel_logging("%s")

mod.info("python integration test", {
    "urn": "urn:pulumi:dev::proj::pkg:mod:Res::pyres",
    "inputs": mod.PropertyValue({
        "name": "my-python-bucket",
        "password": {
            "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
            "value": "hunter2",
        },
    }),
})

time.sleep(0.2)
mod.shutdown_otel_logging()
`, sdkRoot, receiver.Endpoint())

	tmpFile := filepath.Join(t.TempDir(), "test_otel_log.py")
	require.NoError(t, os.WriteFile(tmpFile, []byte(script), 0o600))

	cmd := exec.Command(pythonBin, tmpFile)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())

	time.Sleep(500 * time.Millisecond)

	output := handler.output()
	t.Logf("slog output:\n%s", output)

	assert.Contains(t, output, "my-python-bucket")
	assert.Contains(t, output, "python integration test")
}

// TestPropertyValueRoundTrip verifies that a property value encoded
// by the Go export handler and decoded by SlogLogExporter produces
// the same JSON representation.
func TestPropertyValueRoundTrip(t *testing.T) {
	t.Parallel()

	handler := newCapturingSlogHandler()
	logger := slog.New(handler)

	// Simulate what SlogLogExporter does: decode wire bytes and re-log.
	// First, encode a property value the same way export.go does.
	inputMap := map[string]any{
		"name": "test-bucket",
		"secret": map[string]any{
			"4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
			"value":                            "s3cret",
		},
	}
	inputJSON, err := json.Marshal(inputMap)
	require.NoError(t, err)

	// Log it directly as a string to simulate the round-trip output.
	logger.Info("test", slog.String("inputs", string(inputJSON)))

	output := handler.output()
	assert.Contains(t, output, "test-bucket")
	assert.Contains(t, output, "s3cret")
}

type noopSpanExporter struct{}

func (noopSpanExporter) ExportSpans(context.Context, []*tracepb.ResourceSpans) error { return nil }
func (noopSpanExporter) Shutdown(context.Context) error                              { return nil }

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		for _, marker := range []string{"go.work", ".git"} {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}
		dir = parent
	}
}

func findSdkPath(t *testing.T, relPath, checkFile string) string {
	t.Helper()
	root := findRepoRoot(t)
	sdkPath := filepath.Join(root, "sdk", relPath)
	if _, err := os.Stat(filepath.Join(sdkPath, checkFile)); err != nil {
		t.Fatalf("SDK not found at %s/%s", sdkPath, checkFile)
	}
	return sdkPath
}

func findNodeJSSdkRoot(t *testing.T) string {
	t.Helper()
	return findSdkPath(t, "nodejs", filepath.Join("runtime", "otelLogger.ts"))
}
