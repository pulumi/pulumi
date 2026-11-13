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

type recordedHint struct {
	hint    Hint
	message string
}

type testRecorder struct {
	mu   sync.Mutex
	seen []recordedHint
}

func (r *testRecorder) RecordHint(hint Hint, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen = append(r.seen, recordedHint{hint, message})
}

var escHint = recordedHint{HintDynamicCredentials, "use ESC"}

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
				{Reason: "no credentials", Hint: pulumirpc.Hint_HINT_DYNAMIC_CREDENTIALS, HintMessage: "use ESC"},
			}}, nil
		},
	}
	rec := &testRecorder{}
	p := NewProviderWithClient(newHintTestContext(t, rec), client, false /* disablePreview */)

	resp, err := CheckConfig2(t.Context(), p, CheckConfigRequest{
		URN:  resource.NewURN("stack", "proj", "", "pulumi:providers:aws", "default"),
		News: property.Map{},
	})
	require.NoError(t, err)
	require.Len(t, resp.Failures, 2)
	assert.Equal(t, HintUnspecified, resp.Failures[0].Hint)
	assert.Equal(t, HintDynamicCredentials, resp.Failures[1].Hint)
	assert.Equal(t, "use ESC", resp.Failures[1].HintMessage)
	assert.Equal(t, []recordedHint{escHint}, rec.seen)
}

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
