// Copyright 2016-2025, Pulumi Corporation.
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

package deploy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func TestRetryLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("success should not retry", func(t *testing.T) {
		t.Parallel()
		testRetries(t, &retryTestConfig{
			registrationResponses: []ResultState{ResultStateSuccess},
			retryPatterns:         []bool{},
			totalRetries:          1,
		})
	})

	t.Run("skip should not retry", func(t *testing.T) {
		t.Parallel()
		testRetries(t, &retryTestConfig{
			registrationResponses: []ResultState{ResultStateSkipped},
			retryPatterns:         []bool{},
			totalRetries:          1,
		})
	})

	t.Run("failure with retry=false should not retry", func(t *testing.T) {
		t.Parallel()
		testRetries(t, &retryTestConfig{
			registrationResponses: []ResultState{ResultStateFailed},
			retryPatterns:         []bool{false},
			totalRetries:          1,
		})
	})

	t.Run("failure with retry=true then false should retry once", func(t *testing.T) {
		t.Parallel()
		testRetries(t, &retryTestConfig{
			registrationResponses: []ResultState{ResultStateFailed, ResultStateFailed},
			retryPatterns:         []bool{true, false},
			totalRetries:          2,
		})
	})

	t.Run("always fail with always retry=true should hit 100 limit", func(t *testing.T) {
		t.Parallel()
		registrationResponses := make([]ResultState, 150)
		retryPatterns := make([]bool, 150)

		for i := range 150 {
			registrationResponses[i] = ResultStateFailed
			retryPatterns[i] = true
		}

		testRetries(t, &retryTestConfig{
			registrationResponses: registrationResponses,
			retryPatterns:         retryPatterns,
			totalRetries:          100,
		})
	})
}

type retryTestConfig struct {
	registrationResponses []ResultState
	retryPatterns         []bool
	totalRetries          int
}

func testRetries(t *testing.T, config *retryTestConfig) {
	callbackServer, err := deploytest.NewCallbacksServer()
	require.NoError(t, err)
	defer callbackServer.Close()

	var retries int

	retryCallback, err := callbackServer.Allocate(func(req []byte) (proto.Message, error) {
		var retryReq pulumirpc.RetryRequest
		err := proto.Unmarshal(req, &retryReq)
		if err != nil {
			return nil, err
		}

		retries++

		shouldRetry := false
		if retries <= len(config.retryPatterns) {
			shouldRetry = config.retryPatterns[retries-1]
		}

		return &pulumirpc.RetryResponse{
			ShouldRetry: shouldRetry,
		}, nil
	})
	require.NoError(t, err)

	plugctx, err := plugin.NewContext(context.Background(),
		&deploytest.NoopSink{}, &deploytest.NoopSink{},
		deploytest.NewPluginHostF(nil, nil, nil)(),
		nil, "", nil, false, nil)
	require.NoError(t, err)

	regChan := make(chan *registerResourceEvent, 150)

	mon, err := newResourceMonitor(&evalSource{
		runinfo: &EvalRunInfo{
			ProjectRoot: "/",
			Pwd:         "/",
			Program:     ".",
			Proj:        &workspace.Project{Name: "proj"},
			Target: &Target{
				Name: tokens.MustParseStackName("stack"),
			},
		},
		plugctx: plugctx,
	}, &providerSourceMock{
		Provider: &deploytest.Provider{},
	}, regChan, nil, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)

	mon.abortChan = make(chan bool)
	mon.cancel = make(chan bool)

	var attempts int

	go func() {
		for {
			select {
			case <-mon.cancel:
				return
			case evt := <-regChan:
				resultState := ResultStateSuccess

				// Providers etc always succeed, we only care about our test resource
				if evt.goal.Type == tokens.Type("test:index:Resource") && evt.goal.Name == "test-resource" {
					resultState = config.registrationResponses[attempts]
					attempts++
				}

				urn := resource.URN("urn:pulumi:stack::project::" + string(evt.goal.Type) + "::" + evt.goal.Name)
				id := resource.ID("id-" + evt.goal.Name)

				state := &resource.State{
					Type: evt.goal.Type,
					URN:  urn,
					ID:   id,
				}

				if resultState == ResultStateFailed {
					state.InitErrors = []string{"oh dear"}
				}

				evt.done <- &RegisterResult{
					State:  state,
					Result: resultState,
				}
			}
		}
	}()

	_, err = mon.RegisterResource(context.Background(), &pulumirpc.RegisterResourceRequest{
		Type:                    "test:index:Resource",
		Name:                    "test-resource",
		Custom:                  true,
		Object:                  &structpb.Struct{},
		SupportsResultReporting: true,
		RetryWith:               retryCallback,
	})

	require.NoError(t, err)
	close(mon.cancel)

	assert.Equal(t, config.totalRetries, attempts)
}
