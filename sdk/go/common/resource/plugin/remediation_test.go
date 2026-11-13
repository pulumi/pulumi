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

package plugin

import (
	"testing"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckConfigFailureRemediationsRoundTrip(t *testing.T) {
	t.Parallel()

	signIn := &pulumirpc.CommandRemediation{
		Description: "Sign in to AWS",
		Command:     []string{"aws", "login", "--region", "us-west-2"},
		Env:         map[string]string{"AWS_CLI_AGENT_TOOLKIT_HINT_DISABLED": "true"},
	}
	rpc := &pulumirpc.CheckConfigFailure{
		Property: "region",
		Reason:   "No valid credential sources found.",
		Remediations: []*pulumirpc.CommandRemediation{
			signIn,
			// Remediations without a command are dropped.
			{Description: "Do nothing"},
			{Command: []string{""}},
		},
	}
	failure := checkConfigFailureFromRPC(rpc)
	assert.Equal(t, CheckConfigFailure{
		Property: "region",
		Reason:   "No valid credential sources found.",
		Remediations: []CommandRemediation{{
			Description: "Sign in to AWS",
			Command:     []string{"aws", "login", "--region", "us-west-2"},
			Env:         map[string]string{"AWS_CLI_AGENT_TOOLKIT_HINT_DISABLED": "true"},
		}},
	}, failure)

	back := checkConfigFailureToRPC(failure)
	assert.Equal(t, "region", back.GetProperty())
	assert.Equal(t, rpc.GetReason(), back.GetReason())
	require.Len(t, back.GetRemediations(), 1)
	assert.Equal(t, signIn.GetDescription(), back.GetRemediations()[0].GetDescription())
	assert.Equal(t, signIn.GetCommand(), back.GetRemediations()[0].GetCommand())
	assert.Equal(t, signIn.GetEnv(), back.GetRemediations()[0].GetEnv())
}

func TestCheckConfigFailureWithoutRemediations(t *testing.T) {
	t.Parallel()

	failure := checkConfigFailureFromRPC(&pulumirpc.CheckConfigFailure{Property: "region", Reason: "missing"})
	assert.Equal(t, CheckConfigFailure{Property: "region", Reason: "missing"}, failure)
	assert.Empty(t, checkConfigFailureToRPC(failure).GetRemediations())
}
