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

package esc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyForEnvWithID(t *testing.T) {
	t.Parallel()

	t.Run("records the ID beside the name", func(t *testing.T) {
		t.Parallel()

		ec, err := NewExecContext(nil)
		require.NoError(t, err)

		copy := ec.CopyForEnvWithID("dev", "dev-uuid")
		assert.Equal(t, "dev", copy.GetCurrentEnvironmentName())
		assert.Equal(t, "dev-uuid", copy.GetCurrentEnvironmentID())
		assert.Equal(t, "dev", copy.GetRootEnvironmentName())
		assert.Equal(t, "dev-uuid", copy.GetRootEnvironmentID())

		expected := map[string]Value{
			"name": NewValue("dev"),
			"id":   NewValue("dev-uuid"),
		}
		assert.Equal(t, NewValue(expected), copy.Values()["currentEnvironment"])
		assert.Equal(t, NewValue(expected), copy.Values()["rootEnvironment"])
	})

	t.Run("keeps the root ID through nested copies", func(t *testing.T) {
		t.Parallel()

		ec, err := NewExecContext(nil)
		require.NoError(t, err)

		copy := ec.CopyForEnvWithID("root", "root-uuid").CopyForEnvWithID("child", "child-uuid")
		assert.Equal(t, "child", copy.GetCurrentEnvironmentName())
		assert.Equal(t, "child-uuid", copy.GetCurrentEnvironmentID())
		assert.Equal(t, "root", copy.GetRootEnvironmentName())
		assert.Equal(t, "root-uuid", copy.GetRootEnvironmentID())

		assert.Equal(t, NewValue(map[string]Value{
			"name": NewValue("child"),
			"id":   NewValue("child-uuid"),
		}), copy.Values()["currentEnvironment"])
		assert.Equal(t, NewValue(map[string]Value{
			"name": NewValue("root"),
			"id":   NewValue("root-uuid"),
		}), copy.Values()["rootEnvironment"])
	})

	t.Run("resolves an anonymous root to the first named environment", func(t *testing.T) {
		t.Parallel()

		ec, err := NewExecContext(nil)
		require.NoError(t, err)

		copy := ec.CopyForEnv(AnonymousEnvironmentName).CopyForEnvWithID("dev", "dev-uuid")
		assert.Equal(t, "dev", copy.GetRootEnvironmentName())
		assert.Equal(t, "dev-uuid", copy.GetRootEnvironmentID())
	})

	t.Run("an empty ID matches CopyForEnv", func(t *testing.T) {
		t.Parallel()

		ec, err := NewExecContext(nil)
		require.NoError(t, err)

		withID := ec.CopyForEnvWithID("dev", "")
		plain := ec.CopyForEnv("dev")
		assert.Equal(t, plain.Values(), withID.Values())
		assert.Equal(t, "", withID.GetCurrentEnvironmentID())
		assert.Equal(t, "", withID.GetRootEnvironmentID())

		// In particular, no `id` property is exposed for interpolation.
		current, ok := plain.Values()["currentEnvironment"].Value.(map[string]Value)
		require.True(t, ok)
		assert.NotContains(t, current, "id")
	})
}
