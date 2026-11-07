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
	"errors"
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var signIn = &pulumirpc.CommandRemediation{
	Description: "Sign in to AWS",
	Command:     []string{"aws", "login", "--region", "us-west-2"},
	Env:         map[string]string{"AWS_CLI_AGENT_TOOLKIT_HINT_DISABLED": "true"},
	Interactive: true,
}

func TestCheckFailureRemediationsRoundTrip(t *testing.T) {
	t.Parallel()

	rpc := &pulumirpc.CheckFailure{
		Property: "region",
		Reason:   "No valid credential sources found.",
		Remediations: []*pulumirpc.CommandRemediation{
			signIn,
			// Remediations without a command are dropped.
			{Description: "Do nothing"},
			{Command: []string{""}},
		},
	}
	failure := checkFailureFromRPC(rpc)
	assert.Equal(t, CheckFailure{
		Property: "region",
		Reason:   "No valid credential sources found.",
		Remediations: []CommandRemediation{{
			Description: "Sign in to AWS",
			Command:     []string{"aws", "login", "--region", "us-west-2"},
			Env:         map[string]string{"AWS_CLI_AGENT_TOOLKIT_HINT_DISABLED": "true"},
			Interactive: true,
		}},
	}, failure)

	back := checkFailureToRPC(failure)
	assert.Equal(t, "region", back.GetProperty())
	assert.Equal(t, rpc.GetReason(), back.GetReason())
	require.Len(t, back.GetRemediations(), 1)
	assert.Equal(t, signIn.GetCommand(), back.GetRemediations()[0].GetCommand())
	assert.Equal(t, signIn.GetEnv(), back.GetRemediations()[0].GetEnv())
	assert.True(t, back.GetRemediations()[0].GetInteractive())
}

func TestCheckFailureWithoutRemediations(t *testing.T) {
	t.Parallel()

	failure := checkFailureFromRPC(&pulumirpc.CheckFailure{Property: "region", Reason: "missing"})
	assert.Equal(t, CheckFailure{Property: "region", Reason: "missing"}, failure)
	assert.Empty(t, checkFailureToRPC(failure).GetRemediations())
}

func TestConfigureErrorRemediations(t *testing.T) {
	t.Parallel()

	st, err := status.New(codes.Unknown, "The AWS SSO session has expired.").WithDetails(signIn)
	require.NoError(t, err)
	want := []CommandRemediation{{
		Description: "Sign in to AWS",
		Command:     []string{"aws", "login", "--region", "us-west-2"},
		Env:         map[string]string{"AWS_CLI_AGENT_TOOLKIT_HINT_DISABLED": "true"},
		Interactive: true,
	}}

	// As the provider client returns it from Configure.
	converted := createConfigureError(rpcerror.Convert(st.Err()))
	assert.Equal(t, want, ConfigureErrorRemediations(converted))
	assert.Equal(t, want, ConfigureErrorRemediations(fmt.Errorf("configure: %w", converted)))
	// As a raw gRPC status error.
	assert.Equal(t, want, ConfigureErrorRemediations(st.Err()))

	assert.Empty(t, ConfigureErrorRemediations(status.Error(codes.Unknown, "no details")))
	assert.Empty(t, ConfigureErrorRemediations(errors.New("not an RPC error")))
	assert.Empty(t, ConfigureErrorRemediations(nil))
}
