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

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
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
	plugctx, err := plugin.NewContext(context.Background(),
		&deploytest.NoopSink{}, &deploytest.NoopSink{},
		deploytest.NewPluginHostF(nil, nil, nil)(),
		nil, "", nil, false, nil)
	require.NoError(t, err)

	// Create ResourceHooks instance
	resourceHooks := NewResourceHooks(plugctx.DialOptions)

	// Track hook calls
	var hookCalls int

	// Register the error hook
	err = resourceHooks.RegisterErrorHook(ErrorHook{
		Name: "test-error-hook",
		Callback: func(ctx context.Context, urn resource.URN, id resource.ID,
			name string, typ tokens.Type, newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap,
			errorMessages []string,
		) (bool, error) {
			hookCalls++

			shouldRetry := false
			if hookCalls <= len(config.retryPatterns) {
				shouldRetry = config.retryPatterns[hookCalls-1]
			}

			return shouldRetry, nil
		},
		OnDryRun: false,
	})
	require.NoError(t, err)

	// Create a provider that can fail and track Create calls
	var attempts int
	testProvider := &deploytest.Provider{
		CreateF: func(ctx context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
			attempts++
			// Use modulo to handle cases where we have more attempts than responses
			idx := (attempts - 1) % len(config.registrationResponses)
			resultState := config.registrationResponses[idx]
			if resultState == ResultStateFailed {
				return plugin.CreateResponse{}, &plugin.InitError{
					Reasons: []string{"oh dear"},
				}
			}
			return plugin.CreateResponse{
				ID:         resource.ID("id-test-resource"),
				Properties: resource.PropertyMap{},
			}, nil
		},
	}

	// Create a deployment with the resource hooks
	deployment := &Deployment{
		opts: &Options{
			DryRun: false,
		},
		resourceHooks: resourceHooks,
		ctx:           plugctx,
	}

	// Create a mock register event
	regChan := make(chan *RegisterResult, 1)
	reg := &registerResourceEvent{
		done: regChan,
	}

	// Create the CreateStep with error hook configured
	urn := resource.URN("urn:pulumi:stack::project::test:index:Resource::test-resource")
	step := &CreateStep{
		deployment: deployment,
		reg:        reg,
		provider:   testProvider,
		new: &resource.State{
			URN:    urn,
			Type:   tokens.Type("test:index:Resource"),
			Custom: true,
			ResourceHooks: map[resource.HookType][]string{
				resource.OnCreateError: {"test-error-hook"},
			},
			Inputs:         resource.PropertyMap{},
			CustomTimeouts: resource.CustomTimeouts{},
		},
	}

	// Execute the step
	_, _, err = step.Apply()

	// For success cases, we expect no error
	if config.registrationResponses[0] == ResultStateSuccess {
		require.NoError(t, err)
	} else {
		// For failure cases, we expect an error (unless we retried and eventually succeeded)
		if attempts < config.totalRetries {
			require.Error(t, err)
		}
	}

	// Verify the number of attempts matches expected retries
	assert.Equal(t, config.totalRetries, attempts)
}
