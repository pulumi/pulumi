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
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestWatchRemovalError(t *testing.T) {
	run := func() error {
		cmd := NewWatchCmd()
		cmd.SetArgs(nil)
		cmd.SetErr(io.Discard)
		cmd.SetOut(io.Discard)
		return cmd.Execute()
	}

	t.Run("default", func(t *testing.T) {
		t.Setenv(env.DisableExperimentalWatchRemovalError.Var().Name(), "false")
		assert.EqualError(t, run(), "command \"watch\" is disabled by default because it will be removed, "+
			"see https://github.com/pulumi/pulumi/issues/24717.\n"+
			"To run the command until it is removed, set PULUMI_DISABLE_EXPERIMENTAL_WATCH_REMOVAL_ERROR=1.")
	})

	t.Run("disabled", func(t *testing.T) {
		t.Chdir(t.TempDir())
		t.Setenv(env.DisableExperimentalWatchRemovalError.Var().Name(), "true")
		assert.ErrorIs(t, run(), workspace.ErrProjectNotFound)
	})
}
