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
		// flag set so --extension and the `--` separator behave exactly as in the CLI.
		args    []string
		want    []string
		wantErr string
	}{
		{
			name: "replacement passes every token after the source",
			args: []string{"example.com/base", "p1", "p2"},
			want: []string{"p1", "p2"},
		},
		{
			name: "replacement with no parameters",
			args: []string{"example.com/base"},
			want: []string{},
		},
		{
			name: "replacement ignores a dash separator",
			args: []string{"example.com/base", "--", "p1"},
			want: []string{"p1"},
		},
		{
			name: "extension after dash",
			args: []string{"--extension", "example.com/base", "--", "e1", "e2"},
			want: []string{"e1", "e2"},
		},
		{
			name: "extension with no parameters",
			args: []string{"--extension", "example.com/base"},
			want: []string{},
		},
		{
			name:    "parameters without a dash are rejected",
			args:    []string{"--extension", "example.com/base", "e1", "e2"},
			wantErr: "with --extension, parameters must come after '--'",
		},
		{
			name:    "parameters before the dash are rejected",
			args:    []string{"--extension", "example.com/base", "b1", "--", "e1"},
			wantErr: "with --extension, parameters must come after '--'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := &cobra.Command{Use: "test"}
			var asExtension bool
			AddExtensionFlag(cmd, &asExtension)
			require.NoError(t, cmd.Flags().Parse(tt.args))

			got, err := ExtensionArgs(cmd, cmd.Flags().Args(), asExtension)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
