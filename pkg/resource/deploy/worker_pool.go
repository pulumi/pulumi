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
	errors     []error
}

// Creates a new workerPool. This will allow upto size concurrent workers.
//
// IF size is <= 1, OR > NumCPU THEN use the number of available Logical CPUs
func newWorkerPool(size int) *workerPool {
	if size <= 1 || size > runtime.NumCPU() {
		size = runtime.NumCPU()
	}
	return &workerPool{
		numWorkers: size,
		sem:        make(chan struct{}, size),
	}
}

// Add a worker to the pool.
//
// If possible this will start a goroutine for the worker, otherwise this will
// wait until the worker can be scheduled.
func (s *workerPool) AddWorker(thunk func() error) {
	s.wg.Add(1)
	s.sem <- struct{}{}

	go func() {
		defer func() {
			<-s.sem
			s.wg.Done()
		}()

		if err := thunk(); err != nil {
			s.errorMutex.Lock()
			defer s.errorMutex.Unlock()

			s.errors = append(s.errors, err)
		}
	}()
}

// Waits until all workers, including any pending workers, have completed
// execution.
//
// This returns an error that contains all errors (if any) returned by worker
// functions
//
// If clearErrors is true then this resets the error list in the pool
func (s *workerPool) Wait(clearErrors bool) error {
	s.wg.Wait()

	s.errorMutex.Lock()
	defer s.errorMutex.Unlock()

	var err error

	switch len(s.errors) {
	case 0:
		return nil
	case 1:
		err = s.errors[0]
	default:
		err = errors.Join(s.errors...)
	}

	if clearErrors {
		s.errors = nil
	}
	return err
}
