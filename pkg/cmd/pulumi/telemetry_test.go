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

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/telemetry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
)

func emittedTelemetry(t *testing.T, ctx context.Context, info []telemetry.KV) ([]byte, map[string]any) {
	home := t.TempDir()
	ctx = telemetry.Init(ctx, telemetry.InitArgs{
		HomeDir: home,
		Diags:   diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never}),
		Env:     env.NewEnv(env.MapStore{"PULUMI_TELEMETRY": "0", "PULUMI_TELEMETRY_LOG": "1"}),
	})
	telemetry.Emit(ctx, info...)

	line, err := os.ReadFile(filepath.Join(home, "telemetry", "log", "events.jsonl"))
	require.NoError(t, err)
	var entry struct {
		Event struct{ Facts map[string]any }
	}
	require.NoError(t, json.Unmarshal(line, &entry))
	return line, entry.Event.Facts
}

func TestInvocationTelemetry(t *testing.T) {
	t.Parallel()

	root, cleanup := NewPulumiCmd()
	defer cleanup()
	c, args, err := root.Find([]string{
		"stack", "init", "acme-corp/website/prod", "--secrets-provider", "awskms://alias/acme", "--non-interactive",
	})
	require.NoError(t, err)
	require.NoError(t, c.ParseFlags(args))

	line, facts := emittedTelemetry(t, t.Context(), invocationTelemetry(c))

	require.ElementsMatch(t, []string{
		"command", "flags", "env-vars", "ci", "ci-system", "interactive", "automation-api", "using-ai", "os", "arch",
	}, slices.Collect(maps.Keys(facts)))
	require.Equal(t, "pulumi:stack:new", facts["command"])
	require.Equal(t, []any{"non-interactive", "secrets-provider"}, facts["flags"])
	require.NotContains(t, string(line), "acme")
}

func TestExitTelemetry(t *testing.T) {
	t.Parallel()

	err := errors.New("boom")
	ctx := cmdutil.ContextWithProcessStartTime(t.Context(), time.Now().Add(-time.Second))

	_, facts := emittedTelemetry(t, ctx, exitTelemetry(ctx, err))

	require.Equal(t, float64(cmd.ExitCodeFor(err)), facts["exit-code"])
	require.GreaterOrEqual(t, facts["duration-ms"], float64(1000))

	_, facts = emittedTelemetry(t, t.Context(), exitTelemetry(t.Context(), nil))
	require.Equal(t, map[string]any{"exit-code": float64(0)}, facts)
}

//nolint:paralleltest // NewPulumiCmd binds flags to process-wide variables
func TestTelemetrySchema(t *testing.T) {
	root, cleanup := NewPulumiCmd()
	defer cleanup()
	root.InitDefaultHelpCmd()

	var names []string
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		names = append(names, telemetry.CommandName(c))
		c.Flags().VisitAll(func(f *pflag.Flag) {
			require.NoError(t, telemetry.ValidateFact("flags", []string{f.Name}), telemetry.CommandName(c))
		})
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
	slices.Sort(names)
	for _, v := range env.Variables() {
		if strings.HasPrefix(v.Name(), env.Prefix) {
			require.NoError(t, telemetry.ValidateFact("env-vars", []string{v.Name()}))
		}
	}

	schema, err := telemetry.SchemaJSON(names)
	require.NoError(t, err)

	path := filepath.Join("..", "..", "telemetry", "schema.json")
	if cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT")) {
		require.NoError(t, os.WriteFile(path, schema, 0o600))
		return
	}
	existing, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, string(schema), string(existing),
		"pkg/telemetry/schema.json is out of date, regenerate it with "+
			"`PULUMI_ACCEPT=true go test ./cmd/pulumi -run TestTelemetrySchema`")
}

func TestUnknownCommand(t *testing.T) {
	t.Parallel()

	_, facts := emittedTelemetry(t, t.Context(), []telemetry.KV{telemetry.Command(&cobra.Command{Use: "my-plugin"})})
	require.Equal(t, "other", facts["command"])
}
