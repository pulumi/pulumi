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

package constrictor

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtensionArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// args is the raw command line after the subcommand, parsed through a real
		// flag set so --extension behaves exactly as in the CLI.
		args          []string
		wantParams    []string
		wantExtension bool
		wantErr       string
	}{
		{
			name:       "replacement passes every token after the source",
			args:       []string{"example.com/base", "p1", "p2"},
			wantParams: []string{"p1", "p2"},
		},
		{
			name:       "replacement with no parameters",
			args:       []string{"example.com/base"},
			wantParams: []string{},
		},
		{
			name:       "replacement ignores a dash separator",
			args:       []string{"example.com/base", "--", "p1"},
			wantParams: []string{"p1"},
		},
		{
			name:          "extension splits its value like a shell line",
			args:          []string{"--extension", "-c crd.yaml -n gateway", "example.com/base"},
			wantParams:    []string{"-c", "crd.yaml", "-n", "gateway"},
			wantExtension: true,
		},
		{
			name:          "extension honors shell quoting",
			args:          []string{"--extension", `-f "multi word value"`, "example.com/base"},
			wantParams:    []string{"-f", "multi word value"},
			wantExtension: true,
		},
		{
			name:          "extension with no parameters",
			args:          []string{"--extension", "", "example.com/base"},
			wantParams:    []string{},
			wantExtension: true,
		},
		{
			name:    "combining replacement and extension is rejected",
			args:    []string{"--extension", "-c crd.yaml", "example.com/base", "p1"},
			wantErr: "combining replacement parameters with --extension is not supported yet",
		},
		{
			name:    "unbalanced quotes are a parse error",
			args:    []string{"--extension", `-f "unterminated`, "example.com/base"},
			wantErr: "parse --extension parameters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := &cobra.Command{Use: "test"}
			AddExtensionFlag(cmd)
			require.NoError(t, cmd.Flags().Parse(tt.args))

			got, asExtension, err := ExtensionArgs(cmd, cmd.Flags().Args())
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantParams, got)
			assert.Equal(t, tt.wantExtension, asExtension)
		})
	}
}
