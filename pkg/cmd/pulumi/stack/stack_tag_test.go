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

package stack

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The sandbox stack tag can only be set when a stack is created, so it can't be edited to turn a stack that deploys to
// real clouds into a local one or the other way around.
func TestStackTagLocalIsReadOnly(t *testing.T) {
	t.Parallel()

	var stack string
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"set", func() error {
			cmd := newStackTagSetCmd(&stack)
			return cmd.RunE(cmd, []string{"pulumi:sandbox", "false"})
		}},
		{"rm", func() error {
			cmd := newStackTagRemoveCmd(&stack)
			return cmd.RunE(cmd, []string{"pulumi:sandbox"})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorIs(t, tc.run(), errSandboxStackTagReadOnly)
		})
	}
}
