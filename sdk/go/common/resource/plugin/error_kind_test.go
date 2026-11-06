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

func TestProviderServerSendsCheckFailureKind(t *testing.T) {
	t.Parallel()

	server := NewProviderServer(&MockProvider{
		CheckConfigF: func(context.Context, CheckConfigRequest) (CheckConfigResponse, error) {
			return CheckConfigResponse{Failures: []CheckFailure{{Reason: "no credentials", Kind: ErrorKindCredentials}}}, nil
		},
	})
	resp, err := server.CheckConfig(t.Context(), &pulumirpc.CheckRequest{
		Urn: "urn:pulumi:stack::proj::pulumi:providers:aws::default",
	})
	require.NoError(t, err)
	require.Len(t, resp.GetFailures(), 1)
	assert.Equal(t, pulumirpc.ErrorKind_ERROR_KIND_CREDENTIALS, resp.GetFailures()[0].GetKind())
}

func TestErrorKindRoundTrip(t *testing.T) {
	t.Parallel()

	for _, k := range []ErrorKind{ErrorKindUnspecified, ErrorKindCredentials} {
		assert.Equal(t, k, errorKindFromRPC(k.rpc()))
	}
	assert.Equal(t, ErrorKindUnspecified, errorKindFromRPC(pulumirpc.ErrorKind(99)))
}
