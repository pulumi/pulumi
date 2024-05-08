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
	"runtime"
	"sync"
)

// A worker pool that can be used to constrain the number of concurrent workers.
type workerPool struct {
	numWorkers int
	// This channel is used as a semaphore to limit how many workers can be active
	sem chan struct{}
	wg  sync.WaitGroup

	errorMutex sync.Mutex
	cancel     context.CancelFunc
	errors     []error
}

// Creates a new workerPool. This will allow upto size concurrent workers.
// IF size is <= 1 THEN use the number of available Logical CPUs * 4
func newWorkerPool(size int, cancel context.CancelFunc) *workerPool {
	if size <= 1 {
		// This number is chosen to match the value of defaultParallel in /pkg/cmd/pulumi/up.go
		// See https://github.com/pulumi/pulumi/issues/14989 for context around the cpu * 4 choice.
		size = runtime.NumCPU() * 4
	}
	return &workerPool{
		numWorkers: size,
		sem:        make(chan struct{}, size),
		cancel:     cancel,
	}
}

// Add a worker to the pool.
//
// If possible this will start a goroutine for the worker, otherwise this will
// wait until the worker can be scheduled.
func (w *workerPool) AddWorker(thunk func() error) {
	w.wg.Add(1)
	w.sem <- struct{}{}

	go func() {
		defer func() {
			<-w.sem
			w.wg.Done()
		}()

		if err := thunk(); err != nil {
			w.errorMutex.Lock()
			defer w.errorMutex.Unlock()

			w.errors = append(w.errors, err)
			// cancel the context on the first error
			if len(w.errors) == 1 {
				w.cancel()
			}
		}
	}()
}

// Waits until all workers, including any pending workers, have completed
// execution.
//
// This returns an error that contains all errors (if any) returned by worker
// functions
func (w *workerPool) Wait() error {
	w.wg.Wait()

	w.errorMutex.Lock()
	defer w.errorMutex.Unlock()

	return errors.Join(w.errors...)
}
