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
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	commonenv "github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/ciutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
)

func emitted(t *testing.T, info ...KV) map[string]any {
	home := t.TempDir()
	ctx := Init(t.Context(), InitArgs{
		HomeDir: home,
		Diags:   diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never}),
		Env:     env.NewEnv(env.MapStore{"PULUMI_TELEMETRY": "0", "PULUMI_TELEMETRY_LOG": "1"}),
	})
	Emit(ctx, info...)

	data, err := os.ReadFile(filepath.Join(home, "telemetry", "log", "events.jsonl"))
	require.NoError(t, err)
	var entry struct {
		Event struct{ Facts map[string]any }
	}
	require.NoError(t, json.Unmarshal(data, &entry))
	return entry.Event.Facts
}

func TestCommandAndFlags(t *testing.T) {
	t.Parallel()

	root := &cobra.Command{Use: "pulumi"}
	root.PersistentFlags().Bool("non-interactive", false, "")
	stack := &cobra.Command{Use: "stack"}
	initCmd := &cobra.Command{Use: "new <stack-name>"}
	initCmd.Flags().String("secrets-provider", "", "")
	initCmd.Flags().Bool("no-select", false, "")
	root.AddCommand(stack)
	stack.AddCommand(initCmd)
	require.NoError(t, initCmd.ParseFlags([]string{
		"my-org/my-stack", "--secrets-provider", "awskms://alias/secret", "--non-interactive",
	}))

	facts := emitted(t, Command(initCmd), Flags(initCmd))

	require.Equal(t, map[string]any{
		"command": "pulumi:stack:new",
		"flags":   []any{"non-interactive", "secrets-provider"},
	}, facts)
}

func TestEnvVars(t *testing.T) {
	t.Parallel()

	facts := emitted(t, EnvVars(env.MapStore{
		commonenv.SkipUpdateCheck.Var().Name(): "1",
		commonenv.DoNotTrack.Var().Name():      "1",
		"PULUMI_MY_SECRET_PROJECT":             "1",
		"AWS_SECRET_ACCESS_KEY":                "secret",
	}))

	require.Equal(t, map[string]any{"env-vars": []any{"PULUMI_SKIP_UPDATE_CHECK"}}, facts)
}

func TestCISystem(t *testing.T) {
	t.Parallel()

	require.Equal(t, "github-actions", emitted(t, CISystem(ciutil.GitHubActions))["ci-system"])
	require.Equal(t, "gitlab", emitted(t, CISystem(ciutil.GitLab))["ci-system"])
	require.Equal(t, "none", emitted(t, CISystem(""))["ci-system"])
	require.Equal(t, "other", emitted(t, CISystem("My Build Server"))["ci-system"])
}

func TestUsingAI(t *testing.T) {
	t.Parallel()

	getenv := func(vars map[string]string) func(string) string {
		return func(key string) string { return vars[key] }
	}

	require.Equal(t, "claude", emitted(t, UsingAI(getenv(map[string]string{"CLAUDECODE": "1"})))["using-ai"])
	require.Equal(t, "claude",
		emitted(t, UsingAI(getenv(map[string]string{"AI_AGENT": "claude-code_2-1-285_agent"})))["using-ai"])
	require.Equal(t, "codex", emitted(t, UsingAI(getenv(map[string]string{"AI_AGENT": "codex_1-0_agent"})))["using-ai"])
	require.Equal(t, "other", emitted(t, UsingAI(getenv(map[string]string{"AI_AGENT": "my-agent"})))["using-ai"])
	require.Equal(t, "no", emitted(t, UsingAI(getenv(nil)))["using-ai"])
}

func TestBackend(t *testing.T) {
	t.Parallel()

	for url, expected := range map[string]string{
		"https://api.pulumi.com":          "pulumi",
		"https://pulumi.example.com":      "pulumi",
		"file:///home/me/.pulumi":         "file",
		"s3://my-bucket?region=us-east-1": "s3",
		"azblob://my-container":           "azblob",
		"gs://my-bucket":                  "gs",
	} {
		require.Equal(t, expected, emitted(t, backendFact(url))["backend"], url)
	}
}

func TestResult(t *testing.T) {
	t.Parallel()

	facts := emitted(t, ExitCode(255), Duration(1500*time.Millisecond), Interactive(true), AutomationAPI(false),
		CI(true), OS(), Arch())

	require.Equal(t, map[string]any{
		"exit-code":      float64(255),
		"duration-ms":    float64(1500),
		"interactive":    true,
		"automation-api": false,
		"ci":             true,
		"os":             runtime.GOOS,
		"arch":           runtime.GOARCH,
	}, facts)
}
