// Copyright 2016, Pulumi Corporation.
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
	"bytes"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseRootPersistentFlags guards against a --help / -h token causing pflag to drop the root
// flags that follow it (e.g. --otel-traces), which left tracing silently disabled for `pulumi do`.
func TestParseRootPersistentFlags(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		args      []string
		wantOtel  string
		wantColor string
	}{
		{
			name:     "otel flag, no help",
			args:     []string{"random:index:RandomString", "create", "--otel-traces", "grpc://localhost:4317"},
			wantOtel: "grpc://localhost:4317",
		},
		{
			name:     "help before otel flag",
			args:     []string{"random:index:RandomString", "create", "--help", "--otel-traces", "grpc://localhost:4317"},
			wantOtel: "grpc://localhost:4317",
		},
		{
			name:     "short help before otel flag",
			args:     []string{"random:index:RandomString", "create", "-h", "--otel-traces", "grpc://localhost:4317"},
			wantOtel: "grpc://localhost:4317",
		},
		{
			name: "help between two root flags, with unknown provider flags",
			args: []string{
				"random:index:RandomString", "create", "--length", "8",
				"--help", "--otel-traces", "file:///tmp/t.json", "--color", "never",
			},
			wantOtel:  "file:///tmp/t.json",
			wantColor: "never",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			var otelTraces, color string
			root := pflag.NewFlagSet("pulumi", pflag.ContinueOnError)
			root.StringVar(&otelTraces, "otel-traces", "", "")
			root.StringVar(&color, "color", "", "")

			parseRootPersistentFlags(root, c.args)

			assert.Equal(t, c.wantOtel, otelTraces)
			assert.Equal(t, c.wantColor, color)
		})
	}
}

// A bare group command such as `pulumi env` should still print help, but exit
// non-zero since it did not do anything.
func TestGroupCommandsBareInvocationExitsNonZero(t *testing.T) {
	// A bare group invocation is runnable and so executes the root
	// PersistentPreRunE, which would otherwise fire a background network
	// update check. t.Setenv also rules out t.Parallel here.
	t.Setenv("PULUMI_SKIP_UPDATE_CHECK", "true")

	pulumiCmd, cleanup := NewPulumiCmd()
	defer cleanup()

	var stdout, stderr bytes.Buffer
	pulumiCmd.SetOut(&stdout)
	pulumiCmd.SetErr(&stderr)
	pulumiCmd.SetArgs([]string{"env"})

	err := pulumiCmd.Execute()
	require.Error(t, err)
	assert.True(t, result.IsBail(err), "expected a bail error so no message is printed after the help text")
	assert.Equal(t, cmd.ExitCodeError, cmd.ExitCodeFor(err))
	assert.Contains(t, stdout.String(), "Usage:", "help text should still be printed")
}

// Requesting help explicitly must keep exiting 0.
//
//nolint:paralleltest // NewPulumiCmd registers env vars in a process-wide registry
func TestGroupCommandsHelpFlagSucceeds(t *testing.T) {
	pulumiCmd, cleanup := NewPulumiCmd()
	defer cleanup()

	var stdout, stderr bytes.Buffer
	pulumiCmd.SetOut(&stdout)
	pulumiCmd.SetErr(&stderr)
	pulumiCmd.SetArgs([]string{"env", "--help"})

	err := pulumiCmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Usage:")
}
