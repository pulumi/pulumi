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

	"github.com/stretchr/testify/require"
)

func TestConstructEnvWithAdditionalEnv(t *testing.T) {
	t.Parallel()

	opts := &PolicyAnalyzerOptions{
		Organization: "test-org",
		Project:      "test-project",
		Stack:        "test-stack",
		DryRun:       false,
		AdditionalEnv: map[string]string{
			"MY_SECRET":  "secret-value",
			"AWS_REGION": "us-west-2",
		},
	}

	result, err := constructEnv(opts, "nodejs")
	require.NoError(t, err)

	// Verify standard env vars are set.
	val, found := result.GetStore().Raw("PULUMI_ORGANIZATION")
	require.True(t, found)
	require.Equal(t, "test-org", val)

	// Verify AdditionalEnv vars are injected.
	val, found = result.GetStore().Raw("MY_SECRET")
	require.True(t, found)
	require.Equal(t, "secret-value", val)

	val, found = result.GetStore().Raw("AWS_REGION")
	require.True(t, found)
	require.Equal(t, "us-west-2", val)
}

func TestConstructEnvWithoutAdditionalEnv(t *testing.T) {
	t.Parallel()

	opts := &PolicyAnalyzerOptions{
		Organization: "test-org",
		Project:      "test-project",
		Stack:        "test-stack",
		DryRun:       true,
	}

	result, err := constructEnv(opts, "python")
	require.NoError(t, err)

	// Standard vars should still be set.
	val, found := result.GetStore().Raw("PULUMI_DRY_RUN")
	require.True(t, found)
	require.Equal(t, "true", val)

	// Node.js-specific vars should not be set for python runtime.
	_, found = result.GetStore().Raw("PULUMI_NODEJS_ORGANIZATION")
	require.False(t, found)
}
