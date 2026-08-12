// Copyright 2025, Pulumi Corporation.
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

package plugin

import (
	"testing"

	envutil "github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
	"github.com/stretchr/testify/require"
)

func TestConstructEnv(t *testing.T) {
	t.Parallel()

	opts := &PolicyAnalyzerOptions{
		Organization: "test-org",
		Project:      "test-project",
		Stack:        "test-stack",
		DryRun:       true,
	}

	env := envutil.NewEnv(envutil.MapStore{})
	result, err := constructEnv(env, opts, "python")
	require.NoError(t, err)

	// Standard vars should still be set.
	val, found := result.GetStore().Raw("PULUMI_DRY_RUN")
	require.True(t, found)
	require.Equal(t, "true", val)

	// Node.js-specific vars should not be set for python runtime.
	_, found = result.GetStore().Raw("PULUMI_NODEJS_ORGANIZATION")
	require.False(t, found)
}
