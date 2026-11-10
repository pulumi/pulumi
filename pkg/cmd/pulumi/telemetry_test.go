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
	"testing"
	"time"

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
