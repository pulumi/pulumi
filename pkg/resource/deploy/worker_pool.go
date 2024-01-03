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

type workerPool struct {
	numWorkers int
	sem        chan struct{}
	wg         sync.WaitGroup

	errorMutex sync.Mutex
	errors     []error
}

func newWorkerPool(size int) *workerPool {
	if size <= 0 || size > runtime.GOMAXPROCS(0) {
		size = runtime.GOMAXPROCS(0)
	}
	return &workerPool{
		numWorkers: size,
		sem:        make(chan struct{}, size),
	}
}

func (s *workerPool) AddWorker(thunk func() error) {
	s.wg.Add(1)
	s.sem <- struct{}{}

	go func() {
		defer s.done()
		if err := thunk(); err != nil {
			s.addError(err)
		}
	}()
}

func (s *workerPool) done() {
	<-s.sem
	s.wg.Done()
}

func (s *workerPool) Wait() error {
	s.wg.Wait()

	return s.getErrors()
}

func (s *workerPool) addError(err error) {
	s.errorMutex.Lock()
	defer s.errorMutex.Unlock()

	s.errors = append(s.errors, err)
}

func (s *workerPool) getErrors() error {
	s.errorMutex.Lock()
	defer s.errorMutex.Unlock()

	switch len(s.errors) {
	case 0:
		return nil
	case 1:
		return s.errors[0]
	default:
		return errors.Join(s.errors...)
	}
}
