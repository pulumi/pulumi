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

package plugin_logging

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPluginLoggingDecrypt verifies the full plugin logging lifecycle:
// a provider plugin logs a property value, the log is encrypted on disk,
// and `pulumi logs decrypt` can recover the original content.
func TestPluginLoggingDecrypt(t *testing.T) {
	t.Parallel()

	// Build the test provider binary.
	providerDir := t.TempDir()
	binName := "pulumi-resource-testlogging"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(providerDir, binName)

	srcDir, err := filepath.Abs("testprovider")
	require.NoError(t, err)

	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = srcDir
	out, err := build.CombinedOutput()
	require.NoError(t, err, "building test provider: %s", string(out))

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	tracesFile := filepath.Join(e.RootPath, "traces.json")

	e.Env = append(e.Env,
		"PULUMI_ENABLE_AUTOMATIC_LOGGING=true",
	)

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

	pulumiYAML := fmt.Sprintf(`name: test-plugin-logging
runtime: go
plugins:
  providers:
    - name: testlogging
      path: %s
`, providerDir)

	e.WriteTestFile("Pulumi.yaml", pulumiYAML)
	e.WriteTestFile("main.go", goProgram)
	e.WriteTestFile("go.mod", goMod)

	// Set up the SDK replace directive.
	sdkDir, err := filepath.Abs(filepath.Join("..", "..", "..", "sdk"))
	require.NoError(t, err)
	e.RunCommand("go", "mod", "edit", "-replace",
		"github.com/pulumi/pulumi/sdk/v3="+sdkDir)
	e.RunCommand("go", "mod", "tidy")

	e.RunCommand("pulumi", "stack", "init", "dev")
	e.RunCommand("pulumi", "up", "--yes",
		"--otel-traces", "file://"+tracesFile)

	// Find and decrypt all log files.
	logsDir := filepath.Join(e.HomePath, "logs")
	entries, err := os.ReadDir(logsDir)
	require.NoError(t, err)

	var allDecrypted strings.Builder
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		logFile := filepath.Join(logsDir, entry.Name())

		raw, err := os.ReadFile(logFile)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), "plugin-log-test-marker",
			"expected encrypted log %s to not contain plaintext marker", entry.Name())

		stdout, _ := e.RunCommand("pulumi", "logs", "decrypt", logFile)
		allDecrypted.WriteString(stdout)
	}

	decrypted := allDecrypted.String()
	timeRe := regexp.MustCompile(`,"time":"[^"]*"`)

	var found bool
	for _, line := range strings.Split(decrypted, "\n") {
		if !strings.Contains(line, "plugin-log-test-marker") {
			continue
		}
		found = true
		stripped := timeRe.ReplaceAllString(line, "")
		assert.Equal(t,
			`{"level":"INFO","msg":"plugin-log-test-marker: creating resource with inputs map[value:hello-from-plugin]"}`,
			stripped)
		break
	}
	assert.True(t, found, "expected to find a log line with plugin-log-test-marker in decrypted output")
}

const goProgram = `package main

import (
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := NewResource(ctx, "myResource", &ResourceArgs{
			Value: pulumi.String("hello-from-plugin"),
		})
		return err
	})
}

type Resource struct {
	pulumi.CustomResourceState
	Value pulumi.StringOutput ` + "`pulumi:\"value\"`" + `
}

func NewResource(ctx *pulumi.Context, name string, args *ResourceArgs, opts ...pulumi.ResourceOption) (*Resource, error) {
	var resource Resource
	err := ctx.RegisterResource("testlogging:index:Resource", name, args, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

type resourceArgs struct {
	Value string ` + "`pulumi:\"value\"`" + `
}

type ResourceArgs struct {
	Value pulumi.StringInput
}

func (ResourceArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*resourceArgs)(nil)).Elem()
}
`

const goMod = `module test-plugin-logging

go 1.25.0

require github.com/pulumi/pulumi/sdk/v3 v3.156.0
`
