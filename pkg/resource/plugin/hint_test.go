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
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type testRecorder struct {
	mu   sync.Mutex
	seen map[tokens.Package]Hint
}

func (r *testRecorder) RecordHint(pkg tokens.Package, hint Hint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.seen == nil {
		r.seen = map[tokens.Package]Hint{}
	}
	r.seen[pkg] = hint
}

func TestHintOf(t *testing.T) {
	t.Parallel()

	credsErr := WithHint(rpcerror.New(codes.Unauthenticated, "expired token"), HintDynamicCredentials)

	tests := []struct {
		name string
		err  error
		want Hint
	}{
		{"nil", nil, HintUnspecified},
		{"plain error", errors.New("boom"), HintUnspecified},
		{"status without detail", rpcerror.New(codes.Unauthenticated, "boom"), HintUnspecified},
		{"status with detail", credsErr, HintDynamicCredentials},
		{"converted and wrapped", fmt.Errorf("configure: %w", rpcerror.Convert(credsErr)), HintDynamicCredentials},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, HintOf(tt.err))
		})
	}
}

func TestProviderRecordsCheckConfigFailureHint(t *testing.T) {
	t.Parallel()

	client := &stubClient{
		CheckConfigF: func(*pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
			return &pulumirpc.CheckResponse{Failures: []*pulumirpc.CheckFailure{
				{Reason: "unrelated"},
				{Reason: "no credentials", Hint: pulumirpc.Hint_HINT_DYNAMIC_CREDENTIALS},
			}}, nil
		},
	}
	p := NewProviderWithClient(newTestContext(t), client, false /* disablePreview */)

	rec := &testRecorder{}
	resp, err := p.CheckConfig(WithHintRecorder(t.Context(), rec), CheckConfigRequest{
		URN:  resource.NewURN("stack", "proj", "", "pulumi:providers:aws", "default"),
		News: property.Map{},
	})
	require.NoError(t, err)
	require.Len(t, resp.Failures, 2)
	assert.Equal(t, HintUnspecified, resp.Failures[0].Hint)
	assert.Equal(t, HintDynamicCredentials, resp.Failures[1].Hint)
	assert.Equal(t, map[tokens.Package]Hint{"aws": HintDynamicCredentials}, rec.seen)
}

func TestProviderRecordsConfigureErrorHintViaPluginContext(t *testing.T) {
	t.Parallel()

	// The engine calls providers with detached contexts, so the recorder is found through the plugin context.
	rec := &testRecorder{}
	cwd, err := os.Getwd()
	require.NoError(t, err)
	sink := diagtest.LogSink(t)
	pctx, err := NewContext(WithHintRecorder(t.Context(), rec), sink, sink,
		&MockHost{}, nil /* source */, cwd, nil /* options */, false, nil, nil /* span */)
	require.NoError(t, err)

	client := &stubClient{
		ConfigureF: func(*pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
			return nil, WithHint(rpcerror.New(codes.Unauthenticated, "expired token"), HintDynamicCredentials)
		},
	}
	p := NewProviderWithClient(pctx, client, false /* disablePreview */)

	_, err = p.Configure(t.Context(), ConfigureRequest{Type: new(tokens.Type("pulumi:providers:gcp"))})
	require.Error(t, err)
	assert.Equal(t, HintDynamicCredentials, HintOf(err))
	assert.Equal(t, map[tokens.Package]Hint{"gcp": HintDynamicCredentials}, rec.seen)
}

func TestProviderServerSendsCheckFailureHint(t *testing.T) {
	t.Parallel()

	server := NewProviderServer(&MockProvider{
		CheckConfigF: func(context.Context, CheckConfigRequest) (CheckConfigResponse, error) {
			return CheckConfigResponse{Failures: []CheckFailure{{Reason: "no credentials", Hint: HintDynamicCredentials}}}, nil
		},
	})
	resp, err := server.CheckConfig(t.Context(), &pulumirpc.CheckRequest{
		Urn: "urn:pulumi:stack::proj::pulumi:providers:aws::default",
	})
	require.NoError(t, err)
	require.Len(t, resp.GetFailures(), 1)
	assert.Equal(t, pulumirpc.Hint_HINT_DYNAMIC_CREDENTIALS, resp.GetFailures()[0].GetHint())
}
