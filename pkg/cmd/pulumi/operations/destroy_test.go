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

package operations

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStackRemoveHint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		stackRef string
		cwd      string
		expected string
	}{
		{
			name:     "no cwd flag",
			stackRef: "dev",
			expected: "pulumi stack rm dev",
		},
		{
			name:     "no cwd flag, qualified ref",
			stackRef: "acmecorp/website/dev",
			expected: "pulumi stack rm acmecorp/website/dev",
		},
		{
			name:     "relative cwd",
			stackRef: "dev",
			cwd:      ".e2e/dev",
			expected: "pulumi -C .e2e/dev stack rm dev",
		},
		{
			name:     "absolute cwd",
			stackRef: "dev",
			cwd:      "/home/user/infra",
			expected: "pulumi -C /home/user/infra stack rm dev",
		},
		{
			name:     "cwd with a space is quoted",
			stackRef: "dev",
			cwd:      "my infra",
			expected: "pulumi -C 'my infra' stack rm dev",
		},
		{
			name:     "cwd with a quote is escaped",
			stackRef: "dev",
			cwd:      "it's/infra",
			expected: `pulumi -C 'it'\''s/infra' stack rm dev`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, stackRemoveHint(tt.stackRef, tt.cwd))
		})
	}
}

func TestRootCwdFlag(t *testing.T) {
	t.Parallel()

	t.Run("unset", func(t *testing.T) {
		t.Parallel()
		root := &cobra.Command{Use: "pulumi"}
		root.PersistentFlags().StringP("cwd", "C", "", "")
		child := &cobra.Command{Use: "destroy"}
		root.AddCommand(child)

		assert.Equal(t, "", rootCwdFlag(child))
	})

	t.Run("set on root, read from child", func(t *testing.T) {
		t.Parallel()
		root := &cobra.Command{Use: "pulumi"}
		root.PersistentFlags().StringP("cwd", "C", "", "")
		child := &cobra.Command{Use: "destroy"}
		root.AddCommand(child)
		require.NoError(t, root.PersistentFlags().Set("cwd", ".e2e/dev"))

		assert.Equal(t, ".e2e/dev", rootCwdFlag(child))
	})

	t.Run("no cwd flag registered", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "", rootCwdFlag(&cobra.Command{Use: "destroy"}))
	})
}
