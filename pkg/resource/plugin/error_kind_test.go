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
	seen map[tokens.Package]ErrorKind
}

func (r *testRecorder) RecordErrorKind(pkg tokens.Package, kind ErrorKind) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.seen == nil {
		r.seen = map[tokens.Package]ErrorKind{}
	}
	r.seen[pkg] = kind
}

func TestErrorKindOf(t *testing.T) {
	t.Parallel()

	credsErr := WithErrorKind(rpcerror.New(codes.Unauthenticated, "expired token"), ErrorKindCredentials)

	tests := []struct {
		name string
		err  error
		want ErrorKind
	}{
		{"nil", nil, ErrorKindUnspecified},
		{"plain error", errors.New("boom"), ErrorKindUnspecified},
		{"status without detail", rpcerror.New(codes.Unauthenticated, "boom"), ErrorKindUnspecified},
		{"status with detail", credsErr, ErrorKindCredentials},
		{"converted and wrapped", fmt.Errorf("configure: %w", rpcerror.Convert(credsErr)), ErrorKindCredentials},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ErrorKindOf(tt.err))
		})
	}
}

func TestProviderRecordsCredentialsCheckConfigFailure(t *testing.T) {
	t.Parallel()

	client := &stubClient{
		CheckConfigF: func(*pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
			return &pulumirpc.CheckResponse{Failures: []*pulumirpc.CheckFailure{
				{Reason: "unrelated"},
				{Reason: "no credentials", Kind: pulumirpc.ErrorKind_ERROR_KIND_CREDENTIALS},
			}}, nil
		},
	}
	p := NewProviderWithClient(newTestContext(t), client, false /* disablePreview */)

	rec := &testRecorder{}
	resp, err := p.CheckConfig(WithErrorKindRecorder(t.Context(), rec), CheckConfigRequest{
		URN:  resource.NewURN("stack", "proj", "", "pulumi:providers:aws", "default"),
		News: property.Map{},
	})
	require.NoError(t, err)
	require.Len(t, resp.Failures, 2)
	assert.Equal(t, ErrorKindUnspecified, resp.Failures[0].Kind)
	assert.Equal(t, ErrorKindCredentials, resp.Failures[1].Kind)
	assert.Equal(t, map[tokens.Package]ErrorKind{"aws": ErrorKindCredentials}, rec.seen)
}

func TestProviderRecordsCredentialsConfigureErrorViaPluginContext(t *testing.T) {
	t.Parallel()

	// The engine calls providers with detached contexts, so the recorder is found through the plugin context.
	rec := &testRecorder{}
	cwd, err := os.Getwd()
	require.NoError(t, err)
	sink := diagtest.LogSink(t)
	pctx, err := NewContext(WithErrorKindRecorder(t.Context(), rec), sink, sink,
		&MockHost{}, nil /* source */, cwd, nil /* options */, false, nil, nil /* span */)
	require.NoError(t, err)

	client := &stubClient{
		ConfigureF: func(*pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
			return nil, WithErrorKind(rpcerror.New(codes.Unauthenticated, "expired token"), ErrorKindCredentials)
		},
	}
	p := NewProviderWithClient(pctx, client, false /* disablePreview */)

	_, err = p.Configure(t.Context(), ConfigureRequest{Type: new(tokens.Type("pulumi:providers:gcp"))})
	require.Error(t, err)
	assert.Equal(t, ErrorKindCredentials, ErrorKindOf(err))
	assert.Equal(t, map[tokens.Package]ErrorKind{"gcp": ErrorKindCredentials}, rec.seen)
}

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
