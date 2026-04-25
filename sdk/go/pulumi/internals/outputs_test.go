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

package internals

import (
	"errors"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func await(t *testing.T, out pulumi.Output) (any, bool, bool, []pulumi.Resource, error) {
	result, err := UnsafeAwaitOutput(t.Context(), out)

	return result.Value, result.Known, result.Secret, result.Dependencies, err
}

func TestBasicOutputs(t *testing.T) {
	t.Parallel()

	ctx, err := pulumi.NewContext(t.Context(), pulumi.RunInfo{
		Project: "proj",
		Stack:   "stack",
	})
	require.NoError(t, err)

	// Just test basic resolve and reject functionality.
	{
		out, resolve, _ := ctx.NewOutput()
		go func() {
			resolve(42)
		}()
		v, known, secret, deps, err := await(t, out)
		require.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		assert.Nil(t, deps)
		require.NotNil(t, v)
		assert.Equal(t, 42, v.(int))
	}
	{
		out, _, reject := ctx.NewOutput()
		go func() {
			reject(errors.New("boom"))
		}()
		v, _, _, _, err := await(t, out)
		assert.EqualError(t, err, "boom")
		assert.Nil(t, v)
	}
}
