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
	"runtime"
	"sync"

	"github.com/hashicorp/go-multierror"
)

// workerPool is a worker pool that runs tasks in parallel.
// It's similar to errgroup.Group with the following differences:
//
//   - re-uses goroutines between tasks instead of spawning them anew
//   - allows multiple Wait calls separately from Close
//   - does not stop on the first error, recording them all instead
//
// This makes it a better fit for phased operations
// where we want to run all tasks to completion and report all errors
// and then run more tasks if those succeed
// without the overhead of spawning new goroutines.
//
// Typical usage is:
//
//	pool := newWorkerPool(numWorkers)
//	defer pool.Close()
//
//	for _, x := range items {
//		x := x // capture loop variable
//		pool.Enqueue(func() error {
//			...
//			return err
//		})
//	}
//	if err := pool.Wait(); err != nil {
//		...
//	}
//
//	// Add more tasks:
//	pool.Enqueue(func() error { ... })
//	...
//	if err := pool.Wait(); err != nil {
//		...
//	}
type workerPool struct {
	// Target number of workers.
	numWorkers int

	// Tracks individual workers.
	//
	// When this hits zero, all workers have exited and Close can return.
	workers sync.WaitGroup

	// Tracks ongoing tasks.
	//
	// When this hits zero, we no longer have any work in flight,
	// so Wait can return.
	ongoing sync.WaitGroup

	// Enqueues tasks for workers.
	tasks chan func() error

	// Errors encountered while running tasks.
	errs  []error
	errMu sync.Mutex // guards errs
}

// newWorkerPool creates a new worker pool with the given number of workers.
// If numWorkers is 0, it defaults to GOMAXPROCS.
//
// numTasksHint is a hint for the maximum number of tasks that will be enqueued
// before a Wait call. It's used to avoid overallocating workers.
// It may be 0 if the number of tasks is unknown.
func newWorkerPool(numWorkers, numTasksHint int) *workerPool {
	if numWorkers <= 0 {
		numWorkers = runtime.GOMAXPROCS(0)
	}

	// If we have a task hint, use it to avoid overallocating workers.
	if numTasksHint > 0 && numTasksHint < numWorkers {
		numWorkers = numTasksHint
	}

	p := &workerPool{
		numWorkers: numWorkers,
		// We use an unbuffered channel.
		// This blocks Enqueue until a worker is ready
		// to accept a task.
		// This keeps usage patterns simple:
		// a few Enqueue calls followed by a Wait call.
		// It discourages attempts to Enqueue from inside a worker
		// because that would deadlock.
		tasks: make(chan func() error),
	}

	p.workers.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go p.worker()
	}

	return p
}

func (p *workerPool) worker() {
	defer p.workers.Done()

	for task := range p.tasks {
		if err := task(); err != nil {
			p.errMu.Lock()
			p.errs = append(p.errs, err)
			p.errMu.Unlock()
		}
		p.ongoing.Done()
	}
}

// Enqueue adds a task to the pool.
// It blocks until the task is accepted by a worker.
// DO NOT call Enqueue from inside a worker
// or concurrently with Wait or Close.
//
// If the task returns an error, it can be retrieved from Wait.
func (p *workerPool) Enqueue(task func() error) {
	p.ongoing.Add(1)
	p.tasks <- task
}

// Wait blocks until all enqueued tasks have finished
// and returns all errors encountered while running them.
// The errors may be combined into a multierror.
//
// Consecutive calls to Wait return errors encountered since last Wait.
// Typically, you'll want to call Wait after all Enqueue calls
// and then again after any additional Enqueue calls.
//
//	pool.Enqueue(t1)
//	pool.Enqueue(t2)
//	pool.Enqueue(t3)
//	if err := pool.Wait(); err != nil {
//		// One or more of t1, t2, t3 failed.
//		return err
//	}
//
//	pool.Enqueue(t4)
//	pool.Enqueue(t5)
//	if err := pool.Wait(); err != nil {
//		// One or more of t4, t5 failed.
//		return err
//	}
func (p *workerPool) Wait() error {
	p.ongoing.Wait()

	p.errMu.Lock()
	var errors []error
	errors, p.errs = p.errs, nil
	p.errMu.Unlock()

	switch len(errors) {
	case 0:
		return nil
	case 1:
		return errors[0]
	default:
		return multierror.Append(errors[0], errors[1:]...)
	}
}

// Close stops the pool, and cleans up all resources.
//
// Do not call Enqueue or Wait after Close.
func (p *workerPool) Close() {
	close(p.tasks)
	p.workers.Wait()
}
