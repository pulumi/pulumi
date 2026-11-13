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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func TestProviderServerSendsCheckConfigFailureHint(t *testing.T) {
	t.Parallel()

	server := NewProviderServer(&MockProvider{
		CheckConfig2F: func(context.Context, CheckConfigRequest) (CheckConfig2Response, error) {
			return CheckConfig2Response{Failures: []CheckConfigFailure{
				{Reason: "no credentials", Hint: HintDynamicCredentials, HintMessage: "use ESC"},
			}}, nil
		},
	})
	resp, err := server.CheckConfig2(t.Context(), &pulumirpc.CheckConfigRequest{
		Urn: "urn:pulumi:stack::proj::pulumi:providers:aws::default",
	})
	require.NoError(t, err)
	require.Len(t, resp.GetFailures(), 1)
	assert.Equal(t, pulumirpc.Hint_HINT_DYNAMIC_CREDENTIALS, resp.GetFailures()[0].GetHint())
	assert.Equal(t, "use ESC", resp.GetFailures()[0].GetHintMessage())
}

func TestHintRoundTrip(t *testing.T) {
	t.Parallel()

	for _, h := range []Hint{HintUnspecified, HintDynamicCredentials} {
		assert.Equal(t, h, hintFromRPC(h.rpc()))
	}
	assert.Equal(t, HintUnspecified, hintFromRPC(pulumirpc.Hint(99)))
}
