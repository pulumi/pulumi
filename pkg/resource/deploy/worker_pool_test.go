// Copyright 2016-2024, Pulumi Corporation.
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
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestWorkerPool_NoError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.TODO())
	workerPool := newWorkerPool(0, cancel)

	const numTasks = 100

	for i := 0; i < numTasks; i++ {
		workerPool.AddWorker(func() error {
			runtime.Gosched()
			return nil
		})
	}

	err := workerPool.Wait(true)

	assert.Nil(t, err)
	assert.Nil(t, ctx.Err())
}

func TestWorkerPool_error(t *testing.T) {
	t.Parallel()

	_, cancel := context.WithCancel(context.TODO())
	workerPool := newWorkerPool(0, cancel)

	const numTasks = 100

	// Create N unique errors to return from the tasks.
	errors := make([]error, numTasks)
	for i := range errors {
		errors[i] = fmt.Errorf("error %d", i)
	}

	for _, err := range errors {
		err := err
		workerPool.AddWorker(func() error {
			return err
		})
	}

	err := workerPool.Wait(true)
	require.Error(t, err)

	// Validate that the returned error matches
	// the errors returned by the tasks.
	for i, err := range errors {
		assert.ErrorIs(t, err, errors[i])
	}
}

func TestWorkerPool_oneError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.TODO())
	workerPool := newWorkerPool(0, cancel)

	const numTasks = 10
	giveErr := errors.New("great sadness")
	for i := 0; i < numTasks; i++ {
		i := i
		workerPool.AddWorker(func() error {
			if i == 7 {
				return giveErr
			}
			return nil
		})
	}

	err := workerPool.Wait(true)
	require.Error(t, err)
	assert.ErrorIs(t, err, giveErr)

	assert.NotNil(t, ctx.Err())
	assert.ErrorIs(t, ctx.Err(), context.Canceled)
}

func TestWorkerPool_workerCount(t *testing.T) {
	t.Parallel()

	gomaxprocs := runtime.GOMAXPROCS(0)

	tests := []struct {
		desc            string
		numWorkers      int
		expectedWorkers int
	}{
		{
			desc:            "default",
			expectedWorkers: gomaxprocs,
		},
		{
			desc:            "negative",
			numWorkers:      -1,
			expectedWorkers: gomaxprocs,
		},
		{
			desc:            "explicit",
			numWorkers:      2,
			expectedWorkers: 2,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			_, cancel := context.WithCancel(context.TODO())
			workerPool := newWorkerPool(tt.numWorkers, cancel)

			assert.Equal(t, tt.expectedWorkers, workerPool.numWorkers)
		})
	}
}

// Verifies that no combination of core actions on a workerPool
// can cause it to deadlock or panic.
func TestWorkerPool_randomActions(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		ctx, cancel := context.WithCancel(context.TODO())
		workerPool := newWorkerPool(0, cancel)

		// Number of tasks epqueued but not yet running.
		var pending atomic.Int64
		var errorMutex sync.Mutex
		errors := make([]error, 0)

		// Runs a random sequence of actions from the
		// map of actions.
		t.Run(map[string]func(*rapid.T){
			"addWorkerNoError": func(t *rapid.T) {
				pending.Add(1)

				workerPool.AddWorker(func() error {
					defer pending.Add(-1)

					// Yield to other goroutines
					// so the enqueue doesn't resolve
					// immediately.
					runtime.Gosched()

					return nil
				})
			},
			"addWorkerWithError": func(t *rapid.T) {
				pending.Add(1)

				workerPool.AddWorker(func() error {
					defer pending.Add(-1)

					// Yield to other goroutines
					// so the enqueue doesn't resolve
					// immediately.
					runtime.Gosched()

					errorMutex.Lock()
					defer errorMutex.Unlock()

					currentError := len(errors)
					err := fmt.Errorf("%d", currentError)
					errors = append(errors, err)

					return err
				})
			},
			"wait": func(t *rapid.T) {
				err := workerPool.Wait(false)

				errorMutex.Lock()
				defer errorMutex.Unlock()
				if len(errors) == 0 {
					assert.NoError(t, err)
				} else {
					for _, el := range errors {
						assert.ErrorIs(t, err, el)
					}
				}
			},
		})

		err := workerPool.Wait(true)
		if len(errors) == 0 {
			assert.NoError(t, err)
			assert.Nil(t, ctx.Err())
		} else {
			for _, el := range errors {
				assert.ErrorIs(t, err, el)
			}
			assert.ErrorIs(t, ctx.Err(), context.Canceled)
		}
		assert.Zero(t, pending.Load())
	})
}
