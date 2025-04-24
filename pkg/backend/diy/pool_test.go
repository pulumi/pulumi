// Copyright 2016-2023, Pulumi Corporation.
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

package diy

import (
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestWorkerPool_reusable(t *testing.T) {
	t.Parallel()

	pool := newWorkerPool(0, 0)
	defer pool.Close()

	const (
		numPhases = 10
		numTasks  = 100
	)

	// Verifies that a worker pool is re-usable
	// by enqueuing a bunch of tasks to it,
	// waiting for them, and then enqueuing more.
	//
	// Each enqueue-wait cycle is called a "phase".
	// We run multiple phases to verify that the pool
	// is re-usable and does not get stuck after the first phase.
	for phase := 0; phase < numPhases; phase++ {
		var count atomic.Int64
		for task := 0; task < numTasks; task++ {
			pool.Enqueue(func() error {
				count.Add(1)
				return nil
			})
		}
		require.NoError(t, pool.Wait())
		assert.Equal(t, int64(numTasks), count.Load())
	}
}

func TestWorkerPool_error(t *testing.T) {
	t.Parallel()

	pool := newWorkerPool(0, 0)
	defer pool.Close()

	const numTasks = 100

	// Create N unique errors to return from the tasks.
	errors := make([]error, numTasks)
	for i := range errors {
		errors[i] = fmt.Errorf("error %d", i)
	}

	for _, err := range errors {
		err := err
		pool.Enqueue(func() error {
			return err
		})
	}

	err := pool.Wait()
	require.Error(t, err)

	// Validate that the returned error matches
	// the errors returned by the tasks.
	for i, err := range errors {
		assert.ErrorIs(t, err, errors[i])
	}
}

func TestWorkerPool_oneError(t *testing.T) {
	t.Parallel()

	pool := newWorkerPool(0, 0)
	defer pool.Close()

	const numTasks = 10
	giveErr := errors.New("great sadness")
	for i := 0; i < numTasks; i++ {
		i := i
		pool.Enqueue(func() error {
			if i == 7 {
				return giveErr
			}
			return nil
		})
	}

	err := pool.Wait()
	require.Error(t, err)
	assert.ErrorIs(t, err, giveErr)
}

func TestWorkerPool_workerCount(t *testing.T) {
	t.Parallel()

	gomaxprocs := runtime.GOMAXPROCS(0)

	tests := []struct {
		desc         string
		numWorkers   int
		numTasksHint int
		wantWorkers  int
	}{
		{
			desc:        "default",
			wantWorkers: gomaxprocs,
		},
		{
			desc:        "negative",
			numWorkers:  -1,
			wantWorkers: gomaxprocs,
		},
		{
			desc:        "explicit",
			numWorkers:  2,
			wantWorkers: 2,
		},
		{
			desc:         "hint/too small",
			numWorkers:   4,
			numTasksHint: 2,
			wantWorkers:  2,
		},
		{
			desc:         "hint/large",
			numWorkers:   1,
			numTasksHint: 42,
			wantWorkers:  1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			pool := newWorkerPool(tt.numWorkers, tt.numTasksHint)
			defer pool.Close()

			assert.Equal(t, tt.wantWorkers, pool.numWorkers)
		})
	}
}

// Verifies that no combination of core actions on a pool
// can cause it to deadlock or panic.
func TestWorkerPool_randomActions(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		pool := newWorkerPool(0, 0)

		// Number of tasks epqueued but not yet running.
		var pending atomic.Int64

		// Runs a random sequence of actions from the
		// map of actions.
		t.Run(map[string]func(*rapid.T){
			"enqueue": func(t *rapid.T) {
				pending.Add(1)

				pool.Enqueue(func() error {
					defer pending.Add(-1)

					// Yield to other goroutines
					// so the enqueue doesn't resolve
					// immediately.
					runtime.Gosched()

					return nil
				})
			},
			"wait": func(t *rapid.T) {
				assert.NoError(t, pool.Wait())
				assert.Zero(t, pending.Load())
			},
		})

		pool.Close()
		assert.Zero(t, pending.Load())
	})
}
