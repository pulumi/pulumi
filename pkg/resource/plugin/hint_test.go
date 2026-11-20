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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type testRecorder struct {
	mu   sync.Mutex
	seen []Hint
}

func (r *testRecorder) RecordHint(hint Hint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen = append(r.seen, hint)
}

var escHint = Hint{Kind: HintDynamicCredentials, Message: "use ESC"}

// newHintTestContext returns a plugin context that reports hints to rec.
func newHintTestContext(t *testing.T, rec HintRecorder) *Context {
	pctx := newTestContext(t)
	pctx.HintRecorder = rec
	return pctx
}

func TestProviderRecordsCheckConfigFailureHint(t *testing.T) {
	t.Parallel()

	client := &stubClient{
		CheckConfig2F: func(*pulumirpc.CheckConfigRequest) (*pulumirpc.CheckConfigResponse, error) {
			return &pulumirpc.CheckConfigResponse{Failures: []*pulumirpc.CheckConfigFailure{
				{Reason: "unrelated"},
				{Reason: "no credentials", Hint: &pulumirpc.Hint{Kind: pulumirpc.Hint_DYNAMIC_CREDENTIALS, Message: "use ESC"}},
			}}, nil
		},
	}
	rec := &testRecorder{}
	p := NewProviderWithClient(newHintTestContext(t, rec), client, false /* disablePreview */)

	resp, err := p.CheckConfig(t.Context(), CheckConfigRequest{
		URN:  resource.NewURN("stack", "proj", "", "pulumi:providers:aws", "default"),
		News: property.Map{},
	})
	require.NoError(t, err)
	require.Len(t, resp.Failures, 2)
	assert.Nil(t, resp.Failures[0].Hint)
	assert.Equal(t, &Hint{Kind: HintDynamicCredentials, Message: "use ESC"}, resp.Failures[1].Hint)
	assert.Equal(t, []Hint{escHint}, rec.seen)
}

func TestProviderServerSendsCheckConfigFailureHint(t *testing.T) {
	t.Parallel()

	server := NewProviderServer(&MockProvider{
		CheckConfigF: func(context.Context, CheckConfigRequest) (CheckConfigResponse, error) {
			return CheckConfigResponse{Failures: []CheckConfigFailure{
				{Reason: "no credentials", Hint: &Hint{Kind: HintDynamicCredentials, Message: "use ESC"}},
			}}, nil
		},
	})
	resp, err := server.CheckConfig2(t.Context(), &pulumirpc.CheckConfigRequest{
		Urn: "urn:pulumi:stack::proj::pulumi:providers:aws::default",
	})
	require.NoError(t, err)
	require.Len(t, resp.GetFailures(), 1)
	assert.Equal(t, pulumirpc.Hint_DYNAMIC_CREDENTIALS, resp.GetFailures()[0].GetHint().GetKind())
	assert.Equal(t, "use ESC", resp.GetFailures()[0].GetHint().GetMessage())
}

func TestHintFromRPCIgnoresUnknownKinds(t *testing.T) {
	t.Parallel()

	// A provider built against a newer protocol may send a kind this engine doesn't know.
	h := hintFromRPC(&pulumirpc.Hint{Kind: pulumirpc.Hint_Kind(99), Message: "something new"})
	assert.Equal(t, &Hint{Kind: HintUnspecified, Message: "something new"}, h)
}

func TestHintRPCFailsOnUnknownKind(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() { (&Hint{Kind: HintKind(99)}).rpc() })
}
